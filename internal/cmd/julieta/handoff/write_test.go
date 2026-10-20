// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/handoff"
	hf "github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
)

// readHandoff reads the one handoff file of the memory dir i.
func (s *sandbox) readHandoff(t *testing.T, i int, name string) hf.File {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.mem(i), "handoff", name))
	require.NoError(t, err)
	f, err := hf.Decode(data)
	require.NoError(t, err)
	return f
}

// TestWriteRefusesANarrativeWithoutItsHeadings refuses a narrative
// without its headings (five for clear, six for final) as the error
// usage, and writes nothing (REQ-0245, REQ-0413).
func TestWriteRefusesANarrativeWithoutItsHeadings(t *testing.T) {
	s := newSandbox(t)
	for _, c := range []struct {
		args  []string
		body  string
		heads string
	}{
		{[]string{"handoff", "write", "--json"}, "## Goal\nx\n", "## State"},
		{[]string{"handoff", "write", "--final", "--json"}, narrative, "## What would be lost"},
		{[]string{"handoff", "write", "--json"}, "", "## Goal"},
	} {
		code, stdout, stderr := s.julieta(t, c.body, c.args...)
		assert.Equal(t, 2, code, stdout)
		assert.Equal(t, "RJ-349", errorID(t, stderr))
		assert.Contains(t, stderr, c.heads)
	}
	code, _, stderr := s.julieta(t, narrative, "handoff", "write", "--final", "--facts")
	assert.Equal(t, 2, code, stderr)
	assert.Empty(t, s.names(t, 0))
}

// TestWriteStampsTheFrontMatter writes a clear and a final handoff: the
// file <ULID>-<kind>.md in the memory dir of the repo of the working
// directory, 0600 in a dir 0700, the front matter stamped by julieta
// with the git facts of every repo of the manifest, and the id, the path
// and the repo printed, in text and with --json (REQ-0278, REQ-0413,
// REQ-0677, REQ-0678, REQ-0679).
func TestWriteStampsTheFrontMatter(t *testing.T) {
	s := newSandbox(t)
	head := git(t, s.repo(0), "rev-parse", "HEAD")
	require.NoError(t, os.WriteFile(filepath.Join(s.repo(0), "new.txt"), []byte("x"), 0o600))

	out := s.ok(t, narrative, "handoff", "write", "--json")
	rep, err := handoff.Decode([]byte(out))
	require.NoError(t, err, out)
	require.Len(t, rep.Files, 1)
	f := rep.Files[0]
	assert.Regexp(t, `^[0-9a-hjkmnp-tv-z]{26}$`, f.ID)
	assert.Equal(t, "clear", f.Kind)
	assert.Equal(t, "api", f.Repo)
	assert.Equal(t, filepath.Join(s.mem(0), "handoff", f.ID+"-clear.md"), f.Path)
	info, err := os.Stat(f.Path)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode())
	info, err = os.Stat(filepath.Dir(f.Path))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())

	got := s.readHandoff(t, 0, f.ID+"-clear.md")
	assert.Equal(t, hf.Front{Schema: "handoff/v1", Kind: hf.Clear, CreatedAt: s.now, Project: "shop", Generation: s.m.Generation,
		Agent: "sbx-kit-claude", Repos: []hf.RepoFacts{
			{Dir: "api", Branch: "main", Head: head, Dirty: 1},
			{Dir: "web", Branch: "main", Head: git(t, s.repo(1), "rev-parse", "HEAD")},
		}}, got.Front)
	assert.Equal(t, narrative, got.Body)
	assert.Empty(t, s.names(t, 1), "filed under the repo of the working directory only")

	s.advance(time.Minute)
	text := s.ok(t, final, "handoff", "write", "--final", "--repo", "web")
	assert.Regexp(t, `^wrote [0-9a-hjkmnp-tv-z]{26} final `+s.mem(1)+`/handoff/[0-9a-hjkmnp-tv-z]{26}-final\.md repo web\n$`, text)
	require.Len(t, s.names(t, 1), 1)
	assert.Equal(t, hf.Final, s.readHandoff(t, 1, s.names(t, 1)[0]).Kind)
}

// TestWriteFactsHasNoBody writes a facts handoff: no body, and no stdin
// read (REQ-0245, REQ-0668).
func TestWriteFactsHasNoBody(t *testing.T) {
	s := newSandbox(t)
	out := s.ok(t, "ignored", "handoff", "write", "--facts")
	assert.Regexp(t, `^wrote [0-9a-hjkmnp-tv-z]{26} facts `, out)
	require.Len(t, s.names(t, 0), 1)
	f := s.readHandoff(t, 0, s.names(t, 0)[0])
	assert.Equal(t, hf.Facts, f.Kind)
	assert.Empty(t, f.Body)
	assert.Len(t, f.Repos, 2)
}

