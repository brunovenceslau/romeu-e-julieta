// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/hooks/actions"
)

// systemHook writes the hook name in the directory dir, a shell script
// that logs as the repo's hooks do, then runs body, and returns dir.
func (r *repo) systemHook(dir, name, body string) string {
	r.t.Helper()
	require.NoError(r.t, os.MkdirAll(dir, 0o755))
	script := "#!/bin/sh\necho \"system " + name + " args=$*\" >>" + r.log + "\necho \"system " + name + " stdin=$(cat)\" >>" + r.log + "\n" + body + "\n"
	require.NoError(r.t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	return dir
}

// runSystem runs hook as run does, with system as the recorded system
// hooks path and dispatcher as julieta's own.
func (r *repo) runSystem(hook, stdin, system, dispatcher string, acts ...actions.Action) (int, string) {
	r.t.Helper()
	var stdout, stderr bytes.Buffer
	status, err := hooks.Run(r.t.Context(), hooks.Options{
		Hook: hook, Args: []string{"one", "two"}, Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
		Stdin: strings.NewReader(stdin), Stdout: &stdout, Stderr: &stderr,
		StateDir: r.state, Now: func() time.Time { return clock }, Actions: acts,
		System: system, Dispatcher: dispatcher,
	})
	require.NoError(r.t, err)
	return status, stderr.String()
}

// TestRunSystemStageRunsFirst: the hook of the recorded system path
// runs before julieta's action and the repo's hooks, with the hook's
// arguments, and takes the stdin, as the first hook that runs.
func TestRunSystemStageRunsFirst(t *testing.T) {
	r := newRepo(t)
	system := r.systemHook(filepath.Join(t.TempDir(), "kit"), "pre-commit", "")
	r.hook(".githooks/pre-commit", 0o755, "")
	status, stderr := r.runSystem("pre-commit", "refs", system, filepath.Join(t.TempDir(), "dispatcher"), r.logAction("act", nil, false))
	assert.Equal(t, 0, status)
	assert.Empty(t, stderr)
	assert.Equal(t, []string{
		"system pre-commit args=one two",
		"system pre-commit stdin=refs",
		"action act args=one two",
		".githooks/pre-commit args=one two",
		".githooks/pre-commit stdin=refs",
	}, r.logged(), "each hook gets its own copy of the stdin")
}

// TestRunASystemHookThatIgnoresStdinLeavesItToTheRepoHook is the
// audit's case: a system pre-push that reads nothing and exits 0 must
// not take the ref lines from the tracked pre-push, which refuses the
// push of main.
func TestRunASystemHookThatIgnoresStdinLeavesItToTheRepoHook(t *testing.T) {
	r := newRepo(t)
	system := filepath.Join(t.TempDir(), "kit")
	require.NoError(t, os.MkdirAll(system, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(system, "pre-push"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks", "pre-push"),
		[]byte("#!/bin/sh\ngrep -q refs/heads/main && exit 1\nexit 0\n"), 0o755))
	status, stderr := r.runSystem("pre-push", "refs/heads/main abc refs/heads/main def\n", system, filepath.Join(t.TempDir(), "dispatcher"))
	assert.Equal(t, 1, status, "the tracked hook saw the ref lines and refused the push")
	assert.Contains(t, stderr, "stage tracked")
}

// TestRunStdinOverTheBound: a stdin longer than the bound, lowered for
// the test, fails the run before any stage starts.
func TestRunStdinOverTheBound(t *testing.T) {
	t.Cleanup(hooks.SetMaxStdin(4))
	r := newRepo(t)
	r.hook(".githooks/pre-push", 0o755, "")
	var out bytes.Buffer
	_, err := hooks.Run(t.Context(), hooks.Options{Hook: "pre-push", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
		Stdin: strings.NewReader("12345"), Stdout: &out, Stderr: &out, StateDir: r.state, Now: func() time.Time { return clock }})
	require.ErrorContains(t, err, "longer than 4 bytes")
	assert.Empty(t, r.logged(), "no stage ran")

	status, _ := r.run("pre-push", "1234")
	assert.Equal(t, 0, status, "at the bound")
}

// TestRunProcReceiveStreamsStdin: proc-receive talks a protocol with
// git over its stdin, which git keeps open, so its stdin goes as it is
// to the first hook that runs and is not read to its end first.
func TestRunProcReceiveStreamsStdin(t *testing.T) {
	r := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks", "proc-receive"),
		[]byte("#!/bin/sh\nread line\necho \"got $line\" >>"+r.log+"\n"), 0o755))
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer func() { _ = pr.Close(); _ = pw.Close() }()
	_, err = pw.WriteString("version=1\n") // and the pipe stays open, as git keeps it
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var out bytes.Buffer
	status, err := hooks.Run(ctx, hooks.Options{Hook: "proc-receive", Dir: r.dir, Env: []string{"PATH=/usr/bin:/bin"},
		Stdin: pr, Stdout: &out, Stderr: &out, StateDir: r.state, Now: func() time.Time { return clock }})
	require.NoError(t, err, "the run did not wait for the end of the stdin")
	assert.Equal(t, 0, status)
	assert.Equal(t, []string{"got version=1"}, r.logged())
}

