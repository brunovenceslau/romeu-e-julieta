// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package hooks is the hook dispatcher of julieta (04 4.3, REQ-0408,
// REQ-0409): the chain git's hooks run through in a sandbox, the
// scripts that point git at it (Install) and the failures it records
// in julieta state (Record, Failures).
//
// For a hook, Run executes up to four stages in this order: the hook of
// the system core.hooksPath that julieta setup recorded (SystemFile),
// which the global key hides from git; julieta's own action
// (internal/hooks/actions); the repo's tracked .githooks/<hook>; and
// $GIT_DIR/hooks/<hook>; each hook only if it exists and is executable.
// The arguments of the hook go to each, and so does a copy of its
// stdin, kept in a temporary file with no bound (readStdin), except for
// proc-receive, whose stdin goes to the first hook that runs. The system hook runs
// with SystemMarker in its environment, and a run that starts with it
// set skips the system stage, so a system hook that leads back to the
// dispatcher cannot loop. The first non-zero status stops the chain
// and is the status of the dispatcher, except for an action that
// continues (the snapshot), whose failure is recorded and does not stop
// the chain.
//
// The system hook belongs to the sandbox, and the two repo hooks are
// repository content that runs on purpose, and only in the sandbox
// (I21, REQ-0505): they have no timeout (the Unbounded class of
// internal/procgroup, REQ-0300) and get the environment git gave the
// hook, which they need (GIT_INDEX_FILE, PATH and the rest). The
// scrubbed environment of 04 4.1 is for julieta's own subprocesses.
package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks/actions"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// Names are the hooks of githooks(5) for git 2.53 and the floor of
// internal/gitsafe: every hook git can run, the client-side ones, the
// ones a receiving repository runs (bare repositories included, where
// the hooks run in the git dir) and those of git-p4 and
// git-send-email. Once the dispatcher is the global core.hooksPath, git
// no longer looks in $GIT_DIR/hooks, so a hook missing here would stop
// running in every repo; each one has a script and is forwarded to the
// repo's own hooks at least. Run refuses any other name, so a name
// never becomes a path outside .githooks/ and hooks/.
//
// fsmonitor-watchman is here for that reason, though its output is a
// protocol that the CLI writer's escaping can corrupt; git then falls
// back to a full scan of the tree.
var Names = []string{
	"applypatch-msg", "pre-applypatch", "post-applypatch",
	"pre-commit", "pre-merge-commit", "prepare-commit-msg", "commit-msg", "post-commit",
	"pre-rebase", "post-checkout", "post-merge", "pre-push", "pre-receive", "update",
	"proc-receive", "post-receive", "post-update", "reference-transaction", "push-to-checkout",
	"pre-auto-gc", "post-rewrite", "sendemail-validate", "fsmonitor-watchman", "post-index-change",
	"p4-changelist", "p4-prepare-changelist", "p4-post-changelist", "p4-pre-submit",
}

// The four stages of the chain, as the failure line names them.
const (
	StageSystem  = "system"
	StageJulieta = "julieta"
	StageTracked = "tracked"
	StageLocal   = "local"
)

// statusNotStarted is the status of a stage that could not be started,
// as a shell reports a program it cannot execute.
const statusNotStarted = 126

// Options is one run of the dispatcher.
type Options struct {
	// Hook is the hook name, one of Names.
	Hook string
	// Args are the arguments git gave the hook.
	Args []string
	// Dir is the directory git ran the hook in: the root of the work
	// tree, or the git dir of a bare repository. It names the repo in
	// the recorded failures.
	Dir string
	// Env is the environment of the stages, as git gave it.
	Env []string
	// Stdin is the stdin of the hook, copied once to a file and given to
	// every hook stage as a reader of its own (readStdin). Nil means an
	// empty one.
	Stdin io.Reader
	// Stdout and Stderr receive the output of every stage and the line
	// the dispatcher prints on a failure.
	Stdout, Stderr io.Writer
	// StateDir is the directory the failures are recorded in.
	StateDir string
	// Now is the clock.
	Now func() time.Time
	// Actions are the actions of julieta for the hook, in order
	// (actions.For).
	Actions []actions.Action
	// ActionError prints the error of a failed action, its id and fix,
	// and returns the exit status it maps to. Nil prints the text of
	// the error and returns 1.
	ActionError func(err error) int
	// System is the system hooks path julieta setup recorded
	// (ReadSystem), empty for none. A path that is not absolute is
	// ignored.
	System string
	// Dispatcher is the directory of julieta's own dispatcher
	// (DispatcherDir): a System that names it is not run, so the chain
	// never calls itself.
	Dispatcher string
}

