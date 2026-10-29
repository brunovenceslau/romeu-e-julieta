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
	"regexp"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// definition is the command's registered definition.
var definition = cli.Register(func() cli.Command {
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
				if err := f.check(); err != nil {
					return err
				}
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

// fromOrigin is the value of --from that names the head of the config
// ref.
const fromOrigin = "origin"

// fullObjectID is a full SHA-1 or SHA-256 object id, as --from takes
// it: gitsafe takes no abbreviated id.
var fullObjectID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

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

// check refuses, as usage (exit 2), a command line whose shape is wrong
// whatever the spec holds: no project name and no --all, both together,
// or a --from that is neither origin nor a full object id. It runs
// before step 1, so such a line takes no lock and writes nothing; a
// name the spec lacks is step 3's (project-unknown).
func (f flags) check() error {
	switch {
	case f.all && len(f.names) > 0:
		return usage("targets", "--all and project names exclude each other")
	case !f.all && len(f.names) == 0:
		return usage("targets", "name the projects to sync, or pass --all")
	case f.from != fromOrigin && !fullObjectID.MatchString(f.from):
		return usage("from-invalid", "--from "+termsafe.Escape(f.from)+" is neither origin nor a full commit id")
	}
	return nil
}
