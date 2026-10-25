// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
)

// The path of julieta the cases install the kit for.
const julieta = "/abs/.romeu/bin/julieta-linux-arm64"

// install lays out below home and managed what the kit's steps leave,
// for julieta at path: the skills of the kit in ~/.claude/skills and
// its settings file, the placeholder replaced, in the managed-settings
// drop-in dir. It returns the drop-in's path.
func install(t *testing.T, home, managed, path string) string {
	t.Helper()
	src := filepath.Join(kitDir, "skills")
	require.NoError(t, os.CopyFS(filepath.Join(home, claude.SkillsDir), os.DirFS(src)))
	settings, err := os.ReadFile(filepath.Join(kitDir, claude.SettingsFile))
	require.NoError(t, err)
	dropIn := filepath.Join(managed, "managed-settings.d", "50-julieta-claude.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(dropIn), 0o755))
	require.NoError(t, os.WriteFile(dropIn, bytes.ReplaceAll(settings, []byte(claude.Placeholder), []byte(path)), 0o644))
	return dropIn
}

// TestInstalledDigest holds the read of setup (skills-stale, REQ-0402):
// what the kit installed for this julieta has the digest julieta
// embeds; a changed skill, a missing skill, settings for another
// julieta and no settings each have another.
func TestInstalledDigest(t *testing.T) {
	home, managed := t.TempDir(), t.TempDir()
	dropIn := install(t, home, managed, julieta)
	files, err := claude.Installed(home, dropIn, julieta)
	require.NoError(t, err)
	assert.Equal(t, claude.KitDigest, claude.Digest(files))

	other, err := claude.Installed(home, dropIn, "/other/julieta")
	require.NoError(t, err)
	assert.NotEqual(t, claude.KitDigest, claude.Digest(other), "the hooks of another julieta")

	none, err := claude.Installed(home, filepath.Join(managed, "absent.json"), julieta)
	require.NoError(t, err)
	assert.NotEqual(t, claude.KitDigest, claude.Digest(none), "no settings")

	skill := filepath.Join(home, claude.SkillsDir, "julieta", "SKILL.md")
	require.NoError(t, os.WriteFile(skill, []byte("changed\n"), 0o644))
	changed, err := claude.Installed(home, dropIn, julieta)
	require.NoError(t, err)
	assert.NotEqual(t, claude.KitDigest, claude.Digest(changed), "a changed skill")

	require.NoError(t, os.RemoveAll(filepath.Join(home, claude.SkillsDir, "handoff")))
	gone, err := claude.Installed(home, dropIn, julieta)
	require.NoError(t, err)
	assert.NotContains(t, strings.Join(keys(gone), " "), "skills/handoff/")
	assert.NotEqual(t, claude.KitDigest, claude.Digest(gone), "a missing skill")

	// A skills dir that is a file cannot be read: an error, not a pass.
	require.NoError(t, os.RemoveAll(filepath.Join(home, claude.SkillsDir, "julieta")))
	require.NoError(t, os.WriteFile(filepath.Join(home, claude.SkillsDir, "julieta"), nil, 0o644))
	_, err = claude.Installed(home, dropIn, julieta)
	assert.Error(t, err)
}

// keys returns the names of files.
func keys(files map[string][]byte) []string {
	var out []string
	for k := range files {
		out = append(out, k)
	}
	return out
}

// TestMissing holds the read of julieta status (agent-hooks-missing,
// REQ-0424): with the kit installed nothing is missing; each hook the
// settings lack, by event and source, and each skill not installed is
// named; the hooks may come from any settings file of the agent.
func TestMissing(t *testing.T) {
	home, managed := t.TempDir(), t.TempDir()
	install(t, home, managed, julieta)
	files, err := claude.SettingsFiles(managed, home)
	require.NoError(t, err)
	missing, err := claude.Missing(home, files, julieta)
	require.NoError(t, err)
	assert.Empty(t, missing)

	// Another julieta: every hook is missing.
	missing, err = claude.Missing(home, files, "/other/julieta")
	require.NoError(t, err)
	assert.Equal(t, []string{"SessionStart startup", "SessionStart resume", "SessionStart clear", "SessionStart compact", "SessionEnd", "PreToolUse Bash"}, missing)

	// The hooks moved to the user's settings still count, and a skill
	// that is gone is named.
	dropIn := filepath.Join(managed, "managed-settings.d", "50-julieta-claude.json")
	b, err := os.ReadFile(dropIn)
	require.NoError(t, err)
	require.NoError(t, os.Remove(dropIn))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), b, 0o600))
	require.NoError(t, os.RemoveAll(filepath.Join(home, claude.SkillsDir, "handoff")))
	files, err = claude.SettingsFiles(managed, home)
	require.NoError(t, err)
	missing, err = claude.Missing(home, files, julieta)
	require.NoError(t, err)
	assert.Equal(t, []string{"skill handoff"}, missing)

	// A settings file that is not JSON is an error naming it.
	bad := filepath.Join(managed, "managed-settings.json")
	require.NoError(t, os.WriteFile(bad, []byte("{"), 0o644))
	files, err = claude.SettingsFiles(managed, home)
	require.NoError(t, err)
	assert.Equal(t, bad, files[0], "managed-settings.json first, as the agent reads it")
	_, err = claude.Missing(home, files, julieta)
	require.ErrorContains(t, err, bad)
}

