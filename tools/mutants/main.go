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
// run that SIGINT, SIGTERM or SIGHUP cut short prints one "interrupted"
// line and exits 128 plus the signal number. Started through "go run",
// every status other than 0 reaches the caller as 1, with the real one
// in the "exit status" line that go prints.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/procgroup"
)

// The exit statuses. A run that a signal cut short exits with 128 plus
// the signal number (130 for SIGINT), see finish.
const (
	exitOK    = 0 // every mutant was killed
	exitFail  = 1 // a mutant survived or is invalid
	exitError = 2 // the run could not proceed
	// exitSignalBase is added to the number of the signal that ended a run.
	exitSignalBase = 128
)

const usage = `usage:
  go run ./tools/mutants [-worktree] <mutants-file>
`

// signals are the signals that end a run: each cancels it, which kills
// the go command, restores the file and removes the clone. SIGQUIT
// keeps its default, the goroutine dump of the Go runtime.
var signals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

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
	// group is the process group of the go command that runs; nil means
	// a new one, which run makes.
	group  *group
	stdout io.Writer
	stderr io.Writer
}

// group holds the process group of the go command that runs, so that
// the handler of a signal can kill it before it steps aside.
type group struct {
	mu  sync.Mutex
	pid int // the leader of the group; 0 when no go command runs
}

// combinedOutput runs cmd as procgroup.CombinedOutput does, its group
// killed when it returns, and holds the group while it runs. The lock
// covers the start, so kill finds either no group or the one started.
func (g *group) combinedOutput(cmd *exec.Cmd) ([]byte, error) {
	procgroup.KillWithGroup(cmd)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	g.mu.Lock()
	err := cmd.Start()
	if err == nil {
		g.pid = cmd.Process.Pid
	}
	g.mu.Unlock()
	if err != nil {
		return nil, err
	}
	err = cmd.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	_ = procgroup.KillGroup(g.pid)
	g.pid = 0
	return out.Bytes(), err
}

// kill kills the group of the go command that runs, if one does. The
// pgid is not reused while the group has members, as in
// procgroup.CombinedOutput.
func (g *group) kill() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.pid != 0 {
		_ = procgroup.KillGroup(g.pid)
	}
}

func main() {
	e := env{dir: ".", stdout: os.Stdout, stderr: os.Stderr}
	os.Exit(runSignalled(e, os.Args[1:]))
}

// runSignalled runs the command with a handler for the signals. The
// first signal cancels the context of the run and kills the process
// group of the go command that runs (group.kill); the run then writes
// the mutated file back and removes the clone. The handler steps aside
// once the group is dead, so a second signal ends the process at once
// and leaves no test binary behind.
func runSignalled(e env, args []string) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if e.group == nil {
		e.group = &group{}
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, signals...)
	defer signal.Stop(sigs)
	caught := make(chan syscall.Signal, 1)
	go func() {
		select {
		case sig := <-sigs:
			// Caught before cancel, so the run that cancel ends finds the
			// signal; the group killed before Stop, so a second signal
			// that ends this process leaves no go command behind.
			caught <- sig.(syscall.Signal)
			cancel()
			e.group.kill()
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
// after a failure; a run that passed (0) stays 0, as the signal came
// after its work was done.
func finish(w io.Writer, status int, sig syscall.Signal) int {
	if sig == 0 || status == exitOK {
		return status
	}
	say(w, "mutants: interrupted\n")
	return exitSignalBase + int(sig)
}

// run reads the mutants file, runs the mutants, and returns the exit
// status.
func run(ctx context.Context, e env, args []string) int {
	if e.group == nil {
		e.group = &group{}
	}
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
		// The one line of an interrupted run is finish's.
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
