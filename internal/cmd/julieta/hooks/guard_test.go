// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package hooks_test

import (
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/hooks"
)

// recorded is the PreToolUse document Claude Code 2.1.296 sent a hook,
// the fixture of the container e2e; its command is gh pr merge 1.
func recorded(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../../../e2e/testdata/agent/2.1.296/pre-tool-use-bash.json")
	require.NoError(t, err)
	return string(b)
}

// withCommand returns the recorded document with its command replaced.
func withCommand(t *testing.T, command string) string {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(recorded(t)), &doc))
	doc["tool_input"].(map[string]any)["command"] = command
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	return string(b)
}

// guard runs julieta hooks guard with stdin and the args, and returns
// the exits it asked for, stdout and stderr.
func guard(t *testing.T, stdin string, args ...string) ([]int, string, string) {
	t.Helper()
	var exits []int
	restore := hooks.Process(stdin, nil, func(code int) { exits = append(exits, code) })
	defer restore()
	code, stdout, stderr := julieta(t, append([]string{"hooks", "guard"}, args...)...)
	require.Equal(t, 0, code, stderr)
	return exits, stdout, stderr
}

// TestGuardRefuses holds julieta hooks guard to 04 4.3 (REQ-0410): the
// recorded document of each pattern exits 2 with the refusal text of
// 06 6.4 on stderr, which says the action is the maintainer's and names
// the route (REQ-0598).
func TestGuardRefuses(t *testing.T) {
	for name, command := range map[string]string{
		"gh pr merge":        "gh pr merge 1",
		"a v* tag push":      "git push origin v1.2.0",
		"gh api to rulesets": "gh api -X POST repos/o/r/rulesets --input r.json",
		"gh api to settings": "gh api -X PATCH repos/o/r -f allow_auto_merge=true",
	} {
		exits, stdout, stderr := guard(t, withCommand(t, command))
		assert.Equal(t, []int{2}, exits, name)
		assert.Empty(t, stdout, name)
		assert.Contains(t, stderr, "the maintainer's action", name)
		assert.Contains(t, stderr, "in an issue, or write it into the handoff", name)
		assert.Equal(t, 1, strings.Count(stderr, "\n"), "%s: one line: %q", name, stderr)
	}
	exits, _, _ := guard(t, recorded(t))
	assert.Equal(t, []int{2}, exits, "the recorded document as it is")
}

// TestGuardAllows holds the other exit of 04 4.3: a command no pattern
// hits, and a tool other than Bash, exit 0 with nothing printed.
func TestGuardAllows(t *testing.T) {
	for _, doc := range []string{
		withCommand(t, "gh pr view 1"),
		`{"hook_event_name":"PreToolUse","tool_name":"Edit","tool_input":{"file_path":"/x","old_string":"gh pr merge"}}`,
	} {
		exits, stdout, stderr := guard(t, doc)
		assert.Empty(t, exits, doc)
		assert.Empty(t, stdout+stderr, doc)
	}
}

// TestGuardFailsClosed holds the guard to fail closed (REQ-0410): a
// document it cannot decode, an empty stdin and one over the cap exit 2
// with a line that says why.
func TestGuardFailsClosed(t *testing.T) {
	for name, doc := range map[string]string{
		"not json":   "{",
		"empty":      "",
		"no command": `{"tool_name":"Bash","tool_input":{}}`,
		"over cap":   strings.Repeat(" ", 1<<20+1),
	} {
		exits, _, stderr := guard(t, doc)
		assert.Equal(t, []int{2}, exits, name)
		assert.Contains(t, stderr, "cannot be decoded", name)
		assert.Contains(t, stderr, "fails closed", name)
	}
}

// guardJSON is the --json document of julieta hooks guard, read
// strictly.
type guardJSON struct {
	Decision string `json:"decision"`
	Message  string `json:"message"`
	Rule     string `json:"rule"`
	Schema   string `json:"schema"`
}

// TestGuardJSON holds the --json form: the verdict on stdout, the same
// exits and the refusal on stderr, where the agent reads it.
func TestGuardJSON(t *testing.T) {
	exits, stdout, stderr := guard(t, withCommand(t, "gh pr merge 1"), "--json")
	assert.Equal(t, []int{2}, exits)
	var g guardJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &g, json.RejectUnknownMembers(true)), stdout)
	assert.Equal(t, guardJSON{Decision: "refuse", Rule: "gh-pr-merge", Schema: "julieta-hooks-guard/v1", Message: strings.TrimSuffix(stderr, "\n")}, g)

	exits, stdout, _ = guard(t, withCommand(t, "ls"), "--json")
	assert.Empty(t, exits)
	g = guardJSON{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &g, json.RejectUnknownMembers(true)), stdout)
	assert.Equal(t, guardJSON{Decision: "allow", Schema: "julieta-hooks-guard/v1"}, g)

	exits, stdout, _ = guard(t, "{", "--json")
	assert.Equal(t, []int{2}, exits)
	g = guardJSON{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &g, json.RejectUnknownMembers(true)), stdout)
	assert.Equal(t, "undecodable", g.Rule)
}
