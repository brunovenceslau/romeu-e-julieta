// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package initcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The unit tests run init through cli.Main with a fixture in place of
// the clone (cloneRepo): the real clone, over TLS, is the e2e's.

const projectSpec = "schema: project/v1\nname: cfg\nrepos:\n  - url: https://github.com/owner/cfg\n    primary: true\n" +
	"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: sbx-kit-claude\n"

// machine is a temporary home, a git and an sbx that exist.
type machine struct {
	home, bin string
	env       map[string]string
	path      map[string]string
	// helpers are the credential helpers the clones were given, one per
	// clone.
	helpers []string
}

// gitIn runs the machine's git in dir.
func gitIn(t *testing.T, git, dir string, args ...string) {
	t.Helper()
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolGit, Argv: append([]string{git}, args...), Dir: dir,
		Env: []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=f", "GIT_AUTHOR_EMAIL=f@example.com", "GIT_COMMITTER_NAME=f", "GIT_COMMITTER_EMAIL=f@example.com"},
	})
	require.NoError(t, err, string(out))
}

// newMachine makes a machine whose clone holds spec at projects/cfg.yaml
// (no file when spec is empty) on the branch main of origin.
func newMachine(t *testing.T, spec string) *machine {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	m := &machine{home: filepath.Join(tmp, "home"), bin: filepath.Join(tmp, "bin"), path: map[string]string{"git": git}}
	require.NoError(t, os.MkdirAll(m.home, 0o700))
	require.NoError(t, os.MkdirAll(m.bin, 0o700))
	m.path["sbx"] = filepath.Join(m.bin, "sbx")
	require.NoError(t, os.WriteFile(m.path["sbx"], []byte("#!/bin/sh\n"), 0o700))
	m.env = map[string]string{"HOME": m.home, "XDG_CONFIG_HOME": filepath.Join(tmp, "cfg"), "XDG_STATE_HOME": filepath.Join(tmp, "state")}

	oldGetenv, oldLook, oldClone := getenv, lookPath, cloneRepo
	getenv = func(k string) string { return m.env[k] }
	lookPath = func(name string) (string, error) {
		if p, ok := m.path[name]; ok {
			return p, nil
		}
		return "", procgroup.ErrNotFound
	}
	cloneRepo = func(ctx context.Context, _ *gitsafe.Runner, url, dest, helper string) error {
		m.helpers = append(m.helpers, helper)
		if strings.HasSuffix(url, "/missing") {
			// As gitsafe returns a git that exited 128.
			return fmt.Errorf("gitsafe: git clone: %w: fatal: repository 'https://github.com/owner/missing/' not found", &procgroup.ExitError{Argv0: "git", Code: 128})
		}
		if strings.HasSuffix(url, "/refused") {
			return fmt.Errorf("%w: a refusal of gitsafe", gitsafe.ErrRefused)
		}
		if strings.HasSuffix(url, "/badhead") {
			// As gitsafe returns a HEAD of the remote it refuses.
			return fmt.Errorf("%w: %w: object id %q", gitsafe.ErrRemoteContent, gitsafe.ErrRefused, "not-an-oid")
		}
		if err := os.MkdirAll(dest, 0o700); err != nil {
			return err
		}
		gitIn(t, git, dest, "init", "-q", "-b", "main")
		if spec != "" {
			if err := os.MkdirAll(filepath.Join(dest, "projects"), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dest, "projects", "cfg.yaml"), []byte(spec), 0o600); err != nil {
				return err
			}
		}
		gitIn(t, git, dest, "add", "-A")
		gitIn(t, git, dest, "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "x")
		gitIn(t, git, dest, "update-ref", "refs/remotes/origin/main", "HEAD")
		gitIn(t, git, dest, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		return nil
	}
	t.Cleanup(func() { getenv, lookPath, cloneRepo = oldGetenv, oldLook, oldClone })
	return m
}

// init runs romeu init with args and returns the exit status and stderr.
func (m *machine) init(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		ULIDs: canon.NewULIDSource(strings.NewReader(strings.Repeat("x", 1024)))}
	code := cli.Main(t.Context(), append([]string{"init"}, args...), env)
	return code, stdout.String(), stderr.String()
}

