// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	jsonv2 "encoding/json/v2"
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

// TestPreflightStandardJSON shows the reading of the comments and commas
// of the settings of VS Code: each text reads as the value VS Code reads,
// and a text that is not JSONC is an error, never a guess.
func TestPreflightStandardJSON(t *testing.T) {
	tests := []struct {
		name, in string
		want     map[string]any
		invalid  bool
	}{
		{name: "plain", in: `{"a": "1"}`, want: map[string]any{"a": "1"}},
		{name: "line comment", in: "{\"a\": \"1\"} // x", want: map[string]any{"a": "1"}},
		{name: "block comment", in: `{/* x */"a": "1"}`, want: map[string]any{"a": "1"}},
		{name: "trailing comma in an object", in: `{"a": "1",}`, want: map[string]any{"a": "1"}},
		{name: "trailing comma after a comment and CRLF", in: "{\"a\": \"1\", // x\r\n}", want: map[string]any{"a": "1"}},
		{name: "two members", in: `{"a": "1", "b": "2"}`, want: map[string]any{"a": "1", "b": "2"}},
		{name: "slashes in a string", in: `{"a": "//x"}`, want: map[string]any{"a": "//x"}},
		{name: "an escaped quote in a string", in: `{"a": "q\" // not a comment"}`, want: map[string]any{"a": `q" // not a comment`}},
		{name: "a byte order mark", in: "\xef\xbb\xbf{\"a\": \"1\"}", want: map[string]any{"a": "1"}},
		{name: "an unterminated string", in: `{"a": "x`, invalid: true},
		{name: "an unterminated block comment", in: `{"a": "1"} /* x`, invalid: true},
		{name: "not JSON at all", in: `a = 1`, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			std, err := standardJSON([]byte(tt.in))
			if tt.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, jsonv2.Unmarshal(std, &got))
			assert.Equal(t, tt.want, got)
		})
	}
}
