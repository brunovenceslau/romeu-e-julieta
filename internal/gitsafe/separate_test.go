// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// stagedOf returns a StagedFile of path that reads data.
func stagedOf(path, data string) StagedFile {
	return StagedFile{Path: path, Size: int64(len(data)), Open: func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte(data))), nil
	}}
}

// newSeparate returns a Separate in a new directory: a work tree that
// exists and a git dir that does not.
func newSeparate(t *testing.T) Separate {
	t.Helper()
	base := t.TempDir()
	s := Separate{GitDir: filepath.Join(base, "memory.git"), WorkTree: filepath.Join(base, "memory")}
	require.NoError(t, os.Mkdir(s.WorkTree, 0o700))
	return s
}

// writeSep writes data at the slash path name below dir, making its
// directories.
func writeSep(t *testing.T, dir, name, data string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
}

// sepGit runs plain git on s.
func sepGit(t *testing.T, s Separate, args ...string) string {
	t.Helper()
	return plainGit(t, s.WorkTree, slices.Concat([]string{"--git-dir=" + s.GitDir, "--work-tree=" + s.WorkTree, "-c", "core.bare=false"}, args)...)
}

// TestInitSeparate makes a bare git dir with HEAD on main, writes
// nothing into the work tree and no core.worktree into the config, and
// refuses an existing git dir, sandbox mode and nested paths.
func TestInitSeparate(t *testing.T) {
	r := newRunner(t, Host)
	s := newSeparate(t)
	require.NoError(t, r.InitSeparate(t.Context(), s))
	entries, err := os.ReadDir(s.WorkTree)
	require.NoError(t, err)
	assert.Empty(t, entries, "no .git in the work tree")
	assert.Equal(t, "refs/heads/main", plainGit(t, s.GitDir, "symbolic-ref", "HEAD"))
	config, err := os.ReadFile(filepath.Join(s.GitDir, "config"))
	require.NoError(t, err)
	assert.NotContains(t, string(config), "worktree")
	_, err = os.Stat(filepath.Join(s.GitDir, "hooks"))
	require.ErrorIs(t, err, os.ErrNotExist, "no template")

	require.ErrorIs(t, r.InitSeparate(t.Context(), s), ErrRefused, "the git dir exists")
	require.ErrorIs(t, newRunner(t, Sandbox).InitSeparate(t.Context(), newSeparate(t)), ErrRefused)
	nested := newSeparate(t)
	nested.GitDir = filepath.Join(nested.WorkTree, "x.git")
	require.ErrorIs(t, r.InitSeparate(t.Context(), nested), ErrRefused)
	relative := newSeparate(t)
	relative.GitDir = "memory.git"
	require.ErrorIs(t, r.InitSeparate(t.Context(), relative), ErrRefused)
}

