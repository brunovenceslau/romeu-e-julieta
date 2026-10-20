// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// repoState is what a read of a working tree must leave as it found
// it: every ref and HEAD, the config, and the index's bytes and
// modification time.
type repoState struct {
	refs, head string
	config     []byte
	index      []byte
	indexTime  time.Time
}

// stateOf reads the state of the repository in dir, whose .git is a
// directory.
func stateOf(t *testing.T, dir string) repoState {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	var s repoState
	s.refs = plainGit(t, dir, "for-each-ref", "--format=%(refname) %(objectname) %(symref)")
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	require.NoError(t, err)
	s.head = string(head)
	s.config, err = os.ReadFile(filepath.Join(gitDir, "config"))
	require.NoError(t, err)
	s.index, err = os.ReadFile(filepath.Join(gitDir, "index"))
	if !errors.Is(err, os.ErrNotExist) {
		require.NoError(t, err)
		info, err := os.Stat(filepath.Join(gitDir, "index"))
		require.NoError(t, err)
		s.indexTime = info.ModTime()
	}
	return s
}

// noWrite records the state of the repository in dir, whose .git is a
// directory, and returns a check that it is unchanged.
func noWrite(t *testing.T, dir string) func() {
	t.Helper()
	before := stateOf(t, dir)
	return func() {
		t.Helper()
		assert.Equal(t, before, stateOf(t, dir), "the read wrote to the repository")
	}
}

// writeFile writes content to the file name of the work tree in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

// TestStatusWorkingTreeRealGit reads a clean tree, a staged change, an
// unstaged change and an untracked file, one fixture repository each,
// in both modes.
func TestStatusWorkingTreeRealGit(t *testing.T) {
	fixtures := map[string]struct {
		change func(t *testing.T, dir string)
		want   func(head string) Status
	}{
		"clean": {
			change: func(*testing.T, string) {},
			want:   func(head string) Status { return Status{Branch: "refs/heads/main", OID: head} },
		},
		"staged": {
			change: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "a", "staged\n")
				plainGit(t, dir, "add", "a")
			},
			want: func(head string) Status { return Status{Branch: "refs/heads/main", OID: head, Staged: true} },
		},
		"unstaged": {
			change: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "a", "changed\n")
			},
			want: func(head string) Status { return Status{Branch: "refs/heads/main", OID: head, Unstaged: true} },
		},
		"untracked": {
			change: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "new", "x\n")
			},
			want: func(head string) Status { return Status{Branch: "refs/heads/main", OID: head, Untracked: true} },
		},
	}
	for name, f := range fixtures {
		for _, mode := range []Mode{Host, Sandbox} {
			t.Run(name+"/"+mode.String(), func(t *testing.T) {
				dir, _ := newRepo(t)
				head := commitFile(t, dir, "a", "one\n", "a", "2026-02-01T00:00:00Z")
				f.change(t, dir)
				check := noWrite(t, dir)
				got, err := newRunner(t, mode).Status(t.Context(), dir)
				require.NoError(t, err)
				assert.Equal(t, f.want(head), got)
				assert.Equal(t, name == "clean", got.Clean())
				check()
			})
		}
	}
}

// TestStatusDetachedAndUnbornRealGit: a detached HEAD has a commit and
// no branch; an unborn branch has a branch and no commit; neither is
// an error. A conflict reads as unmerged.
func TestStatusDetachedAndUnbornRealGit(t *testing.T) {
	r := newRunner(t, Sandbox)

	dir, head := newRepo(t)
	plainGit(t, dir, "checkout", "-q", "--detach")
	check := noWrite(t, dir)
	got, err := r.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, Status{OID: head}, got)
	check()

	unborn := t.TempDir()
	plainGit(t, unborn, "init", "-q")
	check = noWrite(t, unborn)
	got, err = r.Status(t.Context(), unborn)
	require.NoError(t, err)
	assert.Equal(t, Status{Branch: "refs/heads/main"}, got)
	assert.True(t, got.Clean())
	check()

	conflict, _ := newRepo(t)
	commitFile(t, conflict, "a", "base\n", "base", "2026-02-01T00:00:00Z")
	plainGit(t, conflict, "checkout", "-q", "-b", "side")
	commitFile(t, conflict, "a", "side\n", "side", "2026-02-02T00:00:00Z")
	plainGit(t, conflict, "checkout", "-q", "main")
	commitFile(t, conflict, "a", "main\n", "main", "2026-02-03T00:00:00Z")
	_, err = plainGitErr(t, conflict, nil, "merge", "-q", "side")
	require.Error(t, err, "the fixture merge conflicts")
	got, err = r.Status(t.Context(), conflict)
	require.NoError(t, err)
	assert.True(t, got.Unmerged)
	assert.False(t, got.Clean())
}

