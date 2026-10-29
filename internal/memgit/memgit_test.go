// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package memgit_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memgit"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// gitPath returns the absolute path of the git on the PATH.
func gitPath(t *testing.T) string {
	t.Helper()
	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		p := filepath.Join(dir, "git")
		if fi, err := os.Stat(p); err == nil && filepath.IsAbs(dir) && fi.Mode().IsRegular() {
			return p
		}
	}
	require.FailNow(t, "no git on the PATH of the test process")
	return ""
}

// fixtureEnv is the environment of the plain git that builds fixtures:
// no system or global config, and a fixed identity.
func fixtureEnv(home string) []string {
	return []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + home, "LANG=C",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
	}
}

// git runs plain git in dir, for fixtures and checks, and returns its
// standard output, trimmed.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.JulietaGit,
		Argv:   append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...),
		Dir:    dir,
		Env:    fixtureEnv(t.TempDir()),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, errors.Join(err), "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// remote is a bare repository served over git:// on 127.0.0.1, with
// one URL that takes pushes and one that refuses them.
type remote struct {
	dir, url, readOnly string
}

// newRemote makes an empty bare repository and serves it.
func newRemote(t *testing.T) remote {
	t.Helper()
	base := t.TempDir()
	git(t, base, "init", "-q", "--bare", "m.git")
	rw := serve(t, base, true)
	ro := serve(t, base, false)
	return remote{dir: filepath.Join(base, "m.git"), url: rw + "m.git", readOnly: ro + "m.git"}
}

// main returns the remote's main, or "".
func (r remote) main(t *testing.T) string {
	t.Helper()
	return git(t, r.dir, "for-each-ref", "--format=%(objectname)", "refs/heads/main")
}

// serve serves base over git:// on 127.0.0.1, with pushes when push is
// set, and returns the URL prefix.
func serve(t *testing.T, base string, push bool) string {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	argv := []string{gitPath(t), "daemon", "--inetd", "--export-all", "--base-path=" + base}
	if push {
		argv = append(argv, "--enable=receive-pack")
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				_ = procgroup.Run(ctx, procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Env: fixtureEnv(base), Stdin: conn, Stdout: conn})
			})
		}
	})
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		wg.Wait()
	})
	return "git://" + ln.Addr().String() + "/"
}

// newProject returns a project whose tree is a new directory, with the
// memory dirs sync makes and the files julieta writes, a planted
// salvage/x and snapshot/heads.json among them.
func newProject(t *testing.T, url string) memgit.Project {
	t.Helper()
	env := filepath.Join(t.TempDir(), "shop-env")
	for name, data := range map[string]string{
		"_shared/entries/01handoff.md": "handoff\n",
		"_shared/stamps/01stamp.json":  "{}\n",
		"api/entries/02entry.md":       "entry\n",
		"api/entries/.tmp-01ABC":       "half\n",
		"api/salvage/x":                "secret\n",
		"api/snapshot/heads.json":      "{}\n",
	} {
		write(t, filepath.Join(env, "memory"), name, data)
	}
	return memgit.Project{Name: "shop", EnvDir: env, Remote: url}
}

// write writes data at the slash path name below dir.
func write(t *testing.T, dir, name, data string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
}

// runner returns a host-mode runner over the test's git.
func runner(t *testing.T) *gitsafe.Runner {
	t.Helper()
	g, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: gitPath(t), Home: t.TempDir()})
	require.NoError(t, err)
	return g
}

// sshAgent serves an in-process ssh-agent holding one new ed25519 key
// and returns its socket and the key's public line.
func sshAgent(t *testing.T) (socket, public string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: priv}))
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	dir := filepath.Join(os.TempDir(), "mg"+rand.Text()[:8])
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket = filepath.Join(dir, "s")
	ln, err := new(net.ListenConfig).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()
	return socket, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}

// TestNotices warns memory-local for none and memory-unsigned without a
// socket or a key, every time, and nothing for a signed remote.
func TestNotices(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    memgit.Project
		want string
	}{
		{"none", memgit.Project{Name: "shop", Remote: "none", SigningKey: "k", AgentSocket: "/s"}, memgit.SlugLocal},
		{"no socket", memgit.Project{Name: "shop", Remote: "https://h/m", SigningKey: "k"}, memgit.SlugUnsigned},
		{"no key", memgit.Project{Name: "shop", Remote: "https://h/m", AgentSocket: "/s"}, memgit.SlugUnsigned},
		{"signed", memgit.Project{Name: "shop", Remote: "https://h/m", SigningKey: "k", AgentSocket: "/s"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := memgit.Notices(tc.p)
			if tc.want == "" {
				assert.Empty(t, w)
				return
			}
			require.Len(t, w, 1)
			assert.Equal(t, tc.want, w[0].Slug)
		})
	}
}

