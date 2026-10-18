// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// The command signaltest of this test binary alone: it sends its own
// process SIGINT and returns the cancellation of its context once the
// handler of procgroup.Main has cancelled it.
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly)
	c.Binary, c.Name = "julieta", "signaltest"
	c.New = func(*cli.Env) *cobra.Command {
		return &cobra.Command{
			Use:   "signaltest",
			Short: "a command of the tests",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
					return err
				}
				<-c.Context().Done()
				return c.Context().Err()
			},
		}
	}
	return c
}())

// TestStartVersion runs julieta version through the composition root: the
// version stamp of a build with no -ldflags, and exit 0.
func TestStartVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	assert.Equal(t, 0, start([]string{"version"}, &stdout, &stderr))
	assert.Empty(t, stderr.String())
	assert.True(t, strings.HasPrefix(stdout.String(), "version   devel\ncommit    unknown\ndate      unknown\n"), stdout.String())
	assert.Contains(t, stdout.String(), "\nprotocol  1\n")
}

// TestStartInterrupted sends SIGINT while a command runs: the handler
// cancels its context, cli.Main prints nothing for the cancellation,
// and the binary exits 130 with the one line of an interrupted command
// (04 4.1, Signals). version returns the same cancellation when its
// context is done (TestVersionInterrupted).
func TestStartInterrupted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	assert.Equal(t, 130, start([]string{"signaltest"}, &stdout, &stderr))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "julieta: interrupted\n", stderr.String())
}
