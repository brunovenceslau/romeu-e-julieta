// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// binaries are the two binaries a command can belong to.
var binaries = []string{"romeu", "julieta"}

// registry holds definitions in the order of their registrations.
type registry struct{ cmds []Command }

// commands is the registry Register fills from init at program start;
// nothing writes it after main starts (10 10.6).
var commands registry

// Register adds the definition to the commands Main dispatches, and
// returns it, for a package-level variable: var _ = cli.Register(def).
// It panics on a definition that is not whole (an unknown binary, no
// name, the name help, no Run, a side-effect field left unset, a flag
// that clashes with --help or --json) and on a name its binary already
// has, so the mistake fails at init, in every test of the binary.
func Register(c Command) Command {
	commands.register(c)
	return c
}

// register adds c, or panics as Register says.
func (r *registry) register(c Command) {
	if err := c.check(); err != nil {
		panic(err)
	}
	for _, have := range r.cmds {
		if have.Binary == c.Binary && have.Name == c.Name {
			panic(fmt.Sprintf("cli: %s %s is registered twice", c.Binary, c.Name))
		}
	}
	r.cmds = append(r.cmds, c)
}

// check returns why the definition is not whole, or nil.
func (c Command) check() error {
	var missing []string
	if !slices.Contains(binaries, c.Binary) {
		missing = append(missing, fmt.Sprintf("a binary of %s, not %q", strings.Join(binaries, " or "), c.Binary))
	}
	if c.Name == "" || c.Name == "help" || strings.HasPrefix(c.Name, "-") || strings.ContainsAny(c.Name, " \t\n") {
		missing = append(missing, fmt.Sprintf("a name of one word other than help, not %q", c.Name))
	}
	if c.Run == nil {
		missing = append(missing, "Run")
	}
	e := c.effects
	if e.Preflight == PreflightUnset {
		missing = append(missing, "the preflight field")
	}
	if e.Lock == LockUnset {
		missing = append(missing, "the lock field")
	}
	if !e.Appending.set {
		missing = append(missing, "the appending field")
	}
	if e.Snapshot == SnapshotUnset {
		missing = append(missing, "the snapshot field")
	}
	if e.Idempotency == IdempotencyUnset {
		missing = append(missing, "the idempotency field")
	}
	if err := c.flagsBind(); err != nil {
		missing = append(missing, err.Error())
	}
	if len(missing) > 0 {
		return fmt.Errorf("cli: the definition of %s %q lacks %s", c.Binary, c.Name, strings.Join(missing, ", "))
	}
	return nil
}

// flagsBind binds the command's flags to a scratch set beside --help
// and --json, as Main does, and returns an error for a flag that clashes
// with them, which flag reports by a panic.
func (c Command) flagsBind() (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("flags that bind once beside --help and --json (%v)", p)
		}
	}()
	c.flagSet()
	return nil
}

// flagSet returns the set Main parses the command's flags with: --help,
// --json for a command that takes it, then the command's own.
func (c Command) flagSet() (*flag.FlagSet, *bool, *bool) {
	fs := flag.NewFlagSet(c.Name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	help := fs.Bool("help", false, "print this help")
	json := new(bool)
	if c.JSON {
		fs.BoolVar(json, "json", false, "print one JSON document")
	}
	if c.Flags != nil {
		c.Flags(fs)
	}
	return fs, help, json
}

// Commands returns the registered definitions of the binary, sorted by
// name.
func Commands(binary string) []Command { return commands.list(binary) }

// list returns the definitions of the binary, sorted by name.
func (r *registry) list(binary string) []Command {
	var out []Command
	for _, c := range r.cmds {
		if c.Binary == binary {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b Command) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// lookup returns the definition of the binary's command name.
func (r *registry) lookup(binary, name string) (Command, bool) {
	for _, c := range r.cmds {
		if c.Binary == binary && c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}
