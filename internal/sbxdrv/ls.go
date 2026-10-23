// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"context"
	"errors"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

var _ = registerBuilder("ls", func(Fixture) []Call { return []Call{List()} })

// List is sbx ls --json: a read (timeout class sbx-read), captured. Its
// output is read by ParseList; the identity step of the preflight, the
// generation machine and run step 3 read it.
func List() Call {
	return mustCall((&argvBuilder{}).lit("ls", "--json").call("ls", procgroup.SbxRead, Captured))
}

// List runs sbx ls --json and reads its output. A call that fails to run
// or exits non-zero is the error sbx-unknown (RJ-308, exit 1): nothing
// is known about the sandboxes, and nothing changes (01 1.6, REQ-0102);
// its details are those of the failed call. A call that ran past its
// timeout is subprocess-timeout (RJ-323), as for every call. An output
// that is not a list is upstream-shape (RJ-310), with the live sbx
// version in its details when the runner knows it.
func (r *Runner) List(ctx context.Context, o Opts) ([]Sandbox, error) {
	res, err := r.Run(ctx, List(), o)
	if err != nil {
		if ce, ok := errors.AsType[*CallError](err); ok && !timedOut(err) {
			return nil, SbxUnknown.Wrap(ce, "sbx ls --json failed, so it is not known whether a sandbox exists", ce.Details()...)
		}
		return nil, err
	}
	list, err := ParseList(res.Stdout)
	if err != nil {
		return nil, r.withLive(err)
	}
	return list, nil
}

// withLive adds the live sbx version to the details of an error
// upstream-shape, when the runner knows it.
func (r *Runner) withLive(err error) error {
	if e, ok := errors.AsType[*cli.Error](err); ok && e.ID == cli.UpstreamShape.ID && r.Versions.Sbx != "" {
		e.Details = append(e.Details, cli.Detail{Key: "sbx version", Value: r.Versions.Sbx})
	}
	return err
}

// timedOut reports whether the error holds the timeout of procgroup.
func timedOut(err error) bool {
	_, ok := errors.AsType[*procgroup.TimeoutError](err)
	return ok
}
