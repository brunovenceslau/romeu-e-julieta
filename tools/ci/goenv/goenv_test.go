// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package goenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLists pins the three lists, so a change to one is a change to a
// test too, read with the callers its doc names.
func TestLists(t *testing.T) {
	assert.Equal(t, []string{
		"PATH", "HOME", "TMPDIR",
		"XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME",
		"MISE_CACHE_DIR", "MISE_DATA_DIR", "MISE_STATE_DIR",
		"GOPATH", "GOCACHE", "GOMODCACHE",
	}, PassThrough)
	assert.Equal(t, []string{"GOENV=off", "GOTOOLCHAIN=local"}, Pin)
	assert.Equal(t, []string{"GOWORK=off", "GOFLAGS=-mod=readonly"}, Build)
}
