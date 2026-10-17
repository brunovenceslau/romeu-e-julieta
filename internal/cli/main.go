// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// Main dispatches args, the arguments after the binary's name, to the
// registered command of env.Binary that args[0] names, and returns the
// exit status (04 4.1, Exit codes): 0, or the exit code of the error the
// command returned, which it prints first (Report). --help on the binary
// or a command prints the help to stdout and returns 0. A command, flag
// or argument the definitions do not have is the error usage (RJ-349).
// The binary's main runs it under procgroup.Main, which gives ctx and
// maps a signal to 130 or 143.
func Main(ctx context.Context, args []string, env *Env) int {
	env.JSON = false
	if len(args) > 0 && isHelp(args[0]) {
		writeBinaryHelp(env.Stdout, env.Binary)
		return 0
	}
	if len(args) == 0 {
		return Report(env, usageError(env.Binary, "", "no command given"))
	}
	c, ok := commands.lookup(env.Binary, args[0])
	if !ok {
		return Report(env, usageError(env.Binary, "", fmt.Sprintf("%s has no command %q", env.Binary, args[0])))
	}
	fs, help, json := c.flagSet()
	if err := fs.Parse(args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			return Report(env, usageError(env.Binary, c.Name, err.Error()))
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
// named, the command, with its hint filled: the help to read.
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
