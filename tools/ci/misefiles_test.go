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

// TestMiseEnvList pins the allowlist of environment variables of every
// mise run and of the pre-push hook, and its fixed part, miseEnv:
//
//   - passThrough: where things are on this machine (PATH to find mise
//     and go, HOME for the installs, the caches and the git config,
//     TMPDIR, the XDG directories for those of mise and git, the state
//     and cache directories of mise, and the Go path and caches that
//     hold the module cache);
//   - networkPassThrough: the proxy and the certificate bundle for a
//     download of a module or a tool;
//   - miseEnv and goBuildEnv: the fixed settings.
//
// Every other variable of the caller is dropped, so none of the lists
// holds a key of miseEnv or of goBuildEnv. The module variables
// (goModulePassThrough) are only the hook's: setup's mise install and
// go mod download keep the checksum database as their guard. TestHookEnvIsTheAllowlist
// and TestMiseNeverReads measure what that closes.
func TestMiseEnvList(t *testing.T) {
	assert.Equal(t, []string{
		"PATH", "HOME", "TMPDIR",
		"XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
		"MISE_CACHE_DIR", "MISE_DATA_DIR", "MISE_STATE_DIR",
		"GOPATH", "GOCACHE", "GOMODCACHE",
	}, passThrough)
	assert.Equal(t, []string{"GOPROXY", "GOPRIVATE", "GONOSUMDB", "GOSUMDB", "GOINSECURE"}, goModulePassThrough)
	assert.Equal(t, []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}, networkPassThrough)
	assert.Equal(t, slices.Concat(passThrough, networkPassThrough, goModulePassThrough), hookAllow)
	assert.Equal(t, []string{
		"MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml",
		"MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=none",
		"MISE_ENV=",
		"MISE_AUTO_ENV=false",
		"MISE_GLOBAL_CONFIG_FILE=/dev/null/mise-global.toml",
		"MISE_SYSTEM_CONFIG_FILE=/dev/null/mise-system.toml",
		"GOENV=off",
		"GOTOOLCHAIN=local",
	}, miseEnv)
	assert.Equal(t, []string{"GOWORK=off", "GOFLAGS=-mod=readonly"}, goBuildEnv)
	assert.Equal(t, []string{"GOENV=off", "GOTOOLCHAIN=local"}, goPinEnv)
	assert.Subset(t, stepEnv("/go"), goPinEnv, "stepEnv holds goPinEnv")
	assert.Subset(t, stepEnv("/go"), goBuildEnv, "stepEnv holds goBuildEnv")
	assert.Equal(t, "MISE_CEILING_PATHS=/a/b", miseCeiling("/a/b/tree"))
	for _, kv := range slices.Concat(miseEnv, goBuildEnv) {
		key, _, _ := strings.Cut(kv, "=")
		for _, list := range [][]string{passThrough, networkPassThrough, dockerPassThrough} {
			assert.NotContains(t, list, key)
		}
	}
	for _, key := range slices.Concat(passThrough, networkPassThrough, dockerPassThrough, goModulePassThrough) {
		assert.NotContains(t, []string{"MISE_CEILING_PATHS", "MISE_CD", "MISE_ENV_FILE", "MISE_TRUSTED_CONFIG_PATHS", "MISE_GLOBAL_CONFIG_FILE", "MISE_SYSTEM_CONFIG_FILE", "MISE_CONFIG_DIR", "XDG_CONFIG_HOME", "GOFLAGS", "GOENV", "GOWORK", "GOTOOLCHAIN"}, key)
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
	assert.Equal(t, append(append([]string{"PATH=/x"}, miseEnv...), "MISE_CEILING_PATHS=/a/b"), miseRunEnviron(base, "/a/b/tree"))
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
// the four is there for nothing. mise
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
	for _, key := range []string{"MISE_OVERRIDE_CONFIG_FILENAMES", "MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES", "MISE_ENV", "MISE_AUTO_ENV"} {
		kvIndex := slices.IndexFunc(miseEnv, func(kv string) bool { return strings.HasPrefix(kv, key+"=") })
		require.GreaterOrEqual(t, kvIndex, 0, key)
		kv := miseEnv[kvIndex]
		without := slices.DeleteFunc(miseEnviron(base), func(v string) bool { return v == kv })
		assert.NotEmpty(t, written(without), "miseEnv without %s", key)
	}
}

