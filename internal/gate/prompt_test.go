// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

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

// TestAskOnATerminal drives the answer through a /dev/ptmx pair on
// stdin and reads what Ask writes to stderr (10 10.1, REQ-0790).
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
			master, terminal := openPTY(t)
			_, err := io.WriteString(master, tt.typed)
			require.NoError(t, err)

			var stderr strings.Builder
			d := sampleDiff(t)
			got, err := Ask(terminal, &stderr, d)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			var want strings.Builder
			_, err = d.WriteTo(&want)
			require.NoError(t, err)
			assert.Equal(t, want.String(), stderr.String(), "the diff, the summary again and the prompt go to stderr")
			assert.True(t, strings.HasSuffix(stderr.String(), PromptLine))
		})
	}
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

	tests := []struct {
		name  string
		stdin io.Reader
	}{
		{"a pipe", pipeRead},
		{"a file", file},
		{"a reader that is no file", strings.NewReader("y\n")},
		{"no reader", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stderr strings.Builder
			got, err := Ask(tt.stdin, &stderr, sampleDiff(t))
			require.ErrorIs(t, err, ErrNoTerminal)
			assert.False(t, got)
			assert.Empty(t, stderr.String())
		})
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
