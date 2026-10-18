// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// leaf returns a New whose command is named use, takes no arguments and
// does nothing, then has edit applied.
func leaf(use string, edit func(*cobra.Command)) func(*cli.Env) *cobra.Command {
	return func(*cli.Env) *cobra.Command {
		c := &cobra.Command{Use: use, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return nil }}
		if edit != nil {
			edit(c)
		}
		return c
	}
}

// whole returns a definition of julieta name with every field set, for
// the cases that unset or break one.
func whole(name string) cli.Command {
	c := cli.Define(cli.PreflightRuns, cli.LockTaken, cli.Appends("a record"), cli.SnapshotNone, cli.AppendsEachRun)
	c.Binary, c.Name, c.New = "julieta", name, leaf(name, nil)
	return c
}

// TestRegisterRefusesAnUnsetField panics at registration on each
// side-effect field left unset.
func TestRegisterRefusesAnUnsetField(t *testing.T) {
	tests := []struct {
		name string
		def  cli.Command
		want string
	}{
		{"preflight", cli.Define(cli.PreflightUnset, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly), "the preflight field"},
		{"lock", cli.Define(cli.PreflightNone, cli.LockUnset, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly), "the lock field"},
		{"appending", cli.Define(cli.PreflightNone, cli.LockNone, cli.Appending{}, cli.SnapshotNone, cli.ReadsOnly), "the appending field"},
		{"snapshot", cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotUnset, cli.ReadsOnly), "the snapshot field"},
		{"idempotency", cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.IdempotencyUnset), "the idempotency field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := "unset-" + tt.name
			tt.def.Binary, tt.def.Name, tt.def.New = "romeu", name, leaf(name, nil)
			assert.PanicsWithError(t, "cli: the definition of romeu \""+name+"\" lacks "+tt.want, func() { cli.Register(tt.def) })
		})
	}
}

// TestRegisterRefusesABrokenDefinition panics at registration on a
// definition that could not join a tree whole.
func TestRegisterRefusesABrokenDefinition(t *testing.T) {
	tests := []struct {
		name string
		edit func(*cli.Command)
		want string
	}{
		{"another binary", func(c *cli.Command) { c.Binary = "other" }, "a binary of romeu or julieta"},
		{"no name", func(c *cli.Command) { c.Name = "" }, "a name of one word"},
		{"the name help", func(c *cli.Command) { c.Name, c.New = "help", leaf("help", nil) }, "a name of one word"},
		{"the name completion", func(c *cli.Command) { c.Name, c.New = "completion", leaf("completion", nil) }, "a name of one word"},
		{"two words", func(c *cli.Command) { c.Name = "two words" }, "a name of one word"},
		{"a capital", func(c *cli.Command) { c.Name = "Broken" }, "a name of one word"},
		{"a bad subcommand", func(c *cli.Command) { c.Sub = "-x" }, "a subcommand of one word"},
		{"no New", func(c *cli.Command) { c.New = nil }, "New"},
		{"a New that returns nil", func(c *cli.Command) { c.New = func(*cli.Env) *cobra.Command { return nil } }, "a New that returns a command"},
		{"another Use", func(c *cli.Command) { c.New = leaf("other", nil) }, `a Use that starts with "broken"`},
		{"no Args", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.Args = nil }) }, "an Args validator"},
		{"no RunE", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.RunE = nil }) }, "a RunE"},
		{"a Run", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.Run = func(*cobra.Command, []string) {} })
		}, "no run function but RunE"},
		{"a PreRunE", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.PreRunE = func(*cobra.Command, []string) error { return nil } })
		}, "no run function but RunE"},
		{"Deprecated", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.Deprecated = "old" }) }, "no Deprecated or Version text"},
		{"Version", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.Version = "1" }) }, "no Deprecated or Version text"},
		{"a help flag", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.Flags().Bool("help", false, "") })
		}, "no flag help or -h"},
		{"a -h flag", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.Flags().BoolP("host", "h", false, "") })
		}, "no flag help or -h"},
		{"a json flag", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.Flags().Bool("json", false, "") })
		}, "no flag json"},
		{"a persistent json flag", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.PersistentFlags().Bool("json", false, "") })
		}, "no flag json"},
		{"Aliases", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.Aliases = []string{"help"} }) }, "no Aliases or SuggestFor"},
		{"SuggestFor", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.SuggestFor = []string{"x"} }) }, "no Aliases or SuggestFor"},
		{"Hidden", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.Hidden = true }) }, "neither Hidden nor commands of its own"},
		{"a command of its own", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) {
				c.AddCommand(&cobra.Command{Use: "kid", Run: func(*cobra.Command, []string) {}})
			})
		}, "neither Hidden nor commands of its own"},
		{"FParseErrWhitelist", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.FParseErrWhitelist.UnknownFlags = true })
		}, "no FParseErrWhitelist"},
		{"its own stdout", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.SetOut(os.Stdout) }) }, "no output stream, help or usage of its own"},
		{"its own stderr", func(c *cli.Command) { c.New = leaf("broken", func(c *cobra.Command) { c.SetErr(os.Stderr) }) }, "no output stream, help or usage of its own"},
		{"its own help", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.SetHelpFunc(func(*cobra.Command, []string) {}) })
		}, "no output stream, help or usage of its own"},
		{"its own usage", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) { c.SetUsageFunc(func(*cobra.Command) error { return nil }) })
		}, "no output stream, help or usage of its own"},
		{"a flag completion function", func(c *cli.Command) {
			c.New = leaf("broken", func(c *cobra.Command) {
				c.Flags().String("name", "", "")
				_ = c.RegisterFlagCompletionFunc("name", cobra.NoFileCompletions)
			})
		}, "no completion function on the flag name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := whole("broken")
			tt.edit(&c)
			assert.PanicsWithError(t, "cli: the definition of "+c.Binary+" \""+c.Line()+"\" lacks "+wantPrefix(t, c, tt.want), func() { cli.Register(c) })
		})
	}
}

