// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorPassesAWellKeptHost: on a host where every check passes,
// doctor exits 0, reports one object per registered check, each with
// the result pass, and changes nothing it read (04 4.2, REQ-0331).
func TestDoctorPassesAWellKeptHost(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	rep, out, err := f.output(t)

	require.NoError(t, err, out)
	assert.Len(t, rep.Checks, len(rows))
	for _, c := range rep.Checks {
		assert.Equal(t, resultPass, c.Result, "%s: %s", c.Slug, c.Message)
	}
	assert.Empty(t, rep.Leftovers)
}

// TestDoctorExitsBySeverity: every failed check is printed whatever the
// exit; a check that could not run makes the exit 1 (doctor-incomplete,
// RJ-361) with the failed checks still printed, and failed checks alone
// make it 6 (check-findings, RJ-328); a warning changes no exit
// (REQ-0331).
func TestDoctorExitsBySeverity(t *testing.T) {
	t.Parallel()
	t.Run("a failed check exits 6 with its slug", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.settings.Root = f.home

		rep, out, err := f.output(t)

		assert.Equal(t, 6, exitOf(err), out)
		assert.Equal(t, resultFail, check(t, rep, "root-unsafe").Result)
	})
	t.Run("a check that could not run exits 1 and the failed checks are still printed", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.settings.Root = f.home
		f.sbx.listErr = os.ErrDeadlineExceeded

		rep, out, err := f.output(t)

		assert.Equal(t, 1, exitOf(err), out)
		assert.Equal(t, DoctorIncomplete.Slug, check(t, rep, "generation-sandbox").Error)
		assert.Equal(t, resultError, check(t, rep, "generation-sandbox").Result)
		assert.Equal(t, resultFail, check(t, rep, "root-unsafe").Result)
		assert.Contains(t, out, "root-unsafe")
	})
	t.Run("a warning alone exits 0", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		require.NoError(t, os.Mkdir(filepath.Join(f.root, "shop-env"), 0o700))

		rep, out, err := f.output(t)

		require.NoError(t, err, out)
		assert.Equal(t, resultWarn, check(t, rep, "root-indexed").Result)
	})
}

// TestDoctorListsLeftoverTemporaryFiles: the temporary files an
// interrupted atomic write left in the state directory are listed for
// the operator to remove, and none is deleted (T049, re-plan of
// 2026-10-22).
func TestDoctorListsLeftoverTemporaryFiles(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.store(t)
	left := filepath.Join(f.stateDir(), "projects", ".shop.json123456")
	require.NoError(t, os.WriteFile(left, []byte("{"), 0o600))

	rep, out, err := f.output(t)

	require.NoError(t, err, out)
	assert.Equal(t, []string{left}, rep.Leftovers)
	assert.FileExists(t, left, "doctor deletes nothing")
}
