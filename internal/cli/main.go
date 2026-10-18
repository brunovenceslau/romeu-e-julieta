// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Main runs args, the arguments after the binary's name, through a new
// cobra tree of env.Binary (Tree) and returns the exit status (04 4.1,
// Exit codes): 0, or the exit code of the error the command returned,
// which it prints first (Report). Every error a command returns holds a
// *Error, since the tree wraps each RunE; so any other error of the run
// comes from the parser (an unknown command, subcommand or flag, or an
// argument count a command refuses) and is the error usage (RJ-349),
// whose hint names the help of the command that refused it. The binary's
// main runs it under procgroup.Main, which gives ctx and maps a signal to
// 130 or 143. An error that is the cancellation of ctx is printed by
// no one here: procgroup.Main prints the one line "<binary>:
// interrupted" (04 4.1, Signals), so Main returns 1 for it, the status
// of a failed operation, which the signal then overrides. An error that
// carries a row of the table besides the cancellation is still printed.
func Main(ctx context.Context, args []string, env *Env) int {
	env.JSON = false
	root := Tree(env)
	completing := len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd)
	if completing {
		// cobra v1.10.2 writes two things of a completion line outside
		// the writers it was given, and offers no hook for either: a
		// flag parse error as "[Error] ..." straight to os.Stderr, and
		// debug lines to the file BASH_COMP_DEBUG_FILE names. The
		// variable is unset so no file is written, and os.Stderr is a
		// pipe for the run, whose text goes through the CLI writer of
		// stderr after it (04 4.1, Output). Only args[0] is looked at:
		// cobra's completion scripts always call "<binary> __complete"
		// first, and the root has no persistent flag, so a flag cannot
		// precede the command (cobra's Find would strip it).
		_ = os.Unsetenv("BASH_COMP_DEBUG_FILE")
		if env.Stderr != nil {
			defer captureStderr(env.Stderr)()
		}
	}
	if completing && env.Stdout != nil {
		// The output of the hidden __complete commands is a named
		// exception to the CLI writer (04 4.1, Output, REQ-0270): the
		// shell reads a real tab between a name and its description, and
		// completionWriter escapes the rest.
		root.SetOut(completionWriter{env.Stdout.out})
	}
	// A nil slice would make cobra read os.Args.
	root.SetArgs(append([]string{}, args...))
	cmd, err := root.ExecuteContextC(ctx)
	if err != nil && ctx.Err() != nil && errors.Is(err, ctx.Err()) && Of(err).ID == Internal.ID {
		return 1
	}
	if err != nil {
		if _, ok := errors.AsType[*Error](err); !ok {
			err = usageError(cmd, err.Error())
		}
	}
	return Report(env, err)
}

// captureStderr swaps os.Stderr for a pipe and returns the function that
// restores it and writes what was written to the pipe to w in one Write,
// so a multi-byte sequence is not split and w escapes it. A goroutine drains the pipe meanwhile, so a long
// text cannot fill it. If the pipe cannot be made, os.Stderr stays and
// the returned function does nothing.
func captureStderr(w io.Writer) func() {
	r, pw, err := os.Pipe()
	if err != nil {
		return func() {}
	}
	old := os.Stderr
	os.Stderr = pw
	done := make(chan []byte)
	go func() {
		b, _ := io.ReadAll(r)
		_ = r.Close()
		done <- b
	}()
	return func() {
		os.Stderr = old
		_ = pw.Close()
		if b := <-done; len(b) > 0 {
			_, _ = w.Write(b)
		}
	}
}

// Tree returns a new cobra tree of env.Binary: the root, a group command
// above each set of subcommands, the command of each registered
// definition, the help command and the completion command (ADR 0015,
// decisions 2, 4 and 5). Each call builds every command anew, so no
// package holds a command of the tree and nothing is written to the
// registry after main starts. Its output and error streams are
// env.Stdout and env.Stderr, its help is this package's, and its errors
// and usage are printed by Report alone.
func Tree(env *Env) *cobra.Command {
	root := group(env.Binary, summaries[env.Binary])
	root.CompletionOptions.DisableDefaultCmd = true
	root.SilenceErrors = true
	root.SilenceUsage = true
	if env.Stdout != nil {
		root.SetOut(env.Stdout)
	}
	if env.Stderr != nil {
		root.SetErr(env.Stderr)
	}
	root.SetHelpFunc(writeHelp)
	root.SetUsageFunc(func(c *cobra.Command) error {
		writeHelp(c, nil)
		return nil
	})
	root.SetHelpCommand(helpCommand())
	groups := map[string]*cobra.Command{}
	for _, def := range commands.list(env.Binary) {
		c := def.New(env)
		run := c.RunE
		c.RunE = func(c *cobra.Command, args []string) error {
			if err := run(c, args); err != nil {
				return Of(err)
			}
			return nil
		}
		if c.ValidArgsFunction != nil {
			c.ValidArgsFunction = bound(c.ValidArgsFunction)
		}
		if def.JSON {
			c.Flags().BoolVar(&env.JSON, "json", false, "print one JSON document")
		}
		if def.Sub == "" {
			root.AddCommand(c)
			continue
		}
		g, ok := groups[def.Name]
		if !ok {
			g = group(def.Name, "the "+def.Name+" commands")
			groups[def.Name] = g
			root.AddCommand(g)
		}
		g.AddCommand(c)
	}
	root.AddCommand(completionCommand())
	return root
}

// summaries are the lines the help and the reference page of each
// binary open with (04 4.2, 04 4.3).
var summaries = map[string]string{
	"romeu":   "the commands that run on the host",
	"julieta": "the commands that run inside a sandbox",
}

// group returns a command above a set of subcommands: it takes no
// arguments, so a word that is no subcommand of it is the error usage,
// and run alone it is the error usage too, rather than cobra's help with
// exit 0.
func group(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			return usageError(c, c.CommandPath()+" needs a command")
		},
	}
}

// usageError returns the error usage for the command c refused, with its
// hint filled: the help of the binary, or of the command when c is
// below the root. c is nil when cobra found no command at all.
func usageError(c *cobra.Command, message string) *Error {
	e := Usage.New(message)
	e.Fill = map[string]string{"<binary>": "", " <command>": ""}
	if c != nil {
		binary, command, _ := strings.Cut(c.CommandPath(), " ")
		e.Fill["<binary>"] = binary
		if command != "" {
			e.Fill[" <command>"] = " " + command
		}
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
