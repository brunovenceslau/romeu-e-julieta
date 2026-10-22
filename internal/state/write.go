// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"fmt"
	"os"

	"github.com/google/renameio/v2"
)

// writeFile replaces the file name below root with data by the atomic
// write rule of 03 3.8 (REQ-0239): a temporary file in the same
// directory, mode 0600, fsync of the file, rename, fsync of the
// directory. A reader sees the old file or the new one, never a part.
// github.com/google/renameio/v2 does the first four steps, and the
// repository already uses it for the same job; it does not sync the
// directory, so that step is here (ADR 0038). root is the directory
// itself, so the temporary file lands beside name.
func writeFile(root *os.Root, name string, data []byte) error {
	t, err := renameio.NewPendingFile(name, renameio.WithRoot(root), renameio.WithStaticPermissions(FileMode))
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(data); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	if err := t.CloseAtomicallyReplace(); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	return syncDir(root)
}

// syncDir makes the rename durable: fsync of the directory.
func syncDir(root *os.Root) error {
	d, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("state: %w", err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("state: sync the directory: %w", err)
	}
	return nil
}
