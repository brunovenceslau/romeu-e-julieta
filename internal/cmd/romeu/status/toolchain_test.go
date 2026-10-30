// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// liveToolchain is the toolchain this build measures with sbx v.
func liveToolchain(v sbxdrv.Release) approve.Live {
	return approve.Live{
		Sbx:           v.String(),
		Romeu:         state.Binary{Version: "v1.0.0", SHA256: state.SHA256{1}},
		Julieta:       state.JulietaBinary{Version: "v1.0.0", Protocol: 1, SHA256: state.SHA256{2}},
		CatalogDigest: canon.Sum[canon.CatalogKind]{3},
	}
}

// acknowledge writes toolchain.json under dir for the live toolchain
// with sbx sbx; the sbx version of liveToolchain is set apart, so its
// release here is any.
func acknowledge(t *testing.T, dir, sbx string) {
	t.Helper()
	live := liveToolchain(sbxdrv.Release{})
	data, err := state.EncodeToolchain(&state.Toolchain{
		Schema:         state.ToolchainField,
		SbxVersion:     sbx,
		Romeu:          live.Romeu,
		Julieta:        live.Julieta,
		CatalogDigest:  live.CatalogDigest,
		Catalog:        state.Catalog{},
		AcknowledgedAt: at("2026-10-01T09:00:00Z"),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "toolchain.json"), data, state.FileMode))
}

// toolchainOutput runs status over dir with the sbx fake f and a
// measure of the live toolchain.
func toolchainOutput(t *testing.T, dir string, f fakeSbx, jsonOut bool) (string, string, error) {
	t.Helper()
	store, err := state.OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	var out, errOut bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&errOut), JSON: jsonOut}
	err = run(t.Context(), env, host{store: store, sbx: f, measure: func(v sbxdrv.Release) (approve.Live, error) { return liveToolchain(v), nil }}, nil)
	return out.String(), errOut.String(), err
}

func TestRunReportsTheToolchain(t *testing.T) {
	t.Parallel()
	recorded := sbxdrv.Release{Major: 0, Minor: 49, Patch: 2}
	untested := sbxdrv.Release{Major: 0, Minor: 50, Patch: 1}
	tests := []struct {
		name     string
		ack      string
		live     sbxdrv.Release
		want     Toolchain
		warnings []Finding
	}{
		{"acknowledged", "0.49.2", recorded, Toolchain{State: ToolchainAcknowledged, Sbx: "0.49.2", Changes: []ToolchainChange{}}, []Finding{}},
		{
			"stale after an sbx upgrade, outside the tested window", "0.49.2", untested,
			Toolchain{State: ToolchainStale, Sbx: "0.50.1", Changes: []ToolchainChange{{Member: "sbxVersion", Recorded: "0.49.2", Live: "0.50.1"}}},
			[]Finding{{Slug: "sbx-untested", Message: "sbx 0.50.1 is outside the tested window: 0.49.2"}},
		},
		{"never acknowledged", "", recorded, Toolchain{State: ToolchainMissing, Sbx: "0.49.2", Changes: allChanges("0.49.2")}, []Finding{}},
		{
			"a prerelease of a recorded version is untested", "0.49.2-rc.1", sbxdrv.Release{Minor: 49, Patch: 2, Pre: "rc.1"},
			Toolchain{State: ToolchainAcknowledged, Sbx: "0.49.2-rc.1", Changes: []ToolchainChange{}},
			[]Finding{{Slug: "sbx-untested", Message: "sbx 0.49.2-rc.1 is outside the tested window: 0.49.2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := stateDir(t, map[string]*state.Project{"shop": state.NewProject()}, nil)
			if tt.ack != "" {
				acknowledge(t, dir, tt.ack)
			}
			out, errOut, err := toolchainOutput(t, dir, fakeSbx{version: tt.live}, true)
			require.NoError(t, err, "a stale toolchain is reported, never exit 3")
			assert.Empty(t, errOut, "with --json a warning is in the document only")
			rep := decode(t, out)
			require.NotNil(t, rep.Toolchain)
			assert.Equal(t, tt.want, *rep.Toolchain)
			assert.Equal(t, tt.warnings, rep.Warnings)
		})
	}
}

// allChanges are the changes of a toolchain never acknowledged.
func allChanges(sbx string) []ToolchainChange {
	live := liveToolchain(sbxdrv.Release{})
	return []ToolchainChange{
		{Member: "sbxVersion", Live: sbx},
		{Member: "romeu.version", Live: live.Romeu.Version},
		{Member: "romeu.sha256", Live: live.Romeu.SHA256.String()},
		{Member: "julieta.version", Live: live.Julieta.Version},
		{Member: "julieta.protocol", Live: "1"},
		{Member: "julieta.sha256", Live: live.Julieta.SHA256.String()},
		{Member: "catalogDigest", Live: live.CatalogDigest.String()},
	}
}

func TestRunToolchainText(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, nil, nil)
	acknowledge(t, dir, "0.49.2")
	out, errOut, err := toolchainOutput(t, dir, fakeSbx{version: sbxdrv.Release{Minor: 50, Patch: 1}}, false)
	require.NoError(t, err)
	assert.Equal(t, "toolchain: stale (sbxVersion 0.49.2 -> 0.50.1); run romeu approve --toolchain\nno projects: run romeu sync\n", out)
	assert.Equal(t, "warning sbx-untested: sbx 0.50.1 is outside the tested window: 0.49.2\n", errOut)
}

