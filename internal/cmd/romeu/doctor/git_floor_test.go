// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// TestDoctorGitFloor: gitsafe's floor check failing is git-floor
// (REQ-0368); git that cannot be run at all leaves the check unrun.
func TestDoctorGitFloor(t *testing.T) {
	t.Parallel()
	t.Run("below the floor", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.gitErr = &gitsafe.FloorError{Found: "2.30.0"}

		rep, out, err := f.output(t)

		assert.Equal(t, resultFail, check(t, rep, "git-floor").Result, out)
		assert.Equal(t, 6, exitOf(err))
	})
	t.Run("another failure is an error", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.gitErr = errors.New("git cannot be started")

		rep, out, err := f.output(t)

		assert.Equal(t, resultError, check(t, rep, "git-floor").Result, out)
		assert.Equal(t, 1, exitOf(err))
	})
}
