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
		"MISE_GLOBAL_CONFIG_FILE=/dev/null/mise-global.toml",
		"MISE_ENV_FILE=",
		"MISE_CD=",
		"MISE_TRUSTED_CONFIG_PATHS=",
		"GOENV=off",
		"GOTOOLCHAIN=local",
	}, miseEnv)
	assert.Equal(t, []string{"GOENV=off", "GOTOOLCHAIN=local"}, goPinEnv)
	assert.Subset(t, stepEnv("/go"), goPinEnv, "stepEnv holds goPinEnv")
	for _, kv := range miseEnv {
		key, _, _ := strings.Cut(kv, "=")
		for _, list := range [][]string{passThrough, networkPassThrough, dockerPassThrough} {
			assert.NotContains(t, list, key)
		}
	}
	// A value of the caller's for a variable of miseEnv loses to the list's.
	cmd := exec.CommandContext(t.Context(), "env")
	cmd.Env = miseEnviron([]string{"GOENV=/hostile", "GOTOOLCHAIN=go1.99.0"})
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "GOENV=off\n")
	assert.Contains(t, string(out), "GOTOOLCHAIN=local\n")
	base := []string{"PATH=/x"}
	assert.Equal(t, append([]string{"PATH=/x"}, miseEnv...), miseEnviron(base))
	assert.Equal(t, []string{"PATH=/x"}, base, "the base is not changed")
}

// isolatedMiseEnv is an environment for the real mise built from
// nothing but PATH: its home, configuration, state and cache
// directories are new and empty, the system and global configuration
// files do not exist, and it stays off the network. So no
// configuration of the person who runs the tests (a global
// "paranoid = true", say) changes what a test measures. A test that
// needs the pinned tools below HOME uses isolateMiseConfig instead.
func isolatedMiseEnv(t *testing.T) []string {
	t.Helper()
	none := filepath.Join(t.TempDir(), "none.toml")
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"XDG_CONFIG_HOME=" + t.TempDir(),
		"MISE_CONFIG_DIR=" + t.TempDir(),
		"MISE_STATE_DIR=" + t.TempDir(),
		"MISE_CACHE_DIR=" + t.TempDir(),
		"MISE_SYSTEM_CONFIG_FILE=" + none,
		"MISE_GLOBAL_CONFIG_FILE=" + none,
		"MISE_OFFLINE=1",
	}
}

// TestMiseEnvStopsConfigs measures, with the mise on the search path,
// what miseEnv is for: in a directory whose mise.toml is trusted, each
// file below holds a template that writes a marker when mise loads it,
// one of them reached through a .config symbolic link. "mise env"
// loads every one without miseEnv, and none with it; and each variable
// of miseEnv, left out alone, lets a file through again, so none of
// the four that name a file of the tree is there for nothing. mise
// runs in isolatedMiseEnv.
func TestMiseEnvStopsConfigs(t *testing.T) {
	dir, markers := t.TempDir(), t.TempDir()
	base := isolatedMiseEnv(t)
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
		cmd.Env = env
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
	mise(miseEnviron(base), "trust", filepath.Join(dir, "mise.toml"))

	assert.Equal(t, []string{"ci", "confd", "link", "local", "tool-versions", "unix"}, written(base), "without miseEnv")
	assert.Empty(t, written(miseEnviron(base)), "with miseEnv")
	for _, kv := range miseEnv {
		key, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(key, "MISE_") {
			continue // a go variable, which mise does not read
		}
		if key == "MISE_GLOBAL_CONFIG_FILE" || key == "MISE_ENV_FILE" || key == "MISE_CD" || key == "MISE_TRUSTED_CONFIG_PATHS" {
			continue // TestMiseGlobalConfigIsOff, TestMiseEnvFileIsOff and TestMiseCdAndTrustAreOff measure them
		}
		without := slices.DeleteFunc(miseEnviron(base), func(v string) bool { return v == kv })
		assert.NotEmpty(t, written(without), "miseEnv without %s", key)
	}
}

