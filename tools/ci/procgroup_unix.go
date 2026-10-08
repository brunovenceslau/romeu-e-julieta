// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// killWaitDelay is how long Wait lets the output pipes drain after the
// command is killed or exits.
const killWaitDelay = 2 * time.Second

// killWithGroup starts cmd in a process group of its own and makes its
// cancellation kill the whole group, so a grandchild that holds the
// output pipe cannot keep Wait past the deadline. WaitDelay bounds what
// remains. The caller's signals do not reach the group any more, so
// main cancels the context on SIGINT and SIGTERM, which kills it.
func killWithGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = killWaitDelay
}