// TestStatusRunsNoCleanFilter plants filter drivers in the local
// config, one named with a dot and one long-running process, and in
// the sandbox's global config, each required and named by
// .gitattributes, and makes every tracked file racy, so git status
// hashes it: Status runs no filter and reads the tree as clean, and
// plain git status runs the local one (the positive control).
//
//romeu:invariant I3 test
func TestStatusRunsNoCleanFilter(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			writeFile(t, dir, ".gitattributes", "a filter=evil\nb filter=ev.il\nc filter=glob\n")
			writeFile(t, dir, "a", "a\n")
			writeFile(t, dir, "b", "b\n")
			writeFile(t, dir, "c", "c\n")
			plainGit(t, dir, "add", ".")
			plainGit(t, dir, "commit", "-q", "-m", "files")
			marker := filepath.Join(t.TempDir(), "marker")
			script := filepath.Join(t.TempDir(), "filter.sh")
			require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\ncat\n"), 0o700))
			plainGit(t, dir, "config", "filter.evil.clean", script)
			plainGit(t, dir, "config", "filter.evil.required", "true")
			plainGit(t, dir, "config", "filter.ev.il.process", script)
			plainGit(t, dir, "config", "filter.ev.il.required", "true")
			home := t.TempDir()
			writeFile(t, home, ".gitconfig", "[filter \"glob\"]\n\tclean = "+script+"\n\trequired = true\n")
			// An index older than its entries makes each of them racy.
			past := time.Now().Add(-time.Hour)
			require.NoError(t, os.Chtimes(filepath.Join(dir, ".git", "index"), past, past))

			r, err := New(Config{Mode: mode, Git: gitPath(t), Home: home})
			require.NoError(t, err)
			check := noWrite(t, dir)
			got, err := r.Status(t.Context(), dir)
			require.NoError(t, err)
			assert.True(t, got.Clean(), "%+v", got)
			assert.NoFileExists(t, marker)
			check()

			_, _ = plainGitErr(t, dir, nil, "status", "--porcelain")
			assert.FileExists(t, marker, "the plant is live: plain git runs it")
		})
	}
}

// TestStatusLeavesTheIndexAlone: a tracked file whose stat data no
// longer matches the index makes plain git status refresh the index
// on disk (the positive control); Status reads it as clean and leaves
// the index's bytes and time as they were.
func TestStatusLeavesTheIndexAlone(t *testing.T) {
	dir, _ := newRepo(t)
	commitFile(t, dir, "a", "one\n", "a", "2026-02-01T00:00:00Z")
	later := time.Now().Add(time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(dir, "a"), later, later))
	check := noWrite(t, dir)
	got, err := newRunner(t, Sandbox).Status(t.Context(), dir)
	require.NoError(t, err)
	assert.True(t, got.Clean())
	check()

	before := stateOf(t, dir)
	plainGit(t, dir, "status", "--porcelain")
	assert.NotEqual(t, before, stateOf(t, dir), "the control is live: plain git status rewrites the index")
}

// TestStatusBranchesRealGit lists the local branches, sorted by name,
// with their commits, upstreams and the current one, including two
// unusual but legal names; the branch named (detached), checked out,
// is read as a branch and not as a detached HEAD.
func TestStatusBranchesRealGit(t *testing.T) {
	src, _ := newRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	plainGit(t, "", "clone", "-q", src, dir)
	main := plainGit(t, dir, "rev-parse", "HEAD")
	plainGit(t, dir, "branch", "--no-track", "(detached)")
	plainGit(t, dir, "branch", "--no-track", "feat/naïve+x@y#1")
	plainGit(t, dir, "checkout", "-q", "(detached)")
	side := commit(t, dir, "side")

	r := newRunner(t, Sandbox)
	check := noWrite(t, dir)
	got, err := r.Branches(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, []Branch{
		{Name: "refs/heads/(detached)", OID: side, Current: true},
		{Name: "refs/heads/feat/naïve+x@y#1", OID: main},
		{Name: "refs/heads/main", OID: main, Upstream: "refs/remotes/origin/main"},
	}, got)
	status, err := r.Status(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, Status{Branch: "refs/heads/(detached)", OID: side}, status)
	check()

	empty := t.TempDir()
	plainGit(t, empty, "init", "-q")
	none, err := r.Branches(t.Context(), empty)
	require.NoError(t, err)
	assert.Empty(t, none, "an unborn branch is not listed")
}

