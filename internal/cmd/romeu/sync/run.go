// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
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
// are the contract between the steps' files: a step that needs a value
// no field holds asks for the field to be added here, in one change,
// rather than keeping it on the side.
type run struct {
	// env is the CLI environment; flags is the command line.
	env   *cli.Env
	flags flags

	// Step 1: the host settings, HOME, the state store and the lock
	// taken on it, which runSteps releases; the julieta binary of
	// .romeu/bin, built once from the embedded bytes, so a romeu with
	// none stops here (julieta-not-embedded); and the egress catalog
	// with its digest, which gate 1 checked.
	settings      spec.HostSettings
	home          string
	store         *state.Store
	lock          *state.Lock
	bin           render.Bin
	catalog       catalog.Catalog
	catalogDigest canon.Sum[canon.CatalogKind]

	// Step 2: the hardened git runner of the host, the config repo's
	// clone dir, and the spec commit with the romeu-owned ref it was
	// read from (render.SpecSource).
	git        *gitsafe.Runner
	configDir  string
	specRef    string
	specCommit string

	// Step 3: every spec at the spec commit, validated together, and
	// the projects this run syncs, in the order of their names.
	specs   []spec.Project
	targets []*target

	// Any step: the warnings of the run, in the order they were raised,
	// which step 10 prints last, so they close the command's output
	// (04 4.2).
	warnings []warning
}

// target is one project this run syncs. A step runs over every target,
// and an error on one stops the run.
type target struct {
	// spec is the project's validated spec, its defaults set (step 3).
	spec spec.Project

	// Step 4: the project's <name>-env/ dir, and by repo dir: the clone
	// dir, the memory dir, the romeu-owned ref egressCommit was resolved
	// from (refs/romeu/origin/<dir>/<ref>) and that commit.
	envDir        string
	cloneDirs     map[string]string
	memoryDirs    map[string]string
	egressRefs    map[string]string
	egressCommits map[string]string

	// Step 6: the project record as read, the candidate, what was
	// rendered into it, the egress derived from the pinned commits, the
	// candidate side of gate 2 (its widening set, kit files, secret argv
	// and render), and the record written as the candidate's
	// render.json, which holds the widening and recreate digests.
	record    *state.Project
	candidate *state.Candidate
	rendered  render.Rendered
	egress    egress.Result
	side      gate.Side
	renderRec render.Record

	// Step 7: the live files that drift from the current render.json,
	// which --overwrite-drift lets step 8 replace.
	drifted []string

	// Step 8: what became of the candidate: "promoted", "unchanged"
	// (promoted with no prompt), "declined" or "no-terminal" (exit 3).
	// Step 8 also writes the project record's lastRunAt.
	outcome string
}

// warning is one warning of the run: its slug and its message.
type warning struct {
	slug, message string
}

// newRun returns the run of one command line.
func newRun(env *cli.Env, f flags) *run {
	return &run{env: env, flags: f}
}

// warn records a warning, which step 10 prints.
func (r *run) warn(slug, message string) {
	r.warnings = append(r.warnings, warning{slug, message})
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
