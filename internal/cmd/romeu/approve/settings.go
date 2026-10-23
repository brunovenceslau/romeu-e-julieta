// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// loadSettings reads and validates the host settings (03 3.4): gate 1
// runs the sbx they name. Settings that are absent, or that do not
// decode, are usage (RJ-349) with the slug settings-missing or
// settings-invalid; an error of the table the decoder raises
// (format-newer, secret-in-argv) passes through.
func loadSettings(env *cli.Env) (spec.HostSettings, error) {
	path := env.SettingsPath
	if path == "" {
		var err error
		if path, err = spec.SettingsPath(getenv); err != nil {
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
