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
