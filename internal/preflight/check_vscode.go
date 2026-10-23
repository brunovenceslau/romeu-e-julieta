// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

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
		std, err := standardJSON(data)
		if err == nil {
			err = jsonv2.Unmarshal(std, &s)
		}
		if err != nil {
			return VSCodeTrust.New("the VS Code settings in "+path+" do not parse, so they cannot be checked: "+describe(err),
				cli.Detail{Key: "file", Value: path})
		}
		if s.TrustEnabled != nil && !*s.TrustEnabled {
			return VSCodeTrust.New("VS Code workspace trust is turned off",
				cli.Detail{Key: "setting", Value: "security.workspace.trust.enabled=false"}, cli.Detail{Key: "file", Value: path})
		}
	}
	if h.deps.VSCodeTrustedFolders == nil {
		*h.unchecked = append(*h.unchecked, UncheckedVSCodeFolders)
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

// standardJSON turns the JSON with comments and trailing commas that VS
// Code reads (JSONC) into standard JSON, through
// github.com/tailscale/hujson (ADR 0038). A leading UTF-8 byte order
// mark, which an editor may write, is dropped first, and a newline is
// added at the end, because hujson refuses a line comment that ends the
// file without one and VS Code reads it. A text hujson cannot read is an
// error, so the row fails rather than guess.
func standardJSON(in []byte) ([]byte, error) {
	in = bytes.TrimPrefix(in, []byte("\xef\xbb\xbf"))
	return hujson.Standardize(append(slices.Clip(in), '\n'))
}