// Run runs the chain of the hook and returns the status of the
// dispatcher: 0, or the status of the stage that stopped the chain
// (128 plus the signal for a stage a signal ended). A failure it
// returns as an error is not a stage's: an unknown hook, a failure to
// record, or the cancellation of ctx, which kills the stage that runs.
//
// A failure is recorded in julieta state before Run returns (an error
// of the record is said on stderr and changes no status), and a run
// in which no stage failed clears the record of the same hook in the
// same repo (REQ-0675); a failure that did not stop the chain stays
// recorded, since the hook did not succeed.
func Run(ctx context.Context, o Options) (int, error) {
	if !slices.Contains(Names, o.Hook) {
		return 0, fmt.Errorf("%q is not a hook the dispatcher runs", o.Hook)
	}
	// The repo is named by its real path, so a link to it is the same repo.
	repoKey := o.Dir
	if real, err := filepath.EvalSymlinks(o.Dir); err == nil {
		repoKey = real
	}
	var kept *Failure // the failure that did not stop the chain
	fail := func(stage, path string, status int) Failure {
		return Failure{Hook: o.Hook, Repo: repoKey, Stage: stage, Path: path, ExitCode: status, At: o.Now().UTC()}
	}
	// A failure to keep the books never changes the chain's status:
	// it is said on stderr and the stage's status stands.
	note := func(what string, err error) {
		if err != nil {
			_, _ = fmt.Fprintf(o.Stderr, "julieta hooks: %s: %s: %v\n", o.Hook, what, err)
		}
	}
	stop := func(f Failure) (int, error) {
		note("the failure could not be recorded", Record(o.StateDir, f))
		return f.ExitCode, nil
	}
	stdin, closeStdin, err := readStdin(o.Hook, o.Stdin)
	if err != nil {
		return 0, err
	}
	defer closeStdin()
	// runHook runs one hook stage, if its hook is executable, with env,
	// and reports whether it stopped the chain, with its status.
	runHook := func(stage, path string, env []string) (bool, int, error) {
		if !executable(path) {
			return false, 0, nil
		}
		err := procgroup.Run(ctx, procgroup.Command{
			Class: procgroup.Unbounded, Argv: append([]string{path}, o.Args...), Dir: o.Dir, Env: env,
			Stdin: stdin(), Stdout: o.Stdout, Stderr: o.Stderr,
		})
		if cerr := ctx.Err(); cerr != nil {
			return false, 0, cerr
		}
		if err == nil {
			return false, 0, nil
		}
		status, said := statusOf(err)
		if status == statusNotStarted {
			_, _ = fmt.Fprintf(o.Stderr, "julieta hooks: %s stage %s (%s) did not start: %v\n", o.Hook, stage, path, err)
		}
		if err := line(o, stage, path, said); err != nil {
			return false, 0, err
		}
		status, err = stop(fail(stage, path, status))
		return true, status, err
	}
	if path := systemHook(o.System, o.Dispatcher, o.Hook); path != "" && !slices.Contains(o.Env, SystemMarker) {
		if stopped, status, err := runHook(StageSystem, path, append(slices.Clone(o.Env), SystemMarker)); stopped || err != nil {
			return status, err
		}
	}
	for _, a := range o.Actions {
		err := a.Run(ctx, actions.Input{Hook: o.Hook, Args: o.Args, Dir: o.Dir, Env: o.Env, Stdout: o.Stdout, Stderr: o.Stderr})
		if cerr := ctx.Err(); cerr != nil {
			return 0, cerr
		}
		if err == nil {
			continue
		}
		status := 1
		if o.ActionError != nil {
			status = o.ActionError(err)
		} else {
			_, _ = fmt.Fprintf(o.Stderr, "error: %v\n", err)
		}
		if status == 0 {
			status = 1
		}
		if err := line(o, StageJulieta, a.Name, strconv.Itoa(status)); err != nil {
			return 0, err
		}
		f := fail(StageJulieta, a.Name, status)
		if !a.Continues {
			return stop(f)
		}
		kept = &f
	}
	type repoHook struct{ stage, path string }
	repoHooks := []repoHook{{StageTracked, filepath.Join(o.Dir, ".githooks", o.Hook)}}
	if dir := hooksDir(o.Dir, o.Env); dir != "" {
		repoHooks = append(repoHooks, repoHook{StageLocal, filepath.Join(dir, o.Hook)})
	}
	for _, s := range repoHooks {
		if stopped, status, err := runHook(s.stage, s.path, o.Env); stopped || err != nil {
			return status, err
		}
	}
	if kept != nil {
		note("the failure could not be recorded", Record(o.StateDir, *kept))
		return 0, nil
	}
	note("the record of an earlier failure could not be cleared", Clear(o.StateDir, repoKey, o.Hook))
	return 0, nil
}

