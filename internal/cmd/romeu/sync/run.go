// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// run is what one romeu sync carries from step to step. Each field
// names the step that fills it; a later step only reads it. A slice of
// T043 adds the fields its step fills, with the code that fills them.
type run struct {
	// env is the CLI environment; flags is the command line.
	env   *cli.Env
	flags flags

	// Step 1: the state store and the lock taken on it, which runSteps
	// releases.
	store *state.Store
	lock  *state.Lock
	// Step 1: the host settings, validated, and HOME, an absolute path,
	// which every git and sbx call runs with.
	settings spec.HostSettings
	home     string
	// Step 1: the julieta binary of .romeu/bin, whose files render.json
	// names.
	bin render.Bin

	// Step 2: the hardened git of the host settings, the path of the
	// config repo's clone, and the spec commit, a full object id the
	// fetched config ref reaches.
	git         *gitsafe.Runner
	configClone string
	specCommit  string

	// Step 3: every spec of the config repo's projects/, validated, in
	// its order, which step 10 reads, and the specs the command line
	// names among them, with their secrets bound.
	specs   []spec.Project
	targets []spec.Project

	// Step 4: the part of the host tree of each target, in the order of
	// targets: its env dir, its memory dirs, the clone of each repo and
	// its egress ref and commit.
	trees []tree

	// Step 6: the candidate of each target, in the order of targets:
	// its dir, its kits, its egress, its widening set and digest, the
	// candidate side of the gate 2 diff and its render.json, which
	// step 8 reads.
	candidates []candidate

	// Step 7: the drifted live files --overwrite-drift lets step 8's
	// promotion replace, in the order of targets, which step 8 adds to
	// the warnings as lost edits once their project is promoted.
	overwritten []lostEdit

	// Steps 4 and 5 (and each later step that warns): the warnings of
	// the run, in the order they were found. Each is a print to the CLI
	// writer of stderr, which runSteps makes once the steps end, after
	// step 10's output or the step that failed (04 4.2: the warnings
	// close the output); a print, not a slug and a message, so each slug
	// stays a literal of its cli.Warn call, as the test of 04 4.1,
	// Warnings reads it.
	warnings []func(*cli.Writer)
}

// newRun returns the run of one command line.
func newRun(env *cli.Env, f flags) *run {
	return &run{env: env, flags: f}
}

// warn prints the warnings of the run to the CLI writer of stderr, in
// the order they were found, once.
func (r *run) warn() {
	if r.env == nil || r.env.Stderr == nil {
		return
	}
	for _, w := range r.warnings {
		w(r.env.Stderr)
	}
	r.warnings = nil
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
