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
// with sbx sbx.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := stateDir(t, map[string]*state.Project{"shop": state.NewProject()}, nil)
			if tt.ack != "" {
				acknowledge(t, dir, tt.ack)
			}
			out, _, err := toolchainOutput(t, dir, fakeSbx{version: tt.live}, true)
			require.NoError(t, err, "a stale toolchain is reported, never exit 3")
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
