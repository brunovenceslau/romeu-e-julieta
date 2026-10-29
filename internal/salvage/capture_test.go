// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// The ids of the salvages of the tests.
const (
	salvageGen = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	salvageID  = "01jb000000000000000000000a"
)

// put writes content to the file name below dir, making its parents.
func put(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
}

// resolved returns dir with its symbolic links resolved, as git names a
// worktree: on macOS the temporary dirs are below /var, a link to
// /private/var.
func resolved(t *testing.T, dir string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return r
}

// sandbox is the fixture of a salvage: a repo with a commit, its memory
// dir, and a HOME.
type sandbox struct {
	repo, memory, home string
	head               string
}

// newSandbox makes a repo whose main holds kept and .gitignore (which
// ignores *.log and cache/), a memory dir and a HOME.
func newSandbox(t *testing.T) sandbox {
	t.Helper()
	s := sandbox{repo: resolved(t, repo(t)), memory: filepath.Join(t.TempDir(), "api"), home: t.TempDir()}
	require.NoError(t, os.Mkdir(s.memory, 0o700))
	put(t, s.repo, "kept", "one\n")
	put(t, s.repo, ".gitignore", "*.log\ncache/\nbuild/\n")
	git(t, s.repo, "add", ".")
	git(t, s.repo, "commit", "-q", "-m", "files")
	s.head = git(t, s.repo, "rev-parse", "HEAD")
	return s
}

// config returns the config of a salvage of s, cap 1 MiB.
func (s sandbox) config(t *testing.T) salvage.Config {
	t.Helper()
	return salvage.Config{
		Git: runner(t), Home: s.home, Work: filepath.Join(t.TempDir(), "work"),
		Repos:      []salvage.Repo{{Dir: "api", Path: s.repo, Memory: s.memory, Primary: true}},
		Generation: salvageGen, SalvageID: salvageID, CapBytes: 1 << 20,
		Now:   func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC) },
		ULIDs: canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 4096))),
	}
}

// dir returns the salvage dir of s.
func (s sandbox) dir() string {
	return filepath.Join(s.memory, "salvage", salvageGen, salvageID)
}

// readManifest decodes the manifest of the salvage dir strictly.
func readManifest(t *testing.T, dir string) salvage.Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, salvage.ManifestFile))
	require.NoError(t, err)
	var m salvage.Manifest
	require.NoError(t, json.Unmarshal(data, &m, json.RejectUnknownMembers(true)))
	return m
}

// archive returns the entries of a tar.gz file, by name, with the
// content of each regular file and "-> target" for a link.
func archive(t *testing.T, file string) map[string]string {
	t.Helper()
	f, err := os.Open(file)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	got := map[string]string{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return got
		}
		require.NoError(t, err)
		assert.Equal(t, int64(0o600), h.Mode, "%s", h.Name)
		if h.Typeflag == tar.TypeSymlink {
			got[h.Name] = "-> " + h.Linkname
			continue
		}
		b, err := io.ReadAll(tr)
		require.NoError(t, err)
		got[h.Name] = string(b)
	}
}

// reasons returns the skipped items of m as "reason item".
func reasons(m salvage.Manifest) []string {
	var out []string
	for _, s := range m.Skipped {
		out = append(out, s.Reason+" "+s.Item)
	}
	return out
}

