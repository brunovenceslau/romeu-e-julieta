// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks/actions"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

var clock = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

// repo is a work tree with the two places a repo keeps hooks, and the
// log the hooks of the tests write to.
type repo struct {
	t     *testing.T
	dir   string
	state string
	log   string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".githooks"), 0o755))
	return &repo{t: t, dir: dir, state: filepath.Join(t.TempDir(), "state"), log: filepath.Join(t.TempDir(), "log")}
}

// hook writes the hook at rel (below the repo) with mode: a shell
// script that logs its name, its arguments and its stdin, then runs
// body.
func (r *repo) hook(rel string, mode os.FileMode, body string) {
	r.t.Helper()
	script := "#!/bin/sh\necho \"" + rel + " args=$*\" >>" + r.log + "\necho \"" + rel + " stdin=$(cat)\" >>" + r.log + "\n" + body + "\n"
	require.NoError(r.t, os.WriteFile(filepath.Join(r.dir, rel), []byte(script), mode))
}

func (r *repo) logged() []string {
	r.t.Helper()
	b, err := os.ReadFile(r.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(r.t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// run runs hook with args and stdin, with the given actions, and
// returns the status and the stderr.
func (r *repo) run(hook string, stdin string, acts ...actions.Action) (int, string) {
	r.t.Helper()
	var stderr bytes.Buffer
	status, err := hooks.Run(r.t.Context(), hooks.Options{
		Hook: hook, Args: []string{"one", "two"}, Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
		Stdin: strings.NewReader(stdin), Stdout: &stderr, Stderr: &stderr,
		StateDir: r.state, Now: func() time.Time { return clock }, Actions: acts,
	})
	require.NoError(r.t, err)
	return status, stderr.String()
}

func (r *repo) failures() []hooks.Failure {
	r.t.Helper()
	f, err := hooks.Failures(r.state)
	require.NoError(r.t, err)
	return f
}

// logAction is an action of julieta that logs that it ran.
func (r *repo) logAction(name string, err error, continues bool) actions.Action {
	return actions.Action{Name: name, Continues: continues, Run: func(_ context.Context, in actions.Input) error {
		f, e := os.OpenFile(r.log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		require.NoError(r.t, e)
		defer func() { _ = f.Close() }()
		_, e = f.WriteString("action " + name + " args=" + strings.Join(in.Args, " ") + "\n")
		require.NoError(r.t, e)
		return err
	}}
}

func TestRunChainOrderArgumentsAndStdin(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "")
	r.hook(".git/hooks/pre-commit", 0o755, "")
	status, stderr := r.run("pre-commit", "refs", r.logAction("act", nil, false))
	assert.Equal(t, 0, status)
	assert.Empty(t, stderr)
	assert.Equal(t, []string{
		"action act args=one two",
		".githooks/pre-commit args=one two",
		".githooks/pre-commit stdin=refs",
		".git/hooks/pre-commit args=one two",
		".git/hooks/pre-commit stdin=",
	}, r.logged(), "stdin goes to the first repo hook only")
	assert.Empty(t, r.failures())
}

func TestRunStdinGoesToTheFirstHookThatRuns(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/post-rewrite", 0o644, "") // exists, not executable: does not run
	r.hook(".git/hooks/post-rewrite", 0o755, "")
	status, _ := r.run("post-rewrite", "abc def")
	assert.Equal(t, 0, status)
	assert.Equal(t, []string{".git/hooks/post-rewrite args=one two", ".git/hooks/post-rewrite stdin=abc def"}, r.logged())
}

// A file is handed to a stage as it is, as git's own stdin is, so the
// stage that does not read it leaves it for the next one unless the
// dispatcher stops offering it.
func TestRunStdinIsNotOfferedAgainToTheSecondHook(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks/pre-push"), []byte("#!/bin/sh\nexit 0\n"), 0o755)) // reads nothing
	r.hook(".git/hooks/pre-push", 0o755, "")
	in, err := os.Open(writeFile(t, "refs/heads/main abc\n"))
	require.NoError(t, err)
	defer func() { _ = in.Close() }()
	var out bytes.Buffer
	status, err := hooks.Run(t.Context(), hooks.Options{Hook: "pre-push", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"}, Stdin: in,
		Stdout: &out, Stderr: &out, StateDir: r.state, Now: func() time.Time { return clock }})
	require.NoError(t, err)
	assert.Equal(t, 0, status)
	assert.Equal(t, []string{".git/hooks/pre-push args=", ".git/hooks/pre-push stdin="}, r.logged(),
		"the first hook that ran took the stdin, whether it read it or not")
}

func writeFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	require.NoError(t, os.WriteFile(path, []byte(text), 0o600))
	return path
}

func TestRunFirstFailureStopsTheChain(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "exit 7")
	r.hook(".git/hooks/pre-commit", 0o755, "")
	status, stderr := r.run("pre-commit", "")
	assert.Equal(t, 7, status)
	assert.Equal(t, "julieta hooks: pre-commit stage tracked ("+r.dir+"/.githooks/pre-commit) exited 7\n", stderr)
	assert.Len(t, r.logged(), 2, "the local hook did not run")
	assert.Equal(t, []hooks.Failure{{At: clock, ExitCode: 7, Hook: "pre-commit", Path: r.dir + "/.githooks/pre-commit", Repo: r.dir, Stage: "tracked"}}, r.failures())
}

func TestRunLocalHookFailure(t *testing.T) {
	r := newRepo(t)
	r.hook(".git/hooks/post-commit", 0o755, "exit 3")
	status, stderr := r.run("post-commit", "")
	assert.Equal(t, 3, status)
	assert.Contains(t, stderr, "stage local ("+r.dir+"/.git/hooks/post-commit) exited 3")
}

func TestRunAStageEndedByASignal(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "kill -TERM $$")
	status, stderr := r.run("pre-commit", "")
	assert.Equal(t, 128+15, status)
	assert.Contains(t, stderr, " exited 128+15\n")
	assert.Equal(t, 143, r.failures()[0].ExitCode)
}