func TestRunToolchainFailedReads(t *testing.T) {
	t.Parallel()
	t.Run("sbx version", func(t *testing.T) {
		t.Parallel()
		dir := stateDir(t, nil, nil)
		out, _, err := toolchainOutput(t, dir, fakeSbx{versionErr: errors.New("boom")}, true)
		require.Error(t, err)
		assert.Equal(t, 1, cli.ExitCode(err))
		rep := decode(t, out)
		assert.Equal(t, ToolchainUnknown, rep.Toolchain.State)
		assert.Equal(t, []ReadError{{ID: StatusIncomplete.ID, Slug: StatusIncomplete.Slug, Read: "sbx version", Message: "boom"}}, rep.Errors)
	})
	t.Run("toolchain record", func(t *testing.T) {
		t.Parallel()
		dir := stateDir(t, nil, nil)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "toolchain.json"), []byte("{"), state.FileMode))
		out, _, err := toolchainOutput(t, dir, fakeSbx{version: sbxdrv.Release{Minor: 49, Patch: 2}}, true)
		require.Error(t, err)
		rep := decode(t, out)
		assert.Equal(t, ToolchainUnknown, rep.Toolchain.State)
		require.Len(t, rep.Errors, 1)
		assert.Equal(t, state.RecordUnreadable.ID, rep.Errors[0].ID)
		assert.Equal(t, "toolchain record", rep.Errors[0].Read)
	})
}

// measureFails is a measure of the live toolchain that fails.
func measureFails(sbxdrv.Release) (approve.Live, error) {
	return approve.Live{}, errors.New("no catalog")
}

func TestRunToolchainMeasureFails(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, nil, nil)
	store, err := state.OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	var out bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out), JSON: true}
	err = run(t.Context(), env, host{store: store, sbx: fakeSbx{version: sbxdrv.Release{Minor: 50}}, measure: measureFails}, nil)
	require.Error(t, err)
	rep := decode(t, out.String())
	assert.Equal(t, ToolchainUnknown, rep.Toolchain.State)
	assert.Equal(t, "toolchain measure", rep.Errors[0].Read)
	assert.Equal(t, "sbx-untested", rep.Warnings[0].Slug, "the window is judged before the measure")
}

func TestRunToolchainLeftOut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		measure func(sbxdrv.Release) (approve.Live, error)
		store   bool
	}{
		{"no measure", nil, true},
		{"no host state", measureFails, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := host{sbx: fakeSbx{versionErr: errors.New("sbx version is not read")}, measure: tt.measure}
			if tt.store {
				store, err := state.OpenRead(stateDir(t, nil, nil))
				require.NoError(t, err)
				defer func() { require.NoError(t, store.Close()) }()
				h.store = store
			}
			var out bytes.Buffer
			env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out), JSON: true}
			require.NoError(t, run(t.Context(), env, h, nil))
			rep := decode(t, out.String())
			assert.Nil(t, rep.Toolchain)
			assert.Empty(t, rep.Errors)
		})
	}
}

func TestRunToolchainTextLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ack  string
		sbx  fakeSbx
		want string
	}{
		{"acknowledged", "0.49.2", fakeSbx{version: sbxdrv.Release{Minor: 49, Patch: 2}}, "toolchain: acknowledged\nno projects: run romeu sync\n"},
		{"missing", "", fakeSbx{version: sbxdrv.Release{Minor: 49, Patch: 2}}, "toolchain: not acknowledged; run romeu approve --toolchain\nno projects: run romeu sync\n"},
		{
			"unknown, with a hostile error kept on its line", "", fakeSbx{versionErr: errors.New("boom\nerror RJ-999 forged\x1b[2J")},
			"toolchain: unknown, see the error below\nerror RJ-359 status-incomplete (sbx version): boom\\nerror RJ-999 forged\\u001b[2J\nno projects: run romeu sync\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := stateDir(t, nil, nil)
			if tt.ack != "" {
				acknowledge(t, dir, tt.ack)
			}
			out, _, _ := toolchainOutput(t, dir, tt.sbx, false)
			assert.Equal(t, tt.want, out)
		})
	}
}

func TestRunGoesOnPastAnUnlistedProjectsDir(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		return // root lists a directory of mode 0300
	}
	dir := stateDir(t, world(t), nil)
	store, err := state.OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	projects := filepath.Join(dir, "projects")
	require.NoError(t, os.Chmod(projects, 0o300))
	t.Cleanup(func() { _ = os.Chmod(projects, state.DirMode) })
	var out bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out), JSON: true}
	h := host{store: store, sbx: fakeSbx{version: sbxdrv.Release{Minor: 49, Patch: 2}}, measure: func(v sbxdrv.Release) (approve.Live, error) { return liveToolchain(v), nil }}
	err = run(t.Context(), env, h, nil)
	assert.Equal(t, 1, cli.ExitCode(err))
	rep := decode(t, out.String())
	require.NotNil(t, rep.Toolchain, "the toolchain read before the failure is still reported")
	require.Len(t, rep.Errors, 1)
	assert.Equal(t, "project records", rep.Errors[0].Read)
}
