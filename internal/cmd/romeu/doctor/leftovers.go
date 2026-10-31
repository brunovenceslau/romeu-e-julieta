// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"strings"
)

// leftovers returns the temporary files an interrupted atomic write left
// in the state directory (03 3.8): the dotfiles renameio's pending files
// leave beside the record they were to replace, in the state directory,
// projects/ and descriptors/. doctor lists them for the operator to
// remove and deletes none (T049, re-plan of 2026-10-22); a directory it
// cannot read lists none. Each path is made valid UTF-8, for the JSON
// and the text output alike. It is never nil.
func leftovers(h *host) []string {
	out := []string{}
	if h.store == nil {
		return out
	}
	for _, dir := range []string{h.stateDir, filepath.Join(h.stateDir, "projects"), filepath.Join(h.stateDir, "descriptors")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") && e.Type().IsRegular() {
				// A name is any bytes; one that is not UTF-8 would stop
				// the JSON encoder and lose the whole report.
				out = append(out, strings.ToValidUTF8(filepath.Join(dir, e.Name()), "\uFFFD"))
			}
		}
	}
	return out
}
