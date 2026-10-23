// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package sbxdrv is the only code that runs sbx (12 12.2, 04 4.1,
// Subprocesses): the argv builders, one file for each sbx call romeu
// makes; the runner that starts a call by absolute path, in the
// scrubbed environment of 04 4.1 and under the timeout class the
// builder names, through internal/procgroup; and the parsers of the
// outputs of sbx that romeu reads.
//
// Every shape here comes from the sbx the package is written for and
// from the sessions under e2e/testdata/sbx/<version>/ (10 10.3): a
// builder has a session that holds its argv shape, and a shape in a
// session has a builder, which a test holds. The guards of I4 (no
// --auto-approve, and no flag that approves a plan without asking) and
// I25 (sbx secret set is only called with --command) are in guard.go.
//
// The output of sbx is a format between binaries that romeu does not
// own: a parser skips a member it does not know, requires the members
// it needs, and returns the error upstream-shape (RJ-310) for a shape
// it does not recognize, never an empty result (03, opening, REQ-0160 and
// REQ-0170; the acceptance of T029).
package sbxdrv
