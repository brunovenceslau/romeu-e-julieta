// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bufio"
	"context"
	"encoding/pem"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// hostFlags is the flag list of host mode, as 05 5.1 writes it.
var hostFlags = []string{
	"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "submodule.recurse=false",
	"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "protocol.git.allow=always",
	"-c", "core.sshCommand=false", "-c", "core.gitProxy=", "-c", "core.askPass=", "-c", "core.alternateRefsCommand=", "-c", "diff.external=", "-c", "core.pager=cat", "-c", "credential.helper=",
	"-c", "fetch.prune=false", "-c", "transfer.fsckObjects=true", "-c", "transfer.bundleURI=false", "-c", "fetch.bundleURI=",
	"-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "core.symlinks=false",
	"-c", "advice.detachedHead=false",
}

// sandboxFlags is the flag list of sandbox mode (05 5.1).
var sandboxFlags = []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "submodule.recurse=false", "-c", "fetch.bundleURI="}

// ambient are the variables of I3 a hostile environment sets; none may
// reach git.
var ambient = map[string]string{
	"GIT_EXTERNAL_DIFF":  "/bin/false",
	"GIT_CONFIG_COUNT":   "1",
	"GIT_CONFIG_KEY_0":   "core.hooksPath",
	"GIT_CONFIG_VALUE_0": "/tmp/hooks",
	"GIT_SSH_COMMAND":    "/bin/false",
	"GIT_ASKPASS":        "/bin/false",
	"GIT_DIR":            "/tmp/elsewhere",
	"GIT_EXEC_PATH":      "/tmp/exec",
	"GIT_PROXY_COMMAND":  "/tmp/proxy-command",
	"GIT_TEMPLATE_DIR":   "/tmp/templates",
	"GIT_SSL_CAINFO":     "/tmp/git-ca.pem",
	"ALL_PROXY":          "http://all.example:3128",
	"CURL_CA_BUNDLE":     "/tmp/curl-ca.pem",
	"SSL_CERT_DIR":       "/tmp/certs",
	"LD_PRELOAD":         "/tmp/preload.so",
	"ROMEU_PLANTED":      "yes",
}

// network are the variables of the network pass-through list, with the
// values a sandbox sets; sandbox mode passes each, host mode none.
var network = map[string]string{
	"HTTPS_PROXY":   "http://proxy.example:3128",
	"HTTP_PROXY":    "http://proxy.example:3129",
	"NO_PROXY":      "localhost,127.0.0.1",
	"https_proxy":   "http://lower.example:3128",
	"http_proxy":    "http://lower.example:3129",
	"no_proxy":      "localhost,::1",
	"SSL_CERT_FILE": "/etc/proxy/ca.pem",
}

