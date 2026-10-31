// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorRecordUnreadable: a project record that does not decode
// fails record-unreadable, naming the file, and is left as it was
// (REQ-0391).
func TestDoctorRecordUnreadable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.store(t)
	p := filepath.Join(f.stateDir(), "projects", "shop.json")
	require.NoError(t, os.WriteFile(p, []byte("{"), 0o600))

	rep, out, err := f.output(t)

	c := check(t, rep, "record-unreadable")
	assert.Equal(t, resultFail, c.Result, out)
	assert.Contains(t, c.Message, "shop.json")
	assert.Equal(t, 6, exitOf(err))
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "{", string(b), "the record is never written over")
}