// TestSalvage runs a whole salvage of a repo that holds every kind of
// work (08 8.5 steps 2 to 6): a dirty main worktree, a dirty linked
// worktree, a detached one, a stash, ignored files with an excluded
// one, a nested repository with dirt and a stash, and the agent's
// state with a transcript. The capture commits are julieta-salvage's
// under refs/salvage/<generation>/<salvage-id>/, the worktree and its
// index are unchanged, the bundle holds every ref, the archives hold
// the files, the salvage dir is 0700 and its files 0600, the memory
// dir keeps its layout, and the fingerprint is ReadFingerprint's.
func TestSalvage(t *testing.T) {
	s := newSandbox(t)
	git(t, s.repo, "checkout", "-q", "-b", "topic")
	topic := commit(t, s.repo, "topic")
	git(t, s.repo, "checkout", "-q", "main")
	put(t, s.repo, "stashed", "stash me\n")
	git(t, s.repo, "add", "stashed")
	git(t, s.repo, "stash", "-q")
	stash := git(t, s.repo, "rev-parse", "stash@{0}")
	put(t, s.repo, "kept", "two\n")
	put(t, s.repo, "loose", "loose\n")
	put(t, s.repo, "a.log", "log\n")
	put(t, s.repo, "cache/big", "excluded\n")
	put(t, s.repo, "build/out", "out\n")
	wt := filepath.Join(resolved(t, t.TempDir()), "wt")
	git(t, s.repo, "worktree", "add", "-q", wt, "topic")
	put(t, wt, "kept", "linked\n")
	detached := filepath.Join(resolved(t, t.TempDir()), "zz-detached")
	git(t, s.repo, "worktree", "add", "-q", "--detach", detached)
	nested := filepath.Join(s.repo, "vendor", "lib")
	require.NoError(t, os.MkdirAll(nested, 0o700))
	git(t, nested, "init", "-q")
	commit(t, nested, "nested")
	put(t, nested, "n", "nested stash\n")
	git(t, nested, "add", "n")
	git(t, nested, "stash", "-q")
	put(t, nested, "dirt", "nested dirt\n")
	put(t, s.home, ".claude/projects/p/memory/MEMORY.md", "memory\n")
	put(t, s.home, ".claude/projects/p/session.jsonl", "{}\n")
	put(t, s.home, ".claude/todos/t.json", "[]\n")
	before := git(t, s.repo, "status", "--porcelain=v2", "--untracked-files=all")
	index, err := os.ReadFile(filepath.Join(s.repo, ".git", "index"))
	require.NoError(t, err)

	c := s.config(t)
	c.ExcludeIgnored = []string{"cache/"}
	c.Profile = &salvage.AgentProfile{Name: "claude", Transcript: func(rel string) bool { return strings.HasSuffix(rel, ".jsonl") },
		Paths: []salvage.AgentPath{{Rel: ".claude/projects", Required: true}, {Rel: ".claude/todos", Required: true}, {Rel: ".claude/plans"}}}
	c.AgentStop = &salvage.AgentStop{Matched: 1}
	fp, err := salvage.ReadFingerprint(t.Context(), c.Git, c.Repos, c.ExcludeIgnored)
	require.NoError(t, err)
	res, err := salvage.Salvage(t.Context(), c)
	require.NoError(t, err)

	assert.Equal(t, fp, res.Fingerprint, "the reads of the salvage are the fingerprint's")
	assert.True(t, res.SecretBearing)
	assert.Equal(t, before, git(t, s.repo, "status", "--porcelain=v2", "--untracked-files=all"), "the worktree is unchanged")
	after, err := os.ReadFile(filepath.Join(s.repo, ".git", "index"))
	require.NoError(t, err)
	assert.Equal(t, index, after, "the index is unchanged")
	assert.NoDirExists(t, c.Work, "the work dir is removed")

	require.Len(t, res.Manifests, 1)
	assert.Equal(t, filepath.Join(s.dir(), salvage.ManifestFile), res.Manifests[0].Path)
	m := readManifest(t, s.dir())
	assert.Equal(t, res.Manifests[0].Manifest, m)
	assert.Equal(t, "salvage/v1", m.Schema)
	assert.Equal(t, salvageID, m.SalvageID)
	assert.Equal(t, salvageGen, m.Generation)
	assert.Equal(t, "api", m.Repo)
	assert.Equal(t, &salvage.AgentStop{Matched: 1}, m.AgentStop)
	assert.ElementsMatch(t, []string{"excluded-by-spec worktree-0/cache", "transcript-excluded .claude/projects/p/session.jsonl"}, reasons(m))
	assert.True(t, m.Complete, "only accepted reasons")

	prefix := salvage.RefPrefix(salvageGen, salvageID)
	require.Len(t, m.Worktrees, 4)
	main := m.Worktrees[0]
	assert.Equal(t, salvage.Worktree{Branch: "refs/heads/main", Capture: prefix + "worktree-0", Dirty: true, Head: s.head, Location: s.repo, Path: "worktree-0"}, main)
	capture := git(t, s.repo, "rev-parse", main.Capture)
	assert.Equal(t, s.head, git(t, s.repo, "rev-parse", capture+"^"))
	assert.Equal(t, "two", git(t, s.repo, "show", capture+":kept"))
	assert.Equal(t, "loose", git(t, s.repo, "show", capture+":loose"))
	assert.Equal(t, "julieta-salvage julieta-salvage@localhost", git(t, s.repo, "log", "-1", "--format=%an %ae", capture))
	assert.NotContains(t, git(t, s.repo, "cat-file", "commit", capture), "gpgsig")
	assert.Equal(t, salvage.Worktree{Branch: "refs/heads/topic", Capture: prefix + "worktree-1", Dirty: true, Head: topic, Location: wt, Path: "worktree-1"}, m.Worktrees[2])
	assert.Equal(t, "linked", git(t, s.repo, "show", prefix+"worktree-1:kept"))
	assert.Equal(t, salvage.Worktree{Detached: true, Head: s.head, Location: detached, Path: "worktree-2"}, m.Worktrees[3], "clean, so no capture")
	nw := m.Worktrees[1]
	assert.Equal(t, "worktree-0/vendor/lib", nw.Path)
	assert.Equal(t, nw.Path, nw.Nested)
	assert.Equal(t, prefix+"worktree-0", nw.Capture)
	assert.Equal(t, "nested dirt", git(t, nested, "show", nw.Capture+":dirt"))
	assert.Equal(t, stash, git(t, s.repo, "rev-parse", prefix+"stash-0"))
	assert.NotEmpty(t, git(t, nested, "rev-parse", prefix+"stash-0"), "the nested repository's stash")

	require.Len(t, m.Bundles, 2)
	for _, b := range m.Bundles {
		data, err := os.ReadFile(filepath.Join(s.dir(), b.File))
		require.NoError(t, err)
		sum := sha256.Sum256(data)
		assert.Equal(t, hex.EncodeToString(sum[:]), b.SHA256)
		assert.Equal(t, int64(len(data)), b.Size)
	}
	assert.Equal(t, salvage.Bundle{File: "nested-1.bundle", Nested: "worktree-0/vendor/lib", SHA256: m.Bundles[0].SHA256, Size: m.Bundles[0].Size},
		m.Bundles[0], "the nested repository's, found while its holder is read")
	assert.Equal(t, salvage.Bundle{File: "repo.bundle", SHA256: m.Bundles[1].SHA256, Size: m.Bundles[1].Size}, m.Bundles[1])
	heads := git(t, s.repo, "bundle", "list-heads", filepath.Join(s.dir(), "repo.bundle")) + "\n"
	for _, ref := range []string{"refs/heads/main", "refs/heads/topic", prefix + "worktree-0", prefix + "worktree-1", prefix + "stash-0"} {
		assert.Contains(t, heads, " "+ref+"\n", "the bundle holds %s", ref)
		assert.True(t, slices.ContainsFunc(m.Refs, func(r salvage.Ref) bool { return r.Ref == ref && r.Nested == "" }), "refs[] lists %s", ref)
	}
	assert.True(t, slices.ContainsFunc(m.Refs, func(r salvage.Ref) bool { return r.Ref == prefix+"stash-0" && r.Nested == nw.Nested }))

	require.Len(t, m.Files, 2)
	assert.Equal(t, salvage.IgnoredFile, m.Files[0].Path)
	assert.Equal(t, map[string]string{"worktree-0/a.log": "log\n", "worktree-0/build/out": "out\n"}, archive(t, filepath.Join(s.dir(), salvage.IgnoredFile)))
	assert.Equal(t, "agent/claude.tar.gz", m.Files[1].Path)
	assert.Equal(t, map[string]string{".claude/projects/p/memory/MEMORY.md": "memory\n", ".claude/todos/t.json": "[]\n"},
		archive(t, filepath.Join(s.dir(), "agent", "claude.tar.gz")))
	for _, f := range m.Files {
		data, err := os.ReadFile(filepath.Join(s.dir(), f.Path))
		require.NoError(t, err)
		sum := sha256.Sum256(data)
		assert.Equal(t, hex.EncodeToString(sum[:]), f.SHA256)
	}

	require.NoError(t, filepath.WalkDir(filepath.Join(s.memory, "salvage"), func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		info, err := d.Info()
		require.NoError(t, err)
		want := fs.FileMode(0o600)
		if d.IsDir() {
			want = fs.ModeDir | 0o700
		}
		assert.Equal(t, want, info.Mode(), "%s", p)
		return nil
	}))
	root, err := os.OpenRoot(s.memory)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	rep, err := memstore.Check(root)
	require.NoError(t, err)
	assert.Empty(t, rep.Findings, "the salvage keeps the layout of the memory dir")

	_, err = salvage.Salvage(t.Context(), s.config(t))
	require.ErrorIs(t, err, fs.ErrExist, "a second salvage with the same id")
}

