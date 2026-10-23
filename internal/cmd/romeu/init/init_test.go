// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package initcmd

import (
	"bytes"
	"encoding/pem"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The unit tests run init through cli.Main against a config repo on a
// local TLS server (git http-backend), the way the e2e does with its
// harness, which internal/ cannot import.

const projectSpec = "schema: project/v1\nname: cfg\nrepos:\n  - url: https://github.com/owner/cfg\n    primary: true\n" +
	"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: sbx-kit-claude\n"

// machine is a temporary home, a git that reaches the origin and an sbx
// that exists.
type machine struct {
	home, bin string
	env       map[string]string
	path      map[string]string
}

// newMachine serves a config repo owner/cfg holding spec at
// projects/cfg.yaml (none when spec is empty) and returns the machine.
func newMachine(t *testing.T, spec string) *machine {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git, err := exec.LookPath("git")
	require.NoError(t, err)
	execPath, err := exec.Command(git, "--exec-path").Output()
	require.NoError(t, err)
	bare := filepath.Join(tmp, "origin", "owner", "cfg")
	require.NoError(t, os.MkdirAll(bare, 0o700))
	gitEnv := []string{"HOME=" + tmp, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=f", "GIT_AUTHOR_EMAIL=f@example.com", "GIT_COMMITTER_NAME=f", "GIT_COMMITTER_EMAIL=f@example.com"}
	run := func(dir string, args ...string) {
		c := exec.CommandContext(t.Context(), git, args...)
		c.Dir, c.Env = dir, append(gitEnv, "PATH=/usr/bin:/bin")
		out, err := c.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	work := filepath.Join(tmp, "work")
	require.NoError(t, os.MkdirAll(filepath.Join(work, "projects"), 0o700))
	run(bare, "init", "-q", "--bare", "-b", "main")
	run(work, "init", "-q", "-b", "main")
	if spec != "" {
		require.NoError(t, os.WriteFile(filepath.Join(work, "projects", "cfg.yaml"), []byte(spec), 0o600))
	}
	run(work, "add", "-A")
	run(work, "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "x")
	run(work, "push", "-q", bare, "HEAD:refs/heads/main")

	srv := httptest.NewTLSServer(&cgi.Handler{
		Path: filepath.Join(strings.TrimSpace(string(execPath)), "git-http-backend"), Root: "/",
		Env:        []string{"GIT_PROJECT_ROOT=" + filepath.Join(tmp, "origin"), "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_GLOBAL=/dev/null"},
		InheritEnv: []string{"PATH"},
	})
	t.Cleanup(srv.Close)
	ca := filepath.Join(tmp, "ca.pem")
	require.NoError(t, os.WriteFile(ca, certPEM(srv), 0o600))

	m := &machine{home: filepath.Join(tmp, "home"), bin: filepath.Join(tmp, "bin"), path: map[string]string{}}
	require.NoError(t, os.MkdirAll(m.home, 0o700))
	require.NoError(t, os.MkdirAll(m.bin, 0o700))
	script := "#!/bin/sh\nexec '" + git + "' -c 'url." + srv.URL + "/.insteadOf=https://github.com/' -c 'http.sslCAInfo=" + ca + "' \"$@\"\n"
	m.path["git"] = filepath.Join(m.bin, "git")
	require.NoError(t, os.WriteFile(m.path["git"], []byte(script), 0o700))
	m.path["sbx"] = filepath.Join(m.bin, "sbx")
	require.NoError(t, os.WriteFile(m.path["sbx"], []byte("#!/bin/sh\n"), 0o700))
	m.env = map[string]string{"HOME": m.home, "XDG_CONFIG_HOME": filepath.Join(tmp, "cfg"), "XDG_STATE_HOME": filepath.Join(tmp, "state")}

	oldGetenv, oldLook := getenv, lookPath
	getenv = func(k string) string { return m.env[k] }
	lookPath = func(name string) (string, error) {
		if p, ok := m.path[name]; ok {
			return p, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { getenv, lookPath = oldGetenv, oldLook })
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

// certPEM returns the certificate of srv as a CA bundle.
func certPEM(srv *httptest.Server) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
}
