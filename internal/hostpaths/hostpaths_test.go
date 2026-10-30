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
	"reflect"
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
// environment names it (spec.SettingsPath).
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

// settingsFile is the name of the host settings file, which only
// internal/spec, internal/state and this package spell out.
const settingsFile = "settings.yaml"

// allowedSettingsSites are the findings of the call-site test that are
// not a copy of the settings path, by file, each with its reason. The
// test fails once one is no longer found, so the list only shrinks.
var allowedSettingsSites = map[string]map[string]string{
	"internal/preflight/check_trust.go": {
		`"XDG_CONFIG_HOME"`: "the config directory of mise, which the auto-trust check scans (04 4.2), not romeu's settings",
	},
	"e2e/internal/romeu/env.go": {
		`"settings.yaml"`: "the e2e harness names the settings path on its own, to check romeu's from outside",
	},
	// Pending: the memory push of romeu (T175), outside the file set of
	// T169, still builds the settings path; it moves to SettingsPath in a
	// change of its own.
	"internal/cmd/romeu/all/memory_push.go": {
		"spec.SettingsPath": "pending: moves to hostpaths.SettingsPath",
	},
}

// TestNoOtherPackageBuildsTheSettingsPath: outside internal/spec, which
// builds the path, and this package, no Go file of the module (tests
// aside) refers to spec.SettingsPath or spec.SettingsEnv, or holds the
// string "ROMEU_SETTINGS" or "XDG_CONFIG_HOME", and outside those two
// and internal/state none holds "settings.yaml", so every command takes
// the settings path from SettingsPath and a copy built by hand from the
// environment is caught. allowedSettingsSites holds the exceptions. A dot
// import of internal/spec is refused, since its bare names would escape
// the scan.
func TestNoOtherPackageBuildsTheSettingsPath(t *testing.T) {
	// The test runs in internal/hostpaths, two levels below the root.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "go.mod"))
	specPath := reflect.TypeFor[spec.HostSettings]().PkgPath()

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
		if strings.HasPrefix(rel, "internal/spec/") || strings.HasPrefix(rel, "internal/hostpaths/") {
			return nil
		}
		for _, hit := range settingsSites(t, p, specPath) {
			if hit == strconv.Quote(settingsFile) && strings.HasPrefix(rel, "internal/state/") {
				continue
			}
			found[rel] = append(found[rel], hit)
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

// settingsSites returns what the Go file at p holds of the settings
// path: a reference to spec.SettingsPath or spec.SettingsEnv of
// internal/spec (the package at specPath), a dot import of it, and the
// strings "ROMEU_SETTINGS", "XDG_CONFIG_HOME" and "settings.yaml",
// quoted.
func settingsSites(t *testing.T, p, specPath string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
	require.NoError(t, err, p)
	var names, hits []string
	for _, im := range f.Imports {
		path, err := strconv.Unquote(im.Path.Value)
		require.NoError(t, err)
		if path != specPath {
			continue
		}
		switch {
		case im.Name == nil:
			names = append(names, "spec")
		case im.Name.Name == ".":
			hits = append(hits, "a dot import of internal/spec")
		default:
			names = append(names, im.Name.Name)
		}
	}
	literals := []string{spec.SettingsEnv, "XDG_CONFIG_HOME", settingsFile}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok && slices.Contains(names, x.Name) && (n.Sel.Name == "SettingsPath" || n.Sel.Name == "SettingsEnv") {
				hits = append(hits, "spec."+n.Sel.Name)
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				if s, err := strconv.Unquote(n.Value); err == nil && slices.Contains(literals, s) {
					hits = append(hits, strconv.Quote(s))
				}
			}
		}
		return true
	})
	return hits
}
