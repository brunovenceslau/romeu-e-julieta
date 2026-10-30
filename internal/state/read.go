// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
		return nil, fmt.Errorf("state: opening %s: %w", dir, err)
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
			return nil, fmt.Errorf("state: opening %s: %w", filepath.Join(dir, sub.name), err)
		}
		*sub.into = r
	}
	return s, nil
}

// ProjectNames returns the name of each project record under projects/,
// in name order: each entry <name>.json whose name passes the rule of
// 03 3.1, whatever its type, so a record damaged into a directory, a
// pipe or a link keeps its name and LoadProject reports it as
// record-unreadable (REQ-0329). A dotfile, such as the temporary file
// an interrupted atomic write leaves (.shop.json123), is left out (T048,
// decided on 2026-10-22), and so is a name romeu never writes, one the
// rule of 03 3.1 refuses (Shop.json): no project has it, so no block
// can hold it.
func (s *Store) ProjectNames() ([]string, error) {
	if s.projects == nil {
		return nil, nil
	}
	dir := filepath.Join(s.dir, projectsDir)
	f, err := s.projects.Open(".")
	if err != nil {
		return nil, fmt.Errorf("state: listing %s: %w", dir, err)
	}
	defer func() { _ = f.Close() }()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("state: listing %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if ok && checkName(name) == nil {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}

// CheckName returns an error that wraps ErrInvalidName unless name is a
// project name by the rule of 03 3.1, the rule a record's file name
// keeps, so a caller can refuse a name before it reads.
func CheckName(name string) error { return checkName(name) }
