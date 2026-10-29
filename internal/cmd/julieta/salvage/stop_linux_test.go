// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build linux

package salvage

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// fixture starts a process named name, a shell script of that name,
// which ignores SIGTERM when ignore is set, through procgroup in a
// goroutine, and waits until it runs; the test ends it. A script's
// process takes the script's name as its comm.
func fixture(t *testing.T, name string, ignore bool) {
	t.Helper()
	script := "#!/bin/sh\n"
	if ignore {
		script += "trap '' TERM\n"
	}
	script += "while :; do sleep 0.05; done\n"
	bin := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = procgroup.Run(ctx, procgroup.Command{Class: procgroup.Unbounded, Argv: []string{bin}, Env: []string{"PATH=/usr/bin:/bin"}})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	require.Eventually(t, func() bool {
		pids, err := matching([]string{name})
		return err == nil && len(pids) == 1
	}, 5*time.Second, 10*time.Millisecond)
}

// fixtureName returns a process name no other process of the machine
// has, within the 15 bytes of a comm.
func fixtureName() string {
	return "jsalv" + strconv.Itoa(os.Getpid()%1_000_000_000)
}

// TestStopAgents: a process that ends on SIGTERM is matched and not
// killed; one that ignores it is killed after the wait; no name matches
// nothing.
func TestStopAgents(t *testing.T) {
	name := fixtureName()
	fixture(t, name, false)
	stop, err := stopAgents([]string{name}, 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, salvage.AgentStop{Matched: 1}, stop)

	fixture(t, name, true)
	start := time.Now()
	stop, err = stopAgents([]string{name}, 300*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, salvage.AgentStop{Matched: 1, Killed: 1}, stop)
	assert.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond, "SIGKILL after the wait")

	stop, err = stopAgents(nil, time.Second)
	require.NoError(t, err)
	assert.Equal(t, salvage.AgentStop{}, stop)
}

// TestRunning reads a stat line whose comm holds a space and a
// parenthesis, a zombie, and lines that are not one.
func TestRunning(t *testing.T) {
	old := procRoot
	procRoot = t.TempDir()
	t.Cleanup(func() { procRoot = old })
	for pid, stat := range map[int]string{
		1: "1 (a b) S 0 1",
		2: "2 (x) y) R 0 1",
		3: "3 (a b) Z 0 1",
		4: "4 a b S",
		5: "5 (a b)",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(procRoot, strconv.Itoa(pid)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"), []byte(stat), 0o600))
	}
	assert.True(t, running(1, []string{"a b"}))
	assert.True(t, running(2, []string{"x) y"}))
	assert.False(t, running(3, []string{"a b"}), "a zombie has ended")
	assert.False(t, running(4, []string{"a b"}))
	assert.False(t, running(5, []string{"a b"}))
	assert.False(t, running(6, []string{"a b"}), "no such process")
	assert.False(t, running(1, []string{"other"}))
	pids, err := matching([]string{"a b"})
	require.NoError(t, err)
	assert.Equal(t, []int{1}, pids)
	procRoot = filepath.Join(procRoot, "missing")
	_, err = matching([]string{"a b"})
	require.Error(t, err)
}

// TestSalvageStopAgents runs --stop-agents with the claude profile: a
// process of the agent that ignores SIGTERM is killed and the manifest
// records it; with none and a session open, agent-not-matched; with
// none and the session ended, zero and no item (REQ-0746).
func TestSalvageStopAgents(t *testing.T) {
	name := fixtureName()
	oldNames, oldWait := processes, stopWait
	processes, stopWait = []string{name}, 200*time.Millisecond
	t.Cleanup(func() { processes, stopWait = oldNames, oldWait })

	s := newSandbox(t, "claude", true)
	for _, p := range []string{".claude/projects", ".claude/todos"} {
		require.NoError(t, os.MkdirAll(filepath.Join(s.home, p), 0o700))
	}
	fixture(t, name, true)
	code, stdout, stderr := julieta(t, "salvage", "--stop-agents")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "wrote "+filepath.Join(s.dir(), "manifest.json")+" repo api salvage "+salvageID+"\n")
	m := manifestOf(t, s.dir())
	assert.Equal(t, &salvage.AgentStop{Matched: 1, Killed: 1}, m.AgentStop)
	assert.Empty(t, m.Skipped)

	for _, tc := range []struct {
		name   string
		end    time.Time
		reason string
	}{
		{"open", time.Time{}, "agent-not-matched"},
		{"ended", clock.Add(time.Minute), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSandbox(t, "claude", true)
			hooks(t, s.home, clock, tc.end)
			code, stdout, stderr := julieta(t, "salvage", "--stop-agents")
			require.Equal(t, 0, code, stderr)
			m := manifestOf(t, s.dir())
			assert.Equal(t, &salvage.AgentStop{}, m.AgentStop)
			reasons := []string{}
			for _, sk := range m.Skipped {
				if sk.Reason != "agent-path-missing" {
					reasons = append(reasons, sk.Reason)
				}
			}
			if tc.reason == "" {
				assert.Empty(t, reasons)
				return
			}
			assert.Equal(t, []string{tc.reason}, reasons)
			assert.Contains(t, stdout, "skipped agent-not-matched claude: ")
		})
	}
}

// hooks writes the record of the hooks in julieta state.
func hooks(t *testing.T, home string, start, end time.Time) {
	t.Helper()
	dir := setup.StateDir(home)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	data, err := json.Marshal(map[string]time.Time{"sessionStartAt": start, "sessionEndAt": end})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.json"), data, 0o600))
}

// manifestOf decodes the manifest of the salvage dir.
func manifestOf(t *testing.T, dir string) salvage.Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)
	var m salvage.Manifest
	require.NoError(t, json.Unmarshal(data, &m, json.RejectUnknownMembers(true)))
	return m
}
