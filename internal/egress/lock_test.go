// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
)

// fixture returns the bytes of a file of testdata.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}

// TestParseLock reads the lock of this repository, written by the pinned
// mise (07 7.5, step 1): each tool with the backend, the version and the
// URL of each platform it records, and none for a version-only tool.
func TestParseLock(t *testing.T) {
	lock, err := egress.ParseLock(fixture(t, "valid.lock"))
	require.NoError(t, err)
	require.Len(t, lock.Tools["gh"], 1)
	gh := lock.Tools["gh"][0]
	assert.Equal(t, "aqua:cli/cli", gh.Backend)
	assert.Equal(t, "2.102.0", gh.Version)
	assert.Equal(t, []string{"linux-arm64", "linux-x64", "macos-arm64", "macos-x64"}, gh.PlatformNames())
	assert.Equal(t, "https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_linux_amd64.tar.gz", gh.URLs["linux-x64"])
	assert.Equal(t, "https://dl.google.com/go/go1.27.3.linux-arm64.tar.gz", lock.Tools["go"][0].URLs["linux-arm64"])
	assert.Equal(t, "core:go", lock.Tools["go"][0].Backend)
	versionOnly := lock.Tools["go:golang.org/x/vuln/cmd/govulncheck"][0]
	assert.Equal(t, "go:golang.org/x/vuln/cmd/govulncheck", versionOnly.Backend)
	assert.Empty(t, versionOnly.URLs)
}

// TestParseLockRefuses holds the reader to the layout of the pinned
// mise: a duplicate key or table, a top-level key it does not have, text
// that is not TOML, a file over the cap and a member of the wrong type
// are each refused, never read in part (REQ-0629).
func TestParseLockRefuses(t *testing.T) {
	const head = "lockfile_version = 3\n"
	entry := func(body string) []byte {
		return []byte(head + "[[tools.gh]]\nversion = \"1\"\nbackend = \"aqua:cli/cli\"\n" + body)
	}
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"a duplicate key", entry("version = \"2\"\n"), "already been defined"},
		{"a duplicate table", entry("[tools.gh.\"platforms.linux-x64\"]\nurl = \"https://a.example/x\"\n[tools.gh.\"platforms.linux-x64\"]\nurl = \"https://b.example/x\"\n"), "already been defined"},
		{"an unknown top-level key", []byte(head + "surprise = 1\n"), "surprise"},
		{"an unknown top-level table", []byte(head + "[surprise]\nx = 1\n"), "surprise"},
		{"not toml", []byte("lockfile_version = \n"), "mise.lock"},
		{"over the cap", []byte("# " + strings.Repeat("x", egress.MaxFileSize) + "\n"), "cap"},
		{"one byte over the cap", lockOfSize(egress.MaxFileSize + 1), "cap"},
		{"an unknown layout version", []byte("lockfile_version = 4\n"), "lockfile_version"},
		{"no layout version", []byte("[[tools.go]]\nversion = \"1\"\n"), "lockfile_version"},
		{"nesting past the cap", []byte(head + "x = " + strings.Repeat("[", 64) + strings.Repeat("]", 64) + "\n"), "nesting"},
		{"a dotted key of too many parts", []byte(head + strings.Repeat("a.", 10000) + "b = 1\n"), "key parts"},
		{"a version that is not a string", []byte(head + "[[tools.gh]]\nversion = 1\nbackend = \"aqua:cli/cli\"\n"), "version is not a string"},
		{"an entry without a backend", []byte(head + "[[tools.gh]]\nversion = \"1\"\n"), "no backend"},
		{"a platform that is not a table", entry("\"platforms.linux-x64\" = \"x\"\n"), "not a table"},
		{"a url on the entry", entry("url = \"https://a.example/x\"\n"), "does not have"},
		{"a table named platforms", entry("[tools.gh.platforms.linux-x64]\nurl = \"https://a.example/x\"\n"), "does not have"},
		{"a url that is not a string", entry("[tools.gh.\"platforms.linux-x64\"]\nurl = 3\n"), "url is not a string"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, " ", "_"), func(t *testing.T) {
			lock, err := egress.ParseLock(tt.data)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Empty(t, lock.Tools, "no partial result")
		})
	}
}

// TestParseLockAtTheCap: a lock of exactly MaxFileSize bytes is read; one
// byte more is refused (TestParseLockRefuses).
func TestParseLockAtTheCap(t *testing.T) {
	data := lockOfSize(egress.MaxFileSize)
	require.Len(t, data, egress.MaxFileSize)
	_, err := egress.ParseLock(data)
	require.NoError(t, err)
}

// lockOfSize is a valid empty lock padded with a comment to n bytes.
func lockOfSize(n int) []byte {
	const head = "lockfile_version = 3\n"
	return []byte("#" + strings.Repeat("x", n-len(head)-2) + "\n" + head)
}

// TestFixtureIsTheMiseOne: 10 10.7 forbids egress to import mise, so
// the two keep a reader each, and this holds them to one fixture of the
// pinned layout: a pin bump that changes the lock of internal/mise
// fails here until the copy follows.
func TestFixtureIsTheMiseOne(t *testing.T) {
	theirs, err := os.ReadFile(filepath.Join("..", "mise", "testdata", "valid.lock"))
	require.NoError(t, err)
	assert.Equal(t, string(theirs), string(fixture(t, "valid.lock")))
}
