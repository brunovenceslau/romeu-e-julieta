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

// pendingSettingsSites are the files outside this task's file set that
// still build the settings path themselves: each moves to SettingsPath
// in a change of its own, and the test fails once one no longer does,
// so the list only shrinks.
var pendingSettingsSites = []string{
	// The memory push of romeu (T175), outside the file set of T169.
	"internal/cmd/romeu/all/memory_push.go",
}

// TestNoOtherPackageBuildsTheSettingsPath: outside internal/spec, which
// builds the path, and this package, no Go file of the module (tests
// aside) refers to spec.SettingsPath or spec.SettingsEnv or holds the
// string "ROMEU_SETTINGS", so every command takes the settings path from
// SettingsPath. A dot import of internal/spec is refused, since its bare
// names would escape the scan.
func TestNoOtherPackageBuildsTheSettingsPath(t *testing.T) {
	// The test runs in internal/hostpaths, two levels below the root.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "go.mod"))
	specPath := reflect.TypeFor[spec.HostSettings]().PkgPath()

	var found []string
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
		if rel == "internal/spec/settings_path.go" || strings.HasPrefix(rel, "internal/hostpaths/") {
			return nil
		}
		if settingsSite(t, p, specPath) {
			found = append(found, rel)
		}
		return nil
	})
	require.NoError(t, err)

	for _, f := range found {
		assert.Contains(t, pendingSettingsSites, f, "%s builds the settings path; take it from hostpaths.SettingsPath", f)
	}
	for _, f := range pendingSettingsSites {
		assert.True(t, slices.Contains(found, f), "%s no longer builds the settings path: remove it from pendingSettingsSites", f)
	}
}

// settingsSite reports whether the Go file at p refers to the settings
// path of internal/spec (the package at specPath) or holds the string
// "ROMEU_SETTINGS".
func settingsSite(t *testing.T, p, specPath string) bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
	require.NoError(t, err, p)
	var names []string
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
			return true
		default:
			names = append(names, im.Name.Name)
		}
	}
	hit := false
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok && slices.Contains(names, x.Name) && (n.Sel.Name == "SettingsPath" || n.Sel.Name == "SettingsEnv") {
				hit = true
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				if s, err := strconv.Unquote(n.Value); err == nil && s == spec.SettingsEnv {
					hit = true
				}
			}
		}
		return !hit
	})
	return hit
}
