// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// ErrNoTerminal is the error of Ask when standard input is not a
// terminal: an approval needs one (01 1.4, REQ-0059). sync then
// declines the candidate, prints the widening digest and exits 3 with
// project-approval-required (01 1.6, REQ-1210).
var ErrNoTerminal = errors.New("gate: an approval needs a terminal on standard input")

// maxAnswer is the most bytes of an answer Ask reads: a longer line is
// no "y" and declines.
const maxAnswer = 4096

// Ask writes the diff d, its summary repeated right above the prompt,
// and PromptLine to stderr, so a piped stdout keeps them visible, and
// reads one answer line from stdin, which must be a terminal
// (REQ-0272). It approves on "y" or "Y" followed by a newline; any
// other line, an end of input before a newline, and a line longer than
// maxAnswer decline (01 1.6, REQ-1208). Without a terminal it writes
// nothing and returns ErrNoTerminal.
//
//romeu:invariant I7 guard
func Ask(stdin io.Reader, stderr io.Writer, d Diff) (bool, error) {
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return false, ErrNoTerminal
	}
	if _, err := d.WriteTo(stderr); err != nil {
		return false, fmt.Errorf("gate: write the diff: %w", err)
	}
	line, err := readAnswer(f)
	if err != nil {
		return false, fmt.Errorf("gate: read the answer: %w", err)
	}
	return line == "y" || line == "Y", nil
}

// readAnswer reads one line from r a byte at a time, so it takes
// nothing past the newline, and returns it without the newline. A line
// that ends without a newline, at the end of input, or that passes
// maxAnswer bytes is returned as "", which declines.
func readAnswer(r io.Reader) (string, error) {
	var line []byte
	var b [1]byte
	for len(line) <= maxAnswer {
		n, err := r.Read(b[:])
		if n == 1 {
			if b[0] == '\n' {
				return string(line), nil
			}
			line = append(line, b[0])
		}
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", nil
}
