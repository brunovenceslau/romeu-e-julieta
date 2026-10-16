// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package cli holds what the commands of both binaries share for their
// output (04 4.1, Output). All output goes through the CLI writer,
// Writer, which escapes with termsafe by default (I29, 05 5.2).
package cli

import (
	"bytes"
	"io"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Writer is the CLI writer: an io.Writer that escapes what it is given
// with termsafe.Escape before it writes it to the writer under it, and
// keeps the line feeds that end the lines of the output. Each call is
// escaped alone and nothing is held between calls, so a code point
// that two calls split prints as the escapes of its bytes rather than
// waiting for a flush; fmt and the JSON encoders write once per call.
// A buffer goes under the Writer, never over it: a bufio.Writer or
// io.Copy over it would cut its text at the size of its buffer.
//
// A line feed inside a value is kept as well, since the writer cannot
// tell a value from the text around it; a value that must stay on one
// line, such as a ref name, is passed through termsafe.Escape before it
// is formatted. A tab is escaped, so aligned output pads with spaces.
// The raw output of 04 4.1, the interactive calls, gives the terminal
// to the subprocess and does not pass through a Writer.
type Writer struct {
	out io.Writer
}

// NewWriter returns a Writer that writes the escaped text to out. It
// escapes from the start: no option turns escaping off.
func NewWriter(out io.Writer) *Writer {
	return &Writer{out: out}
}

// Write escapes p, line by line, and writes the result to the writer
// under it in one call. It returns len(p) when that write succeeds and
// 0 with the error when it does not, since the escaped output does not
// map back to a count of the bytes of p; on an error, part of the
// output may have reached the writer under it. A short write with no
// error is io.ErrShortWrite.
//
//romeu:invariant I29 guard
func (w *Writer) Write(p []byte) (int, error) {
	lines := bytes.Split(p, []byte{'\n'})
	for i, line := range lines {
		lines[i] = []byte(termsafe.Escape(string(line)))
	}
	escaped := bytes.Join(lines, []byte{'\n'})
	n, err := w.out.Write(escaped)
	if err != nil {
		return 0, err
	}
	if n < len(escaped) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}
