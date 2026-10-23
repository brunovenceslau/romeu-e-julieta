// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && unix

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j1"
)

// initSetup is an Env whose origins hold the config repo owner/cfg, and
// the line of J1 step 3 that initializes it.
func initSetup(t *testing.T, files map[string]string) (*romeu.Env, *romeu.Origins, []string) {
	t.Helper()
	o := romeu.NewOrigins(t)
	url := o.PushConfigRepo(t, "owner", "cfg", files)
	return romeu.New(t, o), o, []string{"init", "--config-url", url}
}

// TestInitJ1Step3 runs step 3 of J1 as the scenario function reads it
// from commands.yaml: init writes host settings with absolute tool paths
// and the real root, and clones the config repo to <root>/<cfg>-env/<dir>
// from an origin served over TLS (REQ-0309, 10 10.1).
func TestInitJ1Step3(t *testing.T) {
	o := romeu.NewOrigins(t)
	o.PushConfigRepo(t, "owner", "cfg", nil)
	e := romeu.New(t, o)
	tbl, err := scenarios.Load("j1")
	require.NoError(t, err)

	s := j1.J1(tbl, j1.Vars{Owner: "owner", ConfigRepo: "cfg"})

	require.NoError(t, s.Run(e.World(t)))
	assert.Empty(t, tbl.Unread())
	h := e.ReadSettings(t)
	assert.Equal(t, e.Root, h.Root)
	assert.Equal(t, e.Sbx, h.Tools.Sbx)
	assert.Equal(t, e.Git, h.Tools.Git)
	assert.Equal(t, "https://github.com/owner/cfg", h.Config.URL)
	assert.Equal(t, "main", h.Config.Ref)
	fi, err := os.Stat(e.Settings())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	clone := filepath.Join(e.Root, "cfg-env", "cfg")
	assert.DirExists(t, filepath.Join(clone, ".git"))
	assert.FileExists(t, filepath.Join(clone, "projects", "cfg.yaml"))
	for _, d := range []string{filepath.Join(e.Root, "cfg-env"), e.StateDir()} {
		fi, err := os.Stat(d)
		require.NoError(t, err, d)
		assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm(), d)
	}
	assert.Empty(t, e.SbxCalls(t), "init calls no sbx")
	entries, err := os.ReadDir(filepath.Join(e.Root, "cfg-env"))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no staging directory is left behind")
}

// TestInitCloneDirIsTheSpecs: the clone goes to the dir the config
// project's primary repo names, not to the repo's name.
func TestInitCloneDirIsTheSpecs(t *testing.T) {
	spec := "schema: project/v1\nname: cfg\nrepos:\n  - url: https://github.com/owner/cfg\n    primary: true\n    dir: home\n" +
		"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: sbx-kit-claude\n"
	e, _, args := initSetup(t, map[string]string{"projects/cfg.yaml": spec})

	res := e.Run(t, args)

	require.Equal(t, 0, res.Code, res.Stderr)
	assert.DirExists(t, filepath.Join(e.Root, "cfg-env", "home", ".git"))
}

// TestInitRefusesToOverwriteSettings: a second init exits 2 and leaves
// the first one's settings as they were.
func TestInitRefusesToOverwriteSettings(t *testing.T) {
	e, _, args := initSetup(t, nil)
	require.Equal(t, 0, e.Run(t, args).Code)
	before, err := os.ReadFile(e.Settings())
	require.NoError(t, err)

	res := e.Run(t, args)

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "settings-exist")
	after, err := os.ReadFile(e.Settings())
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

