// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hostpaths

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// envOf returns a getenv over m.
func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestSettingsPath: the path cli.Env holds wins; without one, the
// environment names it (TestSettingsPathOfTheEnvironment).
func TestSettingsPath(t *testing.T) {
	getenv := envOf(map[string]string{"XDG_CONFIG_HOME": "/x", "HOME": "/h"})
	for _, tc := range []struct {
		name    string
		env     *cli.Env
		getenv  func(string) string
		want    string
		wantErr bool
	}{
		{"the path of cli.Env", &cli.Env{SettingsPath: "/e/s.yaml"}, getenv, "/e/s.yaml", false},
		{"the environment", &cli.Env{}, getenv, "/x/romeu/settings.yaml", false},
		{"no cli.Env", nil, getenv, "/x/romeu/settings.yaml", false},
		{"no path at all", &cli.Env{}, envOf(nil), "", true},
		{"a relative path of cli.Env", &cli.Env{SettingsPath: "s.yaml"}, getenv, "", true},
		{"a path of cli.Env made clean", &cli.Env{SettingsPath: "/e/./s.yaml"}, getenv, "/e/s.yaml", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SettingsPath(tc.env, tc.getenv)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestSettingsPathOfTheEnvironment: ROMEU_SETTINGS, then
// XDG_CONFIG_HOME, then HOME; nothing else moves it. Every romeu command
// reads the path through SettingsPath, so they agree on every case.
func TestSettingsPathOfTheEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
		err  bool
	}{
		{"override", map[string]string{"ROMEU_SETTINGS": "/a/s.yaml", "XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/a/s.yaml", false},
		{"override is cleaned", map[string]string{"ROMEU_SETTINGS": "/a//b/../s.yaml"}, "/a/s.yaml", false},
		{"override must be absolute", map[string]string{"ROMEU_SETTINGS": "s.yaml", "HOME": "/h"}, "", true},
		{"xdg", map[string]string{"XDG_CONFIG_HOME": "/x", "HOME": "/h"}, "/x/romeu/settings.yaml", false},
		{"relative xdg is unset", map[string]string{"XDG_CONFIG_HOME": "x", "HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"home", map[string]string{"HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"other variables", map[string]string{"ROMEU_ROOT": "/r", "ROMEU_CONFIG": "/c", "HOME": "/h"}, "/h/.config/romeu/settings.yaml", false},
		{"nothing", map[string]string{}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SettingsPath(nil, envOf(tc.env))
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestStateDir: XDG_STATE_HOME, then HOME (state.DefaultDir).
func TestStateDir(t *testing.T) {
	got, err := StateDir(envOf(map[string]string{"XDG_STATE_HOME": "/x", "HOME": "/h"}))
	require.NoError(t, err)
	assert.Equal(t, "/x/romeu", got)
	_, err = StateDir(envOf(nil))
	require.Error(t, err)
}

// TestLoadSettings: settings that decode are returned; absent settings
// and a path the environment cannot name are settings-missing, settings
// that do not decode settings-invalid, each through the command's usage;
// an error of the table the decoder raises passes through.
func TestLoadSettings(t *testing.T) {
	tmp := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(tmp, name)
		require.NoError(t, os.WriteFile(p, data, 0o600))
		return p
	}
	good, err := spec.EncodeHostSettings(spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   "/h/Developer",
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: "/usr/bin/sbx", Git: "/usr/bin/git"},
	})
	require.NoError(t, err)
	usage := func(slug, message string) error { return errors.New(slug + ": " + message) }

	h, err := LoadSettings(&cli.Env{SettingsPath: write("good.yaml", good)}, envOf(nil), usage)
	require.NoError(t, err)
	assert.Equal(t, "/h/Developer", h.Root)

	for _, tc := range []struct {
		name string
		env  *cli.Env
		want string
	}{
		{"absent", &cli.Env{SettingsPath: filepath.Join(tmp, "none.yaml")}, "settings-missing: no host settings at "},
		{"no path", &cli.Env{}, "settings-missing: neither ROMEU_SETTINGS"},
		{"not yaml", &cli.Env{SettingsPath: write("bad.yaml", []byte("root: [\n"))}, "settings-invalid: "},
		{"a directory", &cli.Env{SettingsPath: tmp}, "settings-invalid: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadSettings(tc.env, envOf(nil), usage)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
	t.Run("an error of the table passes through", func(t *testing.T) {
		newer := strings.Replace(string(good), "host-settings/v1", "host-settings/v99", 1)
		_, err := LoadSettings(&cli.Env{SettingsPath: write("newer.yaml", []byte(newer))}, envOf(nil), usage)
		require.Error(t, err)
		_, ok := errors.AsType[*cli.Error](err)
		assert.True(t, ok, err.Error())
	})
}

// settingsFile is the name of the host settings file, which only this
// package spells out in a path.
const settingsFile = "settings.yaml"

// allowedSettingsSites are the findings of the call-site test that are
// not a copy of the settings path, by file, each with its reason. The
// test fails once one is no longer found, so the list only shrinks.
var allowedSettingsSites = map[string]map[string]string{
	"internal/preflight/check_trust.go": {
		`"XDG_CONFIG_HOME"`: "the config directory of mise, which the auto-trust check scans (04 4.2), not romeu's settings",
	},
	"internal/spec/validate.go": {
		`"settings.yaml"`: "the file a finding of the settings' validation names, not a path",
	},
	"e2e/internal/romeu/env.go": {
		`"settings.yaml"`: "the e2e harness names the settings path on its own, to check romeu's from outside",
	},
}

// TestNoOtherPackageBuildsTheSettingsPath: outside this package, which
// alone builds the settings path, no Go file of the module (tests aside)
// holds the string "ROMEU_SETTINGS", "XDG_CONFIG_HOME" or
// "settings.yaml", so every command takes the settings path from
// SettingsPath and a copy built by hand from the environment is caught.
// allowedSettingsSites holds the exceptions. It matches whole string
// literals only, so a name joined from parts or taken from a constant of
// another package escapes it: it guards against a copy made by accident.
func TestNoOtherPackageBuildsTheSettingsPath(t *testing.T) {
	// The test runs in internal/hostpaths, two levels below the root.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "go.mod"))

	found := map[string][]string{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/hostpaths/") {
			return nil
		}
		if hits := settingsSites(t, p); len(hits) > 0 {
			found[rel] = hits
		}
		return nil
	})
	require.NoError(t, err)

	for f, hits := range found {
		for _, hit := range hits {
			_, ok := allowedSettingsSites[f][hit]
			assert.True(t, ok, "%s: %s builds the settings path by hand; take it from hostpaths.SettingsPath", f, hit)
		}
	}
	for f, hits := range allowedSettingsSites {
		for hit := range hits {
			assert.Contains(t, found[f], hit, "%s no longer holds %s: remove it from allowedSettingsSites", f, hit)
		}
	}
}

// settingsSites returns the strings of the settings path the Go file at
// p holds, quoted: "ROMEU_SETTINGS", "XDG_CONFIG_HOME" and
// "settings.yaml".
func settingsSites(t *testing.T, p string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
	require.NoError(t, err, p)
	literals := []string{SettingsEnv, "XDG_CONFIG_HOME", settingsFile}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		if n, ok := n.(*ast.BasicLit); ok && n.Kind == token.STRING {
			if s, err := strconv.Unquote(n.Value); err == nil && slices.Contains(literals, s) {
				hits = append(hits, strconv.Quote(s))
			}
		}
		return true
	})
	return hits
}
