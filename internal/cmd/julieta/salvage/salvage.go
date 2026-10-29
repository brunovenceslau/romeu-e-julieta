// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package salvage holds the command julieta salvage (04 4.3, REQ-0417),
// the sandbox half of the salvage of 08 8.5: with --stop-agents it
// stops the agent's processes and waits on every store lock (step 1),
// then internal/salvage captures the worktrees, the stashes, the
// bundles, the ignored files and the agent's state and writes one
// manifest per repo (steps 2 to 6). With --fingerprint it reads the
// fingerprint object of 03 3.8 and writes nothing. It passes
// internal/salvage the agent profile and write.Lock, which that
// package, linked by romeu too, does not import.
package salvage

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// flags are the flags of a run.
type flags struct {
	stopAgents, includeTranscripts, fingerprint bool
}

var _ = cli.Register(func() cli.Command {
	// Each salvage adds a salvage dir and its refs under its own id (04
	// 4.1, Idempotency, REQ-0278).
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.Appends("a salvage dir per repo and the refs of the salvage id"),
		cli.SnapshotNone, cli.AppendsEachRun)
	c.Binary, c.Name, c.JSON = "julieta", "salvage", true
	c.New = func(env *cli.Env) *cobra.Command {
		var f flags
		cmd := &cobra.Command{
			Use:   "salvage",
			Short: "capture every repo's unpushed work into its memory dir before the sandbox goes",
			Example: "julieta salvage --stop-agents --json\n" +
				"julieta salvage --fingerprint",
			Long: "Runs the sandbox half of a salvage for the salvageRun id of the manifest: with --stop-agents, it stops the " +
				"agent's processes and waits on every store lock first; then, for each repo of the manifest and each nested " +
				"repository in its worktrees, it records each worktree's uncommitted changes and each stash under " +
				"refs/salvage/<generation>/<salvage-id>/, writes one bundle per repository, archives the ignored files and the " +
				"agent's state within salvage.capBytes, and writes manifest.json last in memory/<dir>/salvage/<generation>/<salvage-id>/. " +
				"Transcripts are kept only with --include-transcripts. With --fingerprint it prints the fingerprint object of the " +
				"same reads and writes nothing.",
			Args: cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				if f.fingerprint && (f.stopAgents || f.includeTranscripts) {
					return usage(c, "--fingerprint writes nothing and stops no agent: give it alone")
				}
				return run(c.Context(), env, f)
			},
		}
		cmd.Flags().BoolVar(&f.stopAgents, "stop-agents", false, "stop the agent's processes and wait on the store locks first")
		cmd.Flags().BoolVar(&f.includeTranscripts, "include-transcripts", false, "keep the agent's transcripts, which may hold echoed tokens")
		cmd.Flags().BoolVar(&f.fingerprint, "fingerprint", false, "print the fingerprint object and write nothing")
		return cmd
	}
	return c
}())

// stopWait is how long --stop-agents waits after SIGTERM before SIGKILL
// (08 8.5 step 1, REQ-0695). A variable, so a test can lower it.
var stopWait = 10 * time.Second

// sharedDir is the dir of the project's shared memory dir, whose store
// lock stamps take (08 8.1, REQ-0642, REQ-0655).
const sharedDir = "_shared"

// run runs the command.
func run(ctx context.Context, env *cli.Env, f flags) error {
	home, err := setup.Home()
	if err != nil {
		return err
	}
	m, _, err := setup.LoadManifest(home)
	if err != nil {
		return err
	}
	repos, err := reposOf(home, m)
	if err != nil {
		return err
	}
	git, err := setup.NewGit(home)
	if err != nil {
		return err
	}
	if f.fingerprint {
		fp, err := salvage.ReadFingerprint(ctx, git, repos, m.Salvage.ExcludeIgnored)
		if err != nil {
			return err
		}
		return writeJSON(env, fp)
	}
	if m.SalvageRun == nil {
		return manifest.ManifestProtocol.New("the manifest has no salvageRun: julieta salvage runs on the salvage call of romeu, which passes the salvage id")
	}
	c := salvage.Config{
		Git: git, Home: home, Work: filepath.Join(setup.StateDir(home), "salvage", m.SalvageRun.ID),
		Repos: repos, Generation: m.Generation, SalvageID: m.SalvageRun.ID,
		CapBytes: m.Salvage.CapBytes, ExcludeIgnored: m.Salvage.ExcludeIgnored,
		Profile: profileOf(m.Agent), IncludeTranscripts: f.includeTranscripts,
		Now: env.Now, ULIDs: env.ULIDs,
	}
	if f.stopAgents {
		if err := stepOne(env, home, m, &c); err != nil {
			return err
		}
	}
	res, err := salvage.Salvage(ctx, c)
	if err != nil {
		return err
	}
	return report(env, m.SalvageRun.ID, res)
}

