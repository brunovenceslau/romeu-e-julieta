// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command new scaffolds what this repository numbers, so that nobody
// types a number, a date or a filename (12 12.3):
//
//	go run ./tools/new adr <title>   a decision record, status Proposed
//
// Each other kind of 12 12.3 lands with the module that owns what it
// scaffolds. It exits 0 when it wrote what it was asked to, 1 when it
// could not, and 2 when the command line is wrong.
package main

import (
	"context"
	"crypto/rand"
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

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adr"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses.
const (
	exitOK    = 0 // written
	exitFail  = 1 // not written
	exitUsage = 2 // the command line is wrong
)

const usage = `usage:
  go run ./tools/new adr <title>
`

// env is what a run reads and writes. Tests fill it with a fixture and
// a fixed clock; main fills it with the process's own.
type env struct {
	repo   git.Repo         // where the command was started
	now    func() time.Time // the injected clock (10 10.4)
	link   linkFunc         // os.Link, or a failure a test injects
	stdout io.Writer
	stderr io.Writer
}

// linkFunc gives the file oldname the second name newname, as os.Link
// does, and fails when newname exists.
type linkFunc func(oldname, newname string) error

func main() {
	e := env{repo: git.Repo{Dir: "."}, now: time.Now, link: os.Link, stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(run(context.Background(), e, os.Args[1:]))
}

// errUsage marks an error of the command line.
var errUsage = errors.New("usage")

// run dispatches one kind and returns the exit status.
func run(ctx context.Context, e env, args []string) int {
	var err error
	switch {
	case len(args) == 0:
		err = errUsage
	case args[0] == "adr":
		err = newADR(ctx, e, args[1:])
	default:
		err = fmt.Errorf("%w: unknown kind %q", errUsage, args[0])
	}
	switch {
	case errors.Is(err, errUsage):
		_, _ = fmt.Fprintf(e.stderr, "new: %v\n%s", err, usage)
		return exitUsage
	case err != nil:
		_, _ = fmt.Fprintf(e.stderr, "new: %v\n", err)
		return exitFail
	}
	return exitOK
}

// newADR writes the next decision record in the layout of ADR 0001,
// rule 6: the next free number, the date of the injected clock in the
// clock's own zone, the "# N. Title" line, the filename from the slug
// of the title, and status Proposed. The rest is the outline of the
// sections, for the author to fill. It never replaces a file.
func newADR(ctx context.Context, e env, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("%w: adr takes one argument, the title, in quotes when it has spaces", errUsage)
	}
	title := args[0]
	switch {
	case strings.ContainsFunc(title, func(r rune) bool { return !unicode.IsPrint(r) }):
		return errors.New("the title is one line of printable characters")
	case strings.TrimSpace(title) != title:
		return errors.New("the title has no space at either end")
	case adr.Slug(title) == "":
		return errors.New("the title needs an ASCII letter or digit, which its filename is made of")
	}
	out, err := e.repo.Run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	top := strings.TrimSuffix(string(out), "\n")
	dir := filepath.Join(top, filepath.FromSlash(adr.Dir))
	n, err := adr.Next(os.DirFS(top), adr.Dir)
	if err != nil {
		return err
	}
	name := adr.FileName(n, title)
	if len(name) > maxName {
		return fmt.Errorf("the filename %s would be %d bytes, and a file system takes %d at most: shorten the title", name[:16]+"...", len(name), maxName)
	}
	if err := createNew(filepath.Join(dir, name), []byte(record(n, title, e.now())), e.link); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(e.stdout, "wrote %s/%s\nrun \"go generate ./...\" to add it to the index, %s/%s\n", adr.Dir, name, adr.Dir, adr.Index)
	return nil
}

// maxName is the longest file name, in bytes, that the file systems of
// the supported platforms take for an ASCII name, which a slug is.
const maxName = 255

// createNew writes data to a new file at path and never replaces a file
// there. The file takes mode 0666 less the umask, as a file the author
// creates in an editor does.
//
// The data goes to a temporary file in the same directory first,
// synced to disk, and link gives it its name, which fails when the
// name exists: so path is either absent or whole, never half written,
// and of two runs that pick the same name one fails. A file system
// without hard links fails link with another error; there the file is
// written through an exclusive create (writeExclusive), which never
// replaces a file either, and is removed when its write fails, though
// a run killed during that write can leave a part of it. The temporary
// file is removed in every case; a run killed before that can leave it
// behind, hidden, with a name that starts with ".new-".
func createNew(path string, data []byte, link linkFunc) (err error) {
	tmp, err := writeTemp(filepath.Dir(path), data)
	if err != nil {
		return err // writeExclusive removed what it created
	}
	defer func() { err = errors.Join(err, os.Remove(tmp)) }()
	err = link(tmp, path)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		err = writeExclusive(path, data)
	}
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s exists, and a record is never replaced: %w", path, fs.ErrExist)
	}
	return err
}

// writeTemp writes data to a new hidden file of dir through
// writeExclusive, and returns its path. The name holds 128 random bits
// (crypto/rand.Text), so it is never one that exists.
func writeTemp(dir string, data []byte) (string, error) {
	path := filepath.Join(dir, ".new-"+rand.Text())
	return path, writeExclusive(path, data)
}

// writeExclusive creates the file name with mode 0666 less the umask,
// and fails when a file has that name. It writes data and syncs it to
// disk, and removes the file when either fails.
func writeExclusive(name string, data []byte) (err error) {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return nil
}

// record returns the text of a new record: the lines rule 6 fixes, and
// under each section a line that says what goes there.
func record(n int, title string, now time.Time) string {
	outline := []string{
		"Proposed",
		"What makes this decision necessary now, and what we measured before\nmaking it.\n\n" +
			"### Alternatives considered\n\n" +
			"- **The first alternative.** What it would have given us, and why we\n  did not choose it.",
		"We will ...",
		"What becomes easier, what becomes harder, and what would make us\nrevisit this decision.",
	}
	var b strings.Builder
	b.WriteString("# " + strconv.Itoa(n) + ". " + title + "\n\nDate: " + now.Format(time.DateOnly) + "\n")
	for i, heading := range adr.Sections {
		b.WriteString("\n" + heading + "\n\n" + outline[i] + "\n")
	}
	return b.String()
}