// TestRunnerEnvironmentAndFlags is the one table test that pins the
// environment and the flags of each mode (05 5.1), on each invocation
// of each verb, with a hostile environment around the process: nothing
// ambient passes through.
//
//romeu:invariant I3 test
func TestRunnerEnvironmentAndFlags(t *testing.T) {
	for k, v := range ambient {
		t.Setenv(k, v)
	}
	for k, v := range network {
		t.Setenv(k, v)
	}
	const git, home = "/usr/bin/git", "/home-of-test"
	dir := stubRepo
	oid := strings.Repeat("a", 40)
	tests := []struct {
		name  string
		mode  Mode
		class procgroup.Class
		env   []string
		flags []string
	}{
		{
			name:  "host",
			mode:  Host,
			class: procgroup.HostGitFetch,
			env: []string{
				"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
				"GIT_ATTR_NOSYSTEM=1", "GIT_NO_LAZY_FETCH=1", "PATH=/usr/bin:/bin", "HOME=" + home, "LANG=C",
			},
			flags: hostFlags,
		},
		{
			name:  "sandbox",
			mode:  Sandbox,
			class: procgroup.JulietaGit,
			env: []string{
				"GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "PATH=/usr/bin:/bin", "HOME=" + home, "LANG=C",
				"HTTPS_PROXY=http://proxy.example:3128", "HTTP_PROXY=http://proxy.example:3129",
				"NO_PROXY=localhost,127.0.0.1", "https_proxy=http://lower.example:3128",
				"http_proxy=http://lower.example:3129", "no_proxy=localhost,::1",
				"SSL_CERT_FILE=/etc/proxy/ca.pem",
			},
			flags: sandboxFlags,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(Config{Mode: tt.mode, Git: git, Home: home})
			require.NoError(t, err)
			var rec recorder
			rec.install(r)
			ctx := t.Context()
			require.NoError(t, r.CheckFloor(ctx))
			dst := map[Mode]string{Host: "refs/romeu/origin/app/*", Sandbox: "refs/remotes/origin/*"}[tt.mode]
			require.NoError(t, r.Fetch(ctx, dir, "https://example.com/o/r.git", []string{"+refs/heads/*:" + dst}))
			created := map[Mode]string{Host: "refs/romeu/base/shop/g1/app", Sandbox: "refs/salvage/g1/s1/stash-0"}[tt.mode]
			require.NoError(t, r.CreateRef(ctx, dir, created, oid))
			require.NoError(t, r.BundleCreate(ctx, dir, "/out/b.bundle", []string{"refs/heads/main"}, []string{oid}))
			if tt.mode == Sandbox {
				require.NoError(t, r.FastForward(ctx, dir, oid))
			}
			require.NoError(t, r.Clone(ctx, "https://example.com/o/r.git", "/nonexistent-gitsafe-test/r"))
			require.NotEmpty(t, rec.commands)
			for _, c := range rec.commands {
				assert.Equal(t, tt.class, c.Class, "%v", c.Argv)
				env := tt.env
				if slices.Equal(c.Argv[len(c.Argv)-5:], []string{"config", "-z", "--name-only", "--get-regexp", `^filter\.`}) {
					// The listing of the filter drivers is a read.
					env = slices.Concat(tt.env, statusEnv)
				}
				assert.Equal(t, env, c.Env, "%v", c.Argv)
				require.Greater(t, len(c.Argv), len(tt.flags))
				assert.Equal(t, git, c.Argv[0])
				assert.Equal(t, tt.flags, c.Argv[1:1+len(tt.flags)], "%v", c.Argv)
				for _, a := range c.Argv {
					assert.NotContains(t, a, "/tmp/", "an ambient value reached the argv")
				}
			}
		})
	}
}

// TestNeutralizedKeys holds the exported list equal to the keys of the
// host flags: the one Go slice that doctor's host-git-config check and
// the I3 plant list read (10 10.1, Git keys).
func TestNeutralizedKeys(t *testing.T) {
	var keys []string
	for i := 1; i < len(hostFlags); i += 2 {
		key, _, ok := strings.Cut(hostFlags[i], "=")
		require.True(t, ok)
		keys = append(keys, key)
	}
	assert.Equal(t, keys, NeutralizedKeys())
	got := NeutralizedKeys()
	got[0] = "changed"
	assert.Equal(t, keys, NeutralizedKeys(), "a caller cannot change the list")
}

// TestNetworkEnv pins the network pass-through list of sandbox mode:
// the proxy variables in both cases and the CA bundle of the proxy,
// and never a GIT_* variable.
func TestNetworkEnv(t *testing.T) {
	want := []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}
	assert.Equal(t, want, NetworkEnv())
	for _, k := range NetworkEnv() {
		assert.False(t, strings.HasPrefix(strings.ToUpper(k), "GIT_"), k)
		assert.Contains(t, network, k, "the plant of the env table covers %s", k)
	}
	got := NetworkEnv()
	got[0] = "changed"
	assert.Equal(t, want, NetworkEnv(), "a caller cannot change the list")
}

