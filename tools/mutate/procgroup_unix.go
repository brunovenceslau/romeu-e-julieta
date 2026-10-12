// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Non-unix builds are unsupported on purpose, as for tools/ci, whose
// lintTargets name linux and darwin only.

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// killWaitDelay is how long Wait lets the output pipes drain after the
// command is killed or exits.
const killWaitDelay = 2 * time.Second

// killWithGroup starts cmd in a process group of its own and makes its
// cancellation kill the whole group: the go command and the test binary
// it starts, which would otherwise keep running in the clone while it is
// removed. It is the same as the one of tools/ci, which lives in a main
// package and cannot be imported.
func killWithGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd.Process.Pid) }
	cmd.WaitDelay = killWaitDelay
}

// killGroup kills the process group that pid leads. A group that is
// already gone is os.ErrProcessDone, which Go does not report.
func killGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