func (m *machine) settings() string {
	return filepath.Join(m.env["XDG_CONFIG_HOME"], "romeu", "settings.yaml")
}

// TestInitWritesSettingsAndClones is the happy path.
func TestInitWritesSettingsAndClones(t *testing.T) {
	m := newMachine(t, projectSpec)

	code, stdout, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg")

	require.Equal(t, 0, code, stderr)
	data, err := os.ReadFile(m.settings())
	require.NoError(t, err)
	assert.Contains(t, string(data), "root: "+filepath.Join(m.home, "Developer"))
	assert.Contains(t, string(data), "git: "+m.path["git"])
	assert.Contains(t, stdout, "settings  "+m.settings())
	assert.FileExists(t, filepath.Join(m.home, "Developer", "cfg-env", "cfg", "projects", "cfg.yaml"))
}

// TestInitRefusals: each refusal exits with its code and writes nothing.
func TestInitRefusals(t *testing.T) {
	url := "https://github.com/owner/cfg"
	for _, tc := range []struct {
		name     string
		spec     string
		args     []string
		prepare  func(m *machine)
		wantCode int
		want     string
	}{
		{"no url", projectSpec, nil, nil, 2, "RJ-349"},
		{"a plain path", projectSpec, []string{"--config-url", "/srv/cfg"}, nil, 2, "RJ-349"},
		{"a bad project name", projectSpec, []string{"--config-url", url, "--config-project", "Bad Name"}, nil, 2, "invalid"},
		{"a relative root", projectSpec, []string{"--config-url", url, "--root", "dev"}, nil, 2, "root-unsafe"},
		{"HOME is the root", projectSpec, []string{"--config-url", url, "--root", "@home"}, nil, 2, "root-unsafe"},
		{"HOME not absolute", projectSpec, []string{"--config-url", url}, func(m *machine) { m.env["HOME"] = "rel" }, 2, "HOME is not an absolute"},
		{"settings exist", projectSpec, []string{"--config-url", url}, func(m *machine) {
			require.NoError(t, os.MkdirAll(filepath.Dir(m.settings()), 0o700))
			require.NoError(t, os.WriteFile(m.settings(), []byte("x"), 0o600))
		}, 2, "settings-exist"},
		{"no sbx", projectSpec, []string{"--config-url", url}, func(m *machine) { delete(m.path, "sbx") }, 2, "sbx is not on PATH"},
		{"no git", projectSpec, []string{"--config-url", url}, func(m *machine) { delete(m.path, "git") }, 2, "git is not on PATH"},
		{"git below the floor", projectSpec, []string{"--config-url", url}, func(m *machine) {
			m.path["git"] = filepath.Join(m.bin, "oldgit")
			require.NoError(t, os.WriteFile(m.path["git"], []byte("#!/bin/sh\necho git version 2.0.0\n"), 0o700))
		}, 2, "git-floor"},
		{"a ref that is not the default branch", projectSpec, []string{"--config-url", url, "--config-ref", "dev"}, nil, 2, "config-ref-not-default"},
		{"no spec", "", []string{"--config-url", url}, nil, 2, "RJ-346"},
		{"a spec that does not decode", "schema: project/v1\nname: [\n", []string{"--config-url", url}, nil, 2, "does not decode"},
		{"a spec of another project", strings.Replace(projectSpec, "name: cfg", "name: other", 1), []string{"--config-url", url}, nil, 2, "RJ-346"},
		{"the clone exists", projectSpec, []string{"--config-url", url}, func(m *machine) {
			require.NoError(t, os.MkdirAll(filepath.Join(m.home, "Developer", "cfg-env", "cfg"), 0o700))
		}, 2, "clone-exists"},
		{"an origin that is not there", projectSpec, []string{"--config-url", "https://github.com/owner/missing"}, nil, 1, "RJ-356 config-clone-failed"},
		{"a credential helper git runs as shell", projectSpec, []string{"--config-url", url, "--git-credential-helper", "!gh auth git-credential"}, nil, 2, "tools.gitCredentialHelper"},
		{"a credential helper over the cap", projectSpec, []string{"--config-url", url, "--git-credential-helper", "/" + strings.Repeat("a", gitsafe.MaxCredentialHelper)}, nil, 2, "over the cap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newMachine(t, tc.spec)
			if tc.prepare != nil {
				tc.prepare(m)
			}
			args := append([]string(nil), tc.args...)
			for i, a := range args {
				if a == "@home" {
					args[i] = m.home
				}
			}
			had := fileExists(m.settings())

			code, _, stderr := m.init(t, args...)

			assert.Equal(t, tc.wantCode, code, stderr)
			assert.Contains(t, stderr, tc.want)
			assert.Equal(t, had, fileExists(m.settings()), "the settings are as they were")
			if tc.name != "the clone exists" {
				entries, _ := os.ReadDir(filepath.Join(m.home, "Developer", "cfg-env"))
				for _, e := range entries {
					assert.NotEqual(t, "cfg", e.Name())
					assert.False(t, strings.HasPrefix(e.Name(), ".init-"), "no staging directory is left")
				}
			}
		})
	}
}