// TestNetworkEnvUnset: sandbox mode passes a variable of the list only
// when the process has it set and not empty (REQ-0470).
func TestNetworkEnvUnset(t *testing.T) {
	for _, k := range NetworkEnv() {
		t.Setenv(k, "x")
		require.NoError(t, os.Unsetenv(k))
	}
	t.Setenv("no_proxy", "")
	r, err := New(Config{Mode: Sandbox, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	assert.Equal(t, []string{"GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "PATH=/usr/bin:/bin", "HOME=/h", "LANG=C"}, r.env)
}

// TestSandboxGitUsesTheProxy: a sandbox-mode fetch over HTTPS goes
// through the proxy the process names, and one to a NO_PROXY host does
// not; a host-mode fetch never does. The proxy is a listener that
// records the first line of each connection and closes it.
func TestSandboxGitUsesTheProxy(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
		noProxy string
		proxied bool
	}{
		{"sandbox", Sandbox, "localhost", true},
		{"sandbox, a NO_PROXY host", Sandbox, "unroutable.invalid", false},
		{"host", Host, "localhost", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := make(chan string, 8)
			proxy := listen(t, func(_ context.Context, conn net.Conn) {
				line, _ := bufio.NewReader(conn).ReadString('\n')
				lines <- strings.TrimSpace(line)
			})
			addr := strings.TrimSuffix(strings.TrimPrefix(proxy, "git://"), "/")
			for _, k := range NetworkEnv() {
				t.Setenv(k, "")
			}
			t.Setenv("https_proxy", "http://"+addr)
			t.Setenv("HTTPS_PROXY", "http://"+addr)
			t.Setenv("no_proxy", tt.noProxy)
			t.Setenv("NO_PROXY", tt.noProxy)
			dir, _ := newRepo(t)
			dst := map[Mode]string{Host: "refs/romeu/origin/r/*", Sandbox: "refs/remotes/origin/*"}[tt.mode]
			err := newRunner(t, tt.mode).Fetch(t.Context(), dir, "https://unroutable.invalid/r.git", []string{"+refs/heads/*:" + dst})
			require.Error(t, err, "nothing serves the repository")
			if !tt.proxied {
				assert.Empty(t, lines, "git went to the proxy")
				return
			}
			require.NotEmpty(t, lines, "git did not go to the proxy: %v", err)
			assert.Equal(t, "CONNECT unroutable.invalid:443 HTTP/1.1", <-lines)
		})
	}
}

// TestCredentialFlags covers the credentials of 05 5.1: the empty
// helper resets the list, the configured helper follows it in host
// mode only, and a caFile reaches its own host only.
func TestCredentialFlags(t *testing.T) {
	const caFile = "/etc/ca/private.pem"
	r, err := New(Config{
		Mode:             Host,
		Git:              "/usr/bin/git",
		Home:             "/h",
		CredentialHelper: "osxkeychain",
		GitHosts:         []GitHost{{Host: "git.example.com", CAFile: caFile}, {Host: "github.com"}},
	})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	ctx := t.Context()
	require.NoError(t, r.Fetch(ctx, stubRepo, "https://GIT.example.com/o/r.git", []string{"refs/heads/main:refs/romeu/origin/r/main"}))
	require.NoError(t, r.Fetch(ctx, stubRepo, "https://github.com/o/r.git", []string{"refs/heads/main:refs/romeu/origin/r/main"}))
	require.NoError(t, r.CreateRef(ctx, stubRepo, "refs/romeu/salvage/shop/g/s/r/main", strings.Repeat("b", 40)))
	for _, c := range rec.commands {
		args := strings.Join(c.Argv, " ")
		assert.Contains(t, args, "-c credential.helper= -c credential.helper=osxkeychain -c fetch.prune=false",
			"the empty value resets the list, then the one helper: %s", args)
		assert.Equal(t, 1, strings.Count(args, "credential.helper=osxkeychain"))
		ca := "-c http.https://git.example.com/.sslCAInfo=" + caFile
		if strings.Contains(args, "https://GIT.example.com/") {
			assert.Contains(t, args, ca)
		} else {
			assert.NotContains(t, args, "sslCAInfo", "a caFile reaches its own host only")
		}
	}

	_, err = New(Config{Mode: Sandbox, Git: "/usr/bin/git", Home: "/h", CredentialHelper: "osxkeychain"})
	require.ErrorIs(t, err, ErrConfig, "the sandbox has no host credentials")
	_, err = New(Config{Mode: Sandbox, Git: "/usr/bin/git", Home: "/h", GitHosts: []GitHost{{Host: "h"}}})
	require.ErrorIs(t, err, ErrConfig)
}

// TestNew refuses a configuration gitsafe cannot run hardened.
func TestNew(t *testing.T) {
	ok := Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"}
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{"no mode", func(c *Config) { c.Mode = 0 }},
		{"unknown mode", func(c *Config) { c.Mode = 9 }},
		{"relative git", func(c *Config) { c.Git = "git" }},
		{"relative home", func(c *Config) { c.Home = "h" }},
		{"empty home", func(c *Config) { c.Home = "" }},
		{"shell helper", func(c *Config) { c.CredentialHelper = "!f() { cat; }; f" }},
		{"helper with a space", func(c *Config) { c.CredentialHelper = "/opt/my helper" }},
		{"helper option", func(c *Config) { c.CredentialHelper = "-x" }},
		{"relative ca file", func(c *Config) { c.GitHosts = []GitHost{{Host: "h.example", CAFile: "ca.pem"}} }},
		{"ca file with a newline", func(c *Config) { c.GitHosts = []GitHost{{Host: "h.example", CAFile: "/ca\n.pem"}} }},
		{"host with a path", func(c *Config) { c.GitHosts = []GitHost{{Host: "h.example/x"}} }},
		{"empty host", func(c *Config) { c.GitHosts = []GitHost{{Host: ""}} }},
		{"host with userinfo", func(c *Config) { c.GitHosts = []GitHost{{Host: "u@h.example"}} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ok
			tt.change(&c)
			_, err := New(c)
			require.ErrorIs(t, err, ErrConfig)
		})
	}
	for _, helper := range []string{"osxkeychain", "store", "/usr/local/bin/git-credential-manager"} {
		c := ok
		c.CredentialHelper = helper
		c.GitHosts = []GitHost{{Host: "127.0.0.1:8443", CAFile: "/ca.pem"}}
		_, err := New(c)
		require.NoError(t, err, helper)
	}
	assert.Equal(t, "host", Host.String())
	assert.Equal(t, "sandbox", Sandbox.String())
	assert.Equal(t, "mode-9", Mode(9).String())
}

// TestRunRefusesPush proves the one runner builds no push argv (I18):
// no verb names push, and the runner refuses it if one ever did.
//
//romeu:invariant I18 test
func TestRunRefusesPush(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		r, err := New(Config{Mode: mode, Git: "/usr/bin/git", Home: "/h"})
		require.NoError(t, err)
		var rec recorder
		rec.install(r)
		for _, sub := range []string{"push", "send-pack"} {
			err := r.run(t.Context(), stubRepo, nil, sub, "https://example.com/o/r.git")
			require.ErrorIs(t, err, ErrRefused, sub)
		}
		require.ErrorIs(t, r.run(t.Context(), stubRepo, nil), ErrRefused, "no subcommand")
		assert.Empty(t, rec.commands, "nothing ran")
	}
}