// TestCommitSeparate stages exactly the files given, whatever the work
// tree holds (a .gitignore that re-includes a secret, a file the files
// do not name), commits them as the project's name with no email,
// unsigned, and reports no staged change once they are committed.
func TestCommitSeparate(t *testing.T) {
	r := newRunner(t, Host)
	s := newSeparate(t)
	require.NoError(t, r.InitSeparate(t.Context(), s))
	writeSep(t, s.WorkTree, ".gitignore", "!*\n")
	writeSep(t, s.WorkTree, "api/salvage/x", "x\n")
	writeSep(t, s.WorkTree, "api/entries/e.md", "on disk\n")

	changed, err := r.StagedChanges(t.Context(), s)
	require.NoError(t, err)
	assert.False(t, changed, "nothing staged on an unborn branch")
	files := []StagedFile{stagedOf("api/entries/e.md", "e\n"), stagedOf("_shared/stamps/s", "")}
	require.NoError(t, r.StageExactly(t.Context(), s, files))
	changed, err = r.StagedChanges(t.Context(), s)
	require.NoError(t, err)
	assert.True(t, changed)
	require.NoError(t, r.Commit(t.Context(), s, Identity{Name: "shop"}, "memory of shop"))
	assert.Equal(t, "shop <>|shop <>|N", sepGit(t, s, "log", "-1", "--format=%an <%ae>|%cn <%ce>|%G?"))
	assert.Equal(t, "_shared/stamps/s\napi/entries/e.md", sepGit(t, s, "ls-tree", "-r", "--name-only", "HEAD"))
	assert.Equal(t, "e", sepGit(t, s, "cat-file", "blob", "HEAD:api/entries/e.md"), "the bytes of the reader, not of the path")

	require.NoError(t, r.StageExactly(t.Context(), s, files))
	changed, err = r.StagedChanges(t.Context(), s)
	require.NoError(t, err)
	assert.False(t, changed)
	require.NoError(t, r.StageExactly(t.Context(), s, files[:1]))
	changed, err = r.StagedChanges(t.Context(), s)
	require.NoError(t, err)
	assert.True(t, changed, "a file left out is a deletion")

	short := stagedOf("api/entries/e.md", "e\n")
	short.Size = 5
	require.ErrorIs(t, r.StageExactly(t.Context(), s, []StagedFile{short}), ErrRefused, "a reader shorter than its size")
	long := stagedOf("api/entries/e.md", "e\n")
	long.Size = 1
	require.ErrorIs(t, r.StageExactly(t.Context(), s, []StagedFile{long}), ErrRefused, "a reader longer than its size")
	for _, bad := range []string{"", "/abs", "../x", "a/../b", "a\tb", "a\nb", ".git/config", "a/*"} {
		require.ErrorIs(t, r.StageExactly(t.Context(), s, []StagedFile{stagedOf(bad, "")}), ErrRefused, "%q", bad)
	}
	require.ErrorIs(t, r.StageExactly(t.Context(), s, []StagedFile{stagedOf("a", ""), stagedOf("a", "")}), ErrRefused, "twice")

	for _, id := range []Identity{{}, {Name: "a<b>"}, {Name: "shop", SigningKey: "ssh-ed25519 AAAA"}, {Name: "shop", AgentSocket: "/s"}} {
		require.ErrorIs(t, r.Commit(t.Context(), s, id, "m"), ErrRefused, "%+v", id)
	}
	require.ErrorIs(t, r.Commit(t.Context(), s, Identity{Name: "shop"}, "a\x1bb"), ErrRefused)
}

// sshAgent serves an in-process ssh-agent that holds one new ed25519
// key on a socket of a short path, and returns the socket and the key's
// public line.
func sshAgent(t *testing.T) (socket, public string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyring := agent.NewKeyring()
	require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: priv}))
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	dir := filepath.Join(os.TempDir(), "ag"+rand.Text()[:8])
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

// TestCommitSeparateSigned signs the commit with the key the agent
// holds, through SSH_AUTH_SOCK of that git alone, and the signature
// verifies against that key.
func TestCommitSeparateSigned(t *testing.T) {
	r := newRunner(t, Host)
	s := newSeparate(t)
	require.NoError(t, r.InitSeparate(t.Context(), s))
	require.NoError(t, r.StageExactly(t.Context(), s, []StagedFile{stagedOf("_shared/stamps/s", "s\n")}))
	socket, public := sshAgent(t)
	require.NoError(t, r.Commit(t.Context(), s, Identity{Name: "shop", SigningKey: public, AgentSocket: socket}, "memory of shop"))
	assert.Contains(t, sepGit(t, s, "cat-file", "commit", "HEAD"), "gpgsig -----BEGIN SSH SIGNATURE-----")
	signers := filepath.Join(t.TempDir(), "allowed")
	require.NoError(t, os.WriteFile(signers, []byte("shop "+public+"\n"), 0o600))
	out := sepGit(t, s, "-c", "gpg.ssh.allowedSignersFile="+signers, "log", "-1", "--format=%G?")
	assert.Equal(t, "G", out)
	config, err := os.ReadFile(filepath.Join(s.GitDir, "config"))
	require.NoError(t, err)
	assert.NotContains(t, string(config), "signingkey", "no key written (I2)")
	assert.NotContains(t, strings.ToLower(string(config)), "gpg")
}

