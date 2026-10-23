// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// The rows of steps 1 to 4 belong to other packages; the tests give the
// stubs rows of the same exit codes as 01 1.5.
var (
	rowToolchain  = cli.Row{ID: "RJ-302", Slug: "toolchain-approval-required", Exit: 3}
	rowPromotion  = cli.Row{ID: "RJ-304", Slug: "interrupted-promotion-abandoned", Exit: 4}
	rowDrift      = cli.Row{ID: "RJ-303", Slug: "drift-detected", Exit: 4}
	rowWidening   = cli.Row{ID: "RJ-301", Slug: "project-approval-required", Exit: 3}
	errNoSbxStubs = errors.New("the stub holds no listing")
)

// fakeSbx is an Sbx that returns what it holds and counts its calls.
type fakeSbx struct {
	list     []sbxdrv.Sandbox
	listErr  error
	policies []byte
	secrets  []byte
	calls    []string
}

func (f *fakeSbx) List(context.Context) ([]sbxdrv.Sandbox, error) {
	f.calls = append(f.calls, "ls")
	return f.list, f.listErr
}

func (f *fakeSbx) PolicyList(context.Context) ([]byte, error) {
	f.calls = append(f.calls, "policy")
	if f.policies == nil {
		return nil, errNoSbxStubs
	}
	return f.policies, nil
}

func (f *fakeSbx) SecretList(context.Context) ([]byte, error) {
	f.calls = append(f.calls, "secret")
	if f.secrets == nil {
		return nil, errNoSbxStubs
	}
	return f.secrets, nil
}

// passingSbx holds the listings of a well-kept host with no sandbox.
func passingSbx(t *testing.T) *fakeSbx {
	t.Helper()
	return &fakeSbx{
		policies: synthetic(t, "policies-recorded.json"),
		secrets:  synthetic(t, "secrets-no-global.txt"),
	}
}

// synthetic reads testdata/synthetic/<name> without its marker lines,
// the lines that start with "#" before the body.
func synthetic(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "synthetic", name))
	require.NoError(t, err)
	body := string(raw)
	for strings.HasPrefix(body, "#") {
		_, rest, _ := strings.Cut(body, "\n")
		body = rest
	}
	return []byte(body)
}

// quietHost is an Input whose home and root hold nothing the host checks
// look at.
func quietHost(t *testing.T) Input {
	t.Helper()
	dir := t.TempDir()
	home, root := filepath.Join(dir, "home"), filepath.Join(dir, "root")
	require.NoError(t, os.MkdirAll(home, 0o755))
	require.NoError(t, os.MkdirAll(root, 0o755))
	return Input{
		Command: CmdSync, Project: "shop", Primary: "shop-api", Root: root, Home: home, OS: "linux",
		UsedServices: []string{"svc-alpha", "svc-beta"},
	}
}

// trace records the steps that ran, and fails the ones in fail.
type trace struct {
	ran  []int
	fail map[int]cli.Row
}

func (tr *trace) step(n int) func(context.Context) error {
	return func(context.Context) error {
		tr.ran = append(tr.ran, n)
		if row, ok := tr.fail[n]; ok {
			return row.New("step failed")
		}
		return nil
	}
}

func (tr *trace) deps(t *testing.T) Deps {
	t.Helper()
	return Deps{
		Toolchain: tr.step(1), Promotion: tr.step(2), Drift: tr.step(3), Widening: tr.step(4),
		Sbx: passingSbx(t),
	}
}

// TestPreflightStepOrder runs the six steps in the order of 01 1.5 and
// stops at the first failure with the exit of its row (REQ-0073).
func TestPreflightStepOrder(t *testing.T) {
	tests := []struct {
		name     string
		fail     map[int]cli.Row
		wantRan  []int
		wantExit int
	}{
		{"all pass", nil, []int{1, 2, 3, 4}, 0},
		{"step 1 stops the rest", map[int]cli.Row{1: rowToolchain}, []int{1}, 3},
		{"step 2", map[int]cli.Row{2: rowPromotion}, []int{1, 2}, 4},
		{"step 3", map[int]cli.Row{3: rowDrift}, []int{1, 2, 3}, 4},
		{"step 4", map[int]cli.Row{4: rowWidening}, []int{1, 2, 3, 4}, 3},
		{"the first failure wins", map[int]cli.Row{2: rowPromotion, 4: rowWidening}, []int{1, 2}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &trace{fail: tt.fail}
			_, err := Run(t.Context(), tr.deps(t), quietHost(t))
			assert.Equal(t, tt.wantRan, tr.ran)
			assert.Equal(t, tt.wantExit, cli.ExitCode(err))
		})
	}
}

