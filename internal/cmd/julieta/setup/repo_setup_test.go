// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// taskMise is a fake mise that lists the task setup and runs body for
// "mise run setup"; "mise trust" records its environment to envFile.
func taskMise(t *testing.T, body string) (calls, envFile string) {
	t.Helper()
	envFile = filepath.Join(t.TempDir(), "trust-env")
	calls = fakeMise(t, `case "$1" in
tasks) echo '[{"name":"build"},{"name":"setup"}]';;
trust) env > '`+envFile+`';;
run) `+body+`;;
esac`)
	return calls, envFile
}

// warningsOf returns the messages of the warnings of slug.
func warningsOf(rep setup.Report, slug string) []string {
	var got []string
	for _, w := range rep.Warnings {
		if w.Slug == slug {
			got = append(got, w.Message)
		}
	}
	return got
}

// TestSetupTrustsThenInstallsThenRunsTheTask: mise trust of the
// mise.toml comes before install, and mise run setup after it
// (04 4.3, REQ-1306, REQ-1307); the trust runs without
// MISE_TRUSTED_CONFIG_PATHS and MISE_PARANOID, as the agent's shell has
// it, and with the mise state below the home of the sandbox.
func TestSetupTrustsThenInstallsThenRunsTheTask(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	marker := filepath.Join(t.TempDir(), "marker")
	calls, envFile := taskMise(t, "pwd > '"+marker+"'")

	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Errors)
	assert.Empty(t, warningsOf(rep, setup.RepoSetupFailed))

	got, err := os.ReadFile(calls)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"trust " + filepath.Join(s.primary(), "mise.toml"),
		"install",
		"tasks ls --json",
		"run setup",
	}, strings.Split(strings.TrimSpace(string(got)), "\n"))

	pwd, err := os.ReadFile(marker)
	require.NoError(t, err, "the task of the repo ran")
	assert.Equal(t, s.primary(), strings.TrimSpace(string(pwd)), "in the repo")

	env, err := os.ReadFile(envFile)
	require.NoError(t, err)
	assert.NotContains(t, string(env), "MISE_PARANOID=")
	assert.NotContains(t, string(env), "MISE_TRUSTED_CONFIG_PATHS=")
	assert.Contains(t, string(env), "MISE_STATE_DIR="+filepath.Join(s.home, ".local", "state", "mise")+"\n")
}

// TestSetupRepoTaskFailure: a task that exits 3 is the warning
// repo-setup-failed naming the repo and "exit 3", and setup exits 0.
func TestSetupRepoTaskFailure(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	taskMise(t, "exit 3")

	code, rep, stderr := setupJSON(t)
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Errors)
	require.Equal(t, []string{"api exit 3"}, warningsOf(rep, setup.RepoSetupFailed))
	assert.Contains(t, stderr, "warning repo-setup-failed: api exit 3\n")
}

// TestSetupRepoTaskTimeout: a task past the budget of the class mise
// (a test seam shortens it) is the same warning, with the timeout
// named, and setup exits 0.
func TestSetupRepoTaskTimeout(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	taskMise(t, "sleep 30")

	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Mise, 500*time.Millisecond)
	code, stdout, stderr := julieta(ctx, "setup", "--json")
	require.Equal(t, 0, code, stderr)
	rep, err := setup.Decode([]byte(stdout))
	require.NoError(t, err)
	got := warningsOf(rep, setup.RepoSetupFailed)
	require.Len(t, got, 1, stderr)
	assert.Contains(t, got[0], "api ran past its limit of 500ms")
	assert.Contains(t, got[0], "timeout")
}

// TestSetupRepoWithoutTheTask: a repo whose tasks do not include setup
// is skipped silently, and mise run is never started.
func TestSetupRepoWithoutTheTask(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	calls := fakeMise(t, `[ "$1" = tasks ] && echo '[{"name":"build"}]'; exit 0`)

	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, warningsOf(rep, setup.RepoSetupFailed), stderr)
	assert.Empty(t, rep.Errors)
	b, err := os.ReadFile(calls)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "run setup")
}

// TestSetupRepoWithoutMiseConfig: a repo with no mise.toml is neither
// trusted nor asked for tasks, and warns nothing.
func TestSetupRepoWithoutMiseConfig(t *testing.T) {
	newSandbox(t)
	calls, _ := taskMise(t, "exit 3")

	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Warnings, stderr)
	_, err := os.Stat(calls)
	assert.ErrorIs(t, err, os.ErrNotExist, "no mise was started")
}

// TestSetupTrustFailure: a mise trust that fails is reported and leaves
// the sandbox usable, and install still runs.
func TestSetupTrustFailure(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	calls := fakeMise(t, `[ "$1" = trust ] && { echo denied >&2; exit 1; }; [ "$1" = tasks ] && echo '[]'; exit 0`)

	code, rep, stderr := setupJSON(t)
	assert.Equal(t, 0, code, stderr)
	require.Len(t, rep.Errors, 1)
	assert.True(t, strings.HasPrefix(rep.Errors[0], "trust: "), rep.Errors[0])
	assert.Contains(t, rep.Errors[0], "mise trust failed in api")
	assert.Contains(t, rep.Errors[0], "denied")
	assert.Equal(t, 1, count(t, calls))
}
