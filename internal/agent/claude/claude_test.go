// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
)

// The directories of the repository these tests read, seen from this
// package.
const (
	kitDir     = "../../../kits/julieta-claude"
	julietaKit = "../../../kits/julieta"
	skillsDir  = "../../../skills"
)

// TestProfileMatches holds the handles of the profile: the built-in
// agent of sbx and the workload kit the pin file names.
func TestProfileMatches(t *testing.T) {
	for _, h := range []string{"claude", "sbx-kit-claude"} {
		assert.True(t, claude.Matches(h), h)
	}
	for _, h := range []string{"", "codex", "sbx-kit-claude2", "Claude"} {
		assert.False(t, claude.Matches(h), h)
	}
}

// TestProfilePaths holds the profile to what 08 8.5 step 5 asks of it:
// the process names, relative state paths split into required and
// optional ones, and the memory dir the kit makes non-writable, an
// absolute path outside $HOME.
func TestProfilePaths(t *testing.T) {
	assert.Equal(t, []string{"claude"}, claude.Processes)
	var required, optional []string
	for _, p := range claude.Paths {
		assert.False(t, filepath.IsAbs(p.Rel), p.Rel)
		assert.Equal(t, filepath.Clean(p.Rel), p.Rel)
		assert.NotEmpty(t, p.What, p.Rel)
		if p.Required {
			required = append(required, p.Rel)
		} else {
			optional = append(optional, p.Rel)
		}
	}
	assert.Equal(t, []string{".claude/projects", ".claude/todos"}, required)
	assert.Equal(t, []string{".claude/plans"}, optional)
	assert.True(t, filepath.IsAbs(claude.MemoryDir))
	assert.False(t, strings.HasPrefix(claude.MemoryDir, "/home/"), "the agent cannot remove a dir outside $HOME")
}

// TestKitSkillsAreTheSkills holds the copy of the kit to skills/: the
// same files, byte for byte, so the kit installs the skills as they are
// (REQ-0595).
func TestKitSkillsAreTheSkills(t *testing.T) {
	want := tree(t, os.DirFS(skillsDir))
	got := tree(t, os.DirFS(filepath.Join(kitDir, "skills")))
	require.Contains(t, want, "julieta/SKILL.md")
	require.Contains(t, want, "handoff/SKILL.md")
	assert.Equal(t, want, got, "kits/julieta-claude/skills is a copy of skills/: copy it again")
}

// tree returns every file below the root of fsys and its content.
func tree(t *testing.T, fsys fs.FS) map[string]string {
	t.Helper()
	files := map[string]string{}
	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		files[p] = string(b)
		return err
	}))
	return files
}

// TestKitDigest holds the digest julieta embeds to the files the kit
// installs: the skills and the hook lines (REQ-0587). A change to
// either fails here until KitDigest is the new one.
func TestKitDigest(t *testing.T) {
	files, err := claude.KitFiles(os.DirFS(kitDir))
	require.NoError(t, err)
	assert.Contains(t, files, claude.SettingsFile)
	assert.Contains(t, files, "skills/julieta/references/memory.md")
	for p := range files {
		assert.True(t, p == claude.SettingsFile || strings.HasPrefix(p, "skills/"), p)
	}
	assert.Equal(t, claude.Digest(files), claude.KitDigest, "set KitDigest to the digest of the kit")
}

// TestDigestFault holds the digest to every byte and name of the files.
func TestDigestFault(t *testing.T) {
	base := map[string][]byte{"skills/a/SKILL.md": []byte("a"), claude.SettingsFile: []byte("{}")}
	d := claude.Digest(base)
	assert.True(t, strings.HasPrefix(d, "sha256:"), d)
	for name, files := range map[string]map[string][]byte{
		"a changed byte": {"skills/a/SKILL.md": []byte("b"), claude.SettingsFile: []byte("{}")},
		"a renamed file": {"skills/a/OTHER.md": []byte("a"), claude.SettingsFile: []byte("{}")},
		"a missing file": {"skills/a/SKILL.md": []byte("a")},
		"an extra file":  {"skills/a/SKILL.md": []byte("a"), "skills/a/x": nil, claude.SettingsFile: []byte("{}")},
	} {
		assert.NotEqual(t, d, claude.Digest(files), name)
	}
	// A name that holds the separators cannot make two sets alike.
	assert.NotEqual(t,
		claude.Digest(map[string][]byte{"a": []byte("x"), "b": []byte("y")}),
		claude.Digest(map[string][]byte{"a\x00x\nb": []byte("y")}))
}