// TestMiseGlobalConfigIsOff measures, with the mise on the search path,
// what MISE_GLOBAL_CONFIG_FILE=miseNoGlobalConfig is for: the global
// configuration of mise holds an [env] table (a GOFLAGS there would
// reach the go that the mise shim starts), and mise reads it from the
// default directory under HOME, from the directory MISE_CONFIG_DIR
// names, with the conf.d of each, and from the file the caller names.
// Each holds a template that writes a marker when mise loads it. All
// load without the variable, and none with miseEnv, whatever the
// caller sets. mise runs in isolatedMiseEnv, less its own directories
// and global file.
func TestMiseGlobalConfigIsOff(t *testing.T) {
	dir, markers := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[settings]\nlockfile = true\n"), 0o600))
	envFile := func(name string) string {
		cmd := `{{ exec(command="touch ` + filepath.ToSlash(filepath.Join(markers, name)) + `") }}`
		return "[env]\nX_" + strings.ToUpper(name) + " = " + strconv.Quote(cmd) + "\n"
	}
	write := func(path, name string) string {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(envFile(name)), 0o600))
		return path
	}
	home, cfg, named := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "named.toml")
	write(filepath.Join(home, ".config", "mise", "config.toml"), "home")
	write(filepath.Join(home, ".config", "mise", "conf.d", "x.toml"), "homeconfd")
	write(filepath.Join(cfg, "config.toml"), "cfg")
	write(filepath.Join(cfg, "conf.d", "y.toml"), "cfgconfd")
	write(named, "named")
	base := slices.DeleteFunc(isolatedMiseEnv(t), func(kv string) bool {
		return strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(kv, "MISE_CONFIG_DIR=") || strings.HasPrefix(kv, "MISE_GLOBAL_CONFIG_FILE=")
	})
	scenarios := []struct {
		name  string
		env   []string
		marks []string
	}{
		{"the default directory", []string{"HOME=" + home}, []string{"home", "homeconfd"}},
		{"MISE_CONFIG_DIR", []string{"HOME=" + t.TempDir(), "MISE_CONFIG_DIR=" + cfg}, []string{"cfg", "cfgconfd"}},
		{"MISE_GLOBAL_CONFIG_FILE", []string{"HOME=" + t.TempDir(), "MISE_GLOBAL_CONFIG_FILE=" + named}, []string{"named"}},
	}
	written := func(env []string) []string {
		t.Helper()
		entries, err := os.ReadDir(markers)
		require.NoError(t, err)
		for _, e := range entries {
			require.NoError(t, os.Remove(filepath.Join(markers, e.Name())))
		}
		cmd := exec.CommandContext(t.Context(), "mise", "-C", dir, "env")
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "mise env\n%s", out)
		entries, err = os.ReadDir(markers)
		require.NoError(t, err)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		return names
	}
	trust := exec.CommandContext(t.Context(), "mise", "-C", dir, "trust", filepath.Join(dir, "mise.toml"))
	trust.Dir = dir
	trust.Env = miseEnviron(slices.Concat(base, scenarios[0].env))
	out, err := trust.CombinedOutput()
	require.NoError(t, err, "mise trust\n%s", out)
	for _, sc := range scenarios {
		env := slices.Concat(base, sc.env)
		without := slices.DeleteFunc(miseEnviron(env), func(kv string) bool {
			return strings.HasPrefix(kv, "MISE_GLOBAL_CONFIG_FILE=") && strings.HasSuffix(kv, miseNoGlobalConfig)
		})
		assert.ElementsMatch(t, sc.marks, written(without), "%s, without the variable", sc.name)
		assert.Empty(t, written(miseEnviron(env)), "%s, with miseEnv", sc.name)
	}
}

// TestMiseEnvFileIsOff measures, with the mise on the search path, what
// the empty MISE_ENV_FILE of miseEnv is for: a caller who sets
// MISE_ENV_FILE=.env, with an untracked .env in the working directory or
// in a parent of it, gets the variables of that file into the go that
// the mise shim starts (a GOFLAGS=-overlay=..., say). The file holds the
// variable PLANTED_MARKER. It loads without the variable, in both
// places, and not with miseEnv, whatever the caller sets. mise runs in
// isolatedMiseEnv.
func TestMiseEnvFileIsOff(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "tree")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[settings]\nlockfile = true\n"), 0o600))
	// A dotenv file holds no template, so its marker is its variable,
	// which "mise env" prints when it loads the file.
	dotenv := "PLANTED_MARKER=planted\n"
	base := isolatedMiseEnv(t)
	printed := func(env []string) bool {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "mise", "-C", dir, "env", "-J")
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.Output()
		require.NoError(t, err, "mise env")
		return strings.Contains(string(out), "PLANTED_MARKER")
	}
	trust := exec.CommandContext(t.Context(), "mise", "-C", dir, "trust", filepath.Join(dir, "mise.toml"))
	trust.Dir = dir
	trust.Env = miseEnviron(base)
	out, err := trust.CombinedOutput()
	require.NoError(t, err, "mise trust\n%s", out)
	for _, place := range []string{dir, parent} {
		path := filepath.Join(place, ".env")
		require.NoError(t, os.WriteFile(path, []byte(dotenv), 0o600))
		env := append(slices.Clone(base), "MISE_ENV_FILE=.env")
		withoutVar := slices.DeleteFunc(miseEnviron(env), func(kv string) bool { return kv == "MISE_ENV_FILE=" })
		assert.True(t, printed(withoutVar), "the .env in %s, without the variable", place)
		assert.False(t, printed(miseEnviron(env)), "the .env in %s, with miseEnv", place)
		require.NoError(t, os.Remove(path))
	}
}

