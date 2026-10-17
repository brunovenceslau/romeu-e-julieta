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
		fmt.Fprintf(&b, "  %-10s %s\n", c.Name, c.Summary)
	}
	fmt.Fprintf(&b, "\nRun %s <command> --help for the help of a command.\n\n%s", binary, exitCodes())
	_, _ = io.WriteString(w, b.String())
}

// writeCommandHelp writes the help of a command: its usage line, its
// flags, its examples, then the exit codes.
func writeCommandHelp(w io.Writer, c Command, fs *flag.FlagSet) {
	var b strings.Builder
	fmt.Fprintf(&b, "Usage: %s %s %s\n\n%s\n\nFlags:\n", c.Binary, c.Name, c.Usage, c.Summary)
	fs.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(&b, "  --%s  %s\n", f.Name, f.Usage)
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
