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
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), "-env"); ok && name != "" && e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
