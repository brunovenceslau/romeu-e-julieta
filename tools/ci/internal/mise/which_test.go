// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"bytes"
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestResolveLintTools checks that the tools of fast are the ones
// mise.toml pins: the linter and the go command, installed by mise.
func TestResolveLintTools(t *testing.T) {
	tools := citest.PinnedLintTools(t, Path, Environ, ResolveLintTools)
	assert.Equal(t, "golangci-lint", filepath.Base(tools.Linter))
	assert.True(t, filepath.IsAbs(tools.Linter), "an absolute path: %s", tools.Linter)
	assert.FileExists(t, filepath.Join(tools.GoDir, "go"))
	assert.FileExists(t, filepath.Join(tools.GoDir, "gofmt"))
}

// TestLockedVersionsFixture reads the versions of a lock, refuses a
// tool locked at two different versions, and accepts the same version
// written twice.
func TestLockedVersionsFixture(t *testing.T) {
	got, err := LockedVersions([]byte(citest.LockFixture))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"go": "1.27.0", "go:golang.org/x/vuln/cmd/govulncheck": "1.8.0", "golangci-lint": "2.14.0"}, got)
	_, err = LockedVersions([]byte(citest.LockFixture + "[[tools.go]]\nversion = \"1.26.0\"\n"))
	require.ErrorContains(t, err, "mise.lock locks go at two different versions")
	got, err = LockedVersions([]byte(citest.LockFixture + "[[tools.go]]\nversion = \"1.27.0\"\n"))
	require.NoError(t, err, "the same version twice")
	assert.Equal(t, "1.27.0", got["go"])
}

