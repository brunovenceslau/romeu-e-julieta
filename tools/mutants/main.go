// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Command mutants runs mutation tests: it makes each mutant of a
// mutants file, one at a time, and tells whether the tests catch it.
//
//	go run ./tools/mutants [-worktree] <mutants-file>
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
// with "-run" set to run. Old must occur exactly once in the file,
// overlapping occurrences counted.
//
// The run never writes to the working tree, the index, a ref, the
// configuration or the stash of the repository it starts in. Every git
// command runs without the variables that point git at another
// repository, index or object store (git.CleanEnv), so a hook that
// starts the tool cannot steer it there. The run clones the commit HEAD
// into a temporary directory ("git clone --no-local", with the hooks
// off and the remote removed) and works there only. Uncommitted changes
// are not part of that commit, and the run says so when there are some.
// With -worktree they are: the run records the working tree, the
// tracked files as they are on disk and the untracked files that are
// not ignored, as a commit on top of HEAD through an index of its own,
// and runs that commit instead (snapshot). That writes objects to the
// repository, and nothing else. As any "git add" does, it runs a clean
// filter that the repository's own configuration sets for a file.
//
// A tree that the clone cannot hold as the source has it is refused: a
// submodule (or an embedded repository that -worktree would record as
// one), and, with -worktree, an index entry marked skip-worktree or
// assume-unchanged, as a sparse checkout makes. A file that a filter of
// .gitattributes (LFS, a clean or smudge filter, an end-of-line rule)
// changes on checkout is not its blob on disk, and a mutant of it is
// invalid.
//
// It first builds, with no mutant, the packages each mutant builds (the
// target, and the package of the file, see below), and runs each
// distinct test target once: the baseline. A package that does not
// build, or a target that fails or runs no test, then stops the run,
// since every mutant would be invalid or no mutant could be killed. The baseline's
// time sets the -timeout of the mutant runs (mutantTimeout). Then, for
// each mutant, it writes the mutated file, builds the test binaries of
// the target and of the package of the file (when the file's directory
// holds one), runs the target, and writes the file back from its blob
// in git. It removes the clone at the end, after an interrupt too.
//
// A mutant is killed when the target fails, a timeout or a panic
// included, and survived when it passes with as many tests as the
// baseline. It is invalid when old does not occur exactly once, when
// the file is not one blob of the commit as it is checked out, when a
// package does not build (a mutant that breaks the build tells nothing
// about the tests), or when the target passes with fewer tests than the
// baseline (a mutant that renames or skips its own test). Vet is off
// ("-vet=off"), so only a test kills a mutant. Each mutant prints one
// line, "<id> killed|survived|invalid <reason>", and a summary line
// follows.
//
// The go command is the one that built this tool, which "go run" takes
// from the search path, the pinned go through the mise shim. Mise would
// refuse the mise.toml of a fresh clone as untrusted, so the tool asks
// "go env GOROOT" once, at the top of the source tree, checks that the
// go found there is the version this tool was built with, and runs
// <GOROOT>/bin/go in the clone directly. Its environment is built from
// nothing, from the lists tools/ci builds its steps with (goenv):
// goenv.PassThrough, goenv.Pin, goenv.Build, and GOPROXY=off as the
// steps of fast add it, so the module cache must already hold the
// modules of go.sum. GOTMPDIR is in the run's temporary directory, so
// the go command's own temporary files go with it.
//
// It exits 0 when every mutant is killed, 1 when one survived or is
// invalid, and 2 when the run could not proceed: the command line or
// the mutants file is wrong, the clone failed, or a baseline is red. A
// run that SIGINT or SIGTERM cut short prints one "interrupted" line and
// exits 128 plus the signal number: the signal rule of
// internal/procgroup (04 4.1, Signals), which runs every subprocess of
// this command too. A SIGHUP takes its default action and ends the
// process at once, leaving the clone in the directory for temporary
// files and a go command that runs, as 04 4.1, Signals, says of every
// signal the handler does not take. Started through "go run", every
// status other than 0 reaches the caller as 1, with the real one in the
// "exit status" line that go prints.
//
// A test earns trust when a wrong line makes it fail. Proving that by
// hand means editing a tracked file, running the tests and putting the
// file back, and the putting back is where work gets lost: a checkout
// or a stash in a shared working tree reaches further than the one
// file. The tool does the whole loop in a throwaway clone, so nobody
// has to edit tracked files to mutate them. -worktree exists so that
// new tests can be checked before the commit that adds them.
//
// It is a separate command from the "tools/ci mutate" gate that task
// T007 of docs/plan.md plans, and the two answer different questions.
// That gate is a check that runs on every pull request it applies to,
// over a fixed set of guards declared in the code: it builds each
// guard's mutate_I<n> stub, which disables the guard through a build
// tag, and requires a tagged test to fail. It lives in tools/ci, on the
// checks ask-first surface. This tool is run by a person or an agent
// while writing tests, over mutants listed in a file for the change at
// hand. It gates nothing, so it lives outside tools/ci, and a change to
// it needs no approval of that surface. Keeping the names apart keeps
// the two from being taken for one another.
//
// If the gate of task T007 needs to build or run code away from the
// working tree, the clone and snapshot engine of this tool is there to
// reuse: the clone with hooks off and its remote removed, the snapshot
// through a private index, and the go environment built from the lists
// of tools/ci/goenv; the process groups are internal/procgroup's.
// Moving that engine into a shared package of tools/ci is the natural
// step when the gate needs it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// The exit statuses. A run that a signal cut short exits with 128 plus
// the signal number (130 for SIGINT), see procgroup.Main.
const (
	exitOK    = 0 // every mutant was killed
	exitFail  = 1 // a mutant survived or is invalid
	exitError = 2 // the run could not proceed
)

