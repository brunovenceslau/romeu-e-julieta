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
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

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

// adrDirFile names, relative to the repository root, the file that says
// where the decision records live (rule 6 of ADR 0001).
const adrDirFile = ".adr-dir"

// env is what a run reads and writes. Tests fill it with a fixture and a
// fixed clock; main fills it with the process's own.
type env struct {
	dir    string           // where the command was started
	gitEnv []string         // the environment of git; nil means this process's
	now    func() time.Time // the clock: the only source of the date
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

// adrFile matches the name of a decision record: four digits, a
// hyphen, and the rest.
var adrFile = regexp.MustCompile(`^(\d{4})-.+\.md$`)

// newADR writes the next decision record, with the layout of rule 6 of
// ADR 0001: the title line, the date from the clock, and the four
// sections, with the status Proposed. The file is created and never
// overwritten, so a name that exists is an error.
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
	root, err := repoRoot(e)
	if err != nil {
		return err
	}
	dirBytes, err := os.ReadFile(filepath.Join(root, adrDirFile))
	if err != nil {
		return err
	}
	rel := strings.TrimSpace(string(dirBytes))
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("%s names %q, which is not a directory inside the repository", adrDirFile, rel)
	}
	dir := filepath.Join(root, rel)
	n, err := nextNumber(dir)
	if err != nil {
		return err
	}
	file := fmt.Sprintf("%04d-%s.md", n, name)
	body := fmt.Sprintf("# %d. %s\n\nDate: %s\n\n## Status\n\nProposed\n\n## Context\n\n## Decision\n\n## Consequences\n",
		n, title, e.now().Format(time.DateOnly))
	f, err := os.OpenFile(filepath.Join(dir, file), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	say(e.stdout, "%s\n", filepath.ToSlash(filepath.Join(rel, file)))
	return nil
}

// checkTitle refuses a title that cannot be the first line of a record:
// one with a control character, a line break included.
func checkTitle(title string) error {
	if title == "" {
		return errors.New("the title is empty")
	}
	for _, r := range title {
		if unicode.IsControl(r) {
			return fmt.Errorf("the title %q holds a control character", title)
		}
	}
	return nil
}

// nextNumber returns one more than the highest number among the records
// in dir, and 1 for a directory with none.
func nextNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	highest := 0
	for _, entry := range entries {
		m := adrFile.FindStringSubmatch(entry.Name())
		if m == nil || entry.Type()&fs.ModeDir != 0 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		highest = max(highest, n)
	}
	return highest + 1, nil
}

// repoRoot returns the top of the working tree the command runs in.
func repoRoot(e env) (string, error) {
	out, err := git.Repo{Dir: e.dir, Env: e.gitEnv}.Run(context.Background(), nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
