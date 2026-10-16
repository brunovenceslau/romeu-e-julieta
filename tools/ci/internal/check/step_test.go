// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/goenv"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStepEnv pins the environment of the steps, written out in full:
// PATH with the directory of the pinned go first, the other variables
// of goenv.PassThrough that are set, and the fixed ones, in that order and
// nothing else; the lint step adds its target. Every variable of the
// process that is outside that list stays out, whatever its name.
func TestStepEnv(t *testing.T) {
	// The fixed variables, and others that change what a step checks,
	// each set in the process to a value no step may take.
	for _, key := range []string{"GOENV", "GOOS", "GOARCH", "CGO_ENABLED", "GOTOOLCHAIN", "GOWORK", "GOPROXY",
		"GOFLAGS", "GOLANGCI_DIFF_PROCESSOR_PATCH", "GOEXPERIMENT", "MISE_INSTALLS_DIR", "XDG_CONFIG_HOME",
		"CI", "MISE_TRUSTED_CONFIG_PATHS"} {
		t.Setenv(key, "from-the-process")
	}
	for _, key := range goenv.PassThrough {
		t.Setenv(key, "/from/"+key)
	}
	// Two that are empty and must be left out.
	t.Setenv("TMPDIR", "")
	t.Setenv("MISE_CACHE_DIR", "")

	sep := string(os.PathListSeparator)
	located := []string{
		"HOME=/from/HOME",
		"XDG_CACHE_HOME=/from/XDG_CACHE_HOME",
		"XDG_DATA_HOME=/from/XDG_DATA_HOME",
		"XDG_STATE_HOME=/from/XDG_STATE_HOME",
		"MISE_DATA_DIR=/from/MISE_DATA_DIR",
		"MISE_STATE_DIR=/from/MISE_STATE_DIR",
		"GOPATH=/from/GOPATH",
		"GOCACHE=/from/GOCACHE",
		"GOMODCACHE=/from/GOMODCACHE",
	}
	fixed := []string{"GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=-mod=readonly", "GOPROXY=off"}
	target := []string{"GOOS=darwin", "GOARCH=amd64", "CGO_ENABLED=0"}

	assert.Equal(t, slices.Concat([]string{"PATH=/pinned/go/bin" + sep + "/from/PATH"}, located, fixed),
		check.StepEnv("/pinned/go/bin"), "stepEnv")
	assert.Equal(t, slices.Concat([]string{"PATH=/pinned/go/bin" + sep + "/from/PATH"}, located, fixed, target),
		lint.Env(lint.Target{GOOS: "darwin", GOARCH: "amd64"}, "/pinned/go/bin"), "lintEnv")
	assert.Equal(t, slices.Concat([]string{"PATH=/from/PATH"}, located), check.PassThroughEnv(), "passThroughEnv")

	// Without PATH in the process, PATH is the go directory alone.
	require.NoError(t, os.Unsetenv("PATH"))
	assert.Equal(t, slices.Concat([]string{"PATH=/pinned/go/bin"}, located, fixed), check.StepEnv("/pinned/go/bin"), "PATH unset")
	assert.Equal(t, located, check.PassThroughEnv(), "passThroughEnv, PATH unset")
}

