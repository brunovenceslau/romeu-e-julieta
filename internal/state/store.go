// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package state is the host state of romeu (02 2.4, 03 3.8): the
// records under $XDG_STATE_HOME/romeu, their atomic write, the one
// lock, and the two state machines of 01 1.6 as transition tables
// (REQ-0961).
//
// The package takes the clock and the ULID source as values its callers
// inject (Seams), and every ULID it writes comes from canon.NewULID
// (10 10.4, REQ-0940), so a test fixes both.
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The modes of host state (02 2.4, 03 3.8, REQ-0239).
const (
	// DirMode is the mode of the state directory and its subdirectories.
	DirMode os.FileMode = 0o700
	// FileMode is the mode of every file of host state.
	FileMode os.FileMode = 0o600
)

// The names in the state directory (02 2.4).
const (
	lockName       = "romeu.lock"
	toolchainName  = "toolchain.json"
	projectsDir    = "projects"
	descriptorsDir = "descriptors"
)

// ErrNoStateDir is the error of an environment that names no state
// directory: neither XDG_STATE_HOME nor HOME is set to an absolute path.
var ErrNoStateDir = errors.New("state: neither XDG_STATE_HOME nor HOME names an absolute path")

// DefaultDir returns the state directory of the environment getenv
// reads: $XDG_STATE_HOME/romeu, or $HOME/.local/state/romeu (02 2.4,
// REQ-0150). A variable that is not an absolute path counts as unset,
// as the XDG specification says.
func DefaultDir(getenv func(string) string) (string, error) {
	if x := getenv("XDG_STATE_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "romeu"), nil
	}
	if h := getenv("HOME"); filepath.IsAbs(h) {
		return filepath.Join(h, ".local", "state", "romeu"), nil
	}
	return "", ErrNoStateDir
}

// Store is the state directory, open. Every path below it goes through
// an os.Root, so a name that holds a separator or a symbolic link cannot
// leave the directory.
type Store struct {
	dir         string
	root        *os.Root
	projects    *os.Root
	descriptors *os.Root
	// readOnly marks a store OpenRead opened, which takes no lock.
	readOnly bool
}

// Open makes the state directory and its two subdirectories, mode
// 0700, when they are absent, sets the mode of an existing one to 0700,
// and returns the store.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, DirMode); err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	if err := os.Chmod(dir, DirMode); err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	s := &Store{dir: dir, root: root}
	for _, sub := range []struct {
		name string
		into **os.Root
	}{{projectsDir, &s.projects}, {descriptorsDir, &s.descriptors}} {
		if err := root.MkdirAll(sub.name, DirMode); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("state: %w", err)
		}
		if err := root.Chmod(sub.name, DirMode); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("state: %w", err)
		}
		if *sub.into, err = root.OpenRoot(sub.name); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("state: %w", err)
		}
	}
	return s, nil
}

// Dir returns the path of the state directory.
func (s *Store) Dir() string { return s.dir }

// Close releases the directory handles. A lock taken from the store
// stays held until its unlock runs.
func (s *Store) Close() error {
	var errs []error
	for _, r := range []*os.Root{s.projects, s.descriptors, s.root} {
		if r != nil {
			errs = append(errs, r.Close())
		}
	}
	return errors.Join(errs...)
}