// TestStatusAheadBehindRealGit counts ahead and behind between a
// branch and its upstream, with a fixture ahead, one behind and one
// diverged, and between a local default branch and origin/<default>;
// a branch with no upstream has none, and asking for a ref that does
// not exist is an error, not a count of zero.
func TestStatusAheadBehindRealGit(t *testing.T) {
	src, _ := newRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	plainGit(t, "", "clone", "-q", src, dir)
	commit(t, src, "upstream one")
	commit(t, src, "upstream two")
	plainGit(t, dir, "fetch", "-q", "origin")
	plainGit(t, dir, "branch", "behind", "origin/main")
	plainGit(t, dir, "reset", "-q", "--hard", "origin/main~2")
	plainGit(t, dir, "branch", "--track", "ahead", "origin/main")
	plainGit(t, dir, "branch", "--track", "diverged", "origin/main")
	plainGit(t, dir, "branch", "--no-track", "lonely")
	plainGit(t, dir, "checkout", "-q", "ahead")
	commit(t, dir, "local")
	plainGit(t, dir, "checkout", "-q", "diverged")
	plainGit(t, dir, "reset", "-q", "--hard", "origin/main~1")
	commit(t, dir, "local one")
	commit(t, dir, "local two")
	plainGit(t, dir, "checkout", "-q", "main")

	r := newRunner(t, Sandbox)
	check := noWrite(t, dir)
	branches, err := r.Branches(t.Context(), dir)
	require.NoError(t, err)
	upstream := map[string]string{}
	for _, b := range branches {
		upstream[b.Name] = b.Upstream
	}
	assert.Equal(t, "refs/remotes/origin/main", upstream["refs/heads/ahead"])
	assert.Empty(t, upstream["refs/heads/lonely"], "a branch with no upstream says so")

	for _, tc := range []struct {
		ref           string
		ahead, behind int
	}{
		{"refs/heads/ahead", 1, 0},
		{"refs/heads/main", 0, 2}, // the local default against origin/<default>
		{"refs/heads/diverged", 2, 1},
		{"refs/heads/behind", 0, 0},
	} {
		ahead, behind, err := r.AheadBehind(t.Context(), dir, tc.ref, "refs/remotes/origin/main")
		require.NoError(t, err, tc.ref)
		assert.Equal(t, [2]int{tc.ahead, tc.behind}, [2]int{ahead, behind}, tc.ref)
	}
	ahead, behind, err := r.AheadBehind(t.Context(), dir, "HEAD", "refs/heads/ahead")
	require.NoError(t, err)
	assert.Equal(t, [2]int{0, 3}, [2]int{ahead, behind})
	_, _, err = r.AheadBehind(t.Context(), dir, "refs/heads/lonely", "refs/remotes/origin/lonely")
	require.Error(t, err, "a missing upstream is an error, not zero")
	check()
}

// TestStatusDefaultBranchRealGit reads origin's default branch from
// refs/remotes/origin/HEAD, as a clone writes it and as set-head moves
// it, and gives "" when the ref is missing or not symbolic.
func TestStatusDefaultBranchRealGit(t *testing.T) {
	src, _ := newRepo(t)
	plainGit(t, src, "branch", "release/2026")
	dir := filepath.Join(t.TempDir(), "clone")
	plainGit(t, "", "clone", "-q", src, dir)
	r := newRunner(t, Sandbox)

	check := noWrite(t, dir)
	got, err := r.DefaultBranch(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, "main", got)
	check()

	plainGit(t, dir, "remote", "set-head", "origin", "release/2026")
	got, err = r.DefaultBranch(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, "release/2026", got)

	plainGit(t, dir, "remote", "set-head", "origin", "--delete")
	got, err = r.DefaultBranch(t.Context(), dir)
	require.NoError(t, err)
	assert.Empty(t, got)

	plainGit(t, dir, "update-ref", "--no-deref", "refs/remotes/origin/HEAD", plainGit(t, dir, "rev-parse", "HEAD"))
	got, err = r.DefaultBranch(t.Context(), dir)
	require.NoError(t, err)
	assert.Empty(t, got, "a ref that is not symbolic names no branch")

	plainGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/heads/main")
	_, err = r.DefaultBranch(t.Context(), dir)
	require.ErrorIs(t, err, ErrRefused, "a target outside refs/remotes/origin/")
}

// TestStatusStashesRealGit lists zero, one and many stashes, newest
// first, with their messages.
func TestStatusStashesRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	commitFile(t, dir, "a", "one\n", "a", "2026-02-01T00:00:00Z")
	r := newRunner(t, Sandbox)

	check := noWrite(t, dir)
	got, err := r.Stashes(t.Context(), dir)
	require.NoError(t, err)
	assert.Empty(t, got)
	check()

	var want []string
	for i, msg := range []string{"first", "second", "third"} {
		writeFile(t, dir, "a", msg+"\n")
		plainGit(t, dir, "stash", "push", "-q", "-m", msg)
		want = slices.Insert(want, 0, plainGit(t, dir, "rev-parse", "refs/stash"))
		check = noWrite(t, dir)
		got, err = r.Stashes(t.Context(), dir)
		require.NoError(t, err)
		check()
		require.Len(t, got, i+1)
		assert.Equal(t, "On main: "+msg, got[0].Subject)
		var oids []string
		for _, s := range got {
			oids = append(oids, s.OID)
		}
		assert.Equal(t, want, oids)
	}
}

