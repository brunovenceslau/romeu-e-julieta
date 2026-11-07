// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// TestPreviewEnv holds the environment of the preview subprocess: the
// scrubbed one of REQ-0296 and the two paths the running romeu read
// (T196, provisional), XDG_STATE_HOME only when it is absolute.
func TestPreviewEnv(t *testing.T) {
	t.Parallel()
	env := map[string]string{"XDG_STATE_HOME": "/s", "XDG_CONFIG_HOME": "/c", "SSH_AUTH_SOCK": "/a", "PATH": "/opt/bin"}
	assert.Equal(t, []string{"HOME=/h", "PATH=/usr/bin:/bin", "LANG=C", "ROMEU_SETTINGS=/c/romeu/settings.yaml", "XDG_STATE_HOME=/s"},
		previewEnv("/h", "/c/romeu/settings.yaml", func(k string) string { return env[k] }))
	assert.Equal(t, []string{"HOME=/h", "PATH=/usr/bin:/bin", "LANG=C", "ROMEU_SETTINGS=/x"},
		previewEnv("/h", "/x", func(k string) string { return map[string]string{"XDG_STATE_HOME": "rel"}[k] }))
}

// TestRunPreview holds the preview step (REQ-1348): the new binary runs
// once, as romeu upgrade --preview <running version>, with the
// environment given and stdin closed, and its text, which ends with
// Question, is returned; a preview that fails, asks nothing or passes
// its cap is verify-failed, with its stderr, escaped, in the details.
func TestRunPreview(t *testing.T) {
	t.Parallel()
	write := func(t *testing.T, script string) (string, string) {
		t.Helper()
		dir := t.TempDir()
		p := filepath.Join(dir, ".romeu-upgrade-1")
		require.NoError(t, os.WriteFile(p, []byte(script), 0o755))
		return p, dir
	}
	t.Run("the text and the question", func(t *testing.T) {
		t.Parallel()
		p, home := write(t, newBinaryScript)
		text, err := runPreview(t.Context(), p, "v1.0.0-alpha.1", home, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "LANG=C"})
		require.NoError(t, err)
		assert.Equal(t, "the preview of the new romeu\n"+Question, text)
		argv, err := os.ReadFile(filepath.Join(home, "preview-argv"))
		require.NoError(t, err)
		assert.Equal(t, "upgrade --preview v1.0.0-alpha.1\n", string(argv))
		env, err := os.ReadFile(filepath.Join(home, "preview-env"))
		require.NoError(t, err)
		for _, want := range []string{"HOME=" + home + "\n", "LANG=C\n", "PATH=/usr/bin:/bin\n"} {
			assert.Contains(t, string(env), want)
		}
	})
	for _, tt := range []struct{ name, script string }{
		{"it fails", "#!/bin/sh\nprintf 'error RJ-343 sbx-floor: \\033[31mold\\n' >&2\nexit 2\n"},
		{"it asks nothing", "#!/bin/sh\necho text\n"},
		{"it passes its cap", "#!/bin/sh\nhead -c 1048577 /dev/zero\nprintf '" + Question + "'\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, home := write(t, tt.script)
			_, err := runPreview(t.Context(), p, "v1.0.0-alpha.1", home, []string{"HOME=" + home})
			requireID(t, err, VerifyFailed)
			for _, d := range cli.Of(err).Details {
				assert.NotContains(t, d.Value, "\x1b", "the stderr of the preview is escaped")
			}
		})
	}
}

// TestPreview holds the new binary's side of the preview (REQ-1348), run
// in this process as romeu upgrade --preview <running version>: the
// changelog sections after the running version up to its own, escaped,
// then what approve --toolchain prints, with Question in place of gate
// 1's question; it reads sbx version and writes nothing. It replaces the
// seams of the package, so it does not run in parallel.
func TestPreview(t *testing.T) {
	m := newMachine(t)
	m.version = "v1.0.0-alpha.2"
	_ = use(t, m, nil, 0, nil)
	old := changelogFile
	t.Cleanup(func() { changelogFile = old })
	changelogFile = func() string {
		return "# Changelog\n\n## v1.0.0-alpha.2\n\n- a \x1b[31mred\x1b[0m entry (#2)\n\n## v1.0.0-alpha.1\n\n- the first (#1)\n"
	}
	before := snapshot(t, m.home, m.stateHome, m.bin)

	got := runCommand(t, m, nil, "", "upgrade", "--preview", "v1.0.0-alpha.1")

	require.Equal(t, 0, got.code, got.stderr)
	assert.True(t, strings.HasPrefix(got.stdout, "## v1.0.0-alpha.2\n\n- a \\u001b[31mred\\u001b[0m entry (#2)\n\n"), got.stdout)
	assert.NotContains(t, got.stdout, "v1.0.0-alpha.1\n", "the running version's own section is not shown")
	assert.Contains(t, got.stdout, "gate 1: the toolchain of this machine\n")
	assert.True(t, strings.HasSuffix(got.stdout, Question), got.stdout)
	assert.Equal(t, 1, strings.Count(got.stdout, "[y/N]"), "one question")
	assert.Equal(t, before, snapshot(t, m.home, m.stateHome, m.bin), "the preview writes nothing")
}

// TestPreviewReadsTheRecord: with a toolchain acknowledged, the preview
// shows the change against it, as approve --toolchain would.
func TestPreviewReadsTheRecord(t *testing.T) {
	m := newMachine(t)
	_ = use(t, m, nil, 0, nil)
	st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	lk, err := st.Lock()
	require.NoError(t, err)
	live, err := approve.Measure(sbxdrv.Release{Major: 0, Minor: 51, Patch: 0}, cli.Version{Version: "v1.0.0-alpha.1", Protocol: 1})
	require.NoError(t, err)
	require.NoError(t, approve.Acknowledge(st, lk, live, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)))
	lk.Unlock()
	require.NoError(t, st.Close())

	got := runCommand(t, m, nil, "", "upgrade", "--preview", "v1.0.0-alpha.1")

	require.Equal(t, 0, got.code, got.stderr)
	assert.Contains(t, got.stdout, "julieta and the catalog are unchanged; "+Question, "one line when only romeu changed (REQ-0065)")
}