func TestStepRun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nvar   X = 1\n"), 0o644))
	tests := []struct {
		name string
		step check.Step
		ok   bool
	}{
		{"a command that exits 0", citest.Passing, true},
		{"a command that exits non-zero", citest.Failing, false},
		{"a command that is not installed", check.Step{Name: "x", Argv: []string{"no-such-program-here"}}, false},
		{"gofmt lists a file: output is a failure", check.Step{Name: "format", Argv: []string{"gofmt", "-l", "."}, Quiet: true}, false},
		{"a quiet command that fails", check.Step{Name: "x", Argv: []string{"gofmt", "-l", "nothing.go"}, Quiet: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.step.Run(t.Context(), dir)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nvar X = 1\n"), 0o644))
	_, err := (check.Step{Name: "format", Argv: []string{"gofmt", "-l", "."}, Quiet: true}).Run(t.Context(), dir)
	require.NoError(t, err, "gofmt on a formatted tree")

	// The command gets its environ and nothing else; a nil one is empty.
	t.Setenv("GOFLAGS", "-run=^$")
	out, err := (check.Step{Name: "env", Argv: []string{"env"}, Environ: []string{"A=1"}}).Run(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, "A=1\n", string(out))
	out, err = (check.Step{Name: "env", Argv: []string{"env"}}).Run(t.Context(), dir)
	require.NoError(t, err)
	assert.Empty(t, string(out))
}

// TestStepRunDeadline checks that a step that outlives the limit of its
// class fails with procgroup's subprocess-timeout, which names the
// class and the limit and wraps context.DeadlineExceeded, where the
// bare error of the killed command says only that a signal ended it; a
// step runs under procgroup.ToolStep unless it names another class.
func TestStepRunDeadline(t *testing.T) {
	sleep := []string{"sleep", "30"}
	environ := []string{"PATH=" + os.Getenv("PATH")}
	t.Run("the limit of the step's class", func(t *testing.T) {
		ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolStep, 50*time.Millisecond)
		_, err := check.Step{Name: "slow", Argv: sleep, Environ: environ}.Run(ctx, t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		timeout, ok := errors.AsType[*procgroup.TimeoutError](err)
		require.True(t, ok, "the error is subprocess-timeout: %v", err)
		assert.Equal(t, procgroup.ToolStep, timeout.Class)
		assert.ErrorContains(t, err, "sleep ran past the 50ms limit of class tool-step")
	})
	t.Run("the race step runs under its own class", func(t *testing.T) {
		ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolRace, 50*time.Millisecond)
		_, err := check.Step{Name: "race", Argv: sleep, Environ: environ, Class: procgroup.ToolRace}.Run(ctx, t.TempDir())
		timeout, ok := errors.AsType[*procgroup.TimeoutError](err)
		require.True(t, ok, "the error is subprocess-timeout: %v", err)
		assert.Equal(t, procgroup.ToolRace, timeout.Class)
	})
	t.Run("the deadline of the run, before the limit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		_, err := check.Step{Name: "slow", Argv: sleep, Environ: environ}.Run(ctx, t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		_, ok := errors.AsType[*procgroup.TimeoutError](err)
		assert.False(t, ok, "the caller's deadline is not the step's subprocess-timeout: %v", err)
	})
	t.Run("a grandchild that holds the output pipe", func(t *testing.T) {
		// sh forks sleep, which inherits the pipe and outlives sh: only
		// killing the process group, or WaitDelay, ends the wait.
		ctx := procgroup.WithTestTimeout(t.Context(), procgroup.ToolStep, 200*time.Millisecond)
		start := time.Now()
		_, err := check.Step{Name: "slow", Argv: []string{"sh", "-c", "sleep 30; true"}, Environ: environ}.Run(ctx, t.TempDir())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 5*time.Second, "the step returns at its deadline, not when the grandchild ends")
	})
	t.Run("a failure that is not a deadline", func(t *testing.T) {
		_, err := check.Step{Name: "fails", Argv: []string{"false"}, Environ: environ}.Run(t.Context(), t.TempDir())
		exit, ok := errors.AsType[*procgroup.ExitError](err)
		require.True(t, ok, "the error is the exit of the command: %v", err)
		assert.Equal(t, 1, exit.Code)
		assert.NotErrorIs(t, err, context.DeadlineExceeded)
	})
}

// TestPassThroughLists pins the two lists of variables a step may keep
// from the process, and the way withProcessEnv adds them: in the order
// of the list, a variable that is empty or unset left out.
func TestPassThroughLists(t *testing.T) {
	assert.Equal(t, []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}, check.NetworkPassThrough)
	assert.Equal(t, []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG"}, check.DockerPassThrough)

	t.Setenv("DOCKER_HOST", "unix:///a")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_CONFIG", "/c")
	base := []string{"PATH=/x"}
	assert.Equal(t, []string{"PATH=/x", "DOCKER_HOST=unix:///a", "DOCKER_CONFIG=/c"}, check.WithProcessEnv(base, check.DockerPassThrough), "an empty variable is left out")
	assert.Equal(t, []string{"PATH=/x"}, base, "the base is not changed")
}
