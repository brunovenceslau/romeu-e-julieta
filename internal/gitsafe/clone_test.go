// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCloneArgv: a clone resolves its URL outside any repository, then
// clones that explicit URL into the destination, from the
// destination's parent, with the clone flags, no checkout and the
// caFile of its host; it then reads HEAD, lists the filter drivers of
// the new repository and checks out HEAD there with each emptied.
func TestCloneArgv(t *testing.T) {
	r, err := New(Config{
		Mode: Host, Git: "/usr/bin/git", Home: "/h",
		GitHosts: []GitHost{{Host: "git.example.com", CAFile: "/ca.pem"}},
	})
	require.NoError(t, err)
	rec := recorder{filters: "filter.lfs.smudge\x00"}
	rec.install(r)
	const url = "https://git.example.com/o/r.git"
	dest := filepath.Join(t.TempDir(), "r")
	require.NoError(t, r.Clone(t.Context(), url, dest))
	require.Len(t, rec.commands, 5)
	ca := []string{"-c", "http.https://git.example.com/.sslCAInfo=/ca.pem"}

	resolve := rec.commands[0]
	assert.Equal(t, "/", resolve.Dir, "no repository's local config takes part")
	assert.Equal(t, append(ca, "ls-remote", "--get-url", url), resolve.Argv[1+len(hostFlags):])

	clone := rec.commands[1]
	assert.Equal(t, filepath.Dir(dest), clone.Dir)
	assert.Equal(t, append(ca, "clone", "--no-tags", "--no-recurse-submodules", "--template=", "--no-checkout", "--", url, dest), clone.Argv[1+len(hostFlags):])

	head := rec.commands[2]
	assert.Equal(t, dest, head.Dir)
	assert.Equal(t, []string{"rev-parse", "--verify", "-q", "HEAD^{commit}"}, head.Argv[1+len(hostFlags):])

	filters := rec.commands[3]
	assert.Equal(t, dest, filters.Dir, "the listing reads the new repository's config")
	assert.Equal(t, []string{"config", "-z", "--name-only", "--get-regexp", `^filter\.`}, filters.Argv[len(filters.Argv)-5:])

	checkout := rec.commands[4]
	assert.Equal(t, dest, checkout.Dir)
	assert.Equal(t, []string{"reset", "-q", "--hard", "HEAD"}, checkout.Argv[1+len(hostFlags):])
	assert.Subset(t, checkout.Env, []string{
		"GIT_CONFIG_COUNT=4",
		"GIT_CONFIG_KEY_0=filter.lfs.clean", "GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_1=filter.lfs.smudge", "GIT_CONFIG_VALUE_1=",
		"GIT_CONFIG_KEY_2=filter.lfs.process", "GIT_CONFIG_VALUE_2=",
		"GIT_CONFIG_KEY_3=filter.lfs.required", "GIT_CONFIG_VALUE_3=false",
	})
}

// TestClone clones over the loopback daemon in each mode: the new
// clone has the default branch checked out and the remote-tracking
// refs of origin, and no tag, since the clone takes no tags.
func TestClone(t *testing.T) {
	src, _ := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(src, "README"), []byte("hello\n"), 0o600))
	plainGit(t, src, "add", "README")
	head := commit(t, src, "two")
	plainGit(t, src, "tag", "v1")
	plainGit(t, src, "branch", "topic")
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "repo")
			require.NoError(t, newRunner(t, mode).Clone(t.Context(), url, dest))
			assert.Equal(t, head, plainGit(t, dest, "rev-parse", "HEAD"))
			assert.Equal(t, "refs/heads/main", plainGit(t, dest, "symbolic-ref", "HEAD"))
			assert.Equal(t, head, refOf(t, dest, "refs/remotes/origin/topic"))
			assert.Empty(t, plainGit(t, dest, "for-each-ref", "refs/tags"))
			assert.Equal(t, url, plainGit(t, dest, "config", "--get", "remote.origin.url"))
			got, err := os.ReadFile(filepath.Join(dest, "README"))
			require.NoError(t, err)
			assert.Equal(t, "hello\n", string(got))
		})
	}
}

