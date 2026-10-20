// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/google/renameio/v2"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
)

// The two hooks julieta wires (08 8.2), named as the failure line
// and the state name them.
const (
	// HookSessionStart is `julieta handoff show --hook`.
	HookSessionStart = hooks.SessionStart
	// HookSessionEnd is `julieta handoff write --facts`.
	HookSessionEnd = hooks.SessionEnd
)

// hooksFile is the record of the hooks in julieta state
// ($HOME/.local/state/julieta/, 02 2.5, REQ-0154), and hooksLock the
// lock that serializes its updates and those of setup.json.
const (
	hooksFile = "hooks.json"
	hooksLock = "hooks.lock"
)

// Hooks is the record of the hooks in julieta state: the time of the
// last SessionStart run (`handoff show --hook`) and of the last
// SessionEnd run (`handoff write --facts`), which 08 8.5 step 1 reads.
// The failures of the hooks are not here: they are recorded in
// internal/hooks's store, hook-failures/, with the git hooks' (08 8.2).
type Hooks struct {
	SessionEndAt   time.Time `json:"sessionEndAt"`
	SessionStartAt time.Time `json:"sessionStartAt"`
}

// ReadHooks returns the record of the hooks below home; none yet is the
// zero Hooks.
func ReadHooks(home string) (Hooks, error) {
	data, err := os.ReadFile(filepath.Join(setup.StateDir(home), hooksFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Hooks{}, nil
	}
	if err != nil {
		return Hooks{}, err
	}
	var h Hooks
	if err := json.Unmarshal(data, &h, json.RejectUnknownMembers(true)); err != nil {
		return Hooks{}, fmt.Errorf("julieta state %s: %w", hooksFile, err)
	}
	return h, nil
}

// writeState replaces the file name of julieta state with data by a
// rename (03 3.8), 0600.
func writeState(home, name string, v any) error {
	data, err := json.Marshal(v, json.Deterministic(true))
	if err != nil {
		return err
	}
	dir := setup.StateDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	t, err := renameio.NewPendingFile(filepath.Join(dir, name), renameio.WithTempDir(dir), renameio.WithStaticPermissions(0o600))
	if err != nil {
		return err
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(data); err != nil {
		return err
	}
	return t.CloseAtomicallyReplace()
}

// updateHooks runs f on the record of the hooks under its lock and
// writes it back when it changed. A record that does not read is
// replaced by the new one (the read error is returned with the write's).
// setup.json is not touched here: its one writer protocol is
// setup.UpdateState.
func updateHooks(home string, f func(h *Hooks)) error {
	unlock, err := lockFile(filepath.Join(setup.StateDir(home), hooksLock))
	if err != nil {
		return err
	}
	defer unlock()
	h, herr := ReadHooks(home)
	old := h
	f(&h)
	if herr == nil && sameHooks(old, h) {
		return nil
	}
	return errors.Join(writeState(home, hooksFile, h), herr)
}

// sameHooks reports whether a and b are the same record.
func sameHooks(a, b Hooks) bool {
	return a.SessionEndAt.Equal(b.SessionEndAt) && a.SessionStartAt.Equal(b.SessionStartAt)
}

// failuresDir is the store of hook failures below julieta state, the
// one internal/hooks records the git hooks' in.
func failuresDir(home string) string { return filepath.Join(setup.StateDir(home), "hook-failures") }

// failureLine words a recorded failure on one line: of a session hook
// with its message, of a git hook with the stage and the script.
func failureLine(f hooks.Failure) string {
	where := esc(f.Hook)
	if f.Repo != "" {
		where += " repo " + esc(f.Repo)
	}
	what := f.Message
	if what == "" {
		what = "stage " + f.Stage + " " + f.Path
	}
	return fmt.Sprintf("hook failure: %s at %s, exit %d: %s", where, f.At.UTC().Format(time.RFC3339), f.ExitCode, oneLine(what))
}

// oneLine escapes s and keeps it on one line, cut at 300 bytes.
func oneLine(s string) string {
	const max = 300
	if len(s) > max {
		s = s[:max] + "..."
	}
	out := []rune(esc(s))
	for i, r := range out {
		if r == '\n' || r == '\r' {
			out[i] = ' '
		}
	}
	return string(out)
}
