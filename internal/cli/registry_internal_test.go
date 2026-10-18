// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRegistryOrderAndDuplicates registers into a registry of its own,
// keeps the order of the registrations, lists a binary's commands by
// name, and panics on a name registered twice.
func TestRegistryOrderAndDuplicates(t *testing.T) {
	def := func(binary, name string) Command {
		c := Define(PreflightNone, LockNone, AppendsNothing(), SnapshotNone, ReadsOnly)
		c.Binary, c.Name = binary, name
		c.Run = func(context.Context, *Env, []string) error { return nil }
		return c
	}
	var r registry
	r.register(def("julieta", "zz"))
	r.register(def("romeu", "mm"))
	r.register(def("julieta", "aa"))
	var order, listed []string
	for _, c := range r.cmds {
		order = append(order, c.Binary+" "+c.Name)
	}
	for _, c := range r.list("julieta") {
		listed = append(listed, c.Name)
	}
	assert.Equal(t, []string{"julieta zz", "romeu mm", "julieta aa"}, order)
	assert.Equal(t, []string{"aa", "zz"}, listed)
	assert.Empty(t, r.named("romeu", "zz"), "a name is looked up in its own binary")
	assert.PanicsWithValue(t, "cli: julieta zz is registered twice", func() { r.register(def("julieta", "zz")) })
}

// TestRegistrySubcommands registers a command's subcommands as
// definitions of their own, lists them by subcommand, and panics on a
// subcommand registered twice and on a name that is both a definition
// and the command of subcommands.
func TestRegistrySubcommands(t *testing.T) {
	def := func(name, sub string) Command {
		c := Define(PreflightNone, LockNone, AppendsNothing(), SnapshotNone, ReadsOnly)
		c.Binary, c.Name, c.Sub = "julieta", name, sub
		c.Run = func(context.Context, *Env, []string) error { return nil }
		return c
	}
	var r registry
	r.register(def("memory", "check"))
	r.register(def("memory", "add"))
	r.register(def("status", ""))
	var lines []string
	for _, c := range r.named("julieta", "memory") {
		lines = append(lines, c.Line())
	}
	assert.Equal(t, []string{"memory add", "memory check"}, lines)
	assert.PanicsWithValue(t, "cli: julieta memory add is registered twice", func() { r.register(def("memory", "add")) })
	assert.PanicsWithValue(t, "cli: julieta memory has subcommands, so it is no definition of its own", func() { r.register(def("memory", "")) })
	assert.PanicsWithValue(t, "cli: julieta status has subcommands, so it is no definition of its own", func() { r.register(def("status", "x")) })
	assert.Panics(t, func() { r.register(def("pin", "help")) }, "a subcommand is not named help")
}
