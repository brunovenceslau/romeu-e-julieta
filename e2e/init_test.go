// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && unix

package e2e

import (
	"os"
	"path/filepath"
	"strings"
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
		{"inside the settings directory", func(e *romeu.Env) string { return filepath.Join(e.ConfigHome, "romeu", "dev") }, "the root is inside the host settings directory"},
		{"inside the state directory", func(e *romeu.Env) string { return filepath.Join(e.StateDir(), "dev") }, "the root is inside the host state directory"},
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

// TestInitRefusesARemoteWithoutADefaultBranch: a remote whose HEAD names
// no branch has no default branch, so no ref is the default one; init
// fails closed, whatever branch exists, and writes nothing.
func TestInitRefusesARemoteWithoutADefaultBranch(t *testing.T) {
	e, o, args := initSetup(t, nil)
	o.Push(t, "owner/cfg", "release", map[string]string{"extra": "1"})
	o.Git(t, o.Bare("owner/cfg"), "symbolic-ref", "HEAD", "refs/heads/gone")

	res := e.Run(t, append(args, "--config-ref", "release"))

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "config-ref-not-default")
	assert.Contains(t, res.Stderr, "no default branch")
	assert.NoFileExists(t, e.Settings())
	envs, err := filepath.Glob(filepath.Join(e.Root, "*-env", "*"))
	require.NoError(t, err)
	assert.Empty(t, envs, "no clone and no staging directory")
}

// TestInitDoesNotFollowALinkedEnvDir: a <cfg>-env that is a symbolic link
// is refused (root-unsafe, exit 2); its target is not chmod'ed and holds
// no clone.
func TestInitDoesNotFollowALinkedEnvDir(t *testing.T) {
	e, _, args := initSetup(t, nil)
	victim := filepath.Join(e.Home, "victim")
	require.NoError(t, os.Mkdir(victim, 0o755))
	require.NoError(t, os.Chmod(victim, 0o755))
	require.NoError(t, os.MkdirAll(e.Root, 0o700))
	require.NoError(t, os.Symlink(victim, filepath.Join(e.Root, "cfg-env")))

	res := e.Run(t, args)

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "root-unsafe")
	assert.Contains(t, res.Stderr, "symbolic link")
	fi, err := os.Stat(victim)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm(), "the target is not chmod'ed")
	entries, err := os.ReadDir(victim)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NoFileExists(t, e.Settings())
}

// TestInitRefusesAGroupWritableRoot: a root that exists and is writable
// by its group is refused after it is made, with nothing written in it.
func TestInitRefusesAGroupWritableRoot(t *testing.T) {
	e, _, args := initSetup(t, nil)
	require.NoError(t, os.MkdirAll(e.Root, 0o700))
	require.NoError(t, os.Chmod(e.Root, 0o770))

	res := e.Run(t, args)

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "root-unsafe")
	assert.Contains(t, res.Stderr, "writable by its group or others")
	entries, err := os.ReadDir(e.Root)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NoFileExists(t, e.Settings())
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

// privateSetup is an Env whose origins hold the config repo owner/cfg
// behind basic authentication, as a private repository of a forge is,
// and the line of J1 step 3 that initializes it.
func privateSetup(t *testing.T) (*romeu.Env, []string) {
	t.Helper()
	o := romeu.NewAuthOrigins(t, "operator", "s3cret-token")
	url := o.PushConfigRepo(t, "owner", "cfg", nil)
	return romeu.New(t, o), []string{"init", "--config-url", url}
}

// assertNothingLeft holds that init wrote no settings and left nothing
// under the root: no clone, no staging directory, and neither the env
// dir nor the root, which this run made.
func assertNothingLeft(t *testing.T, e *romeu.Env) {
	t.Helper()
	assert.NoFileExists(t, e.Settings())
	assert.NoDirExists(t, e.Root, "the root this run made is removed")
}

// TestInitPrivateConfigRepoWithoutAHelper reproduces the failure of
// 2026-10-23: a config repo that needs credentials, and no
// --git-credential-helper. Hardened git has no helper and no prompt, so
// the clone fails; init reports config-clone-failed, exit 1, with git's
// output in the details and the flag in the hint, never internal
// (RJ-348), and leaves nothing behind.
func TestInitPrivateConfigRepoWithoutAHelper(t *testing.T) {
	e, args := privateSetup(t)

	res := e.Run(t, args)

	assert.Equal(t, 1, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "RJ-356")
	assert.Contains(t, res.Stderr, "config-clone-failed")
	assert.NotContains(t, res.Stderr, "RJ-348")
	assert.Contains(t, res.Stderr, "terminal prompts disabled", "git's output is in the details")
	assert.Contains(t, res.Stderr, "--git-credential-helper", "the hint names the flag")
	assertNothingLeft(t, e)
}

