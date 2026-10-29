// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
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

	// Step 2: the hardened git of the host settings, the path of the
	// config repo's clone, and the spec commit, a full object id the
	// fetched config ref reaches.
	git         *gitsafe.Runner
	configClone string
	specCommit  string

	// Step 3: the specs the command line names, validated, with their
	// secrets bound, in the order of the config repo's projects/.
	targets []spec.Project

	// Step 4: the part of the host tree of each target, in the order of
	// targets: its env dir, its memory dirs, the clone of each repo and
	// its egress ref and commit.
	trees []tree

	// Steps 4 and 5 (and each later step that warns): the warnings of
	// the run, in the order they were found. Each is a print to the CLI
	// writer of stderr, which step 10 makes after its own output (04 4.2:
	// the warnings close the output); a print, not a slug and a message,
	// so each slug stays a literal of its cli.Warn call, as the test of
	// 04 4.1, Warnings reads it.
	warnings []func(*cli.Writer)
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
