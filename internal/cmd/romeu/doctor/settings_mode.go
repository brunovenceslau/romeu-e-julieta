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
// not a regular file of mode 0600 that this user owns, judged through
// the descriptor secret-argv reads its argv from (checkedSettings), or
// the state directory is not 0700, read with Lstat. A link in place of
// either is not what romeu wrote. A state directory that does not exist
// yet has no mode to judge.
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
	sf, err := h.checkedSettings()
	if err != nil {
		return errorOf(slug, err)
	}
	if sf.problem != "" {
		found = append(found, h.settingsPath+" "+sf.problem)
	}
	info, err := os.Lstat(h.stateDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return errorOf(slug, err)
	case info.Mode() != state.DirMode|fs.ModeDir:
		found = append(found, fmt.Sprintf("%s is %v, not %v; chmod %04o %s", h.stateDir, info.Mode(), state.DirMode|fs.ModeDir, state.DirMode, h.stateDir))
	}
	if len(found) > 0 {
		return fail(slug, joined(found))
	}
	return pass(slug, "")
}
