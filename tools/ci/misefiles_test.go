// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMiseEnvList pins miseEnv, and holds the lists of variables a step
// keeps from this process to none of its keys, so the value miseEnviron
// appends is the only one a mise run gets.
func TestMiseEnvList(t *testing.T) {
	assert.Equal(t, []string{
		"MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml",
		"MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=none",
		"MISE_ENV=",
		"MISE_AUTO_ENV=false",
	}, miseEnv)
	for _, kv := range miseEnv {
		key, _, _ := strings.Cut(kv, "=")
		for _, list := range [][]string{passThrough, networkPassThrough, dockerPassThrough} {
			assert.NotContains(t, list, key)
		}
	}
	base := []string{"PATH=/x"}
	assert.Equal(t, append([]string{"PATH=/x"}, miseEnv...), miseEnviron(base))
	assert.Equal(t, []string{"PATH=/x"}, base, "the base is not changed")
}

// TestMiseEnvStopsConfigs measures, with the mise on the search path,
// what miseEnv is for: in a directory whose mise.toml is trusted, each
// file below holds a template that writes a marker when mise loads it.
// "mise env" loads every one without miseEnv, and none with it; and
// each variable of miseEnv, left out alone, lets a file through again,
// so none of the four is there for nothing.
func TestMiseEnvStopsConfigs(t *testing.T) {
	dir, markers := t.TempDir(), t.TempDir()
	t.Setenv("MISE_STATE_DIR", t.TempDir())
	template := func(name string) string {
		return `{{ exec(command="touch ` + filepath.ToSlash(filepath.Join(markers, name)) + `") }}`
	}
	envFile := func(name string) string {
		return "[env]\nX_" + strings.ToUpper(name) + " = " + strconv.Quote(template(name)) + "\n"
	}
	files := map[string]string{
		"mise.toml":                  "[settings]\nlockfile = true\n",
		"mise.local.toml":            envFile("local"),
		".config/mise/conf.d/x.toml": envFile("confd"),
		".tool-versions":             "tiny " + template("tool-versions") + "\n",
		".miserc.toml":               "env = [\"ci\"]\nauto_env = true\n",
		"mise.ci.toml":               envFile("ci"),
		"mise.unix.toml":             envFile("unix"),
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	mise := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "mise", append([]string{"-C", dir}, args...)...)
		cmd.Dir = dir
		// MISE_OFFLINE keeps the test off the network: no file here
		// names a tool mise could fetch, and none is wanted.
		cmd.Env = append(slices.Clone(env), "MISE_OFFLINE=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "mise %v\n%s", args, out)
	}
	written := func(env []string) []string {
		t.Helper()
		entries, err := os.ReadDir(markers)
		require.NoError(t, err)
		for _, e := range entries {
			require.NoError(t, os.Remove(filepath.Join(markers, e.Name())))
		}
		mise(env, "env")
		entries, err = os.ReadDir(markers)
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
	mise(miseEnviron(passThroughEnv()), "trust", filepath.Join(dir, "mise.toml"))

	assert.Equal(t, []string{"ci", "confd", "local", "tool-versions", "unix"}, written(passThroughEnv()), "without miseEnv")
	assert.Empty(t, written(miseEnviron(passThroughEnv())), "with miseEnv")
	for _, kv := range miseEnv {
		key, _, _ := strings.Cut(kv, "=")
		without := slices.DeleteFunc(miseEnviron(passThroughEnv()), func(v string) bool { return v == kv })
		assert.NotEmpty(t, written(without), "miseEnv without %s", key)
	}
}

// TestMiseFilesLowercase holds every glob of miseFiles to lower case:
// isMiseFile matches the path in lower case, so a glob with a capital
// letter would match nothing.
func TestMiseFilesLowercase(t *testing.T) {
	for _, g := range miseFiles {
		assert.Equal(t, strings.ToLower(g), g)
	}
}
