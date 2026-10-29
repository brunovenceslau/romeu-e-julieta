// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"errors"
	"net/url"
	"strings"
)

// Memory names the durable copy of a project's memory (03 3.2, 08 8.1,
// REQ-1303): the private git repository romeu commits <name>-env/memory/
// to and pushes, or MemoryNone.
type Memory struct {
	// Remote is the URL of the repository, one gitsafe fetches from and
	// pushes to (https://<host>/..., or git://127.0.0.1 for the tests'
	// daemon), or MemoryNone, which keeps memory on this machine only.
	Remote string `json:"remote" yaml:"remote"`
}

// MemoryNone is the value of memory.remote that opts out of the durable
// copy (REQ-1305): sync, run and doctor then warn memory-local.
const MemoryNone = "none"

// CheckMemoryRemote returns the error memory-remote-missing (RJ-357)
// for a project spec without memory.remote, and nil for one that names
// a remote or MemoryNone (08 8.1, REQ-1305). It is not a rule of the
// general validation, so a spec without the member still decodes and
// validates: sync calls it on the specs it is about to promote.
func CheckMemoryRemote(p *Project) error {
	if p.Memory == nil || p.Memory.Remote == "" {
		return MemoryRemoteMissing.New("the project spec " + p.Name + " names no memory.remote")
	}
	return nil
}

// LocalOnly reports whether the spec keeps memory on this machine only:
// memory.remote is MemoryNone.
func (p *Project) LocalOnly() bool {
	return p.Memory != nil && p.Memory.Remote == MemoryNone
}

// memoryRemote holds a memory.remote that is present: MemoryNone, or a
// URL of the shape gitsafe.CheckURL accepts, which a test of gitsafe
// holds equal on one corpus. internal/spec does not import gitsafe,
// which builds for unix only.
func memoryRemote(v string) error {
	if v == MemoryNone {
		return nil
	}
	if strings.ContainsFunc(v, func(r rune) bool { return r < 0x20 || r == 0x7f || r == ' ' }) {
		return errors.New("holds a space or a control character")
	}
	scheme, _, ok := strings.Cut(v, "://")
	if !ok || (scheme != "https" && scheme != "git") {
		return errors.New("is neither an https:// nor a git:// URL")
	}
	u, err := url.Parse(v)
	switch {
	case err != nil:
		return errors.New("is not a URL")
	case u.Host == "" || u.Opaque != "":
		return errors.New("has no host")
	case u.User != nil:
		return errors.New("carries user information")
	case scheme == "git" && u.Hostname() != "127.0.0.1":
		return errors.New("is a git:// URL that is not on 127.0.0.1")
	}
	return nil
}