// stepOne runs step 1 of 08 8.5: it stops the agent's processes, says
// whether none matched while a session is open (REQ-0696), and waits on
// the store lock of every repo's memory dir and of the shared one
// (REQ-0697).
func stepOne(env *cli.Env, home string, m manifest.Manifest, c *salvage.Config) error {
	var names []string
	if c.Profile != nil {
		names = processes
	}
	stop, err := stopAgents(names, stopWait)
	if err != nil {
		return err
	}
	c.AgentStop = &stop
	if stop.Matched == 0 && c.Profile != nil {
		h, err := handoff.ReadHooks(home)
		if err != nil {
			return err
		}
		c.AgentNotMatched = !h.SessionStartAt.IsZero() && h.SessionEndAt.Before(h.SessionStartAt)
	}
	locks := make([]string, 0, len(m.Repos)+1)
	for _, r := range m.Repos {
		locks = append(locks, write.LockPath(home, r.Dir))
	}
	locks = append(locks, write.LockPath(home, sharedDir))
	return salvage.WaitLocks(locks, write.Lock, func(path string) {
		if env.JSON {
			b, err := json.Marshal(progress{Path: path, Progress: "lock-wait"}, json.Deterministic(true))
			if err == nil {
				_, _ = env.Stderr.Write(append(b, '\n'))
			}
			return
		}
		_, _ = fmt.Fprintf(env.Stderr, "waiting for the store lock %s\n", path)
	})
}

// progress is the line --json prints on stderr while salvage waits more
// than salvage.LockReport on a store lock (REQ-0698).
type progress struct {
	Path     string `json:"path"`
	Progress string `json:"progress"`
}

// processes are the names of the agent's processes, from its profile.
var processes = claude.Processes

// profileOf returns the agent profile the manifest's agent handle
// names, or nil when no profile of this build serves it.
func profileOf(agent string) *salvage.AgentProfile {
	if !claude.Matches(agent) {
		return nil
	}
	p := &salvage.AgentProfile{Name: claude.Name, Transcript: transcript}
	for _, path := range claude.Paths {
		p.Paths = append(p.Paths, salvage.AgentPath{Rel: path.Rel, Required: path.Required})
	}
	return p
}

// transcriptDir is where the agent keeps a transcript per session, one
// JSON Lines file each (probe C5).
const transcriptDir = ".claude/projects/"

// transcript reports whether the file rel below $HOME is a transcript
// of the agent: a .jsonl file below its projects dir.
func transcript(rel string) bool {
	return strings.HasPrefix(rel, transcriptDir) && strings.HasSuffix(rel, ".jsonl")
}

// reposOf returns the repos of m, each checked as setup checks it, with
// a memory dir that is an absolute, clean path (REQ-0271).
func reposOf(home string, m manifest.Manifest) ([]salvage.Repo, error) {
	repos := make([]salvage.Repo, 0, len(m.Repos))
	for _, r := range m.Repos {
		if err := setup.CheckRepo(home, m.Project, r); err != nil {
			return nil, fmt.Errorf("the repo %s: %w", termsafe.Escape(r.Dir), err)
		}
		if !filepath.IsAbs(r.Memory) || filepath.Clean(r.Memory) != r.Memory {
			return nil, fmt.Errorf("the memory dir %q of the repo %s is not an absolute, clean path", termsafe.Escape(r.Memory), r.Dir)
		}
		repos = append(repos, salvage.Repo{Dir: r.Dir, Path: r.Path, Memory: r.Memory, Base: r.Base, Primary: r.Primary})
	}
	return repos, nil
}

// usage returns the error usage (RJ-349) for flags cmd refuses (04 4.1,
// Parsing; exit 2), with the hint of the help of the command.
func usage(cmd *cobra.Command, message string) error {
	e := cli.Usage.New(message)
	binary, command, _ := strings.Cut(cmd.CommandPath(), " ")
	e.Fill = map[string]string{"<binary>": binary, " <command>": ""}
	if command != "" {
		e.Fill[" <command>"] = " " + command
	}
	return e
}
