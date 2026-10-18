// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// Env is what a command gets from the binary instead of reaching for
// the process: the escaping writers, the clock, the ULID source, the
// version stamp, the settings path and the subprocess runner. A test
// builds one with fixed values (10 10.4).
type Env struct {
	// Binary is the name of the binary that runs: romeu or julieta.
	Binary string
	// Stdout and Stderr are the CLI writers of the two streams (I29).
	Stdout, Stderr *Writer
	// Now is the clock.
	Now func() time.Time
	// ULIDs is the one source of the ULIDs the command writes
	// (canon.NewULID).
	ULIDs *canon.ULIDSource
	// Version is the version stamp of the binary.
	Version Version
	// SettingsPath is the path of the host settings file (03 3.4); empty
	// for julieta.
	SettingsPath string
	// Run runs a subprocess and returns its combined output. The binary
	// sets it to a runner over internal/procgroup, under the class the
	// command names (04 4.1, Subprocesses); a test sets a fake.
	Run Runner
	// JSON is set when the command was given --json.
	JSON bool
}

// Runner runs the program argv names under the timeout class class of
// 04 4.1, Starting values, and returns its combined output.
type Runner func(ctx context.Context, class string, argv []string) ([]byte, error)

// Version is the version stamp a binary prints (04 4.1, Version).
type Version struct {
	Version  string
	Commit   string
	Date     string
	Protocol int
}