// TestSettingsFiles holds the list to the sources the pinned agent
// reads: managed-settings.json, the drop-ins in name order (only the
// .json files), then the user's settings; an absent file is left out.
func TestSettingsFiles(t *testing.T) {
	home, managed := t.TempDir(), t.TempDir()
	files, err := claude.SettingsFiles(managed, home)
	require.NoError(t, err)
	assert.Empty(t, files)
	d := filepath.Join(managed, "managed-settings.d")
	require.NoError(t, os.MkdirAll(d, 0o755))
	for _, n := range []string{"20-b.json", "10-a.json", "notes.txt", ".hidden.json"} {
		require.NoError(t, os.WriteFile(filepath.Join(d, n), []byte("{}"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(managed, "managed-settings.json"), []byte("{}"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{}"), 0o644))
	files, err = claude.SettingsFiles(managed, home)
	require.NoError(t, err)
	assert.Equal(t, []string{
		filepath.Join(managed, "managed-settings.json"),
		filepath.Join(d, "10-a.json"),
		filepath.Join(d, "20-b.json"),
		filepath.Join(home, ".claude", "settings.json"),
	}, files)
}

// TestDecodePreToolUse reads the document the pinned agent sends a
// PreToolUse hook, recorded from Claude Code 2.1.296 (the fixture of
// the container e2e), and refuses one it cannot decode.
func TestDecodePreToolUse(t *testing.T) {
	recorded, err := os.ReadFile("../../../e2e/testdata/agent/2.1.296/pre-tool-use-bash.json")
	require.NoError(t, err)
	p, err := claude.DecodePreToolUse(recorded)
	require.NoError(t, err)
	assert.Equal(t, claude.PreToolUseDoc{Tool: "Bash", Command: "gh pr merge 1"}, p)

	p, err = claude.DecodePreToolUse([]byte(`{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"/x"}}`))
	require.NoError(t, err)
	assert.Equal(t, claude.PreToolUseDoc{Tool: "Read"}, p, "another tool carries no command")

	for name, src := range map[string]string{
		"empty":                    "",
		"not json":                 "{",
		"not an object":            `"x"`,
		"no tool":                  `{"tool_input":{"command":"ls"}}`,
		"a tool not a string":      `{"tool_name":1,"tool_input":{}}`,
		"Bash with no command":     `{"tool_name":"Bash","tool_input":{}}`,
		"Bash with no input":       `{"tool_name":"Bash"}`,
		"a command not a string":   `{"tool_name":"Bash","tool_input":{"command":["gh"]}}`,
		"a duplicate command":      `{"tool_name":"Bash","tool_input":{"command":"ls","command":"gh pr merge"}}`,
		"a duplicate tool":         `{"tool_name":"Read","tool_name":"Bash","tool_input":{"command":"gh pr merge"}}`,
		"another event":            `{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`,
		"trailing data":            `{"tool_name":"Bash","tool_input":{"command":"ls"}} {}`,
		"invalid UTF-8 in a value": "{\"tool_name\":\"Bash\",\"tool_input\":{\"command\":\"\xff\"}}",
	} {
		_, err := claude.DecodePreToolUse([]byte(src))
		require.Error(t, err, name)
	}
	_, err = claude.DecodePreToolUse(bytes.Repeat([]byte(" "), claude.MaxPreToolUse+1))
	assert.ErrorContains(t, err, "bytes")
}
