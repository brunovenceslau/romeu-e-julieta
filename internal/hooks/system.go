// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/renameio/v2"
)

// SystemFile is the file of julieta state (02 2.5, REQ-0154) that holds
// the system core.hooksPath julieta setup read before it pointed the
// global key at the dispatcher: the hooks directory of the sandbox kit,
// which git no longer runs once the global key is set, and which the
// dispatcher runs as its system stage (04 4.3, REQ-0408). The
// dispatcher starts no git, so it reads this record instead of the
// system config.
const SystemFile = "system-hooks-path.json"

// systemRecord is the content of SystemFile.
type systemRecord struct {
	Path string `json:"path"`
}

// RecordSystem records path, the system core.hooksPath as git config
// returned it, in the julieta state directory dir, by a rename, so a
// reader sees the old record or the new one. An empty path, a system
// config that sets none, removes the record.
func RecordSystem(dir, path string) error {
	file := filepath.Join(dir, SystemFile)
	if path == "" {
		if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(systemRecord{Path: path}, json.Deterministic(true))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return renameio.WriteFile(file, append(data, '\n'), 0o600, renameio.WithTempDir(dir))
}

// ReadSystem returns the system hooks path recorded in the julieta
// state directory dir, or "" when none is recorded. A record that is
// not the format is an error.
func ReadSystem(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, SystemFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var rec systemRecord
	if err := json.Unmarshal(data, &rec, json.RejectUnknownMembers(true)); err != nil {
		return "", err
	}
	return rec.Path, nil
}

// systemHook returns the system stage's hook of the chain of hook, or
// "" when the stage runs nothing: no recorded path, a recorded path
// that is not absolute, which names no one directory, or one that is
// julieta's own dispatcher, by its path or through a link, so the chain
// never runs itself.
func systemHook(system, dispatcher, hook string) string {
	if system == "" || !filepath.IsAbs(system) {
		return ""
	}
	if sameDir(system, dispatcher) {
		return ""
	}
	return filepath.Join(system, hook)
}

// sameDir reports whether a and b name one directory: equal once
// cleaned, or once their links are resolved.
func sameDir(a, b string) bool {
	if b == "" {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// SystemMarker is the variable the system stage runs with. A dispatcher
// started with it set, by a system hook that leads back to julieta
// hooks run, skips the system stage, so the chain cannot loop.
const SystemMarker = "JULIETA_HOOKS_SYSTEM=1"
