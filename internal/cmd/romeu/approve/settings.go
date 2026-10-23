// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// settingsEnv is the one romeu-specific override of a path, the path of
// settings.yaml (02 2.4, REQ-0150).
const settingsEnv = "ROMEU_SETTINGS"

// settingsPath returns the path of settings.yaml for the environment
// getenv reads: ROMEU_SETTINGS, or $XDG_CONFIG_HOME/romeu/settings.yaml,
// or $HOME/.config/romeu/settings.yaml (REQ-0148). An XDG variable that
// is not an absolute path counts as unset; a ROMEU_SETTINGS that is not
// absolute is refused. It is the rule of romeu init
// (internal/cmd/romeu/init, paths.go), kept private there; one shared
// place for both is a follow-up that the next command reading the
// settings reopens.
func settingsPath(getenv func(string) string) (string, error) {
	if p := getenv(settingsEnv); p != "" {
		if !filepath.IsAbs(p) {
			return "", fmt.Errorf("%s is not an absolute path", settingsEnv)
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

// loadSettings reads and validates the host settings (03 3.4): gate 1
// runs the sbx they name. Settings that are absent, or that do not
// decode, are usage (RJ-349) with the slug settings-missing or
// settings-invalid; an error of the table the decoder raises
// (format-newer, secret-in-argv) passes through.
func loadSettings(env *cli.Env) (spec.HostSettings, error) {
	path := env.SettingsPath
	if path == "" {
		var err error
		if path, err = settingsPath(getenv); err != nil {
			return spec.HostSettings{}, usage("settings-missing", err.Error())
		}
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return spec.HostSettings{}, usage("settings-missing", "no host settings at "+termsafe.Escape(path)+"; run romeu init first")
	}
	if err != nil {
		return spec.HostSettings{}, usage("settings-invalid", termsafe.Escape(err.Error()))
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, spec.MaxFileSize+1))
	if err != nil {
		return spec.HostSettings{}, usage("settings-invalid", termsafe.Escape(err.Error()))
	}
	h, err := spec.DecodeHostSettings(data)
	if _, ok := errors.AsType[*cli.Error](err); ok {
		return spec.HostSettings{}, err
	}
	if err != nil {
		return spec.HostSettings{}, usage("settings-invalid", termsafe.Escape(path)+": "+termsafe.Escape(err.Error()))
	}
	return h, nil
}