// TestFastForwardSeparate moves an unborn branch to a commit, then
// forward, checking the files out, leaves it on an ancestor, and
// refuses a move that is no fast-forward.
func TestFastForwardSeparate(t *testing.T) {
	r := newRunner(t, Host)
	src := t.TempDir()
	plainGit(t, src, "init", "-q")
	writeSep(t, src, "_shared/entries/a.md", "a\n")
	plainGit(t, src, "add", "-A")
	plainGit(t, src, "commit", "-q", "-m", "a")
	first := plainGit(t, src, "rev-parse", "HEAD")
	writeSep(t, src, "_shared/entries/b.md", "b\n")
	plainGit(t, src, "add", "-A")
	plainGit(t, src, "commit", "-q", "-m", "b")
	second := plainGit(t, src, "rev-parse", "HEAD")
	plainGit(t, src, "checkout", "-q", "-b", "side", first)
	writeSep(t, src, "_shared/entries/c.md", "c\n")
	plainGit(t, src, "add", "-A")
	plainGit(t, src, "commit", "-q", "-m", "c")
	side := plainGit(t, src, "rev-parse", "HEAD")

	s := newSeparate(t)
	require.NoError(t, r.InitSeparate(t.Context(), s))
	plainGit(t, s.GitDir, "fetch", "-q", src, "+refs/heads/*:refs/romeu/origin/memory/*")
	require.NoError(t, r.FastForwardSeparate(t.Context(), s, first))
	assert.FileExists(t, filepath.Join(s.WorkTree, "_shared", "entries", "a.md"))
	require.NoError(t, r.FastForwardSeparate(t.Context(), s, second))
	assert.FileExists(t, filepath.Join(s.WorkTree, "_shared", "entries", "b.md"))
	assert.Equal(t, second, sepGit(t, s, "rev-parse", "HEAD"))

	ok, err := r.IsAncestor(t.Context(), s.GitDir, first, second)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = r.IsAncestor(t.Context(), s.GitDir, second, first)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, r.FastForwardSeparate(t.Context(), s, first), "an ancestor is already merged")
	require.Error(t, r.FastForwardSeparate(t.Context(), s, side), "a diverged commit is no fast-forward")
	assert.Equal(t, second, sepGit(t, s, "rev-parse", "HEAD"))
	require.ErrorIs(t, r.FastForwardSeparate(t.Context(), s, "HEAD"), ErrRefused)
}