// TestLockedVersionsOtherTable shows that a "version" key below a table
// header other than [[tools.<name>]] is no tool's.
func TestLockedVersionsOtherTable(t *testing.T) {
	got, err := LockedVersions([]byte("[[tools.go]]\n[tools.go.options]\nversion = \"9.9.9\"\n"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestWhichPinnedThroughLinks resolves a tool when both the root and
// HOME are reached through a link, as on macOS, where the temporary
// directory is below /var, a link to /private/var.
func TestWhichPinnedThroughLinks(t *testing.T) {
	f := citest.NewFakeMise(t)
	homeLink := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.Symlink(f.Home, homeLink))
	t.Setenv("HOME", homeLink)
	goCmd := f.Tool(t, "go", "1.27.0", "bin", "go")
	f.Script(t, "", "", strings.Replace(goCmd, f.Home, homeLink, 1))

	realRoot := citest.LockedRoot(t, citest.LockFixture)
	rootLink := filepath.Join(t.TempDir(), "root")
	require.NoError(t, os.Symlink(realRoot, rootLink))

	got, err := WhichPinned(t.Context(), rootLink, "go", "go", "go")
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(goCmd)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestWhichPinnedReadsPinnedTools checks that WhichPinned resolves no
// tool that PinnedTools does not hold: it fails before it starts mise,
// even with a fake mise that would answer and a lock that locks it.
func TestWhichPinnedReadsPinnedTools(t *testing.T) {
	f := citest.NewFakeMise(t)
	tool := f.Tool(t, "other", "1.0.0", "bin", "other")
	f.Script(t, "", "", tool)
	root := citest.LockedRoot(t, citest.LockFixture+"\n[[tools.other]]\nversion = \"1.0.0\"\n")
	_, err := WhichPinned(t.Context(), root, "other", "other", "other")
	require.ErrorContains(t, err, "other is not one of the pinned tools")
	for _, tool := range []string{GoTool, LintTool, GovulncheckLock} {
		assert.Contains(t, PinnedTools, tool)
	}
}

// TestResolveLintToolsFailsClosed runs ResolveLintTools against a fake
// mise (citest.NewFakeMise). Every case that names an error fails, and none
// falls back to a tool on the search path.
func TestResolveLintToolsFailsClosed(t *testing.T) {
	const fixed = "{installs}" // the resolved installs directory, in want
	tests := []struct {
		name string
		// setup prepares the case and returns what the fake mise prints
		// for golangci-lint and for go; no fake mise is written when
		// noMise is set, and one that exits 1 when fails is set.
		setup  func(t *testing.T, f citest.FakeMise) (linter, goCmd string)
		noMise bool
		fails  bool
		// lock is the mise.lock of the root; "" means lockFixture, and
		// "none" no file.
		lock string
		want string
	}{
		{
			name: "a linter at a version beside the locked one",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint", "2.13.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "mise which golangci-lint: the install is not the version 2.14.0 that mise.lock locks: run \"mise install\" in ",
		},
		{
			name: "a go at a version beside the locked one",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.26.0", "bin", "go")
			},
			want: "mise which go: the install is not the version 1.27.0 that mise.lock locks",
		},
		{
			name: "no mise.lock",
			lock: "none",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "mise which golangci-lint: mise.lock: ",
		},
		{
			name: "a tool the lock does not hold",
			lock: "[[tools.go]]\nversion = \"1.27.0\"\n",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "mise.lock locks no version of golangci-lint",
		},
		{
			name: "a tool locked at two versions",
			lock: citest.LockFixture + "[[tools.golangci-lint]]\nversion = \"2.13.0\"\n",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "mise.lock locks golangci-lint at two different versions",
		},
		{name: "no mise on the search path", noMise: true, want: "mise is not on the search path"},
		{
			name:  "mise which fails, with what mise wrote to standard error",
			fails: true,
			want:  "/mise exited 1; the tool is not installed or the configuration is not trusted: run \"mise trust\" and \"mise install\" in ",
		},
		{
			name: "HOME is not set",
			setup: func(t *testing.T, _ citest.FakeMise) (string, string) {
				t.Helper()
				t.Setenv("HOME", "")
				return "", ""
			},
			want: "HOME is not set",
		},
		{
			name: "no installs directory",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				require.NoError(t, os.Remove(f.Installs()))
				return "", ""
			},
			want: "the mise installs directory " + fixed + ": ",
		},
		{
			name: "a linter outside the installs directory",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return citest.Executable(t, filepath.Join(t.TempDir(), "golangci-lint")), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "golangci-lint is outside " + filepath.Join(fixed, "golangci-lint") + ", where mise installs it: the mise configuration names a tool mise did not install",
		},
		{
			name: "a linter in the directory of another tool",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "go", "1.27.0", "bin", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "is outside " + filepath.Join(fixed, "golangci-lint") + ",",
		},
		{
			name: "a linter in a sibling directory that shares the tool's prefix",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint-evil", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "is outside " + filepath.Join(fixed, "golangci-lint") + ",",
		},
		{
			name: "a go below the installs directory that links outside it",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				link := filepath.Join(f.Installs(), "go", "1.27.0", "bin", "go")
				require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
				require.NoError(t, os.Symlink(citest.Executable(t, filepath.Join(t.TempDir(), "go")), link))
				return f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), link
			},
			want: "mise which go: ",
		},
		{
			name: "both below the installs directory",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				return f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go")
			},
		},
		{
			// As on macOS, where the temporary directory is below /var, a
			// link to /private/var.
			name: "both below an installs directory reached through a link",
			setup: func(t *testing.T, f citest.FakeMise) (string, string) {
				t.Helper()
				link := filepath.Join(t.TempDir(), "home")
				require.NoError(t, os.Symlink(f.Home, link))
				t.Setenv("HOME", link)
				linter := f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint")
				goCmd := f.Tool(t, "go", "1.27.0", "bin", "go")
				return strings.Replace(linter, f.Home, link, 1), strings.Replace(goCmd, f.Home, link, 1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := citest.NewFakeMise(t)
			var linter, goCmd string
			if tt.setup != nil {
				linter, goCmd = tt.setup(t, f)
			}
			switch {
			case tt.fails:
				require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte("#!/bin/sh\necho 'mise ERROR not trusted' >&2\nexit 1\n"), 0o700))
			case !tt.noMise:
				f.Script(t, "", linter, goCmd)
			}
			root := t.TempDir()
			if tt.lock != "none" {
				root = citest.LockedRoot(t, cmp.Or(tt.lock, citest.LockFixture))
			}
			tools, err := ResolveLintTools(t.Context(), root)
			if tt.want != "" {
				installs, evalErr := filepath.EvalSymlinks(f.Installs())
				if evalErr != nil {
					installs = f.Installs()
				}
				require.ErrorContains(t, err, strings.ReplaceAll(tt.want, fixed, installs))
				if tt.fails {
					require.ErrorContains(t, err, "mise which golangci-lint: "+filepath.Join(f.Bin, "mise")+" exited 1;", "the probe and the mise it started")
					require.ErrorContains(t, err, "\nmise ERROR not trusted", "what mise wrote to standard error")
				}
				assert.Equal(t, check.Tools{}, tools)
				return
			}
			require.NoError(t, err)
			realLinter, err := filepath.EvalSymlinks(linter)
			require.NoError(t, err)
			realGo, err := filepath.EvalSymlinks(goCmd)
			require.NoError(t, err)
			assert.Equal(t, check.Tools{Linter: realLinter, GoDir: filepath.Dir(realGo)}, tools)
		})
	}
}