// TestInitPrivateConfigRepoWithAHelper: --git-credential-helper with an
// absolute path clones the private config repo, and the settings record
// the helper. The helper reaches the clone only, scoped to the config
// repo's host as git connects to it: no other git call of init carries
// it, and no call sets a helper for every host.
func TestInitPrivateConfigRepoWithAHelper(t *testing.T) {
	e, args := privateSetup(t)
	helper := e.WriteCredentialHelper(t, "operator", "s3cret-token")
	calls := e.RecordGit(t)

	res := e.Run(t, append(args, "--git-credential-helper", helper))

	require.Equal(t, 0, res.Code, res.Stderr)
	assert.FileExists(t, filepath.Join(e.Root, "cfg-env", "cfg", "projects", "cfg.yaml"))
	assert.Equal(t, helper, e.ReadSettings(t).Tools.GitCredentialHelper)
	var with [][]string
	for _, argv := range calls() {
		for _, a := range argv {
			assert.NotEqual(t, "credential.helper="+helper, a, "no helper for every host")
			if strings.HasPrefix(a, "credential.https://") && strings.HasSuffix(a, ".helper="+helper) {
				with = append(with, argv)
			}
		}
	}
	require.Len(t, with, 1, "one git call carries the helper")
	assert.Contains(t, with[0], "clone")
	assert.NotContains(t, res.Stdout+res.Stderr, "s3cret-token")
}

// TestInitPrivateConfigRepoWithAWrongCredential: a helper whose
// credentials the origin refuses is config-clone-failed too, and the
// password the helper gave is printed nowhere.
func TestInitPrivateConfigRepoWithAWrongCredential(t *testing.T) {
	e, args := privateSetup(t)
	helper := e.WriteCredentialHelper(t, "operator", "wrong-token")

	res := e.Run(t, append(args, "--git-credential-helper", helper))

	assert.Equal(t, 1, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "config-clone-failed")
	assert.NotContains(t, res.Stdout+res.Stderr, "wrong-token")
	assertNothingLeft(t, e)
}

// TestInitRefusesACredentialHelperGitRunsAsShell: a value of
// tools.gitCredentialHelper that REQ-1267 refuses is refused before any
// write, exit 2: git runs a helper through sh, so a shell snippet (the
// "!" form), a relative path, a path with a space or a shell character,
// and a value with a newline are never handed to it.
func TestInitRefusesACredentialHelperGitRunsAsShell(t *testing.T) {
	for name, value := range map[string]string{
		"a shell snippet":         "!gh auth git-credential",
		"a shell snippet by path": "!/opt/homebrew/bin/gh auth git-credential",
		"a relative path":         "bin/git-credential-x",
		"a path with a space":     "/opt/my helper",
		"a command substitution":  "/bin/$(id)",
		"a newline":               "osxkeychain\nstore",
		"an upper-case name":      "OSXKeychain",
		"a path too long":         "/" + strings.Repeat("a", 4096),
	} {
		t.Run(name, func(t *testing.T) {
			e, args := privateSetup(t)

			res := e.Run(t, append(args, "--git-credential-helper", value))

			assert.Equal(t, 2, res.Code, res.Stderr)
			assert.Contains(t, res.Stderr, "gitCredentialHelper")
			assert.NoFileExists(t, e.Settings())
			assert.NoDirExists(t, e.StateDir())
			assert.NoDirExists(t, e.Root)
		})
	}
}

// TestInitRefusesAHelperOthersCanWrite: a helper script that its group
// can write is refused with exit 2 and the rule, before anything is
// written; git never runs it.
func TestInitRefusesAHelperOthersCanWrite(t *testing.T) {
	e, args := privateSetup(t)
	helper := e.WriteCredentialHelper(t, "operator", "s3cret-token")
	require.NoError(t, os.Chmod(helper, 0o770))

	res := e.Run(t, append(args, "--git-credential-helper", helper))

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "is writable by its group or others")
	assert.NoFileExists(t, e.Settings())
	assert.NoDirExists(t, e.StateDir())
	assert.NoDirExists(t, e.Root)
}
