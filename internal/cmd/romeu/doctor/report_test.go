// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// TestDoctorPassesAWellKeptHost: on a host where every check passes,
// doctor exits 0 and reports one object per registered check, each with
// the result pass but vscode-trust, a warning while the trusted folders
// of VS Code are not checked (04 4.2, REQ-0331).
func TestDoctorPassesAWellKeptHost(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	rep, out, err := f.output(t)

	require.NoError(t, err, out)
	assert.Len(t, rep.Checks, len(rows))
	for _, c := range rep.Checks {
		want := resultPass
		if c.Slug == slugVSCodeTrust {
			want = resultWarn
		}
		assert.Equal(t, want, c.Result, "%s: %s", c.Slug, c.Message)
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

// TestDoctorWithoutUsableSettings: settings that are missing, or a HOME
// that is not absolute, make every check that needs them an error with
// the error usage, and doctor exits 1, not 2; the checks that need
// neither still run and are printed (REQ-0331).
func TestDoctorWithoutUsableSettings(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(f *fixture)
	}{
		{"the settings are missing", func(f *fixture) { f.noSettings = true }},
		{"HOME is relative", func(f *fixture) { f.env["HOME"] = "home" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			tc.setup(f)

			rep, out, err := f.output(t)

			assert.Equal(t, 1, exitOf(err), out)
			require.Len(t, rep.Checks, len(rows))
			for _, c := range rep.Checks {
				if c.Slug == "record-unreadable" {
					assert.Equal(t, resultPass, c.Result, "a check that needs no settings still runs")
					continue
				}
				assert.Equal(t, resultError, c.Result, c.Slug)
				assert.Equal(t, "usage", c.Error, c.Slug)
			}
		})
	}
}

// TestDoctorBelowTheSbxFloor: sbx below the floor fails sbx-floor, and
// every check that needs sbx to run is an error with sbx-floor's id
// (RJ-343), so doctor exits 1; sbx-untested still judges the version it
// read (REQ-0366, REQ-0367).
func TestDoctorBelowTheSbxFloor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.sbx.release = sbxdrv.Release{Major: 0, Minor: 40, Patch: 0}

	rep, out, err := f.output(t)

	assert.Equal(t, 1, exitOf(err), out)
	assert.Equal(t, resultFail, check(t, rep, "sbx-floor").Result)
	assert.Equal(t, resultWarn, check(t, rep, "sbx-untested").Result)
	for _, slug := range []string{"toolchain-approval-required", slugGlobalSecret, slugBroadAllow, "generation-sandbox", "removal-pending"} {
		c := check(t, rep, slug)
		assert.Equal(t, resultError, c.Result, slug)
		assert.Equal(t, sbxdrv.SbxFloor.ID, c.ID, slug)
	}
}

// TestDoctorJSONIsItsSchema: the --json document of a report with every
// kind of result validates against schemas/romeu-doctor.v1.json, the
// generated schema of the format.
func TestDoctorJSONIsItsSchema(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.settings.Root = f.home
	f.sbx.listErr = os.ErrDeadlineExceeded
	require.NoError(t, os.Mkdir(filepath.Join(f.root, "shop-env"), 0o700))

	_, out, _ := f.output(t)

	doc, _, _ := strings.Cut(out, "\n")
	sch, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schemas", "romeu-doctor.v1.json"))
	require.NoError(t, err)
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(sch))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("romeu-doctor.v1.json", schemaDoc))
	compiled, err := c.Compile("romeu-doctor.v1.json")
	require.NoError(t, err)
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	require.NoError(t, err)
	assert.NoError(t, compiled.Validate(inst))
	for _, r := range []string{resultPass, resultWarn, resultFail, resultError} {
		assert.Contains(t, doc, `"result":"`+r+`"`, "the document holds every result")
	}
}