// TestPreflightStepFiveAndSixStopInOrder shows that step 5 runs before
// step 6: a failing host check means sbx ls is never read.
func TestPreflightStepFiveAndSixStopInOrder(t *testing.T) {
	sbx := passingSbx(t)
	sbx.secrets = synthetic(t, "secrets-global-used.txt")
	tr := &trace{}
	d := tr.deps(t)
	d.Sbx = sbx
	_, err := Run(t.Context(), d, quietHost(t))
	testsupport.RequireID(t, "RJ-329", err)
	assert.Equal(t, []int{1, 2, 3, 4}, tr.ran)
	assert.NotContains(t, sbx.calls, "ls", "step 6 never ran")
}

// TestPreflightProtectiveCommands shows REQ-0078: a protective command,
// and the salvage half of rm, recreate and retire, print a failure of
// steps 1, 4 and 5 and go on, and stop at step 2, 3 or 6.
func TestPreflightProtectiveCommands(t *testing.T) {
	type input struct {
		cmd    Command
		salvag bool
	}
	protective := []input{
		{CmdSalvage, false}, {CmdSalvageFromHost, false}, {CmdStop, false}, {CmdPull, false},
		{CmdRm, true}, {CmdRecreate, true}, {CmdRetire, true},
	}
	for _, in := range protective {
		name := string(in.cmd)
		if in.salvag {
			name += " (salvage half)"
		}
		t.Run(name+" prints steps 1, 4 and 5", func(t *testing.T) {
			tr := &trace{fail: map[int]cli.Row{1: rowToolchain, 4: rowWidening}}
			d := tr.deps(t)
			sbx := passingSbx(t)
			sbx.secrets = synthetic(t, "secrets-global-used.txt")
			d.Sbx = sbx
			host := quietHost(t)
			host.Command, host.SalvageHalf = in.cmd, in.salvag
			res, err := Run(t.Context(), d, host)
			require.NoError(t, err)
			require.Len(t, res.Printed, 3)
			assert.Equal(t, []int{1, 4, 5}, []int{res.Printed[0].Step, res.Printed[1].Step, res.Printed[2].Step})
			assert.Equal(t, []int{1, 2, 3, 4}, tr.ran)
		})
		for _, stop := range []int{2, 3} {
			t.Run(name+" stops at a failing step "+string(rune('0'+stop)), func(t *testing.T) {
				tr := &trace{fail: map[int]cli.Row{stop: rowDrift}}
				host := quietHost(t)
				host.Command, host.SalvageHalf = in.cmd, in.salvag
				_, err := Run(t.Context(), tr.deps(t), host)
				assert.Equal(t, 4, cli.ExitCode(err))
			})
		}
	}
	t.Run("a command that widens stops at step 1, 4 and 5", func(t *testing.T) {
		for _, cmd := range []Command{CmdSync, CmdRun, CmdRm, CmdRecreate, CmdRetire} {
			tr := &trace{fail: map[int]cli.Row{4: rowWidening}}
			host := quietHost(t)
			host.Command = cmd
			res, err := Run(t.Context(), tr.deps(t), host)
			assert.Equal(t, 3, cli.ExitCode(err), cmd)
			assert.Empty(t, res.Printed, cmd)
		}
	})
}

// TestPreflightRunSteps shows that sync can run step 1 apart from steps
// 2 to 6 (REQ-0074).
func TestPreflightRunSteps(t *testing.T) {
	tr := &trace{}
	d := tr.deps(t)
	_, err := RunSteps(t.Context(), d, quietHost(t), 1, 1)
	require.NoError(t, err)
	assert.Equal(t, []int{1}, tr.ran)
	_, err = RunSteps(t.Context(), d, quietHost(t), 2, 6)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4}, tr.ran)
}

// TestPreflightRefusesWhatACallerGotWrong shows that a nil step, an
// unknown command and a range with no steps are defects, not passes.
func TestPreflightRefusesWhatACallerGotWrong(t *testing.T) {
	tr := &trace{}
	nilStep := tr.deps(t)
	nilStep.Drift = nil
	salvageHalfOfSync := quietHost(t)
	salvageHalfOfSync.SalvageHalf = true
	unknown := quietHost(t)
	unknown.Command = "frobnicate"
	tests := []struct {
		name     string
		deps     Deps
		in       Input
		from, to int
	}{
		{"a nil step", nilStep, quietHost(t), 1, 6},
		{"an unknown command", tr.deps(t), unknown, 1, 6},
		{"a salvage half of sync", tr.deps(t), salvageHalfOfSync, 1, 6},
		{"step 0", tr.deps(t), quietHost(t), 0, 6},
		{"step 7", tr.deps(t), quietHost(t), 1, 7},
		{"a reversed range", tr.deps(t), quietHost(t), 3, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RunSteps(t.Context(), tt.deps, tt.in, tt.from, tt.to)
			testsupport.RequireID(t, cli.Internal.ID, err)
			assert.Empty(t, tr.ran)
		})
	}
}
