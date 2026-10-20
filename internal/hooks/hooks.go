// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package hooks is the hook dispatcher of julieta (04 4.3, REQ-0408,
// REQ-0409): the chain git's hooks run through in a sandbox, the
// scripts that point git at it (Install) and the failures it records
// in julieta state (Record, Failures).
//
// For a hook, Run executes up to three stages in this order: julieta's
// own action (internal/hooks/actions), the repo's tracked
// .githooks/<hook> and $GIT_DIR/hooks/<hook>, each only if it exists
// and is executable. The arguments of the hook go to each; its stdin
// goes to the first of the two repo hooks that runs. The first
// non-zero status stops the chain and is the status of the dispatcher,
// except for an action that continues (the snapshot), whose failure is
// recorded and does not stop the chain.
//
// The two repo hooks are repository content that runs on purpose, and
// only in the sandbox (I21, REQ-0505): they have no timeout (the
// Unbounded class of internal/procgroup, REQ-0300) and get the
// environment git gave the hook, which they need (GIT_INDEX_FILE, PATH
// and the rest). The scrubbed environment of 04 4.1 is for julieta's
// own subprocesses.
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

// The three stages of the chain, as the failure line names them.
const (
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
	// Stdin is the stdin of the hook, given to the first repo hook that
	// runs. Nil means an empty one.
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
	stdin := o.Stdin
	type repoHook struct{ stage, path string }
	repoHooks := []repoHook{{StageTracked, filepath.Join(o.Dir, ".githooks", o.Hook)}}
	if dir := hooksDir(o.Dir, o.Env); dir != "" {
		repoHooks = append(repoHooks, repoHook{StageLocal, filepath.Join(dir, o.Hook)})
	}
	for _, s := range repoHooks {
		if !executable(s.path) {
			continue
		}
		err := procgroup.Run(ctx, procgroup.Command{
			Class: procgroup.Unbounded, Argv: append([]string{s.path}, o.Args...), Dir: o.Dir, Env: o.Env,
			Stdin: stdin, Stdout: o.Stdout, Stderr: o.Stderr,
		})
		stdin = nil // the first hook that ran took it; later ones get /dev/null
		if cerr := ctx.Err(); cerr != nil {
			return 0, cerr
		}
		if err == nil {
			continue
		}
		status, said := statusOf(err)
		if status == statusNotStarted {
			_, _ = fmt.Fprintf(o.Stderr, "julieta hooks: %s stage %s (%s) did not start: %v\n", o.Hook, s.stage, s.path, err)
		}
		if err := line(o, s.stage, s.path, said); err != nil {
			return 0, err
		}
		return stop(fail(s.stage, s.path, status))
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
