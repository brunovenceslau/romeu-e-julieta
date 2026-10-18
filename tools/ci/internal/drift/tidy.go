// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package drift

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// tidyFiles are the files "go mod tidy" writes in the module of the
// repository.
var tidyFiles = []string{"go.mod", "go.sum"}

// tidy is the tidy rule of the generated check (10 10.2): it runs step,
// which is "go mod tidy" in the environment of the steps, in the root of
// repo, and reports go.mod and go.sum when it changed, created or
// removed one. The tree is left as tidy wrote it, as generate leaves it,
// so the fix is to commit the result. It reads and writes through the
// root of the repository, as generated does, and reads no network: the
// environment of the steps has GOPROXY=off.
func tidy(ctx context.Context, repo git.Repo, step check.Step) ([]check.Finding, error) {
	root, err := os.OpenRoot(repo.Dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	before, err := readTidyFiles(root)
	if err != nil {
		return nil, err
	}
	if out, err := step.Run(ctx, repo.Dir); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", step.Name, err, out)
	}
	after, err := readTidyFiles(root)
	if err != nil {
		return nil, err
	}
	var findings []check.Finding
	for _, name := range tidyFiles {
		had, hadFile := before[name]
		has, hasFile := after[name]
		switch {
		case hadFile && !hasFile:
			findings = append(findings, check.Finding{Where: name, Rule: "tidy", Msg: "go mod tidy removed this file; commit the removal"})
		case !hadFile && hasFile:
			findings = append(findings, check.Finding{Where: name, Rule: "tidy", Msg: "go mod tidy created this file; run it and commit the result"})
		case !bytes.Equal(had, has):
			findings = append(findings, check.Finding{Where: name, Rule: "tidy", Msg: "go mod tidy changed this file; run it and commit the result"})
		}
	}
	return findings, nil
}

// readTidyFiles returns the bytes of the files of tidyFiles that exist.
func readTidyFiles(root *os.Root) (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, name := range tidyFiles {
		data, err := root.ReadFile(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	return files, nil
}