// TestInitRefusesAnUnsafeRoot: a root inside a git repository, equal to
// $HOME, holding the settings or the state, not absolute or not owned
// by the user is refused with exit 2 before anything is written (I20,
// REQ-0375): no settings, no state directory, no root.
func TestInitRefusesAnUnsafeRoot(t *testing.T) {
	cases := []struct {
		name   string
		root   func(e *romeu.Env) string
		reason string
	}{
		{"inside a git repository", func(e *romeu.Env) string {
			require.NoError(t, os.MkdirAll(filepath.Join(e.Home, "dotfiles", ".git"), 0o700))
			return filepath.Join(e.Home, "dotfiles", "dev")
		}, "the root is inside the git repository at"},
		{"equal to HOME", func(e *romeu.Env) string { return e.Home }, "the root is $HOME"},
		{"holding the settings", func(e *romeu.Env) string { return e.ConfigHome }, "the root holds the host settings"},
		{"holding the state", func(e *romeu.Env) string { return e.StateHome }, "the root holds the host state"},
		{"not absolute", func(*romeu.Env) string { return "dev" }, "the root is not an absolute, clean path"},
	}
	if os.Geteuid() != 0 { // root owns /usr
		cases = append(cases, struct {
			name   string
			root   func(e *romeu.Env) string
			reason string
		}{"not owned by the user", func(*romeu.Env) string { return "/usr" }, "the root is not owned by the user"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _, args := initSetup(t, nil)
			root := tc.root(e)

			res := e.Run(t, append(args, "--root", root))

			assert.Equal(t, 2, res.Code, res.Stderr)
			assert.Contains(t, res.Stderr, "root-unsafe")
			assert.Contains(t, res.Stderr, tc.reason)
			assert.NoFileExists(t, e.Settings())
			assert.NoDirExists(t, e.StateDir())
			if filepath.IsAbs(root) && root != "/usr" {
				assert.NoDirExists(t, filepath.Join(root, "cfg-env"))
			}
		})
	}
}

// TestInitRootIsStoredReal: a root reached through a symbolic link is
// stored as the real path, resolved once (03 3.4).
func TestInitRootIsStoredReal(t *testing.T) {
	e, _, args := initSetup(t, nil)
	real := filepath.Join(e.Home, "real")
	require.NoError(t, os.Mkdir(real, 0o700))
	link := filepath.Join(e.Home, "link")
	require.NoError(t, os.Symlink(real, link))

	res := e.Run(t, append(args, "--root", filepath.Join(link, "dev")))

	require.Equal(t, 0, res.Code, res.Stderr)
	assert.Equal(t, filepath.Join(real, "dev"), e.ReadSettings(t).Root)
	assert.DirExists(t, filepath.Join(real, "dev", "cfg-env", "cfg", ".git"))
}

