// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gone reports whether the process is dead: no such pid, or a zombie
// that /proc shows in state Z, as pid 1 of a container may not reap.
func gone(pid int) bool {
	if syscall.Kill(pid, 0) == syscall.ESRCH {
		return true
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	i := strings.LastIndex(string(b), ") ")
	return i >= 0 && i+2 < len(b) && b[i+2] == 'Z'
}

// TestCombinedOutputKillsTheGroupAfterTheLeader checks that a
// grandchild that outlives a leader which ended at once is killed when
// CombinedOutput returns, and that the output and the status are the
// leader's.
func TestCombinedOutputKillsTheGroupAfterTheLeader(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "pid")
	script := `sleep 60 >/dev/null 2>&1 & echo $! > "$0"; echo out; exit 3`
	cmd := exec.CommandContext(t.Context(), "sh", "-c", script, pidfile)
	out, err := CombinedOutput(cmd)
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 3, exit.ExitCode())
	assert.Equal(t, "out\n", string(out))
	b, err := os.ReadFile(pidfile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	assert.Eventually(t, func() bool { return gone(pid) }, 5*time.Second, 10*time.Millisecond, "the grandchild %d is dead", pid)
}

// TestKillGroupOfAGroupThatIsGone checks that a group that no longer
// exists is os.ErrProcessDone, which os/exec does not report.
func TestKillGroupOfAGroupThatIsGone(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "true")
	KillWithGroup(cmd)
	require.NoError(t, cmd.Run())
	assert.ErrorIs(t, KillGroup(cmd.Process.Pid), os.ErrProcessDone)
}
