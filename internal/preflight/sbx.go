// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package preflight

import (
	"context"
	"errors"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// HostSbx is the real Sbx: it reads the sbx of the host through the
// runner of sbxdrv, and nothing else (T168). The runner has passed the
// floor check of its command (RequireFloor) before the preflight runs.
type HostSbx struct {
	// Runner runs the sbx calls.
	Runner *sbxdrv.Runner
	// Opts are what the calls know about the command that makes them.
	Opts sbxdrv.Opts
}

var _ Sbx = (*HostSbx)(nil)

// List is sbx ls --json, decoded by sbxdrv.
func (h *HostSbx) List(ctx context.Context) ([]sbxdrv.Sandbox, error) {
	return h.Runner.List(ctx, h.Opts)
}

// PolicyList is the output of sbx policy ls --json, which parsePolicies
// reads.
func (h *HostSbx) PolicyList(ctx context.Context) ([]byte, error) {
	return h.read(ctx, sbxdrv.PolicyList(), "sbx policy ls --json failed, so the policies of sbx are not known")
}

// SecretList is the output of sbx secret ls, which parseSecrets reads.
func (h *HostSbx) SecretList(ctx context.Context) ([]byte, error) {
	return h.read(ctx, sbxdrv.SecretList(), "sbx secret ls failed, so the secrets of sbx are not known")
}

// read runs a listing. A call that fails to run or exits non-zero is
// sbx-unknown (RJ-308), as for sbx ls --json: nothing is known and
// nothing changes. A timeout and a cancellation pass on unchanged.
func (h *HostSbx) read(ctx context.Context, c sbxdrv.Call, why string) ([]byte, error) {
	res, err := h.Runner.Run(ctx, c, h.Opts)
	if err != nil {
		if ce, ok := errors.AsType[*sbxdrv.CallError](err); ok && !timedOut(err) {
			return nil, sbxdrv.SbxUnknown.Wrap(ce, why, ce.Details()...)
		}
		return nil, err
	}
	if res.Truncated {
		return nil, upstreamShape(c.Name, "the output is longer than romeu reads")
	}
	return res.Stdout, nil
}

// timedOut reports whether the error holds the timeout of procgroup.
func timedOut(err error) bool {
	_, ok := errors.AsType[*procgroup.TimeoutError](err)
	return ok
}