// TestInitPaths: ROMEU_SETTINGS moves the settings path; XDG_CONFIG_HOME
// and XDG_STATE_HOME move the settings and the state; no other ROMEU_*
// variable moves a path, and the root is the flag's or $HOME/dev
// (REQ-0150).
func TestInitPaths(t *testing.T) {
	t.Run("ROMEU_SETTINGS", func(t *testing.T) {
		e, _, args := initSetup(t, nil)
		elsewhere := filepath.Join(t.TempDir(), "s", "settings.yaml")

		res := e.Run(t, args, "ROMEU_SETTINGS="+elsewhere)

		require.Equal(t, 0, res.Code, res.Stderr)
		assert.FileExists(t, elsewhere)
		assert.NoFileExists(t, e.Settings())
	})
	t.Run("XDG_CONFIG_HOME and XDG_STATE_HOME", func(t *testing.T) {
		e, _, args := initSetup(t, nil)
		x := t.TempDir()

		res := e.Run(t, args, "XDG_CONFIG_HOME="+filepath.Join(x, "c"), "XDG_STATE_HOME="+filepath.Join(x, "s"))

		require.Equal(t, 0, res.Code, res.Stderr)
		assert.FileExists(t, filepath.Join(x, "c", "romeu", "settings.yaml"))
		assert.FileExists(t, filepath.Join(x, "s", "romeu", "romeu.lock"))
		assert.NoFileExists(t, e.Settings())
		assert.NoDirExists(t, e.StateDir())
	})
	t.Run("no other ROMEU variable", func(t *testing.T) {
		e, _, args := initSetup(t, nil)
		x := t.TempDir()

		res := e.Run(t, args, "ROMEU_ROOT="+filepath.Join(x, "root"), "ROMEU_STATE="+filepath.Join(x, "state"),
			"ROMEU_CONFIG="+filepath.Join(x, "config"), "ROMEU_HOME="+x)

		require.Equal(t, 0, res.Code, res.Stderr)
		assert.Equal(t, e.Root, e.ReadSettings(t).Root)
		assert.FileExists(t, e.Settings())
		entries, err := os.ReadDir(x)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

// TestInitConfigProjectMissing: a config repo whose ref holds no
// projects/<cfg>.yaml, or whose spec does not name it as the primary, is
// config-project-missing (RJ-346), exit 2, and leaves no settings and no
// clone.
func TestInitConfigProjectMissing(t *testing.T) {
	other := "schema: project/v1\nname: cfg\nrepos:\n  - url: https://github.com/owner/else\n    primary: true\n" +
		"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: sbx-kit-claude\n"
	for name, tc := range map[string]struct {
		files map[string]string
		args  []string
	}{
		"no spec file":    {map[string]string{"projects/cfg.yaml": ""}, []string{"--config-project", "nothere"}},
		"another primary": {map[string]string{"projects/cfg.yaml": other}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			e, _, args := initSetup(t, tc.files)

			res := e.Run(t, append(args, tc.args...))

			assert.Equal(t, 2, res.Code, res.Stderr)
			assert.Contains(t, res.Stderr, "RJ-346")
			assert.NoFileExists(t, e.Settings())
			envs, err := filepath.Glob(filepath.Join(e.Root, "*-env", "*"))
			require.NoError(t, err)
			assert.Empty(t, envs, "no clone and no staging directory")
		})
	}
}

// TestInitRefusesARefThatIsNotTheDefaultBranch: in v1 init takes the
// default branch of the config repo only. A second branch of the origin
// is refused with exit 2 and the slug, whether or not it holds the spec,
// and nothing is written; the default branch is still accepted.
func TestInitRefusesARefThatIsNotTheDefaultBranch(t *testing.T) {
	e, o, args := initSetup(t, nil)
	o.Push(t, "owner/cfg", "release", map[string]string{"extra": "1"})

	for _, ref := range []string{"release", "nobranch"} {
		res := e.Run(t, append(args, "--config-ref", ref))

		assert.Equal(t, 2, res.Code, res.Stderr)
		assert.Contains(t, res.Stderr, "config-ref-not-default")
		assert.NoFileExists(t, e.Settings())
		envs, err := filepath.Glob(filepath.Join(e.Root, "*-env", "*"))
		require.NoError(t, err)
		assert.Empty(t, envs, "no clone and no staging directory")
	}
	assert.Equal(t, 0, e.Run(t, append(args, "--config-ref", "main")).Code)
}

// TestInitTakesTheLock: init holds romeu.lock (04 4.1, Locking), so
// another romeu running is locked (RJ-101), exit 1, and nothing is
// written.
func TestInitTakesTheLock(t *testing.T) {
	e, _, args := initSetup(t, nil)
	require.NoError(t, os.MkdirAll(e.StateDir(), 0o700))
	f, err := os.OpenFile(filepath.Join(e.StateDir(), "romeu.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB))

	res := e.Run(t, args)

	assert.Equal(t, 1, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "RJ-101")
	assert.NoFileExists(t, e.Settings())
}

// TestInitNeedsTheConfigURL is a usage error, exit 2.
func TestInitNeedsTheConfigURL(t *testing.T) {
	e := romeu.New(t, nil)

	res := e.Run(t, []string{"init"})

	assert.Equal(t, 2, res.Code)
	assert.Contains(t, res.Stderr, "RJ-349")
}
