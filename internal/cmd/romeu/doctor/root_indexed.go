// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The check root-indexed (04 4.2, REQ-0376), a warning: the root under
// a folder a cloud service syncs (~/Library/Mobile Documents,
// ~/Library/CloudStorage, ~/Dropbox), or a <name>-env/ without its
// .metadata_never_index marker, the file sync writes (step 4). Whether
// the root is excluded from Time Machine is not checked: it needs
// tmutil, which no admitted call site of 04 4.1 runs.
var _ = register(row{slug: "root-indexed", run: checkRootIndexed})

// neverIndex is the marker that keeps Spotlight out of a <name>-env/.
const neverIndex = ".metadata_never_index"

// syncedFolders are the folders below HOME that a cloud service syncs.
var syncedFolders = []string{
	filepath.Join("Library", "Mobile Documents"),
	filepath.Join("Library", "CloudStorage"),
	"Dropbox",
}

func checkRootIndexed(_ context.Context, h *host) Check {
	const slug = "root-indexed"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	root := h.settings.Root
	var found []string
	for _, f := range syncedFolders {
		if within(filepath.Join(h.home, f), root) {
			found = append(found, "the root "+root+" is under ~/"+f+", which a cloud service syncs")
		}
	}
	envs, err := envDirs(root)
	if err != nil {
		return errorOf(slug, err)
	}
	for _, env := range envs {
		info, err := os.Lstat(filepath.Join(root, env, neverIndex))
		switch {
		case errors.Is(err, fs.ErrNotExist) || err == nil && !info.Mode().IsRegular():
			found = append(found, filepath.Join(root, env)+" has no "+neverIndex+" marker")
		case err != nil:
			return errorOf(slug, err)
		}
	}
	if len(found) > 0 {
		return warn(slug, joined(found)+"; a root excluded from Time Machine needs another copy of its memory dirs, handoffs and unpushed work")
	}
	return pass(slug, "")
}

// within reports whether p is dir or a path below it.
func within(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
