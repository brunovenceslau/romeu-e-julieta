// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

//go:generate go run . generate

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adr"
)

// The files generate writes besides the index of the records (12 12.3).
const (
	codeownersPath   = ".github/CODEOWNERS"
	askFirstPagePath = "docs/reference/ask-first.md"
)

// generatedFile is one file generate writes: its path from the top of
// the working tree, with forward slashes, and what it holds.
type generatedFile struct {
	path string
	data []byte
}

// generateFiles returns the files of the generator rows of 12 12.3 that
// tools/ci owns, from their sources in fsys, a tree whose root is the
// top of the repository: from .github/ask-first.yaml, CODEOWNERS and
// the reference page; from the title and the status of each record,
// the index of docs/adr. generate reads the working tree (os.DirFS),
// and the generated step the tree of the commit it judges (commitTree),
// so both run the same generators over the same kind of input. Every
// generator of the repository runs here, the one entry the generated
// step judges (12 12.3).
//
// The generator that reads the ask-first list is a tools/ci subcommand
// on purpose: the code that writes CODEOWNERS is then on the checks
// surface of the list (12 12.3).
func generateFiles(fsys fs.FS) ([]generatedFile, error) {
	goMod, err := readSource(fsys, "go.mod")
	if err != nil {
		return nil, err
	}
	module, err := modulePath(goMod)
	if err != nil {
		return nil, err
	}
	data, err := readSource(fsys, askFirstPath)
	if err != nil {
		return nil, err
	}
	list, err := parseAskFirst(data, module)
	if err != nil {
		return nil, err
	}
	if err := list.checkDirs(fsys); err != nil {
		return nil, fmt.Errorf("%s: %w", askFirstPath, err)
	}
	records, err := adr.Read(fsys, adr.Dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", adr.Dir, err)
	}
	return []generatedFile{
		{codeownersPath, codeowners(list)},
		{askFirstPagePath, askFirstPage(list)},
		{adr.Dir + "/" + adr.Index, adrIndex(records)},
	}, nil
}

// readSource reads the source name of the generators from fsys. It
// must be a regular file: a symbolic link is refused in the working
// tree and in a commit alike (fs.Lstat), as a record is (adr.Read), so
// the generators never follow a link, out of the tree or in a loop.
func readSource(fsys fs.FS, name string) ([]byte, error) {
	info, err := fs.Lstat(fsys, name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file; a source of the generators is never a link", name)
	}
	return fs.ReadFile(fsys, name)
}

// runGenerate writes the files of generateFiles, from the sources in
// the working tree, that differ from what the working tree holds, and
// names each one it writes. The generate directive at the top of this
// file runs it, so "go generate ./..." from the top of the tree runs
// it, and tools/ci generated checks that a commit holds what it writes.
// It writes nothing when a source cannot be read.
func runGenerate(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generate takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	files, err := generateFiles(os.DirFS(root))
	if err != nil {
		return false, err
	}
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.path))
		if current(path, f.data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, err
		}
		if err := writeFile(path, f.data); err != nil {
			return false, err
		}
		say(e.stdout, "generate: wrote %s\n", f.path)
	}
	return true, nil
}

// current reports whether the file at path is a regular file, not
// executable, that holds data: what generate writes, and what the
// generated step requires of the commit (regularMode). A symbolic link
// or an executable file with the right bytes is written again, so that
// generate also repairs the mode the step refuses.
func current(path string, data []byte) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 != 0 {
		return false
	}
	old, err := os.ReadFile(path)
	return err == nil && bytes.Equal(old, data)
}

// adrIndex returns docs/adr/README.md, the index of the records (ADR
// 0001, rule 7): a row per record with its number, its title linked to
// its file, and its status. The file name is escaped as a path segment,
// so a name the sequences check would refuse still makes a link and
// never Markdown of its own.
func adrIndex(records []adr.Record) []byte {
	var b bytes.Buffer
	b.WriteString("<!-- " + generatedFrom("the records in "+adr.Dir) + " -->\n\n")
	b.WriteString("# Decision records\n\n")
	b.WriteString("Each record holds one decision: why we made it, what we weighed\n" +
		"against it, and what it costs. An accepted record is never rewritten; a\n" +
		"new one supersedes it. `go run ./tools/new adr \"<title>\"` starts the\n" +
		"next one, with its number, its date and status Proposed.\n\n")
	b.WriteString("| Record | Title | Status |\n|---|---|---|\n")
	for _, r := range records {
		b.WriteString("| " + strconv.Itoa(r.Number) + " | [" + mdEscape(r.Title) + "](" + url.PathEscape(r.File) + ") | " + mdEscape(r.Status) + " |\n")
	}
	return b.Bytes()
}
