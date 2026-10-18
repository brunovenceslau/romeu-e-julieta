// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// writeHelp writes the help of c to its output stream, the CLI writer of
// stdout (ADR 0015, decision 5): its usage line, its summary, the
// commands below it, its flags, each with its value name and default,
// its examples, then the exit-code section that go generate writes from
// the error table (exitcodes_gen.go). It replaces cobra's template help
// on every level.
func writeHelp(c *cobra.Command, _ []string) {
	c.InitDefaultHelpFlag()
	var b strings.Builder
	if c.HasAvailableSubCommands() {
		fmt.Fprintf(&b, "Usage: %s <command> [flags]\n", c.CommandPath())
	} else {
		fmt.Fprintf(&b, "Usage: %s\n", c.UseLine())
	}
	if c.Short != "" {
		fmt.Fprintf(&b, "\n%s\n", c.Short)
	}
	if c.Long != "" {
		fmt.Fprintf(&b, "\n%s\n", c.Long)
	}
	var subs []*cobra.Command
	width := 0
	for _, s := range c.Commands() {
		if !s.Hidden {
			subs = append(subs, s)
			width = max(width, len(s.Name()))
		}
	}
	if len(subs) > 0 {
		b.WriteString("\nCommands:\n")
		for _, s := range subs {
			fmt.Fprintf(&b, "  %-*s  %s\n", width, s.Name(), s.Short)
		}
	}
	if c.HasAvailableFlags() {
		fmt.Fprintf(&b, "\nFlags:\n%s", c.Flags().FlagUsages())
	}
	if c.Example != "" {
		b.WriteString("\nExamples:\n")
		for line := range strings.SplitSeq(c.Example, "\n") {
			fmt.Fprintf(&b, "  %s\n", strings.TrimSpace(line))
		}
	}
	if len(subs) > 0 {
		fmt.Fprintf(&b, "\nRun %s <command> --help for the help of a command.\n", c.CommandPath())
	}
	b.WriteString("\n" + exitCodesHelp)
	_, _ = io.WriteString(c.OutOrStdout(), b.String())
}

// helpCommand returns the help command of the tree: help [<command>]
// prints the help of the binary or of the command named, and a command
// the tree does not have is the error usage (04 4.2, 04 4.3).
func helpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "print the help of a command",
		Args:  cobra.ArbitraryArgs,
		RunE: func(c *cobra.Command, args []string) error {
			root := c.Root()
			target, rest, err := root.Find(args)
			if err != nil || len(rest) > 0 || target.Hidden {
				return usageError(root, fmt.Sprintf("%s has no command %q", root.Name(), strings.Join(args, " ")))
			}
			writeHelp(target, nil)
			return nil
		},
	}
}
