// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"syscall"
)

// readRegular reads the file name below root as a reader that takes no
// lock must: O_NONBLOCK keeps the open of a FIFO from waiting for a
// writer, anything but a regular file is refused, and with limit > 0 at
// most limit+1 bytes are read, so a caller sees a file over its cap
// without reading it whole. A file that is absent is an error that wraps
// fs.ErrNotExist.
func readRegular(root *os.Root, name string, limit int64) ([]byte, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("render: %s is %s, not a regular file", name, info.Mode().Type())
	}
	var r io.Reader = f
	if limit > 0 {
		r = io.LimitReader(f, limit+1)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return data, nil
}

// readLive reads the live render.json below root with readRegular: the
// record, and false when there is none.
func readLive(root *os.Root) (Record, bool, error) {
	data, err := readRegular(root, LiveRecord, MaxRecordSize)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	rec, err := DecodeRecord(data)
	if err != nil {
		return Record{}, false, err
	}
	return rec, true, nil
}

// ReadLive reads the live render.json of the env dir envDir for a
// reader that takes no lock, romeu status (04 4.2): the record, and
// false when there is none, before the first promotion or for an env dir
// that does not exist. A record that does not decode is an error that
// wraps ErrRecordInvalid; a render.json that is not a regular file is
// refused without waiting on it.
func ReadLive(envDir string) (Record, bool, error) {
	root, err := os.OpenRoot(envDir)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	return readLive(root)
}

// Drift returns the live paths of envDir that differ from rec, sorted:
// each file and kit dir of rec.Files whose digest differs or that is
// missing (01 1.2, REQ-0047; I15). With rec nil, before the first
// promotion, a live sbxenv.yaml is the drift, since no digest says what
// it should hold. An env dir that does not exist has none. It reads the
// record the caller read, so a reader that takes no lock reads
// render.json once; the caller handles a record with promoting, which
// is never drift.
func Drift(envDir string, rec *Record) ([]string, error) {
	root, err := os.OpenRoot(envDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	return drift(root, rec)
}

// drift is Drift on the env dir open as root.
func drift(root *os.Root, rec *Record) ([]string, error) {
	if rec == nil {
		ok, err := liveExists(root, LiveEnvFile)
		if err != nil || !ok {
			return nil, err
		}
		return []string{LiveEnvFile}, nil
	}
	var drifted []string
	for _, p := range slices.Sorted(maps.Keys(rec.Files)) {
		ok, err := liveMatches(root, p, rec.Files[p])
		if err != nil {
			return nil, err
		}
		if !ok {
			drifted = append(drifted, p)
		}
	}
	return drifted, nil
}
