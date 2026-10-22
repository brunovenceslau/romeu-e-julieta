// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package mise is the mise driver of julieta (04 4.3, 07 7.3): the
// readers of mise.lock and mise.toml, the freshness check of julieta
// lock --check, the environment of every mise call, and the runs of mise
// install and mise lock through internal/procgroup. Both files are
// TOML, read only, by github.com/BurntSushi/toml (12 12.1, REQ-1045).
package mise

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// MaxFileSize is the cap of a mise.toml or a mise.lock this package
// reads, 1 MiB: the file cap every repository blob romeu decodes
// shares (07 7.5, step 1).
const MaxFileSize = 1 << 20

// LockfileVersion is the lockfile_version the pinned mise (2026.10.3)
// writes, the one layout this package reads; another is refused, so a
// layout it does not know is never read in part (R6 of 2026-10-22).
const LockfileVersion = 3

// The two linux platforms a sandbox runs (07 7.3, REQ-0613).
const (
	LinuxX64   = "linux-x64"
	LinuxArm64 = "linux-arm64"
)

// Lock is a mise.lock as the pinned mise writes it (lockfile_version 3,
// mise 2026.10.3): the tools by the key of mise.toml, each an array of
// entries, one for each version locked.
type Lock struct {
	// Version is lockfile_version.
	Version int
	// Tools are the entries of each tool, by the name mise.toml gives
	// it ("go", "aqua:cli/cli", "go:golang.org/x/tools/gopls").
	Tools map[string][]Entry
}

// Entry is one locked version of a tool.
type Entry struct {
	// Backend is the backend:tool of the entry ("core:go",
	// "aqua:cli/cli").
	Backend string
	// Version is the version locked.
	Version string
	// Specifiers are the requests the version answers.
	Specifiers []string
	// Platforms are the platforms the entry covers, by mise's name for
	// them ("linux-x64"). A version-only backend records none.
	Platforms map[string]Platform
}

// Platform is what the lock records of one platform of an entry.
type Platform struct {
	// Checksum is "sha256:<hex>", empty when the lock holds none.
	Checksum string
	// URL is the artifact's address, empty when the lock holds none.
	URL string
}

// PlatformNames returns the platforms of the entry, sorted.
func (e Entry) PlatformNames() []string {
	return slices.Sorted(maps.Keys(e.Platforms))
}

// platformPrefix starts the key of a platform's table inside an entry:
// the lock writes [tools.gh."platforms.linux-x64"], one quoted key.
const platformPrefix = "platforms."

// rawLock is the document as the decoder reads it. The entries stay
// untyped, since a newer mise may add a member to one, and every
// member the code reads is checked for its type below.
type rawLock struct {
	LockfileVersion int                         `toml:"lockfile_version"`
	Tools           map[string][]map[string]any `toml:"tools"`
}

// ParseLock reads a mise.lock. The file is TOML, so a duplicate key is
// an error, which the decoder reports and this function returns as it
// is. A top-level key the pinned layout does not have, a lockfile_version
// other than LockfileVersion, a member of the wrong type, nesting past
// MaxDepth and a file over MaxFileSize are each an error as well: never
// a partial read (07 7.5, step 1, REQ-0629).
func ParseLock(data []byte) (Lock, error) {
	if len(data) > MaxFileSize {
		return Lock{}, fmt.Errorf("mise.lock: %d bytes pass the cap of %d", len(data), MaxFileSize)
	}
	if err := CheckDepth(data); err != nil {
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
	if raw.LockfileVersion != LockfileVersion {
		return Lock{}, fmt.Errorf("mise.lock: lockfile_version %d is not %d, the layout of the pinned mise", raw.LockfileVersion, LockfileVersion)
	}
	lock := Lock{Version: raw.LockfileVersion, Tools: make(map[string][]Entry, len(raw.Tools))}
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
	if v, ok := m["specifiers"]; ok {
		list, ok := v.([]any)
		if !ok {
			return Entry{}, errors.New("specifiers is not a list")
		}
		for _, s := range list {
			str, ok := s.(string)
			if !ok {
				return Entry{}, errors.New("a specifier is not a string")
			}
			e.Specifiers = append(e.Specifiers, str)
		}
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
		var p Platform
		if p.Checksum, err = stringOf(table, "checksum"); err != nil {
			return Entry{}, fmt.Errorf("the platform %q: %w", name, err)
		}
		if p.URL, err = stringOf(table, "url"); err != nil {
			return Entry{}, fmt.Errorf("the platform %q: %w", name, err)
		}
		if e.Platforms == nil {
			e.Platforms = map[string]Platform{}
		}
		e.Platforms[name] = p
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
