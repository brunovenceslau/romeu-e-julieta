// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package version_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/version"
)

// stamp is the version stamp the tests give the binary.
var stamp = cli.Version{Version: "v1.2.3", Commit: "abc1234", Date: "2026-10-18T00:00:00Z", Protocol: 7}

// run runs romeu with args under ctx and returns the exit status, stdout
// and stderr.
func run(ctx context.Context, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), Version: stamp}
	code := cli.Main(ctx, args, env)
	return code, stdout.String(), stderr.String()
}

// TestVersionHelp is the help test of the command (12 12.3): its usage,
// its line and its examples, on --help and through help.
func TestVersionHelp(t *testing.T) {
	for _, args := range [][]string{{"version", "--help"}, {"help", "version"}} {
		code, stdout, stderr := run(t.Context(), args...)
		assert.Equal(t, 0, code, "%v", args)
		assert.Empty(t, stderr, "%v", args)
		assert.True(t, strings.HasPrefix(stdout, "Usage: romeu version [flags]\n\nprint the version of romeu\n"), "%v:\n%s", args, stdout)
		assert.Contains(t, stdout, "      --json   print one JSON document\n", "%v", args)
		assert.Contains(t, stdout, "Examples:\n  romeu version\n  romeu version --json\n", "%v", args)
	}
}

// TestVersionText prints the version, commit, build date, Go version
// and protocol (04 4.1, Version).
func TestVersionText(t *testing.T) {
	code, stdout, stderr := run(t.Context(), "version")
	assert.Equal(t, 0, code)
	assert.Empty(t, stderr)
	assert.Equal(t, "version   v1.2.3\ncommit    abc1234\ndate      2026-10-18T00:00:00Z\ngo        "+runtime.Version()+"\nprotocol  7\n", stdout)
}

// TestVersionJSON prints one JSON document that adds the sha256 of the
// running binary, its GOARCH and the versions object to the fields of
// the text. With no host settings and no host state, the versions object
// lists the settings as a failed read, and the command still exits 0
// (04 4.2).
func TestVersionJSON(t *testing.T) {
	t.Setenv("ROMEU_SETTINGS", filepath.Join(t.TempDir(), "none.yaml"))
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	code, stdout, stderr := run(t.Context(), "version", "--json")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
	assert.Equal(t, 1, strings.Count(stdout, "\n"), "one line")
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	exe, err := os.Executable()
	require.NoError(t, err)
	bin, err := os.ReadFile(exe)
	require.NoError(t, err)
	sum := sha256.Sum256(bin)
	assert.Equal(t, "v1.2.3", doc["version"])
	assert.Equal(t, "abc1234", doc["commit"])
	assert.Equal(t, "2026-10-18T00:00:00Z", doc["date"])
	assert.Equal(t, runtime.Version(), doc["go"])
	assert.InDelta(t, 7, doc["protocol"], 0)
	assert.Equal(t, hex.EncodeToString(sum[:]), doc["sha256"])
	assert.Equal(t, runtime.GOARCH, doc["goarch"])
	assert.Len(t, doc, 8, "the fields of 04 4.1")
	versions, ok := doc["versions"].(map[string]any)
	require.True(t, ok, "%v", doc["versions"])
	assert.Equal(t, []any{}, versions["projects"])
	errs, ok := versions["errors"].([]any)
	require.True(t, ok)
	require.Len(t, errs, 1)
	assert.Equal(t, "RJ-349", errs[0].(map[string]any)["id"])
	assert.Equal(t, "host settings", errs[0].(map[string]any)["read"])
	assert.NotContains(t, versions, "sbx", "no settings, so sbx was not run")
	assert.NotContains(t, versions, "git", "no settings, so git was not run")
	if runtime.GOOS != "darwin" {
		assert.NotContains(t, versions, "macos")
	}
}

// TestVersionInterrupted returns the cancellation of its context, so
// cli.Main prints nothing and procgroup.Main maps the status to 130 or
// 143 (04 4.1, Signals).
func TestVersionInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, args := range [][]string{{"version"}, {"version", "--json"}} {
		code, stdout, stderr := run(ctx, args...)
		assert.Equal(t, 1, code, "%v", args)
		assert.Empty(t, stdout, "%v", args)
		assert.Empty(t, stderr, "%v", args)
	}
}
