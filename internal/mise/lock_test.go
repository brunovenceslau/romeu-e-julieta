// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
)

// fixture returns the bytes of a file of testdata.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}

// TestParseLock reads the lock of this repository, written by the
// pinned mise: each tool with its backend, its version and the
// platforms it covers.
func TestParseLock(t *testing.T) {
	lock, err := mise.ParseLock(fixture(t, "valid.lock"))
	require.NoError(t, err)
	assert.Equal(t, 3, lock.Version)
	require.Len(t, lock.Tools["gh"], 1)
	gh := lock.Tools["gh"][0]
	assert.Equal(t, "aqua:cli/cli", gh.Backend)
	assert.Equal(t, "2.102.0", gh.Version)
	assert.Equal(t, []string{"linux-arm64", "linux-x64", "macos-arm64", "macos-x64"}, gh.PlatformNames())
	assert.True(t, strings.HasPrefix(gh.Platforms["linux-x64"].Checksum, "sha256:"))
	assert.Equal(t, "go:golang.org/x/vuln/cmd/govulncheck", lock.Tools["go:golang.org/x/vuln/cmd/govulncheck"][0].Backend)
	assert.Empty(t, lock.Tools["go:golang.org/x/vuln/cmd/govulncheck"][0].Platforms)
}

// TestParseLockRefuses holds the reader to the layout of the pinned
// mise: a duplicate key, a top-level key it does not have, text that is
// not TOML and a file over the cap are each refused, never read in
// part (REQ-0629, REQ-1049).
func TestParseLockRefuses(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"a duplicate key", fixture(t, "invalid-duplicate-key.lock"), "already been defined"},
		{"an unknown top-level key", fixture(t, "invalid-unknown-key.lock"), "surprise"},
		{"not toml", []byte("lockfile_version = \n"), "lock"},
		{"over the cap", []byte("# " + strings.Repeat("x", mise.MaxFileSize) + "\n"), "cap"},
		{"an unknown layout version", []byte("lockfile_version = 4\n"), "lockfile_version"},
		{"no layout version", []byte("[[tools.go]]\nversion = \"1\"\n"), "lockfile_version"},
		{"nesting past the cap", []byte("lockfile_version = 3\nx = " + strings.Repeat("[", 64) + strings.Repeat("]", 64) + "\n"), "nesting"},
		{"a tool that is not a list of entries", []byte("lockfile_version = 3\n[tools]\ngo = 1\n"), "go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mise.ParseLock(tt.data)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestParseConfig reads the mise.toml of this repository: its tools,
// and no lockfile_platforms.
func TestParseConfig(t *testing.T) {
	cfg, err := mise.ParseConfig(fixture(t, "valid-config.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "gh", "golangci-lint", "go:golang.org/x/tools/gopls", "go:golang.org/x/vuln/cmd/govulncheck"}, cfg.Tools)
	assert.Empty(t, cfg.LockfilePlatforms)

	cfg, err = mise.ParseConfig([]byte("[settings]\nlockfile_platforms = [\"macos-arm64\", \"linux-x64\"]\n[tools]\nnode = { version = \"22\" }\njq = [\"1.7\"]\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"macos-arm64", "linux-x64"}, cfg.LockfilePlatforms)
	assert.Equal(t, []string{"node", "jq"}, cfg.Tools)
}

// TestParseConfigRefuses holds mise.toml to TOML: a duplicate key is
// refused, as in a lock.
func TestParseConfigRefuses(t *testing.T) {
	_, err := mise.ParseConfig([]byte("[tools]\ngo = \"1\"\ngo = \"2\"\n"))
	require.Error(t, err)
	_, err = mise.ParseConfig([]byte("[settings]\nlockfile_platforms = \"linux-x64\"\n"))
	require.Error(t, err, "lockfile_platforms is a list")
}

// TestParseConfigRefusesDeepNesting: a mise.toml of inline tables
// nested past the cap is refused before the decoder reads it, so a file
// of the size cap cannot exhaust memory (the audit of 2026-10-22).
func TestParseConfigRefusesDeepNesting(t *testing.T) {
	deep := "[tools]\nx = " + strings.Repeat("{a=", 5000) + "1" + strings.Repeat("}", 5000) + "\n"
	_, err := mise.ParseConfig([]byte(deep))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nesting")
	_, err = mise.ParseConfig([]byte("[tools]\nx = " + strings.Repeat("{a=", 20) + "1" + strings.Repeat("}", 20) + "\n"))
	require.NoError(t, err, "a depth under the cap is read")
}

// TestPlatforms is the list of 07 7.3: the two linux platforms, then the
// repo's own, each once.
func TestPlatforms(t *testing.T) {
	assert.Equal(t, []string{"linux-x64", "linux-arm64"}, mise.Platforms(mise.Config{}))
	assert.Equal(t, []string{"linux-x64", "linux-arm64", "macos-arm64"},
		mise.Platforms(mise.Config{LockfilePlatforms: []string{"macos-arm64", "linux-x64"}}))
}
