// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// The check tree-mode (04 4.2, REQ-0377): $ROMEU_ROOT/.attic or a
// <name>-env/ that its group or others can access, by any of read,
// write or search. A link in place of either is a finding too: it is
// not the directory romeu made, and its mode is the target's.
var _ = register(row{slug: "tree-mode", run: checkTreeMode})

func checkTreeMode(_ context.Context, h *host) Check {
	const slug = "tree-mode"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	root := h.settings.Root
	dirs, links, err := envEntries(root)
	if err != nil {
		return errorOf(slug, err)
	}
	dirs = append([]string{".attic"}, dirs...)
	var found []string
	for _, l := range links {
		found = append(found, notMade(filepath.Join(root, l)))
	}
	for _, d := range dirs {
		p := filepath.Join(root, d)
		info, err := os.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return errorOf(slug, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			found = append(found, notMade(p))
			continue
		}
		if info.IsDir() && info.Mode().Perm()&0o077 != 0 {
			found = append(found, fmt.Sprintf("%s is mode %04o; chmod 0700 %s", p, info.Mode().Perm(), p))
		}
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, "")
}
