// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// completionCommand returns the completion command of the tree, with one
// subcommand per shell (04 4.2, 04 4.3; ADR 0015, decision 6). The
// script is a named exception to the CLI writer (04 4.1, Output,
// REQ-0270): it needs real tabs and line feeds, so it is written to the
// stream under the writer of stdout, and it holds only the binary's own
// text.
func completionCommand() *cobra.Command {
	c := group("completion", "print the shell completion script")
	shells := []struct {
		name string
		gen  func(root *cobra.Command, w io.Writer) error
	}{
		{"bash", func(root *cobra.Command, w io.Writer) error { return root.GenBashCompletionV2(w, true) }},
		{"zsh", func(root *cobra.Command, w io.Writer) error { return root.GenZshCompletion(w) }},
		{"fish", func(root *cobra.Command, w io.Writer) error { return root.GenFishCompletion(w, true) }},
	}
	for _, shell := range shells {
		c.AddCommand(&cobra.Command{
			Use:   shell.name,
			Short: "print the " + shell.name + " completion script",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				out := c.OutOrStdout()
				if w, ok := out.(*Writer); ok {
					out = w.out
				}
				if err := shell.gen(c.Root(), out); err != nil {
					return Internal.Wrap(err, "the "+shell.name+" completion script")
				}
				return nil
			},
		})
	}
	return c
}

// The bounds of a completion function (04 4.1, Completion, REQ-1245).
// completionTimeout is a variable only so a test can shorten it.
const maxCompletions = 1000

var completionTimeout = time.Second

// Complete returns the completion function of a command's arguments or
// flag values from names, which reads records only: it starts no
// subprocess, opens no network connection, takes no lock and writes
// nothing (04 4.1, Completion, REQ-1245). It offers only the names valid
// accepts, the validators of 03 3.1 that fit what is completed, within
// the bounds of bound. valid is required: Complete panics on a nil one,
// so the mistake fails when the command is built.
func Complete(names func(ctx context.Context, args []string, toComplete string) []string, valid func(string) bool) cobra.CompletionFunc {
	if valid == nil {
		panic("cli: Complete needs the validator of the names it offers")
	}
	return func(c *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		var out []cobra.Completion
		for _, name := range names(contextOf(c), args, toComplete) {
			if valid(name) {
				out = append(out, name)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// contextOf returns the context of c, or the background context before
// cobra has set one.
func contextOf(c *cobra.Command) context.Context {
	if ctx := c.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// bound returns f within the bounds of 04 4.1, Completion (REQ-1245): f
// runs under a context that ends after one second, and bound returns no
// name once it has ended, whether or not f has returned; otherwise it
// returns at most 1000 names. The tree applies it to the completion
// function of every command, so no command can leave it out, and the
// shell completes no file name in place of the names.
func bound(f cobra.CompletionFunc) cobra.CompletionFunc {
	return func(c *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		ctx, cancel := context.WithTimeout(contextOf(c), completionTimeout)
		defer cancel()
		c.SetContext(ctx)
		done := make(chan []cobra.Completion, 1)
		go func() {
			got, _ := f(c, args, toComplete)
			done <- got
		}()
		var got []cobra.Completion
		select {
		case got = <-done:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return got[:min(len(got), maxCompletions)], cobra.ShellCompDirectiveNoFileComp
	}
}

// completionWriter is the stream of the hidden __complete commands: each
// line is a name, then a tab and its description when it has one, and
// the last line is the directive. It escapes the name and the
// description of each line with termsafe and keeps the one tab between
// them and the line feeds, which the shell reads (04 4.1, Output,
// REQ-0270), so a name from a record that holds a control character, a
// tab or a bidi code point reaches the shell and the terminal escaped.
type completionWriter struct{ out io.Writer }

// Write escapes p line by line and writes it in one call.
func (w completionWriter) Write(p []byte) (int, error) {
	lines := strings.Split(string(p), "\n")
	for i, line := range lines {
		name, desc, hasDesc := strings.Cut(line, "\t")
		lines[i] = termsafe.Escape(name)
		if hasDesc {
			lines[i] += "\t" + termsafe.Escape(desc)
		}
	}
	if _, err := io.WriteString(w.out, strings.Join(lines, "\n")); err != nil {
		return 0, err
	}
	return len(p), nil
}
