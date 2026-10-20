// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package setup holds the command julieta setup (04 4.3): it links the
// binary on PATH, clones the missing secondary repos, fetches origin in
// every repo and fast-forwards the clean default branches, recording
// the warnings in julieta state for julieta status and the next
// SessionStart. Its steps are the slice of steps.go. Every git call
// runs through gitsafe in sandbox mode (05 5.1).
//
// The package also holds what julieta's commands share about the
// manifest and julieta state: LoadManifest, ReadState, NewGit and
// CheckRepo.
package setup

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// run is one run of setup: what its steps read and what they report.
type run struct {
	home   string
	m      manifest.Manifest
	git    *gitsafe.Runner
	report Report
}

// fail records a failure that leaves the sandbox unusable.
func (r *run) fail(format string, args ...any) {
	r.report.Unusable = append(r.report.Unusable, fmt.Sprintf(format, args...))
}

// warn records a warning of this run.
func (r *run) warn(slug, message string) {
	r.report.Warnings = append(r.report.Warnings, Warning{Slug: slug, Message: message})
}

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name, c.JSON = "julieta", "setup", true
	c.New = func(env *cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:     "setup",
			Short:   "prepare the sandbox: the PATH link, the repos and their default branches",
			Example: "julieta setup\njulieta setup --json",
			Args:    cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return setup(c.Context(), env)
			},
		}
	}
	return c
}())

// setup runs the steps, records the state, writes the report and
// returns the error of an unusable sandbox, exit 1 (04 4.3).
func setup(ctx context.Context, env *cli.Env) error {
	home, err := Home()
	if err != nil {
		return err
	}
	m, fromEnv, err := LoadManifest(home)
	if err != nil {
		return err
	}
	git, err := NewGit(home)
	if err != nil {
		return err
	}
	r := &run{home: home, m: m, git: git,
		report: Report{Schema: Schema, Version: env.Version.Version, Errors: []string{}, Repos: []Repo{}, Unusable: []string{}, Warnings: []Warning{}}}
	if fromEnv {
		if err := cacheManifest(home, m); err != nil {
			r.fail("the manifest cache: %v", err)
		}
	}
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		s(ctx, r)
	}
	prev, err := ReadState(home)
	if err != nil {
		r.report.Errors = append(r.report.Errors, fmt.Sprintf("%v; this run's state replaces it", err))
	}
	if err := recordState(home, prev, env.Now(), env.Version.Version, r.report.Warnings); err != nil {
		r.fail("julieta state: %v", err)
	}
	if err := write(env, r.report); err != nil {
		return err
	}
	if len(r.report.Unusable) > 0 {
		return fmt.Errorf("the sandbox is left unusable: %s", strings.Join(r.report.Unusable, "; "))
	}
	return nil
}

// write writes the report: the warnings and the reads that failed to
// stderr, as every warning goes (04 4.1, Output and Warnings;
// cli.Warn), and the --json
// document or the text lines to stdout.
func write(env *cli.Env, rep Report) error {
	for _, w := range rep.Warnings {
		PrintWarning(env.Stderr, w)
	}
	for _, e := range rep.Errors {
		if _, err := fmt.Fprintf(env.Stderr, "error  %s\n", e); err != nil {
			return err
		}
	}
	if env.JSON {
		b, err := json.Marshal(rep, json.Deterministic(true))
		if err != nil {
			return err
		}
		_, err = env.Stdout.Write(append(b, '\n'))
		return err
	}
	if rep.Link.Target != "" {
		state := "kept"
		if rep.Link.Changed {
			state = "made"
		}
		if _, err := fmt.Fprintf(env.Stdout, "link  %s -> %s (%s)\n", rep.Link.Path, rep.Link.Target, state); err != nil {
			return err
		}
	}
	for _, repo := range rep.Repos {
		if _, err := fmt.Fprintf(env.Stdout, "repo  %s  %s\n", repo.Dir, repoLine(repo)); err != nil {
			return err
		}
	}
	return nil
}

// repoLine says what setup did in one repo, in one line.
func repoLine(r Repo) string {
	var parts []string
	if r.Cloned {
		parts = append(parts, "cloned")
	}
	if r.Fetched {
		parts = append(parts, "fetched")
	}
	switch r.FastForward {
	case ffMoved:
		parts = append(parts, fmt.Sprintf("%s moved %s..%s", r.DefaultBranch, r.From, r.To))
	case ffNotFetched:
	default:
		parts = append(parts, strings.TrimSpace(r.DefaultBranch+" "+r.FastForward))
	}
	if r.Error != "" {
		parts = append(parts, "error: "+r.Error)
	}
	return strings.Join(parts, ", ")
}

// PrintWarning prints the warning w to the CLI writer out. Each slug
// julieta records is a literal of its call, which the test of 04 4.1,
// Warnings reads; a slug julieta does not record is printed as what it
// is, an entry of julieta state this build does not know.
func PrintWarning(out *cli.Writer, w Warning) {
	switch w.Slug {
	case FetchFailed:
		cli.Warn(out, "fetch-failed", w.Message)
	case DefaultBranchMoved:
		cli.Warn(out, "default-branch-moved", w.Message)
	default:
		_, _ = fmt.Fprintf(out, "julieta state holds a warning this julieta does not know, %q: %s\n", w.Slug, w.Message)
	}
}