// TestCloneEmpty clones an empty repository in each mode: the clone
// succeeds with HEAD on an unborn branch and no checkout, as git clone
// leaves it.
func TestCloneEmpty(t *testing.T) {
	src := t.TempDir()
	plainGit(t, src, "init", "-q")
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "repo")
			require.NoError(t, newRunner(t, mode).Clone(t.Context(), url, dest))
			assert.Empty(t, refOf(t, dest, "HEAD"))
			assert.NoFileExists(t, filepath.Join(dest, ".git", "index"))
		})
	}
}

// TestCloneCAFile clones over HTTPS from a server whose certificate
// only the caFile of its host trusts.
func TestCloneCAFile(t *testing.T) {
	src, head := newRepo(t)
	host, caFile := serveHTTPS(t, bareCopy(t, src, "repo.git"))
	url := "https://" + host + "/repo.git"
	for _, tt := range []struct {
		name  string
		hosts []GitHost
		ok    bool
	}{
		{"its own host", []GitHost{{Host: host, CAFile: caFile}}, true},
		{"no ca file", []GitHost{{Host: host}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(Config{Mode: Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: tt.hosts})
			require.NoError(t, err)
			dest := filepath.Join(t.TempDir(), "repo")
			err = r.Clone(t.Context(), url, dest)
			if !tt.ok {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), "certificate")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, head, plainGit(t, dest, "rev-parse", "HEAD"))
		})
	}
}

// TestCloneRefuses: a clone refuses what a fetch refuses, and a
// destination that is not a new, absolute, clean path; nothing runs.
func TestCloneRefuses(t *testing.T) {
	base := t.TempDir()
	existing := filepath.Join(base, "existing")
	require.NoError(t, os.Mkdir(existing, 0o700))
	file := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	dangling := filepath.Join(base, "dangling")
	require.NoError(t, os.Symlink(filepath.Join(base, "nowhere"), dangling))
	const url = "https://github.com/o/r.git"
	fresh := filepath.Join(base, "fresh")
	tests := []struct {
		name, url, dest string
	}{
		{"file URL", "file:///srv/r.git", fresh},
		{"plain path", "/srv/r.git", fresh},
		{"git on another host", "git://example.com/r.git", fresh},
		{"ssh", "ssh://git@github.com/o/r.git", fresh},
		{"http", "http://github.com/o/r.git", fresh},
		{"user information", "https://u:p@github.com/o/r.git", fresh},
		{"option as URL", "--upload-pack=/bin/sh", fresh},
		{"relative destination", url, "repo"},
		{"unclean destination", url, base + "/x/../fresh"},
		{"trailing slash", url, fresh + "/"},
		{"root", url, "/"},
		{"control character", url, fresh + "\n"},
		{"an empty directory", url, existing},
		{"a file", url, file},
		{"a dangling symlink", url, dangling},
	}
	for _, mode := range []Mode{Host, Sandbox} {
		for _, tt := range tests {
			t.Run(mode.String()+"/"+tt.name, func(t *testing.T) {
				r := newRunner(t, mode)
				var rec recorder
				rec.install(r)
				require.ErrorIs(t, r.Clone(t.Context(), tt.url, tt.dest), ErrRefused)
				assert.Empty(t, rec.commands, "a refused clone runs nothing")
			})
		}
	}
	assert.NoFileExists(t, fresh)
}

// TestCloneNeedsItsParent: the parent of the destination is the
// caller's to make, with the mode it wants; a clone into a missing one
// fails and creates nothing.
func TestCloneNeedsItsParent(t *testing.T) {
	src, _ := newRepo(t)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	parent := filepath.Join(t.TempDir(), "missing")
	err := newRunner(t, Host).Clone(t.Context(), url, filepath.Join(parent, "repo"))
	require.Error(t, err)
	assert.NoDirExists(t, parent)

	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	r := newRunner(t, Host)
	var rec recorder
	rec.install(r)
	err = r.Clone(t.Context(), url, filepath.Join(file, "repo"))
	require.Error(t, err, "a parent that is a file")
	require.NotErrorIs(t, err, ErrRefused, "an error of the file system, not a refusal")
	assert.Empty(t, rec.commands)
}

