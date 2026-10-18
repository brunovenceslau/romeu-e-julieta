// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// TestCompleteHoldsREQ1245 holds a completion function to the bounds of
// 04 4.1, Completion: only the valid names, at most 1000, no file names
// in their place, and nothing once its time has run out, whether or not
// the function has returned.
func TestCompleteHoldsREQ1245(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	cmd.SetContext(t.Context())
	many := func(context.Context, []string, string) []string {
		names := []string{"Bad Name"}
		for i := range 1500 {
			names = append(names, fmt.Sprintf("name-%04d", i))
		}
		return names
	}
	valid := func(s string) bool { return !strings.ContainsAny(s, " ") }
	got, directive := bound(Complete(many, valid))(cmd, nil, "")
	assert.Len(t, got, maxCompletions)
	assert.Equal(t, "name-0000", got[0], "an invalid name is left out")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Panics(t, func() { Complete(many, nil) }, "a validator is required")

	old := completionTimeout
	completionTimeout = 10 * time.Millisecond
	defer func() { completionTimeout = old }()
	release := make(chan struct{})
	defer close(release)
	slow := func(context.Context, []string, string) []string {
		<-release
		return []string{"late"}
	}
	start := time.Now()
	got, directive = bound(Complete(slow, valid))(cmd, nil, "")
	assert.Empty(t, got)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Less(t, time.Since(start), 5*time.Second, "bound returns once its time has run out")
}

// TestTreeBoundsEveryCompletion finds the completion function of a
// command bounded by the tree, though the command set it without
// Complete: it returns at most 1000 names.
func TestTreeBoundsEveryCompletion(t *testing.T) {
	r := registry{}
	r.register(func() Command {
		c := Define(PreflightNone, LockNone, AppendsNothing(), SnapshotNone, ReadsOnly)
		c.Binary, c.Name = "julieta", "unbounded"
		c.New = func(*Env) *cobra.Command {
			return &cobra.Command{
				Use: "unbounded", Args: cobra.ArbitraryArgs, RunE: func(*cobra.Command, []string) error { return nil },
				ValidArgsFunction: func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
					return make([]cobra.Completion, 2000), cobra.ShellCompDirectiveDefault
				},
			}
		}
		return c
	}())
	saved := commands
	commands = r
	defer func() { commands = saved }()
	tree := Tree(&Env{Binary: "julieta"})
	c, _, err := tree.Find([]string{"unbounded"})
	require.NoError(t, err)
	c.SetContext(t.Context())
	got, directive := c.ValidArgsFunction(c, nil, "")
	assert.Len(t, got, maxCompletions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

// TestCompletionWriter escapes the name and the description of each line
// of the __complete output and keeps the tab between them and the line
// feeds.
func TestCompletionWriter(t *testing.T) {
	var out bytes.Buffer
	in := "na\x1bme\tde\tsc\u202e\n:4\n"
	n, err := completionWriter{&out}.Write([]byte(in))
	require.NoError(t, err)
	assert.Equal(t, len(in), n)
	assert.Equal(t, termsafe.Escape("na\x1bme")+"\t"+termsafe.Escape("de\tsc\u202e")+"\n:4\n", out.String())
}