func fileExists(p string) bool { _, err := os.Lstat(p); return err == nil }

// TestInitUsesEnvSettingsPath: Env.SettingsPath, when the binary sets
// it, wins over the environment.
func TestInitUsesEnvSettingsPath(t *testing.T) {
	m := newMachine(t, projectSpec)
	path := filepath.Join(filepath.Dir(m.home), "elsewhere", "s.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), SettingsPath: path}

	code := cli.Main(t.Context(), []string{"init", "--config-url", "https://github.com/owner/cfg"}, env)

	require.Equal(t, 0, code, stderr.String())
	assert.FileExists(t, path)
}

// TestInitSettingsDirUnwritable: a settings directory init cannot write
// leaves no clone and no settings, is a precondition (exit 2,
// settings-unwritable) and not an internal error, and the retry after the
// mode is fixed succeeds.
func TestInitSettingsDirUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		return // root writes into a 0500 directory
	}
	m := newMachine(t, projectSpec)
	dir := filepath.Dir(m.settings())
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	args := []string{"--config-url", "https://github.com/owner/cfg"}

	code, _, stderr := m.init(t, args...)

	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "RJ-354")
	assert.Contains(t, stderr, "settings-unwritable")
	assert.NotContains(t, stderr, "RJ-348")
	assert.NoFileExists(t, m.settings())
	assert.NoDirExists(t, filepath.Join(m.home, "Developer", "cfg-env", "cfg"))
	entries, err := os.ReadDir(filepath.Join(m.home, "Developer", "cfg-env"))
	require.NoError(t, err)
	assert.Empty(t, entries, "no clone and no staging directory")

	require.NoError(t, os.Chmod(dir, 0o700))
	code, _, stderr = m.init(t, args...)

	require.Equal(t, 0, code, stderr)
	assert.FileExists(t, m.settings())
	assert.DirExists(t, filepath.Join(m.home, "Developer", "cfg-env", "cfg", ".git"))
}

