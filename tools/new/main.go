// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command new scaffolds a file of this repository with the next free
// id, so nobody types a number, a date or a filename (12 12.3).
//
//	go run ./tools/new adr <title>    write the next decision record
//	go run ./tools/new task <title>   write the next task file of the plan
//
// Each kind registers itself from its own file, <kind>.go (rule 4 of
// docs/plan.md), so a kind adds a file and edits none; the other kinds
// of 12 12.3 land with the module that owns what they scaffold. It exits
// 0 when it wrote the file, and 2 when it could not or the command line
// is wrong. Started through "go run", every status
// other than 0 reaches the caller as 1, with the real one in the "exit
// status" line that go prints.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses.
const (
	exitOK    = 0 // the file is written
	exitError = 2 // nothing was written, or the command line is wrong
)

// env is what a run reads and writes. Tests fill it with a fixture and a
// fixed clock; main fills it with the process's own.
type env struct {
	dir    string           // where the command was started
	gitEnv []string         // the environment of git; nil means this process's
	now    func() time.Time // the clock: the only source of the date; main's is time.Now, so the date is the local day
	// afterScan, when set, runs between a kind's scan for the next free
	// id and its write: the tests' seam for a run that takes the same id
	// at the same time.
	afterScan func()
	stdout    io.Writer
	stderr    io.Writer
}

func main() {
	e := env{dir: ".", now: time.Now, stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(run(e, os.Args[1:]))
}

// run dispatches one kind and returns the exit status.
func run(e env, args []string) int {
	if len(args) == 0 {
		say(e.stderr, "%s", usage())
		return exitError
	}
	k, ok := kinds[args[0]]
	if !ok {
		say(e.stderr, "new: unknown kind %q\n%s", args[0], usage())
		return exitError
	}
	if err := k.run(e, args[1:]); err != nil {
		say(e.stderr, "new: %v\n", strings.TrimSpace(err.Error()))
		return exitError
	}
	return exitOK
}

// say writes one piece of output. A write that fails has nowhere left
// to be reported, so the error is dropped here, in one place.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// create writes content to a new file at path below root, and removes
// the file when the write fails. A file that exists is an error.
func create(root *os.Root, path, content string) error {
	f, err := root.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.Join(err, root.Remove(path))
	}
	return nil
}

// blankRendering is the one character that draws as a blank and is in
// none of the tables of invisible: the empty Braille pattern (U+2800).
// The Hangul fillers (U+115F, U+1160, U+3164, U+FFA0) are in
// unicode.Other_Default_Ignorable_Code_Point.
const blankRendering = "\u2800"

// invisible reports whether r is a control character, or one that moves
// or hides text where it is shown. The tables of the standard library
// give Default_Ignorable_Code_Point as its definition in the Unicode
// standard (DerivedCoreProperties) builds it: Other_Default_Ignorable_
// Code_Point (the combining grapheme joiner U+034F and the Hangul
// fillers among them), the format category (Cf) and the variation
// selectors, less a few white space and format exceptions. Cf is refused
// whole, so the exceptions are refused too, which is stricter and safe
// for a title. The line and paragraph separators (Zl, Zp) and
// blankRendering come on top.
func invisible(r rune) bool {
	return unicode.IsControl(r) ||
		unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp, unicode.Other_Default_Ignorable_Code_Point, unicode.Variation_Selector) ||
		strings.ContainsRune(blankRendering, r)
}

// checkTitle refuses a title that cannot be the first line of a record:
// one with a control character (a line break included), or with a
// character that moves or hides text where it is shown, such as a
// right-to-left override, a zero-width space, a line separator or a
// default-ignorable code point (invisible). A title of spaces only, a
// no-break space among them, is refused as empty earlier
// (strings.TrimSpace in the caller). The zero width joiners (U+200C,
// U+200D) and the variation selectors (such as the emoji selector
// U+FE0F) are refused too: titles are English, the language of the
// repository, and a joiner hides in a title.
func checkTitle(title string) error {
	if title == "" {
		return errors.New("the title is empty")
	}
	for _, r := range title {
		if invisible(r) {
			return fmt.Errorf("the title %q holds a character that does not show (%U)", title, r)
		}
	}
	return nil
}

// repoRoot returns the top of the working tree the command runs in.
func repoRoot(e env) (string, error) {
	out, err := git.Repo{Dir: e.dir, Env: e.gitEnv}.Run(context.Background(), nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
