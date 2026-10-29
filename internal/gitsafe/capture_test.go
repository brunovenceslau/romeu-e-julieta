// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dirtyRepo makes a repository whose main holds the files kept, gone
// and same, then changes the working tree: kept modified, gone deleted,
// new staged, sub/loose untracked, build/out ignored, and a nested
// repository at nested/.
func dirtyRepo(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	plainGit(t, dir, "init", "-q")
	writeFile(t, dir, "kept", "one\n")
	writeFile(t, dir, "gone", "gone\n")
	writeFile(t, dir, "same", "same\n")
	writeFile(t, dir, ".gitignore", "build/\n")
	plainGit(t, dir, "add", ".")
	plainGit(t, dir, "commit", "-q", "-m", "files")
	head = plainGit(t, dir, "rev-parse", "HEAD")
	writeFile(t, dir, "kept", "two\n")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone")))
	writeFile(t, dir, "new", "new\n")
	plainGit(t, dir, "add", "new")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o700))
	writeFile(t, dir, "sub/loose", "loose\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build"), 0o700))
	writeFile(t, dir, "build/out", "out\n")
	nested := filepath.Join(dir, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o700))
	plainGit(t, nested, "init", "-q")
	commit(t, nested, "nested")
	return dir, head
}

// TestChangesRealGit reads the changes of a dirty worktree in both
// modes and writes nothing.
func TestChangesRealGit(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, head := dirtyRepo(t)
			check := noWrite(t, dir)
			c, err := newRunner(t, mode).Changes(t.Context(), dir)
			require.NoError(t, err)
			check()
			assert.Equal(t, head, c.OID)
			assert.Equal(t, "refs/heads/main", c.Branch)
			assert.Empty(t, c.Operation)
			assert.Equal(t, []string{"gone", "kept", "new"}, c.Tracked)
			assert.Equal(t, []string{"new"}, c.Added)
			assert.Equal(t, []string{"nested/", "sub/loose"}, c.Untracked)
		})
	}
}

// TestChangesDetachedAndUnbornRealGit: a detached HEAD has no branch,
// and an unborn branch no commit.
func TestChangesDetachedAndUnbornRealGit(t *testing.T) {
	r := newRunner(t, Sandbox)
	dir, head := newRepo(t)
	plainGit(t, dir, "checkout", "-q", "--detach")
	c, err := r.Changes(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, head, c.OID)
	assert.Empty(t, c.Branch)

	unborn := t.TempDir()
	plainGit(t, unborn, "init", "-q")
	writeFile(t, unborn, "a", "a\n")
	c, err = r.Changes(t.Context(), unborn)
	require.NoError(t, err)
	assert.Empty(t, c.OID)
	assert.Equal(t, "refs/heads/main", c.Branch)
	assert.Equal(t, []string{"a"}, c.Untracked)
}

// TestChangesOperation: each mark of an operation in progress in the
// git dir is the operation, the worktree's own git dir included.
func TestChangesOperation(t *testing.T) {
	r := newRunner(t, Sandbox)
	for _, tc := range []struct{ mark, want string }{
		{"rebase-merge", OperationRebase},
		{"rebase-apply", OperationRebase},
		{"MERGE_HEAD", OperationMerge},
		{"CHERRY_PICK_HEAD", OperationCherryPick},
		{"REVERT_HEAD", OperationRevert},
		{"BISECT_LOG", OperationBisect},
	} {
		t.Run(tc.mark, func(t *testing.T) {
			dir, head := newRepo(t)
			wt := filepath.Join(t.TempDir(), "wt")
			plainGit(t, dir, "worktree", "add", "-q", "--detach", wt)
			gitDir := plainGit(t, wt, "rev-parse", "--absolute-git-dir")
			require.NoError(t, os.WriteFile(filepath.Join(gitDir, tc.mark), []byte(head+"\n"), 0o600))
			c, err := r.Changes(t.Context(), wt)
			require.NoError(t, err)
			assert.Equal(t, tc.want, c.Operation)
			c, err = r.Changes(t.Context(), dir)
			require.NoError(t, err)
			assert.Empty(t, c.Operation, "the main worktree has none")
		})
	}
}

// TestChangesRealRebaseRealGit: a rebase stopped on a conflict is
// in progress.
func TestChangesRealRebaseRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	writeFile(t, dir, "f", "base\n")
	plainGit(t, dir, "add", "f")
	plainGit(t, dir, "commit", "-q", "-m", "base")
	plainGit(t, dir, "checkout", "-q", "-b", "topic")
	writeFile(t, dir, "f", "topic\n")
	plainGit(t, dir, "commit", "-q", "-am", "topic")
	plainGit(t, dir, "checkout", "-q", "main")
	writeFile(t, dir, "f", "main\n")
	plainGit(t, dir, "commit", "-q", "-am", "main")
	_, err := plainGitErr(t, dir, nil, "rebase", "topic")
	require.Error(t, err, "the rebase stops on its conflict")
	c, err := newRunner(t, Sandbox).Changes(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, OperationRebase, c.Operation)
	assert.Contains(t, c.Tracked, "f", "the conflict is a tracked change")
}

