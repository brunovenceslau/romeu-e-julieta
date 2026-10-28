// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeTheM11Types reads each type the kit schema of v3.0.0-m.11
// adds, and port@1's family, into the set the gate shows (I28): an
// interactive verb given as an empty list stays apart from one left out,
// so the gate sees the difference; agent-context@2 is a Context of
// version 2.
func TestNormalizeTheM11Types(t *testing.T) {
	tests := []struct {
		name   string
		typ    string
		config string
		check  func(t *testing.T, c Capabilities)
	}{
		{"interactive sessions", typeInteractive, `{prompt: ["--", "{{.Prompt}}"], continue: [], list: [node, ls.mjs]}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.NotNil(t, c.InteractiveSessions)
			assert.Equal(t, []string{"--", "{{.Prompt}}"}, c.InteractiveSessions.Prompt)
			assert.Equal(t, &[]string{}, c.InteractiveSessions.Continue, "an empty list is the launch argv alone")
			assert.Nil(t, c.InteractiveSessions.NewSession, "a verb left out stays nil: the schema falls back to the lifecycle launch")
			assert.Equal(t, &Command{Argv: []string{"node", "ls.mjs"}}, c.InteractiveSessions.List)
		}},
		{"an empty continue alone", typeInteractive, `{continue: []}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.NotNil(t, c.InteractiveSessions)
			assert.Equal(t, &[]string{}, c.InteractiveSessions.Continue)
		}},
		{"acp", typeACP, `{agent: claude, command: [claude-acp], env: {MODE: x}, protocolVersion: 2}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []ACP{{Agent: "claude", Command: Command{Argv: []string{"claude-acp"}}, Env: map[string]string{"MODE": "x"}, ProtocolVersion: "2"}}, c.ACP)
		}},
		{"agent-context@2", typeContext2, `{content: hello, filename: CLAUDE.md, directory: /home/agent/.claude}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Context{{Filename: "CLAUDE.md", Content: "hello", Directory: "/home/agent/.claude", Version: 2}}, c.Context)
		}},
		{"port family", typePort, `{container: 8080, family: dual}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Port{{Container: "8080", Family: "dual"}}, c.Ports)
		}},
		{"a prompt that is a kit arg", typeInteractive, `{prompt: ["${{ kit.args.p }}"]}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.NotNil(t, c.InteractiveSessions)
			assert.Equal(t, []string{"${{ kit.args.p }}"}, c.InteractiveSessions.Prompt)
		}},
		{"acp with kit args", typeACP, `{agent: "${{ kit.args.a }}", command: x, protocolVersion: "${{ kit.args.v }}"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.Len(t, c.ACP, 1)
			assert.Equal(t, "${{ kit.args.a }}", c.ACP[0].Agent)
			assert.Equal(t, "${{ kit.args.v }}", c.ACP[0].ProtocolVersion)
		}},
		{"acp with a provider", typeACP, `{agent: deb/claude-code, command: x}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.Len(t, c.ACP, 1)
			assert.Equal(t, "deb/claude-code", c.ACP[0].Agent)
		}},
		{"context@2 with kit args", typeContext2, `{filename: "${{ kit.args.f }}", directory: "${{ kit.args.d }}"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Context{{Filename: "${{ kit.args.f }}", Directory: "${{ kit.args.d }}", Version: 2}}, c.Context)
		}},
		{"a port family that is a kit arg", typePort, `{container: 8080, family: "${{ kit.args.f }}"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Port{{Container: "8080", Family: "${{ kit.args.f }}"}}, c.Ports)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, err := ParseDescriptor(capability(tt.typ, tt.config))
			require.NoError(t, err)
			tt.check(t, k.Capabilities)
		})
	}

	empty, err := ParseDescriptor(capability(typeInteractive, `{prompt: ["--", "{{.Prompt}}"], newSession: []}`))
	require.NoError(t, err)
	absent, err := ParseDescriptor(capability(typeInteractive, `{prompt: ["--", "{{.Prompt}}"]}`))
	require.NoError(t, err)
	assert.NotEqual(t, absent.Capabilities, empty.Capabilities, "newSession: [] and no newSession are two sets")

	ab, err := ParseDescriptor(withCaps("  - type: " + typeACP + "\n    config: {agent: a, command: x}\n  - type: " + typeACP + "\n    config: {agent: b, command: y}\n"))
	require.NoError(t, err)
	ba, err := ParseDescriptor(withCaps("  - type: " + typeACP + "\n    config: {agent: b, command: y}\n  - type: " + typeACP + "\n    config: {agent: a, command: x}\n"))
	require.NoError(t, err)
	assert.Equal(t, ab.Capabilities, ba.Capabilities, "the order of acp entries is not part of the set")
}