// TestMiseNeverReads measures, with the real mise, what the allowlist
// closes: a variable of the caller, or a file it plants, that would put
// an [env] into the go that the mise shim starts (a GOFLAGS=-overlay=...
// reaches "go run ./tools/ci" and runs code before any check). Each row
// plants one, and "mise exec" (what the shim does) prints it. Run in the
// environment of the caller, every plant takes effect (the control, so
// no row passes by planting nothing); run in the environment tools/ci
// builds (passThroughEnv, then miseRunEnviron, with the allowlist of
// the hook) none does. The rows are the escapes found by the audits of
// the hook: a global or system configuration, a conf.d, a
// MISE_CONFIG_DIR, a named global file, a .env file of MISE_ENV_FILE in
// the tree or above it, MISE_CD, a trusted or an untrusted mise.toml in
// a parent directory. mise runs with a new state directory for each
// run, since "mise exec" trusts a file it reads for the runs after it.
func TestMiseNeverReads(t *testing.T) {
	envFile := func(name string) string { return "[env]\nPLANTED = \"" + name + "\"\n" }
	type layout struct{ parent, dir, home, elsewhere string }
	write := func(t *testing.T, path, content string) string {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}
	rows := []struct {
		name  string
		plant func(t *testing.T, l layout) map[string]string
	}{
		{"a global configuration below HOME", func(t *testing.T, l layout) map[string]string {
			write(t, filepath.Join(l.home, ".config", "mise", "config.toml"), envFile("a global configuration below HOME"))
			return nil
		}},
		{"a conf.d below HOME", func(t *testing.T, l layout) map[string]string {
			write(t, filepath.Join(l.home, ".config", "mise", "conf.d", "x.toml"), envFile("a conf.d below HOME"))
			return nil
		}},
		{"MISE_CONFIG_DIR", func(t *testing.T, l layout) map[string]string {
			cfg := t.TempDir()
			write(t, filepath.Join(cfg, "config.toml"), envFile("MISE_CONFIG_DIR"))
			return map[string]string{"MISE_CONFIG_DIR": cfg}
		}},
		{"a conf.d of MISE_CONFIG_DIR", func(t *testing.T, l layout) map[string]string {
			cfg := t.TempDir()
			write(t, filepath.Join(cfg, "conf.d", "y.toml"), envFile("a conf.d of MISE_CONFIG_DIR"))
			return map[string]string{"MISE_CONFIG_DIR": cfg}
		}},
		{"MISE_GLOBAL_CONFIG_FILE", func(t *testing.T, l layout) map[string]string {
			return map[string]string{"MISE_GLOBAL_CONFIG_FILE": write(t, filepath.Join(t.TempDir(), "g.toml"), envFile("MISE_GLOBAL_CONFIG_FILE"))}
		}},
		{"MISE_SYSTEM_CONFIG_FILE", func(t *testing.T, l layout) map[string]string {
			return map[string]string{"MISE_SYSTEM_CONFIG_FILE": write(t, filepath.Join(t.TempDir(), "s.toml"), envFile("MISE_SYSTEM_CONFIG_FILE"))}
		}},
		{"MISE_ENV_FILE with a .env in the tree", func(t *testing.T, l layout) map[string]string {
			write(t, filepath.Join(l.dir, ".env"), "PLANTED=MISE_ENV_FILE with a .env in the tree\n")
			return map[string]string{"MISE_ENV_FILE": ".env"}
		}},
		{"MISE_ENV_FILE with a .env in a parent", func(t *testing.T, l layout) map[string]string {
			write(t, filepath.Join(l.parent, ".env"), "PLANTED=MISE_ENV_FILE with a .env in a parent\n")
			return map[string]string{"MISE_ENV_FILE": ".env"}
		}},
		{"MISE_TRUSTED_CONFIG_PATHS", func(t *testing.T, l layout) map[string]string {
			write(t, filepath.Join(l.parent, "mise.toml"), envFile("MISE_TRUSTED_CONFIG_PATHS"))
			return map[string]string{"MISE_TRUSTED_CONFIG_PATHS": l.parent}
		}},
		{"an untrusted mise.toml in a parent", func(t *testing.T, l layout) map[string]string {
			write(t, filepath.Join(l.parent, "mise.toml"), envFile("an untrusted mise.toml in a parent"))
			return nil
		}},
		{"MISE_CD", func(t *testing.T, l layout) map[string]string {
			return map[string]string{"MISE_CD": l.elsewhere}
		}},
		{"GOFLAGS", func(t *testing.T, l layout) map[string]string {
			return map[string]string{"GOFLAGS": "-overlay=/planted.json"}
		}},
		{"GOENV", func(t *testing.T, l layout) map[string]string {
			return map[string]string{"GOENV": "/planted/env"}
		}},
	}
	// printed runs "mise exec", what the shim of go does, and returns
	// what the command sees: the planted marker, the working directory,
	// and the GOFLAGS and GOENV of its environment.
	type seen struct{ marker, pwd, goflags, goenv string }
	printed := func(t *testing.T, env []string, dir string) seen {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "mise", "exec", "--", "sh", "-c",
			`printf '%s|%s|%s|%s' "${PLANTED:-nothing}" "$(pwd -P)" "${GOFLAGS-unset}" "${GOENV-unset}"`)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.Output()
		require.NoError(t, err, "mise exec")
		parts := strings.Split(string(out), "|")
		require.Len(t, parts, 4, string(out))
		return seen{parts[0], parts[1], parts[2], parts[3]}
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			// The tree is reached through a symbolic link to its parent, as
			// a temporary directory is on macOS (/var is /private/var) and
			// a push from a linked directory is anywhere: mise walks the
			// real path, so a ceiling computed from the path as given
			// would never match.
			l := layout{parent: mustEval(t, t.TempDir()), home: t.TempDir(), elsewhere: t.TempDir()}
			link := filepath.Join(t.TempDir(), "link")
			require.NoError(t, os.Symlink(l.parent, link))
			l.dir = filepath.Join(link, "tree")
			write(t, filepath.Join(l.dir, "mise.toml"), "[settings]\nlockfile = true\n")
			t.Setenv("HOME", l.home)
			t.Setenv("MISE_STATE_DIR", t.TempDir())
			t.Setenv("MISE_CACHE_DIR", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", "")
			t.Setenv("MISE_OFFLINE", "1")
			for k, v := range row.plant(t, l) {
				t.Setenv(k, v)
			}
			wantDir, err := filepath.EvalSymlinks(l.dir)
			require.NoError(t, err)

			// The control: the environment of the caller, as it is.
			t.Setenv("MISE_STATE_DIR", t.TempDir())
			control := printed(t, os.Environ(), l.dir)
			switch row.name {
			case "MISE_CD":
				wantElsewhere, err := filepath.EvalSymlinks(l.elsewhere)
				require.NoError(t, err)
				assert.Equal(t, wantElsewhere, control.pwd, "without the allowlist")
			case "GOFLAGS":
				assert.Equal(t, "-overlay=/planted.json", control.goflags, "without the allowlist")
			case "GOENV":
				assert.Equal(t, "/planted/env", control.goenv, "without the allowlist")
			default:
				assert.Equal(t, row.name, control.marker, "without the allowlist")
			}

			// The environment tools/ci builds, with MISE_OFFLINE added for the test.
			t.Setenv("MISE_STATE_DIR", t.TempDir())
			env := append(miseRunEnviron(passThroughEnv(), l.dir), "MISE_OFFLINE=1")
			guarded := printed(t, env, l.dir)
			assert.Equal(t, seen{"nothing", wantDir, "unset", "off"}, guarded, "with the allowlist")
		})
	}
}

// TestMiseCeiling holds the ceiling to the parent of the real path of
// the tree: a root given through a symbolic link, which git can report
// when the working tree is configured so, gives the same ceiling.
func TestMiseCeiling(t *testing.T) {
	real := mustEval(t, t.TempDir())
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(real, link))
	assert.Equal(t, "MISE_CEILING_PATHS="+filepath.Dir(real), miseCeiling(link))
	assert.Equal(t, "MISE_CEILING_PATHS="+filepath.Dir(real), miseCeiling(real))
	assert.Equal(t, "MISE_CEILING_PATHS=/nonexistent", miseCeiling("/nonexistent/tree"), "a root that cannot be resolved keeps its own parent")
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