// TestStatusWorktreesRealGit lists the main worktree, a linked one on a
// branch, a detached one and a prunable one whose directory is gone.
func TestStatusWorktreesRealGit(t *testing.T) {
	dir, head := newRepo(t)
	base := t.TempDir()
	linked := filepath.Join(base, "linked")
	gone := filepath.Join(base, "prunable")
	plainGit(t, dir, "worktree", "add", "-q", "-b", "side", linked)
	plainGit(t, dir, "worktree", "add", "-q", "--detach", gone)
	require.NoError(t, os.RemoveAll(gone))
	real := func(p string) string {
		r, err := filepath.EvalSymlinks(p)
		require.NoError(t, err)
		return r
	}

	check := noWrite(t, dir)
	got, err := newRunner(t, Sandbox).Worktrees(t.Context(), dir)
	require.NoError(t, err)
	check()
	require.Len(t, got, 3)
	assert.Equal(t, Worktree{Path: real(dir), OID: head, Branch: "refs/heads/main"}, got[0])
	assert.Equal(t, Worktree{Path: real(linked), OID: head, Branch: "refs/heads/side"}, got[1])
	assert.Equal(t, filepath.Join(real(base), "prunable"), got[2].Path)
	assert.True(t, got[2].Prunable)
	assert.True(t, got[2].Detached)

	fromLinked, err := newRunner(t, Sandbox).Worktrees(t.Context(), linked)
	require.NoError(t, err)
	assert.Equal(t, got, fromLinked, "a linked worktree lists the same set")

	unborn := t.TempDir()
	plainGit(t, unborn, "init", "-q")
	one, err := newRunner(t, Sandbox).Worktrees(t.Context(), unborn)
	require.NoError(t, err)
	assert.Equal(t, []Worktree{{Path: real(unborn), Branch: "refs/heads/main"}}, one)
}

// TestStatusHooksPathRealGit reads core.hooksPath from the repository's
// own config only: neither a value planted in the global config nor
// the /dev/null both modes force is reported; a local value is, from
// the main worktree and from a linked one; a value in a linked
// worktree's own config.worktree is not.
func TestStatusHooksPathRealGit(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			home := t.TempDir()
			writeFile(t, home, ".gitconfig", "[core]\n\thooksPath = /global/hooks\n")
			r, err := New(Config{Mode: mode, Git: gitPath(t), Home: home})
			require.NoError(t, err)

			check := noWrite(t, dir)
			path, set, err := r.HooksPath(t.Context(), dir)
			require.NoError(t, err)
			assert.False(t, set, "got %q", path)
			check()

			plainGit(t, dir, "config", "core.hooksPath", ".githooks")
			linked := filepath.Join(t.TempDir(), "linked")
			plainGit(t, dir, "worktree", "add", "-q", "-b", "side", linked)
			check = noWrite(t, dir)
			for _, d := range []string{dir, linked} {
				path, set, err = r.HooksPath(t.Context(), d)
				require.NoError(t, err)
				assert.True(t, set)
				assert.Equal(t, ".githooks", path)
			}
			check()

			plainGit(t, dir, "config", "--unset", "core.hooksPath")
			plainGit(t, dir, "config", "extensions.worktreeConfig", "true")
			plainGit(t, linked, "config", "--worktree", "core.hooksPath", "/worktree/hooks")
			assert.Equal(t, "/worktree/hooks", plainGit(t, linked, "config", "core.hooksPath"), "the plant is live for plain git")
			_, set, err = r.HooksPath(t.Context(), linked)
			require.NoError(t, err)
			assert.False(t, set, "a per-worktree value is not reported")
		})
	}
}

// TestStatusListsAreBounded: a repository over each list's bound,
// lowered for the test, is refused, and one at it is read.
func TestStatusListsAreBounded(t *testing.T) {
	dir, _ := newRepo(t)
	commitFile(t, dir, "a", "one\n", "a", "2026-02-01T00:00:00Z")
	plainGit(t, dir, "branch", "b")
	for _, msg := range []string{"s1", "s2"} {
		writeFile(t, dir, "a", msg+"\n")
		plainGit(t, dir, "stash", "push", "-q", "-m", msg)
	}
	plainGit(t, dir, "worktree", "add", "-q", "--detach", filepath.Join(t.TempDir(), "w"))
	r := newRunner(t, Sandbox)
	reads := []struct {
		name  string
		bound *int
		read  func() (int, error)
	}{
		{"branches", &maxBranches, func() (int, error) { b, err := r.Branches(t.Context(), dir); return len(b), err }},
		{"stashes", &maxStashes, func() (int, error) { s, err := r.Stashes(t.Context(), dir); return len(s), err }},
		{"worktrees", &maxWorktrees, func() (int, error) { w, err := r.Worktrees(t.Context(), dir); return len(w), err }},
	}
	for _, tc := range reads {
		t.Run(tc.name, func(t *testing.T) {
			lower(t, tc.bound, 2)
			n, err := tc.read()
			require.NoError(t, err)
			assert.Equal(t, 2, n, "at the bound")
			lower(t, tc.bound, 1)
			_, err = tc.read()
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), "more than 1 "+tc.name)
		})
	}
	t.Run("record", func(t *testing.T) {
		lower(t, &maxStatusRecord, 8)
		_, err := r.Worktrees(t.Context(), dir)
		require.ErrorIs(t, err, ErrRefused)
		assert.Contains(t, err.Error(), "longer than 8 bytes")
	})
}