// wantPrefix returns the message Register panics with for c, after
// checking that it holds want, so each case names its reason alone.
func wantPrefix(t *testing.T, c cli.Command, want string) string {
	t.Helper()
	var msg string
	func() {
		defer func() {
			if p := recover(); p != nil {
				err, ok := p.(error)
				require.True(t, ok, "Register panics with an error")
				msg = err.Error()
			}
		}()
		cli.Register(c)
	}()
	_, lacks, ok := strings.Cut(msg, " lacks ")
	require.True(t, ok, "Register panics for %s: %q", c.Line(), msg)
	require.Contains(t, lacks, want)
	return lacks
}

// twice is registered once, as a command registers, for the test below.
var _ = cli.Register(whole("twice"))

// TestRegisterRefusesATwiceRegisteredPath panics on a command path its
// binary already has, and on a command that is both a definition and
// the group of subcommands.
func TestRegisterRefusesATwiceRegisteredPath(t *testing.T) {
	assert.PanicsWithError(t, "cli: julieta twice is registered twice", func() { cli.Register(whole("twice")) })
	sub := whole("twice")
	sub.Sub, sub.New = "one", leaf("one", nil)
	assert.PanicsWithError(t, "cli: julieta twice has subcommands, so it is no definition of its own", func() { cli.Register(sub) })
}

// TestRegistryOrder lists each binary's definitions sorted by command
// line, whatever the order of their registrations.
func TestRegistryOrder(t *testing.T) {
	var lines []string
	for _, c := range cli.Commands("julieta") {
		lines = append(lines, c.Line())
	}
	assert.True(t, slices.IsSorted(lines), "%v", lines)
	assert.Contains(t, lines, "fixturegroup one")
	assert.Less(t, slices.Index(lines, "fixturegroup one"), slices.Index(lines, "fixturegroup two"))
	for _, c := range cli.Commands("romeu") {
		assert.Equal(t, "romeu", c.Binary)
	}
}

// TestTreeIsNewOnEachCall builds two trees of a binary and finds no
// command shared between them (ADR 0015, decision 2).
func TestTreeIsNewOnEachCall(t *testing.T) {
	a, b := cli.Tree(&cli.Env{Binary: "julieta"}), cli.Tree(&cli.Env{Binary: "julieta"})
	assert.NotSame(t, a, b)
	for i, c := range a.Commands() {
		assert.NotSame(t, c, b.Commands()[i], c.Name())
	}
}

// lockList returns the romeu commands that take the lock, from the
// Locking row of 04 4.1: "The commands that take it are ...;".
func lockList(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("../../docs/spec/04-cli.md")
	require.NoError(t, err)
	_, rest, ok := strings.Cut(string(src), "The commands that take it are the ones whose Writes cell in 4.2 is not `nothing`: ")
	require.True(t, ok, "the Locking row of 04 4.1 lists the commands that take the lock")
	list, _, ok := strings.Cut(rest, ";")
	require.True(t, ok)
	var names []string
	for _, m := range regexp.MustCompile("`([a-z-]+)`").FindAllStringSubmatch(list, -1) {
		names = append(names, m[1])
	}
	require.NotEmpty(t, names)
	return names
}

// TestLockField holds the lock field of each definition to the list of
// 04 4.1, Locking: a romeu command takes the lock exactly when the list
// names it; julieta takes no romeu lock.
func TestLockField(t *testing.T) {
	takes := lockList(t)
	assert.Contains(t, takes, "sync")
	for _, binary := range []string{"romeu", "julieta"} {
		for _, c := range cli.Commands(binary) {
			if strings.HasPrefix(c.Name, "unset-") || c.Name == "broken" || c.Name == "twice" {
				// The definitions of the registry tests above.
				continue
			}
			want := cli.LockNone
			if binary == "romeu" && slices.Contains(takes, c.Name) {
				want = cli.LockTaken
			}
			assert.Equal(t, want, c.Effects().Lock, "the lock field of %s %s", binary, c.Line())
		}
	}
}
