// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFreeNameGivesATakenNameASuffix: a reflog commit whose name is
// taken gets the first free suffix -<k>, so no commit is lost to a
// name (decided while building T060); a name below a name of the set
// has none free.
func TestFreeNameGivesATakenNameASuffix(t *testing.T) {
	s := newNameSet()
	s.add("refs/salvage-reflog/worktrees/0/1")
	s.add("refs/salvage-reflog/worktrees/0/1-1")
	s.add("refs/salvage-reflog/x/2/y")

	got, ok := s.free("refs/salvage-reflog/main/1")
	assert.True(t, ok)
	assert.Equal(t, "refs/salvage-reflog/main/1", got)
	got, ok = s.free("refs/salvage-reflog/worktrees/0/1")
	assert.True(t, ok)
	assert.Equal(t, "refs/salvage-reflog/worktrees/0/1-2", got)
	got, ok = s.free("refs/salvage-reflog/x/2")
	assert.True(t, ok, "a directory of the set takes the suffix")
	assert.Equal(t, "refs/salvage-reflog/x/2-1", got)
	_, ok = s.free("refs/salvage-reflog/worktrees/0/1/3")
	assert.False(t, ok, "below a name of the set")
}