// TestResolveLintToolsEnvironment checks that "mise which" runs in
// check.PassThroughEnv with MiseEnv, at the top of the tree: a fake mise
// writes its environment and its working directory to files, and every
// variable of goenv.PassThrough and of MiseEnv reaches it and nothing else,
// with the ones a shell adds itself, whatever else this process holds;
// a value of MiseEnv is the list's, never the process's. Each variable
// of goenv.PassThrough is set here, so the result does not depend on the
// caller's environment (a TMPDIR or a MISE_STATE_DIR of its own).
func TestResolveLintToolsEnvironment(t *testing.T) {
	envPath, err := procgroup.LookPath("env")
	require.NoError(t, err)
	f := citest.NewFakeMise(t) // sets HOME and PATH
	for _, key := range []string{"GOFLAGS", "MISE_INSTALLS_DIR", "MISE_TRUSTED_CONFIG_PATHS", "CI", "MISE_ENV"} {
		t.Setenv(key, "from-the-process")
	}
	for _, key := range goenv.PassThrough {
		if key != "HOME" && key != "PATH" {
			t.Setenv(key, "/from/"+key)
		}
	}
	t.Setenv("MISE_DATA_DIR", filepath.Dir(f.Installs())) // where the fake's tools are
	dump, cwd := filepath.Join(t.TempDir(), "env"), filepath.Join(t.TempDir(), "cwd")
	f.Script(t, "'"+envPath+"' > '"+dump+"'\npwd -P > '"+cwd+"'\n", f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.Tool(t, "go", "1.27.0", "bin", "go"))
	root := citest.LockedRoot(t, citest.LockFixture)
	_, err = ResolveLintTools(t.Context(), root)
	require.NoError(t, err)

	data, err := os.ReadFile(dump)
	require.NoError(t, err)
	var keys []string
	values := map[string]string{}
	for line := range strings.Lines(string(data)) {
		key, value, _ := strings.Cut(strings.TrimSuffix(line, "\n"), "=")
		if !slices.Contains([]string{"PWD", "OLDPWD", "SHLVL", "_"}, key) {
			keys = append(keys, key)
			values[key] = value
		}
	}
	want := slices.Clone(goenv.PassThrough)
	for _, kv := range MiseEnv {
		key, value, _ := strings.Cut(kv, "=")
		want = append(want, key)
		assert.Equal(t, value, values[key], key)
	}
	want = append(want, "MISE_CEILING_PATHS")
	assert.Equal(t, strings.TrimPrefix(miseCeiling(root), "MISE_CEILING_PATHS="), values["MISE_CEILING_PATHS"], "the ceiling is the parent of the tree")
	assert.Equal(t, slices.Sorted(slices.Values(want)), slices.Sorted(slices.Values(keys)), "the variables mise gets")
	dir, err := os.ReadFile(cwd)
	require.NoError(t, err)
	real, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Equal(t, real+"\n", string(dir), "mise starts at the top of the tree, where it finds .miserc.toml")
}

// TestMiseInstalls holds miseInstalls to the data directory that mise
// itself reports in the environment it runs mise in, for each way the
// directory is chosen (HOME, XDG_DATA_HOME, MISE_DATA_DIR), and pins it
// on the variables it reads.
func TestMiseInstalls(t *testing.T) {
	data, xdg := t.TempDir(), t.TempDir()
	misePath := pinnedMise(t)
	for _, tt := range []struct {
		name, data, xdg string
		want            func(home string) string
	}{
		{"below HOME", "", "", func(home string) string { return filepath.Join(home, ".local", "share", "mise", "installs") }},
		{"XDG_DATA_HOME", "", xdg, func(string) string { return filepath.Join(xdg, "mise", "installs") }},
		{"MISE_DATA_DIR", data, xdg, func(string) string { return filepath.Join(data, "installs") }},
	} {
		t.Run("mise reports "+tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("MISE_DATA_DIR", tt.data)
			t.Setenv("XDG_DATA_HOME", tt.xdg)
			t.Setenv("MISE_STATE_DIR", t.TempDir())
			var out bytes.Buffer
			err := procgroup.Run(t.Context(), procgroup.Command{
				Class:  procgroup.ToolProbe,
				Argv:   []string{misePath, "-C", t.TempDir(), "doctor", "--json"},
				Env:    append(Environ(check.PassThroughEnv()), "MISE_OFFLINE=1"),
				Stdout: &out,
			})
			require.NoError(t, err, "mise doctor")
			var doctor struct{ Dirs struct{ Data string } }
			require.NoError(t, json.Unmarshal(out.Bytes(), &doctor))
			require.NotEmpty(t, doctor.Dirs.Data, "the data directory mise reports")
			installs, err := miseInstalls()
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(doctor.Dirs.Data, "installs"), installs, "the installs directory of mise")
			assert.Equal(t, tt.want(home), installs)
		})
	}

	tests := []struct {
		name            string
		data, xdg, home string
		want            string // the directory, or the start of the error
	}{
		{"below HOME", "", "", "/h", "/h/.local/share/mise/installs"},
		{"MISE_DATA_DIR set", "/m", "/x", "/h", "/m/installs"},
		{"XDG_DATA_HOME set", "", "/x", "/h", "/x/mise/installs"},
		{"HOME not set", "", "", "", "HOME is not set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MISE_DATA_DIR", tt.data)
			t.Setenv("XDG_DATA_HOME", tt.xdg)
			t.Setenv("HOME", tt.home)
			got, err := miseInstalls()
			if !strings.HasPrefix(tt.want, "/") {
				require.ErrorContains(t, err, tt.want)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.FromSlash(tt.want), got)
		})
	}
}
