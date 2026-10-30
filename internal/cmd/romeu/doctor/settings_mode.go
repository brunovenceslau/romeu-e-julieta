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

	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The check settings-mode (04 4.2, REQ-0381): the host settings file is
// not 0600, or the state directory is not 0700. The modes are read with
// Lstat: a link in place of either is not the file romeu wrote. A state
// directory that does not exist yet has no mode to judge.
var _ = register(row{slug: "settings-mode", run: checkSettingsMode})

func checkSettingsMode(_ context.Context, h *host) Check {
	const slug = "settings-mode"
	if h.baseErr != nil {
		return errorOf(slug, h.baseErr)
	}
	if h.stateErr != nil {
		return errorOf(slug, h.stateErr)
	}
	var found []string
	for _, f := range []struct {
		path string
		mode fs.FileMode
		dir  bool
	}{{h.settingsPath, state.FileMode, false}, {h.stateDir, state.DirMode, true}} {
		info, err := os.Lstat(f.path)
		switch {
		case errors.Is(err, fs.ErrNotExist) && f.dir:
			continue
		case err != nil:
			return errorOf(slug, err)
		}
		if info.Mode() != f.mode|dirBit(f.dir) {
			found = append(found, fmt.Sprintf("%s is %v, not %v; chmod %04o %s", f.path, info.Mode(), f.mode|dirBit(f.dir), f.mode, f.path))
		}
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, "")
}

// dirBit is fs.ModeDir for a directory.
func dirBit(dir bool) fs.FileMode {
	if dir {
		return fs.ModeDir
	}
	return 0
}