// TestIgnoredRealGit lists an ignored directory once, with its slash,
// and an ignored file, and writes nothing.
func TestIgnoredRealGit(t *testing.T) {
	dir, _ := dirtyRepo(t)
	writeFile(t, dir, ".git/info/exclude", "*.log\n")
	writeFile(t, dir, "a.log", "log\n")
	check := noWrite(t, dir)
	got, err := newRunner(t, Sandbox).Ignored(t.Context(), dir)
	require.NoError(t, err)
	check()
	assert.Equal(t, []string{"a.log", "build/"}, got)
}

// TestGitlinksRealGit lists the submodules the index records, once.
func TestGitlinksRealGit(t *testing.T) {
	dir, head := newRepo(t)
	plainGit(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",mod")
	writeFile(t, dir, "plain", "plain\n")
	plainGit(t, dir, "add", "plain")
	got, err := newRunner(t, Sandbox).Gitlinks(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"mod"}, got)
}

// TestCaptureListsAreBounded: a list over maxCapturePaths is refused.
func TestCaptureListsAreBounded(t *testing.T) {
	old := maxCapturePaths
	maxCapturePaths = 1
	t.Cleanup(func() { maxCapturePaths = old })
	r := newRunner(t, Sandbox)
	dir, head := dirtyRepo(t)
	writeFile(t, dir, "x.o", "x\n")
	writeFile(t, dir, ".gitignore", "build/\n*.o\n")
	_, err := r.Changes(t.Context(), dir)
	require.ErrorIs(t, err, ErrRefused)
	_, err = r.Ignored(t.Context(), dir)
	require.ErrorIs(t, err, ErrRefused)
	plainGit(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",m1")
	plainGit(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+head+",m2")
	_, err = r.Gitlinks(t.Context(), dir)
	require.ErrorIs(t, err, ErrRefused)
}

// capture returns a capture of the files to add, into a new index
// file.
func capture(t *testing.T, parent string, add ...string) Capture {
	t.Helper()
	return Capture{
		Index: filepath.Join(t.TempDir(), "index"), Parent: parent, Add: add,
		Name: "julieta-salvage", Email: "julieta-salvage@localhost", Message: "salvage",
		When: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
	}
}

// TestCaptureCommitRealGit records the working tree of a dirty
// worktree: the modified file's new content, the deleted file gone,
// the staged and the kept untracked files, the ignored file and the
// untracked file not added left out; the author, the committer and the
// date are the capture's; no signature although the repository asks
// for one with a gpg that fails; the worktree, its index and its refs
// are unchanged.
//
//romeu:invariant I18 test
func TestCaptureCommitRealGit(t *testing.T) {
	dir, head := dirtyRepo(t)
	writeFile(t, dir, "other", "other\n")
	plainGit(t, dir, "config", "commit.gpgsign", "true")
	plainGit(t, dir, "config", "gpg.program", "/bin/false")
	check := noWrite(t, dir)
	c := capture(t, head, "new", "sub/loose")
	oid, err := newRunner(t, Sandbox).CaptureCommit(t.Context(), dir, c)
	require.NoError(t, err)
	check()
	require.NotEmpty(t, oid)
	assert.Equal(t, head, plainGit(t, dir, "rev-parse", oid+"^"))
	files := plainGit(t, dir, "ls-tree", "-r", "--name-only", oid)
	assert.Equal(t, ".gitignore\nkept\nnew\nsame\nsub/loose", files)
	assert.Equal(t, "two", plainGit(t, dir, "show", oid+":kept"))
	assert.Equal(t, "julieta-salvage <julieta-salvage@localhost> 1793275200|julieta-salvage <julieta-salvage@localhost> 1793275200",
		plainGit(t, dir, "log", "-1", "--format=%an <%ae> %at|%cn <%ce> %ct", oid))
	raw := plainGit(t, dir, "cat-file", "commit", oid)
	assert.NotContains(t, raw, "gpgsig", "no signature")

	_, err = plainGitErr(t, dir, nil, "-c", "commit.gpgsign=true", "commit", "--allow-empty", "-q", "-m", "x")
	require.Error(t, err, "the plant is live: a commit of plain git that signs fails")
}