// TestSalvageCap: untracked files count against the cap and the ones
// beyond it are over-cap, left out of the capture (REQ-0703); ignored
// entries and agent files take what is left, and the rest is over-cap.
func TestSalvageCap(t *testing.T) {
	s := newSandbox(t)
	put(t, s.repo, "small", "12345")
	put(t, s.repo, "large", strings.Repeat("x", 100))
	put(t, s.repo, "b.log", "abc")
	put(t, s.repo, "c.log", "abcdef")
	put(t, s.home, ".claude/todos/t.json", "123")
	c := s.config(t)
	c.CapBytes = 10
	c.Profile = &salvage.AgentProfile{Name: "claude", Paths: []salvage.AgentPath{{Rel: ".claude/todos", Required: true}}}
	_, err := salvage.Salvage(t.Context(), c)
	require.NoError(t, err)
	m := readManifest(t, s.dir())
	assert.ElementsMatch(t, []string{"over-cap worktree-0/large", "over-cap worktree-0/c.log", "over-cap .claude/todos/t.json"}, reasons(m))
	assert.False(t, m.Complete)
	files := git(t, s.repo, "ls-tree", "-r", "--name-only", m.Worktrees[0].Capture)
	assert.Contains(t, files, "small")
	assert.NotContains(t, files, "large")
	assert.Equal(t, map[string]string{"worktree-0/b.log": "abc"}, archive(t, filepath.Join(s.dir(), salvage.IgnoredFile)))
	assert.NoFileExists(t, filepath.Join(s.dir(), "agent", "claude.tar.gz"))
}

