// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// tempFile plants a temporary file of julieta whose ULID is of the time
// at in the dir sub of the memory dir mem, and returns its path.
func tempFile(t *testing.T, mem, sub string, at time.Time) string {
	t.Helper()
	u, err := canon.NewULID(at, canon.NewULIDSource(bytes.NewReader(make([]byte, 10))))
	require.NoError(t, err)
	p := filepath.Join(mem, sub, memstore.TempName(u))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
	require.NoError(t, os.WriteFile(p, []byte("partial"), 0o600))
	return p
}

// TestSetupRemovesOldTemps removes a .tmp-<ULID> older than the start of
// setup and keeps a newer one, which a julieta that runs now may still
// be writing (REQ-0654).
func TestSetupRemovesOldTemps(t *testing.T) {
	s := newSandbox(t)
	mem := s.m.Repos[0].Memory
	old := tempFile(t, mem, "entries", clock.Add(-time.Second))
	newer := tempFile(t, mem, "handoff", clock.Add(time.Second))
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Unusable)
	assert.NoFileExists(t, old)
	assert.FileExists(t, newer)
}

// TestSetupMemoryCheck leaves the sandbox unusable, exit 1, when a
// memory dir is not mounted or breaks the layout allowlist (04 4.3,
// REQ-0400), and the other steps still run.
func TestSetupMemoryCheck(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, os.Remove(s.m.Repos[1].Memory))
	require.NoError(t, os.Symlink("/etc", filepath.Join(s.m.Repos[0].Memory, "entries")))
	code, rep, stderr := setupJSON(t)
	assert.Equal(t, 1, code, stderr)
	require.Len(t, rep.Unusable, 2, rep.Unusable)
	assert.True(t, strings.HasPrefix(rep.Unusable[0], "memory check of api: "), rep.Unusable[0])
	assert.Contains(t, rep.Unusable[0], "symlink entries")
	assert.True(t, strings.HasPrefix(rep.Unusable[1], "memory check of web: memstore: the memory dir is not mounted"), rep.Unusable[1])
	require.Len(t, rep.Repos, 2)
	assert.True(t, rep.Repos[0].Fetched)
}

// TestSetupAgentMemoryDir warns agent-memory-writable when the agent's
// own memory dir is writable or not empty, and not when it is absent
// or read-only and empty (04 4.3, REQ-0400; 08 8.1, REQ-0658).
func TestSetupAgentMemoryDir(t *testing.T) {
	s := newSandbox(t)
	dir := filepath.Join(s.home, ".agent", "memory")
	defer setup.SetAgentMemoryDir(func(home, agent string) (string, bool) {
		assert.Equal(t, s.home, home)
		assert.Equal(t, "sbx-kit-claude", agent)
		return dir, true
	})()
	warnings := func() []setup.Warning {
		t.Helper()
		code, rep, stderr := setupJSON(t)
		require.Equal(t, 0, code, stderr)
		var out []setup.Warning
		for _, w := range rep.Warnings {
			if w.Slug == setup.AgentMemoryWritable {
				out = append(out, w)
			}
		}
		return out
	}
	assert.Empty(t, warnings(), "absent")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	assert.Equal(t, []setup.Warning{{Slug: setup.AgentMemoryWritable, Message: dir + " is writable"}}, warnings())
	code, _, stderr := julieta(t.Context(), "setup")
	require.Equal(t, 0, code)
	assert.Contains(t, stderr, "warning agent-memory-writable: "+dir+" is writable\n")
	if os.Geteuid() == 0 {
		return // root writes a read-only directory
	}
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	assert.Empty(t, warnings(), "read-only and empty")
	require.NoError(t, os.Chmod(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MEMORY.md"), nil, 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	assert.Equal(t, []setup.Warning{{Slug: setup.AgentMemoryWritable, Message: dir + " is not empty"}}, warnings())
}
