// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux || darwin

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

// typeAtTerminal runs "hygiene add" with a pseudo-terminal as standard
// input, types the lines into it, and returns the exit status, what
// the command printed, and what the terminal showed. A command that
// ends before it reads the terminal is not typed to.
func typeAtTerminal(t *testing.T, r *gittest.Repo, typed ...string) (code int, printed, shown string) {
	t.Helper()
	master, terminal := openPTY(t)

	// The master is read from the start, so the terminal never blocks
	// on output. The read ends when the last terminal handle closes.
	var mu sync.Mutex
	var screen bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 1024)
		for {
			n, err := master.Read(buf)
			mu.Lock()
			screen.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	var out bytes.Buffer
	e := env{dir: r.Dir, gitEnv: r.Env, stdin: terminal, stdout: &out, stderr: &out}
	runDone := make(chan int, 1)
	go func() { runDone <- run(t.Context(), e, []string{"hygiene", "add"}) }()

	// The name is typed once the command has switched echo off. Typed
	// earlier it would still be read, but the terminal would show it,
	// and the test would not tell a working command from a broken one.
	deadline := time.Now().Add(10 * time.Second)
	ended := false
	for !ended {
		tio, err := unix.IoctlGetTermios(int(terminal.Fd()), getTermios)
		require.NoError(t, err, "read the terminal's settings")
		if tio.Lflag&unix.ECHO == 0 {
			break
		}
		select {
		case code = <-runDone:
			ended = true
		default:
		}
		require.False(t, time.Now().After(deadline), "the command did not switch echo off")
		time.Sleep(time.Millisecond)
	}
	if !ended {
		// Both lines go in at once: the terminal holds the second
		// until the command asks for it.
		_, err := io.WriteString(master, strings.Join(typed, "\n")+"\n")
		require.NoError(t, err, "type the name")
		select {
		case code = <-runDone:
		case <-time.After(10 * time.Second):
			require.FailNow(t, "the command did not end")
		}
	}
	require.NoError(t, terminal.Close(), "close the terminal")
	select {
	case <-readDone:
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the terminal did not close")
	}
	mu.Lock()
	defer mu.Unlock()
	return code, out.String(), screen.String()
}

// closeAtEnd closes f when the test ends. A file the test closed
// itself is left alone.
func closeAtEnd(t *testing.T, f *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			require.NoError(t, err, "close %s", f.Name())
		}
	})
}

func loadDenylistFile(t *testing.T, r *gittest.Repo) *names.List {
	t.Helper()
	l, err := names.Load(filepath.Join(r.Dir, filepath.FromSlash(names.Path)))
	require.NoError(t, err)
	return l
}

// TestAddAtATerminal drives "hygiene add" through /dev/ptmx (10 10.1).
func TestAddAtATerminal(t *testing.T) {
	r := newTree(t)
	require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
	r.Commit("fixture")

	// The name is typed twice, because it is typed blind: a slip of
	// the keyboard would write an entry that matches nothing, and no
	// check could see it.
	steps := []struct {
		name    string
		typed   []string
		code    int
		entries int
	}{
		{"the first entry creates the file", []string{"Zorvex Quimby", "Zorvex Quimby"}, exitOK, 1},
		{"a second entry is appended", []string{"plimsor", "plimsor"}, exitOK, 2},
		{"the same name again changes nothing", []string{"zorvex  quimby", "zorvex  quimby"}, exitOK, 2},
		{"two lines that differ are refused", []string{"brimlow", "brimlaw"}, exitError, 2},
		{"a separator inside a segment is refused", []string{"zorvex-quimby", "zorvex-quimby"}, exitError, 2},
		{"an empty line is refused", []string{"", ""}, exitError, 2},
	}
	for _, s := range steps {
		code, printed, shown := typeAtTerminal(t, r, s.typed...)
		assert.Equal(t, s.code, code, "%s: exit status\n%s", s.name, printed)
		assert.Equal(t, s.entries, loadDenylistFile(t, r).Len(), "%s: entries in the denylist", s.name)
		// The name is neither echoed by the terminal nor printed by
		// the command.
		for _, piece := range []string{"zorvex", "quimby", "plimsor", "brimlow", "brimlaw"} {
			assert.NotContains(t, strings.ToLower(printed+shown), piece,
				"%s: the name was shown, in what the command printed and the terminal showed", s.name)
		}
	}

	l := loadDenylistFile(t, r)
	for _, line := range []string{"zorvex-quimby", "ZORVEXQUIMBY", "a plimsor b"} {
		assert.True(t, l.Match([]byte(line)), "the written denylist matches %q", line)
	}
	data, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(names.Path)))
	require.NoError(t, err)
	for _, piece := range []string{"zorvex", "quimby", "plimsor"} {
		assert.NotContains(t, strings.ToLower(string(data)), piece, "the denylist file")
	}
	// With the entries written, the tree passes.
	r.Commit("denylist")
	code, out := runCI(t, r, nil, nil, "hygiene")
	assert.Equal(t, exitOK, code, "hygiene after add\n%s", out)
}

