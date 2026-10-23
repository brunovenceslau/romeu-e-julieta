// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package testsupport_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// guardFails reports whether FailOnUpdateInCI fails for update.
func guardFails(t *testing.T, update bool) (failed bool) {
	t.Helper()
	r := &recorder{TB: t}
	defer func() {
		if p := recover(); p != nil && p != r {
			panic(p)
		}
		failed = r.failed
	}()
	testsupport.FailOnUpdateInCI(r, update)
	return false
}

// The -update flag of a golden test fails the test in CI, where CI or
// GITHUB_ACTIONS is set, so a CI run never rewrites a golden
// (REQ-0942); locally, and in CI without the flag, the test runs.
func TestFailOnUpdateInCI(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	assert.False(t, guardFails(t, true), "locally, -update rewrites the goldens")
	assert.False(t, guardFails(t, false))

	t.Setenv("CI", "true")
	assert.True(t, guardFails(t, true), "CI=true with -update")
	assert.False(t, guardFails(t, false), "CI=true without -update")

	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	assert.True(t, guardFails(t, true), "GITHUB_ACTIONS=true with -update")
	assert.False(t, guardFails(t, false), "GITHUB_ACTIONS=true without -update")
}
