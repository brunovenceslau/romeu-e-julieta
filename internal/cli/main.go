// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Main dispatches args, the arguments after the binary's name, to the
// registered definition of env.Binary that args[0] names, and args[1]
// as well when that command has subcommands, and returns the exit status
// (04 4.1, Exit codes): 0, or the exit code of the error the definition
// returned, which it prints first (Report). --help on the binary, on a
// command with subcommands and on a definition prints its help to stdout
// and returns 0 (04 4.1, Parsing: --help on every level). A command,
// subcommand, flag or argument the definitions do not have is the error
// usage (RJ-349). The binary's main runs it under procgroup.Main, which
// gives ctx and maps a signal to 130 or 143.
func Main(ctx context.Context, args []string, env *Env) int {
	env.JSON = false
	if len(args) > 0 && isHelp(args[0]) {
		writeBinaryHelp(env.Stdout, env.Binary)
		return 0
	}
	if len(args) == 0 {
		return Report(env, usageError(env.Binary, "", "no command given"))
	}
	defs := commands.named(env.Binary, args[0])
	if len(defs) == 0 {
		return Report(env, usageError(env.Binary, "", fmt.Sprintf("%s has no command %q", env.Binary, args[0])))
	}
	c, rest := defs[0], args[1:]
	if c.Sub != "" {
		if len(rest) > 0 && isHelp(rest[0]) {
			writeSubcommandsHelp(env.Stdout, defs)
			return 0
		}
		if len(rest) == 0 {
			return Report(env, usageError(env.Binary, c.Name, fmt.Sprintf("%s %s needs a subcommand", env.Binary, c.Name)))
		}
		if strings.HasPrefix(rest[0], "-") {
			return Report(env, usageError(env.Binary, c.Name, fmt.Sprintf("the flags of %s %s go after its subcommand, not %q", env.Binary, c.Name, rest[0])))
		}
		i := slices.IndexFunc(defs, func(d Command) bool { return d.Sub == rest[0] })
		if i < 0 {
			return Report(env, usageError(env.Binary, c.Name, fmt.Sprintf("%s %s has no subcommand %q", env.Binary, c.Name, rest[0])))
		}
		c, rest = defs[i], rest[1:]
	}
	fs, help, json := c.flagSet()
	if err := fs.Parse(rest); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			return Report(env, usageError(env.Binary, c.Line(), err.Error()))
		}
		*help = true
	}
	env.JSON = *json
	if *help {
		writeCommandHelp(env.Stdout, c, fs)
		return 0
	}
	return Report(env, c.Run(ctx, env, fs.Args()))
}

// isHelp reports whether arg asks for the binary's help.
func isHelp(arg string) bool {
	return arg == "--help" || arg == "-help" || arg == "-h" || arg == "help"
}

// usageError returns the error usage for the binary and, when one was
// named, the command line (a command, or a command and its subcommand),
// with its hint filled: the help to read.
func usageError(binary, command, message string) *Error {
	e := Usage.New(message)
	e.Fill = map[string]string{"<binary>": binary, " <command>": ""}
	if command != "" {
		e.Fill[" <command>"] = " " + command
	}
	return e
}

// oneLine returns s with each line feed written as the two characters
// \n, so a message, a hint or a detail stays on its own line: a value
// that holds a subprocess's stderr cannot start a line of its own that
// forges another error, a fix line or a CI annotation ("::error::"). The
// CLI writer escapes every other code point of I29.
func oneLine(s string) string { return strings.ReplaceAll(s, "\n", `\n`) }

// Report prints the error err carries to env.Stderr, in the text form or,
// with env.JSON, the --json form (04 4.1, Error ids, item 4), and returns
// its exit code; it prints nothing and returns 0 for a nil err.
func Report(env *Env, err error) int {
	if err == nil {
		return 0
	}
	e := Of(err)
	if env.JSON {
		writeJSONError(env.Stderr, e)
		return e.Exit
	}
	var b strings.Builder
	fmt.Fprintf(&b, "error %s %s: %s\n", e.ID, e.Slug, oneLine(e.Error()))
	fmt.Fprintf(&b, "fix: %s\n", oneLine(e.Hint()))
	for _, d := range e.Details {
		fmt.Fprintf(&b, "%s: %s\n", oneLine(d.Key), oneLine(d.Value))
	}
	_, _ = io.WriteString(env.Stderr, b.String())
	return e.Exit
}
