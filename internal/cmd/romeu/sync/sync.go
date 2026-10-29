// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package sync is the command romeu sync (04 4.2, REQ-1221): it renders
// the specs at a named commit into candidates, runs gate 2, promotes,
// and reports. The steps of 04 4.2, "How romeu sync renders, gates and
// promotes", run in the order of the slice in steps.go, each in a file
// of its own; the render-time rules join through the registry of
// rules.go (rule 14 of docs/plan.md).
//
// The command is dark until all/import_sync.go links it in (T043).
package sync

import (
	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightRuns, cli.LockTaken, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "sync"
	c.New = func(env *cli.Env) *cobra.Command {
		var f flags
		cmd := &cobra.Command{
			Use:     "sync [--all] [--from origin|<sha>] [--overwrite-drift] [<name>...]",
			Short:   "render the specs into candidates, run gate 2 and promote",
			Example: "romeu sync shop",
			Args:    cobra.ArbitraryArgs,
			RunE: func(c *cobra.Command, names []string) error {
				f.names = names
				return runSteps(c.Context(), newRun(env, f), steps)
			},
		}
		cmd.Flags().BoolVar(&f.all, "all", false, "sync every project of the spec")
		cmd.Flags().StringVar(&f.from, "from", "origin", "the spec commit: origin, the head of the config ref, or a sha reachable from refs/romeu/origin/*")
		cmd.Flags().BoolVar(&f.overwriteDrift, "overwrite-drift", false, "replace an edited live file; its edit is lost")
		return cmd
	}
	return c
}())

// flags are the command line of one run.
type flags struct {
	// all is --all.
	all bool
	// from is --from: "origin" or a commit sha.
	from string
	// overwriteDrift is --overwrite-drift.
	overwriteDrift bool
	// names are the project names given as arguments.
	names []string
}
