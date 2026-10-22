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

// sampleDiff is the diff of a first approval of the base inputs.
func sampleDiff(t *testing.T) Diff {
	t.Helper()
	d, err := NewDiff(nil, side(t, baseInputs()))
	require.NoError(t, err)
	return d
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

// askOnPTY runs Ask with both stdin and stderr on the terminal of a
// /dev/ptmx pair. It types early before Ask starts, waits for the
// prompt, types answer, and returns Ask's result and the screen.
func askOnPTY(t *testing.T, d Diff, early, answer string) (bool, string) {
	t.Helper()
	master, terminal := openPTY(t)
	read := screen(t, master)
	if early != "" {
		_, err := io.WriteString(master, early)
		require.NoError(t, err)
		waitFor(t, read, strings.TrimSuffix(early, "\n"))
	}
	type result struct {
		ok  bool
		err error
	}
	done := make(chan result, 1)
	go func() {
		ok, err := Ask(terminal, terminal, d)
		done <- result{ok, err}
	}()
	waitFor(t, read, PromptLine)
	_, err := io.WriteString(master, answer)
	require.NoError(t, err)
	r := <-done
	require.NoError(t, r.err)
	return r.ok, read()
}

// TestAskOnATerminal drives the answer through a /dev/ptmx pair on
// stdin, typed once the prompt is on the screen, and reads what Ask
// writes to stderr, the same terminal (10 10.1, REQ-0790).
//
//romeu:invariant I7 test
func TestAskOnATerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		typed string
		want  bool
	}{
		{"y approves", "y\n", true},
		{"Y approves", "Y\n", true},
		{"N declines", "N\n", false},
		{"an empty line declines", "\n", false},
		{"yes declines", "yes\n", false},
		{"y after a space declines", " y\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := sampleDiff(t)
			got, shown := askOnPTY(t, d, "", tt.typed)
			assert.Equal(t, tt.want, got)

			var want strings.Builder
			_, err := d.WriteTo(&want)
			require.NoError(t, err)
			assert.Contains(t, shown, want.String(), "the diff, the summary again and the prompt go to stderr")
		})
	}
}

// TestAskDropsWhatWasTypedBeforeThePrompt: a "y" typed, or pasted,
// before the diff was on the screen is not an answer to it; Ask drops
// the terminal's input queue once the diff is written.
//
//romeu:invariant I7 test
func TestAskDropsWhatWasTypedBeforeThePrompt(t *testing.T) {
	t.Parallel()
	got, _ := askOnPTY(t, sampleDiff(t), "y\n", "n\n")
	assert.False(t, got)
}

// TestAskWithoutATerminal: a pipe, a file or a reader that is no file
// on stdin is no terminal, so Ask approves nothing and writes nothing.
//
//romeu:invariant I7 test
func TestAskWithoutATerminal(t *testing.T) {
	t.Parallel()
	pipeRead, pipeWrite, err := os.Pipe()
	require.NoError(t, err)
	closeAtEnd(t, pipeRead)
	closeAtEnd(t, pipeWrite)
	_, err = io.WriteString(pipeWrite, "y\n")
	require.NoError(t, err)
	file, err := os.CreateTemp(t.TempDir(), "answer")
	require.NoError(t, err)
	closeAtEnd(t, file)

	_, terminal := openPTY(t)
	tests := []struct {
		name   string
		stdin  io.Reader
		stderr io.Writer
	}{
		{"a pipe on stdin", pipeRead, terminal},
		{"a file on stdin", file, terminal},
		{"a reader that is no file on stdin", strings.NewReader("y\n"), terminal},
		{"no reader on stdin", nil, terminal},
		{"a writer that is no file on stderr", terminal, &strings.Builder{}},
		{"a file on stderr", terminal, file},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Ask(tt.stdin, tt.stderr, sampleDiff(t))
			require.ErrorIs(t, err, ErrNoTerminal)
			assert.False(t, got)
			if b, ok := tt.stderr.(*strings.Builder); ok {
				assert.Empty(t, b.String())
			}
		})
	}
	info, err := file.Stat()
	require.NoError(t, err)
	assert.Zero(t, info.Size(), "nothing is written without a terminal")
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