// TestAddKeepsADenylistItCannotRead shows that a denylist that does
// not parse stops the command: it is not taken for a missing file and
// written over.
func TestAddKeepsADenylistItCannotRead(t *testing.T) {
	r := newTree(t)
	const damaged = "entries: [\n"
	r.Write(names.Path, damaged)
	code, printed, _ := typeAtTerminal(t, r, "zorvex quimby", "zorvex quimby")
	assert.Equal(t, exitError, code, "exit status\n%s", printed)
	got, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(names.Path)))
	require.NoError(t, err)
	assert.Equal(t, damaged, string(got), "the denylist was rewritten")
}

// TestWriteFile covers the three promises of the denylist's writer:
// the content and mode, a replacement that never edits the old file in
// place, and no temporary file left behind when it fails.
func TestWriteFile(t *testing.T) {
	leftovers := func(t *testing.T, dir string) []string {
		t.Helper()
		found, err := filepath.Glob(filepath.Join(dir, ".denylist-*"))
		require.NoError(t, err)
		return found
	}

	t.Run("content and mode", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "denylist.yaml")
		require.NoError(t, writeFile(path, []byte("new\n")))
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "new\n", string(got), "content")
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "mode")
		assert.Empty(t, leftovers(t, dir), "temporary files left")
	})

	t.Run("the old file is replaced, not edited", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "denylist.yaml")
		other := filepath.Join(dir, "other-name")
		require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o644))
		// A second name for the old file still reads the old content
		// after a rename, and the new content after a write in place.
		require.NoError(t, os.Link(path, other))
		require.NoError(t, writeFile(path, []byte("new\n")))
		got, err := os.ReadFile(other)
		require.NoError(t, err)
		assert.Equal(t, "old\n", string(got), "the old file was edited in place")
	})

	t.Run("a failed write leaves nothing behind", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "denylist.yaml")
		require.NoError(t, os.Mkdir(path, 0o755)) // a directory cannot be replaced by a file
		require.Error(t, writeFile(path, []byte("new\n")), "writeFile over a directory")
		assert.Empty(t, leftovers(t, dir), "temporary files left")
	})
}

// TestAddRefuses covers the two ways a name could reach the command
// without a person typing it: an argument, and input that is no
// terminal.
func TestAddRefuses(t *testing.T) {
	pipeRead, pipeWrite, err := os.Pipe()
	require.NoError(t, err)
	_, err = pipeWrite.WriteString("zorvex quimby\n")
	require.NoError(t, err)
	require.NoError(t, pipeWrite.Close())
	closeAtEnd(t, pipeRead)
	file, err := os.Open(os.DevNull)
	require.NoError(t, err)
	closeAtEnd(t, file)
	// An empty line waits in the terminal, so a command that read it
	// after all would end with another message, and not wait for ever.
	master, terminal := openPTY(t)
	_, err = io.WriteString(master, "\n")
	require.NoError(t, err)

	tests := []struct {
		name  string
		stdin io.Reader
		args  []string
		want  string
	}{
		{"a pipe on standard input", pipeRead, nil, "needs a terminal"},
		{"a file on standard input", file, nil, "needs a terminal"},
		{"a reader that is no file", strings.NewReader("zorvex quimby\n"), nil, "needs a terminal"},
		{"a name as an argument, even at a terminal", terminal, []string{"zorvex"}, "takes no argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			before := loadDenylistFile(t, r).Marshal()
			code, out := runCI(t, r, tt.stdin, nil, append([]string{"hygiene", "add"}, tt.args...)...)
			assert.Equal(t, exitError, code, "exit status")
			assert.Contains(t, out, tt.want)
			assert.NotContains(t, out, "zorvex", "the output repeats the argument")
			assert.Equal(t, before, loadDenylistFile(t, r).Marshal(), "the denylist changed")
		})
	}
}