// TestMiseCdAndTrustAreOff measures, with the mise on the search path,
// what the empty MISE_CD and MISE_TRUSTED_CONFIG_PATHS of miseEnv are
// for. A caller's MISE_CD makes mise run its command (the go of the
// shim) in another directory, and a caller's MISE_TRUSTED_CONFIG_PATHS
// makes mise apply the [env] of an untracked mise.toml in a parent of
// the tree. Both take effect without the variable and neither with
// miseEnv, whatever the caller sets. mise runs in isolatedMiseEnv.
func TestMiseCdAndTrustAreOff(t *testing.T) {
	parent, elsewhere := t.TempDir(), t.TempDir()
	dir := filepath.Join(parent, "tree")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[settings]\nlockfile = true\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(parent, "mise.toml"), []byte("[env]\nPLANTED_MARKER = \"planted\"\n"), 0o600))
	base := isolatedMiseEnv(t)
	run := func(env []string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "mise", args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.Output()
		return string(out), err
	}
	_, err := run(miseEnviron(base), "trust", filepath.Join(dir, "mise.toml"))
	require.NoError(t, err)
	without := func(env []string, key string) []string {
		return slices.DeleteFunc(miseEnviron(env), func(kv string) bool { return kv == key+"=" })
	}

	cd := append(slices.Clone(base), "MISE_CD="+elsewhere)
	want, err := filepath.EvalSymlinks(elsewhere)
	require.NoError(t, err)
	out, err := run(without(cd, "MISE_CD"), "exec", "--", "pwd", "-P")
	require.NoError(t, err)
	assert.Equal(t, want, strings.TrimSpace(out), "MISE_CD, without the variable")
	here, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	out, err = run(miseEnviron(cd), "exec", "--", "pwd", "-P")
	require.NoError(t, err)
	assert.Equal(t, here, strings.TrimSpace(out), "MISE_CD, with miseEnv")

	// A new state directory: a run of "mise exec" trusts the parent file
	// for the runs after it (measured with mise 2026.10.3).
	fresh := isolatedMiseEnv(t)
	_, err = run(miseEnviron(fresh), "trust", filepath.Join(dir, "mise.toml"))
	require.NoError(t, err)
	trusted := append(slices.Clone(fresh), "MISE_TRUSTED_CONFIG_PATHS="+parent)
	out, err = run(without(trusted, "MISE_TRUSTED_CONFIG_PATHS"), "env", "-J")
	require.NoError(t, err)
	assert.Contains(t, out, "PLANTED_MARKER", "MISE_TRUSTED_CONFIG_PATHS, without the variable")
	out, _ = run(miseEnviron(trusted), "env", "-J") // mise may refuse the untrusted file
	assert.NotContains(t, out, "PLANTED_MARKER", "MISE_TRUSTED_CONFIG_PATHS, with miseEnv")
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
// workflow, where the runner sets GITHUB_ACTIONS to "true", each
// variable of miseEnv must be set to its value, an empty MISE_ENV
// included, and each one wrong or unset fails on its own; elsewhere,
// another value of GITHUB_ACTIONS among them, nothing is checked.
func TestHostedMiseEnv(t *testing.T) {
	setAll := func() {
		for _, kv := range miseEnv {
			key, value, _ := strings.Cut(kv, "=")
			t.Setenv(key, value)
		}
	}
	for _, value := range []string{"", "1", "TRUE"} {
		t.Setenv("GITHUB_ACTIONS", value)
		t.Setenv("MISE_ENV", "ci")
		require.NoError(t, hostedMiseEnv(), "GITHUB_ACTIONS=%q is not the hosted workflow: nothing is checked", value)
	}

	t.Setenv("GITHUB_ACTIONS", "true")
	setAll()
	require.NoError(t, hostedMiseEnv())
	for _, kv := range miseEnv {
		key, value, _ := strings.Cut(kv, "=")
		t.Run(key, func(t *testing.T) {
			setAll()
			t.Setenv(key, value+"x")
			require.ErrorContains(t, hostedMiseEnv(), "the workflow sets "+kv+" for every mise run, and this run has "+key+"="+value+"x")
			require.NoError(t, os.Unsetenv(key))
			require.ErrorContains(t, hostedMiseEnv(), "the workflow sets "+kv+" for every mise run, and this run has "+key+" unset")
		})
	}
}