// line prints the line that says which stage failed and how it ended
// (04 4.3, REQ-0409).
func line(o Options, stage, path, exited string) error {
	_, err := fmt.Fprintf(o.Stderr, "julieta hooks: %s stage %s (%s) exited %s\n", o.Hook, stage, path, exited)
	return err
}

// statusOf returns the exit status of a stage that failed and the text
// of "exited <n>": 128 plus the signal for a stage a signal ended, as
// 4.1 Signals says (REQ-0306); a stage that did not start has 126.
func statusOf(err error) (int, string) {
	if e, ok := errors.AsType[*procgroup.ExitError](err); ok {
		if e.Signal != 0 {
			return 128 + int(e.Signal), "128+" + strconv.Itoa(int(e.Signal))
		}
		return e.Code, strconv.Itoa(e.Code)
	}
	return statusNotStarted, strconv.Itoa(statusNotStarted)
}

// executable reports whether path is a file with an execute bit, the
// links followed, as git treats a hook.
func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

// streamedStdin are the hooks whose stdin is a protocol with git, which
// keeps it open while the hook answers (githooks(5), proc-receive):
// reading it to its end first would wait for ever, so it goes as it is
// to the first hook that runs, and the later ones get none.
var streamedStdin = []string{"proc-receive"}

// hookStages is the number of stages that run a hook, and so read a
// copy of the stdin: system, tracked and local.
const hookStages = 3

// readStdin returns the stdin of each hook stage of hook, and a close
// that releases it. in is copied, with no bound, to a file of the
// temporary directory (0600, never in the repository), which is opened
// once for each stage that runs a hook and then removed, before any
// stage runs: each stage gets its own *os.File at offset 0, which
// procgroup.Run passes to the child as it is, so a stage that reads
// none leaves all of it to the next (REQ-0408), and the copy leaves no
// file behind once the stages start, whatever ends the process. A
// large reference-transaction, whose failure aborts the transaction,
// so fails no git command for its size. A streamed hook's stdin is in
// itself, once.
func readStdin(hook string, in io.Reader) (next func() io.Reader, closeAll func(), err error) {
	none := func() io.Reader { return nil }
	if in == nil {
		return none, func() {}, nil
	}
	if slices.Contains(streamedStdin, hook) {
		return func() io.Reader {
			r := in
			in = nil
			return r
		}, func() {}, nil
	}
	f, err := os.CreateTemp("", "julieta-hook-stdin-")
	if err != nil {
		return nil, nil, fmt.Errorf("the copy of the stdin of the hook %s: %w", hook, err)
	}
	path := f.Name()
	defer func() { _ = os.Remove(path) }()
	_, err = io.Copy(f, in)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, nil, fmt.Errorf("the copy of the stdin of the hook %s: %w", hook, err)
	}
	files := make([]*os.File, 0, hookStages)
	closeAll = func() {
		for _, f := range files {
			_ = f.Close()
		}
	}
	for range hookStages {
		f, err := os.Open(path)
		if err != nil {
			closeAll()
			return nil, nil, fmt.Errorf("the copy of the stdin of the hook %s: %w", hook, err)
		}
		files = append(files, f)
	}
	i := 0
	return func() io.Reader {
		if i == len(files) {
			return nil
		}
		i++
		return files[i-1]
	}, closeAll, nil
}
