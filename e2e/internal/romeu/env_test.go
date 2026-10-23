// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// TestEnvRunsRomeuInItsOwnTree runs the built binary with the Env's
// environment: its version command answers, and a variable of the test
// process does not reach it.
func TestEnvRunsRomeuInItsOwnTree(t *testing.T) {
	t.Setenv("ROMEU_SETTINGS", "/elsewhere/settings.yaml")
	e := romeu.New(t, nil)

	res := e.Run(t, []string{"version"})

	assert.Equal(t, 0, res.Code, res.Stderr)
	assert.Contains(t, res.Stdout, "protocol")
	assert.NotContains(t, e.Environ(), "ROMEU_SETTINGS=/elsewhere/settings.yaml")
	assert.Equal(t, filepath.Join(e.ConfigHome, "romeu", "settings.yaml"), e.Settings())
}

// TestEnvExitCode returns the exit status of a refused command.
func TestEnvExitCode(t *testing.T) {
	e := romeu.New(t, nil)

	res := e.Run(t, []string{"no-such-command"})

	assert.Equal(t, 2, res.Code)
	assert.Contains(t, res.Stderr, "usage")
}

// TestEnvWriteSettings fills the Root and the tools, writes mode 0600,
// and reads back what romeu would.
func TestEnvWriteSettings(t *testing.T) {
	e := romeu.New(t, nil)

	e.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/o/cfg", Ref: "main"}})

	fi, err := os.Stat(e.Settings())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	h := e.ReadSettings(t)
	assert.Equal(t, e.Root, h.Root)
	assert.Equal(t, e.Sbx, h.Tools.Sbx)
	assert.Equal(t, e.Git, h.Tools.Git)
}

// TestEnvFakeSbxIsOnPath: the sbx of the settings is the fake, which
// refuses a call its empty session does not record.
func TestEnvFakeSbxIsOnPath(t *testing.T) {
	e := romeu.New(t, nil)

	assert.Equal(t, filepath.Join(e.Bin, "sbx"), e.Sbx)
	assert.FileExists(t, e.Sbx+".json")
	assert.NoError(t, e.SetSbxSession(filepath.Join(e.Bin, "other.jsonl")))
}

// TestOriginsServeARepoOverTLS clones what Push committed through the
// git of an Env, the way romeu init does: the URL has no port, and the
// CA is the server's.
func TestOriginsServeARepoOverTLS(t *testing.T) {
	o := romeu.NewOrigins(t)
	o.Push(t, "owner/cfg", "main", map[string]string{"projects/cfg.yaml": "x: 1\n"})
	e := romeu.New(t, o)
	dest := filepath.Join(t.TempDir(), "clone")

	out := runGit(t, e, filepath.Dir(dest), "clone", "-q", o.URL("owner/cfg"), dest)

	assert.Empty(t, out)
	got, err := os.ReadFile(filepath.Join(dest, "projects", "cfg.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "x: 1\n", string(got))
	assert.True(t, strings.HasPrefix(o.URL("owner/cfg"), "https://github.com/"))
	assert.FileExists(t, o.CAFile())
}

// TestOriginsPushAddsACommit: a second Push to the same repository is a
// new commit on the branch, as an agent's replayed commit is.
func TestOriginsPushAddsACommit(t *testing.T) {
	o := romeu.NewOrigins(t)
	o.Push(t, "owner/repo", "main", map[string]string{"a": "1"})
	o.Push(t, "owner/repo", "main", map[string]string{"b": "2"})
	e := romeu.New(t, o)
	dest := filepath.Join(t.TempDir(), "clone")

	runGit(t, e, filepath.Dir(dest), "clone", "-q", o.URL("owner/repo"), dest)

	assert.Equal(t, "2\n", o.Git(t, dest, "rev-list", "--count", "HEAD"))
}

// runGit runs the git of the Env, a script or a link, in dir, and
// returns its output.
func runGit(t *testing.T, e *romeu.Env, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), e.Git, args...)
	cmd.Dir = dir
	cmd.Env = e.Environ("GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}