// TestSubprocessTimeout proves that subprocess-timeout (RJ-323) fires
// through the seam of the timeout table, under the class of each mode.
func TestSubprocessTimeout(t *testing.T) {
	url := serveSilence(t) + "repo.git"
	for _, tt := range []struct {
		mode  Mode
		class procgroup.Class
	}{{Host, procgroup.HostGitFetch}, {Sandbox, procgroup.JulietaGit}} {
		t.Run(tt.mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			r := newRunner(t, tt.mode)
			ctx := procgroup.WithTestTimeout(t.Context(), tt.class, 500*time.Millisecond)
			dst := map[Mode]string{Host: "refs/sandboxes/shop/*", Sandbox: "refs/remotes/origin/*"}[tt.mode]
			err := r.Fetch(ctx, dir, url, []string{"+refs/heads/*:" + dst})
			timeout, ok := errors.AsType[*procgroup.TimeoutError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, "RJ-323", timeout.ID())
			assert.Equal(t, tt.class, timeout.Class)
		})
	}
}

// TestHooksNeverRun plants a hook through the repository's local
// config and through the process environment; no gitsafe call runs it,
// and the same plant under plain git does (the positive control, 05
// 5.2).
//
//romeu:invariant I3 test
func TestHooksNeverRun(t *testing.T) {
	src, head := newRepo(t)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			hooks := t.TempDir()
			marker := filepath.Join(t.TempDir(), "marker")
			hook := "#!/bin/sh\necho ran >> '" + marker + "'\n"
			require.NoError(t, os.WriteFile(filepath.Join(hooks, "reference-transaction"), []byte(hook), 0o700))
			plainGit(t, dir, "config", "core.hooksPath", hooks)
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
			t.Setenv("GIT_CONFIG_VALUE_0", hooks)

			r := newRunner(t, mode)
			dst := map[Mode]string{Host: "refs/sandboxes/shop/*", Sandbox: "refs/remotes/origin/*"}[mode]
			require.NoError(t, r.Fetch(t.Context(), dir, url, []string{"+refs/heads/*:" + dst}))
			created := map[Mode]string{Host: "refs/romeu/salvage/shop/g/s/r/main", Sandbox: "refs/salvage/g/s/stash-0"}[mode]
			require.NoError(t, r.CreateRef(t.Context(), dir, created, head))
			require.NoError(t, r.BundleCreate(t.Context(), dir, filepath.Join(t.TempDir(), "b"), []string{created}, nil))
			assert.NoFileExists(t, marker)

			plainGit(t, dir, "update-ref", "refs/control/x", head)
			assert.FileExists(t, marker, "the plant is live: plain git runs it")
		})
	}
}

