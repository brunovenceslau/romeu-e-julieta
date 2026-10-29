// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// memoryRun returns the fixture of a run whose step 4 is done.
func memoryRun(t *testing.T) *hostTree {
	t.Helper()
	h := treeRun(t)
	require.NoError(t, makeTree(t.Context(), h.r))
	return h
}

// TestCheckMemoryPassesTheLayout: the memory dirs step 4 made, and a
// repo dir holding entries and a snapshot, pass step 5 with no warning.
func TestCheckMemoryPassesTheLayout(t *testing.T) {
	h := memoryRun(t)
	for _, p := range []string{"memory/shop-api/entries/01j9z3k8m6t2q4w5e7r9y1v3k5.md", "memory/_shared/entries/01j9z3k8m6t2q4w5e7r9y1v3k6.md", "memory/shop-web/snapshot/repo.bundle"} {
		require.NoError(t, os.WriteFile(h.in(p), []byte("x"), 0o600))
	}

	require.NoError(t, checkMemory(t.Context(), h.r))

	assert.Empty(t, h.r.warnings)
}

// TestCheckMemoryRefusesALayoutViolation is I12 and I24 in step 5: a
// memory dir that breaks the allowlist of 08 8.1 is exit 2 naming the
// offending path, escaped, and the rule; nothing is followed or
// changed. A name ".." cannot be planted in a directory, so memstore's
// own tests hold that rule.
func TestCheckMemoryRefusesALayoutViolation(t *testing.T) {
	tests := []struct {
		name string
		// plant makes the violation and returns the path step 5 names.
		plant func(t *testing.T, h *hostTree, outside string) string
		rule  string
	}{
		{
			name: "a link in entries/",
			plant: func(t *testing.T, h *hostTree, outside string) string {
				t.Helper()
				p := h.in("memory/shop-api/entries/01j9z3k8m6t2q4w5e7r9y1v3k5.md")
				require.NoError(t, os.Symlink(filepath.Join(outside, "secret"), p))
				return p
			},
			rule: "symlink",
		},
		{
			name: "a FIFO in snapshot/",
			plant: func(t *testing.T, h *hostTree, _ string) string {
				t.Helper()
				p := h.in("memory/shop-web/snapshot/fifo")
				require.NoError(t, syscall.Mkfifo(p, 0o600))
				return p
			},
			rule: "fifo",
		},
		{
			name: "a top-level name the allowlist lacks",
			plant: func(t *testing.T, h *hostTree, _ string) string {
				t.Helper()
				p := h.in("memory/shop-api/notes")
				require.NoError(t, os.Mkdir(p, 0o700))
				return p
			},
			rule: "top-level",
		},
		{
			name: "a dotfile",
			plant: func(t *testing.T, h *hostTree, _ string) string {
				t.Helper()
				p := h.in("memory/shop-api/entries/.env")
				require.NoError(t, os.WriteFile(p, nil, 0o600))
				return p
			},
			rule: "dotfile",
		},
		{
			name: "a .git in the shared dir",
			plant: func(t *testing.T, h *hostTree, _ string) string {
				t.Helper()
				p := h.in("memory/_shared/.git")
				require.NoError(t, os.Mkdir(p, 0o700))
				return p
			},
			rule: "git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := memoryRun(t)
			outside := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("never read"), 0o600))
			p := tt.plant(t, h, outside)
			before := snapshot(t, h.env)

			err := checkMemory(t.Context(), h.r)

			require.Error(t, err)
			assert.Equal(t, 2, cli.ExitCode(err), err.Error())
			assert.Contains(t, err.Error(), "memory-layout: ")
			assert.Contains(t, err.Error(), tt.rule+" "+p)
			assert.Equal(t, before, snapshot(t, h.env), "nothing is changed")
		})
	}
}

// TestCheckMemoryEscapesThePath: a name an agent wrote cannot forge a
// line of the refusal: its control characters are escaped (I29).
func TestCheckMemoryEscapesThePath(t *testing.T) {
	h := memoryRun(t)
	require.NoError(t, os.WriteFile(h.in("memory/shop-api/x\n\x1b[31mok"), nil, 0o600))

	err := checkMemory(t.Context(), h.r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `x\n\u001b[31mok`)
	assert.NotContains(t, err.Error(), "\x1b")
}

// TestCheckMemoryWarns: what memstore.Check warns refuses nothing, and
// step 5 keeps it as a note line: a leftover .tmp-<ULID> of julieta,
// which is never read (REQ-0654), and a legacy handoff/ over its cap.
func TestCheckMemoryWarns(t *testing.T) {
	tests := []struct {
		name string
		// plant makes the case and returns the note step 5 keeps.
		plant func(t *testing.T, h *hostTree) string
	}{
		{
			name: "a leftover temporary file",
			plant: func(t *testing.T, h *hostTree) string {
				t.Helper()
				p := h.in("memory/_shared/entries/.tmp-01j9z3k8m6t2q4w5e7r9y1v3k5")
				require.NoError(t, os.WriteFile(p, []byte("half written"), 0o600))
				return "note temp-file " + p + ": a leftover julieta temporary file, never read; julieta setup removes it\n"
			},
		},
		{
			name: "a handoff/ over its cap",
			plant: func(t *testing.T, h *hostTree) string {
				t.Helper()
				dir := h.in("memory/shop-api/handoff")
				require.NoError(t, os.Mkdir(dir, 0o700))
				for i := range memstore.MaxHandoffFiles + 1 {
					require.NoError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d", i)), nil, 0o600))
				}
				return fmt.Sprintf("note handoff-cap %s: handoff/ is over its cap of %d files\n", dir, memstore.MaxHandoffFiles)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := memoryRun(t)
			want := tt.plant(t, h)

			require.NoError(t, checkMemory(t.Context(), h.r))

			var out bytes.Buffer
			for _, w := range h.r.warnings {
				w(cli.NewWriter(&out))
			}
			assert.Equal(t, want, out.String())
		})
	}
}
