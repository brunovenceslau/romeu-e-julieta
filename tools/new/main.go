// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command new scaffolds a file of this repository with the next free
// id, so nobody types a number, a date or a filename (12 12.3).
//
//	go run ./tools/new adr <title>   write the next decision record
//
// The other kinds of 12 12.3 land with the module that owns what they
// scaffold. It exits 0 when it wrote the file, and 2 when it could not
// or the command line is wrong. Started through "go run", every status
// other than 0 reaches the caller as 1, with the real one in the "exit
// status" line that go prints.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/slug"
)

// The exit statuses.
const (
	exitOK    = 0 // the file is written
	exitError = 2 // nothing was written, or the command line is wrong
)

const usage = `usage:
  go run ./tools/new adr <title>
`

// env is what a run reads and writes. Tests fill it with a fixture and a
// fixed clock; main fills it with the process's own.
type env struct {
	dir    string           // where the command was started
	gitEnv []string         // the environment of git; nil means this process's
	now    func() time.Time // the clock: the only source of the date; main's is time.Now, so the date is the local day
	stdout io.Writer
	stderr io.Writer
}

func main() {
	e := env{dir: ".", now: time.Now, stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(run(e, os.Args[1:]))
}

// run dispatches one kind and returns the exit status.
func run(e env, args []string) int {
	if len(args) == 0 {
		say(e.stderr, "%s", usage)
		return exitError
	}
	var err error
	switch args[0] {
	case "adr":
		err = newADR(e, args[1:])
	default:
		err = fmt.Errorf("unknown kind %q\n%s", args[0], usage)
	}
	if err != nil {
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

// newADR writes the next decision record, with the layout of rule 6 of
// ADR 0001: the title line, the date from the clock, and the four
// sections, with the status Proposed. The file is created and never
// overwritten, so a name that exists is an error. It reads and writes
// through the root of the repository, so a symbolic link cannot send it
// outside. Two runs that take the same number at once each create their
// own file and then scan again: the run whose re-scan sees two files
// with its number removes its own and fails, so at most one record keeps
// the number.
func newADR(e env, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("adr takes the title as one argument\n%s", usage)
	}
	title := strings.TrimSpace(args[0])
	if err := checkTitle(title); err != nil {
		return err
	}
	name := slug.Slug(title)
	if name == "" {
		return fmt.Errorf("the title %q has no ASCII letter or digit, so its filename would be empty", title)
	}
	top, err := repoRoot(e)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(top)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	rel, err := adrdir.Dir(root)
	if err != nil {
		return err
	}
	byNumber, err := recordsByNumber(root, rel)
	if err != nil {
		return err
	}
	n := nextNumber(byNumber)
	if n > adrdir.MaxNumber {
		return fmt.Errorf("%s holds record %04d, the last number a filename has room for", rel, adrdir.MaxNumber)
	}
	file := fmt.Sprintf("%04d-%s.md", n, name)
	path := filepath.Join(rel, file)
	body := fmt.Sprintf("# %d. %s\n\nDate: %s\n\n## Status\n\nProposed\n\n## Context\n\n## Decision\n\n## Consequences\n",
		n, title, e.now().Format(time.DateOnly))
	if err := create(root, path, body); err != nil {
		return err
	}
	after, err := recordsByNumber(root, rel)
	if err == nil && len(after[n]) > 1 {
		err = fmt.Errorf("another record took number %04d at the same time (%s); run the command again", n, strings.Join(after[n], ", "))
	}
	if err != nil {
		// Not ours to keep: the number is not unique, or not known to be.
		if rmErr := root.Remove(path); rmErr != nil {
			err = errors.Join(err, rmErr)
		}
		return err
	}
	say(e.stdout, "%s\n", filepath.ToSlash(path))
	return nil
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

// blankRendering is the small set of characters that draw as a blank
// and are not in Cf, Zl or Zp: the Hangul fillers (U+115F, U+1160,
// U+3164, U+FFA0) and the empty Braille pattern (U+2800). A title made
// of them would look empty or hide the text next to it.
const blankRendering = "\u115f\u1160\u3164\uffa0\u2800"

// checkTitle refuses a title that cannot be the first line of a record:
// one with a control character (a line break included), or with a
// character that moves or hides text where it is shown, such as a
// right-to-left override, a zero-width space or a line separator. That
// is the format category (Cf) and the two separators of Unicode, and
// blankRendering: the letters and the symbol that draw as blank but are
// outside them.
func checkTitle(title string) error {
	if title == "" {
		return errors.New("the title is empty")
	}
	for _, r := range title {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) || strings.ContainsRune(blankRendering, r) {
			return fmt.Errorf("the title %q holds a control or invisible character (%U)", title, r)
		}
	}
	return nil
}

// recordsByNumber maps each record number in the directory rel below
// root to the filenames that carry it. A directory with a record's name
// is not a record.
func recordsByNumber(root *os.Root, rel string) (map[int][]string, error) {
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(rel))
	if err != nil {
		return nil, err
	}
	byNumber := map[int][]string{}
	for _, entry := range entries {
		m := adrdir.Name.FindStringSubmatch(entry.Name())
		if m == nil || entry.Type()&fs.ModeDir != 0 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, err
		}
		byNumber[n] = append(byNumber[n], entry.Name())
	}
	return byNumber, nil
}

// nextNumber returns one more than the highest number of the records,
// and 1 when there are none.
func nextNumber(byNumber map[int][]string) int {
	highest := 0
	for n := range byNumber {
		highest = max(highest, n)
	}
	return highest + 1
}

// repoRoot returns the top of the working tree the command runs in.
func repoRoot(e env) (string, error) {
	out, err := git.Repo{Dir: e.dir, Env: e.gitEnv}.Run(context.Background(), nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