// TestKitSettingsAreTheCommands holds the settings file of the kit to
// the commands julieta renders: the kit's install step replaces the
// placeholder with the path of julieta, which yields the commands
// julieta renders for that path, the ones julieta status looks for
// (REQ-0424).
func TestKitSettingsAreTheCommands(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(kitDir, claude.SettingsFile))
	require.NoError(t, err)
	wired, err := claude.Wired(src)
	require.NoError(t, err)
	want, err := claude.Commands(claude.Placeholder)
	require.NoError(t, err)
	assert.ElementsMatch(t, want, wired)

	s, err := claude.ReadSettings(src)
	require.NoError(t, err)
	require.NotNil(t, s.AutoMemoryEnabled)
	assert.False(t, *s.AutoMemoryEnabled, "the disabling setting (Q8)")
	assert.Equal(t, claude.MemoryDir, s.AutoMemoryDirectory)
}

// TestCommands holds the wiring of 08 8.2 and 06 6.4: SessionStart once
// for each source the pinned version fires, SessionEnd once, and the
// PreToolUse guard on Bash, each invoking julieta by its absolute path,
// quoted by internal/shquote.
func TestCommands(t *testing.T) {
	cmds, err := claude.Commands("/abs/.romeu/bin/julieta-linux-arm64")
	require.NoError(t, err)
	var starts []string
	count := map[string]int{}
	for _, c := range cmds {
		count[c.Event]++
		assert.True(t, strings.HasPrefix(c.Command, "/abs/.romeu/bin/julieta-linux-arm64 "), c.Command)
		switch c.Event {
		case claude.SessionStart:
			starts = append(starts, c.Matcher)
			assert.Contains(t, c.Command, " handoff show --hook ")
			assert.True(t, strings.HasSuffix(c.Command, "; exit 0"), c.Command)
		case claude.SessionEnd:
			assert.Empty(t, c.Matcher)
			assert.Contains(t, c.Command, " handoff write --facts ")
			assert.True(t, strings.HasSuffix(c.Command, "; exit 0"), c.Command)
		case claude.PreToolUse:
			assert.Equal(t, "Bash", c.Matcher)
			assert.Contains(t, c.Command, " hooks guard ")
			assert.Contains(t, c.Command, "exit 2")
		}
	}
	assert.Equal(t, []string{"startup", "resume", "clear", "compact"}, starts)
	assert.Equal(t, map[string]int{claude.SessionStart: 4, claude.SessionEnd: 1, claude.PreToolUse: 1}, count)

	// A path with a space is quoted, as internal/shquote quotes it.
	cmds, err = claude.Commands("/a b/julieta")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(cmds[0].Command, "'/a b/julieta' handoff show --hook"), cmds[0].Command)
	_, err = claude.Commands("/a\x00b")
	assert.Error(t, err)
}

// TestWiredRefusesWhatIsNotSettings holds the reader of a settings file
// to its shape: a document that is not a JSON object, a duplicate
// member and hooks of the wrong type are errors, and an unknown member
// is not.
func TestWiredRefusesWhatIsNotSettings(t *testing.T) {
	for name, src := range map[string]string{
		"not json":         "{",
		"not an object":    "[]",
		"a duplicate":      `{"hooks":{},"hooks":{}}`,
		"hooks not a map":  `{"hooks":[]}`,
		"a hook not a map": `{"hooks":{"SessionEnd":[1]}}`,
	} {
		_, err := claude.Wired([]byte(src))
		assert.Error(t, err, name)
	}
	w, err := claude.Wired([]byte(`{"model":"x","hooks":{"Stop":[{"hooks":[{"type":"command","command":"c","async":true}]}]}}`))
	require.NoError(t, err)
	assert.Equal(t, []claude.Command{{Event: "Stop", Command: "c"}}, w)
	w, err = claude.Wired([]byte(`{}`))
	require.NoError(t, err)
	assert.Empty(t, w)
}

// TestJulietaKitIsAgentNeutral holds the julieta kit to REQ-0607: no
// value of its descriptor, and no other file of it, names Claude, so
// julieta-claude stays the one agent-aware kit. Comments are left out:
// they may name the workload they were measured on.
func TestJulietaKitIsAgentNeutral(t *testing.T) {
	assert.Empty(t, agentSpecific(os.DirFS(julietaKit)))

	// The fault: a descriptor value, or a file, that names the agent's
	// paths is found.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "k.yaml"), []byte("# claude in a comment\nx:\n  - 'cp a $HOME/.claude/settings.json'\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes"), []byte("CLAUDE_CODE_X=1\n"), 0o600))
	got := agentSpecific(os.DirFS(dir))
	assert.Len(t, got, 2, "%v", got)
	assert.Contains(t, strings.Join(got, "\n"), "k.yaml")
	assert.Contains(t, strings.Join(got, "\n"), "notes")

	// A descriptor that does not parse is a finding too, not a pass.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "k.yaml"), []byte("x: [\n"), 0o600))
	assert.Contains(t, strings.Join(agentSpecific(os.DirFS(dir)), "\n"), "k.yaml")
}
