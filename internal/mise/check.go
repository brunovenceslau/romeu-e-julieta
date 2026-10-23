// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"errors"
	"regexp"
	"slices"
)

// The slugs of what Check finds: the findings of julieta lock --check
// (exit 6, check-findings, RJ-328).
const (
	// ToolMissing is a tool of mise.toml with no entry in the lock.
	ToolMissing = "lock-tool-missing"
	// PlatformMissing is a lockable entry that lacks a linux platform.
	PlatformMissing = "lock-platform-missing"
	// NoChecksum is a lockable entry with a platform that holds no
	// checksum, and the tool is not in the exceptions file (07 7.3,
	// REQ-0407 as decided on 2026-10-22).
	NoChecksum = "lock-no-checksum"
	// BadChecksum is a platform whose checksum is not "sha256:" and 64
	// lowercase hexadecimal digits.
	BadChecksum = "lock-bad-checksum"
	// Unparsed is a file of the repo that does not parse: the Tool is its
	// name and the Detail the decoder's text (REQ-0406: the lock parses
	// with the pinned layout).
	Unparsed = "lock-unparsed"
)

// Finding is one thing Check found: its slug, the tool of mise.toml (or
// the file, for Unparsed) and, for a platform, the platform.
type Finding struct {
	Slug     string
	Tool     string
	Platform string
	Detail   string
}

// String is the finding as one line.
func (f Finding) String() string {
	s := f.Slug + " " + f.Tool
	if f.Platform != "" {
		s += " " + f.Platform
	}
	if f.Detail != "" {
		s += ": " + f.Detail
	}
	return s
}

// Report is what Check found: the findings that fail julieta lock
// --check, in the order of the tools of mise.toml.
type Report struct {
	Findings []Finding
}

// requiredPlatforms are the platforms --check requires of a lockable
// entry: the linux ones only (07 7.3, REQ-0613).
var requiredPlatforms = []string{LinuxX64, LinuxArm64}

// checksumForm is the one form of a checksum: sha256: and 64 lowercase
// hexadecimal digits.
var checksumForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Check judges the freshness of lock against cfg (07 7.3, REQ-0619):
// every tool of mise.toml has an entry, and every lockable entry, one
// whose tool is not version-only by its key (VersionOnly), covers
// linux-x64 and linux-arm64 and holds a well-formed checksum on each
// platform it lists, unless the exceptions excuse its tool from the
// missing one (REQ-0407). It reads nothing but its arguments. The
// catalog lookup of lock-unknown-tool is not here.
func Check(cfg Config, lock Lock, ex Exceptions) Report {
	var rep Report
	add := func(f Finding) { rep.Findings = append(rep.Findings, f) }
	for _, tool := range cfg.Tools {
		entries := lock.Tools[tool]
		if len(entries) == 0 {
			add(Finding{Slug: ToolMissing, Tool: tool})
			continue
		}
		if VersionOnly(tool) {
			continue
		}
		_, excused := ex[tool]
		for _, e := range entries {
			for _, p := range requiredPlatforms {
				if _, ok := e.Platforms[p]; !ok {
					add(Finding{Slug: PlatformMissing, Tool: tool, Platform: p})
				}
			}
			for _, p := range e.PlatformNames() {
				switch sum := e.Platforms[p].Checksum; {
				case sum == "" && !excused:
					add(Finding{Slug: NoChecksum, Tool: tool, Platform: p})
				case sum != "" && !checksumForm.MatchString(sum):
					add(Finding{Slug: BadChecksum, Tool: tool, Platform: p})
				}
			}
		}
	}
	return rep
}

// Check parses the raw files of a repo and judges them. A file that does
// not parse is a finding (Unparsed), never an error, so the check exits
// 6 on it (REQ-0406); a repo with no mise.toml has nothing to check.
func (r Raw) Check() Report {
	if !r.HasConfig {
		return Report{}
	}
	unparsed := func(file string, err error) Report {
		return Report{Findings: []Finding{{Slug: Unparsed, Tool: file, Detail: err.Error()}}}
	}
	cfg, err := ParseConfig(r.Config)
	if err != nil {
		return unparsed(ConfigFile, err)
	}
	var lock Lock
	if r.HasLock {
		if lock, err = ParseLock(r.Lock); err != nil {
			return unparsed(LockFile, err)
		}
	}
	var ex Exceptions
	if r.HasExceptions {
		if ex, err = ParseExceptions(r.Exceptions); err != nil {
			return unparsed(ExceptionsFile, err)
		}
	}
	return Check(cfg, lock, ex)
}

// Parse parses the raw files of a repo for install: the mise.toml, the
// mise.lock when there is one, and the exceptions file. A file that does
// not parse is an error that wraps ErrUnparsed and names the file.
func (r Raw) Parse() (Files, error) {
	var f Files
	var err error
	if r.HasConfig {
		if f.Config, err = ParseConfig(r.Config); err != nil {
			return Files{}, errors.Join(ErrUnparsed, err)
		}
		f.HasConfig = true
	}
	if r.HasLock {
		if f.Lock, err = ParseLock(r.Lock); err != nil {
			return Files{}, errors.Join(ErrUnparsed, err)
		}
		f.HasLock, f.LockData = true, r.Lock
	}
	if r.HasExceptions {
		if f.Exceptions, err = ParseExceptions(r.Exceptions); err != nil {
			return Files{}, errors.Join(ErrUnparsed, err)
		}
	}
	return f, nil
}

// Findings judges the files already parsed, as Check does the raw ones.
func (f Files) Findings() []Finding {
	if !f.HasConfig {
		return nil
	}
	return Check(f.Config, f.Lock, f.Exceptions).Findings
}

// VersionOnlyTools returns the tools of cfg that are version-only by
// their key, once each, in the order of cfg (07 7.3, REQ-0617).
func VersionOnlyTools(cfg Config) []string {
	return slices.DeleteFunc(slices.Clone(cfg.Tools), func(k string) bool { return !VersionOnly(k) })
}
