// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package git runs the git commands the checks read a repository with.
// The checks only read: nothing here writes a ref, the index or the
// working tree.
//
// A clone made with --filter (a partial clone) lacks the blobs it has
// not needed yet, and git here never fetches one (see Run). A check
// that reads such a blob fails: the checks of the pre-push hook need
// the objects they read to be fetched first, and an error about a
// lacking object says how (FetchHint), where the check sees it.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// Repo is a repository on disk.
type Repo struct {
	// Dir is a directory inside the working tree.
	Dir string
	// Env is the environment of each git command. Nil means the
	// environment of this process, which is what a hook needs: git
	// tells a hook which repository it serves through it.
	Env []string
}

// Run runs git with args in the repository, gives it stdin, and returns
// its standard output. A command that exits non-zero is an error that
// carries what git wrote to standard error.
//
// Each command runs through internal/procgroup, in a process group of
// its own, under the class procgroup.ToolGit: the commands here read
// local objects, so a command that runs past its limit is stuck, and a
// stuck command must not hold a push open for ever.
//
// Three settings hold for every command, whatever the configuration
// and the environment. Replace refs are off: a push sends the objects
// themselves, so a local replace ref must not show the checks a different
// object from the one that leaves the machine. core.quotePath is on: a
// path that git quotes then has every byte outside ASCII escaped, so
// unquoting gives back its bytes exactly. GIT_NO_LAZY_FETCH is 1: an
// object that a partial clone lacks is an error, not a fetch, so a check
// reads only what the clone holds and never reaches the network.
func (r Repo) Run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	fixed := []string{"--no-replace-objects", "-c", "core.quotePath=true"}
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class: procgroup.ToolGit,
		Argv:  append([]string{"git"}, append(fixed, args...)...),
		Dir:   r.Dir,
		// Last, so that it wins over the same variable earlier in the
		// environment.
		Env:    append(r.Environ(), "GIT_NO_LAZY_FETCH=1"),
		Stdin:  bytes.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, Printable(strings.TrimSpace(stderr.String())))
	}
	return stdout.Bytes(), nil
}

// Environ returns the environment a git command of the repository runs
// with: a copy of Env, or of this process's environment when Env is
// nil. A command started through internal/procgroup takes it whole,
// since procgroup gives a nil environment an empty one.
func (r Repo) Environ() []string {
	if r.Env == nil {
		return os.Environ()
	}
	return slices.Clone(r.Env)
}

// FetchHint is the advice of an error about the object oid that the
// object store lacks. Git fetches a blob a partial clone lacks when it
// reads it without GIT_NO_LAZY_FETCH, so the hint names such a read.
func FetchHint(oid string) string {
	return `in a partial clone (one made with --filter), fetch it with "git cat-file -p ` + oid + `" and run again`
}

// Printable returns s as it is when it is UTF-8 and every character of
// it prints, and quoted in Go syntax with only ASCII otherwise. A path
// and a message of git pass through it before they are printed: a
// control character or a newline in them would move the terminal or
// add a line of its own to the output, such as a forged "ok" line.
func Printable(s string) string {
	if !utf8.ValidString(s) || strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return strconv.QuoteToASCII(s)
	}
	return s
}

// SafeLines returns text that may hold repository content, such as a path
// in a message, so that it is safe to print line by line. Printable
// serves one value that is quoted whole; SafeLines serves text that
// keeps its lines. It is not reversible: a literal \xff and a raw 0xff
// byte print the same.
//
// In each line every character that does not print (a control
// character, a space other than " ", a line separator, a zero-width or
// direction character) is written as a Go escape, except the tab, and
// each byte that is not UTF-8 as \xNN. Then what the Actions runner
// reads as a workflow command is broken (actions/runner at 67f01c2:
// ActionCommand.cs:62-63 trims the start of the line and then tests
// StartsWith("::"); TryParse, around line 121, finds the legacy "##["
// with IndexOf anywhere in the line). Both comparisons are
// culture-sensitive, and whether they skip printable but ignorable
// characters (a combining mark, a variation selector, a filler) is
// unverified. That does not matter, because the decision is made on the
// ASCII skeleton of the line, the line with every character but the
// printable ASCII and the tab dropped: a line whose skeleton starts,
// after blanks, with "::" gets "./" in front; a line whose skeleton
// holds "##[" has every non-ASCII character escaped as well, and "##["
// written "##\[".
func SafeLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		var b strings.Builder
		for j := 0; j < len(line); {
			r, size := utf8.DecodeRuneInString(line[j:])
			switch {
			case r == utf8.RuneError && size == 1:
				fmt.Fprintf(&b, `\x%02x`, line[j])
			case r != '\t' && !unicode.IsPrint(r):
				b.WriteString(escape(r))
			default:
				b.WriteRune(r)
			}
			j += size
		}
		l := b.String()
		skel := strings.Map(func(r rune) rune {
			if r == '\t' || (r >= 0x20 && r <= 0x7e) {
				return r
			}
			return -1
		}, l)
		if strings.Contains(skel, "##[") {
			l = strings.ReplaceAll(escapeNonASCII(l), "##[", `##\[`)
		}
		if strings.HasPrefix(strings.TrimLeft(skel, " \t"), "::") {
			l = "./" + l
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// escape returns r as a Go escape, \u or \U, with only ASCII.
func escape(r rune) string {
	q := strconv.QuoteToASCII(string(r))
	return q[1 : len(q)-1]
}

// escapeNonASCII writes each character above 0x7e of s as an escape.
func escapeNonASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 0x7e {
			b.WriteString(escape(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
