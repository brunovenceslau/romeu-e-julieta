// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeAtEnd closes f when the test ends.
func closeAtEnd(t *testing.T, f *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			require.NoError(t, err, "close %s", f.Name())
		}
	})
}

// screen reads everything the terminal side of the pair writes, from
// the master, so the terminal never blocks on output; it returns what
// was read so far, with the terminal's CR LF as LF.
func screen(t *testing.T, master *os.File) func() string {
	t.Helper()
	var mu sync.Mutex
	var b bytes.Buffer
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			mu.Lock()
			b.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return strings.ReplaceAll(b.String(), "\r\n", "\n")
	}
}

// waitFor waits until read returns text that holds want.
func waitFor(t *testing.T, read func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(read(), want) {
		require.True(t, time.Now().Before(deadline), "the terminal never showed %q; it shows %q", want, read())
		time.Sleep(5 * time.Millisecond)
	}
}

// answerWait is how long a test waits for a prompt to return once the
// answer is typed.
const answerWait = 10 * time.Second

// within returns what done gives, or fails the test after answerWait
// with what the screen shows: a prompt that waits for an answer it was
// already given fails here, not at the timeout of the package. The
// terminal is closed at the end of the test, which ends the read.
func within[T any](t *testing.T, done <-chan T, screen func() string) T {
	t.Helper()
	select {
	case r := <-done:
		return r
	case <-time.After(answerWait):
		t.Fatalf("the prompt did not return in %s; the screen shows %q", answerWait, screen())
		var zero T
		return zero
	}
}

func TestReadAnswer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a line", "y\nmore\n", "y"},
		{"no newline before the end", "y", ""},
		{"nothing", "", ""},
		{"a line too long", strings.Repeat("y", maxAnswer+1) + "\n", ""},
		{"a line too long with a y after it", strings.Repeat("n", maxAnswer+1) + "y\n", ""},
		{"a line of the longest length", strings.Repeat("n", maxAnswer) + "\n", strings.Repeat("n", maxAnswer)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := readAnswer(strings.NewReader(tt.in))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestReadAnswerDrainsALongLine: the rest of an over-long line is read
// and dropped, so it never becomes the next answer.
func TestReadAnswerDrainsALongLine(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(strings.Repeat("n", maxAnswer+10) + "y\nnext\n")
	got, err := readAnswer(r)
	require.NoError(t, err)
	assert.Empty(t, got)
	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "next\n", string(rest))
}

// TestConfirmOnATerminal: Confirm, the prompt of gates 1 and 2, needs a
// terminal on stdin and on the stderr the text goes to, so the operator
// sees the text before answering (decided on 2026-10-23); the text goes
// through the writer over that stderr, driven through a /dev/ptmx pair
// (10 10.1, REQ-0790), and "y" or "Y" typed once the text is shown
// approves, while a y typed before it is dropped.
//
//romeu:invariant I7 test
func TestConfirmOnATerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		early string
		typed string
		want  bool
	}{
		{"y approves", "", "y\n", true},
		{"Y approves", "", "Y\n", true},
		{"N declines", "", "N\n", false},
		{"an empty line declines", "", "\n", false},
		{"yes declines", "", "yes\n", false},
		{"y after a space declines", "", " y\n", false},
		{"a y typed before the text is dropped", "y\n", "n\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			master, terminal := openPTY(t)
			read := screen(t, master)
			if tt.early != "" {
				_, err := io.WriteString(master, tt.early)
				require.NoError(t, err)
				waitFor(t, read, strings.TrimSuffix(tt.early, "\n"))
			}
			var shown syncBuilder
			done := make(chan error, 1)
			var got bool
			go func() {
				var err error
				got, err = Confirm(terminal, int(terminal.Fd()), &shown, "gate 1: a change\nacknowledge? [y/N] ")
				done <- err
			}()
			waitFor(t, shown.String, "acknowledge? [y/N] ")
			_, err := io.WriteString(master, tt.typed)
			require.NoError(t, err)
			require.NoError(t, within(t, done, shown.String))
			assert.Equal(t, tt.want, got)
			assert.Equal(t, "gate 1: a change\nacknowledge? [y/N] ", shown.String())
		})
	}
}

