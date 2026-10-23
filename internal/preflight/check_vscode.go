// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// vscodeSettings is the member of the user settings of VS Code the check
// reads. A member that is absent leaves workspace trust on, which is
// VS Code's default.
type vscodeSettings struct {
	TrustEnabled *bool `json:"security.workspace.trust.enabled"`
}

// vscodeSettingsPath is where VS Code keeps the user settings.
func (h *hostEnv) vscodeSettingsPath() string {
	if h.in.OS == "darwin" {
		return filepath.Join(h.in.Home, "Library", "Application Support", "Code", "User", "settings.json")
	}
	return filepath.Join(h.configHome(), "Code", "User", "settings.json")
}

// checkVSCodeTrust is the row vscode-trust of I23 and I14 (REQ-0380):
// security.workspace.trust.enabled set to false, or $ROMEU_ROOT, a
// parent of it or a folder under it among the trusted folders. Only VS
// Code is checked; another editor's trust model is the operator's to
// verify. A settings file that cannot be read cannot be checked, and
// fails the row.
func checkVSCodeTrust(_ context.Context, h *hostEnv) error {
	path := h.vscodeSettingsPath()
	data, present, err := readSettings(path)
	if err != nil {
		return VSCodeTrust.Wrap(err, "the VS Code settings in "+path+" cannot be read, so they cannot be checked",
			cli.Detail{Key: "file", Value: path})
	}
	if present {
		var s vscodeSettings
		if err := jsonv2.Unmarshal(stripJSONC(data), &s); err != nil {
			return VSCodeTrust.New("the VS Code settings in "+path+" do not parse, so they cannot be checked: "+describe(err),
				cli.Detail{Key: "file", Value: path})
		}
		if s.TrustEnabled != nil && !*s.TrustEnabled {
			return VSCodeTrust.New("VS Code workspace trust is turned off",
				cli.Detail{Key: "setting", Value: "security.workspace.trust.enabled=false"}, cli.Detail{Key: "file", Value: path})
		}
	}
	if h.deps.VSCodeTrustedFolders == nil {
		return nil
	}
	folders, err := h.deps.VSCodeTrustedFolders()
	if err != nil {
		return VSCodeTrust.Wrap(err, "the trusted folders of VS Code cannot be read, so they cannot be checked")
	}
	for _, f := range folders {
		if h.covers(folderPath(f)) {
			return VSCodeTrust.New(fmt.Sprintf("VS Code trusts a folder that covers $ROMEU_ROOT: %s", f),
				cli.Detail{Key: "folder", Value: f})
		}
	}
	return nil
}

// folderPath returns the path of a trusted folder given as a file:// URI,
// or the folder itself given as a path.
func folderPath(f string) string {
	if rest, ok := strings.CutPrefix(f, "file://"); ok {
		if u, err := url.Parse("file://" + rest); err == nil {
			return u.Path
		}
	}
	return f
}

// stripJSONC turns the JSON with comments and trailing commas that VS
// Code reads into JSON: line and block comments are blanked and a comma
// before a closing bracket is dropped, outside strings. It is written by
// hand because the module has no JSONC reader, and the candidate,
// github.com/tailscale/hujson, is a new dependency, which is ask-first.
func stripJSONC(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case c == '"':
			j := i + 1
			for j < len(in) && in[j] != '"' {
				if in[j] == '\\' {
					j++
				}
				j++
			}
			j = min(j, len(in)-1)
			out = append(out, in[i:j+1]...)
			i = j
		case c == '/' && i+1 < len(in) && in[i+1] == '/':
			for i < len(in) && in[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(in) && in[i+1] == '*':
			i += 2
			for i+1 < len(in) && (in[i] != '*' || in[i+1] != '/') {
				i++
			}
			i++
			out = append(out, ' ')
		case c == '}' || c == ']':
			out = dropTrailingComma(out)
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// dropTrailingComma removes a comma that ends out, white space aside.
func dropTrailingComma(out []byte) []byte {
	for i, o := range slices.Backward(out) {
		switch o {
		case ' ', '\t', '\r', '\n':
			continue
		case ',':
			return append(out[:i], out[i+1:]...)
		}
		break
	}
	return out
}
