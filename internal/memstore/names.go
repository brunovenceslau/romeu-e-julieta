// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore

import (
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// TempPrefix starts the name of julieta's temporary files (REQ-0654):
// .tmp-<ULID>, in the target's own directory.
const TempPrefix = ".tmp-"

// entrySuffix ends the name of an entry file, <ULID>.md.
const entrySuffix = ".md"

// TempName returns the name of the temporary file of the id u.
func TempName(u canon.ULID) string { return TempPrefix + u.String() }

// TempTime returns the time of the ULID of a temporary file's name,
// and false for a name that is not of the grammar .tmp-<ULID>. A name
// of that grammar is never read and is no allowlist violation
// (REQ-0654).
func TempTime(name string) (time.Time, bool) {
	rest, ok := strings.CutPrefix(name, TempPrefix)
	if !ok {
		return time.Time{}, false
	}
	u, err := canon.ParseULID(rest)
	if err != nil {
		return time.Time{}, false
	}
	return ulidTime(u), true
}

// IsTemp reports whether name is of the grammar .tmp-<ULID>.
func IsTemp(name string) bool {
	_, ok := TempTime(name)
	return ok
}

// ulidTime returns the millisecond of u, read by oklog/ulid, the
// library canon writes its ULIDs with.
func ulidTime(u canon.ULID) time.Time { return ulid.Time(ulid.ULID(u).Time()).UTC() }

// EntryName returns the name of the file of the entry u in entries/.
func EntryName(u canon.ULID) string { return u.String() + entrySuffix }

// EntryPath returns the slash path of the file of the entry u, from the
// memory dir.
func EntryPath(u canon.ULID) string { return EntriesDir + "/" + EntryName(u) }

// entryID returns the id of the entry file name, and false for a name
// that is not <ULID>.md with a lowercase ULID (03 3.12, REQ-0258).
func entryID(name string) (canon.ULID, bool) {
	rest, ok := strings.CutSuffix(name, entrySuffix)
	if !ok {
		return canon.ULID{}, false
	}
	u, err := canon.ParseULID(rest)
	return u, err == nil
}

// validName reports whether name is one element of a path: not empty,
// not "." or "..", and without a slash or a NUL.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\x00")
}