// TestSalvageInProgressOperation: a merge in progress skips the
// worktree as in-progress-operation, with no capture.
func TestSalvageInProgressOperation(t *testing.T) {
	s := newSandbox(t)
	put(t, s.repo, "kept", "two\n")
	put(t, s.repo, ".git/MERGE_HEAD", s.head+"\n")
	_, err := salvage.Salvage(t.Context(), s.config(t))
	require.NoError(t, err)
	m := readManifest(t, s.dir())
	assert.Equal(t, []string{"in-progress-operation worktree-0"}, reasons(m))
	assert.Empty(t, m.Worktrees[0].Capture)
	assert.True(t, m.Worktrees[0].Dirty)
}

// TestSalvageWorktreeChanging: a worktree that changes after each
// capture is skipped as worktree-changing after the second; one that
// changes once is captured again, with the later content.
func TestSalvageWorktreeChanging(t *testing.T) {
	s := newSandbox(t)
	put(t, s.repo, "kept", "two\n")
	n := 0
	salvage.SetAfterCapture(t, func(dir string) {
		n++
		put(t, dir, "kept", strings.Repeat("x", n))
	})
	_, err := salvage.Salvage(t.Context(), s.config(t))
	require.NoError(t, err)
	m := readManifest(t, s.dir())
	assert.Equal(t, []string{"worktree-changing worktree-0"}, reasons(m))
	assert.Empty(t, m.Worktrees[0].Capture)
	assert.Equal(t, 2, n, "the capture repeats once")

	s = newSandbox(t)
	put(t, s.repo, "kept", "two\n")
	n = 0
	salvage.SetAfterCapture(t, func(dir string) {
		n++
		if n == 1 {
			put(t, dir, "kept", "three\n")
		}
	})
	_, err = salvage.Salvage(t.Context(), s.config(t))
	require.NoError(t, err)
	m = readManifest(t, s.dir())
	assert.Empty(t, m.Skipped)
	assert.Equal(t, "three", git(t, s.repo, "show", m.Worktrees[0].Capture+":kept"))
}

