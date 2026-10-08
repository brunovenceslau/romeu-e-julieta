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
//
// It exits 0 when every check passes, 1 when a check fails, and 2 when
// it could not run a check at all. Started through "go run", every
// status other than 0 reaches the caller as 1, with the real one in the
// "exit status" line that go prints; a built binary returns it as is.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses.
const (
	exitOK    = 0 // every check passed
	exitFail  = 1 // a check found something
	exitError = 2 // a check could not run, or the command line is wrong
)

const usage = `usage:
  go run ./tools/ci all
  go run ./tools/ci fast [<remote> <url>]
  go run ./tools/ci setup
  go run ./tools/ci coverage <profile>
  go run ./tools/ci workflows
  go run ./tools/ci hygiene [--file <path>]
  go run ./tools/ci hygiene add
`

// env is what a run reads and writes. Tests fill it with a fixture;
// main fills it with the process's own.
type env struct {
	dir    string    // where the command was started
	gitEnv []string  // the environment of git; nil means this process's
	stdin  io.Reader // the hook's input, or the terminal of "hygiene add"
	stdout io.Writer
	stderr io.Writer
	steps  []step // the command steps of fast or all; nil means fastSteps or allSteps with the tools of resolveLintTools
	// profile is the cover profile that coverage reads in all when
	// steps is set; with the real steps, all writes one of its own.
	profile string
	// machine returns what "uname -m" prints; nil means the command.
	machine func(context.Context) string
	// translated reports whether the process runs under Rosetta; nil
	// means procTranslated.
	translated func(context.Context) bool
}

func main() {
	e := env{dir: ".", stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	// A step runs in a process group of its own, which a terminal's
	// interrupt does not reach; cancelling the context kills it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, e, os.Args[1:])
	stop()
	os.Exit(code)
}

// run dispatches one subcommand and returns the exit status.
func run(ctx context.Context, e env, args []string) int {
	if len(args) == 0 {
		say(e.stderr, "%s", usage)
		return exitError
	}
	var ok bool
	var err error
	switch args[0] {
	case "all":
		ok, err = runAll(ctx, e, args[1:])
	case "fast":
		ok, err = runFast(ctx, e, args[1:])
	case "setup":
		ok, err = runSetup(ctx, e, args[1:])
	case "coverage":
		ok, err = runCoverage(ctx, e, args[1:])
	case "workflows":
		ok, err = runWorkflows(ctx, e, args[1:])
	case "hygiene":
		ok, err = runHygiene(ctx, e, args[1:])
	default:
		err = fmt.Errorf("unknown subcommand %q\n%s", args[0], usage)
	}
	switch {
	case err != nil:
		say(e.stderr, "ci: %v\n", strings.TrimSpace(err.Error()))
		return exitError
	case !ok:
		return exitFail
	}
	return exitOK
}

// runHygiene parses the arguments of the hygiene subcommand and prints
// what it finds.
func runHygiene(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) > 0 && args[0] == "add" {
		return true, add(ctx, e, args[1:])
	}
	fs := flag.NewFlagSet("hygiene", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "", "check one file with the name matcher")
	if err := fs.Parse(args); err != nil {
		return false, fmt.Errorf("hygiene: %w\n%s", err, usage)
	}
	if fs.NArg() > 0 {
		return false, fmt.Errorf("hygiene: unexpected argument %q\n%s", fs.Arg(0), usage)
	}
	var findings []finding
	var err error
	if *file != "" {
		findings, err = hygieneFile(ctx, e.repo(), *file)
	} else {
		findings, err = hygiene(ctx, e.repo(), "HEAD")
	}
	if err != nil {
		return false, err
	}
	for _, f := range findings {
		say(e.stdout, "%s\n", f)
	}
	say(e.stdout, "%s\n", summary("hygiene", len(findings)))
	return len(findings) == 0, nil
}

// say writes one piece of output. A write to the terminal or to a pipe
// that fails has nowhere left to be reported, so the error is dropped
// here, in one place.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func summary(check string, findings int) string {
	if findings == 0 {
		return check + ": ok"
	}
	return fmt.Sprintf("%s: %d finding(s)", check, findings)
}

func (e env) repo() git.Repo {
	return git.Repo{Dir: e.dir, Env: e.gitEnv}
}

// repoRoot returns the top of the working tree the command runs in.
// The steps of fast run there, and "hygiene add" writes there.
func repoRoot(ctx context.Context, e env) (string, error) {
	out, err := e.repo().Run(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}