// TestRunSystemStageMarksItsEnvironment: the system hook runs with
// JULIETA_HOOKS_SYSTEM=1 in its environment and the other stages do
// not; a dispatcher started with the marker set, as one the system
// hook starts would be, skips the system stage, so a system dir that
// leads back to the dispatcher cannot loop.
func TestRunSystemStageMarksItsEnvironment(t *testing.T) {
	r := newRepo(t)
	system := filepath.Join(t.TempDir(), "kit")
	require.NoError(t, os.MkdirAll(system, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(system, "pre-commit"),
		[]byte("#!/bin/sh\necho \"system marker=$JULIETA_HOOKS_SYSTEM\" >>"+r.log+"\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, ".githooks", "pre-commit"),
		[]byte("#!/bin/sh\necho \"tracked marker=$JULIETA_HOOKS_SYSTEM\" >>"+r.log+"\n"), 0o755))
	status, stderr := r.runSystem("pre-commit", "", system, filepath.Join(t.TempDir(), "dispatcher"))
	assert.Equal(t, 0, status, stderr)
	assert.Equal(t, []string{"system marker=1", "tracked marker="}, r.logged())

	require.NoError(t, os.Remove(r.log))
	var out bytes.Buffer
	status, err := hooks.Run(t.Context(), hooks.Options{Hook: "pre-commit", Dir: r.dir,
		Env: []string{"PATH=/usr/bin:/bin", hooks.SystemMarker}, Stdout: &out, Stderr: &out, StateDir: r.state,
		Now: func() time.Time { return clock }, System: system, Dispatcher: filepath.Join(t.TempDir(), "dispatcher")})
	require.NoError(t, err)
	assert.Equal(t, 0, status, out.String())
	assert.Equal(t, []string{"tracked marker=1"}, r.logged(), "the marker skips the system stage, and the repo hooks keep git's environment")
}

// TestRunSystemStageFailureStopsTheChain: a failing system hook stops
// the chain, its status is the dispatcher's, and the failure line and
// the record name the stage system.
func TestRunSystemStageFailureStopsTheChain(t *testing.T) {
	r := newRepo(t)
	system := r.systemHook(filepath.Join(t.TempDir(), "kit"), "pre-commit", "exit 4")
	r.hook(".githooks/pre-commit", 0o755, "")
	status, stderr := r.runSystem("pre-commit", "", system, filepath.Join(t.TempDir(), "dispatcher"), r.logAction("act", nil, false))
	assert.Equal(t, 4, status)
	path := filepath.Join(system, "pre-commit")
	assert.Equal(t, "julieta hooks: pre-commit stage system ("+path+") exited 4\n", stderr)
	assert.Len(t, r.logged(), 2, "neither the action nor the tracked hook ran")
	repo, err := filepath.EvalSymlinks(r.dir)
	require.NoError(t, err)
	assert.Equal(t, []hooks.Failure{{At: clock, ExitCode: 4, Hook: "pre-commit", Path: path, Repo: repo, Stage: hooks.StageSystem}}, r.failures())
}

// TestRunSystemStageRunsNothingItShouldNot: no recorded path, a
// relative one, julieta's own dispatcher (by its path or through a
// link), and a system hook that is missing or not executable each run
// nothing in the system stage, and the chain goes on.
func TestRunSystemStageRunsNothingItShouldNot(t *testing.T) {
	r := newRepo(t)
	r.hook(".githooks/pre-commit", 0o755, "")
	dispatcher := r.systemHook(filepath.Join(t.TempDir(), "dispatcher"), "pre-commit", "")
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(dispatcher, link))
	r.systemHook(filepath.Join(r.dir, "kit"), "pre-commit", "") // what "kit" would name from the repo
	notExecutable := r.systemHook(filepath.Join(t.TempDir(), "kit"), "pre-commit", "")
	require.NoError(t, os.Chmod(filepath.Join(notExecutable, "pre-commit"), 0o644))
	t.Chdir(r.dir)
	for name, system := range map[string]string{
		"none":           "",
		"relative":       "kit",
		"dispatcher":     dispatcher,
		"dispatcher/":    dispatcher + "/",
		"link":           link,
		"missing":        filepath.Join(t.TempDir(), "empty"),
		"not executable": notExecutable,
	} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.RemoveAll(r.log))
			status, stderr := r.runSystem("pre-commit", "", system, dispatcher)
			assert.Equal(t, 0, status, stderr)
			assert.Equal(t, []string{".githooks/pre-commit args=one two", ".githooks/pre-commit stdin="}, r.logged())
		})
	}
}

// TestSystemRecord: setup's record of the system hooks path reads back
// as written; an empty path removes it, and a missing record reads as
// none; a record that is not the format is an error.
func TestSystemRecord(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	got, err := hooks.ReadSystem(dir)
	require.NoError(t, err)
	assert.Empty(t, got)

	require.NoError(t, hooks.RecordSystem(dir, "/opt/kit/hooks"))
	got, err = hooks.ReadSystem(dir)
	require.NoError(t, err)
	assert.Equal(t, "/opt/kit/hooks", got)
	info, err := os.Stat(filepath.Join(dir, hooks.SystemFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	require.NoError(t, hooks.RecordSystem(dir, ""))
	_, err = os.Stat(filepath.Join(dir, hooks.SystemFile))
	require.ErrorIs(t, err, os.ErrNotExist, "an empty path removes the record")
	got, err = hooks.ReadSystem(dir)
	require.NoError(t, err)
	assert.Empty(t, got)
	require.NoError(t, hooks.RecordSystem(dir, ""), "removing no record is no error")

	require.NoError(t, os.WriteFile(filepath.Join(dir, hooks.SystemFile), []byte(`{"path":"/a","other":1}`), 0o600))
	_, err = hooks.ReadSystem(dir)
	require.Error(t, err)
}
