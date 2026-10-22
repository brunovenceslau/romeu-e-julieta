// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"crypto/rand"
	"os"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// SetExecutable makes f find the running binary until the returned
// function restores os.Executable.
func SetExecutable(f func() (string, error)) (restore func()) {
	executable = f
	return func() { executable = os.Executable }
}

// WriteState exposes writeState to the external tests.
var WriteState = writeState

// SetStateOnWait sets the seam called when the lock of setup.json is
// held by another and returns the function that restores it.
func SetStateOnWait(f func()) (restore func()) {
	old := stateOnWait
	stateOnWait = f
	return func() { stateOnWait = old }
}

// RecordState exposes recordState to the external tests.
var RecordState = recordState

// RecordSnapshotsDisabledForTest runs the step recordSnapshotsDisabled
// alone on the manifest m, its repos left as they are, and returns the
// report it filled.
func RecordSnapshotsDisabledForTest(ctx context.Context, home string, m manifest.Manifest) Report {
	git, err := NewGit(home)
	if err != nil {
		panic(err)
	}
	r := &run{home: home, m: m, git: git, now: time.Now, ulids: canon.NewULIDSource(rand.Reader)}
	recordSnapshotsDisabled(ctx, r)
	return r.report
}
