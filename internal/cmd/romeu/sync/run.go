// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// run is what one romeu sync carries from step to step. Each field
// names the step that fills it; a later step only reads it. The fields
// are the contract between the steps' files, so a step needs no field
// of its own here beyond its comment.
type run struct {
	// env is the CLI environment; flags is the command line.
	env   *cli.Env
	flags flags

	// Step 1: the host settings, HOME, the state store and the lock
	// taken on it, which runSteps releases.
	settings spec.HostSettings
	home     string
	store    *state.Store
	lock     *state.Lock

	// Step 2: the hardened git runner of the host, the config repo's
	// clone dir and the spec commit, the head of the config ref or the
	// --from commit.
	git        *gitsafe.Runner
	configDir  string
	specCommit string

	// Step 3: every spec at the spec commit, validated together, and
	// the projects this run syncs, in the order of their names.
	specs   []spec.Project
	targets []*target
}

// target is one project this run syncs.
type target struct {
	// spec is the project's validated spec, its defaults set (step 3).
	spec spec.Project

	// Step 4: the project's <name>-env/ dir and the egress commit of
	// each repo, by repo dir, resolved from refs/romeu/origin/<dir>/<ref>.
	envDir        string
	egressCommits map[string]string

	// Step 6: the project record as read, the candidate, what was
	// rendered into it, the egress derived from the pinned commits, the
	// widening set with its digest, and the record written as the
	// candidate's render.json.
	record    *state.Project
	candidate *state.Candidate
	rendered  render.Rendered
	egress    egress.Result
	set       gate.Set
	widening  canon.Sum[canon.WideningKind]
	renderRec render.Record

	// Step 7: the live files that drift from the current render.json,
	// which --overwrite-drift lets step 8 replace.
	drifted []string

	// Step 8: whether the candidate was promoted.
	promoted bool
}

// newRun returns the run of one command line.
func newRun(env *cli.Env, f flags) *run {
	return &run{env: env, flags: f}
}

// release unlocks the state store's lock and closes the store, when
// step 1 took them.
func (r *run) release() {
	if r.lock != nil {
		r.lock.Unlock()
		r.lock = nil
	}
	if r.store != nil {
		_ = r.store.Close()
		r.store = nil
	}
}
