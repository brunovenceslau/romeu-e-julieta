// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package hostpaths is the one place of romeu's host paths: the path of
// the host settings file, the state directory, the reader of the
// settings, the resolution of links in a path, and the root check of
// root-unsafe (04 4.2; I20). init calls all of it; approve and sync take
// the settings and the state directory from it; doctor (T049) is to
// call the root check for its root-unsafe row.
//
// The settings path is built from the environment by spec.SettingsPath
// and the state directory by state.DefaultDir, which their packages
// keep; a command takes both from here, which adds the path the binary
// or a test set in cli.Env.
package hostpaths

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// SettingsPath returns the path of the host settings file (03 3.4):
// env.SettingsPath when the binary or a test set it, or the path the
// environment getenv reads names (spec.SettingsPath: ROMEU_SETTINGS,
// XDG_CONFIG_HOME, HOME; REQ-0148, REQ-0150). A env.SettingsPath that is
// not absolute is refused, as spec.SettingsPath refuses ROMEU_SETTINGS,
// so the path never depends on the working directory.
func SettingsPath(env *cli.Env, getenv func(string) string) (string, error) {
	if env != nil && env.SettingsPath != "" {
		if !filepath.IsAbs(env.SettingsPath) {
			return "", fmt.Errorf("the settings path %q is not an absolute path", env.SettingsPath)
		}
		return filepath.Clean(env.SettingsPath), nil
	}
	return spec.SettingsPath(getenv)
}

// StateDir returns the host state directory of the environment getenv
// reads (state.DefaultDir: XDG_STATE_HOME, HOME; REQ-0150).
func StateDir(getenv func(string) string) (string, error) {
	return state.DefaultDir(getenv)
}

// LoadSettings reads and validates the host settings (03 3.4) at the
// path SettingsPath returns. Settings that are absent, or that do not
// decode, are the error usage returns for the slug settings-missing or
// settings-invalid and the message; usage is the command's, so the hint
// names it. An error of the table the decoder raises (format-newer,
// secret-in-argv) passes through.
func LoadSettings(env *cli.Env, getenv func(string) string, usage func(slug, message string) error) (spec.HostSettings, error) {
	path, err := SettingsPath(env, getenv)
	if err != nil {
		return spec.HostSettings{}, usage("settings-missing", err.Error())
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
