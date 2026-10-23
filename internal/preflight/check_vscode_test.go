// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"errors"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
)

// TestPreflightStep5VSCodeSettings shows I23 and I14 (REQ-0380): VS Code
// workspace trust turned off fails the step, in the settings file of the
// platform, comments and trailing commas read as VS Code reads them.
func TestPreflightStep5VSCodeSettings(t *testing.T) {
	tests := []struct {
		name     string
		os       string
		relPath  string
		content  string
		wantErr  bool
		wantFile bool
	}{
		{"no settings file passes", "linux", "", "", false, false},
		{"trust left on", "linux", ".config/Code/User/settings.json", `{"security.workspace.trust.enabled": true}`, false, true},
		{"trust member absent", "linux", ".config/Code/User/settings.json", `{"editor.fontSize": 14}`, false, true},
		{"trust off on linux", "linux", ".config/Code/User/settings.json", `{"security.workspace.trust.enabled": false}`, true, true},
		{"trust off on darwin", "darwin", "Library/Application Support/Code/User/settings.json", `{"security.workspace.trust.enabled": false}`, true, true},
		{"a darwin settings file is not read on linux", "linux", "Library/Application Support/Code/User/settings.json", `{"security.workspace.trust.enabled": false}`, false, true},
		{"trust off among comments and trailing commas", "linux", ".config/Code/User/settings.json",
			"// my settings\n{\n  /* block */ \"editor.fontSize\": 14, // size\n  \"files.exclude\": {\"**/.git\": true,},\n  \"security.workspace.trust.enabled\": false,\n}\n", true, true},
		{"a comment marker inside a string is no comment", "linux", ".config/Code/User/settings.json",
			`{"http.proxy": "http://host//path", "x": "a/*b*/c", "security.workspace.trust.enabled": false}`, true, true},
		{"settings after a byte order mark that leave trust on pass", "linux", ".config/Code/User/settings.json", "\xef\xbb\xbf{\"editor.fontSize\": 14}", false, true},
		{"trust off after a byte order mark", "linux", ".config/Code/User/settings.json", "\xef\xbb\xbf{\"security.workspace.trust.enabled\": false}", true, true},
		{"a settings file that does not parse cannot be checked", "linux", ".config/Code/User/settings.json", `{"a": `, true, true},
		{"a mistyped trust member cannot be checked", "linux", ".config/Code/User/settings.json", `{"security.workspace.trust.enabled": "no"}`, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := quietHost(t)
			host.OS = tt.os
			if tt.content != "" {
				write(t, filepath.Join(host.Home, tt.relPath), tt.content)
			}
			err := hostChecksOnly(t, host, nil)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-333", err)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Contains(t, cli.Of(err).Hint(), "security.workspace.trust.enabled")
		})
	}
	t.Run("XDG_CONFIG_HOME moves the settings on linux", func(t *testing.T) {
		host := quietHost(t)
		cfg := filepath.Join(host.Home, "cfg")
		write(t, filepath.Join(cfg, "Code", "User", "settings.json"), `{"security.workspace.trust.enabled": false}`)
		host.Getenv = func(k string) string {
			if k == "XDG_CONFIG_HOME" {
				return cfg
			}
			return ""
		}
		testsupport.RequireID(t, "RJ-333", hostChecksOnly(t, host, nil))
	})
}

// TestPreflightStep5VSCodeTrustedFolders shows the trusted folders of
// REQ-0380: the root, a parent of it and a folder under it fail the
// step; a seam that reads them stands in for VS Code's state database.
func TestPreflightStep5VSCodeTrustedFolders(t *testing.T) {
	uri := func(p string) string { return (&url.URL{Scheme: "file", Path: p}).String() }
	tests := []struct {
		name    string
		folders func(root string) []string
		wantErr bool
	}{
		{"none", func(string) []string { return nil }, false},
		{"an unrelated folder", func(string) []string { return []string{"/opt/elsewhere", uri("/opt/else")} }, false},
		{"the root", func(r string) []string { return []string{r} }, true},
		{"the root as a URI", func(r string) []string { return []string{uri(r)} }, true},
		{"a parent of the root", func(r string) []string { return []string{uri(filepath.Dir(r))} }, true},
		{"a folder under the root", func(r string) []string { return []string{uri(filepath.Join(r, "shop-env", "shop-api"))} }, true},
		{"a URI with an escaped space", func(r string) []string { return []string{uri(filepath.Join(r, "a b"))} }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := quietHost(t)
			err := hostChecksOnly(t, host, func(d *Deps) {
				d.VSCodeTrustedFolders = func() ([]string, error) { return tt.folders(host.Root), nil }
			})
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-333", err)
		})
	}
	t.Run("a seam that fails cannot be checked", func(t *testing.T) {
		err := hostChecksOnly(t, quietHost(t), func(d *Deps) {
			d.VSCodeTrustedFolders = func() ([]string, error) { return nil, errors.New("no database") }
		})
		testsupport.RequireID(t, "RJ-333", err)
	})
}

// TestPreflightStripJSONC shows the reading of the comments and commas
// of the settings of VS Code.
func TestPreflightStripJSONC(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", `{"a": 1}`, `{"a": 1}`},
		{"line comment", "{\"a\": 1} // x", "{\"a\": 1} \n"},
		{"block comment", `{/* x */"a": 1}`, `{ "a": 1}`},
		{"trailing comma in an object", `{"a": 1,}`, `{"a": 1}`},
		{"trailing comma in an array with a space", `[1, 2, ]`, `[1, 2 ]`},
		{"comma kept between members", `{"a": 1, "b": 2}`, `{"a": 1, "b": 2}`},
		{"slashes in a string", `{"a": "//x"}`, `{"a": "//x"}`},
		{"an escaped quote in a string", `{"a": "q\" // not a comment"}`, `{"a": "q\" // not a comment"}`},
		{"an unterminated string is left alone", `{"a": "x`, `{"a": "x`},
		{"a byte order mark", "\xef\xbb\xbf{\"a\": 1}", `{"a": 1}`},
		{"an unterminated block comment ends the text", `{"a": 1} /* x`, `{"a": 1}  `},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(stripJSONC([]byte(tt.in))))
		})
	}
}
