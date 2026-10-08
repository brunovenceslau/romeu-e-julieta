// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

//go:generate go run . generate

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
)

// The files that generate writes (12 12.3). A generated file is never
// edited by hand: generated fails when the run of generate changes one,
// and when one is not tracked as a regular file with the bytes that
// generate gives.
const (
	codeownersPath    = ".github/CODEOWNERS"
	askFirstPagePath  = "docs/reference/ask-first.md"
	adrIndexName      = "README.md"
	generatedFileMode = 0o644
)

// runGenerate is the "generate" subcommand, which "go generate ./..."
// runs through the directive above. It writes the consumers of the
// tables of 12 12.3 that exist today: the two of .github/ask-first.yaml
// (the third, the mutate trigger, lands with mutate) and the index of
// the decision records. It says which files it wrote, and none when the
// tree was current. It reads and writes through the root of the
// repository, so a symbolic link cannot send it outside.
func runGenerate(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generate takes no argument\n%s", usage)
	}
	dir, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	outputs, err := generateOutputs(root)
	if err != nil {
		return false, err
	}
	for _, out := range outputs {
		wrote, err := writeIfChanged(root, out.path, out.content)
		if err != nil {
			return false, err
		}
		if wrote {
			say(e.stdout, "wrote %s\n", out.path)
		}
	}
	return true, nil
}

// output is one generated file: its slash-separated path from the root
// and its bytes.
type output struct {
	path    string
	content []byte
}

// generateOutputs reads the sources below root and returns the files
// that follow from them. It writes nothing; "generated" calls it too, to
// learn which files must be tracked and with what bytes.
func generateOutputs(root *os.Root) ([]output, error) {
	src, err := root.ReadFile(askFirstPath)
	if err != nil {
		return nil, err
	}
	list, err := parseAskFirst(src)
	if err != nil {
		return nil, err
	}
	index, indexPath, err := readADRIndex(root)
	if err != nil {
		return nil, err
	}
	return []output{
		{codeownersPath, list.codeowners()},
		{askFirstPagePath, list.referencePage()},
		{indexPath, index},
	}, nil
}

// readADRIndex renders the index of the records in the directory that
// .adr-dir names, and returns it with its path from the root. A record
// is a regular file: a symbolic link with a record's name is skipped.
func readADRIndex(root *os.Root) ([]byte, string, error) {
	dir, err := adrdir.Dir(root)
	if err != nil {
		return nil, "", err
	}
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(dir))
	if err != nil {
		return nil, "", err
	}
	var records []adr
	for _, entry := range entries {
		if !adrdir.Name.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		content, err := root.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, "", err
		}
		record, err := parseADR(entry.Name(), content)
		if err != nil {
			return nil, "", err
		}
		records = append(records, record)
	}
	return adrIndex(records), filepath.ToSlash(filepath.Join(dir, adrIndexName)), nil
}

// writeIfChanged writes content to path below root, with its
// directories, unless a regular file holds exactly those bytes already
// and no execute bit, and reports whether it wrote. A file that is
// current keeps its modification time. The bytes go to a new file in
// the same directory that then replaces path, so a symbolic link or a
// hard link at path is replaced by a regular file and never written
// through: a local hard link to a file outside the repository keeps its
// own bytes.
func writeIfChanged(root *os.Root, path string, content []byte) (bool, error) {
	info, err := root.Lstat(path)
	switch {
	case err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 == 0:
		old, err := root.ReadFile(path)
		if err != nil {
			return false, err
		}
		if bytes.Equal(old, content) {
			return false, nil
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	dir := filepath.Dir(path)
	if err := root.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	tmp := filepath.Join(dir, "."+filepath.Base(path)+"."+rand.Text()+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, generatedFileMode)
	if err != nil {
		return false, err
	}
	_, err = f.Write(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	// The mode is set after the umask took its share, and an execute bit
	// is the one thing git tracks of a mode.
	if err == nil {
		err = root.Chmod(tmp, generatedFileMode)
	}
	if err == nil {
		err = root.Rename(tmp, path)
	}
	if err != nil {
		return false, errors.Join(err, removeIfExists(root, tmp))
	}
	return true, nil
}

// removeIfExists removes the file at path below root, and is not an
// error when there is none.
func removeIfExists(root *os.Root, path string) error {
	if err := root.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
