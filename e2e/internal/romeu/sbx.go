// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
)

// writeFakeSbxConfig writes the configuration the fake sbx reads from
// the file beside it (e2e/fakesbx/config.go): an empty session, which
// replays no call, and a state file in dir. A test that needs sbx to
// answer replaces the session with SetSbxSession.
func writeFakeSbxConfig(sbx, dir string) error {
	session := filepath.Join(dir, "sbx-session.jsonl")
	if err := os.WriteFile(session, nil, 0o600); err != nil {
		return err
	}
	return writeSbxConfig(sbx, session, filepath.Join(dir, "sbx-state.json"))
}

// SetSbxSession points the fake sbx of the Env at the session file
// (an absolute path, a recording under e2e/testdata/sbx/).
func (e *Env) SetSbxSession(session string) error {
	return writeSbxConfig(e.Sbx, session, filepath.Join(filepath.Dir(e.Bin), "sbx-state.json"))
}

// writeSbxConfig writes <sbx>.json.
func writeSbxConfig(sbx, session, state string) error {
	b, err := json.Marshal(map[string]string{"session": session, "state": state})
	if err != nil {
		return err
	}
	return os.WriteFile(sbx+".json", b, 0o600)
}
