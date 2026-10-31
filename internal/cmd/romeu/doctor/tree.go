// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// envDirs returns the name of each <name>-env/ directly under root, in
// name order: each entry whose name ends in -env, with a name before
// it, that is a directory and not a link. A root that does not exist
// yet holds none.
func envDirs(root string) ([]string, error) {
	dirs, _, err := envEntries(root)
	return dirs, err
}

// envEntries returns the <name>-env entries directly under root, in name
// order, split into the directories and the links; another kind of
// entry is neither. A root that does not exist yet holds none.
func envEntries(root string) (dirs, links []string, err error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), "-env")
		switch {
		case !ok || name == "":
		case e.IsDir():
			dirs = append(dirs, e.Name())
		case e.Type()&fs.ModeSymlink != 0:
			links = append(links, e.Name())
		}
	}
	return dirs, links, nil
}

// notMade is what a check says of a link in place of a directory of the
// host tree: romeu made a directory there and follows no link (I12).
func notMade(p string) string { return p + " is a link, not the directory romeu made" }