// TestCaptureCommitClean: a working tree that holds nothing beyond its
// HEAD makes no commit, and so does an unborn branch with nothing to
// add; an unborn branch with a file makes a root commit.
func TestCaptureCommitClean(t *testing.T) {
	r := newRunner(t, Sandbox)
	dir, head := newRepo(t)
	oid, err := r.CaptureCommit(t.Context(), dir, capture(t, head))
	require.NoError(t, err)
	assert.Empty(t, oid)

	unborn := t.TempDir()
	plainGit(t, unborn, "init", "-q")
	oid, err = r.CaptureCommit(t.Context(), unborn, capture(t, ""))
	require.NoError(t, err)
	assert.Empty(t, oid)
	writeFile(t, unborn, "a", "a\n")
	oid, err = r.CaptureCommit(t.Context(), unborn, capture(t, "", "a"))
	require.NoError(t, err)
	require.NotEmpty(t, oid)
	_, err = plainGitErr(t, unborn, nil, "rev-parse", "--verify", "-q", oid+"^")
	require.Error(t, err, "a root commit")
	assert.Equal(t, "a", plainGit(t, unborn, "ls-tree", "--name-only", oid))
}

// TestCaptureCommitTakesLiteralPaths: a name with glob characters is a
// literal path, so a sibling it would match is not added.
func TestCaptureCommitTakesLiteralPaths(t *testing.T) {
	dir, head := newRepo(t)
	writeFile(t, dir, "a*", "star\n")
	writeFile(t, dir, "ab", "other\n")
	oid, err := newRunner(t, Sandbox).CaptureCommit(t.Context(), dir, capture(t, head, "a*"))
	require.NoError(t, err)
	assert.Equal(t, "a*", plainGit(t, dir, "ls-tree", "--name-only", oid))
}

// TestCaptureCommitRunsNoCleanFilter: a clean filter the repository
// requires does not run while the capture hashes the files.
//
//romeu:invariant I3 test
func TestCaptureCommitRunsNoCleanFilter(t *testing.T) {
	dir, head := newRepo(t)
	writeFile(t, dir, ".gitattributes", "* filter=evil\n")
	marker := filepath.Join(t.TempDir(), "marker")
	script := filepath.Join(t.TempDir(), "filter.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\ncat\n"), 0o700))
	plainGit(t, dir, "config", "filter.evil.clean", script)
	plainGit(t, dir, "config", "filter.evil.required", "true")
	writeFile(t, dir, "a", "a\n")
	oid, err := newRunner(t, Sandbox).CaptureCommit(t.Context(), dir, capture(t, head, ".gitattributes", "a"))
	require.NoError(t, err)
	assert.NotEmpty(t, oid)
	assert.NoFileExists(t, marker)
	plainGit(t, dir, "add", "a")
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
}

// TestCaptureCommitRefuses: what CaptureCommit refuses before git runs.
func TestCaptureCommitRefuses(t *testing.T) {
	r := newRunner(t, Sandbox)
	dir, head := newRepo(t)
	existing := filepath.Join(t.TempDir(), "index")
	require.NoError(t, os.WriteFile(existing, nil, 0o600))
	for name, change := range map[string]func(*Capture){
		"relative index":  func(c *Capture) { c.Index = "index" },
		"existing index":  func(c *Capture) { c.Index = existing },
		"parent":          func(c *Capture) { c.Parent = "HEAD" },
		"absolute path":   func(c *Capture) { c.Add = []string{"/etc/passwd"} },
		"dot-dot path":    func(c *Capture) { c.Add = []string{"a/../b"} },
		"empty path":      func(c *Capture) { c.Add = []string{""} },
		"NUL path":        func(c *Capture) { c.Add = []string{"a\x00b"} },
		"identity":        func(c *Capture) { c.Email = "a>b" },
		"no name":         func(c *Capture) { c.Name = "" },
		"no message":      func(c *Capture) { c.Message = "" },
		"no date":         func(c *Capture) { c.When = time.Time{} },
		"control in name": func(c *Capture) { c.Name = "a\x1bb" },
	} {
		t.Run(name, func(t *testing.T) {
			c := capture(t, head, "a")
			change(&c)
			_, err := r.CaptureCommit(t.Context(), dir, c)
			require.ErrorIs(t, err, ErrRefused)
		})
	}
	_, err := r.CaptureCommit(t.Context(), t.TempDir(), capture(t, head))
	require.ErrorIs(t, err, ErrRefused, "not a repository")
	_, err = r.Changes(t.Context(), t.TempDir())
	require.ErrorIs(t, err, ErrRefused)
	_, err = r.Ignored(t.Context(), t.TempDir())
	require.ErrorIs(t, err, ErrRefused)
	_, err = r.Gitlinks(t.Context(), t.TempDir())
	require.ErrorIs(t, err, ErrRefused)
}

// TestCaptureCommitFails: a git that fails, here an add of a path that
// does not exist, fails the capture.
func TestCaptureCommitFails(t *testing.T) {
	dir, head := newRepo(t)
	_, err := newRunner(t, Sandbox).CaptureCommit(t.Context(), dir, capture(t, head, "missing"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git add")
}
