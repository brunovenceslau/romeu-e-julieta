// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteFactsCountsDirtyPaths: dirty is the number of distinct
// changed paths (03 3.10), not the number of kinds of change. In api,
// two unstaged files, one of them also staged, one untracked file and
// one assume-unchanged entry read as 4; in web, three untracked files
// read as 3, which a count of kinds would read as 1.
func TestWriteFactsCountsDirtyPaths(t *testing.T) {
	s := newSandbox(t)
	write := func(dir, name, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	api := s.repo(0)
	for _, name := range []string{"a", "b", "c"} {
		write(api, name, "one\n")
	}
	git(t, api, "add", "a", "b", "c")
	git(t, api, "commit", "-q", "-m", "files")
	write(api, "a", "staged\n")
	git(t, api, "add", "a")
	write(api, "a", "staged, then changed again\n")
	write(api, "b", "unstaged\n")
	write(api, "new", "untracked\n")
	git(t, api, "update-index", "--assume-unchanged", "c")
	write(api, "c", "hidden\n")
	for _, name := range []string{"x", "y", "z"} {
		write(s.repo(1), name, "untracked\n")
	}

	s.ok(t, "", "handoff", "write", "--facts")
	require.Len(t, s.names(t, 0), 1)
	f := s.readHandoff(t, 0, s.names(t, 0)[0])
	require.Len(t, f.Repos, 2)
	assert.Equal(t, "api", f.Repos[0].Dir)
	assert.Equal(t, 4, f.Repos[0].Dirty, "a staged and unstaged path counts once, the hidden entry counts")
	assert.Equal(t, "web", f.Repos[1].Dir)
	assert.Equal(t, 3, f.Repos[1].Dirty, "one per untracked file")
}
