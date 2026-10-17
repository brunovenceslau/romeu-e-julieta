// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"context"
	"flag"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// whole returns a definition with every field set, for the cases that
// unset one.
func whole() cli.Command {
	c := cli.Define(cli.PreflightRuns, cli.LockTaken, cli.Appends("a record"), cli.SnapshotNone, cli.AppendsEachRun)
	c.Binary, c.Name = "julieta", "whole"
	c.Run = func(context.Context, *cli.Env, []string) error { return nil }
	return c
}

// TestRegisterRefusesAnUnsetField panics at registration on each
// side-effect field left unset, and on a missing name, binary or Run.
func TestRegisterRefusesAnUnsetField(t *testing.T) {
	run := func(context.Context, *cli.Env, []string) error { return nil }
	tests := []struct {
		name string
		def  cli.Command
		want string
	}{
		{"preflight", cli.Define(cli.PreflightUnset, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly), "the preflight field"},
		{"lock", cli.Define(cli.PreflightNone, cli.LockUnset, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly), "the lock field"},
		{"appending", cli.Define(cli.PreflightNone, cli.LockNone, cli.Appending{}, cli.SnapshotNone, cli.ReadsOnly), "the appending field"},
		{"snapshot", cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotUnset, cli.ReadsOnly), "the snapshot field"},
		{"idempotency", cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.IdempotencyUnset), "the idempotency field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.def.Binary, tt.def.Name, tt.def.Run = "romeu", "unset-"+tt.name, run
			assert.PanicsWithError(t, "cli: the definition of romeu \"unset-"+tt.name+"\" lacks "+tt.want, func() { cli.Register(tt.def) })
		})
	}
	t.Run("binary, name and run", func(t *testing.T) {
		c := whole()
		c.Binary, c.Name, c.Run = "other", "", nil
		assert.Panics(t, func() { cli.Register(c) })
	})
	t.Run("the name help", func(t *testing.T) {
		c := whole()
		c.Name = "help"
		assert.Panics(t, func() { cli.Register(c) })
	})
	t.Run("a flag that clashes with --json", func(t *testing.T) {
		c := whole()
		c.Name, c.JSON = "clash", true
		c.Flags = func(fs *flag.FlagSet) { fs.Bool("json", false, "") }
		assert.Panics(t, func() { cli.Register(c) })
	})
}

// lockList returns the romeu commands that take the lock, from the
// Locking row of 04 4.1: "The commands that take it are ...;".
func lockList(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("../../docs/spec/04-cli.md")
	require.NoError(t, err)
	_, rest, ok := strings.Cut(string(src), "The commands that take it are the ones whose Writes cell in 4.2 is not `nothing`: ")
	require.True(t, ok, "the Locking row of 04 4.1 lists the commands that take the lock")
	list, _, ok := strings.Cut(rest, ";")
	require.True(t, ok)
	var names []string
	for _, m := range regexp.MustCompile("`([a-z-]+)`").FindAllStringSubmatch(list, -1) {
		names = append(names, m[1])
	}
	require.NotEmpty(t, names)
	return names
}

// TestLockField holds the lock field of each romeu definition to the
// list of 04 4.1, Locking: a command takes the lock exactly when the
// list names it; julieta takes no romeu lock.
func TestLockField(t *testing.T) {
	takes := lockList(t)
	assert.Contains(t, takes, "sync")
	for _, c := range cli.Commands("romeu") {
		want := cli.LockNone
		if slices.Contains(takes, c.Name) {
			want = cli.LockTaken
		}
		assert.Equal(t, want, c.Effects().Lock, "the lock field of romeu %s", c.Name)
	}
}
