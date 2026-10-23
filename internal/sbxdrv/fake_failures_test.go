// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// failureRig is a rig on the failure session of 01 1.6 named.
func failureRig(t *testing.T, name string) *fakeRig {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(sessionsDir, recordedDir, name+".jsonl"))
	require.NoError(t, err)
	return newFakeRig(t, abs)
}

// TestFailureSessionLsError is the first failure row of 01 1.6: an sbx
// ls that exits non-zero is sbx-unknown (RJ-308, exit 1), with the stderr
// of sbx in its details, and not an absent sandbox.
func TestFailureSessionLsError(t *testing.T) {
	rig := failureRig(t, "fail-ls-error")
	_, err := rig.runner.Version(context.Background(), Opts{})
	require.NoError(t, err)
	list, err := rig.runner.List(context.Background(), Opts{Step: "preflight step 6"})
	testsupport.RequireID(t, "RJ-308", err)
	assert.Nil(t, list)
	assert.Equal(t, 1, cli.Of(err).Exit)
	var stderr string
	for _, d := range cli.Of(err).Details {
		if d.Key == "stderr" {
			stderr = d.Value
		}
	}
	assert.Contains(t, stderr, "cannot connect to the sandbox daemon")
}

// TestFailureSessionExecTimeout is the second failure row: an sbx env
// exec that never answers is ended by the timeout of its class, through
// the seam of the table, as subprocess-timeout (RJ-323).
func TestFailureSessionExecTimeout(t *testing.T) {
	rig := failureRig(t, "fail-exec-timeout")
	_, err := rig.runner.List(context.Background(), Opts{})
	require.NoError(t, err)
	setup, err := EnvExec(rig.fx.Dir, procgroup.JulietaSetup, []string{"JULIETA_MANIFEST=e30"}, []string{rig.fx.Julieta, "setup", "--json"})
	require.NoError(t, err)
	ctx := procgroup.WithTestTimeout(context.Background(), procgroup.JulietaSetup, 300*time.Millisecond)
	_, err = rig.runner.Run(ctx, setup, Opts{Step: "run step 6"})
	te, ok := errors.AsType[*procgroup.TimeoutError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "RJ-323", te.ID())
	assert.Equal(t, procgroup.JulietaSetup, te.Class)
}

// TestFailureSessionRunTimeout is the third failure row: the create that
// never answers times out (RJ-323), and sbx ls --json then shows the
// sandbox, because the daemon may finish the create after romeu stopped
// waiting (01 1.6, REQ-0089).
func TestFailureSessionRunTimeout(t *testing.T) {
	rig := failureRig(t, "fail-run-timeout")
	list, err := rig.runner.List(context.Background(), Opts{})
	require.NoError(t, err)
	require.Empty(t, list)
	create, err := EnvCreate(rig.fx.Dir)
	require.NoError(t, err)
	ctx := procgroup.WithTestTimeout(context.Background(), procgroup.SbxCreate, 300*time.Millisecond)
	_, err = rig.runner.Run(ctx, create, Opts{Step: "run step 3"})
	te, ok := errors.AsType[*procgroup.TimeoutError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, procgroup.SbxCreate, te.Class)
	list, err = rig.runner.List(context.Background(), Opts{})
	require.NoError(t, err)
	require.Len(t, list, 1, "the sandbox is there after the timeout")
	assert.Equal(t, "shop", list[0].Name)
}

// TestFailureSessionVMDoesNotStart is the fourth failure row: the create
// exits non-zero and sbx ls --json shows the half built sandbox stopped
// (01 1.6, REQ-1213): the call is a failure of the call, whose stderr
// went to the terminal, and the list is not empty.
func TestFailureSessionVMDoesNotStart(t *testing.T) {
	rig := failureRig(t, "fail-vm-no-start")
	_, err := rig.runner.List(context.Background(), Opts{})
	require.NoError(t, err)
	create, err := EnvCreate(rig.fx.Dir)
	require.NoError(t, err)
	_, err = rig.runner.Run(context.Background(), create, Opts{Step: "run step 3"})
	ce, ok := errors.AsType[*CallError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, "exit 1", ce.status)
	assert.Empty(t, ce.stderr, "the stderr of an interactive call is the terminal's, not captured")
	assert.Contains(t, rig.stdout.String(), "the virtual machine did not start", "it went to the terminal")
	list, err := rig.runner.List(context.Background(), Opts{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Stopped())
}
