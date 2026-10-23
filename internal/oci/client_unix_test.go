// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package oci

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGuardNeverRunsACredentialHelper is the row of I28 "a credential
// helper present in a fake Docker config" (REQ-0515, REQ-0210): the
// config names a helper and holds a stored login for the registry, the
// helper is on the PATH and would leave a marker, and the client reads
// the registry anonymously all the same: the helper never runs, and no
// request carries a login.
func TestGuardNeverRunsACredentialHelper(t *testing.T) {
	f := newFakeRegistry(t)
	f.needToken = true
	index, _ := publish(t, f)
	registry := strings.TrimPrefix(f.srv.URL, "https://")

	dir := t.TempDir()
	marker := filepath.Join(dir, "helper-ran")
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.Mkdir(bin, 0o755))
	script := "#!/bin/sh\necho ran >> " + marker + "\necho '{\"Username\":\"u\",\"Secret\":\"s\"}'\n"
	for _, name := range []string{"docker-credential-evil", "docker-credential-desktop"} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755))
	}
	cfg := filepath.Join(dir, "docker")
	require.NoError(t, os.Mkdir(cfg, 0o755))
	login := base64.StdEncoding.EncodeToString([]byte("u:s"))
	config := `{"credsStore":"desktop","credHelpers":{"` + registry + `":"evil"},"auths":{"` + registry + `":{"auth":"` + login + `"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(cfg, "config.json"), []byte(config), 0o600))
	t.Setenv("DOCKER_CONFIG", cfg)
	t.Setenv("HOME", dir)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := f.client().FetchWorkload(t.Context(), f.ref(index), amd64)

	require.NoError(t, err)
	assert.NoFileExists(t, marker, "the helper never ran")
	for _, r := range f.requests() {
		_, auth, _ := strings.Cut(r, "\t")
		assert.NotContains(t, auth, "Basic", r)
		assert.NotContains(t, auth, login, r)
	}
}