// TestSyncNoneDoesNothing makes no git dir for a spec that opts out.
func TestSyncNoneDoesNothing(t *testing.T) {
	p := newProject(t, "none")
	w, err := memgit.Sync(t.Context(), runner(t), p)
	require.NoError(t, err)
	assert.Empty(t, w)
	assert.NoDirExists(t, filepath.Join(p.EnvDir, "memory.git"))
}

// TestSyncCommitsEntriesAndStamps commits exactly entries/ and stamps/,
// signed with the project's key, as the project's name, pushes it to
// the remote's main, leaves no .git in the memory dir, and on a second
// run with no change commits and pushes nothing (REQ-0277).
func TestSyncCommitsEntriesAndStamps(t *testing.T) {
	r := newRemote(t)
	p := newProject(t, r.url)
	p.AgentSocket, p.SigningKey = sshAgent(t)
	g := runner(t)
	w, err := memgit.Sync(t.Context(), g, p)
	require.NoError(t, err)
	assert.Empty(t, w)

	gitDir := filepath.Join(p.EnvDir, "memory.git")
	head := git(t, gitDir, "rev-parse", "refs/heads/main")
	assert.Equal(t, head, r.main(t), "on the remote's main")
	assert.Equal(t, "_shared/entries/01handoff.md\n_shared/stamps/01stamp.json\napi/entries/02entry.md",
		git(t, gitDir, "ls-tree", "-r", "--name-only", "refs/heads/main"), "salvage/, snapshot/ and .tmp-* are left out")
	assert.Equal(t, "shop <>", git(t, gitDir, "log", "-1", "--format=%an <%ae>", "refs/heads/main"))
	signers := filepath.Join(t.TempDir(), "allowed")
	require.NoError(t, os.WriteFile(signers, []byte("shop "+p.SigningKey+"\n"), 0o600))
	assert.Equal(t, "G", git(t, gitDir, "-c", "gpg.ssh.allowedSignersFile="+signers, "log", "-1", "--format=%G?", "refs/heads/main"))
	assert.NoFileExists(t, filepath.Join(p.EnvDir, "memory", ".git"))
	exclude, err := os.Stat(filepath.Join(gitDir, "info", "exclude"))
	require.NoError(t, err)

	// The second run: a remote that refuses pushes would warn if Sync
	// pushed, and the excludes are not written again.
	p.Remote = r.readOnly
	w, err = memgit.Sync(t.Context(), g, p)
	require.NoError(t, err)
	assert.Empty(t, w, "nothing to push")
	assert.Equal(t, head, git(t, gitDir, "rev-parse", "refs/heads/main"), "nothing committed")
	again, err := os.Stat(filepath.Join(gitDir, "info", "exclude"))
	require.NoError(t, err)
	assert.Equal(t, exclude.ModTime(), again.ModTime())
}

// TestSyncUnsigned commits unsigned without a socket.
func TestSyncUnsigned(t *testing.T) {
	r := newRemote(t)
	p := newProject(t, r.url)
	w, err := memgit.Sync(t.Context(), runner(t), p)
	require.NoError(t, err)
	assert.Empty(t, w)
	assert.Equal(t, "N", git(t, r.dir, "log", "-1", "--format=%G?", "refs/heads/main"))
	require.Len(t, memgit.Notices(p), 1)
}

// pushFrom commits name in a clone of r's main and pushes it, as a
// second machine does, and returns the commit.
func pushFrom(t *testing.T, r remote, name string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "w")
	git(t, filepath.Dir(work), "clone", "-q", r.dir, work)
	write(t, work, name, name+"\n")
	git(t, work, "add", "-A")
	git(t, work, "commit", "-q", "-m", name)
	git(t, work, "push", "-q", "origin", "HEAD:refs/heads/main")
	return git(t, work, "rev-parse", "HEAD")
}

// TestSyncFastForwards fetches a remote that moved on by one commit,
// fast-forwards the memory dir to it, then commits and pushes.
func TestSyncFastForwards(t *testing.T) {
	r := newRemote(t)
	p := newProject(t, r.url)
	g := runner(t)
	_, err := memgit.Sync(t.Context(), g, p)
	require.NoError(t, err)
	moved := pushFrom(t, r, "_shared/entries/03other.md")
	write(t, filepath.Join(p.EnvDir, "memory"), "api/entries/04mine.md", "mine\n")

	w, err := memgit.Sync(t.Context(), g, p)
	require.NoError(t, err)
	assert.Empty(t, w)
	assert.FileExists(t, filepath.Join(p.EnvDir, "memory", "_shared", "entries", "03other.md"))
	gitDir := filepath.Join(p.EnvDir, "memory.git")
	assert.Equal(t, moved, git(t, gitDir, "rev-parse", "refs/heads/main~1"), "the commit is on top of the remote's")
	assert.Equal(t, git(t, gitDir, "rev-parse", "refs/heads/main"), r.main(t))
	assert.Contains(t, git(t, r.dir, "ls-tree", "-r", "--name-only", "main"), "api/entries/04mine.md")
}