// TestStatusStandardErrorIsBounded: the standard error of a git that
// fails reaches the error of each read cut to stderrLimit and escaped.
func TestStatusStandardErrorIsBounded(t *testing.T) {
	git := fakeGit(t, `printf 'fatal: \033[2J'; i=0; while [ $i -lt 4000 ]; do printf 'filler '; i=$((i+1)); done >&2; exit 128`+"\n")
	r, err := New(Config{Mode: Sandbox, Git: git, Home: t.TempDir()})
	require.NoError(t, err)
	ctx := t.Context()
	_, statusErr := r.Status(ctx, stubRepo)
	_, branchErr := r.Branches(ctx, stubRepo)
	_, _, abErr := r.AheadBehind(ctx, stubRepo, "HEAD", "refs/remotes/origin/main")
	_, defaultErr := r.DefaultBranch(ctx, stubRepo)
	_, stashErr := r.Stashes(ctx, stubRepo)
	_, wtErr := r.Worktrees(ctx, stubRepo)
	_, _, hooksErr := r.HooksPath(ctx, stubRepo)
	for _, err := range []error{statusErr, branchErr, abErr, defaultErr, stashErr, wtErr, hooksErr} {
		require.Error(t, err)
		assert.Less(t, len(err.Error()), stderrLimit+256)
		assert.NotContains(t, err.Error(), "\x1b")
	}
}

// statusAnswers returns a start function that records each command and
// answers each read with a valid output.
func statusAnswers(commands *[]procgroup.Command) func(context.Context, procgroup.Command) error {
	oid := strings.Repeat("a", 40)
	return func(_ context.Context, c procgroup.Command) error {
		*commands = append(*commands, c)
		out := ""
		switch {
		case slices.Contains(c.Argv, "symbolic-ref"):
			out = "refs/heads/main\n"
		case slices.Contains(c.Argv, "status"):
			out = "# branch.oid " + oid + "\x00# branch.head main\x00"
		case slices.Contains(c.Argv, "ls-files"):
			out = "H a\x00"
		case slices.Contains(c.Argv, "--count=1"):
			out = "refs/remotes/origin/main " + oid + "\n"
		case slices.Contains(c.Argv, "rev-list"):
			out = "1\t2\n"
		case slices.Contains(c.Argv, "--get-regexp"):
			out = "filter.lfs.clean\x00filter.lfs.smudge\x00filter.a.b.process\x00"
		case slices.Contains(c.Argv, "--get"):
			out = ".githooks\x00"
		}
		_, err := io.WriteString(c.Stdout, out)
		return err
	}
}

// TestStatusArgv pins what each read runs: one git command each, and
// four for Status (the filter drivers, HEAD, the working tree and the
// index flags), all
// with GIT_OPTIONAL_LOCKS=0, the settings of History and the flags of
// the mode; Status's last command empties every filter driver listed.
func TestStatusArgv(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			var commands []procgroup.Command
			r := newRunner(t, mode)
			r.start = statusAnswers(&commands)
			ctx := t.Context()
			_, err := r.Status(ctx, stubRepo)
			require.NoError(t, err)
			_, err = r.Branches(ctx, stubRepo)
			require.NoError(t, err)
			_, _, err = r.AheadBehind(ctx, stubRepo, "refs/heads/main", "refs/remotes/origin/main")
			require.NoError(t, err)
			_, err = r.DefaultBranch(ctx, stubRepo)
			require.NoError(t, err)
			_, err = r.Stashes(ctx, stubRepo)
			require.NoError(t, err)
			_, err = r.Worktrees(ctx, stubRepo)
			require.NoError(t, err)
			_, _, err = r.HooksPath(ctx, stubRepo)
			require.NoError(t, err)
			_, ok, err := r.RefOID(ctx, stubRepo, "refs/remotes/origin/main")
			require.NoError(t, err)
			assert.True(t, ok)

			want := [][]string{
				{"config", "-z", "--name-only", "--get-regexp", `^filter\.`},
				{"symbolic-ref", "-q", "HEAD"},
				{"status", "--porcelain=v2", "-z", "--branch", "--no-ahead-behind", "--untracked-files=normal", "--ignore-submodules=dirty", "--no-renames"},
				{"ls-files", "-v", "-z"},
				{"for-each-ref", "--format=%(HEAD)%(objectname) %(refname) %(upstream)", "refs/heads/"},
				{"rev-list", "--left-right", "--count", "--end-of-options", "refs/heads/main...refs/remotes/origin/main", "--"},
				{"for-each-ref", "--format=%(refname) %(symref)", "refs/remotes/origin/HEAD"},
				{"stash", "list", "-z", "--format=%H %ct %gs"},
				{"worktree", "list", "--porcelain", "-z"},
				{"config", "--local", "--no-includes", "-z", "--get", "core.hooksPath"},
				{"for-each-ref", "--count=1", "--format=%(refname) %(objectname)", "--end-of-options", "refs/remotes/origin/main"},
			}
			require.Len(t, commands, len(want))
			flags := 1 + 2*(len(r.settings)+len(historySettings))
			for i, c := range commands {
				assert.Equal(t, want[i], c.Argv[flags:], "command %d", i)
				for _, flag := range []string{"core.hooksPath=/dev/null", "core.fsmonitor=false", "core.useReplaceRefs=false", "log.showSignature=false"} {
					assert.Contains(t, c.Argv, flag)
				}
				assert.Subset(t, c.Env, []string{"GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_GRAFT_FILE=/dev/null"})
				assert.Equal(t, stubRepo, c.Dir)
			}
			assert.Subset(t, commands[2].Env, []string{
				"GIT_CONFIG_COUNT=6",
				"GIT_CONFIG_KEY_0=filter.lfs.clean", "GIT_CONFIG_VALUE_0=",
				"GIT_CONFIG_KEY_1=filter.lfs.process", "GIT_CONFIG_VALUE_1=",
				"GIT_CONFIG_KEY_2=filter.lfs.required", "GIT_CONFIG_VALUE_2=false",
				"GIT_CONFIG_KEY_3=filter.a.b.clean", "GIT_CONFIG_KEY_5=filter.a.b.required",
			})
		})
	}
}

