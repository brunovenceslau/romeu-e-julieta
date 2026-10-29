// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// step is one step of 04 4.2, "How romeu sync renders, gates and
// promotes": its number there, a name for errors and tests, and its
// function, which reads what earlier steps left in the run and fills
// the fields its comment in run.go names.
type step struct {
	n    int
	name string
	do   func(context.Context, *run) error
}

// steps are the steps of 04 4.2 in their order (rule 14 of
// docs/plan.md): a task that adds a step edits this slice by one line.
// Step 9 (live changes) stays dark until T047.
var steps = []step{
	{n: 1, name: "lock, settings and gate 1", do: lockAndCheck},
	{n: 2, name: "fetch the config repo", do: fetchConfig},
	{n: 3, name: "read and validate the specs", do: readSpecs},
	{n: 4, name: "create the tree and fetch the repos", do: makeTree},
	{n: 5, name: "check the memory dirs", do: checkMemory},
	{n: 6, name: "read descriptors and render the candidate", do: renderCandidates},
	{n: 7, name: "check the live files for drift", do: notBuilt},
	{n: 8, name: "gate 2 and promote", do: gateAndPromote},
	{n: 9, name: "apply live changes", do: skipLive},
	{n: 10, name: "report orphans and the julieta pin", do: notBuilt},
}

// errNotBuilt is what a step returns until its slice of T043 lands.
var errNotBuilt = errors.New("not built yet")

// notBuilt is each step whose slice of T043 has not landed: the slice
// replaces it, in its line of steps, with the step's function in a file
// of its own.
func notBuilt(context.Context, *run) error { return errNotBuilt }

// runSteps runs each step in order and stops at the first error, which
// it returns as is when it is an error of the table (cli.Error) and
// wrapped with the step otherwise. It releases the lock step 1 took,
// whatever happened.
func runSteps(ctx context.Context, r *run, list []step) error {
	defer r.release()
	for _, s := range list {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.do(ctx, r); err != nil {
			if _, ok := errors.AsType[*cli.Error](err); ok {
				return err
			}
			return fmt.Errorf("sync: step %d (%s): %w", s.n, s.name, err)
		}
	}
	return nil
}

// skipLive is step 9, which T047 builds: until then a running sandbox
// gets no live change, and nothing reports a recreate.
func skipLive(context.Context, *run) error { return nil }
