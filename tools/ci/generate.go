// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

//go:generate go run . generate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The files that generate writes (12 12.3). A generated file is never
// edited by hand: generated fails when the run of generate changes one.
const (
	codeownersPath    = ".github/CODEOWNERS"
	askFirstPagePath  = "docs/reference/ask-first.md"
	adrIndexName      = "README.md"
	adrDirFileName    = ".adr-dir"
	generatedFileMode = 0o644
)

// runGenerate is the "generate" subcommand, which "go generate ./..."
// runs through the directive above. It writes the consumers of the
// tables of 12 12.3 that exist today: the two of .github/ask-first.yaml
// (the third, the mutate trigger, lands with mutate) and the index of
// the decision records. It says which files it wrote, and none when the
// tree was current.
func runGenerate(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generate takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	outputs, err := generateOutputs(root)
	if err != nil {
		return false, err
	}
	for _, out := range outputs {
		wrote, err := writeIfChanged(filepath.Join(root, filepath.FromSlash(out.path)), out.content)
		if err != nil {
			return false, err
		}
		if wrote {
			say(e.stdout, "wrote %s\n", out.path)
		}
	}
	return true, nil
}

// output is one generated file: its path from the root and its bytes.
type output struct {
	path    string
	content []byte
}

// generateOutputs reads the sources below root and returns the files
// that follow from them. It writes nothing.
func generateOutputs(root string) ([]output, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(askFirstPath)))
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
// .adr-dir names, and returns it with its path from the root.
func readADRIndex(root string) ([]byte, string, error) {
	rel, err := os.ReadFile(filepath.Join(root, adrDirFileName))
	if err != nil {
		return nil, "", err
	}
	dir := strings.TrimSpace(string(rel))
	if !filepath.IsLocal(dir) {
		return nil, "", fmt.Errorf("%s names %q, which is not a directory inside the repository", adrDirFileName, dir)
	}
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return nil, "", err
	}
	var records []adr
	for _, entry := range entries {
		if !adrName.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, dir, entry.Name()))
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

// writeIfChanged writes content to path, with its directories, unless
// the file holds exactly those bytes already, and reports whether it
// wrote. A file that is current keeps its modification time.
func writeIfChanged(path string, content []byte) (bool, error) {
	old, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(old, content):
		return false, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, content, generatedFileMode)
}
