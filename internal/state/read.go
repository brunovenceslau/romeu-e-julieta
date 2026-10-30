// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// ErrReadOnly is the error of a lock asked of a store opened by
// OpenRead: a reader takes no lock and writes nothing.
var ErrReadOnly = errors.New("state: the store is open to read; it takes no lock")

// OpenRead opens the state directory to read it, for romeu status, which
// is read only (04 4.2): it makes no directory, changes no mode and
// takes no lock, so what it reports is what it found, a mode doctor
// reports included. A state directory that is absent is an error that
// wraps fs.ErrNotExist; a subdirectory that is absent holds no record,
// so a load from it wraps fs.ErrNotExist and ProjectNames is empty.
func OpenRead(dir string) (*Store, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	s := &Store{dir: dir, root: root, readOnly: true}
	for _, sub := range []struct {
		name string
		into **os.Root
	}{{projectsDir, &s.projects}, {descriptorsDir, &s.descriptors}} {
		r, err := root.OpenRoot(sub.name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("state: %w", err)
		}
		*sub.into = r
	}
	return s, nil
}

// ProjectNames returns the name of each project record under projects/,
// in name order: each regular file <name>.json whose name passes the
// rule of 03 3.1. A dotfile, such as the temporary file an interrupted
// atomic write leaves (.shop.json123), any other name and anything but a
// regular file are left out (T048, decided on 2026-10-22).
func (s *Store) ProjectNames() ([]string, error) {
	if s.projects == nil {
		return nil, nil
	}
	f, err := s.projects.Open(".")
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	var names []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if ok && e.Type().IsRegular() && checkName(name) == nil {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}
