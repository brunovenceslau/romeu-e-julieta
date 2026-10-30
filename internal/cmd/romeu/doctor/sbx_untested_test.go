// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
)

// TestDoctorSbxUntested: a live sbx version outside the tested window
// is the warning sbx-untested, naming the version and the recorded
// range, and changes no exit (REQ-0367, 10 10.3).
func TestDoctorSbxUntested(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.sbx.release = sbxdrv.Release{Major: 0, Minor: 50, Patch: 1}

	rep, out, err := f.output(t)

	require.NoError(t, err, out)
	c := check(t, rep, "sbx-untested")
	assert.Equal(t, resultWarn, c.Result)
	assert.Contains(t, c.Message, "0.50.1")
	assert.Contains(t, c.Message, sbxdrv.RecordedVersions[0])
}