// TestStatusRefuses: a directory that is not the root of a repository,
// and a revision AheadBehind does not accept, are refused before git
// starts.
func TestStatusRefuses(t *testing.T) {
	var commands []procgroup.Command
	r := newRunner(t, Sandbox)
	r.start = statusAnswers(&commands)
	ctx := t.Context()
	for _, dir := range []string{filepath.Join(stubRepo, "sub"), "relative", ""} {
		_, err := r.Status(ctx, dir)
		require.ErrorIs(t, err, ErrRefused, dir)
		_, err = r.Branches(ctx, dir)
		require.ErrorIs(t, err, ErrRefused, dir)
		_, _, err = r.AheadBehind(ctx, dir, "HEAD", "HEAD")
		require.ErrorIs(t, err, ErrRefused, dir)
		_, err = r.DefaultBranch(ctx, dir)
		require.ErrorIs(t, err, ErrRefused, dir)
		_, err = r.Stashes(ctx, dir)
		require.ErrorIs(t, err, ErrRefused, dir)
		_, err = r.Worktrees(ctx, dir)
		require.ErrorIs(t, err, ErrRefused, dir)
		_, _, err = r.HooksPath(ctx, dir)
		require.ErrorIs(t, err, ErrRefused, dir)
	}
	for _, rev := range []string{"", "-p", "--all", "main", "HEAD~1", "refs/heads/a..b", "origin/main"} {
		_, _, err := r.AheadBehind(ctx, stubRepo, rev, "HEAD")
		require.ErrorIs(t, err, ErrRefused, rev)
		_, _, err = r.AheadBehind(ctx, stubRepo, "HEAD", rev)
		require.ErrorIs(t, err, ErrRefused, rev)
	}
	assert.Empty(t, commands)
}

// TestStatusRefusesABadAnswer: an answer of git that is not the shape
// asked for, or that names a ref or path gitsafe refuses, is refused.
func TestStatusRefusesABadAnswer(t *testing.T) {
	oid := strings.Repeat("a", 40)
	tests := []struct {
		name string
		out  string
		read func(r *Runner) error
	}{
		{"status without branch.oid", "# branch.head main\x00? x\x00", func(r *Runner) error { _, err := r.Status(t.Context(), stubRepo); return err }},
		{"status rename record", "# branch.oid " + oid + "\x00# branch.head main\x002 R. x\x00", func(r *Runner) error { _, err := r.Status(t.Context(), stubRepo); return err }},
		{"branch outside refs/heads", " " + oid + " refs/tags/v1 \n", func(r *Runner) error { _, err := r.Branches(t.Context(), stubRepo); return err }},
		{"branch bad upstream", " " + oid + " refs/heads/a origin/a\n", func(r *Runner) error { _, err := r.Branches(t.Context(), stubRepo); return err }},
		{"branch short id", " abc refs/heads/a \n", func(r *Runner) error { _, err := r.Branches(t.Context(), stubRepo); return err }},
		{"rev-list", "1 2\n", func(r *Runner) error { _, _, err := r.AheadBehind(t.Context(), stubRepo, "HEAD", "HEAD"); return err }},
		{"HEAD moved", "# branch.oid " + oid + "\x00# branch.head other\x00", func(r *Runner) error { _, err := r.Status(t.Context(), stubRepo); return err }},
		{"branch bidi override", " " + oid + " refs/heads/a\u202eb \n", func(r *Runner) error { _, err := r.Branches(t.Context(), stubRepo); return err }},
		{"upstream line separator", " " + oid + " refs/heads/a refs/remotes/origin/a\u2028b\n", func(r *Runner) error { _, err := r.Branches(t.Context(), stubRepo); return err }},
		{"default branch zero width space", "refs/remotes/origin/HEAD refs/remotes/origin/ma\u200bin\n", func(r *Runner) error { _, err := r.DefaultBranch(t.Context(), stubRepo); return err }},
		{"worktree branch C1 control", "worktree /a\x00branch refs/heads/a\u0085b\x00\x00", func(r *Runner) error { _, err := r.Worktrees(t.Context(), stubRepo); return err }},
		{"ref oid short id", "refs/heads/a abc\n", func(r *Runner) error { _, _, err := r.RefOID(t.Context(), stubRepo, "refs/heads/a"); return err }},
		{"stash bad id", "abc 1 On main: x\x00", func(r *Runner) error { _, err := r.Stashes(t.Context(), stubRepo); return err }},
		{"worktree relative path", "worktree rel\x00\x00", func(r *Runner) error { _, err := r.Worktrees(t.Context(), stubRepo); return err }},
		{"worktree control character", "worktree /a\x1bb\x00\x00", func(r *Runner) error { _, err := r.Worktrees(t.Context(), stubRepo); return err }},
		{"worktree attribute first", "HEAD " + oid + "\x00", func(r *Runner) error { _, err := r.Worktrees(t.Context(), stubRepo); return err }},
		{"hooks path control character", "/a\x1bb\x00", func(r *Runner) error { _, _, err := r.HooksPath(t.Context(), stubRepo); return err }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newRunner(t, Sandbox)
			r.start = func(_ context.Context, c procgroup.Command) error {
				out := tc.out
				if slices.Contains(c.Argv, "symbolic-ref") {
					out = "refs/heads/main\n"
				} else if slices.Contains(c.Argv, "--get-regexp") {
					out = ""
				}
				_, err := io.WriteString(c.Stdout, out)
				return err
			}
			require.ErrorIs(t, tc.read(r), ErrRefused)
		})
	}
}

