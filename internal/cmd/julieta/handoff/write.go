// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.Appends("a handoff file"), cli.SnapshotNone, cli.AppendsEachRun)
	c.Binary, c.Name, c.Sub, c.JSON = "julieta", "handoff", "write", true
	c.New = func(env *cli.Env) *cobra.Command {
		var repo string
		var final, facts bool
		cmd := &cobra.Command{
			Use:   "write",
			Short: "write a handoff of the session to the repo's memory dir",
			Example: "julieta handoff write < handoff.md\n" +
				"julieta handoff write --final < handoff.md\n" +
				"julieta handoff write --facts",
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if final && facts {
					return usage(cmd, "--final and --facts exclude each other")
				}
				if facts {
					return runHook(env, HookSessionEnd, "julieta handoff write --facts", func() (string, string, error) {
						return writeFacts(cmd.Context(), env, repo)
					}, nil)
				}
				kind := hf.Clear
				if final {
					kind = hf.Final
				}
				body, err := readNarrative(cmd, kind)
				if err != nil {
					return err
				}
				return writeNarrative(cmd.Context(), env, repo, kind, body)
			},
		}
		cmd.Flags().BoolVar(&final, "final", false, "write a final handoff, before recreate or rm; it needs a sixth heading")
		cmd.Flags().BoolVar(&facts, "facts", false, "write a facts handoff, with no narrative (the SessionEnd hook)")
		repoFlag(cmd, &repo)
		return cmd
	}
	return c
}())

// repoFlag adds --repo to c.
func repoFlag(c *cobra.Command, repo *string) {
	c.Flags().StringVar(repo, "repo", "", "the repo dir to file under, instead of the repo of the working directory")
}

// readNarrative reads the narrative from stdin (REQ-0413) and refuses
// one that is over the file cap, is not UTF-8, or lacks its headings
// (REQ-0245), as the error usage, before julieta reads any git.
func readNarrative(cmd *cobra.Command, kind hf.Kind) (string, error) {
	data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), memstore.MaxFileSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > memstore.MaxFileSize {
		return "", usage(cmd, fmt.Sprintf("the narrative is over %d bytes, the cap of a handoff file", memstore.MaxFileSize))
	}
	if !utf8.Valid(data) {
		return "", usage(cmd, "the narrative is not UTF-8")
	}
	body := string(data)
	if body != "" && body[len(body)-1] != '\n' {
		body += "\n"
	}
	if err := hf.CheckHeadings(kind, body); err != nil {
		return "", usage(cmd, esc(err.Error()))
	}
	return body, nil
}

// writeNarrative writes a clear or final handoff and prints what it
// wrote.
func writeNarrative(ctx context.Context, env *cli.Env, repo string, kind hf.Kind, body string) error {
	f, _, err := writeHandoff(ctx, env, repo, kind, body)
	if err != nil {
		return err
	}
	text, err := wrote(env, f)
	if err != nil {
		return err
	}
	_, err = env.Stdout.Write([]byte(text))
	return err
}

// writeFacts is the SessionEnd hook: it writes a facts handoff and
// returns the text that says so, and the repo it filed under.
func writeFacts(ctx context.Context, env *cli.Env, repo string) (string, string, error) {
	f, label, err := writeHandoff(ctx, env, repo, hf.Facts, "")
	if err != nil {
		return "", label, err
	}
	text, err := wrote(env, f)
	return text, f.Repo, err
}

// wrote returns what a write prints: the file it wrote, its id, its
// path and the repo it filed it under (REQ-0280, REQ-0413), as a line
// of text or, with --json, one document.
func wrote(env *cli.Env, f File) (string, error) {
	if env.JSON {
		b, err := json.Marshal(Report{Schema: Schema, Files: []File{f}}, json.Deterministic(true))
		return string(b) + "\n", err
	}
	return fmt.Sprintf("wrote %s %s %s repo %s\n", f.ID, f.Kind, esc(f.Path), esc(f.Repo)), nil
}

// writeHandoff writes the handoff file of kind with the body: it takes
// the store lock of the target dir first, before its first git read,
// holds it to the end (REQ-0655), stamps the facts of every repo of the
// manifest (REQ-0679), writes handoff/<ULID>-<kind>.md 0600 in a dir
// 0700 (REQ-0646, REQ-0678) and, for facts, keeps the newest
// hf.KeepFacts facts files (REQ-0649). A handoff/ over its cap warns
// elsewhere and never refuses a write (REQ-0650). It returns the dir of
// the repo it filed under as soon as it knows it, so a failure can name
// it.
func writeHandoff(ctx context.Context, env *cli.Env, repo string, kind hf.Kind, body string) (f File, label string, err error) {
	sb, err := load()
	if err != nil {
		return File{}, label, err
	}
	target, err := sb.target(repo)
	if err != nil {
		return File{}, label, err
	}
	label = target.Dir
	unlock, err := lockFile(write.LockPath(sb.home, target.Dir))
	if err != nil {
		return File{}, label, err
	}
	defer unlock()
	if beforeGit != nil {
		beforeGit()
	}
	git, err := setup.NewGit(sb.home)
	if err != nil {
		return File{}, label, err
	}
	facts := make([]hf.RepoFacts, 0, len(sb.m.Repos))
	for _, r := range sb.m.Repos {
		rf, err := gather(ctx, git, r)
		if err != nil {
			return File{}, label, err
		}
		facts = append(facts, rf)
	}
	// The id takes the clock to the millisecond, not to the second the
	// front matter shows: two julieta processes of one second, such as
	// /handoff and the SessionEnd of /clear, are ordered by their ULIDs
	// (REQ-0680, REQ-0685), and within one millisecond they are not.
	at := env.Now()
	now := memstore.Stamp(at)
	id, err := canon.NewULID(at, env.ULIDs)
	if err != nil {
		return File{}, label, err
	}
	file := hf.File{Schema: hf.Field, Kind: kind, CreatedAt: now, Project: sb.m.Project,
		Generation: sb.m.Generation, Agent: sb.m.Agent, Repos: facts, Body: body}
	data, err := hf.Encode(file)
	if err != nil {
		return File{}, label, err
	}
	r, err := memstore.Open(target.Memory)
	if err != nil {
		return File{}, label, fmt.Errorf("the memory dir %s of the repo %s: %w", target.Memory, target.Dir, err)
	}
	defer func() { _ = r.Close() }()
	if err := mkdir(r.Root(), memstore.HandoffDir); err != nil {
		return File{}, label, err
	}
	if err := atomicWrite(r.Root(), hf.Path(hf.Name{ID: id, Kind: kind}), data, at, env.ULIDs); err != nil {
		return File{}, label, err
	}
	if kind == hf.Facts {
		if _, err := prune(r.Root()); err != nil {
			return File{}, label, fmt.Errorf("the facts handoff %s is written, but handoff/ was not pruned: %w", id, err)
		}
	}
	return fileOf(target.Dir, target.Memory, id, kind), label, nil
}
