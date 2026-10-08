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
// file below holds a template that writes a marker when mise loads it,
// one of them reached through a .config symbolic link. "mise env"
// loads every one without miseEnv, and none with it; and each variable
// of miseEnv, left out alone, lets a file through again, so none of
// the four is there for nothing.
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
		"mise.toml":                        "[settings]\nlockfile = true\n",
		"mise.local.toml":                  envFile("local"),
		".config/mise/conf.d/x.toml":       envFile("confd"),
		".tool-versions":                   "tiny " + template("tool-versions") + "\n",
		".miserc.toml":                     "env = [\"ci\"]\nauto_env = true\n",
		"mise.ci.toml":                     envFile("ci"),
		"mise.unix.toml":                   envFile("unix"),
		"elsewhere/.mise/conf.d/link.toml": envFile("link"),
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	// .mise is a link to a directory under another name.
	require.NoError(t, os.Symlink(filepath.Join("elsewhere", ".mise"), filepath.Join(dir, ".mise")))
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

	assert.Equal(t, []string{"ci", "confd", "link", "local", "tool-versions", "unix"}, written(passThroughEnv()), "without miseEnv")
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

// TestIsMiseFile pins the paths mise reads from the top of the tree, in
// any case, and the ones it does not; and the paths it reads when it
// starts in a directory below.
func TestIsMiseFile(t *testing.T) {
	for _, path := range []string{"mise.toml", "mise.lock", ".mise.toml", "mise.local.toml", ".mise.local.toml",
		"mise.ci.toml", ".mise.ci.local.toml", "mise/config.toml", "mise/conf.d/x.toml", ".mise/config.toml",
		".config/mise.toml", ".config/mise.local.toml", ".config/mise/config.toml", ".config/mise/conf.d/x.toml",
		".tool-versions", ".miserc.toml", ".miserc.local.toml", ".config/miserc.toml",
		"Mise.local.toml", "MISE.TOML", ".TOOL-VERSIONS", ".Config/Mise/conf.d/X.toml", ".MiseRC.toml"} {
		assert.True(t, isMiseFile(path), path)
		assert.True(t, isMiseFileAtAnyDepth(path), path)
		assert.True(t, isMiseFileAtAnyDepth("tools/ci/"+path), "tools/ci/"+path)
	}
	for _, path := range []string{"README.md", "go.mod", "mise.tom", "mise-x.toml", "docs/promise.toml", "REUSE.toml",
		"tools/ci/misefiles.go", "x.tool-versions", ".config/other.toml", "miserc.toml"} {
		assert.False(t, isMiseFile(path), path)
		assert.False(t, isMiseFileAtAnyDepth(path), path)
	}
	for _, path := range []string{"tools/mise.local.toml", "internal/.tool-versions", "a/b/.config/mise/conf.d/x.toml"} {
		assert.False(t, isMiseFile(path), "%s: mise started at the top does not read it", path)
		assert.True(t, isMiseFileAtAnyDepth(path), path)
	}
}

// TestMiseFilesAskFirst holds the dependencies surface of the committed
// ask-first list to go.mod, go.sum, every glob of miseFiles and the
// workflows, in that order: a change to any file mise reads asks first.
func TestMiseFilesAskFirst(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(moduleRoot(t), askFirstPath))
	require.NoError(t, err)
	list, err := parseAskFirst(src)
	require.NoError(t, err)
	i := slices.IndexFunc(list.Surfaces, func(s surface) bool { return s.ID == "dependencies" })
	require.GreaterOrEqual(t, i, 0, "the dependencies surface")
	want := slices.Concat([]string{"go.mod", "go.sum"}, miseFiles, []string{".github/workflows/**"})
	assert.Equal(t, want, list.Surfaces[i].Globs)
	for _, g := range miseFiles {
		assert.NoError(t, checkGlob(g), g)
	}
}

// TestHostedMiseEnv checks the tripwire of setup: in the hosted
// workflow, each variable of miseEnv must be set to its value, an
// empty MISE_ENV included; elsewhere nothing is checked.
func TestHostedMiseEnv(t *testing.T) {
	setAll := func() {
		for _, kv := range miseEnv {
			key, value, _ := strings.Cut(kv, "=")
			t.Setenv(key, value)
		}
	}
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("MISE_ENV", "ci")
	require.NoError(t, hostedMiseEnv(), "off the hosted workflow nothing is checked")

	t.Setenv("GITHUB_ACTIONS", "true")
	setAll()
	require.NoError(t, hostedMiseEnv())

	t.Setenv("MISE_AUTO_ENV", "true")
	require.ErrorContains(t, hostedMiseEnv(), "this run has MISE_AUTO_ENV=true")

	setAll()
	require.NoError(t, os.Unsetenv("MISE_ENV"))
	require.ErrorContains(t, hostedMiseEnv(), "this run has MISE_ENV unset")
}
