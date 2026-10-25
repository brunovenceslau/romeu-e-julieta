// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package status_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/status"
)

// The path the tests give the running julieta, and kits/julieta-claude
// seen from this package.
const (
	selfPath = "/abs/.romeu/bin/julieta-linux-arm64"
	kitDir   = "../../../../kits/julieta-claude"
)

// agentWarnings returns the agent-hooks-missing warnings of rep.
func agentWarnings(rep status.Report) []setup.Warning {
	var out []setup.Warning
	for _, w := range rep.Warnings {
		if w.Slug == setup.AgentHooksMissing {
			out = append(out, w)
		}
	}
	return out
}

// TestStatusAgentHooks holds the read of 04 4.3 (REQ-0424): with the
// kit installed for this julieta, no agent-hooks-missing; with a hook
// removed from the settings, as a personal kit might (REQ-0608), the
// warning names it, in --json and on stderr in text, in place of a
// stale one setup recorded; a settings file that cannot be decoded is
// an error of the report.
func TestStatusAgentHooks(t *testing.T) {
	s := newSandbox(t)
	s.m.Agent = "sbx-kit-claude"
	s.setManifest(t)
	managed := t.TempDir()
	defer status.SetAgent(managed, selfPath)()
	require.NoError(t, os.CopyFS(filepath.Join(s.home, claude.SkillsDir), os.DirFS(filepath.Join(kitDir, "skills"))))
	src, err := os.ReadFile(filepath.Join(kitDir, claude.SettingsFile))
	require.NoError(t, err)
	dropIn := claude.DropInAt(managed)
	require.NoError(t, os.MkdirAll(filepath.Dir(dropIn), 0o755))
	settings := bytes.ReplaceAll(src, []byte(claude.Placeholder), []byte(selfPath))
	require.NoError(t, os.WriteFile(dropIn, settings, 0o644))

	// A stale record of setup is not shown once the hooks are there.
	require.NoError(t, setup.UpdateState(s.home, func(st *setup.State) error {
		st.Warnings = append(st.Warnings, setup.Warning{Slug: setup.AgentHooksMissing, Message: "stale"})
		return nil
	}))
	assert.Empty(t, agentWarnings(statusJSON(t)))

	// SessionEnd removed from the settings.
	var cut = bytes.Replace(settings, []byte(`"SessionEnd"`), []byte(`"SessionEndOff"`), 1)
	require.NoError(t, os.WriteFile(dropIn, cut, 0o644))
	got := agentWarnings(statusJSON(t))
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "lack julieta's SessionEnd;")
	code, stdout, stderr := julieta(t.Context(), "status")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "warning agent-hooks-missing: the agent's settings and skills lack julieta's SessionEnd;")
	assert.NotContains(t, stdout, "agent-hooks-missing")

	require.NoError(t, os.WriteFile(dropIn, []byte("{"), 0o644))
	code, stdout, stderr = julieta(t.Context(), "status", "--json")
	require.Equal(t, 0, code, stderr)
	rep, err := status.Decode([]byte(stdout))
	require.NoError(t, err)
	require.NotEmpty(t, rep.Errors)
	assert.Contains(t, rep.Errors[len(rep.Errors)-1], "agent hooks: the agent's settings "+dropIn)
}
