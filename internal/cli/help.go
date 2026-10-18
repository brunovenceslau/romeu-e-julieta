// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"flag"
	"fmt"
	"io"
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
	for _, c := range Commands(binary) {
		fmt.Fprintf(&b, "  %-16s %s\n", c.Line(), c.Summary)
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
	for _, d := range defs {
		fmt.Fprintf(&b, "  %-10s %s\n", d.Sub, d.Summary)
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
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
		line += fmt.Sprintf(" (default %q)", f.DefValue)
	}
	return line + "\n"
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
