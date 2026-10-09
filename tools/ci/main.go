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
//
// It exits 0 when every check passes, 1 when a check fails, and 2 when
// it could not run a check at all. Inside fast and all, hygiene or
// workflows that cannot judge the commit it read (a denylist that does
// not parse, say) is a failed check, exit 1. Started through "go run", every
// status other than 0 reaches the caller as 1, with the real one in the
// "exit status" line that go prints; a built binary returns it as is.
// A run that SIGINT or SIGTERM cut short prints one "interrupted" line
// and exits 128 plus the signal number, even after a failure.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses. A run that SIGINT or SIGTERM cut short exits with
// 128 plus the signal number (130 and 143), see finish.
const (
	exitOK    = 0 // every check passed
	exitFail  = 1 // a check found something
	exitError = 2 // a check could not run, or the command line is wrong
	// exitSignalBase is added to the number of the signal that ended a run.
	exitSignalBase = 128
)

const usage = `usage:
  go run ./tools/ci all
  go run ./tools/ci fast [<remote> <url>]
  go run ./tools/ci setup
  go run ./tools/ci coverage <profile>
  go run ./tools/ci workflows
  go run ./tools/ci hygiene [--file <path>]
  go run ./tools/ci hygiene add
  go run ./tools/ci generated
  go run ./tools/ci generate
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
	os.Exit(runSignalled(e, os.Args[1:]))
}

// runSignalled runs the command with a handler for SIGINT and SIGTERM.
// The first signal cancels the context of the run. A step runs in a
// process group of its own, which a terminal's interrupt does not
// reach; cancelling the context kills it. The handler then steps
// aside, so a second signal during the drain ends the process at once.
func runSignalled(e env, args []string) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)
	caught := make(chan syscall.Signal, 1)
	go func() {
		select {
		case sig := <-sigs:
			caught <- sig.(syscall.Signal)
			cancel()
			signal.Stop(sigs)
		case <-ctx.Done():
		}
	}()
	status := run(ctx, e, args)
	select {
	case sig := <-caught:
		return finish(e.stderr, status, sig)
	default:
		return finish(e.stderr, status, 0)
	}
}

// finish gives the exit status of a run that ended with status, after
// the signal sig cut it short (0 when none did). An interrupted run
// says so in one line and exits with 128 plus the signal number, even
// when a check had already failed (1) or a refusal (2) happened: the
// signal wins. A run that passed (0) stays 0, and a run that no signal
// touched keeps its status.
func finish(w io.Writer, status int, sig syscall.Signal) int {
	if sig == 0 || status == exitOK {
		return status
	}
	say(w, "ci: interrupted\n")
	return exitSignalBase + int(sig)
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
	case "generated":
		ok, err = runGenerated(ctx, e, args[1:])
	case "generate":
		ok, err = runGenerate(ctx, e, args[1:])
	default:
		err = fmt.Errorf("unknown subcommand %q\n%s", args[0], usage)
	}
	switch {
	case err != nil:
		say(e.stderr, "ci: %s\n", safeText(strings.TrimSpace(err.Error())))
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
// neutral returns line so that a CI runner does not read it as a workflow
// command: one that starts, after any blanks, with "::" or "##[" gets "./"
// in front. The runner trims the blanks before it looks, so a blank is not
// a guard. It is the one guard of every line that names repository
// content: finding.String and safeText both call it.
func neutral(line string) string {
	if t := strings.TrimLeft(line, " \t"); strings.HasPrefix(t, "::") || strings.HasPrefix(t, "##[") {
		return "./" + line
	}
	return line
}

// safeText returns text that may hold repository content, such as the
// output of a generator or a path in an error, so that it is safe to
// print: each control character but the tab is written as an escape, a
// line that is not UTF-8 is quoted, and neutral guards each line.
func safeText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !utf8.ValidString(line) {
			line = strconv.QuoteToASCII(line)
		}
		var b strings.Builder
		for _, r := range line {
			if unicode.IsControl(r) && r != '\t' {
				q := strconv.QuoteToASCII(string(r))
				b.WriteString(q[1 : len(q)-1])
				continue
			}
			b.WriteRune(r)
		}
		lines[i] = neutral(b.String())
	}
	return strings.Join(lines, "\n")
}

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
