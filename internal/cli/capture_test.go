// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCaptureStderrDrains writes about 1 MiB to os.Stderr while it is
// captured, far past the size of a pipe, and finds all of it on the
// writer after the restore: the drain keeps the write from blocking.
func TestCaptureStderrDrains(t *testing.T) {
	old := os.Stderr
	var got bytes.Buffer
	restore := captureStderr(&got)
	assert.NotSame(t, old, os.Stderr)
	want := bytes.Repeat([]byte("x"), 1<<20)
	n, err := os.Stderr.Write(want)
	restore()
	require.NoError(t, err)
	assert.Equal(t, len(want), n)
	assert.Same(t, old, os.Stderr)
	assert.Equal(t, want, got.Bytes())
}

// TestCaptureStderrWithNothingWritten writes nothing to the writer.
func TestCaptureStderrWithNothingWritten(t *testing.T) {
	var got bytes.Buffer
	captureStderr(&got)()
	assert.Empty(t, got.Bytes())
}
