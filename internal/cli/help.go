// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// exitCodes returns the exit-code section of every --help, from
// ExitCodes (04 4.1, Exit codes; Signals).
func exitCodes() string {
	var b strings.Builder
	b.WriteString("Exit codes:\n")
	for _, e := range ExitCodes {
		fmt.Fprintf(&b, "  %-4d %s\n", e.Code, e.Meaning)
	}
	b.WriteString(SignalPrecedence + "\n")
	return b.String()
}

// writeBinaryHelp writes the help of the binary: its commands, then the
// exit codes.
func writeBinaryHelp(w io.Writer, binary string) {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s <command> [flags] [args]\n\nCommands:\n", binary)
	cmds := Commands(binary)
	width := 0
	for _, c := range cmds {
		width = max(width, len(c.Line()))
	}
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, c.Line(), c.Summary)
	}
	fmt.Fprintf(&b, "\nRun %s <command> --help for the help of a command.\n\n%s", binary, exitCodes())
	_, _ = io.WriteString(w, b.String())
}

// writeSubcommandsHelp writes the help of a command that has
// subcommands, defs: its subcommands, then the exit codes.
func writeSubcommandsHelp(w io.Writer, defs []Command) {
	var b strings.Builder
	c := defs[0]
	fmt.Fprintf(&b, "Usage: %s %s <subcommand> [flags] [args]\n\nSubcommands:\n", c.Binary, c.Name)
	width := 0
	for _, d := range defs {
		width = max(width, len(d.Sub))
	}
	for _, d := range defs {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, d.Sub, d.Summary)
	}
	fmt.Fprintf(&b, "\nRun %s %s <subcommand> --help for the help of a subcommand.\n\n%s", c.Binary, c.Name, exitCodes())
	_, _ = io.WriteString(w, b.String())
}

// flagLine returns the help line of one flag: its name, the name of its
// value when it takes one (a back-quoted word of its usage, or the
// type flag.UnquoteUsage gives), its usage, and its default when that
// is not the zero value.
func flagLine(f *flag.Flag) string {
	value, usage := flag.UnquoteUsage(f)
	name := "--" + f.Name
	if value != "" {
		name += " <" + value + ">"
	}
	line := fmt.Sprintf("  %s  %s", name, usage)
	if !isZeroValue(f) {
		if isStringFlag(f) {
			line += fmt.Sprintf(" (default %q)", f.DefValue)
		} else {
			line += fmt.Sprintf(" (default %s)", f.DefValue)
		}
	}
	return line + "\n"
}

// isZeroValue reports whether the default of f is the zero value of its
// type, as flag.PrintDefaults decides it: a new value of the type, which
// is its zero value, prints the same text.
func isZeroValue(f *flag.Flag) bool {
	t := reflect.TypeOf(f.Value)
	var zero reflect.Value
	if t.Kind() == reflect.Pointer {
		zero = reflect.New(t.Elem())
	} else {
		zero = reflect.Zero(t)
	}
	v, ok := zero.Interface().(flag.Value)
	if !ok {
		return false
	}
	return f.DefValue == v.String()
}

// isStringFlag reports whether f holds a string, whose default the help
// quotes.
func isStringFlag(f *flag.Flag) bool {
	getter, ok := f.Value.(flag.Getter)
	if !ok {
		return false
	}
	_, ok = getter.Get().(string)
	return ok
}

// writeCommandHelp writes the help of a command: its usage line, its
// flags, its examples, then the exit codes.
func writeCommandHelp(w io.Writer, c Command, fs *flag.FlagSet) {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s %s %s\n\n%s\n\nFlags:\n", c.Binary, c.Line(), c.Usage, c.Summary)
	fs.VisitAll(func(f *flag.Flag) {
		b.WriteString(flagLine(f))
	})
	if len(c.Examples) > 0 {
		b.WriteString("\nExamples:\n")
		for _, ex := range c.Examples {
			fmt.Fprintf(&b, "  %s\n", ex)
		}
	}
	b.WriteString("\n" + exitCodes())
	_, _ = io.WriteString(w, b.String())
}
