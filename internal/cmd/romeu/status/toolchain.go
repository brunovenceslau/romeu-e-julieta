// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"fmt"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The reads of the toolchain.
const (
	readSbxVersion       = "sbx version"
	readToolchainMeasure = "toolchain measure"
	readToolchainRecord  = "toolchain record"
)

// readToolchain fills the toolchain of rep, gate 1's comparison
// (approve.Changes, REQ-0062) of the live toolchain with the record,
// which status reports and does not stop on, and the warning
// sbx-untested for a live sbx outside the tested window (REQ-0063,
// REQ-0933), with v the sbx version the floor check read. A failed floor
// check, floorErr, is shown here and counted by run. It returns how many
// other reads failed.
func readToolchain(h host, rep *Report, v sbxdrv.Release, floorErr error) int {
	tc := &Toolchain{State: ToolchainUnknown, Changes: []ToolchainChange{}}
	rep.Toolchain = tc
	if floorErr != nil {
		rep.Errors = append(rep.Errors, readError(readSbxVersion, floorErr))
		return 0
	}
	tc.Sbx = v.String()
	if !slices.Contains(sbxdrv.RecordedVersions, tc.Sbx) {
		rep.Warnings = append(rep.Warnings, Finding{
			Slug:    "sbx-untested",
			Message: fmt.Sprintf("sbx %s is outside the tested window: %s", tc.Sbx, strings.Join(sbxdrv.RecordedVersions, ", ")),
		})
	}
	live, err := h.measure(v)
	if err != nil {
		rep.Errors = append(rep.Errors, readError(readToolchainMeasure, err))
		return 1
	}
	var rec *state.Toolchain
	if h.store != nil {
		if rec, err = approve.Load(h.store); err != nil {
			rep.Errors = append(rep.Errors, readError(readToolchainRecord, err))
			return 1
		}
	}
	for _, c := range approve.Changes(rec, live) {
		tc.Changes = append(tc.Changes, ToolchainChange{Member: c.Member, Recorded: c.Recorded, Live: c.Live})
	}
	switch {
	case rec == nil:
		tc.State = ToolchainMissing
	case len(tc.Changes) > 0:
		tc.State = ToolchainStale
	default:
		tc.State = ToolchainAcknowledged
	}
	return 0
}

// writeToolchain prints the toolchain line of the text report; nothing
// for a toolchain that was not read.
func writeToolchain(out *cli.Writer, tc *Toolchain) error {
	if tc == nil {
		return nil
	}
	var s string
	switch tc.State {
	case ToolchainAcknowledged:
		s = "toolchain: acknowledged"
	case ToolchainMissing:
		s = "toolchain: not acknowledged; run romeu approve --toolchain"
	case ToolchainStale:
		changes := make([]string, 0, len(tc.Changes))
		for _, c := range tc.Changes {
			changes = append(changes, fmt.Sprintf("%s %s -> %s", c.Member, c.Recorded, c.Live))
		}
		s = fmt.Sprintf("toolchain: stale (%s); run romeu approve --toolchain", strings.Join(changes, ", "))
	default:
		s = "toolchain: unknown, see the error below"
	}
	_, err := fmt.Fprintln(out, line(s))
	return err
}
