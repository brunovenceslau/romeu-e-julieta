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

// ErrNoTerminal is the error of Ask when standard input or standard
// error is not a terminal: an approval needs both (01 1.4, REQ-0059). sync then
// declines the candidate, prints the widening digest and exits 3 with
// project-approval-required (01 1.6, REQ-1210).
var ErrNoTerminal = errors.New("gate: an approval needs a terminal on standard input and standard error")

// maxAnswer is the most bytes of an answer Ask reads: a longer line is
// no "y" and declines.
const maxAnswer = 4096

// Ask writes the diff d, its summary repeated right above the prompt,
// and PromptLine to stderr, so a piped stdout keeps them visible, and
// reads one answer line from stdin (REQ-0272). Both must be a terminal,
// so the operator reads the diff where the answer is typed. Once the
// diff is written, Ask drops the terminal's input queue, so a line
// typed or pasted before the diff was on the screen is not an answer
// to it. It approves on "y" or "Y" followed by a newline; any other
// line, an end of input before a newline, and a line longer than
// maxAnswer decline (01 1.6, REQ-1208). Without a terminal it writes
// nothing and returns ErrNoTerminal.
//
//romeu:invariant I7 guard
func Ask(stdin io.Reader, stderr io.Writer, d Diff) (bool, error) {
	in, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return false, ErrNoTerminal
	}
	out, ok := stderr.(*os.File)
	if !ok || !term.IsTerminal(int(out.Fd())) {
		return false, ErrNoTerminal
	}
	if _, err := d.WriteTo(out); err != nil {
		return false, fmt.Errorf("gate: write the diff: %w", err)
	}
	if err := flushInput(int(in.Fd())); err != nil {
		return false, fmt.Errorf("gate: drop what was typed before the prompt: %w", err)
	}
	line, err := readAnswer(in)
	if err != nil {
		return false, fmt.Errorf("gate: read the answer: %w", err)
	}
	return line == "y" || line == "Y", nil
}

// Confirm is the prompt of gate 1 (01 1.4, REQ-0061): it writes text,
// which ends with its question, to stderr and reads one answer line from
// stdin, by the rules of Ask: once text is written it drops the
// terminal's input queue, and "y" or "Y" followed by a newline approves
// while any other line, or an end of input, declines. Like Ask, it needs
// a terminal on stdin and on the process's standard error, the file
// descriptor stderrFD, so the operator sees the text where the answer is
// typed (decided on 2026-10-23); stderr is the command's CLI writer over
// that standard error, which escapes what it is given. Without both
// terminals it writes nothing and returns ErrNoTerminal.
//
//romeu:invariant I7 guard
func Confirm(stdin io.Reader, stderrFD int, stderr io.Writer, text string) (bool, error) {
	in, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return false, ErrNoTerminal
	}
	if !term.IsTerminal(stderrFD) {
		return false, ErrNoTerminal
	}
	if _, err := io.WriteString(stderr, text); err != nil {
		return false, fmt.Errorf("gate: write the prompt: %w", err)
	}
	if err := flushInput(int(in.Fd())); err != nil {
		return false, fmt.Errorf("gate: drop what was typed before the prompt: %w", err)
	}
	line, err := readAnswer(in)
	if err != nil {
		return false, fmt.Errorf("gate: read the answer: %w", err)
	}
	return line == "y" || line == "Y", nil
}

// readAnswer reads one line from r a byte at a time, so it takes
// nothing past the newline, and returns it without the newline. A line
// that ends without a newline, at the end of input, is returned as "",
// which declines; so is a line longer than maxAnswer bytes, whose rest
// is read up to its newline and dropped, so it never becomes the next
// answer.
func readAnswer(r io.Reader) (string, error) {
	var line []byte
	var b [1]byte
	for {
		n, err := r.Read(b[:])
		if n == 1 {
			if b[0] == '\n' {
				if len(line) > maxAnswer {
					return "", nil
				}
				return string(line), nil
			}
			if len(line) <= maxAnswer {
				line = append(line, b[0])
			}
		}
		if errors.Is(err, io.EOF) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
	}
}