// TestNothingAmbientReachesGit plants a config key through the process
// environment and reads it back through the runner: git never sees it,
// and plain git with the same environment does.
//
//romeu:invariant I3 test
func TestNothingAmbientReachesGit(t *testing.T) {
	dir, _ := newRepo(t)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "romeu.planted")
	t.Setenv("GIT_CONFIG_VALUE_0", "yes")
	for _, mode := range []Mode{Host, Sandbox} {
		var out strings.Builder
		err := newRunner(t, mode).run(t.Context(), dir, &out, "config", "--get", "romeu.planted")
		require.Error(t, err, "config --get exits 1 on a missing key")
		assert.Empty(t, out.String())
	}
	got, err := plainGitErr(t, dir, []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=romeu.planted", "GIT_CONFIG_VALUE_0=yes"},
		"config", "--get", "romeu.planted")
	require.NoError(t, err)
	assert.Equal(t, "yes", got, "the plant is live")
}

// TestRunError names the subcommand and keeps git's own message.
func TestRunError(t *testing.T) {
	dir := t.TempDir()
	err := newRunner(t, Host).run(t.Context(), dir, nil, "rev-parse", "--verify", "refs/heads/none")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gitsafe: git rev-parse")
	assert.Contains(t, err.Error(), "not a git repository")
	_, ok := errors.AsType[*procgroup.ExitError](err)
	assert.True(t, ok)
}

