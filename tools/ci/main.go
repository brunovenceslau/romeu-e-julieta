// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command ci is the one entry point for the checks of this repository:
// the pre-push hook, a developer and the hosted workflow all run this
// code, so there is no second list of checks to keep in step (10 10.2).
//
//	go run ./tools/ci all                     every check, as the hosted workflow runs it
//	go run ./tools/ci fast [<remote> <url>]   the fast checks; with the
//	                                          hook's arguments, the pushed range too
//	go run ./tools/ci setup                   trust mise.toml, install the tools, fill the module cache
//	go run ./tools/ci generated               go generate ./... changes no generated file
//	go run ./tools/ci generate                write the generated files (what go generate runs)
//	go run ./tools/ci hygiene                 the hygiene rules over the tree at HEAD
//	go run ./tools/ci hygiene --file <path>   the name matcher over one file
//	go run ./tools/ci hygiene add             add a denylist entry, from a terminal
//	go run ./tools/ci workflows               the workflow grammar over the tree at HEAD
//	go run ./tools/ci reqs                    the requirement ids of 12 12.11 over the tree at HEAD
//	go run ./tools/ci reqs allocate           number the REQ-new placeholders of the working tree
//	go run ./tools/ci coverage <profile>      the thresholds of S9 over a cover profile
//
// It exits 0 when every check passes, 1 when a check fails, 2 when it
// could not run a check at all, and 3 when every check of all that
// failed is an infrastructure error: a precondition of the container
// e2e that could not be met (REQ-0798 of 10 10.1). Inside fast and all, hygiene or
// workflows that cannot judge the commit it read (a denylist that does
// not parse, say) is a failed check, exit 1. Started through "go run", every
// status other than 0 reaches the caller as 1, with the real one in the
// "exit status" line that go prints; a built binary returns it as is.
// A run that SIGINT or SIGTERM cut short prints one "interrupted" line
// and exits 128 plus the signal number, even after a failure: the
// signal rule of internal/procgroup (04 4.1, Signals), which runs every
// subprocess of this command too.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/setup"
)

func init() {
	check.RegisterCommand(check.Command{Name: "all", Usage: []string{"all"}, Run: runAll})
	check.RegisterCommand(check.Command{Name: "fast", Usage: []string{"fast [<remote> <url>]"}, Run: runFast})
	check.RegisterCommand(check.Command{Name: "setup", Usage: []string{"setup"}, Run: setup.Run})
}

func main() {
	e := check.Env{Dir: ".", Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(runMain(e, os.Args[1:]))
}

// runMain runs the command under the signal rule of procgroup.Main: the
// first SIGINT or SIGTERM cancels the context of the run and kills the
// process group of every step that runs under it, a second one before
// the handler steps aside is absorbed, and the status is 128 plus the
// signal number, even after a failure.
func runMain(e check.Env, args []string) int {
	return procgroup.Main("ci", e.Stderr, func(ctx context.Context) int {
		return run(ctx, e, args)
	})
}

// run dispatches one subcommand, from the registry, and returns the
// exit status.
func run(ctx context.Context, e check.Env, args []string) int {
	if len(args) == 0 {
		check.Say(e.Stderr, "%s", check.Usage())
		return check.ExitError
	}
	cmd, found := check.Lookup(args[0])
	if !found {
		return check.Status(e.Stderr, false, fmt.Errorf("unknown subcommand %q\n%s", args[0], check.Usage()))
	}
	ok, err := cmd.Run(ctx, e, args[1:])
	return check.Status(e.Stderr, ok, err)
}
