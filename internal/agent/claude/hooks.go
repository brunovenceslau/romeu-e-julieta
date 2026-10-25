// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/shquote"
)

// The hook events the kit wires.
const (
	SessionStart = "SessionStart"
	SessionEnd   = "SessionEnd"
	PreToolUse   = "PreToolUse"
)

// StartSources are the sources the pinned version fires SessionStart
// for (08 8.2, REQ-0663); the kit wires one entry for each.
var StartSources = []string{"startup", "resume", "clear", "compact"}

// The files of the kit and where it installs them.
const (
	// Placeholder stands for the path of julieta in the kit's
	// settings file; the kit's install step replaces it with the arg
	// julietaBin.
	Placeholder = "@JULIETA_BIN@"
	// SettingsFile is the kit's settings file, relative to the kit's
	// directory.
	SettingsFile = "managed-settings.json"
	// ManagedDir is the directory of the agent's managed settings.
	ManagedDir = "/etc/claude-code"
	// dropInName is the name of the drop-in the kit installs its
	// settings file as, in managed-settings.d.
	dropInName = "50-julieta-claude.json"
	// SkillsDir is the agent's skills directory, relative to $HOME.
	SkillsDir = ".claude/skills"
)

// DropInAt returns the drop-in the kit installs its settings file as,
// below the managed settings directory managed (ManagedDir).
func DropInAt(managed string) string {
	return filepath.Join(managed, "managed-settings.d", dropInName)
}

// Skills are the skills the kit installs, each a directory of skills/
// (06 6.4, REQ-0595).
var Skills = []string{"julieta", "handoff"}

// Command is one hook command of the agent's settings: its event, the
// matcher of its entry (empty for none) and the shell command.
type Command struct {
	Event, Matcher, Command string
}

// Commands returns the hook commands the kit wires for julieta at the
// absolute path julieta, quoted by internal/shquote (REQ-0596): each is
// a shell wrapper that, when julieta cannot run (the .romeu/bin mount
// absent), prints one line naming the failure and julieta status and
// exits 0 for a session hook, and exits 2 for the PreToolUse guard,
// which fails closed (REQ-0673). A status of 126 or more is the shell's
// for a program it could not run.
func Commands(julieta string) ([]Command, error) {
	q, err := shquote.Quote(julieta)
	if err != nil {
		return nil, err
	}
	session := func(args, what string) (string, error) {
		line, err := shquote.Quote(what + " failed: " + julieta + " could not run, the .romeu/bin mount may be absent; run julieta status")
		return q + " " + args + " || [ $? -lt 126 ] || echo " + line + "; exit 0", err
	}
	start, err := session("handoff show --hook", "julieta handoff show --hook")
	if err != nil {
		return nil, err
	}
	end, err := session("handoff write --facts", "julieta handoff write --facts")
	if err != nil {
		return nil, err
	}
	line, err := shquote.Quote("julieta hooks guard failed: " + julieta + " could not run, so the tool call is refused (the guard fails closed); the .romeu/bin mount may be absent; run julieta status")
	if err != nil {
		return nil, err
	}
	var cmds []Command
	for _, s := range StartSources {
		cmds = append(cmds, Command{Event: SessionStart, Matcher: s, Command: start})
	}
	return append(cmds,
		Command{Event: SessionEnd, Command: end},
		Command{Event: PreToolUse, Matcher: "Bash", Command: q + " hooks guard || { [ $? -eq 2 ] || echo " + line + " >&2; exit 2; }"},
	), nil
}

// Settings is the part of a settings file of the agent julieta reads.
// A member it does not name is skipped.
type Settings struct {
	AutoMemoryEnabled   *bool                  `json:"autoMemoryEnabled"`
	AutoMemoryDirectory string                 `json:"autoMemoryDirectory"`
	Hooks               map[string][]hookEntry `json:"hooks"`
}

// hookEntry is one entry of an event of the hooks of a settings file.
type hookEntry struct {
	Matcher string `json:"matcher"`
	Hooks   []struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	} `json:"hooks"`
}

// ReadSettings decodes a settings file of the agent: a JSON object, no
// duplicate member.
func ReadSettings(data []byte) (Settings, error) {
	var s Settings
	err := json.Unmarshal(data, &s)
	return s, err
}

// Wired returns the command hooks a settings file wires, in its order:
// events by name, then entries and hooks as they come.
func Wired(data []byte) ([]Command, error) {
	s, err := ReadSettings(data)
	if err != nil {
		return nil, err
	}
	var events []string
	for e := range s.Hooks {
		events = append(events, e)
	}
	slices.Sort(events)
	var out []Command
	for _, e := range events {
		for _, entry := range s.Hooks[e] {
			for _, h := range entry.Hooks {
				if h.Type == "command" {
					out = append(out, Command{Event: e, Matcher: entry.Matcher, Command: h.Command})
				}
			}
		}
	}
	return out, nil
}

// SettingsFiles returns the settings files of the agent that exist, in
// the order the pinned version reads them: managed-settings.json and
// the .json drop-ins of managed-settings.d in name order, below
// managed, then the user's ~/.claude/settings.json below home. A name
// that starts with a dot is left out.
func SettingsFiles(managed, home string) ([]string, error) {
	var files []string
	exists := func(p string) error {
		_, err := os.Stat(p)
		switch {
		case err == nil:
			files = append(files, p)
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
		return nil
	}
	if err := exists(filepath.Join(managed, "managed-settings.json")); err != nil {
		return nil, err
	}
	dir := filepath.Join(managed, "managed-settings.d")
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && !strings.HasPrefix(e.Name(), ".") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if err := exists(filepath.Join(home, ".claude", "settings.json")); err != nil {
		return nil, err
	}
	return files, nil
}

// Missing returns what agent-hooks-missing names (04 4.3, REQ-0424):
// each hook of Commands(julieta) that no file of files wires, as
// "<event> <matcher>", and each skill of Skills whose SKILL.md is not
// below home, as "skill <name>". A settings file that cannot be read or
// decoded is an error naming it.
func Missing(home string, files []string, julieta string) ([]string, error) {
	want, err := Commands(julieta)
	if err != nil {
		return nil, err
	}
	var wired []Command
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		w, err := Wired(b)
		if err != nil {
			return nil, fmt.Errorf("the agent's settings %s: %w", f, err)
		}
		wired = append(wired, w...)
	}
	var missing []string
	for _, c := range want {
		if !slices.Contains(wired, c) {
			missing = append(missing, strings.TrimSpace(c.Event+" "+c.Matcher))
		}
	}
	for _, s := range Skills {
		info, err := os.Stat(filepath.Join(home, SkillsDir, s, "SKILL.md"))
		if err != nil || !info.Mode().IsRegular() {
			missing = append(missing, "skill "+s)
		}
	}
	return missing, nil
}