// TestWriteFilesUnderTheRepoOfTheWorkingDirectory: the working
// directory decides, never a question, and a directory outside every
// repo is an error that names --repo (REQ-0677).
func TestWriteFilesUnderTheRepoOfTheWorkingDirectory(t *testing.T) {
	s := newSandbox(t)
	t.Chdir(s.repo(1))
	s.ok(t, "", "handoff", "write", "--facts")
	assert.Len(t, s.names(t, 1), 1)
	assert.Empty(t, s.names(t, 0))
	t.Chdir(t.TempDir())
	code, _, stderr := s.julieta(t, "", "handoff", "write", "--facts")
	assert.Equal(t, 0, code, "the SessionEnd hook always exits 0")
	_ = stderr
	code, _, stderr = s.julieta(t, narrative, "handoff", "write")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "--repo")
}

// TestWriteFactsWaitsForTheStoreLock holds the store lock of the target
// dir, as a memory add or a snapshot does, and runs handoff write
// --facts: it waits until the lock is released, and it takes the lock
// before its first git read (REQ-0655). The seams tell when the command
// has met the held lock and when it reaches git, so no test sleeps.
func TestWriteFactsWaitsForTheStoreLock(t *testing.T) {
	s := newSandbox(t)
	lock := write.LockPath(s.home, "api")
	require.NoError(t, os.MkdirAll(filepath.Dir(lock), 0o700))
	lf, err := os.OpenFile(lock, os.O_RDWR|os.O_CREATE, 0o600)
	require.NoError(t, err)
	defer func() { _ = lf.Close() }()
	require.NoError(t, unix.Flock(int(lf.Fd()), unix.LOCK_EX))

	var mu sync.Mutex
	var events []string
	note := func(e string) { mu.Lock(); events = append(events, e); mu.Unlock() }
	waiting := make(chan string, 1)
	defer handoff.SetOnWait(func(l string) { note("waiting"); waiting <- l })()
	defer handoff.SetBeforeGit(func() { note("git") })()

	done := make(chan string, 1)
	go func() {
		code, stdout, stderr := s.julieta(t, "", "handoff", "write", "--facts")
		done <- fmt.Sprintf("%d %s %s", code, stdout, stderr)
	}()
	assert.Equal(t, lock, <-waiting, "the command met the held lock of its target dir")
	select {
	case got := <-done:
		t.Fatalf("handoff write --facts finished while the store lock was held: %s", got)
	default:
	}
	assert.Empty(t, s.names(t, 0), "nothing written while the lock is held")
	require.NoError(t, unix.Flock(int(lf.Fd()), unix.LOCK_UN))
	got := <-done
	assert.True(t, strings.HasPrefix(got, "0 wrote "), got)
	assert.Len(t, s.names(t, 0), 1)
	mu.Lock()
	assert.Equal(t, []string{"waiting", "git"}, events, "the lock is taken before the first git read")
	mu.Unlock()

	// The lock is released at exit: this run does not wait.
	s.ok(t, "", "handoff", "write", "--facts")
}

// TestWriteKeepsTheNewestFiftyFacts: after each facts write the newest
// 50 facts files stay, every narrative stays, and the newest facts file
// is never removed (REQ-0649, 08 8.1, 8.6).
func TestWriteKeepsTheNewestFiftyFacts(t *testing.T) {
	s := newSandbox(t)
	dir := filepath.Join(s.mem(0), "handoff")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	src := canon.NewULIDSource(strings.NewReader(strings.Repeat("\x01", 1<<12)))
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var narratives []string
	for i := range 60 {
		kind := "facts"
		if i%20 == 0 {
			kind = []string{"clear", "final"}[i/20%2]
		}
		u, err := canon.NewULID(old.Add(time.Duration(i)*time.Minute), src)
		require.NoError(t, err)
		name := u.String() + "-" + kind + ".md"
		if kind != "facts" {
			narratives = append(narratives, name)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600))
	}
	out := s.ok(t, "", "handoff", "write", "--facts", "--json")
	rep, err := handoff.Decode([]byte(out))
	require.NoError(t, err)
	newest := filepath.Base(rep.Files[0].Path)

	var facts []string
	have := map[string]bool{}
	for _, n := range s.names(t, 0) {
		have[n] = true
		if strings.HasSuffix(n, "-facts.md") {
			facts = append(facts, n)
		}
	}
	assert.Len(t, facts, hf.KeepFacts)
	for _, n := range narratives {
		assert.True(t, have[n], "the narrative %s stays", n)
	}
	assert.True(t, have[newest], "the newest facts file stays")
	// 58 facts files (57 old and the new one) became 50: the 8 oldest
	// went.
	assert.Len(t, s.names(t, 0), hf.KeepFacts+len(narratives))
}
