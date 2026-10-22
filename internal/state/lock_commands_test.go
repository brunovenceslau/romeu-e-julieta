// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// lockers are the eleven commands that take romeu.lock, and lockFree
// the ones that do not (04 4.1, Locking, REQ-0284): the commands whose
// Writes cell in 04 4.2 is not "nothing" take it. The test is in the
// external package, so that it links every command package of romeu,
// which will import this one.
var (
	lockers  = []string{"init", "approve", "sync", "run", "adopt", "stop", "salvage", "rm", "recreate", "retire", "pull"}
	lockFree = []string{"status", "doctor", "handoff", "version"}
)

// TestRomeuCommands_lock is the table over the command definitions of
// REQ-0284: each definition's lock field is the one 04 4.1 gives its
// name. A command of the lists that no package defines yet is named in
// the log of its case, and a definition in neither list fails, so a new
// command states its lock before it is merged.
func TestRomeuCommands_lock(t *testing.T) {
	require.Len(t, lockers, 11, "the eleven commands of 04 4.1, Locking")
	defined := map[string]cli.Command{}
	for _, c := range cli.Commands("romeu") {
		defined[c.Name] = c
		assert.Truef(t, slices.Contains(lockers, c.Name) || slices.Contains(lockFree, c.Name),
			"romeu %s is a command 04 4.1 (Locking) says nothing about", c.Name)
	}
	check := func(names []string, want cli.Lock) {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				c, ok := defined[name]
				if !ok {
					// No definition yet: the case asserts the command when
					// its package is linked in (a test cannot skip, 10 10.6).
					t.Logf("romeu %s has no definition yet", name)
					return
				}
				assert.Equal(t, want, c.Effects().Lock)
			})
		}
	}
	t.Run("takes the lock", func(t *testing.T) { check(lockers, cli.LockTaken) })
	t.Run("takes no lock", func(t *testing.T) { check(lockFree, cli.LockNone) })
}
