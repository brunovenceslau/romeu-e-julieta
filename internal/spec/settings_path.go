// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"errors"
	"fmt"
	"path/filepath"
)

// SettingsEnv is the one romeu-specific override of a path: the path of
// settings.yaml, used by tests (02 2.4, REQ-0150). No other ROMEU_*
// variable moves a path, and the root comes only from the settings file.
const SettingsEnv = "ROMEU_SETTINGS"

// SettingsPath returns the path of the host settings file, settings.yaml,
// for the environment getenv reads: ROMEU_SETTINGS, or
// $XDG_CONFIG_HOME/romeu/settings.yaml, or
// $HOME/.config/romeu/settings.yaml (REQ-0148). A XDG variable that is
// not an absolute path counts as unset, as the XDG specification says; a
// ROMEU_SETTINGS that is not absolute is refused. Every romeu command
// that reads or writes the settings takes the path from here.
func SettingsPath(getenv func(string) string) (string, error) {
	if p := getenv(SettingsEnv); p != "" {
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("%s is not an absolute path", SettingsEnv)
		}
		return filepath.Clean(p), nil
	}
	if x := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "romeu", "settings.yaml"), nil
	}
	if h := getenv("HOME"); filepath.IsAbs(h) {
		return filepath.Join(h, ".config", "romeu", "settings.yaml"), nil
	}
	return "", errors.New("neither ROMEU_SETTINGS, XDG_CONFIG_HOME nor HOME names an absolute path")
}
