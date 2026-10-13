// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriterEscapesByDefault checks that a Writer made with no option
// escapes one code point of each class of I29, a byte that is not
// UTF-8 and a carriage return, and keeps the line feeds that end its
// lines.
//
//romeu:invariant I29 test
func TestWriterEscapesByDefault(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"control", "a\x1b[2Jb\n", `a\u001b[2Jb` + "\n"},
		{"c1 control", "a\u009bb\n", `a\u009bb` + "\n"},
		{"bidi", "fix\u202eexe.txt\n", `fix\u202eexe.txt` + "\n"},
		{"zero-width", "ma\u200bin\n", `ma\u200bin` + "\n"},
		{"tag characters", "hi\U000E0041\n", `hi\udb40\udc41` + "\n"},
		{"line separator", "a\u2028b\n", `a\u2028b` + "\n"},
		{"paragraph separator", "a\u2029b\n", `a\u2029b` + "\n"},
		{"not utf-8", "a\xffb\n", `a\xffb` + "\n"},
		{"carriage return", "ok\rFAIL\n", `ok\rFAIL` + "\n"},
		{"tab", "a\tb\n", `a\tb` + "\n"},
		{"lines kept", "one\n\ntwo\n", "one\n\ntwo\n"},
		{"no final line feed", "one\ntwo", "one\ntwo"},
		{"plain unicode", "ol\u00e1 \u65e5\u672c\n", "ol\u00e1 \u65e5\u672c\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			n, err := NewWriter(&out).Write([]byte(tt.in))
			require.NoError(t, err)
			assert.Equal(t, len(tt.in), n, "the count of the bytes of p")
			assert.Equal(t, tt.want, out.String())
		})
	}
}

// TestWriterUnderFmt checks the writer the way commands use it, under
// fmt: a value with an escape sequence prints escaped, and the line
// feed of the format is kept.
func TestWriterUnderFmt(t *testing.T) {
	var out bytes.Buffer
	_, err := fmt.Fprintf(NewWriter(&out), "branch %s is %d commits ahead\n", "feat\u202e\x1b]0;x\x07", 2)
	require.NoError(t, err)
	assert.Equal(t, `branch feat\u202e\u001b]0;x\u0007 is 2 commits ahead`+"\n", out.String())
}

// TestWriterEscapesEachWriteAlone checks that the writer keeps no state
// between calls: a code point split over two writes prints as the
// escapes of its bytes, and nothing is held back.
func TestWriterEscapesEachWriteAlone(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out)
	e := []byte("\u00e9")
	_, err := w.Write(e[:1])
	require.NoError(t, err)
	assert.Equal(t, `\xc3`, out.String(), "the first byte, written at once")
	_, err = w.Write(e[1:])
	require.NoError(t, err)
	assert.Equal(t, `\xc3\xa9`, out.String())
}

// TestWriterEmptyWrite checks that an empty write writes nothing.
func TestWriterEmptyWrite(t *testing.T) {
	var out bytes.Buffer
	n, err := NewWriter(&out).Write(nil)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, out.String())
}

// failingWriter writes the first n bytes it is given and then fails
// with err, or, when err is nil, reports the short count alone.
type failingWriter struct {
	n   int
	err error
}

// Write writes at most f.n bytes of p and returns f.err.
func (f *failingWriter) Write(p []byte) (int, error) {
	return min(f.n, len(p)), f.err
}

// TestWriterErrors checks that an error of the writer under it reaches
// the caller with no byte of p counted as written, and that a short
// write with no error, even one byte short, is io.ErrShortWrite.
func TestWriterErrors(t *testing.T) {
	broken := errors.New("broken pipe")
	n, err := NewWriter(&failingWriter{n: 2, err: broken}).Write([]byte("hello\n"))
	require.ErrorIs(t, err, broken)
	assert.Zero(t, n)

	n, err = NewWriter(&failingWriter{n: 5}).Write([]byte("hello\n")) // one byte short
	require.ErrorIs(t, err, io.ErrShortWrite)
	assert.Zero(t, n)
}
