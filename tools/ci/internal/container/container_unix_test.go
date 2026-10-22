// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package container

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestReadNoticesRefusesAFIFO holds the reader off a FIFO, which would
// block a read for ever, and the step with it.
func TestReadNoticesRefusesAFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notices")
	require.NoError(t, syscall.Mkfifo(path, 0o600))
	_, err := readNotices(path)
	require.Error(t, err)
	require.ErrorContains(t, err, "not a regular file")
	assert.NotErrorIs(t, err, check.ErrInfrastructure)
}

// TestAStepWhoseNoticeFileIsAFIFOFails is the same through the step.
func TestAStepWhoseNoticeFileIsAFIFOFails(t *testing.T) {
	s := fakeRun(t, `rm "$`+NoticeFileEnv+`"; mkfifo "$`+NoticeFileEnv+`"; `+
		`printf '%s\n' '{"Action":"run","Test":"TestA"}' '{"Action":"pass","Test":"TestA"}' '{"Action":"pass"}'`)
	_, err := s.Run(t.Context(), t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}