// TestSalvageAgent: a missing required path is agent-path-missing and a
// missing optional one no item; a session open with no process matched
// is agent-not-matched; agentStop is in the primary's manifest only,
// and a secondary repo holds no agent archive.
func TestSalvageAgent(t *testing.T) {
	s := newSandbox(t)
	web := newSandbox(t)
	c := s.config(t)
	c.Repos = append(c.Repos, salvage.Repo{Dir: "web", Path: web.repo, Memory: web.memory})
	c.Profile = &salvage.AgentProfile{Name: "claude", Paths: []salvage.AgentPath{{Rel: ".claude/projects", Required: true}, {Rel: ".claude/plans"}}}
	c.AgentStop = &salvage.AgentStop{}
	c.AgentNotMatched = true
	res, err := salvage.Salvage(t.Context(), c)
	require.NoError(t, err)
	require.Len(t, res.Manifests, 2)
	m := readManifest(t, s.dir())
	assert.Equal(t, []string{"agent-not-matched claude", "agent-path-missing .claude/projects"}, reasons(m))
	assert.Equal(t, &salvage.AgentStop{}, m.AgentStop)
	assert.False(t, m.Complete)
	w := readManifest(t, filepath.Join(web.memory, "salvage", salvageGen, salvageID))
	assert.Nil(t, w.AgentStop)
	assert.Empty(t, w.Skipped)
	assert.True(t, w.Complete)
	assert.False(t, res.SecretBearing)
	assert.Len(t, res.Fingerprint.Repos, 2)
}

// TestSalvageBase: a repo whose refs the base all reaches needs no
// bundle.
func TestSalvageBase(t *testing.T) {
	s := newSandbox(t)
	c := s.config(t)
	c.Repos[0].Base = []string{s.head}
	_, err := salvage.Salvage(t.Context(), c)
	require.NoError(t, err)
	m := readManifest(t, s.dir())
	assert.Empty(t, m.Bundles)
	assert.Empty(t, m.Refs)
	assert.True(t, m.Complete)
}

// TestSalvageRefuses: a config with no runner, a bad id or a memory dir
// that is not absolute is an error, and nothing is written.
func TestSalvageRefuses(t *testing.T) {
	s := newSandbox(t)
	for name, change := range map[string]func(*salvage.Config){
		"no runner":     func(c *salvage.Config) { c.Git = nil },
		"generation":    func(c *salvage.Config) { c.Generation = "x" },
		"salvage id":    func(c *salvage.Config) { c.SalvageID = "X" },
		"memory":        func(c *salvage.Config) { c.Repos[0].Memory = "memory" },
		"memory a file": func(c *salvage.Config) { c.Repos[0].Memory = filepath.Join(s.repo, "kept") },
	} {
		t.Run(name, func(t *testing.T) {
			c := s.config(t)
			change(&c)
			_, err := salvage.Salvage(t.Context(), c)
			require.Error(t, err)
			assert.NoDirExists(t, s.dir())
		})
	}
}

// TestFailReason: a full disk is disk-full, by errno or by git's
// message (REQ-0703); any other error is unreadable.
func TestFailReason(t *testing.T) {
	assert.Equal(t, salvage.ReasonDiskFull, salvage.FailReason(&fs.PathError{Op: "write", Path: "x", Err: syscall.ENOSPC}))
	assert.Equal(t, salvage.ReasonDiskFull, salvage.FailReason(errors.New("git add: exit 128: error: unable to write: No space left on device")))
	assert.Equal(t, salvage.ReasonUnreadable, salvage.FailReason(errors.New("permission denied")))
}

