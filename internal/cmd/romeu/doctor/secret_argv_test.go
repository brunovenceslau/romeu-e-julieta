// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// sentinel is the value a secret argv stub prints: it must appear in no
// output of doctor and in no file doctor could have written.
const sentinel = "SENTINEL-6f1c9e2a-secret-value"

// stub writes a secret argv stub that runs body under /bin/sh and
// returns its path.
func stub(t *testing.T, f *fixture, name, body string) string {
	t.Helper()
	p := filepath.Join(f.tmp, "bin", name)
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o700))
	return p
}

// TestDoctorNeverShowsASecretArgvOutput is the case "doctor never shows
// a secret argv's output" (I25): a secret argv that prints a sentinel on
// stdout and on stderr and exits 0, and one that prints it and exits 1.
// The first passes secret-argv and the second fails it, and the
// sentinel is in no stdout, stderr or --json output of doctor and in no
// file under HOME and the XDG directories after the run. The positive
// control runs the same stub directly and sees the sentinel.
func TestDoctorNeverShowsASecretArgvOutput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		exit string
		want string
	}{
		{"exit 0", "exit 0", resultPass},
		{"exit 1", "exit 1", resultFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			argv := stub(t, f, "secret", "echo "+sentinel+"; echo "+sentinel+" >&2; "+tc.exit)
			f.settings.Secrets = map[string]spec.Secret{"token@shop": {Service: "svc-alpha", Argv: []string{argv}}}

			control, _ := procgroup.CombinedOutput(t.Context(), procgroup.Command{Class: procgroup.Secret, Argv: []string{argv}})
			require.Contains(t, string(control), sentinel, "positive control: the stub prints the sentinel")

			rep, out, _ := f.output(t)

			assert.Equal(t, tc.want, check(t, rep, "secret-argv").Result, out)
			assert.NotContains(t, out, sentinel)
			for _, dir := range []string{f.home, f.configHome, f.stateHome} {
				assert.Empty(t, fileHolding(t, dir, sentinel), "a file under %s holds the sentinel", dir)
			}
		})
	}
}

// TestDoctorSecretArgv: a secret argv that prints nothing fails
// secret-argv, and so does one whose argv[0] is not there; each argv
// runs with exactly HOME and PATH=/usr/bin:/bin (03 3.4, REQ-0382).
func TestDoctorSecretArgv(t *testing.T) {
	t.Parallel()
	t.Run("prints nothing", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		argv := stub(t, f, "secret", "exit 0")
		f.settings.Secrets = map[string]spec.Secret{"token@shop": {Service: "svc-alpha", Argv: []string{argv}}}

		rep, out, err := f.output(t)

		c := check(t, rep, "secret-argv")
		assert.Equal(t, resultFail, c.Result, out)
		assert.Contains(t, c.Message, "printed nothing")
		assert.Equal(t, 6, exitOf(err))
	})
	t.Run("argv[0] is not there", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.settings.Secrets = map[string]spec.Secret{"token@shop": {Service: "svc-alpha", Argv: []string{filepath.Join(f.tmp, "bin", "absent")}}}

		rep, out, _ := f.output(t)

		assert.Equal(t, resultFail, check(t, rep, "secret-argv").Result, out)
	})
	t.Run("exactly HOME and PATH", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		envFile := filepath.Join(f.tmp, "env")
		argv := stub(t, f, "secret", "/usr/bin/env > "+envFile+"; echo value")
		f.settings.Secrets = map[string]spec.Secret{"token@shop": {Service: "svc-alpha", Argv: []string{argv}}}
		f.env["SECRET_LEAK"] = "x"

		rep, out, err := f.output(t)

		require.NoError(t, err, out)
		assert.Equal(t, resultPass, check(t, rep, "secret-argv").Result)
		b, err := os.ReadFile(envFile)
		require.NoError(t, err)
		var names []string
		for kv := range strings.SplitSeq(strings.TrimSpace(string(b)), "\n") {
			name, value, _ := strings.Cut(kv, "=")
			switch name {
			case "PWD", "SHLVL", "OLDPWD", "_":
				continue // what the shell of the stub sets itself
			case "HOME":
				assert.Equal(t, f.home, value)
			case "PATH":
				assert.Equal(t, "/usr/bin:/bin", value)
			}
			names = append(names, name)
		}
		slices.Sort(names)
		assert.Equal(t, []string{"HOME", "PATH"}, names)
	})
}

// TestDoctorRunsNoSecretArgvFromForeignSettings: settings that are not a
// regular file of mode 0600 owned by this user run no secret argv; the
// check is an error that names settings-mode.
func TestDoctorRunsNoSecretArgvFromForeignSettings(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ran := filepath.Join(f.tmp, "ran")
	argv := stub(t, f, "secret", "/usr/bin/touch "+ran+"; echo value")
	f.settings.Secrets = map[string]spec.Secret{"token@shop": {Service: "svc-alpha", Argv: []string{argv}}}
	f.settingsMode = 0o640

	rep, out, err := f.output(t)

	c := check(t, rep, "secret-argv")
	assert.Equal(t, resultError, c.Result, out)
	assert.Contains(t, c.Message, "settings-mode")
	assert.Equal(t, resultFail, check(t, rep, "settings-mode").Result)
	assert.Equal(t, 1, exitOf(err))
	assert.NoFileExists(t, ran, "no secret argv ran")
}
