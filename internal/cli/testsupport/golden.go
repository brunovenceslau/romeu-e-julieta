// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package testsupport

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// ciVariables are the variables a CI runner sets: CI on every runner,
// GITHUB_ACTIONS on GitHub's.
var ciVariables = []string{"CI", "GITHUB_ACTIONS"}

// FailOnUpdateInCI fails the test when update, the -update flag of a
// golden test, is set in CI: a golden changes only with -update, run
// locally, and CI never rewrites one (10 10.4, REQ-0941, REQ-0942).
// Every golden helper with an -update flag calls it first; tools/ci
// golden refuses the flag in the invocations CI runs, and this guard
// holds at run time whatever runs the test.
func FailOnUpdateInCI(tb testing.TB, update bool) {
	tb.Helper()
	if !update {
		return
	}
	for _, v := range ciVariables {
		if os.Getenv(v) != "" {
			require.FailNow(tb, "-update rewrites the goldens, and it runs only locally; "+v+" is set, so this is CI (REQ-0942)")
		}
	}
}
