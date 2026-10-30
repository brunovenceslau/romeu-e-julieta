// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ReadLive reads the live render.json of the env dir envDir for a
// reader that takes no lock, romeu status (04 4.2): the record, and
// false when there is none, before the first promotion or for an env dir
// that does not exist. A record that does not decode is an error that
// wraps ErrRecordInvalid. It reads as CheckDrift and Interrupted read,
// through an os.Root of the env dir.
func ReadLive(envDir string) (Record, bool, error) {
	root, err := os.OpenRoot(envDir)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	return readLiveRecord(root)
}