// fakeMiseVersion is the version the fake mise of a test reports, and
// answerVersion the lines of its script that report it, as "mise
// --version" prints it.
const (
	fakeMiseVersion = "2026.10.3"
	answerVersion   = "if [ \"$1\" = --version ]; then echo '" + fakeMiseVersion + " linux-x64 (2026-10-05)'; exit 0; fi\n"
)

// miseWorkflow returns goodWorkflow with its mise-action step pinning
// mise at version.
func miseWorkflow(t *testing.T, version string) string {
	t.Helper()
	return edits(t, "          sha256: ${{ matrix.mise_sha256 }}\n", "          version: "+version+"\n          sha256: ${{ matrix.mise_sha256 }}\n")
}

// TestPinnedMiseVersion reads the version of mise that ci.yml pins, in
// this repository and in fixtures: one version, however many steps use
// the action, and an error for none, for two, for an empty one and for
// a missing workflow.
func TestPinnedMiseVersion(t *testing.T) {
	got, err := pinnedMiseVersion(moduleRoot(t))
	require.NoError(t, err)
	assert.Equal(t, "2026.10.3", got, "the version ci.yml pins")

	twice := func(second string) string {
		src := miseWorkflow(t, "2026.10.3")
		step := "      - uses: jdx/mise-action/sub/path@"
		i := strings.Index(src, step)
		require.GreaterOrEqual(t, i, 0)
		return src[:i] + "      - uses: JDX/Mise-Action@" + strings.Repeat("a", 40) + "\n        with:\n          version: " + second + "\n" + src[i:]
	}
	tests := []struct {
		name, src, want string
	}{
		{"one step", miseWorkflow(t, "2026.10.3"), "2026.10.3"},
		{"two steps, one version, the action in any case", twice("2026.10.3"), "2026.10.3"},
		{"two versions", twice("2026.10.4"), `holds ["2026.10.4" "2026.10.3"]`},
		{"no version", goodWorkflow, `holds [""]`},
		{"no mise-action step", strings.ReplaceAll(goodWorkflow, "jdx/mise-action", "jdx/other"), "holds []"},
		{"no workflow", "", "the version of mise is pinned in .github/workflows/ci.yml: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.src != "" {
				path := filepath.Join(root, filepath.FromSlash(pinnedMiseWorkflow))
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte(tt.src), 0o600))
			}
			got, err := pinnedMiseVersion(root)
			if !strings.Contains(tt.want, " ") {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// TestCheckMiseVersion refuses a mise whose version is not the one
// ci.yml pins, and runs "mise --version" with miseEnv at the top of
// the tree.
func TestCheckMiseVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(pinnedMiseWorkflow))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(miseWorkflow(t, fakeMiseVersion)), 0o600))
	f := newFakeMise(t)
	dump := filepath.Join(t.TempDir(), "env")
	write := func(version string) {
		script := "#!/bin/sh\n[ \"$1\" = --version ] || exit 3\necho \"$MISE_OVERRIDE_CONFIG_FILENAMES $(pwd -P)\" > '" + dump + "'\necho '" + version + " linux-x64 (2026-10-05)'\necho 'mise WARN  mise version 2026.10.4 available' >&2\n"
		require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))
	}
	write(fakeMiseVersion)
	require.NoError(t, checkMiseVersion(t.Context(), root))
	data, err := os.ReadFile(dump)
	require.NoError(t, err)
	real, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Equal(t, "mise.toml "+real+"\n", string(data), "with miseEnv, at the top of the tree")

	write("2026.10.4")
	require.ErrorContains(t, checkMiseVersion(t.Context(), root), "mise is at version 2026.10.4, and .github/workflows/ci.yml pins 2026.10.3")
	for _, other := range []string{"2026.10.30", "2026.10"} {
		write(other)
		require.ErrorContains(t, checkMiseVersion(t.Context(), root), "mise is at version "+other+",", "a version that only shares a prefix with the pin")
	}
	write("2024.11.37")
	require.ErrorContains(t, checkMiseVersion(t.Context(), root), "mise is at version 2024.11.37")

	require.NoError(t, os.Remove(filepath.Join(f.bin, "mise")))
	require.ErrorContains(t, checkMiseVersion(t.Context(), root), "mise is not on the search path")
}
