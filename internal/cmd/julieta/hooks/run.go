// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks/actions"
)

// failuresDir is the directory of julieta state that holds the
// recorded failures of the hooks (REQ-0154, REQ-0675).
const failuresDir = "hook-failures"

// The process the command runs in: the stdin the hook's first repo
// stage reads, the environment the stages get, and the exit that ends
// the process with the status of a stage. Tests replace them, and a
// test that does must not run in parallel with another.
var (
	stdin   io.Reader = os.Stdin
	environ           = os.Environ
	exit              = os.Exit
)

var _ = cli.Register(func() cli.Command {
	// A failing run replaces its record, so a second run leaves the
	// state as the first did, but for the time.
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name, c.Sub = "julieta", "hooks", "run"
	c.New = func(env *cli.Env) *cobra.Command {
		cmd := &cobra.Command{
			Use:   "run <hook> [args]",
			Short: "run the chain of a git hook: julieta's action, .githooks/<hook>, then the local hook",
			Example: "julieta hooks run pre-commit\n" +
				"julieta hooks run commit-msg .git/COMMIT_EDITMSG",
			Args: cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return run(cmd, env, args[0], args[1:])
			},
		}
		// What follows the hook is the hook's own arguments, flags or
		// not: git's hooks get paths, names and object ids, and the
		// script that git runs passes them on as they are.
		cmd.Flags().SetInterspersed(false)
		return cmd
	}
	return c
}())

// run runs the chain of hook with args and ends the process with the
// status of the stage that stopped it (04 4.3, REQ-0408): the stage
// prints its own line, so the failure is not returned as an error,
// which would print a second one.
func run(cmd *cobra.Command, env *cli.Env, hook string, args []string) error {
	if !slices.Contains(hooks.Names, hook) {
		return usage(cmd, fmt.Sprintf("%q is not a hook the dispatcher runs; they are %s", hook, strings.Join(hooks.Names, ", ")))
	}
	home, err := setup.Home()
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	status, err := hooks.Run(cmd.Context(), hooks.Options{
		Hook: hook, Args: args, Dir: dir, Env: environ(), Stdin: stdin,
		Stdout: env.Stdout, Stderr: env.Stderr,
		StateDir: filepath.Join(setup.StateDir(home), failuresDir), Now: env.Now,
		Actions:     actions.For(hook),
		ActionError: func(err error) int { return cli.Report(env, err) },
	})
	if err != nil {
		return err
	}
	if status != 0 {
		exit(status)
	}
	return nil
}

// usage returns the error usage (RJ-349) for an argument that cmd
// refuses (04 4.1, Parsing; exit 2), with the hint of the help of the
// command.
func usage(cmd *cobra.Command, message string) error {
	e := cli.Usage.New(message)
	binary, command, _ := strings.Cut(cmd.CommandPath(), " ")
	e.Fill = map[string]string{"<binary>": binary, " <command>": ""}
	if command != "" {
		e.Fill[" <command>"] = " " + command
	}
	return e
}
