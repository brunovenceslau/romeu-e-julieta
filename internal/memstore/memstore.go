// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package memstore holds the readers of a memory dir (08 8.1), linked
// by both binaries: the layout allowlist and its check (Check,
// CheckDir), the caps, the strict decoder of a memory entry
// (memory-entry.v1, 03 3.9) and the reader of the entries (Reader:
// List and Get). Every read goes through an os.Root on the memory dir,
// so no name in it reaches outside, and every node is judged with
// Lstat (05 5.2, I12 and I24). The writer, Store with its lock, is
// internal/memstore/write, which julieta alone links.
package memstore

import "errors"

// TopLevel is the layout allowlist of 08 8.1 (REQ-0645), its one Go
// list: the names the top level of a memory dir may hold, each a
// directory.
var TopLevel = []string{EntriesDir, HandoffDir, SnapshotDir, SalvageDir}

// The four top-level directories of a memory dir.
const (
	// EntriesDir holds the memory entries, one <ULID>.md each (03 3.9).
	EntriesDir = "entries"
	// HandoffDir holds the handoff files (03 3.10).
	HandoffDir = "handoff"
	// SnapshotDir holds the snapshot (08 8.4).
	SnapshotDir = "snapshot"
	// SalvageDir holds the salvage payloads (08 8.5).
	SalvageDir = "salvage"
)

// The caps of 08 8.1, at the starting values of 08 8.6 (REQ-0648).
const (
	// MaxFileSize is the size of one entry or handoff file, 1 MiB.
	MaxFileSize = 1 << 20
	// MaxEntries is the number of entries of one memory dir.
	MaxEntries = 10000
	// MaxHandoffFiles is the number of files in handoff/.
	MaxHandoffFiles = 1000
)

// maxNodes bounds the walk of Check: a tree with more nodes is a
// finding, so the check of a dir an agent filled stays bounded. A
// variable, so a test can lower it.
var maxNodes = 200000

// The modes julieta gives what it makes in a memory dir (REQ-0646).
const (
	// DirMode is the mode of a directory julieta makes.
	DirMode = 0o700
	// FileMode is the mode of a file julieta writes.
	FileMode = 0o600
)

// ErrInvalid is the error of a memory entry this build does not read:
// it does not decode strictly, breaks a rule of 03 3.9 or passes the
// file cap. A newer version is format-newer (RJ-311) instead.
var ErrInvalid = errors.New("memstore: not a memory entry")

// ErrLayout is the error of a memory dir whose layout a reader refuses:
// a name or a file type the allowlist of 08 8.1 does not admit, or a
// cap passed.
var ErrLayout = errors.New("memstore: the memory dir breaks its layout")

// ErrNotFound is the error of an id no entry of the memory dir has.
var ErrNotFound = errors.New("memstore: no such entry")
