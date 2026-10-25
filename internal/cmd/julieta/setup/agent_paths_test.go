// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// claudeSandbox is a sandbox whose manifest names the agent handle of
// the claude profile, the workload kit of the pin file.
func claudeSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := newSandbox(t)
	s.m.Agent = "sbx-kit-claude"
	s.setManifest(t)
	return s
}

// slugs returns the warnings of rep whose slug is slug.
func slugs(rep setup.Report, slug string) []setup.Warning {
	var out []setup.Warning
	for _, w := range rep.Warnings {
		if w.Slug == slug {
			out = append(out, w)
		}
	}
	return out
}

// TestSetupAgentPaths holds the step of the required agent-profile
// paths (04 4.3, REQ-0401; 08 8.5 step 5): one agent-path-missing for
// each required path that is absent, exit 0, the warning recorded for
// status and the next SessionStart; none once they exist, and none for
// an optional path.
func TestSetupAgentPaths(t *testing.T) {
	s := claudeSandbox(t)
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	missing := slugs(rep, setup.AgentPathMissing)
	require.Len(t, missing, 2, "%v", rep.Warnings)
	assert.True(t, strings.HasPrefix(missing[0].Message, filepath.Join(s.home, ".claude/projects")+", "), missing[0].Message)
	assert.True(t, strings.HasPrefix(missing[1].Message, filepath.Join(s.home, ".claude/todos")+", "), missing[1].Message)
	assert.Contains(t, missing[0].Message, "the agent profile claude")
	assert.Equal(t, 1, strings.Count(stderr, "warning agent-path-missing: "+filepath.Join(s.home, ".claude/projects")), stderr)
	st, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Contains(t, st.Warnings, missing[0], "recorded for status and SessionStart")

	for _, p := range []string{".claude/projects", ".claude/todos"} {
		require.NoError(t, os.MkdirAll(filepath.Join(s.home, p), 0o700))
	}
	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, slugs(rep, setup.AgentPathMissing), ".claude/plans is optional")
}
