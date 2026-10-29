// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// TestSyncHarnessEmbedsJulieta: the romeu of romeu.NewEmbedded, the
// host-native build of tools/release, carries a julieta (REQ-0131, the
// e2e row of 10 10.2), which gate 1 reads: it prints no placeholder,
// and records a dev build's version and a sha256 that is not the
// placeholder's, the sha256 of no bytes.
func TestSyncHarnessEmbedsJulieta(t *testing.T) {
	t.Parallel()
	e := romeu.NewEmbedded(t, nil)
	e.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}})
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, e.SetSbxSession(session))

	res := approve(t, e)

	assert.NotContains(t, res.Stderr, "v0.0.0-placeholder")
	assert.NotContains(t, res.Stderr, "sha256 e3b0c44298fc")
	r := readToolchain(t, e)
	assert.True(t, strings.HasPrefix(r.Julieta.Version, "v0.0.0-dev+"), r.Julieta.Version)
	assert.Len(t, r.Julieta.SHA256, 64)
	assert.NotEqual(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", r.Julieta.SHA256)
}

// syncSetup is an Env, embedded or plain, with host settings and the
// fake sbx replaying the handshake of the recorded sbx 0.49.2, which
// answers sbx version.
func syncSetup(t *testing.T, embedded bool) *romeu.Env {
	t.Helper()
	e := romeu.New
	if embedded {
		e = romeu.NewEmbedded
	}
	env := e(t, nil)
	env.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}})
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, env.SetSbxSession(session))
	return env
}

// TestSyncRefusesABuildWithoutJulieta: sync is linked into romeu, and a
// plain go build, which embeds no julieta (REQ-0130), exits 2 with
// julieta-not-embedded (RJ-312) at step 1, before it takes the lock,
// calls sbx or writes anything (04 4.2, REQ-0335).
func TestSyncRefusesABuildWithoutJulieta(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, false)

	res := e.Run(t, []string{"sync", "--all"})

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "julieta-not-embedded")
	assert.Empty(t, e.SbxCalls(t), "no sbx call")
	assert.NoDirExists(t, e.StateDir(), "no state is written")
	assert.NoDirExists(t, e.Root, "nothing is written under the root")
}

// TestSyncStopsAtGate1: the romeu of the embedded build passes the
// julieta check of step 1 and stops at gate 1 with
// toolchain-approval-required (RJ-302, exit 3) on a toolchain not
// acknowledged, having read sbx version alone, so a stale toolchain
// stops sync before it promotes (04 4.2, REQ-0335).
func TestSyncStopsAtGate1(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, true)

	res := e.Run(t, []string{"sync", "--all"})

	assert.Equal(t, 3, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "toolchain-approval-required")
	assert.NotContains(t, res.Stderr, "julieta-not-embedded")
	assert.Equal(t, [][]string{{"version"}}, e.SbxCalls(t))
	assert.NoDirExists(t, e.Root, "nothing is written under the root")
}

// TestSyncPassesGate1: once the toolchain is acknowledged, sync passes
// step 1 and stops at step 2 when no clone of the config repo exists,
// exit 2 naming romeu init, with nothing written under the root.
func TestSyncPassesGate1(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, true)
	approve(t, e)

	res := e.Run(t, []string{"sync", "--all"})

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "config-clone-missing")
	assert.NoDirExists(t, e.Root, "nothing is written under the root")
}