// TestCloneInsteadOf: in the sandbox, the global config is the
// sandbox's own and can rewrite a clone URL; a rewrite to a refused
// protocol stops the clone before it starts, and one to another HTTPS
// host does not.
func TestCloneInsteadOf(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		refused bool
	}{
		{"to file", "file:///srv/", true},
		{"to the loopback daemon", "git://127.0.0.1:9418/", true},
		{"to another https host", "https://mirror.invalid/", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			config := "[url \"" + tt.base + "\"]\n\tinsteadOf = https://github.com/\n"
			require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(config), 0o600))
			r, err := New(Config{Mode: Sandbox, Git: gitPath(t), Home: home})
			require.NoError(t, err)
			dest := filepath.Join(t.TempDir(), "repo")
			got, err := r.resolveURL(t.Context(), "/", "https://github.com/o/r.git")
			if !tt.refused {
				require.NoError(t, err)
				assert.Equal(t, tt.base+"o/r.git", got)
				return
			}
			require.ErrorIs(t, err, ErrRefused)
			require.ErrorIs(t, r.Clone(t.Context(), "https://github.com/o/r.git", dest), ErrRefused)
			assert.NoDirExists(t, dest)
		})
	}
}

// TestCloneRunsNoHook plants a post-checkout hook through the process
// environment, as a hooks path and as a template directory: no gitsafe
// clone runs it or copies it, and plain git with the same environment
// does (the positive control, 05 5.2).
//
//romeu:invariant I3 test
func TestCloneRunsNoHook(t *testing.T) {
	src, _ := newRepo(t)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	marker := filepath.Join(t.TempDir(), "marker")
	hook := []byte("#!/bin/sh\necho ran >> '" + marker + "'\n")
	hooks := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "post-checkout"), hook, 0o700))
	templates := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(templates, "hooks"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(templates, "hooks", "post-checkout"), hook, 0o700))
	plant := []string{
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=" + hooks,
		"GIT_TEMPLATE_DIR=" + templates,
	}
	for _, kv := range plant {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "repo")
			require.NoError(t, newRunner(t, mode).Clone(t.Context(), url, dest))
			assert.NoFileExists(t, marker)
			assert.NoFileExists(t, filepath.Join(dest, ".git", "hooks", "post-checkout"))
		})
	}

	control := t.TempDir()
	_, err := plainGitErr(t, control, plant, "clone", "-q", url, "repo")
	require.NoError(t, err)
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
	assert.FileExists(t, filepath.Join(control, "repo", ".git", "hooks", "post-checkout"))
}

// TestCloneRunsNoSmudgeFilter plants filter drivers in the sandbox's
// global config, one with an empty name, each required and named by the
// .gitattributes of the cloned tree: the clone's checkout runs no
// filter and writes the files as the tree holds them. Plain git with
// the same global config runs them (the positive control).
//
//romeu:invariant I3 test
func TestCloneRunsNoSmudgeFilter(t *testing.T) {
	src, _ := newRepo(t)
	_, marker, global := filterPlant(t, src)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	home := t.TempDir()
	writeFile(t, home, ".gitconfig", global)

	r, err := New(Config{Mode: Sandbox, Git: gitPath(t), Home: home})
	require.NoError(t, err)
	dest := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, r.Clone(t.Context(), url, dest))
	assert.NoFileExists(t, marker)
	got, err := os.ReadFile(filepath.Join(dest, "c"))
	require.NoError(t, err)
	assert.Equal(t, "c\n", string(got))

	env := []string{"GIT_CONFIG_GLOBAL=" + filepath.Join(home, ".gitconfig")}
	_, err = plainGitErr(t, t.TempDir(), env, "clone", "-q", url, "repo")
	require.NoError(t, err)
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
}

