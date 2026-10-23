// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// MaxFileSize is the cap of a mise file this package reads, 1 MiB: the
// file cap every repository blob romeu decodes shares (07 7.5, step 1).
const MaxFileSize = 1 << 20

// lockfileVersion is the lockfile_version the pinned mise (2026.10.3)
// writes, the one layout this reader knows; another is refused, so a
// layout it does not know is never read in part.
const lockfileVersion = 3

// Lock is what egress reads of a mise.lock as the pinned mise writes it
// (lockfile_version 3): the tools by the key of mise.toml, each an array
// of entries, one for each version locked.
type Lock struct {
	Tools map[string][]Entry
}

// Entry is one locked version of a tool.
type Entry struct {
	// Backend is the backend:tool of the entry ("core:go",
	// "aqua:cli/cli"), the key of the catalog (07 7.5, step 2).
	Backend string
	// Version is the version locked.
	Version string
	// URLs are the artifact URLs the lock records, by platform. A
	// version-only backend records none.
	URLs map[string]string
}

// PlatformNames returns the platforms that have a URL, sorted.
func (e Entry) PlatformNames() []string {
	return slices.Sorted(maps.Keys(e.URLs))
}

// platformPrefix starts the key of a platform's table inside an entry:
// the lock writes [tools.gh."platforms.linux-x64"], one quoted key.
const platformPrefix = "platforms."

// rawLock is the document as the decoder reads it. The entries stay
// untyped, since a newer mise may add a member to one, and every member
// the code reads is checked for its type below.
type rawLock struct {
	LockfileVersion int                         `toml:"lockfile_version"`
	Tools           map[string][]map[string]any `toml:"tools"`
}

// ParseLock reads a mise.lock. The file is TOML, so a duplicate key or
// table is an error, which the decoder reports and this function returns
// as it is. A top-level key the pinned layout does not have, a
// lockfile_version other than the pinned one, a member of the wrong type,
// an entry without a backend, nesting past maxDepth, a key of more than
// maxKeyParts parts and a file over MaxFileSize are each an error as well:
// never a partial read (07 7.5, step 1, REQ-0629). The reader keeps what
// egress needs, the backend, the version and the URL of each platform, and
// leaves the checksum to julieta lock --check.
func ParseLock(data []byte) (Lock, error) {
	if len(data) > MaxFileSize {
		return Lock{}, fmt.Errorf("mise.lock: %d bytes pass the cap of %d", len(data), MaxFileSize)
	}
	if err := checkShape(data); err != nil {
		return Lock{}, fmt.Errorf("mise.lock: %w", err)
	}
	var raw rawLock
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&raw)
	if err != nil {
		return Lock{}, fmt.Errorf("mise.lock: %w", err)
	}
	// Undecoded also lists what the untyped entries hold, so only the
	// first part of each key says whether a top-level key is unknown.
	for _, key := range md.Undecoded() {
		if key[0] != "lockfile_version" && key[0] != "tools" {
			return Lock{}, fmt.Errorf("mise.lock: the top-level key %q is not one of the layout of the pinned mise", key[0])
		}
	}
	if raw.LockfileVersion != lockfileVersion {
		return Lock{}, fmt.Errorf("mise.lock: lockfile_version %d is not %d, the layout of the pinned mise", raw.LockfileVersion, lockfileVersion)
	}
	lock := Lock{Tools: make(map[string][]Entry, len(raw.Tools))}
	for name, entries := range raw.Tools {
		for _, m := range entries {
			e, err := entryOf(m)
			if err != nil {
				return Lock{}, fmt.Errorf("mise.lock: tool %q: %w", name, err)
			}
			lock.Tools[name] = append(lock.Tools[name], e)
		}
	}
	return lock, nil
}

// entryOf reads one entry of the lock.
func entryOf(m map[string]any) (Entry, error) {
	var e Entry
	var err error
	if e.Version, err = stringOf(m, "version"); err != nil {
		return Entry{}, err
	}
	if e.Backend, err = stringOf(m, "backend"); err != nil {
		return Entry{}, err
	}
	if e.Backend == "" {
		return Entry{}, errors.New("an entry has no backend")
	}
	for key, v := range m {
		name, ok := strings.CutPrefix(key, platformPrefix)
		if !ok {
			continue
		}
		table, ok := v.(map[string]any)
		if !ok {
			return Entry{}, fmt.Errorf("the platform %q is not a table", name)
		}
		url, err := stringOf(table, "url")
		if err != nil {
			return Entry{}, fmt.Errorf("the platform %q: %w", name, err)
		}
		if url == "" {
			continue
		}
		if e.URLs == nil {
			e.URLs = map[string]string{}
		}
		e.URLs[name] = url
	}
	return e, nil
}

// stringOf returns the string member key of m, empty when it is absent.
func stringOf(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s is not a string", key)
	}
	return s, nil
}
