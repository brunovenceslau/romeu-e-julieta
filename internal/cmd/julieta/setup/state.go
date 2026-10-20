// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	storewrite "github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/google/renameio/v2"
)

// ManifestEnv is the variable romeu passes the manifest in (03 3.6,
// REQ-0215).
const ManifestEnv = "JULIETA_MANIFEST"

// The files of julieta state below $HOME (02 2.5, REQ-0154).
const (
	// stateDir is julieta state.
	stateDir = ".local/state/julieta"
	// manifestFile is the manifest cache (03 3.6).
	manifestFile = "manifest.json"
	// warningsFile holds the warnings setup recorded for status and
	// the next SessionStart (04 4.3).
	warningsFile = "setup.json"
)

// Home returns $HOME, which julieta state, the PATH link and the
// secondary clones are below. A HOME that is not an absolute, clean
// path names no place julieta may write.
func Home() (string, error) {
	home := os.Getenv("HOME")
	if !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return "", fmt.Errorf("HOME %q is not an absolute, clean path", home)
	}
	return home, nil
}

// StateDir returns julieta state below home.
func StateDir(home string) string { return filepath.Join(home, stateDir) }

// LoadManifest returns the manifest of $JULIETA_MANIFEST, or, when the
// variable is unset or empty, the one cached in julieta state, through
// the strict decoder of internal/manifest (03 3.6). fromEnv says which.
// A manifest that is missing or that the decoder refuses stops every
// command whose Reads cell names the manifest with exit 2 (04 4.3,
// REQ-0425): a refusal that carries an id of the error table keeps it
// (format-newer, manifest-protocol), and any other is manifest-protocol
// (RJ-342), whose hint, a romeu run that passes a manifest of the
// current protocol, is the fix of both.
func LoadManifest(home string) (m manifest.Manifest, fromEnv bool, err error) {
	if v := os.Getenv(ManifestEnv); v != "" {
		m, err = manifest.DecodeEnv(v)
		return m, true, manifestError(err, "the manifest of "+ManifestEnv)
	}
	path := filepath.Join(StateDir(home), manifestFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return manifest.Manifest{}, false, manifest.ManifestProtocol.New(
			fmt.Sprintf("no manifest: %s is unset and none is cached at %s", ManifestEnv, path))
	}
	if err != nil {
		return manifest.Manifest{}, false, manifest.ManifestProtocol.Wrap(err, "the cached manifest cannot be read")
	}
	m, err = manifest.Decode(data)
	return m, false, manifestError(err, "the cached manifest "+path)
}

// manifestError returns err as LoadManifest says, what naming the
// manifest that was read.
func manifestError(err error, what string) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*cli.Error](err); ok {
		return err
	}
	return manifest.ManifestProtocol.Wrap(err, what+" is not one this julieta reads")
}

// cacheManifest writes m to the manifest cache, so a later command
// without the variable reads the manifest of the last exec.
func cacheManifest(home string, m manifest.Manifest) error {
	data, err := manifest.Encode(m)
	if err != nil {
		return err
	}
	return writeState(home, manifestFile, data)
}

// Warning is one warning of julieta: its slug, an id-less row of 04
// 4.1, Warnings, and its message.
type Warning struct {
	Message string `json:"message"`
	Slug    string `json:"slug"`
}

// The warnings setup records.
const (
	// FetchFailed is a fetch of setup that failed: the repo and the
	// git error.
	FetchFailed = "fetch-failed"
	// DefaultBranchMoved is a fast-forward of a default branch:
	// "<repo> <old>..<new>".
	DefaultBranchMoved = "default-branch-moved"
)

// State is what setup records in julieta state for status and the next
// SessionStart (04 4.3): when it last ran, the julieta that ran it, and
// the warnings to show. A fetch-failed warning holds until the next
// setup; a default-branch-moved one until the SessionStart that shows
// it clears it (08 8.2).
type State struct {
	At       time.Time `json:"at"`
	Version  string    `json:"version"`
	Warnings []Warning `json:"warnings"`
}

// ReadState returns the state setup recorded below home; none yet is
// the zero State.
func ReadState(home string) (State, error) {
	data, err := os.ReadFile(filepath.Join(StateDir(home), warningsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s, json.RejectUnknownMembers(true)); err != nil {
		return State{}, fmt.Errorf("julieta state %s: %w", warningsFile, err)
	}
	return s, nil
}

// stateLock is the lock file of setup.json in julieta state: every
// writer of it, setup's and the SessionStart hook's, reads, changes and
// writes it under this lock, so none loses the other's change.
const stateLock = "state.lock"

// stateOnWait is the test seam of the lock: called when the lock is
// held by another and the caller is about to wait (10 10.4).
var stateOnWait func()

// UpdateState reads the state setup recorded, calls f to change it and
// writes it back, all under the lock of the file (03 3.8), so a change
// another process makes meanwhile is not lost. A state that cannot be
// read is an error and the file is left as it is; so is an error of f,
// which writes nothing.
func UpdateState(home string, f func(*State) error) error {
	_, err := updateState(home, false, f)
	return err
}

// updateState is UpdateState; with replace set, a state that cannot be
// read is changed from the zero State and replaced, and the read error
// is returned beside the nil error of the write (setup does so for its
// own record).
func updateState(home string, replace bool, f func(*State) error) (readErr, err error) {
	unlock, err := storewrite.Lock(filepath.Join(StateDir(home), stateLock), stateOnWait)
	if err != nil {
		return nil, err
	}
	defer unlock()
	s, err := ReadState(home)
	if err != nil {
		if !replace {
			return nil, err
		}
		readErr, s = err, State{}
	}
	if err := f(&s); err != nil {
		return readErr, err
	}
	data, err := json.Marshal(s, json.Deterministic(true))
	if err != nil {
		return readErr, err
	}
	return readErr, writeState(home, warningsFile, data)
}

// recordState writes the state of a setup that ran at at: its warnings
// and the default-branch-moved warnings that no SessionStart has
// cleared, each once, read under the lock so a clear that happened
// while setup ran is not undone. It returns the error of a state that
// could not be read, which this one replaces.
func recordState(home string, at time.Time, version string, warnings []Warning) (readErr, err error) {
	return updateState(home, true, func(prev *State) error {
		s := State{At: at.UTC(), Version: version}
		for _, w := range prev.Warnings {
			if w.Slug == DefaultBranchMoved {
				s.Warnings = append(s.Warnings, w)
			}
		}
		for _, w := range warnings {
			if !slices.Contains(s.Warnings, w) {
				s.Warnings = append(s.Warnings, w)
			}
		}
		*prev = s
		return nil
	})
}

// writeState replaces the file name of julieta state with data, by a
// rename, so a reader sees the old file or the new one (03 3.8). The
// directory is the user's alone.
func writeState(home, name string, data []byte) error {
	dir := StateDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	t, err := renameio.NewPendingFile(filepath.Join(dir, name),
		renameio.WithTempDir(dir), renameio.WithStaticPermissions(0o600))
	if err != nil {
		return err
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(data); err != nil {
		return err
	}
	return t.CloseAtomicallyReplace()
}