// TestInitSettingsAppearAfterTheClone: a file that appears at the
// settings path once the clone is in place is not replaced: init exits 2
// with settings-exist, leaves that file as it was, and removes the clone
// this run made.
func TestInitSettingsAppearAfterTheClone(t *testing.T) {
	m := newMachine(t, projectSpec)
	clone := cloneRepo
	cloneRepo = func(ctx context.Context, g *gitsafe.Runner, url, dest, helper string) error {
		if err := clone(ctx, g, url, dest, helper); err != nil {
			return err
		}
		return os.WriteFile(m.settings(), []byte("theirs"), 0o600)
	}

	code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg")

	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "RJ-353")
	assert.Contains(t, stderr, "settings-exist")
	got, err := os.ReadFile(m.settings())
	require.NoError(t, err)
	assert.Equal(t, "theirs", string(got))
	assert.NoDirExists(t, filepath.Join(m.home, "Developer", "cfg-env", "cfg"))
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(m.settings()), ".settings.yaml*"))
	assert.Empty(t, left, "the temporary file is removed")
}

// TestInitNoDefaultBranch: a remote whose HEAD names no branch fails
// closed, whatever ref is asked.
func TestInitNoDefaultBranch(t *testing.T) {
	m := newMachine(t, projectSpec)
	clone := cloneRepo
	cloneRepo = func(ctx context.Context, g *gitsafe.Runner, url, dest, helper string) error {
		if err := clone(ctx, g, url, dest, helper); err != nil {
			return err
		}
		return os.Remove(filepath.Join(dest, ".git", "refs", "remotes", "origin", "HEAD"))
	}

	code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg")

	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "no default branch")
	assert.NoFileExists(t, m.settings())
}

// TestInitRootWithTrailingSlash: a trailing slash is cleaned, not refused.
func TestInitRootWithTrailingSlash(t *testing.T) {
	m := newMachine(t, projectSpec)
	root := filepath.Join(m.home, "tree") + "/"

	code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg", "--root", root)

	require.Equal(t, 0, code, stderr)
	assert.DirExists(t, filepath.Join(m.home, "tree", "cfg-env", "cfg", ".git"))
}

// TestInitCloneErrors: a git clone that exits non-zero is
// config-clone-failed (RJ-356), exit 1, with git's output in the details
// and the flag in the hint; any other error of the clone is not, so a
// defect still reads as one.
func TestInitCloneErrors(t *testing.T) {
	t.Run("git exited non-zero", func(t *testing.T) {
		m := newMachine(t, projectSpec)

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/missing")

		assert.Equal(t, 1, code, stderr)
		assert.Contains(t, stderr, "RJ-356 config-clone-failed")
		assert.Contains(t, stderr, "repository 'https://github.com/owner/missing/' not found")
		assert.Contains(t, stderr, "a private config repo needs --git-credential-helper")
		assert.NotContains(t, stderr, "RJ-348")
	})
	t.Run("a refusal of gitsafe is usage, not internal", func(t *testing.T) {
		m := newMachine(t, projectSpec)

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/refused")

		assert.Equal(t, 2, code, stderr)
		assert.Contains(t, stderr, "RJ-349")
		assert.Contains(t, stderr, "a refusal of gitsafe")
		assert.NotContains(t, stderr, "RJ-348")
		assert.NotContains(t, stderr, "config-clone-failed")
	})
	t.Run("a refusal of what the remote sent is config-clone-failed", func(t *testing.T) {
		m := newMachine(t, projectSpec)

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/badhead")

		assert.Equal(t, 1, code, stderr)
		assert.Contains(t, stderr, "RJ-356 config-clone-failed")
		assert.Contains(t, stderr, `object id "not-an-oid"`, "the reason is in the details")
		assert.NotContains(t, stderr, "RJ-349")
		assert.NoDirExists(t, filepath.Join(m.home, "Developer"))
	})
	t.Run("the levels above the root stay, the root this run made goes", func(t *testing.T) {
		m := newMachine(t, projectSpec)
		root := filepath.Join(m.home, "a", "b", "dev")

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/missing", "--root", root)

		assert.Equal(t, 1, code, stderr)
		assert.NoDirExists(t, root)
		assert.DirExists(t, filepath.Dir(root))
	})
	t.Run("the root and the env dir this run made are removed", func(t *testing.T) {
		m := newMachine(t, projectSpec)

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/missing")

		assert.Equal(t, 1, code, stderr)
		assert.NoDirExists(t, filepath.Join(m.home, "Developer"))
	})
	t.Run("a root that was there stays, and so does a non-empty env dir", func(t *testing.T) {
		m := newMachine(t, projectSpec)
		keep := filepath.Join(m.home, "Developer", "cfg-env", "keep")
		require.NoError(t, os.MkdirAll(keep, 0o700))

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/missing")

		assert.Equal(t, 1, code, stderr)
		assert.DirExists(t, keep)
	})
	t.Run("an empty root that was there stays", func(t *testing.T) {
		m := newMachine(t, projectSpec)
		require.NoError(t, os.Mkdir(filepath.Join(m.home, "Developer"), 0o700))

		code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/missing")

		assert.Equal(t, 1, code, stderr)
		assert.DirExists(t, filepath.Join(m.home, "Developer"))
		assert.NoDirExists(t, filepath.Join(m.home, "Developer", "cfg-env"))
	})
}