// TestCloneRunsNoFilterFromAConditionalInclude: a smudge driver that
// the sandbox's global config defines only in a file it includes under
// a condition the new clone meets (includeIf "gitdir:", "onbranch:" and
// "hasconfig:remote.*.url:") is neutralized too; plain git with the
// same global config runs it (the positive control).
//
//romeu:invariant I3 test
func TestCloneRunsNoFilterFromAConditionalInclude(t *testing.T) {
	src, _ := newRepo(t)
	_, marker, global := filterPlant(t, src)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	for _, tt := range []struct{ name, condition string }{
		{"gitdir", "gitdir:"},
		{"onbranch", "onbranch:main"},
		{"hasconfig", "hasconfig:remote.*.url:" + url},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.RemoveAll(marker))
			parent := t.TempDir()
			// git matches gitdir: against the resolved path, and TMPDIR
			// sits behind a symbolic link on macOS.
			resolved, err := filepath.EvalSymlinks(parent)
			require.NoError(t, err)
			condition := tt.condition
			if condition == "gitdir:" {
				condition += resolved + "/**"
			}
			home := t.TempDir()
			writeFile(t, home, "included", global)
			writeFile(t, home, ".gitconfig", "[includeIf \""+condition+"\"]\n\tpath = "+filepath.Join(home, "included")+"\n")

			r, err := New(Config{Mode: Sandbox, Git: gitPath(t), Home: home})
			require.NoError(t, err)
			dest := filepath.Join(parent, "repo")
			require.NoError(t, r.Clone(t.Context(), url, dest))
			assert.NoFileExists(t, marker)
			got, err := os.ReadFile(filepath.Join(dest, "c"))
			require.NoError(t, err)
			assert.Equal(t, "c\n", string(got))
			assert.Equal(t, "refs/heads/main", plainGit(t, dest, "symbolic-ref", "HEAD"))

			env := []string{"GIT_CONFIG_GLOBAL=" + filepath.Join(home, ".gitconfig")}
			_, err = plainGitErr(t, parent, env, "clone", "-q", url, "control")
			require.NoError(t, err)
			assert.FileExists(t, marker, "the plant is live: plain git runs it")
		})
	}
}

// TestSandboxCloneThroughTheProxy clones in sandbox mode through a
// local HTTPS proxy, as julieta setup does in a sandbox (REQ-0398,
// REQ-0470): the URL names a host only the proxy sends to the server,
// and the server's certificate is one only a test CA trusts. git's TLS
// library may ignore SSL_CERT_FILE (libcurl-gnutls does), so the test
// CA reaches git the way a sandbox can always give it one, through the
// sandbox's global config, which sandbox mode keeps; the same clone
// fails without it.
func TestSandboxCloneThroughTheProxy(t *testing.T) {
	src, head := newRepo(t)
	addr, caFile := serveHTTPS(t, bareCopy(t, src, "repo.git"))
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	// The test certificate of httptest names example.com; the proxy
	// alone sends that name to the server.
	url := "https://example.com:" + port + "/repo.git"
	connects := make(chan string, 8)
	proxy := listen(t, func(ctx context.Context, conn net.Conn) {
		tunnel(ctx, conn, addr, connects)
	})
	proxyURL := "http://" + strings.TrimSuffix(strings.TrimPrefix(proxy, "git://"), "/")
	for _, tt := range []struct {
		name      string
		globalCfg string
		ok        bool
	}{
		{"the sandbox trusts the test ca", "[http]\n\tsslCAInfo = " + caFile + "\n", true},
		{"the sandbox trusts the system cas alone", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// A CONNECT the last case left must not count for this one.
			for len(connects) > 0 {
				<-connects
			}
			for _, k := range NetworkEnv() {
				t.Setenv(k, "")
			}
			t.Setenv("HTTPS_PROXY", proxyURL)
			t.Setenv("https_proxy", proxyURL)
			home := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(tt.globalCfg), 0o600))
			r, err := New(Config{Mode: Sandbox, Git: gitPath(t), Home: home})
			require.NoError(t, err)
			dest := filepath.Join(t.TempDir(), "repo")
			err = r.Clone(t.Context(), url, dest)
			require.NotEmpty(t, connects, "git did not go to the proxy: %v", err)
			assert.Equal(t, "example.com:"+port, <-connects)
			if !tt.ok {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), "certificate")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, head, plainGit(t, dest, "rev-parse", "HEAD"))
		})
	}
}

// tunnel serves one CONNECT request on conn: it reports the target on
// connects, opens a connection to addr whatever the target names, and
// copies bytes both ways until either side closes or ctx ends.
func tunnel(ctx context.Context, conn net.Conn, addr string, connects chan<- string) {
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil || req.Method != http.MethodConnect {
		return
	}
	connects <- req.Host
	upstream, err := new(net.Dialer).DialContext(ctx, "tcp", addr)
	if err != nil {
		return
	}
	defer func() { _ = upstream.Close() }()
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, conn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(conn, upstream); done <- struct{}{} }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