// TestRefuseTheM11Types refuses what the kit schema of v3.0.0-m.11 does
// not admit in the types it adds, a NUL anywhere in a descriptor (its
// root rule noNul), and declarationBlocks, a key this grammar does not
// model and so refuses on purpose (REQ-0573) until a kit needs it.
func TestRefuseTheM11Types(t *testing.T) {
	for _, tc := range []struct{ name, doc, want string }{
		{"no verb", string(capability(typeInteractive, `{}`)), "declares no verb"},
		{"a prompt without its placeholder", string(capability(typeInteractive, `{prompt: ["--"]}`)), "holds no {{.Prompt}}"},
		{"a resume without its placeholder", string(capability(typeInteractive, `{resume: ["--resume"]}`)), "holds no {{.SessionID}}"},
		{"a blank list", string(capability(typeInteractive, `{continue: [], list: "  "}`)), "list runs nothing"},
		{"an unknown verb", string(capability(typeInteractive, `{fork: []}`)), "unknown"},
		{"a second interactive sessions", string(withCaps("  - type: " + typeInteractive + "\n    config: {continue: []}\n  - type: " + typeInteractive + "\n    config: {continue: []}\n")), "singleton"},
		{"acp without a command", string(capability(typeACP, `{agent: claude}`)), "command"},
		{"acp with a blank command", string(capability(typeACP, `{agent: claude, command: ["", x]}`)), "command runs nothing"},
		{"acp with a bad env name", string(capability(typeACP, `{agent: claude, command: x, env: {1bad: y}}`)), "env name"},
		{"acp with protocol version 0", string(capability(typeACP, `{agent: claude, command: x, protocolVersion: 0}`)), "above zero"},
		{"acp with a text protocol version", string(capability(typeACP, `{agent: claude, command: x, protocolVersion: abc}`)), "one whole kit arg"},
		{"acp with a quoted number", string(capability(typeACP, `{agent: claude, command: x, protocolVersion: "2"}`)), "one whole kit arg"},
		{"acp with a kit arg inside text", string(capability(typeACP, `{agent: claude, command: x, protocolVersion: "1${{ kit.args.v }}"}`)), "one whole kit arg"},
		{"acp with an agent that is not a provides name", string(capability(typeACP, `{agent: "Bad Name/", command: x}`)), "provides name"},
		{"acp with an uppercase agent", string(capability(typeACP, `{agent: Claude, command: x}`)), "provides name"},
		{"acp twice for one agent", string(withCaps("  - type: " + typeACP + "\n    config: {agent: a, command: x}\n  - type: " + typeACP + "\n    config: {agent: a, command: y}\n")), "declared twice"},
		{"acp twice for one agent, bare and under com.docker.kit", string(withCaps("  - type: " + typeACP + "\n    config: {agent: a, command: x}\n  - type: " + typeACP + "\n    config: {agent: com.docker.kit/a, command: y}\n")), "declared twice"},
		{"context@2 with nothing", string(capability(typeContext2, `{directory: /a}`)), "neither a filename nor content"},
		{"context@2 with an empty directory", string(capability(typeContext2, `{filename: f, directory: ""}`)), "directory is empty"},
		{"context@2 with a backslash in a directory that is a kit arg", string(capability(typeContext2, `{filename: f, directory: "${{ kit.args.d }}\\x"}`)), "absolute path"},
		{"context@2 with a dot filename", string(capability(typeContext2, `{filename: ., directory: /a}`)), "one name"},
		{"acp with a blank shell command", string(capability(typeACP, `{agent: claude, command: "  "}`)), "command runs nothing"},
		{"a bare ${{ is not a kit arg", string(capability(typeInteractive, `{prompt: ["${{"]}`)), "holds no {{.Prompt}}"},
		{"context@2 with a relative directory", string(capability(typeContext2, `{filename: f, directory: a/b}`)), "absolute path"},
		{"context@2 with a dot-dot directory", string(capability(typeContext2, `{filename: f, directory: /a/..}`)), "absolute path"},
		{"context@2 with a path for a filename", string(capability(typeContext2, `{filename: a/b, directory: /a}`)), "one name"},
		{"a port family that is not one", string(capability(typePort, `{container: 1, family: bogus}`)), "family"},
		{"a NUL in a value", minimal + "capabilities:\n  - type: " + typeVolume + "\n    config: {path: \"/a\\0b\"}\n", "NUL"},
		{"a NUL in displayName", "schemaVersion: \"3\"\nkind: mixin\ndisplayName: \"a\\0b\"\n", "NUL"},
		{"a NUL in a key", "schemaVersion: \"3\"\nkind: mixin\n\"a\\0b\": 1\n", "NUL"},
		{"a NUL in a list", minimal + "capabilities:\n  - type: " + typeInteractive + "\n    config: {continue: [\"a\\0b\"]}\n", "NUL"},
		{"declarationBlocks", minimal + "capabilities:\n  - type: " + typePrivileged + "\n    declarationBlocks: [b1]\n", "declarationBlocks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDescriptor([]byte(tc.doc))
			require.ErrorContains(t, err, tc.want)
		})
	}
	for name, doc := range map[string]string{
		"a NUL in a value": `{"schemaVersion":"3","kind":"mixin","displayName":"a\u0000b"}`,
		"a NUL in a key":   `{"schemaVersion":"3","kind":"mixin","a\u0000b":1}`,
	} {
		t.Run("the JSON road refuses "+name, func(t *testing.T) {
			_, err := ParseAnnotation([]byte(doc))
			require.ErrorContains(t, err, "NUL")
		})
	}
}
