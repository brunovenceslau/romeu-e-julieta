// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// syncRuns records the runs of the command sync of this test binary,
// and syncResult is what it returns. The tests that read them do not
// run in parallel.
var (
	syncRuns   [][]string
	syncResult error
)

// The command sync of this test binary alone: it stands for romeu sync,
// which this package does not link, and records its flags.
var _ = cli.Register(func() cli.Command {
	c := cli.Define(cli.PreflightRuns, cli.LockTaken, cli.AppendsNothing(), cli.SnapshotNone, cli.Converges)
	c.Binary, c.Name = "romeu", "sync"
	c.NoJourney = "a fixture of the tests of internal/cmd/romeu/upgrade"
	c.New = func(*cli.Env) *cobra.Command {
		var all bool
		cmd := &cobra.Command{
			Use:   "sync [--all]",
			Short: "a command of the tests",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				if all {
					syncRuns = append(syncRuns, []string{"--all"})
				} else {
					syncRuns = append(syncRuns, nil)
				}
				return syncResult
			},
		}
		cmd.Flags().BoolVar(&all, "all", false, "every project")
		return cmd
	}
	return c
}())

// TestFinish holds REQ-1330 and REQ-0061: romeu upgrade --finish
// <running version>, the process the swap step starts, writes
// toolchain.json with no prompt, from the binary it runs as, then runs
// the steps of romeu sync --all, whose result is the command's.
func TestFinish(t *testing.T) {
	m := newMachine(t)
	_ = use(t, m, nil, 0, nil)
	syncRuns, syncResult = nil, nil
	t.Cleanup(func() { syncRuns, syncResult = nil, nil })

	got := runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")

	require.Equal(t, 0, got.code, got.stderr)
	assert.Equal(t, "upgraded      romeu v1.0.0-alpha.0 -> v1.0.0-alpha.1\nacknowledged  "+
		filepath.Join(m.stateHome, "romeu", "toolchain.json")+"\n", got.stdout)
	assert.Equal(t, [][]string{{"--all"}}, syncRuns)
	st, err := state.OpenRead(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	defer func() { _ = st.Close() }()
	rec, err := approve.Load(st)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "v1.0.0-alpha.1", rec.Romeu.Version)
	assert.Equal(t, "0.49.2", rec.SbxVersion)

	syncResult = cli.Usage.New("sync refused")
	got = runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")
	assert.Equal(t, 2, got.code, "the result of sync is the command's")
}

// TestFinishStopsAtTheLock: a finish that cannot take the state lock
// writes nothing and runs no sync.
func TestFinishStopsAtTheLock(t *testing.T) {
	m := newMachine(t)
	_ = use(t, m, nil, 0, nil)
	syncRuns = nil
	t.Cleanup(func() { syncRuns = nil })
	st, err := state.Open(filepath.Join(m.stateHome, "romeu"))
	require.NoError(t, err)
	lk, err := st.Lock()
	require.NoError(t, err)
	t.Cleanup(func() { lk.Unlock(); _ = st.Close() })

	got := runCommand(t, m, nil, "", "upgrade", "--finish", "v1.0.0-alpha.0")

	assert.Equal(t, state.Locked.Exit, got.code, got.stderr)
	assert.Contains(t, got.stderr, "error "+state.Locked.ID+" ")
	assert.Empty(t, syncRuns)
	assert.NoFileExists(t, filepath.Join(m.stateHome, "romeu", "toolchain.json"))
}
