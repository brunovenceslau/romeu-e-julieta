// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDoctorRootUnsafe: a root that breaks a rule of hostpaths'
// root check fails root-unsafe, naming the rule (REQ-0375, REQ-1277).
func TestDoctorRootUnsafe(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.settings.Root = f.home

	rep, out, err := f.output(t)

	c := check(t, rep, "root-unsafe")
	assert.Equal(t, resultFail, c.Result, out)
	assert.Contains(t, c.Message, "HOME")
	assert.Equal(t, 6, exitOf(err))
}
