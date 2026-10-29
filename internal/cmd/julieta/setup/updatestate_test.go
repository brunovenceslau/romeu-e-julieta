// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// TestUpdateStateKeepsAWarningSetupRecordsMeanwhile: while one writer
// of setup.json holds the lock to clear a warning, setup
// records a new warning; it waits (the seam says so, no sleep) and its
// warning is there after the clear, which does not undo it.
func TestUpdateStateKeepsAWarningSetupRecordsMeanwhile(t *testing.T) {
	home := t.TempDir()
	old := setup.Warning{Slug: setup.FetchFailed, Message: "api: offline"}
	_, err := setup.RecordState(home, time.Unix(1, 0), "v1", []setup.Warning{old})
	require.NoError(t, err)

	waiting := make(chan struct{})
	defer setup.SetStateOnWait(func() { close(waiting) })()
	done := make(chan error, 1)
	err = setup.UpdateState(home, func(s *setup.State) error {
		go func() {
			_, err := setup.RecordState(home, time.Unix(2, 0), "v2", []setup.Warning{{Slug: setup.FetchFailed, Message: "web: offline"}})
			done <- err
		}()
		select {
		case <-waiting: // setup is blocked behind this writer
		case <-time.After(30 * time.Second):
			t.Error("setup did not wait for the lock")
		}
		s.Warnings = slices.DeleteFunc(s.Warnings, func(w setup.Warning) bool { return w == old })
		return nil
	})
	require.NoError(t, err)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("setup never got the lock")
	}
	got, err := setup.ReadState(home)
	require.NoError(t, err)
	assert.Equal(t, []setup.Warning{{Slug: setup.FetchFailed, Message: "web: offline"}}, got.Warnings,
		"setup's new warning survives the clear, and the clear is not undone")
}

// TestUpdateStateLeavesAnUnreadableStateAlone: a setup.json that cannot
// be read is an error and stays byte for byte as it was; an error of f
// writes nothing.
func TestUpdateStateLeavesAnUnreadableStateAlone(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(setup.StateDir(home), "setup.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"at":`), 0o600))
	called := false
	require.Error(t, setup.UpdateState(home, func(*setup.State) error { called = true; return nil }))
	assert.False(t, called)
	data, _ := os.ReadFile(path)
	assert.Equal(t, `{"at":`, string(data))

	require.NoError(t, os.Remove(path))
	_, err := setup.RecordState(home, time.Unix(1, 0), "v1", nil)
	require.NoError(t, err)
	before, _ := os.ReadFile(path)
	require.Error(t, setup.UpdateState(home, func(s *setup.State) error {
		s.Version = "changed"
		return os.ErrInvalid
	}))
	after, _ := os.ReadFile(path)
	assert.Equal(t, string(before), string(after))
}
