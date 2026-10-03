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
		if err != nil {
			t.Fatalf("read the terminal's settings: %v", err)
		}
		if tio.Lflag&unix.ECHO == 0 {
			break
		}
		select {
		case code = <-runDone:
			ended = true
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the command did not switch echo off")
		}
		time.Sleep(time.Millisecond)
	}
	if !ended {
		// Both lines go in at once: the terminal holds the second
		// until the command asks for it.
		if _, err := io.WriteString(master, strings.Join(typed, "\n")+"\n"); err != nil {
			t.Fatalf("type the name: %v", err)
		}
		select {
		case code = <-runDone:
		case <-time.After(10 * time.Second):
			t.Fatal("the command did not end")
		}
	}
	if err := terminal.Close(); err != nil {
		t.Fatalf("close the terminal: %v", err)
	}
	select {
	case <-readDone:
	case <-time.After(10 * time.Second):
		t.Fatal("the terminal did not close")
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
			t.Errorf("close %s: %v", f.Name(), err)
		}
	})
}

func loadDenylistFile(t *testing.T, r *gittest.Repo) *names.List {
	t.Helper()
	l, err := names.Load(filepath.Join(r.Dir, filepath.FromSlash(names.Path)))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// TestAddAtATerminal drives "hygiene add" through /dev/ptmx (10 10.1).
func TestAddAtATerminal(t *testing.T) {
	r := newTree(t)
	if err := os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))); err != nil {
		t.Fatal(err)
	}
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
		if code != s.code {
			t.Errorf("%s: exit = %d, want %d\n%s", s.name, code, s.code, printed)
		}
		if got := loadDenylistFile(t, r).Len(); got != s.entries {
			t.Errorf("%s: the denylist holds %d entries, want %d", s.name, got, s.entries)
		}
		// The name is neither echoed by the terminal nor printed by
		// the command.
		for _, piece := range []string{"zorvex", "quimby", "plimsor", "brimlow", "brimlaw"} {
			if strings.Contains(strings.ToLower(printed+shown), piece) {
				t.Errorf("%s: the name was shown:\nprinted: %q\nterminal: %q", s.name, printed, shown)
			}
		}
	}

	l := loadDenylistFile(t, r)
	for _, line := range []string{"zorvex-quimby", "ZORVEXQUIMBY", "a plimsor b"} {
		if !l.Match([]byte(line)) {
			t.Errorf("the written denylist does not match %q", line)
		}
	}
	data, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(names.Path)))
	if err != nil {
		t.Fatal(err)
	}
	for _, piece := range []string{"zorvex", "quimby", "plimsor"} {
		if strings.Contains(strings.ToLower(string(data)), piece) {
			t.Errorf("the denylist file holds %q", piece)
		}
	}
	// With the entries written, the tree passes.
	r.Commit("denylist")
	if code, out := runCI(t, r, nil, nil, "hygiene"); code != exitOK {
		t.Errorf("hygiene after add: exit = %d\n%s", code, out)
	}
}

// TestAddKeepsADenylistItCannotRead shows that a denylist that does
// not parse stops the command: it is not taken for a missing file and
// written over.
func TestAddKeepsADenylistItCannotRead(t *testing.T) {
	r := newTree(t)
	const damaged = "entries: [\n"
	r.Write(names.Path, damaged)
	code, printed, _ := typeAtTerminal(t, r, "zorvex quimby", "zorvex quimby")
	if code != exitError {
		t.Errorf("exit = %d, want %d\n%s", code, exitError, printed)
	}
	got, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(names.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != damaged {
		t.Errorf("the denylist was rewritten: %q", got)
	}
}

// TestWriteFile covers the three promises of the denylist's writer:
// the content and mode, a replacement that never edits the old file in
// place, and no temporary file left behind when it fails.
func TestWriteFile(t *testing.T) {
	leftovers := func(t *testing.T, dir string) []string {
		t.Helper()
		found, err := filepath.Glob(filepath.Join(dir, ".denylist-*"))
		if err != nil {
			t.Fatal(err)
		}
		return found
	}

	t.Run("content and mode", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "denylist.yaml")
		if err := writeFile(path, []byte("new\n")); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new\n" {
			t.Errorf("content = %q", got)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("mode = %o, want 644", info.Mode().Perm())
		}
		if l := leftovers(t, dir); len(l) != 0 {
			t.Errorf("temporary files left: %v", l)
		}
	})

	t.Run("the old file is replaced, not edited", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "denylist.yaml")
		other := filepath.Join(dir, "other-name")
		if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A second name for the old file still reads the old content
		// after a rename, and the new content after a write in place.
		if err := os.Link(path, other); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(path, []byte("new\n")); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(other)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "old\n" {
			t.Errorf("the old file was edited in place: %q", got)
		}
	})

	t.Run("a failed write leaves nothing behind", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "denylist.yaml")
		if err := os.Mkdir(path, 0o755); err != nil { // a directory cannot be replaced by a file
			t.Fatal(err)
		}
		if err := writeFile(path, []byte("new\n")); err == nil {
			t.Error("writeFile over a directory succeeded")
		}
		if l := leftovers(t, dir); len(l) != 0 {
			t.Errorf("temporary files left: %v", l)
		}
	})
}

// TestAddRefuses covers the two ways a name could reach the command
// without a person typing it: an argument, and input that is no
// terminal.
func TestAddRefuses(t *testing.T) {
	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pipeWrite.WriteString("zorvex quimby\n"); err != nil {
		t.Fatal(err)
	}
	if err := pipeWrite.Close(); err != nil {
		t.Fatal(err)
	}
	closeAtEnd(t, pipeRead)
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	closeAtEnd(t, file)
	// An empty line waits in the terminal, so a command that read it
	// after all would end with another message, and not wait for ever.
	master, terminal := openPTY(t)
	if _, err := io.WriteString(master, "\n"); err != nil {
		t.Fatal(err)
	}

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
			if code != exitError {
				t.Errorf("exit = %d, want %d", code, exitError)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("the output lacks %q:\n%s", tt.want, out)
			}
			if strings.Contains(out, "zorvex") {
				t.Errorf("the output repeats the argument:\n%s", out)
			}
			if after := loadDenylistFile(t, r).Marshal(); !bytes.Equal(before, after) {
				t.Error("the denylist changed")
			}
		})
	}
}