// TestStatusRefOIDRealGit reads the tip of a ref, before and after it
// moves; a missing ref, and one missing while a ref below its name
// exists, give false and no error; a name gitsafe refuses is refused
// before git starts.
func TestStatusRefOIDRealGit(t *testing.T) {
	src, _ := newRepo(t)
	dir := filepath.Join(t.TempDir(), "clone")
	plainGit(t, "", "clone", "-q", src, dir)
	r := newRunner(t, Sandbox)
	old := plainGit(t, dir, "rev-parse", "refs/remotes/origin/main")

	check := noWrite(t, dir)
	got, ok, err := r.RefOID(t.Context(), dir, "refs/remotes/origin/main")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, old, got)
	check()

	moved := commit(t, src, "upstream")
	plainGit(t, dir, "fetch", "-q", "origin")
	got, ok, err = r.RefOID(t.Context(), dir, "refs/remotes/origin/main")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, moved, got)

	plainGit(t, dir, "update-ref", "refs/x/below/y", moved)
	for _, missing := range []string{"refs/remotes/origin/none", "refs/x/below"} {
		got, ok, err = r.RefOID(t.Context(), dir, missing)
		require.NoError(t, err, missing)
		assert.False(t, ok, missing)
		assert.Empty(t, got, missing)
	}

	var commands []procgroup.Command
	r.start = statusAnswers(&commands)
	for _, bad := range []string{"", "main", "HEAD", "-p", "refs/heads/a..b", "refs/heads/a\u202eb", "refs/heads/a\u0085b"} {
		_, _, err := r.RefOID(t.Context(), dir, bad)
		require.ErrorIs(t, err, ErrRefused, bad)
	}
	assert.Empty(t, commands)
}

// TestStatusStashMessagesRealGit: a stash message is display text, so
// one that holds a zero width joiner, a tab or a byte that is not
// UTF-8 is listed escaped, and one longer than maxSubjectBytes is cut;
// none fails the list.
func TestStatusStashMessagesRealGit(t *testing.T) {
	dir, _ := newRepo(t)
	commitFile(t, dir, "a", "one\n", "a", "2026-02-01T00:00:00Z")
	long := strings.Repeat("é", maxSubjectBytes)
	msgs := []string{"family 👩\u200d👩\u200d👧", "a\tb", "bad \xff byte", long}
	for _, msg := range msgs {
		writeFile(t, dir, "a", msg+"\n")
		plainGit(t, dir, "stash", "push", "-q", "-m", msg)
	}
	check := noWrite(t, dir)
	got, err := newRunner(t, Sandbox).Stashes(t.Context(), dir)
	require.NoError(t, err)
	check()
	require.Len(t, got, len(msgs))
	for i, msg := range msgs[:3] {
		s := got[len(msgs)-1-i].Subject
		// git's reflog turns the tab into a space before gitsafe sees it.
		assert.Equal(t, termsafe.Escape("On main: "+strings.ReplaceAll(msg, "\t", " ")), s)
		assert.True(t, utf8.ValidString(s))
		assert.NotContains(t, s, "\t")
	}
	assert.LessOrEqual(t, len(got[0].Subject), maxSubjectBytes)
	assert.True(t, utf8.ValidString(got[0].Subject))
	assert.True(t, strings.HasPrefix("On main: "+long, got[0].Subject), "cut, not changed")
}