// TestConfirmWithoutATerminal: without a terminal on stdin, or on
// stderr, Confirm approves nothing and writes nothing.
//
//romeu:invariant I7 test
func TestConfirmWithoutATerminal(t *testing.T) {
	t.Parallel()
	pipeRead, pipeWrite, err := os.Pipe()
	require.NoError(t, err)
	closeAtEnd(t, pipeRead)
	closeAtEnd(t, pipeWrite)
	_, err = io.WriteString(pipeWrite, "y\n")
	require.NoError(t, err)
	file, err := os.CreateTemp(t.TempDir(), "log")
	require.NoError(t, err)
	closeAtEnd(t, file)
	_, terminal := openPTY(t)
	for _, tt := range []struct {
		name     string
		stdin    io.Reader
		stderrFD int
	}{
		{"a pipe on stdin", pipeRead, int(terminal.Fd())},
		{"a file on stdin", file, int(terminal.Fd())},
		{"a reader that is no file on stdin", strings.NewReader("y\n"), int(terminal.Fd())},
		{"no reader on stdin", nil, int(terminal.Fd())},
		{"a pipe on stderr", terminal, int(pipeWrite.Fd())},
		{"a file on stderr", terminal, int(file.Fd())},
		{"no stderr", terminal, -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var shown strings.Builder
			got, err := Confirm(tt.stdin, tt.stderrFD, &shown, "acknowledge? [y/N] ")
			require.ErrorIs(t, err, ErrNoTerminal)
			assert.False(t, got)
			assert.Empty(t, shown.String())
		})
	}
}

// syncBuilder is a strings.Builder a test reads while another goroutine
// writes it.
type syncBuilder struct {
	mu sync.Mutex
	b  strings.Builder
}

// Write appends p.
func (s *syncBuilder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// String returns what was written so far.
func (s *syncBuilder) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// typingWriter is a stderr that types answer on the terminal, through
// master, in the same Write that shows question: the fastest operator
// there is, who answers the moment the question is on the screen.
type typingWriter struct {
	master           *os.File
	question, answer string
	shown            syncBuilder
}

// Write records p and, when p shows the question, types the answer
// before it returns.
func (w *typingWriter) Write(p []byte) (int, error) {
	n, err := w.shown.Write(p)
	if strings.Contains(string(p), w.question) {
		if _, werr := io.WriteString(w.master, w.answer); werr != nil {
			return n, werr
		}
	}
	return n, err
}

// TestConfirmTakesAnAnswerTypedAtTheQuestion: an answer typed the moment
// the question is shown is the answer. The input queue is dropped before
// the question is written, never after it, so no answer to the question
// can be dropped and leave the prompt waiting (the hang of e2e on
// 2026-10-23); what was typed before the question still is.
//
//romeu:invariant I7 test
func TestConfirmTakesAnAnswerTypedAtTheQuestion(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"gate 1: a change\nacknowledge? [y/N] ",
		"gate 1: one line; acknowledge? [y/N] ",
	} {
		t.Run(strings.ReplaceAll(text, "\n", " "), func(t *testing.T) {
			t.Parallel()
			master, terminal := openPTY(t)
			screen(t, master)
			w := &typingWriter{master: master, question: "acknowledge? [y/N] ", answer: "y\n"}
			type result struct {
				ok  bool
				err error
			}
			done := make(chan result, 1)
			go func() {
				ok, err := Confirm(terminal, int(terminal.Fd()), w, text)
				done <- result{ok, err}
			}()
			select {
			case r := <-done:
				require.NoError(t, r.err)
				assert.True(t, r.ok)
			case <-time.After(5 * time.Second):
				t.Fatal("the answer typed at the question was dropped, and Confirm still waits for one")
			}
			assert.Equal(t, text, w.shown.String())
		})
	}
}