// TestEncodeManifest: the schema is set, complete is recomputed from
// the skipped reasons, and nil lists are empty ones.
func TestEncodeManifest(t *testing.T) {
	data, err := salvage.EncodeManifest(salvage.Manifest{Complete: false, Skipped: []salvage.Skipped{{Item: "x", Reason: salvage.ReasonTranscriptExcluded}}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"bundles":[],"complete":true,"files":[],"generation":"","refs":[],"repo":"","salvageId":"","schema":"salvage/v1","skipped":[{"detail":"","item":"x","reason":"transcript-excluded"}],"worktrees":[]}`, string(data))
	assert.True(t, strings.HasPrefix(string(data), `{"bundles":[],"complete":true,`), "members sorted")
	assert.True(t, strings.HasSuffix(string(data), "}\n"), "a final line feed")
	data, err = salvage.EncodeManifest(salvage.Manifest{Complete: true, Skipped: []salvage.Skipped{{Item: "x", Reason: salvage.ReasonOverCap}}})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"complete":false`)
}

// TestWaitLocks: a lock held longer than LockReport is reported once,
// with its path; a free one is not; the error of a lock stops the wait.
func TestWaitLocks(t *testing.T) {
	old := salvage.LockReport
	salvage.LockReport = 10 * time.Millisecond
	t.Cleanup(func() { salvage.LockReport = old })
	var released []string
	lock := func(path string, onWait func()) (func(), error) {
		if path == "held" {
			onWait()
			time.Sleep(200 * time.Millisecond)
		}
		if path == "broken" {
			return nil, errors.New("broken")
		}
		return func() { released = append(released, path) }, nil
	}
	reported := make(chan string, 4)
	require.NoError(t, salvage.WaitLocks([]string{"free", "held"}, lock, func(p string) { reported <- p }))
	assert.Equal(t, []string{"free", "held"}, released, "each lock is released at once")
	assert.Equal(t, "held", <-reported)
	assert.Empty(t, reported)
	require.Error(t, salvage.WaitLocks([]string{"broken"}, lock, func(string) {}))
}

// TestSalvageLinks: a symbolic link is kept as a link and never
// followed, in the capture, the ignored archive and the agent's; a
// deleted file is gone from the capture; a FIFO holds nothing to keep;
// a repo of the manifest that is not cloned is unreadable, and the
// salvage goes on.
func TestSalvageLinks(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(s.repo, "link")))
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(s.repo, "x.log")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(s.repo, "pipe.log"), 0o600))
	require.NoError(t, os.Remove(filepath.Join(s.repo, "kept")))
	put(t, s.home, ".claude/todos/real.json", "[]\n")
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(s.home, ".claude/todos/link.json")))
	missing := sandbox{repo: filepath.Join(t.TempDir(), "web"), memory: filepath.Join(t.TempDir(), "web")}
	require.NoError(t, os.Mkdir(missing.memory, 0o700))
	c := s.config(t)
	c.Repos = append(c.Repos, salvage.Repo{Dir: "web", Path: missing.repo, Memory: missing.memory})
	c.Profile = &salvage.AgentProfile{Name: "claude", Paths: []salvage.AgentPath{{Rel: ".claude/todos", Required: true}}}
	_, err := salvage.Salvage(t.Context(), c)
	require.NoError(t, err)
	m := readManifest(t, s.dir())
	assert.Empty(t, m.Skipped)
	capture := m.Worktrees[0].Capture
	assert.Equal(t, "120000 link", git(t, s.repo, "ls-tree", "--format=%(objectmode) %(path)", capture, "link"))
	assert.Equal(t, "/etc/passwd", git(t, s.repo, "cat-file", "-p", capture+":link"))
	assert.Empty(t, git(t, s.repo, "ls-tree", capture, "kept"), "the deleted file is gone")
	assert.Equal(t, map[string]string{"worktree-0/x.log": "-> /etc/passwd"}, archive(t, filepath.Join(s.dir(), salvage.IgnoredFile)))
	assert.Equal(t, map[string]string{".claude/todos/real.json": "[]\n", ".claude/todos/link.json": "-> /etc/passwd"},
		archive(t, filepath.Join(s.dir(), "agent", "claude.tar.gz")))
	w := readManifest(t, filepath.Join(missing.memory, "salvage", salvageGen, salvageID))
	assert.Equal(t, []string{"unreadable worktrees"}, reasons(w))

	fp, err := salvage.ReadFingerprint(t.Context(), c.Git, c.Repos[:1], nil)
	require.NoError(t, err)
	require.Len(t, fp.Repos[0].Worktrees, 1)
	assert.NotEmpty(t, fp.Repos[0].Worktrees[0].Status)
	_, err = salvage.ReadFingerprint(t.Context(), c.Git, c.Repos, nil)
	require.Error(t, err, "a repo whose worktrees git cannot list")
}

