// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package git

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestCleanEnv checks the variables CleanEnv drops and the one it
// replaces: the list git itself gives for a repository's location,
// every GIT_CONFIG variable, and a GIT_OPTIONAL_LOCKS of the caller,
// which it replaces with 0.
func TestCleanEnv(t *testing.T) {
	var stdout strings.Builder
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolGit,
		Argv:   []string{"git", "rev-parse", "--local-env-vars"},
		Env:    os.Environ(),
		Stdout: &stdout,
	})
	require.NoError(t, err)
	out := stdout.String()
	assert.ElementsMatch(t, strings.Fields(out), LocalEnv, "git rev-parse --local-env-vars")

	base := []string{"PATH=/bin", "HOME=/h", "GIT_AUTHOR_NAME=a", "GIT_OPTIONAL_LOCKS=1",
		"GIT_CONFIG_KEY_0=core.worktree", "GIT_CONFIG_VALUE_0=/elsewhere", "GIT_CONFIG_GLOBAL=/g", "GIT_CONFIG_NOSYSTEM=1"}
	for _, key := range LocalEnv {
		base = append(base, key+"=/x")
	}
	assert.Equal(t, []string{"PATH=/bin", "HOME=/h", "GIT_AUTHOR_NAME=a", "GIT_OPTIONAL_LOCKS=0"}, CleanEnv(base))

	t.Setenv("GIT_INDEX_FILE", "/from/the/process")
	assert.NotContains(t, CleanEnv(nil), "GIT_INDEX_FILE=/from/the/process", "nil is this process's environment")
	assert.Contains(t, CleanEnv(nil), "GIT_OPTIONAL_LOCKS=0")
}