// TestLimitedBuffer keeps the first bytes of a long output and drops
// the rest without failing the writer.
func TestLimitedBuffer(t *testing.T) {
	b := limitedBuffer{limit: 4}
	n, err := b.Write([]byte("abc"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	n, err = b.Write([]byte("defg"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "abcd", b.String())
	assert.True(t, b.truncated)
}

// TestVerbsRefuseADirThatIsNotARepository: a verb given a directory
// inside a repository, or one that does not exist, is refused before
// git starts, so git never walks up and acts on the enclosing
// repository.
func TestVerbsRefuseADirThatIsNotARepository(t *testing.T) {
	outer, head := newRepo(t)
	inner := filepath.Join(outer, "sub")
	require.NoError(t, os.Mkdir(inner, 0o700))
	missing := filepath.Join(t.TempDir(), "missing")
	url := serveGit(t, bareCopy(t, outer, "repo.git")) + "repo.git"
	for _, dir := range []string{inner, missing, "relative"} {
		for _, mode := range []Mode{Host, Sandbox} {
			r := newRunner(t, mode)
			ctx := t.Context()
			dst := map[Mode]string{Host: "refs/romeu/origin/r/*", Sandbox: "refs/remotes/origin/*"}[mode]
			created := map[Mode]string{Host: "refs/romeu/base/shop/g1/app", Sandbox: "refs/salvage/g1/s1/stash-0"}[mode]
			require.ErrorIs(t, r.Fetch(ctx, dir, url, []string{"+refs/heads/*:" + dst}), ErrRefused, "%s %s", mode, dir)
			require.ErrorIs(t, r.CreateRef(ctx, dir, created, head), ErrRefused, "%s %s", mode, dir)
			require.ErrorIs(t, r.BundleCreate(ctx, dir, filepath.Join(t.TempDir(), "b"), []string{"refs/heads/main"}, nil), ErrRefused, "%s %s", mode, dir)
			if mode == Sandbox {
				require.ErrorIs(t, r.FastForward(ctx, dir, head), ErrRefused, "%s %s", dir, mode)
			}
		}
	}
	assert.Empty(t, plainGit(t, outer, "for-each-ref", "refs/romeu", "refs/remotes", "refs/salvage"))
}

// TestConfigProgramsNeverRun plants, in a host clone's local config,
// the keys that make git run a program on a fetch, core.askPass on a
// 401 and core.alternateRefsCommand with an alternate: host mode runs
// neither, and plain git with the same config runs each (the positive
// control).
//
//romeu:invariant I3 test
func TestConfigProgramsNeverRun(t *testing.T) {
	marker := func(t *testing.T) (path, script string) {
		t.Helper()
		path = filepath.Join(t.TempDir(), "marker")
		script = filepath.Join(t.TempDir(), "plant.sh")
		require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+path+"'\n"), 0o700))
		return path, script
	}
	t.Run("core.askPass", func(t *testing.T) {
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("WWW-Authenticate", `Basic realm="r"`)
			w.WriteHeader(http.StatusUnauthorized)
		}))
		srv.Config.ErrorLog = slog.NewLogLogger(slog.DiscardHandler, slog.LevelError)
		srv.StartTLS()
		t.Cleanup(srv.Close)
		caFile := filepath.Join(t.TempDir(), "ca.pem")
		require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600))
		host := srv.Listener.Addr().String()
		url := "https://" + host + "/r.git"
		dir, _ := newRepo(t)
		path, script := marker(t)
		plainGit(t, dir, "config", "core.askPass", script)
		r, err := New(Config{Mode: Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: []GitHost{{Host: host, CAFile: caFile}}})
		require.NoError(t, err)
		require.Error(t, r.Fetch(t.Context(), dir, url, []string{"+refs/heads/*:refs/romeu/origin/r/*"}))
		assert.NoFileExists(t, path)

		_, err = plainGitErr(t, dir, []string{"GIT_TERMINAL_PROMPT=0"}, "-c", "http.sslCAInfo="+caFile, "fetch", url)
		require.Error(t, err)
		assert.FileExists(t, path, "the plant is live: plain git runs it")
	})
	t.Run("core.alternateRefsCommand", func(t *testing.T) {
		src, _ := newRepo(t)
		url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
		other, _ := newRepo(t)
		dir, _ := newRepo(t)
		alternates := filepath.Join(dir, ".git", "objects", "info", "alternates")
		require.NoError(t, os.WriteFile(alternates, []byte(filepath.Join(other, ".git", "objects")+"\n"), 0o600))
		path, script := marker(t)
		plainGit(t, dir, "config", "core.alternateRefsCommand", script)
		require.NoError(t, newRunner(t, Host).Fetch(t.Context(), dir, url, []string{"+refs/heads/*:refs/romeu/origin/r/*"}))
		assert.NoFileExists(t, path)

		plainGit(t, dir, "fetch", url, "+refs/heads/*:refs/control/*")
		assert.FileExists(t, path, "the plant is live: plain git runs it")
	})
}
