// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"context"
	"maps"
	"strconv"
	"time"
)

// Class is the timeout class of a subprocess, a row of the timeout
// table. Every caller names the class of each command it runs; the
// zero value names none, and Run refuses it.
type Class int

// The classes of the timeout table, the rows of 04 4.1, Starting
// values. The classes of tools/ci and tools/mutants come with the task
// that switches them to this package (T105).
const (
	classNone    Class = iota // no class named: Run refuses it
	SbxRead                   // an sbx read: version, ls --json, env plan, policy ls, the secret and settings listings
	SbxChange                 // a mutating policy or secret call, a settings set, sbx stop, sbx env rm
	SbxStart                  // sbx env run -d <dir>, a start
	SbxCreate                 // sbx env run -d --clone <dir>, a create: image pulls and kit builds
	JulietaSetup              // the sbx env exec of julieta setup, mise downloads inside it
	JulietaExec               // every other sbx env exec of julieta, the salvage exec excepted
	HostGitFetch              // git clone and git fetch on the host
	Secret                    // each secret argv and the credential helper
	JulietaGit                // julieta's own git
	Mise                      // mise install and mise lock
	Herdr                     // herdr
	Unbounded                 // the salvage exec, the hook stages and the pane steps: no timeout
)

// timeouts is the timeout table, keyed by class. The rows of the
// product are the starting values of 04 4.1, which the first B5
// measurement confirms or resets; a change to them is a pull request
// that edits this table alone. Where two proposed tables differed, the
// longer value was taken. Zero means no timeout: the salvage exec moves
// up to the salvage cap and stops only on an interrupt, and the hook
// stages and the pane steps are the user's own programs (04 4.1,
// Subprocesses, item 5). Nothing writes the table: Go has no constant
// map, and a table keyed by class is what 04 4.1 asks for.
var timeouts = map[Class]time.Duration{
	SbxRead:      30 * time.Second,
	SbxChange:    2 * time.Minute,
	SbxStart:     5 * time.Minute,
	SbxCreate:    30 * time.Minute,
	JulietaSetup: 30 * time.Minute,
	JulietaExec:  5 * time.Minute,
	HostGitFetch: 15 * time.Minute,
	Secret:       30 * time.Second,
	JulietaGit:   5 * time.Minute,
	Mise:         30 * time.Minute,
	Herdr:        30 * time.Second,
	Unbounded:    0,
}

// classNames are the names the details of subprocess-timeout print.
var classNames = map[Class]string{
	SbxRead:      "sbx-read",
	SbxChange:    "sbx-change",
	SbxStart:     "sbx-start",
	SbxCreate:    "sbx-create",
	JulietaSetup: "julieta-setup",
	JulietaExec:  "julieta-exec",
	HostGitFetch: "host-git-fetch",
	Secret:       "secret",
	JulietaGit:   "julieta-git",
	Mise:         "mise",
	Herdr:        "herdr",
	Unbounded:    "unbounded",
}

// Timeout returns the limit of the class in the table; zero means the
// class has no timeout, or is not a class.
func (c Class) Timeout() time.Duration {
	return timeouts[c]
}

// String returns the name of the class, as subprocess-timeout prints
// it.
func (c Class) String() string {
	if name, ok := classNames[c]; ok {
		return name
	}
	return "class-" + strconv.Itoa(int(c))
}

// valid reports whether the class is a row of the table.
func (c Class) valid() bool {
	_, ok := timeouts[c]
	return ok
}

// testTimeoutKey is the context key of the limits WithTestTimeout sets.
type testTimeoutKey struct{}

// WithTestTimeout returns a context under which commands of the class
// have the limit d instead of the one of the table. It is the seam of
// the table, and only tests set it: the call-site scan of this
// package fails a call outside a _test.go file. A context carries it,
// so tests that run in parallel each have their own table. A d at or
// below zero means no timeout, as a zero does in the table.
func WithTestTimeout(ctx context.Context, c Class, d time.Duration) context.Context {
	limits := map[Class]time.Duration{}
	if outer, ok := ctx.Value(testTimeoutKey{}).(map[Class]time.Duration); ok {
		maps.Copy(limits, outer)
	}
	limits[c] = d
	return context.WithValue(ctx, testTimeoutKey{}, limits)
}

// limitOf returns the limit of the class under ctx: the one a test set
// through WithTestTimeout, else the one of the table.
func limitOf(ctx context.Context, c Class) time.Duration {
	if limits, ok := ctx.Value(testTimeoutKey{}).(map[Class]time.Duration); ok {
		if limit, ok := limits[c]; ok {
			return limit
		}
	}
	return c.Timeout()
}
