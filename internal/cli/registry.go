// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// binaries are the two binaries a command can belong to.
var binaries = []string{"romeu", "julieta"}

// Builtins are the commands cobra and this package add to every tree,
// which have no definition: the tests that read the side-effect fields
// skip them (ADR 0015, decision 2).
var Builtins = []string{"help", "completion", "__complete", "__completeNoDesc"}

// registry holds definitions in the order of their registrations.
type registry struct{ cmds []Command }

// commands is the registry Register fills from package-level variables
// at program start; nothing writes it after main starts (REQ-1229).
var commands registry

// Register adds the definition to the tree Main builds for its binary,
// and returns it, for a package-level variable: var _ = cli.Register(def).
// It panics on a definition that is not whole and on a command path its
// binary already has, so the mistake fails at init, in every test of the
// binary.
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
		if have.Binary != c.Binary || have.Name != c.Name {
			continue
		}
		if have.Sub == c.Sub {
			panic(fmt.Errorf("cli: %s %s is registered twice", c.Binary, c.Line()))
		}
		if have.Sub == "" || c.Sub == "" {
			panic(fmt.Errorf("cli: %s %s has subcommands, so it is no definition of its own", c.Binary, c.Name))
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
	if !isWord(c.Name) {
		missing = append(missing, fmt.Sprintf("a name of one word that is no command of every tree, not %q", c.Name))
	}
	if c.Sub != "" && !isWord(c.Sub) {
		missing = append(missing, fmt.Sprintf("a subcommand of one word that is no command of every tree, not %q", c.Sub))
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
	if c.New == nil {
		missing = append(missing, "New")
	} else {
		missing = append(missing, c.cobraProblems()...)
	}
	if len(missing) > 0 {
		return fmt.Errorf("cli: the definition of %s %q lacks %s", c.Binary, c.Line(), strings.Join(missing, ", "))
	}
	return nil
}

// cobraProblems builds the definition's command once and returns what
// keeps it from joining a tree: a Use that names another command, no
// Args validator, no RunE, a run function other than RunE, which could
// print or fail outside Main's report, a Deprecated or Version text,
// which cobra prints on its own, a help or json flag, which the tree
// adds, a name of its own beside its path (Aliases, SuggestFor), Hidden,
// commands of its own, which would skip these checks, a flag error it
// lets pass (FParseErrWhitelist), its own output streams, help or usage,
// which would print outside the CLI writer, and a completion function on
// a flag, which the tree cannot bound (REQ-1245). DisableFlagParsing is
// left to the command: julieta hooks run passes git's arguments on
// unparsed (ADR 0015, decision 3).
func (c Command) cobraProblems() []string {
	cmd := c.New(&Env{Binary: c.Binary})
	if cmd == nil {
		return []string{"a New that returns a command"}
	}
	var missing []string
	want := c.Name
	if c.Sub != "" {
		want = c.Sub
	}
	if cmd.Name() != want {
		missing = append(missing, fmt.Sprintf("a Use that starts with %q, not %q", want, cmd.Use))
	}
	if cmd.Args == nil {
		missing = append(missing, "an Args validator")
	}
	if cmd.RunE == nil {
		missing = append(missing, "a RunE")
	}
	if cmd.Run != nil || cmd.PreRun != nil || cmd.PreRunE != nil || cmd.PostRun != nil || cmd.PostRunE != nil ||
		cmd.PersistentPreRun != nil || cmd.PersistentPreRunE != nil || cmd.PersistentPostRun != nil || cmd.PersistentPostRunE != nil {
		missing = append(missing, "no run function but RunE")
	}
	if cmd.Deprecated != "" || cmd.Version != "" {
		missing = append(missing, "no Deprecated or Version text")
	}
	for _, flags := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()} {
		if flags.Lookup("help") != nil || flags.ShorthandLookup("h") != nil {
			missing = append(missing, "no flag help or -h, which the tree adds")
		}
		if flags.Lookup("json") != nil {
			missing = append(missing, "no flag json, which the tree adds when JSON is set")
		}
	}
	if len(cmd.Aliases) > 0 || len(cmd.SuggestFor) > 0 {
		missing = append(missing, "no Aliases or SuggestFor")
	}
	if cmd.Hidden || cmd.HasSubCommands() {
		missing = append(missing, "neither Hidden nor commands of its own")
	}
	if cmd.FParseErrWhitelist.UnknownFlags {
		missing = append(missing, "no FParseErrWhitelist")
	}
	if ownsOutput(cmd) {
		missing = append(missing, "no output stream, help or usage of its own")
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if _, ok := cmd.GetFlagCompletionFunc(f.Name); ok {
			missing = append(missing, "no completion function on the flag "+f.Name)
		}
	})
	return missing
}

// ownsOutput reports whether cmd sets its own output or error stream,
// help or usage: under a parent that sets sentinels of each, cmd uses
// one of its own.
func ownsOutput(cmd *cobra.Command) bool {
	var out, errOut strings.Builder
	helped, used := false, false
	parent := &cobra.Command{Use: "probe"}
	parent.SetOut(&out)
	parent.SetErr(&errOut)
	parent.SetHelpFunc(func(*cobra.Command, []string) { helped = true })
	parent.SetUsageFunc(func(*cobra.Command) error {
		used = true
		return nil
	})
	parent.AddCommand(cmd)
	defer parent.RemoveCommand(cmd)
	cmd.HelpFunc()(cmd, nil)
	_ = cmd.UsageFunc()(cmd)
	return cmd.OutOrStdout() != io.Writer(&out) || cmd.ErrOrStderr() != io.Writer(&errOut) || !helped || !used
}

// isWord reports whether s can name a command or a subcommand: one
// word, starting with a letter, that is no command of every tree.
func isWord(s string) bool {
	if s == "" || slices.Contains(Builtins, s) || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
	})
}

// Commands returns the registered definitions of the binary, sorted by
// their command line.
func Commands(binary string) []Command { return commands.list(binary) }

// list returns the definitions of the binary, sorted by command line.
func (r *registry) list(binary string) []Command {
	var out []Command
	for _, c := range r.cmds {
		if c.Binary == binary {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b Command) int { return strings.Compare(a.Line(), b.Line()) })
	return out
}
