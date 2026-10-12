// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package procgroup runs a command in a process group of its own and
// kills the whole group, so that no grandchild outlives the command:
// the steps of tools/ci and the go commands of tools/mutants both run
// this way. A grandchild that calls setsid leaves the group, and the
// group kill does not reach it. Non-unix builds are unsupported on
// purpose: the lint targets of tools/ci name linux and darwin only, so
// no other platform runs these tools.
package procgroup

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// WaitDelay is how long Wait lets the output pipes drain after the
// command is killed or exits.
const WaitDelay = 2 * time.Second

// KillWithGroup starts cmd, which exec.CommandContext made, in a
// process group of its own and makes its cancellation kill the whole
// group, so a grandchild that holds the output pipe cannot keep Wait
// past the deadline. WaitDelay bounds what remains. The caller's
// signals do not reach the group any more, so a caller cancels the
// context on SIGINT and SIGTERM, which kills it.
func KillWithGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return KillGroup(cmd.Process.Pid) }
	cmd.WaitDelay = WaitDelay
}

// KillGroup kills the process group that pid leads. Go can call Cancel
// after Wait reaped the leader, when the group is already gone: that
// is os.ErrProcessDone, which Go does not report, and not an error.
func KillGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// CombinedOutput runs cmd as KillWithGroup sets it up and returns its
// standard output and standard error together, then kills the group.
//
// os/exec calls Cancel only while the leader runs, so a grandchild that
// outlives a leader that ended, and holds the output pipe until
// WaitDelay, is still ours to kill. A group that is gone is not an
// error (ESRCH). While the group has members its pgid is not reused,
// so -pgid reaches only this group. The window after the group has
// fully emptied (between the last reap and the kill, which a reuse
// would need a full pid wrap to hit) is accepted, as it is on the
// Cancel path. Other errors of that kill are dropped on purpose: the
// command's result already stands and WaitDelay has bounded the wait.
func CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	KillWithGroup(cmd)
	out, err := cmd.CombinedOutput()
	if cmd.Process != nil {
		_ = KillGroup(cmd.Process.Pid)
	}
	return out, err
}
