// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hostpaths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realTemp returns a new temporary directory as a real path, as a macOS
// temporary directory is not.
func realTemp(t *testing.T) string {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return tmp
}

// TestRealPaths resolves the three paths and names the one it cannot.
func TestRealPaths(t *testing.T) {
	tmp := realTemp(t)
	require.NoError(t, os.Symlink(tmp, filepath.Join(tmp, "link")))
	loop := filepath.Join(tmp, "loop")
	require.NoError(t, os.Symlink(loop, loop))

	got, err := RealPaths(filepath.Join(tmp, "link", "h"), filepath.Join(tmp, "link", "s.yaml"), filepath.Join(tmp, "st"))

	require.NoError(t, err)
	assert.Equal(t, Paths{Home: filepath.Join(tmp, "h"), Settings: filepath.Join(tmp, "s.yaml"), State: filepath.Join(tmp, "st")}, got)
	for _, tc := range []struct{ name, home, settings, state, want string }{
		{"settings", tmp, loop, tmp, "the settings path: "},
		{"state", tmp, tmp, loop, "the state path: "},
		{"home", loop, tmp, tmp, "HOME: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RealPaths(tc.home, tc.settings, tc.state)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestRealPath resolves the links of the part that exists and keeps the
// rest, and leaves a relative path as it is.
func TestRealPath(t *testing.T) {
	tmp := realTemp(t)
	require.NoError(t, os.Mkdir(filepath.Join(tmp, "real"), 0o700))
	require.NoError(t, os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "link")))

	got, err := RealPath(filepath.Join(tmp, "link", "a", "b"))

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "real", "a", "b"), got)
	loop := filepath.Join(tmp, "loop")
	require.NoError(t, os.Symlink(loop, loop))
	_, err = RealPath(loop)
	require.Error(t, err)
	got, err = RealPath("dev")
	require.NoError(t, err)
	assert.Equal(t, "dev", got)
}

// TestRootProblem holds the root check to one row per rule of
// root-unsafe (04 4.2, REQ-0375, REQ-1277; I20, REQ-1281, REQ-1282):
// each unsafe root fails with the message that names its rule, and a
// safe root, absent or present, passes. A rule that a link could hide
// has a row through a link too.
func TestRootProblem(t *testing.T) {
	tmp := realTemp(t)
	home := filepath.Join(tmp, "home")
	cfgDir := filepath.Join(home, ".config", "romeu")
	stateDir := filepath.Join(home, ".local", "state", "romeu")
	p := Paths{Home: home, Settings: filepath.Join(cfgDir, "settings.yaml"), State: stateDir}
	for _, d := range []string{cfgDir, stateDir, filepath.Join(home, "repo", ".git"), filepath.Join(home, "dev"), filepath.Join(home, "target")} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(home, "file"), nil, 0o600))
	link := func(name, target string) string {
		l := filepath.Join(tmp, name)
		require.NoError(t, os.Symlink(target, l))
		return l
	}
	dir := func(name string, mode os.FileMode) string {
		d := filepath.Join(home, name)
		require.NoError(t, os.MkdirAll(d, 0o700))
		require.NoError(t, os.Chmod(d, mode))
		return d
	}
	// The env dirs: <root>/cfg-env as a link, as a file, as a directory
	// its group can write.
	envLink := dir("envlink", 0o700)
	require.NoError(t, os.Symlink(filepath.Join(home, "target"), filepath.Join(envLink, "cfg-env")))
	envFile := dir("envfile", 0o700)
	require.NoError(t, os.WriteFile(filepath.Join(envFile, "cfg-env"), nil, 0o600))
	envOpen := dir("envopen", 0o700)
	require.NoError(t, os.Mkdir(filepath.Join(envOpen, "cfg-env"), 0o700))
	require.NoError(t, os.Chmod(filepath.Join(envOpen, "cfg-env"), 0o770))
	envSafe := dir("envsafe", 0o700)
	require.NoError(t, os.Mkdir(filepath.Join(envSafe, "cfg-env"), 0o700))

	for _, tc := range []struct{ name, root, want string }{
		{"safe and absent", filepath.Join(home, "new", "dev"), ""},
		{"safe and present", filepath.Join(home, "dev"), ""},
		{"safe with its env dir", envSafe, ""},
		{"not absolute", "dev", "the root is not an absolute, clean path"},
		{"not clean", home + "/x/../dev", "the root is not an absolute, clean path"},
		{"$HOME", home, "the root is $HOME"},
		{"$HOME through a link", link("homealias", home), "the root is $HOME"},
		{"holds the settings", filepath.Join(home, ".config"), "the root holds the host settings"},
		{"holds the settings through a link", link("cfgalias", filepath.Join(home, ".config")), "the root holds the host settings"},
		{"the settings directory", cfgDir, "the root holds the host settings"},
		{"holds the state", filepath.Join(home, ".local"), "the root holds the host state"},
		{"the state directory", stateDir, "the root holds the host state"},
		{"inside the settings directory", filepath.Join(cfgDir, "dev"), "the root is inside the host settings directory"},
		{"inside the settings directory through a link", filepath.Join(link("cfgdiralias", cfgDir), "dev"), "the root is inside the host settings directory"},
		{"inside the state directory", filepath.Join(stateDir, "dev"), "the root is inside the host state directory"},
		{"inside the state directory through a link", filepath.Join(link("statealias", stateDir), "dev"), "the root is inside the host state directory"},
		{"inside a git repository", filepath.Join(home, "repo", "dev"), "the root is inside the git repository at " + filepath.Join(home, "repo")},
		{"a git repository", filepath.Join(home, "repo"), "the root is inside the git repository at " + filepath.Join(home, "repo")},
		{"a symbolic link", link("rootlink", filepath.Join(home, "dev")), "the root is a symbolic link"},
		{"not a directory", filepath.Join(home, "file"), "the root is not a directory"},
		{"writable by its group", dir("opengroup", 0o770), "the root is writable by its group or others"},
		{"writable by others", dir("openother", 0o702), "the root is writable by its group or others"},
		{"an env dir that is a symbolic link", envLink, "cfg-env is a symbolic link"},
		{"an env dir that is not a directory", envFile, "cfg-env is not a directory"},
		{"an env dir writable by its group", envOpen, "cfg-env is writable by its group or others"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RootProblem(tc.root, "cfg-env", p))
		})
	}
	t.Run("the env dir is left out when no name is given", func(t *testing.T) {
		assert.Empty(t, RootProblem(envLink, "", p))
	})
}

