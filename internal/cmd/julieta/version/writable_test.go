// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package version

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWritable answers access(2) for a directory and for a file: yes for
// one this process may write, and no once its write bits are gone,
// unless the process may write it anyway, as root may; a probe that
// creates or opens the file is the oracle.
func TestWritable(t *testing.T) {
	dir := t.TempDir()
	assert.True(t, writable(dir))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	assert.Equal(t, canCreate(dir), writable(dir), "a directory with no write bit")
	file := filepath.Join(dir, "julieta")
	require.NoError(t, os.Chmod(dir, 0o700))
	require.NoError(t, os.WriteFile(file, nil, 0o500))
	assert.Equal(t, canOpenForWrite(file), writable(file), "a file with no write bit")
}

// canCreate reports whether this process can create a file in dir,
// which is what access(2) answers for the real user, root and its
// capabilities included.
func canCreate(dir string) bool {
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	_ = f.Close()
	return os.Remove(f.Name()) == nil
}

// canOpenForWrite reports whether this process can open file to write.
func canOpenForWrite(file string) bool {
	f, err := os.OpenFile(file, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	return f.Close() == nil
}
