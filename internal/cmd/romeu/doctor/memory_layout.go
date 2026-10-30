// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The check memory-layout (04 4.2, REQ-0387): a memory dir of the tree,
// <name>-env/memory/<dir>/ or memory/_shared/, that breaks the layout
// allowlist of 08 8.1, judged by memstore.Check, the check julieta runs
// (I24); an entry of memory/ that is not a directory is a finding too.
// Every node is read with Lstat through an os.Root on the memory dir,
// so no link is followed.
var _ = register(row{slug: "memory-layout", run: checkMemoryLayout})

func checkMemoryLayout(_ context.Context, h *host) Check {
	const slug = "memory-layout"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	envs, err := envDirs(h.settings.Root)
	if err != nil {
		return errorOf(slug, err)
	}
	var found []string
	for _, env := range envs {
		memory := filepath.Join(h.settings.Root, env, "memory")
		entries, err := os.ReadDir(memory)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return errorOf(slug, err)
		}
		for _, e := range entries {
			rel := path.Join(env, "memory", e.Name())
			if !e.IsDir() {
				found = append(found, rel+": not a directory")
				continue
			}
			rep, err := checkMemoryDir(filepath.Join(memory, e.Name()))
			if err != nil {
				return errorOf(slug, err)
			}
			for _, f := range rep.Findings {
				found = append(found, path.Join(rel, f.Path)+": "+f.Rule)
			}
		}
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, "")
}

// checkMemoryDir runs memstore.Check on the memory dir at dir.
func checkMemoryDir(dir string) (memstore.Report, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return memstore.Report{}, err
	}
	defer func() { _ = root.Close() }()
	return memstore.Check(root)
}