const usage = `usage:
  go run ./tools/mutants [-worktree] <mutants-file>
`

// errInterrupted ends a run whose context a signal cancelled.
var errInterrupted = errors.New("interrupted")

// env is what a run reads and writes. Tests fill it with a fixture;
// main fills it with the process's own.
type env struct {
	dir string // where the command was started
	// gitEnv is the base of the environment of git, which runs with it
	// through git.CleanEnv; nil means this process's.
	gitEnv []string
	// goBin is the go command to run in the clone; empty means the one
	// resolveGo finds.
	goBin string
	// tmpDir is where the temporary directory of the clone is made;
	// empty means the default directory for temporary files.
	tmpDir string
	// minTimeout is the least -timeout of a mutant run; zero means
	// defaultMinTimeout.
	minTimeout time.Duration
	stdout     io.Writer
	stderr     io.Writer
}

func main() {
	e := env{dir: ".", stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(runMain(e, os.Args[1:]))
}

// runMain runs the command under the signal rule of procgroup.Main: the
// first SIGINT or SIGTERM cancels the context of the run and kills the
// process group of the go command that runs; the run then writes the
// mutated file back and removes the clone. A second signal before the
// handler steps aside is absorbed, and one after it ends the process,
// with no go command left behind.
func runMain(e env, args []string) int {
	return procgroup.Main("mutants", e.stderr, func(ctx context.Context) int {
		return run(ctx, e, args)
	})
}

// run reads the mutants file, runs the mutants, and returns the exit
// status.
func run(ctx context.Context, e env, args []string) int {
	fs := flag.NewFlagSet("mutants", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	worktree := fs.Bool("worktree", false, "run a snapshot of the working tree instead of the commit HEAD")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 || strings.HasPrefix(fs.Arg(0), "-") {
		say(e.stderr, "%s", usage)
		return exitError
	}
	ok, err := runMutants(ctx, e, fs.Arg(0), *worktree)
	switch {
	case err != nil && (errors.Is(err, errInterrupted) || ctx.Err() != nil):
		// The one line of an interrupted run is procgroup.Main's.
		return exitError
	case err != nil:
		say(e.stderr, "mutants: %s\n", git.SafeLines(strings.TrimSpace(err.Error())))
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
