// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package initcmd

import (
	"bytes"
	"context"
	"errors"
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
	cloneRepo = func(ctx context.Context, _ *gitsafe.Runner, url, dest string) error {
		if strings.HasSuffix(url, "/missing") {
			return errors.New("repository not found")
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
	assert.Contains(t, string(data), "root: "+filepath.Join(m.home, "dev"))
	assert.Contains(t, string(data), "git: "+m.path["git"])
	assert.Contains(t, stdout, "settings  "+m.settings())
	assert.FileExists(t, filepath.Join(m.home, "dev", "cfg-env", "cfg", "projects", "cfg.yaml"))
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
		{"no spec", "", []string{"--config-url", url}, nil, 2, "RJ-346"},
		{"a spec that does not decode", "schema: project/v1\nname: [\n", []string{"--config-url", url}, nil, 2, "does not decode"},
		{"a spec of another project", strings.Replace(projectSpec, "name: cfg", "name: other", 1), []string{"--config-url", url}, nil, 2, "RJ-346"},
		{"the clone exists", projectSpec, []string{"--config-url", url}, func(m *machine) {
			require.NoError(t, os.MkdirAll(filepath.Join(m.home, "dev", "cfg-env", "cfg"), 0o700))
		}, 2, "clone-exists"},
		{"an origin that is not there", projectSpec, []string{"--config-url", "https://github.com/owner/missing"}, nil, 1, "error"},
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
				entries, _ := os.ReadDir(filepath.Join(m.home, "dev", "cfg-env"))
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
	path := filepath.Join(m.home, "s.yaml")
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr), SettingsPath: path}

	code := cli.Main(t.Context(), []string{"init", "--config-url", "https://github.com/owner/cfg"}, env)

	require.Equal(t, 0, code, stderr.String())
	assert.FileExists(t, path)
}

// TestSettingsPath: ROMEU_SETTINGS, then XDG_CONFIG_HOME, then HOME;
// nothing else moves it.
func TestSettingsPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
		err  bool
	}{
		{"override", map[string]string{"ROMEU_SETTINGS": "/a/s.yaml", "XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/a/s.yaml", false},
		{"override is cleaned", map[string]string{"ROMEU_SETTINGS": "/a//b/../s.yaml"}, "/a/s.yaml", false},
		{"override must be absolute", map[string]string{"ROMEU_SETTINGS": "s.yaml", "HOME": "/h"}, "", true},
		{"xdg", map[string]string{"XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/x/romeu/settings.yaml", false},
		{"relative xdg is unset", map[string]string{"XDG_CONFIG_HOME": "x", "HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"home", map[string]string{"HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"other variables", map[string]string{"ROMEU_ROOT": "/r", "ROMEU_CONFIG": "/c", "HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"nothing", map[string]string{}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := settingsPath(func(k string) string { return tc.env[k] })
			if tc.err {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestRealPath resolves the links of the part that exists and keeps the
// rest.
func TestRealPath(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(tmp, "real"), 0o700))
	require.NoError(t, os.Symlink(filepath.Join(tmp, "real"), filepath.Join(tmp, "link")))

	got, err := realPath(filepath.Join(tmp, "link", "a", "b"))

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "real", "a", "b"), got)
	loop := filepath.Join(tmp, "loop")
	require.NoError(t, os.Symlink(loop, loop))
	_, err = realPath(loop)
	assert.Error(t, err)
}

// TestRootProblem names each reason a root is unsafe, and accepts a safe
// one that does not exist yet.
func TestRootProblem(t *testing.T) {
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	home := filepath.Join(tmp, "home")
	settings := filepath.Join(home, ".config", "romeu", "settings.yaml")
	state := filepath.Join(home, ".local", "state", "romeu")
	require.NoError(t, os.MkdirAll(filepath.Join(home, "repo", ".git"), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Join(home, "dev"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "file"), nil, 0o600))

	for _, tc := range []struct{ name, root, want string }{
		{"safe and absent", filepath.Join(home, "new", "dev"), ""},
		{"safe and present", filepath.Join(home, "dev"), ""},
		{"home", home, "$HOME"},
		{"holds the settings", filepath.Join(home, ".config"), "holds the host settings"},
		{"holds the state", filepath.Join(home, ".local"), "holds the host state"},
		{"in a repository", filepath.Join(home, "repo", "dev"), "git repository"},
		{"a repository", filepath.Join(home, "repo"), "git repository"},
		{"a file", filepath.Join(home, "file"), "not a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := rootProblem(tc.root, home, settings, state)
			if tc.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tc.want)
		})
	}
	if os.Geteuid() != 0 {
		assert.Contains(t, rootProblem("/usr", home, settings, state), "not owned")
	}
}