// TestStatusHiddenRealGit: a file changed under assume-unchanged or
// skip-worktree does not show in git status, so Status reports the
// flag as Hidden and the tree as not clean.
func TestStatusHiddenRealGit(t *testing.T) {
	for _, flag := range []string{"--assume-unchanged", "--skip-worktree"} {
		t.Run(flag, func(t *testing.T) {
			dir, _ := newRepo(t)
			head := commitFile(t, dir, "a", "one\n", "a", "2026-02-01T00:00:00Z")
			plainGit(t, dir, "update-index", flag, "a")
			writeFile(t, dir, "a", "changed\n")
			assert.Empty(t, plainGit(t, dir, "status", "--porcelain"), "git status hides the change")
			check := noWrite(t, dir)
			got, err := newRunner(t, Sandbox).Status(t.Context(), dir)
			require.NoError(t, err)
			check()
			assert.Equal(t, Status{Branch: "refs/heads/main", OID: head, Hidden: true}, got)
			assert.False(t, got.Clean())
		})
	}
}

// TestStatusRefusesUnsafeRefNamesRealGit: a ref name git accepts that
// holds a C1 control, a direction override, a line separator or a zero
// width space fails the read that would return it.
func TestStatusRefusesUnsafeRefNamesRealGit(t *testing.T) {
	r := newRunner(t, Sandbox)
	for _, name := range []string{"a\u0085b", "a\u202eb", "a\u2028b", "a\u200bb"} {
		t.Run(strings.ToValidUTF8(name, "?"), func(t *testing.T) {
			dir, _ := newRepo(t)
			plainGit(t, dir, "branch", name)
			_, err := r.Branches(t.Context(), dir)
			require.ErrorIs(t, err, ErrRefused)

			plainGit(t, dir, "checkout", "-q", name)
			_, err = r.Status(t.Context(), dir)
			require.ErrorIs(t, err, ErrRefused)
			_, err = r.Worktrees(t.Context(), dir)
			require.ErrorIs(t, err, ErrRefused)

			plainGit(t, dir, "update-ref", "refs/remotes/origin/"+name, "HEAD")
			plainGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+name)
			_, err = r.DefaultBranch(t.Context(), dir)
			require.ErrorIs(t, err, ErrRefused)
		})
	}
	dir, _ := newRepo(t)
	plainGit(t, dir, "config", "branch.main.remote", ".")
	plainGit(t, dir, "config", "branch.main.merge", "refs/heads/a\u202eb")
	_, err := r.Branches(t.Context(), dir)
	require.ErrorIs(t, err, ErrRefused, "an upstream")
}

// TestStatusHooksPathWithoutAValue: a core.hooksPath key with no value,
// or an empty one, is refused, not reported as a set empty path.
func TestStatusHooksPathWithoutAValue(t *testing.T) {
	dir, _ := newRepo(t)
	config := filepath.Join(dir, ".git", "config")
	f, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString("[core]\n\thooksPath\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	r := newRunner(t, Sandbox)
	path, set, err := r.HooksPath(t.Context(), dir)
	require.ErrorIs(t, err, ErrRefused)
	assert.False(t, set)
	assert.Empty(t, path)

	empty, _ := newRepo(t)
	plainGit(t, empty, "config", "core.hooksPath", "")
	_, set, err = r.HooksPath(t.Context(), empty)
	require.ErrorIs(t, err, ErrRefused)
	assert.False(t, set)
}

// TestStatusRunsNoFilterFromAnIncludeIf: a filter driver and the
// attribute that names it, both defined only in a file the local
// config includes with includeIf, are neutralized too; plain git runs
// the filter (the positive control).
//
//romeu:invariant I3 test
func TestStatusRunsNoFilterFromAnIncludeIf(t *testing.T) {
	dir, _ := newRepo(t)
	writeFile(t, dir, "a", "a\n")
	plainGit(t, dir, "add", "a")
	plainGit(t, dir, "commit", "-q", "-m", "a")
	marker := filepath.Join(t.TempDir(), "marker")
	extra := t.TempDir()
	script := filepath.Join(extra, "filter.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho ran >> '"+marker+"'\ncat\n"), 0o700))
	writeFile(t, extra, "attributes", "a filter=inc\n")
	writeFile(t, extra, "included", "[core]\n\tattributesFile = "+filepath.Join(extra, "attributes")+
		"\n[filter \"inc\"]\n\tclean = "+script+"\n\trequired = true\n")
	// git matches gitdir: against the resolved path, and TMPDIR sits
	// behind a symbolic link on macOS (/var to /private/var).
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	plainGit(t, dir, "config", "includeIf.gitdir:"+filepath.Join(resolved, ".git")+".path", filepath.Join(extra, "included"))
	past := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(dir, ".git", "index"), past, past))

	check := noWrite(t, dir)
	got, err := newRunner(t, Sandbox).Status(t.Context(), dir)
	require.NoError(t, err)
	assert.True(t, got.Clean(), "%+v", got)
	assert.NoFileExists(t, marker)
	check()

	_, _ = plainGitErr(t, dir, nil, "status", "--porcelain")
	assert.FileExists(t, marker, "the plant is live: plain git runs it")
}
