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
// test that does must not run in parallel with another. stdin is read
// once per run: procgroup.Run does not wait for the copy of a reader
// that is not a file, so a run must not hand the next one a reader that
// copy may still be reading.
var (
	stdin   = func() io.Reader { return os.Stdin }
	environ = os.Environ
	exit    = os.Exit
)

var _ = cli.Register(func() cli.Command {
	// A failing run replaces its record, so a second run leaves the
	// state as the first did, but for the time.
	c := cli.Define(cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name, c.Sub = "julieta", "hooks", "run"
	c.New = func(env *cli.Env) *cobra.Command {
		cmd := &cobra.Command{
			Use:   "run <hook> [args]",
			Short: "run the chain of a git hook: the system hook, julieta's action, .githooks/<hook>, then the local hook",
			Example: "julieta hooks run pre-commit\n" +
				"julieta hooks run commit-msg .git/COMMIT_EDITMSG",
			Long: "Git runs this through the scripts of the dispatcher, which pass the hook name first and then the hook's own arguments.\n" +
				"A flag before the hook name, such as --help, is read by julieta; whatever follows the name is the hook's.\n\n" +
				"The stages, in order: system, the hook of the system core.hooksPath that julieta setup recorded before it set the global one " +
				"(an absolute path other than the dispatcher's own); julieta, julieta's own action; tracked, the repo's .githooks/<hook>; " +
				"and local, $GIT_DIR/hooks/<hook>. A hook stage runs only when its file is executable, and the first hook that runs gets the stdin.\n\n" +
				"The stages run with pipes for standard input, output and error, not the terminal, and each in a process group of its own. " +
				"A job a hook leaves running in the background is ended when the hook ends, which plain git does not do. " +
				"Their output goes through the CLI writer, so terminal controls in it are escaped.",
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
	// The system hooks path setup recorded: the dispatcher starts no
	// git to read the system config. A record it cannot read fails the
	// hook rather than skip the sandbox's own hook unseen.
	system, err := hooks.ReadSystem(setup.StateDir(home))
	if err != nil {
		return fmt.Errorf("the recorded system hooks path: %w", err)
	}
	status, err := hooks.Run(cmd.Context(), hooks.Options{
		Hook: hook, Args: args, Dir: dir, Env: environ(), Stdin: stdin(),
		Stdout: env.Stdout, Stderr: env.Stderr,
		StateDir: filepath.Join(setup.StateDir(home), failuresDir), Now: env.Now,
		Actions:     actions.For(hook),
		ActionError: func(err error) int { return cli.Report(env, err) },
		System:      system, Dispatcher: hooks.DispatcherDir(home),
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