// TestInitRecordsTheCredentialHelper: the helper of the flag reaches
// the clone and the settings as tools.gitCredentialHelper; without the
// flag the clone has none and the settings name none.
func TestInitRecordsTheCredentialHelper(t *testing.T) {
	for name, helper := range map[string]string{"none": "", "a name": "osxkeychain", "a path": "@script"} {
		t.Run(name, func(t *testing.T) {
			m := newMachine(t, projectSpec)
			if helper == "@script" {
				helper = m.helperScript(t, 0o700)
			}
			args := []string{"--config-url", "https://github.com/owner/cfg"}
			if helper != "" {
				args = append(args, "--git-credential-helper", helper)
			}

			code, _, stderr := m.init(t, args...)

			require.Equal(t, 0, code, stderr)
			assert.Equal(t, []string{helper}, m.helpers)
			data, err := os.ReadFile(m.settings())
			require.NoError(t, err)
			if helper == "" {
				assert.NotContains(t, string(data), "gitCredentialHelper")
			} else {
				assert.Contains(t, string(data), "gitCredentialHelper: "+helper)
			}
		})
	}
}

// helperScript writes an executable credential helper of the given mode
// in the machine's bin and returns its path.
func (m *machine) helperScript(t *testing.T, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(m.bin, "git-credential-test")
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700))
	require.NoError(t, os.Chmod(p, mode))
	return p
}

// TestInitRefusesAHelperOthersCanWrite: an absolute-path helper that is
// not a regular executable file of the user's or root's, or that sits in
// a directory others can write, is a usage error naming the rule, before
// anything is written and before any clone.
func TestInitRefusesAHelperOthersCanWrite(t *testing.T) {
	for _, tc := range []struct {
		name, rule string
		helper     func(m *machine) string
	}{
		{"writable by the group", "is writable by its group or others; fix it with: chmod g-w ", func(m *machine) string { return m.helperScript(t, 0o770) }},
		{"not executable", "is not executable", func(m *machine) string { return m.helperScript(t, 0o600) }},
		{"missing", "cannot be resolved", func(m *machine) string { return filepath.Join(m.bin, "none") }},
		{"in a directory others can write", "is writable by its group or others", func(m *machine) string {
			p := m.helperScript(t, 0o700)
			require.NoError(t, os.Chmod(m.bin, 0o777))
			return p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newMachine(t, projectSpec)
			helper := tc.helper(m)

			code, _, stderr := m.init(t, "--config-url", "https://github.com/owner/cfg", "--git-credential-helper", helper)

			assert.Equal(t, 2, code, stderr)
			assert.Contains(t, stderr, "RJ-349")
			assert.Contains(t, stderr, tc.rule)
			assert.Empty(t, m.helpers, "no clone ran")
			assert.NoFileExists(t, m.settings())
			assert.NoDirExists(t, filepath.Join(m.home, "Developer"))
			assert.NoDirExists(t, m.env["XDG_STATE_HOME"])
		})
	}
}
