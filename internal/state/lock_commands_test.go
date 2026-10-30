// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// The commands of 04 4.1, Locking (REQ-0284): the eleven that take
// romeu.lock, and the ones that do not. completion and help are
// builtins of every tree, which have no definition (cli.Builtins).
var (
	lockers  = []string{"init", "approve", "sync", "run", "adopt", "stop", "salvage", "rm", "recreate", "retire", "pull"}
	lockFree = []string{"status", "doctor", "handoff", "version", "completion", "help"}
)

// notDefinedYet maps each command of the two lists that no package
// defines yet to the task that will. TestRomeuCommands_lock holds the
// map equal to the real set of undefined commands, so a command that
// lands without leaving the map fails the test, and a name left in the
// map after its package is linked in is a stale line. The map only
// shrinks.
var notDefinedYet = map[string]string{
	"run": "T066", "adopt": "T050",
	"stop": "T067", "pull": "T067", "salvage": "T068", "rm": "T068", "recreate": "T068",
	"retire": "T069", "status": "T048", "handoff": "T070",
}

// TestRomeuCommands_lock is the table over the command definitions of
// REQ-0284: the lock field of each definition is the one 04 4.1 gives
// its name, and a definition in neither list fails, so a new command
// states its lock before it is merged.
func TestRomeuCommands_lock(t *testing.T) {
	require.Len(t, lockers, 11, "the eleven commands of 04 4.1, Locking")
	defined := map[string]cli.Command{}
	for _, c := range cli.Commands("romeu") {
		defined[c.Name] = c
		assert.Truef(t, slices.Contains(lockers, c.Name) || slices.Contains(lockFree, c.Name),
			"romeu %s is a command 04 4.1 (Locking) says nothing about", c.Name)
	}

	t.Run("the commands not defined yet are the ones the list names", func(t *testing.T) {
		var undefined []string
		for _, name := range slices.Concat(lockers, lockFree) {
			if _, ok := defined[name]; !ok && !slices.Contains(cli.Builtins, name) {
				undefined = append(undefined, name)
			}
		}
		assert.ElementsMatch(t, undefined, slices.Collect(maps.Keys(notDefinedYet)),
			"notDefinedYet is the set of commands of 04 4.1 with no definition; remove a command from it when its package lands")
	})
	t.Run("completion and help are builtins", func(t *testing.T) {
		assert.Subset(t, cli.Builtins, []string{"completion", "help"})
	})
	t.Run("a defined command has the lock 04 4.1 gives it", func(t *testing.T) {
		for name, c := range defined {
			want := cli.LockNone
			if slices.Contains(lockers, name) {
				want = cli.LockTaken
			}
			assert.Equalf(t, want, c.Effects().Lock, "romeu %s", name)
		}
	})
}