// TestRootProblemAsAnotherUser holds the rules that need a directory the
// user cannot own or read, which the superuser can: a root the user does
// not own, a parent the root's path cannot be searched in, which is
// unsafe and not taken for an absent root with no repository above it,
// and a root the user can read but not search, whose .git cannot be
// looked for (the error branch of the git rule).
func TestRootProblemAsAnotherUser(t *testing.T) {
	if os.Geteuid() == 0 {
		return // the superuser owns /usr and reads every directory
	}
	tmp := realTemp(t)
	p := Paths{Home: filepath.Join(tmp, "home"), Settings: "/s/settings.yaml", State: "/s/state"}
	locked := filepath.Join(tmp, "locked")
	require.NoError(t, os.Mkdir(locked, 0o700))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	assert.Equal(t, "the root is not owned by the user", RootProblem("/usr", "", p))
	assert.Contains(t, RootProblem(filepath.Join(locked, "dev"), "", p), "the root cannot be read")
	unsearchable := filepath.Join(tmp, "unsearchable")
	require.NoError(t, os.Mkdir(unsearchable, 0o700))
	require.NoError(t, os.Chmod(unsearchable, 0o600))
	t.Cleanup(func() { _ = os.Chmod(unsearchable, 0o700) })
	assert.Contains(t, RootProblem(unsearchable, "", p), "the root cannot be checked for a git repository: ")
}

// TestDirProblem names each fault of a directory of the tree.
func TestDirProblem(t *testing.T) {
	tmp := realTemp(t)
	ok := filepath.Join(tmp, "ok")
	require.NoError(t, os.Mkdir(ok, 0o700))
	open := filepath.Join(tmp, "open")
	require.NoError(t, os.Mkdir(open, 0o700))
	require.NoError(t, os.Chmod(open, 0o770))
	file := filepath.Join(tmp, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	link := filepath.Join(tmp, "link")
	require.NoError(t, os.Symlink(ok, link))

	for _, tc := range []struct{ name, path, want string }{
		{"safe", ok, ""},
		{"writable by its group", open, "is writable by its group or others"},
		{"not a directory", file, "is not a directory"},
		{"a symbolic link", link, "is a symbolic link"},
		{"absent", filepath.Join(tmp, "none"), "cannot be read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DirProblem(os.Lstat, tc.path)
			if tc.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tc.want)
		})
	}
	if os.Geteuid() != 0 {
		assert.Equal(t, "is not owned by the user", DirProblem(os.Lstat, "/usr"))
	}
}