// TestFingerprintMoves: the fingerprint changes with an edit of a dirty
// file, a new untracked file and an ignored file's size, and not with
// an excluded ignored file (03 3.8).
func TestFingerprintMoves(t *testing.T) {
	s := newSandbox(t)
	c := s.config(t)
	exclude := []string{"cache/"}
	read := func() salvage.FingerprintWorktree {
		fp, err := salvage.ReadFingerprint(t.Context(), c.Git, c.Repos, exclude)
		require.NoError(t, err)
		return fp.Repos[0].Worktrees[0]
	}
	put(t, s.repo, "kept", "dirty\n")
	base := read()
	assert.Equal(t, s.head, base.Head)
	put(t, s.repo, "cache/x", "excluded\n")
	assert.Equal(t, base, read(), "an excluded path")
	put(t, s.repo, "kept", "dirtier\n")
	edited := read()
	assert.NotEqual(t, base.Status, edited.Status)
	put(t, s.repo, "new", "n\n")
	added := read()
	assert.NotEqual(t, edited.Status, added.Status)
	put(t, s.repo, "a.log", "1")
	logged := read()
	assert.NotEqual(t, added.Ignored, logged.Ignored)
	assert.Equal(t, added.Status, logged.Status)
	put(t, s.repo, "a.log", "12")
	assert.NotEqual(t, logged.Ignored, read().Ignored)
}

// TestSalvageIndexOnlyPaths: an intent-to-add file is in the capture;
// a file added to the index and then deleted from the worktree does not
// fail the capture of the rest; a directory whose .git is a file or a
// link, which could name any git dir, is not a nested repository but an
// unreadable item, since git keeps none of it; and a
// worktree that holds only a nested repository is not dirty.
func TestSalvageIndexOnlyPaths(t *testing.T) {
	s := newSandbox(t)
	put(t, s.repo, "intent", "intent\n")
	git(t, s.repo, "add", "-N", "intent")
	put(t, s.repo, "staged", "staged\n")
	git(t, s.repo, "add", "staged")
	require.NoError(t, os.Remove(filepath.Join(s.repo, "staged")))
	put(t, s.repo, "kept", "two\n")
	put(t, s.repo, "fake/.git", "gitdir: "+filepath.Join(s.repo, ".git")+"\n")
	_, err := salvage.Salvage(t.Context(), s.config(t))
	require.NoError(t, err)
	m := readManifest(t, s.dir())
	assert.Equal(t, []string{"unreadable worktree-0/fake"}, reasons(m))
	require.Len(t, m.Worktrees, 1, "fake/ is no nested repository")
	files := git(t, s.repo, "ls-tree", "-r", "--name-only", m.Worktrees[0].Capture)
	assert.Contains(t, files, "intent")
	assert.NotContains(t, files, "fake")
	assert.NotContains(t, files, "staged")
	assert.Equal(t, "two", git(t, s.repo, "show", m.Worktrees[0].Capture+":kept"))

	clean := newSandbox(t)
	nested := filepath.Join(clean.repo, "lib")
	require.NoError(t, os.Mkdir(nested, 0o700))
	git(t, nested, "init", "-q")
	commit(t, nested, "nested")
	_, err = salvage.Salvage(t.Context(), clean.config(t))
	require.NoError(t, err)
	m = readManifest(t, clean.dir())
	require.Len(t, m.Worktrees, 2)
	assert.False(t, m.Worktrees[0].Dirty, "a nested repository alone is no dirt of its holder")
}
