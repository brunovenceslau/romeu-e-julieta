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
//	go run ./tools/ci coverage <profile>      the thresholds of S9 over a cover profile
//	go run ./tools/ci workflows               the workflow grammar over the tree at HEAD
//	go run ./tools/ci hygiene                 the hygiene rules over the tree at HEAD
//	go run ./tools/ci hygiene --file <path>   the name matcher over one file
//	go run ./tools/ci hygiene add             add a denylist entry, from a terminal
//	go run ./tools/ci generated               go generate ./... changes no generated file
//	go run ./tools/ci generate                write the generated files (what go generate runs)
//	go run ./tools/ci reqs                    the requirement ids of 12 12.11 over the tree at HEAD
//	go run ./tools/ci reqs allocate           number the REQ-new placeholders of the working tree
//
// It exits 0 when every check passes, 1 when a check fails, and 2 when
// it could not run a check at all. Inside fast and all, hygiene or
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
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/coverage"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/workflows"
)

func init() {
	check.RegisterCommand(check.Command{Name: "all", Usage: []string{"all"}, Run: runAll})
	check.RegisterCommand(check.Command{Name: "fast", Usage: []string{"fast [<remote> <url>]"}, Run: runFast})
	check.RegisterCommand(check.Command{Name: "setup", Usage: []string{"setup"}, Run: runSetup})
	check.RegisterCommand(check.Command{Name: "coverage", Usage: []string{"coverage <profile>"}, Run: coverage.Run})
	check.RegisterCommand(check.Command{Name: "workflows", Usage: []string{"workflows"}, Run: workflows.Run})
	check.RegisterCommand(check.Command{Name: "hygiene", Usage: []string{"hygiene [--file <path>]", "hygiene add"}, Run: runHygiene})
	check.RegisterCommand(check.Command{Name: "generated", Usage: []string{"generated"}, Run: runGenerated})
	check.RegisterCommand(check.Command{Name: "generate", Usage: []string{"generate"}, Run: runGenerate})
	check.RegisterCommand(check.Command{Name: "reqs", Usage: []string{"reqs [allocate]"}, Run: runReqs})
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

// runHygiene parses the arguments of the hygiene subcommand and prints
// what it finds.
func runHygiene(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) > 0 && args[0] == "add" {
		return true, add(ctx, e, args[1:])
	}
	fs := flag.NewFlagSet("hygiene", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "", "check one file with the name matcher")
	if err := fs.Parse(args); err != nil {
		return false, fmt.Errorf("hygiene: %w\n%s", err, check.Usage())
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("hygiene: unexpected argument %q\n%s", fs.Arg(0), check.Usage())
	}
	var findings []check.Finding
	var err error
	if *file != "" {
		findings, err = hygieneFile(ctx, e.Repo(), *file)
	} else {
		findings, err = hygiene(ctx, e.Repo(), "HEAD")
	}
	if err != nil {
		return false, err
	}
	for _, f := range findings {
		check.Say(e.Stdout, "%s\n", f)
	}
	check.Say(e.Stdout, "%s\n", check.Summary("hygiene", len(findings)))
	return len(findings) == 0, nil
}
