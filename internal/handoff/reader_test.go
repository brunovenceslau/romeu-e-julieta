// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// TestReadLatest answers with the greatest ULID among the clear and
// final files over every dir as the narrative, labelled with its dir,
// and the greatest among the facts files as the newest facts; a later
// facts file does not hide the narrative (REQ-0680, REQ-0685).
func TestReadLatest(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, a, 1*time.Minute, file(handoff.Clear, 1*time.Minute, api('a', 0)))
	want := put(t, b, 2*time.Minute, file(handoff.Final, 2*time.Minute, api('b', 0)))
	put(t, a, 3*time.Minute, file(handoff.Facts, 3*time.Minute, api('c', 0)))
	facts := put(t, b, 4*time.Minute, file(handoff.Facts, 4*time.Minute, api('d', 0)))
	got, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: a}, {Label: "web", Path: b}})
	require.NoError(t, err)
	require.NotNil(t, got.Narrative)
	assert.Equal(t, want, got.Narrative.ID)
	assert.Equal(t, "web", got.Narrative.Repo)
	assert.Equal(t, handoff.Final, got.Narrative.File.Kind)
	require.NotNil(t, got.Facts)
	assert.Equal(t, facts, got.Facts.ID)
	assert.Equal(t, "web", got.Facts.Repo)
}

// TestReadLatestNone answers none for memory dirs without handoffs, and
// a facts file alone leaves the narrative nil.
func TestReadLatestNone(t *testing.T) {
	a := t.TempDir()
	got, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: a}})
	require.NoError(t, err)
	assert.Equal(t, handoff.Latest{}, got)
	put(t, a, time.Minute, file(handoff.Facts, time.Minute, api('a', 0)))
	got, err = handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: a}})
	require.NoError(t, err)
	assert.Nil(t, got.Narrative)
	assert.NotNil(t, got.Facts)
}

// TestReadLatestReadsOnlyTheChosenFiles leaves a file that is not the
// greatest of its kind unread, since the kind is in the name.
func TestReadLatestReadsOnlyTheChosenFiles(t *testing.T) {
	a := t.TempDir()
	old := id(t, time.Minute)
	require.NoError(t, os.MkdirAll(filepath.Join(a, "handoff"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(a, "handoff", handoff.FileName(old, handoff.Clear)), []byte("not a handoff"), 0o600))
	put(t, a, 2*time.Minute, file(handoff.Clear, 2*time.Minute, api('a', 0)))
	got, err := handoff.ReadLatest([]handoff.Dir{{Label: "api", Path: a}})
	require.NoError(t, err)
	assert.Equal(t, id(t, 2*time.Minute), got.Narrative.ID)
}

// TestReadLatestRefuses names the memory dir of a file or a layout the
// reader refuses.
func TestReadLatestRefuses(t *testing.T) {
	a := t.TempDir()
	dirs := []handoff.Dir{{Label: "api", Path: a}}
	dir := filepath.Join(a, "handoff")
	require.NoError(t, os.MkdirAll(dir, 0o700))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), nil, 0o600))
	_, err := handoff.ReadLatest(dirs)
	require.ErrorIs(t, err, memstore.ErrLayout)
	assert.Contains(t, err.Error(), "the memory dir of api")
	require.NoError(t, os.Remove(filepath.Join(dir, "notes.md")))

	// A temporary name is never read.
	require.NoError(t, os.WriteFile(filepath.Join(dir, memstore.TempName(id(t, 0))), nil, 0o600))
	_, err = handoff.ReadLatest(dirs)
	require.NoError(t, err)

	// A chosen file that does not decode, and a kind that is not the
	// name's.
	u := id(t, time.Hour)
	require.NoError(t, os.WriteFile(filepath.Join(dir, handoff.FileName(u, handoff.Clear)), []byte("x"), 0o600))
	_, err = handoff.ReadLatest(dirs)
	require.ErrorIs(t, err, handoff.ErrInvalid)
	data, err := handoff.Encode(file(handoff.Final, time.Hour, api('a', 0)))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, handoff.FileName(u, handoff.Clear)), data, 0o600))
	_, err = handoff.ReadLatest(dirs)
	require.ErrorIs(t, err, handoff.ErrInvalid)
	assert.Contains(t, err.Error(), "not the one in the file's name")

	// A symlinked handoff/ and a memory dir that is not there.
	require.NoError(t, os.RemoveAll(dir))
	require.NoError(t, os.Symlink(a, dir))
	_, err = handoff.ReadLatest(dirs)
	require.ErrorIs(t, err, memstore.ErrLayout)
	_, err = handoff.ReadLatest([]handoff.Dir{{Label: "gone", Path: filepath.Join(a, "gone")}})
	require.ErrorIs(t, err, memstore.ErrUnmounted)
}

// TestParseName reads the grammar of a handoff file name.
func TestParseName(t *testing.T) {
	u := id(t, 0)
	n, ok := handoff.ParseName(u.String() + "-facts.md")
	assert.True(t, ok)
	assert.Equal(t, handoff.Name{ID: u, Kind: handoff.Facts}, n)
	for _, bad := range []string{u.String() + ".md", u.String() + "-other.md", "x-clear.md", u.String() + "-clear.md.bak",
		"01J9Z3K8M6T2Q4W5E7R9Y1V3K5-clear.md"} {
		_, ok := handoff.ParseName(bad)
		assert.False(t, ok, bad)
	}
}
