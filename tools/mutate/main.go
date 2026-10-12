// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command mutate runs mutation tests: it makes each mutant of a mutants
// file, one at a time, and tells whether the tests catch it.
//
//	go run ./tools/mutate [-worktree] <mutants-file>
//
// The mutants file is YAML, and a key it does not know is an error:
//
//	mutants:
//	  - id: keep-the-last-line
//	    file: tools/ci/hygiene.go
//	    old: "i < len(lines)"
//	    new: "i < len(lines)-1"
//	    package: ./tools/ci
//	    run: ^TestHygiene$
//
// Each mutant replaces the one occurrence of old in file (a path from
// the top of the repository) with new, then runs "go test" on package
// with "-run" set to run. Old must occur exactly once in the file.
//
// The run never writes to the working tree, the index, a ref or the
// stash of the repository it starts in. It clones the commit HEAD into
// a temporary directory ("git clone --no-local") and works there only.
// Uncommitted changes are not part of that commit, and the run says so
// when there are some. With -worktree they are: the run records the
// working tree, the tracked files as they are on disk and the untracked
// files that are not ignored, as a commit on top of HEAD through an
// index of its own, and runs that commit instead (snapshot). That
// writes objects to the repository, and nothing else. It first runs each distinct test target
// once with no mutant: the baseline. A target that fails then, or that
// runs no test, stops the run, since such a target can kill no mutant.
// Then, for each mutant, it writes the mutated file, builds the test
// binaries ("go test -c") of the target and of the package of the file
// (when the file's directory holds one), runs the target, and writes
// the file back from its blob in git. It removes the clone at the end,
// after an interrupt too.
//
// A mutant is killed when the target fails, survived when it passes,
// and invalid when old does not occur exactly once, when the file is
// not one blob of HEAD as it is checked out, or when a package does not
// build: a mutant that breaks the build tells nothing about the tests.
// Vet is off ("-vet=off"), so only a test kills a mutant. Each mutant
// prints one line, "<id> killed|survived|invalid <reason>", and a
// summary line follows.
//
// The go command is the one that built this tool, which "go run" takes
// from the search path, the pinned go through the mise shim. Mise would
// refuse the mise.toml of a fresh clone as untrusted, so the tool asks
// "go env GOROOT" once, at the top of the source tree, checks that the
// go found there is the version this tool was built with, and runs
// <GOROOT>/bin/go in the clone directly, with GOTOOLCHAIN=local, so no
// other go is fetched or run. Its environment is built from nothing, as
// the steps of tools/ci build theirs: a few variables pass through
// (passThrough), and the rest are fixed (fixedGoEnv), GOPROXY=off among
// them, so the module cache must already hold the modules of go.sum.
//
// It exits 0 when every mutant is killed, 1 when one survived or is
// invalid, and 2 when the run could not proceed: the command line or
// the mutants file is wrong, the clone failed, or a baseline is red. A
// run that SIGINT or SIGTERM cut short prints one "interrupted" line
// and exits 128 plus the signal number. Started through "go run", every
// status other than 0 reaches the caller as 1, with the real one in the
// "exit status" line that go prints.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses. A run that SIGINT or SIGTERM cut short exits with
// 128 plus the signal number (130 and 143), see finish.
const (
	exitOK    = 0 // every mutant was killed
	exitFail  = 1 // a mutant survived or is invalid
	exitError = 2 // the run could not proceed
	// exitSignalBase is added to the number of the signal that ended a run.
	exitSignalBase = 128
)

const usage = `usage:
  go run ./tools/mutate [-worktree] <mutants-file>
`

// errInterrupted ends a run whose context a signal cancelled.
var errInterrupted = errors.New("interrupted")

// env is what a run reads and writes. Tests fill it with a fixture;
// main fills it with the process's own.
type env struct {
	dir    string   // where the command was started
	gitEnv []string // the environment of git; nil means this process's
	// goBin is the go command to run in the clone; empty means the one
	// resolveGo finds.
	goBin string
	// tmpDir is where the temporary directory of the clone is made;
	// empty means the default directory for temporary files.
	tmpDir string
	stdout io.Writer
	stderr io.Writer
}

func main() {
	e := env{dir: ".", stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(runSignalled(e, os.Args[1:]))
}

// runSignalled runs the command with a handler for SIGINT and SIGTERM.
// The first signal cancels the context of the run, which kills the
// process group of the go command that runs (killWithGroup); the run
// then writes the mutated file back and removes the clone. The handler
// steps aside, so a second signal ends the process at once.
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
// says so in one line and exits with 128 plus the signal number.
func finish(w io.Writer, status int, sig syscall.Signal) int {
	if sig == 0 {
		return status
	}
	say(w, "mutate: interrupted\n")
	return exitSignalBase + int(sig)
}

// run reads the mutants file, runs the mutants, and returns the exit
// status.
func run(ctx context.Context, e env, args []string) int {
	fs := flag.NewFlagSet("mutate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	worktree := fs.Bool("worktree", false, "run a snapshot of the working tree instead of the commit HEAD")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 || strings.HasPrefix(fs.Arg(0), "-") {
		say(e.stderr, "%s", usage)
		return exitError
	}
	ok, err := mutate(ctx, e, fs.Arg(0), *worktree)
	switch {
	case errors.Is(err, errInterrupted):
		return exitError
	case err != nil:
		say(e.stderr, "mutate: %s\n", git.SafeLines(strings.TrimSpace(err.Error())))
		return exitError
	case !ok:
		return exitFail
	}
	return exitOK
}

// say writes one piece of output. A write that fails has nowhere left
// to be reported, so the error is dropped here, in one place.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