// TestSyncDiverged warns memory-diverged for a history that cannot
// fast-forward, and commits, pushes and rewrites nothing.
func TestSyncDiverged(t *testing.T) {
	r := newRemote(t)
	p := newProject(t, r.url)
	g := runner(t)
	_, err := memgit.Sync(t.Context(), g, p)
	require.NoError(t, err)
	theirs := pushFrom(t, r, "_shared/entries/03other.md")
	gitDir := filepath.Join(p.EnvDir, "memory.git")
	// A local commit the remote does not have: the histories diverge.
	write(t, filepath.Join(p.EnvDir, "memory"), "api/entries/04mine.md", "mine\n")
	git(t, p.EnvDir, "--git-dir="+gitDir, "--work-tree="+filepath.Join(p.EnvDir, "memory"), "-c", "core.bare=false", "add", "-A")
	git(t, p.EnvDir, "--git-dir="+gitDir, "--work-tree="+filepath.Join(p.EnvDir, "memory"), "-c", "core.bare=false", "commit", "-q", "-m", "mine")
	ours := git(t, gitDir, "rev-parse", "refs/heads/main")
	write(t, filepath.Join(p.EnvDir, "memory"), "api/entries/05later.md", "later\n")

	w, err := memgit.Sync(t.Context(), g, p)
	require.NoError(t, err)
	require.Len(t, w, 1)
	assert.Equal(t, memgit.SlugDiverged, w[0].Slug)
	assert.Contains(t, w[0].Message, "have diverged")
	assert.Equal(t, theirs, r.main(t), "nothing pushed")
	assert.Equal(t, ours, git(t, gitDir, "rev-parse", "refs/heads/main"), "nothing committed or rewritten")
}

// TestSyncPushRefused warns memory-push-failed for a remote that
// refuses the push, and returns no error: the command exits 0.
func TestSyncPushRefused(t *testing.T) {
	r := newRemote(t)
	p := newProject(t, r.readOnly)
	w, err := memgit.Sync(t.Context(), runner(t), p)
	require.NoError(t, err)
	require.Len(t, w, 1)
	assert.Equal(t, memgit.SlugPushFailed, w[0].Slug)
	assert.Contains(t, w[0].Message, "the push to")
	assert.Empty(t, r.main(t))
	assert.NotEmpty(t, git(t, filepath.Join(p.EnvDir, "memory.git"), "rev-parse", "refs/heads/main"), "committed locally; the next command pushes")
}

// TestSyncClonesIntoPlace takes the remote's history into an empty
// memory dir, as on a second machine, with the separate git dir.
func TestSyncClonesIntoPlace(t *testing.T) {
	r := newRemote(t)
	first := newProject(t, r.url)
	_, err := memgit.Sync(t.Context(), runner(t), first)
	require.NoError(t, err)

	env := filepath.Join(t.TempDir(), "shop-env")
	require.NoError(t, os.MkdirAll(filepath.Join(env, "memory", "_shared"), 0o700))
	second := memgit.Project{Name: "shop", EnvDir: env, Remote: r.url}
	w, err := memgit.Sync(t.Context(), runner(t), second)
	require.NoError(t, err)
	assert.Empty(t, w)
	data, err := os.ReadFile(filepath.Join(env, "memory", "_shared", "entries", "01handoff.md"))
	require.NoError(t, err)
	assert.Equal(t, "handoff\n", string(data))
	assert.NoFileExists(t, filepath.Join(env, "memory", ".git"))
	assert.Equal(t, r.main(t), git(t, filepath.Join(env, "memory.git"), "rev-parse", "refs/heads/main"))

	// An absent memory dir is made and filled the same way.
	env = filepath.Join(t.TempDir(), "shop-env")
	require.NoError(t, os.Mkdir(env, 0o700))
	_, err = memgit.Sync(t.Context(), runner(t), memgit.Project{Name: "shop", EnvDir: env, Remote: r.url})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(env, "memory", "api", "entries", "02entry.md"))
}

// TestSyncRefusesAForeignTree warns memory-diverged for a remote whose
// main holds a path outside entries/ and stamps/, and checks out
// nothing of it.
func TestSyncRefusesAForeignTree(t *testing.T) {
	r := newRemote(t)
	pushFrom(t, r, "api/salvage/x")
	env := filepath.Join(t.TempDir(), "shop-env")
	require.NoError(t, os.MkdirAll(filepath.Join(env, "memory"), 0o700))
	w, err := memgit.Sync(t.Context(), runner(t), memgit.Project{Name: "shop", EnvDir: env, Remote: r.url})
	require.NoError(t, err)
	require.Len(t, w, 1)
	assert.Equal(t, memgit.SlugDiverged, w[0].Slug)
	assert.Contains(t, w[0].Message, "api/salvage/x")
	assert.NoFileExists(t, filepath.Join(env, "memory", "api", "salvage", "x"))
}