func TestRunAFailureLastsUntilTheSameHookSucceeds(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "exit 1")
	r.hook(".githooks/pre-push", 0o755, "exit 1")
	r.run("pre-commit", "")
	r.run("pre-push", "")
	require.Len(t, r.failures(), 2)

	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks/pre-commit"), []byte("#!/bin/sh\n"), 0o755))
	status, _ := r.run("pre-commit", "")
	assert.Equal(t, 0, status)
	left := r.failures()
	require.Len(t, left, 1, "the success of pre-commit clears pre-commit alone")
	assert.Equal(t, "pre-push", left[0].Hook)

	other := newRepo(t)
	other.state = r.state
	other.hook(".githooks/pre-push", 0o755, "")
	other.run("pre-push", "")
	assert.Len(t, r.failures(), 1, "a success in another repo clears nothing of this one")
}

func TestRunAnActionThatContinuesRecordsAndTheChainGoesOn(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/post-commit", 0o755, "")
	boom := errors.New("bundle not written")
	status, stderr := r.run("post-commit", "", r.logAction("snapshot", boom, true))
	assert.Equal(t, 0, status)
	assert.Equal(t, "error: bundle not written\njulieta hooks: post-commit stage julieta (snapshot) exited 1\n", stderr)
	assert.Len(t, r.logged(), 3, "the action and the tracked hook ran")
	require.Len(t, r.failures(), 1, "a run that ends with a recorded failure keeps it")
	assert.Equal(t, "julieta", r.failures()[0].Stage)

	status, _ = r.run("post-commit", "", r.logAction("snapshot", nil, true))
	assert.Equal(t, 0, status)
	assert.Empty(t, r.failures(), "the next clean run clears it")
}

