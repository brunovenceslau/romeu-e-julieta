// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// The test seams of stopAgents (10 10.4): the proc file system it
// reads and how often it looks again while it waits.
var (
	procRoot = "/proc"
	stopPoll = 100 * time.Millisecond
)

// killWait bounds the wait for the processes SIGKILL ends, which the
// kernel ends at once unless they are stuck in the kernel.
const killWait = 5 * time.Second

// stopAgents stops the processes whose name is one of names (08 8.5
// step 1, REQ-0695): SIGTERM to each, a wait of up to wait for them to
// end, then SIGKILL to those left. A process's name is its comm in
// /proc/<pid>/stat, exact; julieta's own process is never one, and a
// zombie, which holds nothing, has ended. Reading /proc by hand, rather
// than through a process library, is about twenty lines and needs no
// new module (decided while building T061; ADR 0038). It returns the
// number matched and the number that needed SIGKILL.
func stopAgents(names []string, wait time.Duration) (salvage.AgentStop, error) {
	if len(names) == 0 {
		return salvage.AgentStop{}, nil
	}
	pids, err := matching(names)
	if err != nil {
		return salvage.AgentStop{}, err
	}
	for _, pid := range pids {
		signal(pid, syscall.SIGTERM)
	}
	left := waitGone(pids, names, wait)
	for _, pid := range left {
		signal(pid, syscall.SIGKILL)
	}
	// A process SIGKILL has not ended yet, one stuck in the kernel, is
	// counted as killed and does not stop the salvage, which is when its
	// work most needs keeping.
	waitGone(left, names, killWait)
	return salvage.AgentStop{Matched: len(pids), Killed: len(left)}, nil
}

// signal sends sig to pid; a process that has ended meanwhile is no
// error.
func signal(pid int, sig syscall.Signal) {
	_ = syscall.Kill(pid, sig)
}

// waitGone waits up to wait for the processes pids to end, and returns
// those still running then.
func waitGone(pids []int, names []string, wait time.Duration) []int {
	deadline := time.Now().Add(wait)
	for {
		left := slices.DeleteFunc(slices.Clone(pids), func(pid int) bool { return !running(pid, names) })
		if len(left) == 0 || !time.Now().Before(deadline) {
			return left
		}
		time.Sleep(stopPoll)
	}
}

// matching returns the pids of the running processes whose name is one
// of names, julieta's own left out.
func matching(names []string) ([]int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		if running(pid, names) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// running reports whether pid is a process, not a zombie, whose name is
// one of names, from /proc/<pid>/stat: "<pid> (<comm>) <state> ...",
// the comm between the first "(" and the last ")".
func running(pid int, names []string) bool {
	data, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	s := string(data)
	open, end := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || end < open {
		return false
	}
	state := strings.Fields(s[end+1:])
	return slices.Contains(names, s[open+1:end]) && len(state) > 0 && state[0] != "Z" && state[0] != "X"
}