// TestSeparateArgv pins the argv and environment of the verbs: the
// flags of host mode, core.bare=false, the git dir and the work tree on
// the command line, and SSH_AUTH_SOCK on the signed commit alone.
func TestSeparateArgv(t *testing.T) {
	home := t.TempDir()
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: home})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	start := r.start
	oid := strings.Repeat("b", 40)
	r.start = func(ctx context.Context, c procgroup.Command) error {
		if (slices.Contains(c.Argv, "write-tree") || slices.Contains(c.Argv, "commit-tree")) && c.Stdout != nil {
			_ = start(ctx, c)
			_, err := io.WriteString(c.Stdout, oid+"\n")
			return err
		}
		return start(ctx, c)
	}
	s := newSeparate(t)
	require.NoError(t, os.Mkdir(s.GitDir, 0o700))
	writeSep(t, s.GitDir, "HEAD", "ref: refs/heads/main\n")
	require.NoError(t, os.Mkdir(filepath.Join(s.GitDir, "objects"), 0o700))

	require.NoError(t, r.StageExactly(t.Context(), s, nil))
	require.NoError(t, r.Commit(t.Context(), s, Identity{Name: "shop", SigningKey: "ssh-ed25519 AAAA k", AgentSocket: "/tmp/s"}, "m"))
	var verbs [][]string
	for _, c := range rec.commands {
		if !slices.Contains(c.Argv, "--git-dir="+s.GitDir) || slices.Contains(c.Argv, "--get-regexp") {
			continue // the listing of the filter drivers, and the read of the branch's tip
		}
		verbs = append(verbs, c.Argv)
		assert.Equal(t, s.WorkTree, c.Dir)
		for _, e := range c.Env {
			if strings.HasPrefix(e, "SSH_AUTH_SOCK=") {
				assert.Contains(t, c.Argv, "commit-tree", "the socket reaches the commit alone")
			}
		}
		assert.NotContains(t, c.Argv, "commit", "git commit, which refreshes the index from the work tree, never runs")
	}
	require.Len(t, verbs, 5)
	head := slices.Concat([]string{"/usr/bin/git"}, hostFlags, []string{"-c", "core.bare=false"})
	dirs := []string{"--git-dir=" + s.GitDir, "--work-tree=" + s.WorkTree}
	assert.Equal(t, slices.Concat(head, dirs, []string{"read-tree", "--empty"}), verbs[0])
	assert.Equal(t, slices.Concat(head, dirs, []string{"update-index", "-z", "--index-info"}), verbs[1])
	assert.Equal(t, slices.Concat(head, dirs, []string{"write-tree"}), verbs[2])
	assert.Equal(t, slices.Concat(head, []string{
		"-c", "gpg.format=ssh", "-c", "gpg.ssh.program=ssh-keygen", "-c", "user.signingKey=key::ssh-ed25519 AAAA k",
	}, dirs, []string{"commit-tree", "-S", "-m", "m", oid}), verbs[3])
	assert.Equal(t, slices.Concat(head, dirs, []string{"update-ref", "refs/heads/main", oid, ""}), verbs[4], "an unborn branch: must not exist")
}

// TestStageExactlyRefusesACollision refuses two paths that are one
// under case folding or Unicode normalization, before git runs: APFS
// keeps one of them, and WalkTree refuses the tree on every other
// machine.
func TestStageExactlyRefusesACollision(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: t.TempDir()})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	s := newSeparate(t)
	require.NoError(t, os.Mkdir(s.GitDir, 0o700))
	writeSep(t, s.GitDir, "HEAD", "ref: refs/heads/main\n")
	require.NoError(t, os.Mkdir(filepath.Join(s.GitDir, "objects"), 0o700))
	for name, pair := range map[string][2]string{
		"case":          {"api/entries/Foo.md", "api/entries/foo.md"},
		"normalization": {"api/entries/caf\u00e9.md", "api/entries/cafe\u0301.md"},
		"exact":         {"api/entries/a.md", "api/entries/a.md"},
	} {
		err := r.StageExactly(t.Context(), s, []StagedFile{stagedOf(pair[0], ""), stagedOf(pair[1], "")})
		require.ErrorIs(t, err, ErrRefused, name)
	}
	assert.Empty(t, rec.commands, "no git ran")
}

// TestStageExactlyTakesAQuoteLiterally stages a path that starts with a
// double quote as that path: --index-info without -z would unquote it
// as a C string, and stage another path or fail every run.
func TestStageExactlyTakesAQuoteLiterally(t *testing.T) {
	r := newRunner(t, Host)
	s := newSeparate(t)
	require.NoError(t, r.InitSeparate(t.Context(), s))
	require.NoError(t, r.StageExactly(t.Context(), s, []StagedFile{stagedOf(`"api/entries/f"`, "q\n"), stagedOf(`"open`, "u\n")}))
	assert.Equal(t, "\"api/entries/f\"\x00\"open\x00", sepGit(t, s, "ls-files", "-z"))
}