func TestRunAnActionThatStops(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "")
	var stderr bytes.Buffer
	status, err := hooks.Run(t.Context(), hooks.Options{
		Hook: "pre-commit", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"}, Stdout: &stderr, Stderr: &stderr,
		StateDir: r.state, Now: func() time.Time { return clock },
		Actions: []actions.Action{r.logAction("lock-check", errors.New("stale"), false)},
		ActionError: func(err error) int {
			_, _ = stderr.WriteString("error RJ-328 check-findings: " + err.Error() + "\n")
			return 6
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 6, status)
	assert.Equal(t, "error RJ-328 check-findings: stale\njulieta hooks: pre-commit stage julieta (lock-check) exited 6\n", stderr.String())
	assert.Equal(t, []string{"action lock-check args="}, r.logged(), "the tracked hook did not run")
}

func TestRunALinkedWorkTreeUsesTheCommonHooksDir(t *testing.T) {
	main := newRepo(t)
	main.hook(".git/hooks/pre-commit", 0o755, "exit 9")
	tree := t.TempDir()
	gitDir := filepath.Join(main.dir, ".git", "worktrees", "wt")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tree, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))
	r := &repo{t: t, dir: tree, state: main.state, log: main.log}
	status, stderr := r.run("pre-commit", "")
	assert.Equal(t, 9, status)
	assert.Contains(t, stderr, "stage local ("+main.dir+"/.git/hooks/pre-commit)")
}

func TestRunGitDirOfTheEnvironment(t *testing.T) {
	r := newRepo(t)
	elsewhere := filepath.Join(t.TempDir(), "hooks")
	require.NoError(t, os.MkdirAll(elsewhere, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(elsewhere, "pre-commit"), []byte("#!/bin/sh\nexit 4\n"), 0o755))
	var stderr bytes.Buffer
	status, err := hooks.Run(t.Context(), hooks.Options{
		Hook: "pre-commit", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin", "GIT_DIR=" + filepath.Dir(elsewhere)},
		Stdout: &stderr, Stderr: &stderr, StateDir: r.state, Now: func() time.Time { return clock },
	})
	require.NoError(t, err)
	assert.Equal(t, 4, status)
}

func TestRunRefusesAHookItDoesNotRun(t *testing.T) {
	r := newRepo(t)
	for _, name := range []string{"", "../x", "pre-receive", "pre-commit/../../x"} {
		_, err := hooks.Run(t.Context(), hooks.Options{Hook: name, Dir: r.dir, StateDir: r.state})
		require.Error(t, err, name)
	}
}

func TestRunACancelledContextIsAnError(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "sleep 30")
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	_, err := hooks.Run(ctx, hooks.Options{Hook: "pre-commit", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
		Stdout: &out, Stderr: &out, StateDir: r.state, Now: func() time.Time { return clock }})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Empty(t, r.failures(), "an interrupted hook is no failure of the hook")
}

func TestRunAStageThatCannotStart(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks/pre-commit"), []byte("#!/no/such/interpreter\n"), 0o755))
	status, stderr := r.run("pre-commit", "")
	assert.Equal(t, 126, status)
	assert.Contains(t, stderr, "did not start")
	assert.Contains(t, stderr, "exited 126")
}

// The hook stages run under the class with no timeout (REQ-0300): with
// every other class of the product cut to 100 ms, a hook that runs for
// 600 ms still succeeds.
func TestRunHasNoTimeout(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "sleep 0.6")
	ctx := t.Context()
	for _, c := range []procgroup.Class{procgroup.SbxRead, procgroup.SbxChange, procgroup.SbxStart, procgroup.SbxCreate,
		procgroup.JulietaSetup, procgroup.JulietaExec, procgroup.HostGitFetch, procgroup.Secret, procgroup.JulietaGit,
		procgroup.Mise, procgroup.Herdr, procgroup.ToolStep, procgroup.ToolRace, procgroup.ToolGit, procgroup.ToolProbe} {
		ctx = procgroup.WithTestTimeout(ctx, c, 100*time.Millisecond)
	}
	var out bytes.Buffer
	status, err := hooks.Run(ctx, hooks.Options{Hook: "pre-commit", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
		Stdout: &out, Stderr: &out, StateDir: r.state, Now: func() time.Time { return clock }})
	require.NoError(t, err)
	assert.Equal(t, 0, status, out.String())
}
