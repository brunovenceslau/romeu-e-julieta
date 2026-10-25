// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
)

// kitDir is kits/julieta-claude seen from this package.
const kitDir = "../../../../kits/julieta-claude"

// selfPath is the path the tests give the running julieta.
const selfPath = "/abs/.romeu/bin/julieta-linux-arm64"

// installKit lays out below home and managed what the kit's install
// steps leave for julieta at path, and returns the drop-in's path.
func installKit(t *testing.T, home, managed, path string) string {
	t.Helper()
	require.NoError(t, os.CopyFS(filepath.Join(home, claude.SkillsDir), os.DirFS(filepath.Join(kitDir, "skills"))))
	src, err := os.ReadFile(filepath.Join(kitDir, claude.SettingsFile))
	require.NoError(t, err)
	dropIn := claude.DropInAt(managed)
	require.NoError(t, os.MkdirAll(filepath.Dir(dropIn), 0o755))
	require.NoError(t, os.WriteFile(dropIn, bytes.ReplaceAll(src, []byte(claude.Placeholder), []byte(path)), 0o644))
	return dropIn
}

// TestSetupSkillsStale holds the step of 06 6.3 step 6 (REQ-0402,
// REQ-0587): with the kit installed for this julieta, no skills-stale
// and no agent-hooks-missing; a changed skill is skills-stale, which
// setup records for the next SessionStart; settings for another
// julieta, or none, are both skills-stale and agent-hooks-missing.
func TestSetupSkillsStale(t *testing.T) {
	s := claudeSandbox(t)
	managed := t.TempDir()
	defer setup.SetAgentManaged(managed)()
	defer setup.SetExecutable(func() (string, error) { return selfPath, nil })()
	dropIn := installKit(t, s.home, managed, selfPath)

	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, slugs(rep, setup.SkillsStale), "%v", rep.Warnings)
	assert.Empty(t, slugs(rep, setup.AgentHooksMissing), "%v", rep.Warnings)

	skill := filepath.Join(s.home, claude.SkillsDir, "handoff", "references", "clear.md")
	require.NoError(t, os.WriteFile(skill, []byte("an older skill\n"), 0o644))
	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, "skills-stale is a warning: %s", stderr)
	stale := slugs(rep, setup.SkillsStale)
	require.Len(t, stale, 1)
	assert.Contains(t, stale[0].Message, claude.KitDigest)
	assert.Contains(t, stale[0].Message, "a recreate installs them")
	assert.Contains(t, stderr, "warning skills-stale: ")
	assert.Empty(t, slugs(rep, setup.AgentHooksMissing), "the skill is there, only changed")
	st, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Contains(t, st.Warnings, stale[0], "recorded for the next SessionStart")

	require.NoError(t, os.RemoveAll(filepath.Join(s.home, claude.SkillsDir)))
	installKit(t, s.home, managed, "/other/julieta")
	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Len(t, slugs(rep, setup.SkillsStale), 1, "the hook lines of another julieta")
	hooks := slugs(rep, setup.AgentHooksMissing)
	require.Len(t, hooks, 1)
	assert.Contains(t, hooks[0].Message, "SessionStart startup, SessionStart resume, SessionStart clear, SessionStart compact, SessionEnd, PreToolUse Bash;")
	assert.Contains(t, stderr, "warning agent-hooks-missing: ")

	require.NoError(t, os.Remove(dropIn))
	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Len(t, slugs(rep, setup.SkillsStale), 1, "no settings")
	assert.Len(t, slugs(rep, setup.AgentHooksMissing), 1, "no settings")
}

// TestSetupSkillsStaleUnreadable holds the reads that fail: a skills
// dir that is a file is skills-stale with the error, and a settings
// file that is not JSON an entry of the report's errors, exit 0.
func TestSetupSkillsStaleUnreadable(t *testing.T) {
	s := claudeSandbox(t)
	managed := t.TempDir()
	defer setup.SetAgentManaged(managed)()
	defer setup.SetExecutable(func() (string, error) { return selfPath, nil })()
	installKit(t, s.home, managed, selfPath)
	require.NoError(t, os.RemoveAll(filepath.Join(s.home, claude.SkillsDir, "julieta")))
	require.NoError(t, os.WriteFile(filepath.Join(s.home, claude.SkillsDir, "julieta"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(managed, "managed-settings.json"), []byte("{"), 0o644))
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	stale := slugs(rep, setup.SkillsStale)
	require.Len(t, stale, 1)
	assert.Contains(t, stale[0].Message, "cannot be read")
	require.NotEmpty(t, rep.Errors)
	assert.True(t, strings.HasPrefix(rep.Errors[len(rep.Errors)-1], "agent-hooks-missing: "), rep.Errors)
}
