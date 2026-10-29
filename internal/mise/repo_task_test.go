// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestTrustRuns proves the call of mise trust: the mise.toml of the repo
// as its argument, and an environment of install without
// MISE_TRUSTED_CONFIG_PATHS and MISE_PARANOID (04 4.3, REQ-1306).
func TestTrustRuns(t *testing.T) {
	repo := t.TempDir()
	rec := filepath.Join(t.TempDir(), "rec")
	d := fakeMise(t, `{ echo "argv: $@"; env | sort; } > '`+rec+`'`)
	require.NoError(t, d.Trust(t.Context(), mise.Target{Dir: repo, Repo: "api"}))
	b, err := os.ReadFile(rec)
	require.NoError(t, err)
	got := string(b)
	assert.Contains(t, got, "argv: trust "+filepath.Join(repo, "mise.toml")+"\n")
	assert.Contains(t, got, "MISE_YES=1\n")
	assert.Contains(t, got, "MISE_STATE_DIR=/h/.local/state/mise\n")
	assert.NotContains(t, got, "MISE_PARANOID")
	assert.NotContains(t, got, "MISE_TRUSTED_CONFIG_PATHS")
	assert.Contains(t, d.Env, "MISE_PARANOID=1", "the driver's own environment is not changed")
}

// TestTrustFailure: a mise trust that fails is mise-failed, and one that
// hangs is subprocess-timeout.
func TestTrustFailure(t *testing.T) {
	d := fakeMise(t, `echo denied >&2; exit 1`)
	err := d.Trust(t.Context(), mise.Target{Dir: t.TempDir(), Repo: "api"})
	require.Error(t, err)
	assert.Equal(t, "RJ-351", cli.Of(err).ID)
	assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "stderr", Value: "denied"})

	d = fakeMise(t, `sleep 30`)
	ctx := procgroup.WithTestTimeout(t.Context(), procgroup.Mise, 300*time.Millisecond)
	err = d.Trust(ctx, mise.Target{Dir: t.TempDir(), Repo: "api"})
	require.Error(t, err)
	assert.Equal(t, "RJ-323", cli.Of(err).ID)
}

// TestHasTask: the list of tasks names setup defined by the mise.toml of
// the repo or not; a task a file defines, another repo's and a name that
// is not setup are not it; no output is no task; a list that does not
// parse and a mise that fails are errors.
func TestHasTask(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		want, fails  bool
	}{
		{"listed", `echo '[{"name":"build","source":"'"$PWD"'/mise.toml"},{"name":"setup","aliases":[],"source":"'"$PWD"'/mise.toml"}]'`, true, false},
		{"not listed", `echo '[{"name":"build","source":"'"$PWD"'/mise.toml"}]'`, false, false},
		{"defined by a file", `echo '[{"name":"setup","source":"'"$PWD"'/mise-tasks/setup"}]'`, false, false},
		{"defined elsewhere", `echo '[{"name":"setup","source":"/other/mise.toml"}]'`, false, false},
		{"no source", `echo '[{"name":"setup"}]'`, false, false},
		{"empty list", `echo '[]'`, false, false},
		{"no output", `:`, false, false},
		{"not json", `echo nope`, false, true},
		{"fails", `exit 2`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fakeMise(t, tc.script)
			repo := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(repo, "mise.toml"), nil, 0o600))
			got, err := d.HasTask(t.Context(), mise.Target{Dir: repo, Repo: "api"}, "setup")
			assert.Equal(t, tc.fails, err != nil, "%v", err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestRunTask: mise run <name> runs in the repo, and its exit status is
// the error.
func TestRunTask(t *testing.T) {
	repo := t.TempDir()
	rec := filepath.Join(t.TempDir(), "rec")
	d := fakeMise(t, `{ echo "$@"; env | sort; } > '`+rec+`'; exit 3`)
	err := d.RunTask(t.Context(), mise.Target{Dir: repo, Repo: "api"}, "setup")
	exit, ok := errors.AsType[*procgroup.ExitError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, 3, exit.Code)
	b, readErr := os.ReadFile(rec)
	require.NoError(t, readErr)
	assert.Contains(t, string(b), "run setup\n")
	assert.Contains(t, string(b), "MISE_PARANOID=1\n", "the task runs with the environment of install")
	assert.Contains(t, string(b), "MISE_TRUSTED_CONFIG_PATHS=/w/api\n")
}
