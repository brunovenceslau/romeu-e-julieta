// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package handoff holds the commands julieta handoff write, show and
// list (04 4.3, REQ-0413 to REQ-0415), one definition each in its own
// file, on the types, the reader and the print of internal/handoff.
//
// write stamps the git facts of every repo of the manifest into
// handoff/<ULID>-<kind>.md of the memory dir of the repo that contains
// the working directory, or of --repo (08 8.3, REQ-0677), under the
// store lock of that dir, taken before its first git read and held to
// the end (REQ-0655). show --hook is the SessionStart print (08 8.2) and
// write --facts the SessionEnd hook; both exit 0 whatever happens,
// print one line naming the failure and julieta status, and record it
// as a hook failure (REQ-0672); both record the time of their run
// (REQ-0669). The write path, with the lock, is here and not in
// internal/handoff, which imports memstore and never memstore/write.
package handoff

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Schema is the value of the schema field of the --json document of
// julieta handoff write and list, the format julieta-handoff.v1.
const Schema = "julieta-handoff/v1"

// Report is the --json document of julieta handoff write and list, the
// format julieta-handoff.v1: the handoff files the command wrote (one)
// or listed, oldest first (04 4.1, Idempotency, REQ-0280).
type Report struct {
	// Files are the handoff files.
	Files []File `json:"files"`
	// Schema is "julieta-handoff/v1".
	Schema string `json:"schema"`
}

// File is one handoff file with the repo whose memory dir holds it.
type File struct {
	// ID is the ULID of the file name.
	ID string `json:"id"`
	// Kind is clear, final or facts.
	Kind string `json:"kind"`
	// Path is the path of the file.
	Path string `json:"path"`
	// Repo is the dir of the repo it is filed under.
	Repo string `json:"repo"`
}

// Decode reads a --json document of the handoff commands: a duplicate member and
// another schema are each an error, and a member this build does not
// know is skipped, as a newer julieta of the same schema may add one
// (REQ-0163).
func Decode(data []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, err
	}
	if r.Schema != Schema {
		return Report{}, fmt.Errorf("the schema %q is not %s", r.Schema, Schema)
	}
	return r, nil
}

// sandbox is what a handoff command reads before it acts: $HOME and the
// manifest, whose every repo is checked before its values become a path
// or an argv element (REQ-0271).
type sandbox struct {
	home string
	m    manifest.Manifest
}

// load returns the sandbox of the running command. A missing manifest
// is exit 2 (REQ-0425).
func load() (sandbox, error) {
	home, err := setup.Home()
	if err != nil {
		return sandbox{}, err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return sandbox{}, err
	}
	for _, r := range m.Repos {
		if err := setup.CheckRepo(home, m.Project, r); err != nil {
			return sandbox{}, err
		}
		if !filepath.IsAbs(r.Memory) || filepath.Clean(r.Memory) != r.Memory {
			return sandbox{}, fmt.Errorf("the memory dir %q of the repo %s is not an absolute, clean path", r.Memory, r.Dir)
		}
	}
	return sandbox{home: home, m: m}, nil
}

// target returns the repo a handoff is filed under: the one whose dir
// is repo, or, when repo is empty, the one whose path holds the working
// directory, the deepest when repos nest (08 8.3, REQ-0677; never a
// question).
func (s sandbox) target(repo string) (manifest.Repo, error) {
	var found *manifest.Repo
	if repo != "" {
		for i := range s.m.Repos {
			if s.m.Repos[i].Dir == repo {
				found = &s.m.Repos[i]
			}
		}
		if found == nil {
			return manifest.Repo{}, fmt.Errorf("no repo of the manifest has the dir %q", repo)
		}
		return *found, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return manifest.Repo{}, err
	}
	for i := range s.m.Repos {
		if within(wd, s.m.Repos[i].Path) && (found == nil || len(s.m.Repos[i].Path) > len(found.Path)) {
			found = &s.m.Repos[i]
		}
	}
	if found == nil {
		return manifest.Repo{}, fmt.Errorf("the working directory %s is in no repo of the manifest: run the command in a repo, or name one with --repo", wd)
	}
	return *found, nil
}

// within reports whether the directory wd is dir or below it, with the
// symbolic links of either resolved when they can be, so a working
// directory reached through a link still names its repo.
func within(wd, dir string) bool {
	in := func(a, b string) bool {
		rel, err := filepath.Rel(b, a)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if in(wd, dir) {
		return true
	}
	rwd, err1 := filepath.EvalSymlinks(wd)
	rdir, err2 := filepath.EvalSymlinks(dir)
	return err1 == nil && err2 == nil && in(rwd, rdir)
}

// esc escapes s with termsafe, so a name or a text an agent wrote
// cannot forge a line of the output (I29).
func esc(s string) string { return termsafe.Escape(s) }

// usage returns the error usage (RJ-349) for a flag or an argument
// value that cmd refuses, with the hint of the help of the command.
func usage(cmd *cobra.Command, message string) error {
	e := cli.Usage.New(message)
	binary, command, _ := strings.Cut(cmd.CommandPath(), " ")
	e.Fill = map[string]string{"<binary>": binary, " <command>": ""}
	if command != "" {
		e.Fill[" <command>"] = " " + command
	}
	return e
}

// fileOf returns the report entry of the handoff file n of the repo.
func fileOf(repo, mem string, id canon.ULID, kind hf.Kind) File {
	return File{ID: id.String(), Kind: string(kind), Path: filepath.Join(mem, memstore.HandoffDir, hf.FileName(id, kind)), Repo: repo}
}

// output writes the report: the --json document, or text.
func output(env *cli.Env, rep Report, text func(out *cli.Writer) error) error {
	if rep.Files == nil {
		rep.Files = []File{}
	}
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	return text(env.Stdout)
}
