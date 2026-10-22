// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimal is the smallest descriptor the grammar reads.
const minimal = "schemaVersion: \"3\"\nkind: mixin\n"

// withCaps returns a mixin whose capabilities are the YAML text caps,
// already indented as a block under "capabilities:".
func withCaps(caps string) []byte {
	return []byte(minimal + "capabilities:\n" + caps)
}

func TestParseDescriptorSittingFixture(t *testing.T) {
	// The sitting M0/M2 descriptor of the git-ssh-sign product kit is the
	// conformance fixture of REQ-0516.
	data, err := os.ReadFile("../../kits/git-ssh-sign/git-ssh-sign.yaml")
	require.NoError(t, err)

	kit, err := ParseDescriptor(data)
	require.NoError(t, err)

	assert.Equal(t, "mixin", kit.Capabilities.Kind)
	assert.Equal(t, "git-ssh-sign", kit.DisplayName)
	require.Len(t, kit.Capabilities.SSHAgent, 1)
	agent := kit.Capabilities.SSHAgent[0]
	assert.Equal(t, "false", agent.Unrestricted)
	assert.Equal(t, []string{"git"}, agent.Sign)
	require.NotNil(t, kit.Capabilities.Lifecycle)
	hooks := kit.Capabilities.Lifecycle.Install
	require.Len(t, hooks, 2)
	assert.Equal(t, "agent", hooks[0].User)
	assert.Equal(t, []string{"GIT_SSH_SIGN_KEY"}, hooks[1].Env)
	assert.Equal(t, []string{"bash", "-euc"}, hooks[0].Command.Argv[:2])
}

func TestParseDescriptorOrderOfCapabilitiesIsNotMeaning(t *testing.T) {
	a := withCaps("  - type: com.docker.sandbox/port@1\n    config: {container: 80}\n  - type: com.docker.sandbox/port@1\n    config: {container: 22}\n")
	b := withCaps("  - type: com.docker.sandbox/port@1\n    config: {container: 22}\n  - type: com.docker.sandbox/port@1\n    config: {container: 80}\n")

	ka, err := ParseDescriptor(a)
	require.NoError(t, err)
	kb, err := ParseDescriptor(b)
	require.NoError(t, err)

	assert.Equal(t, ka.Capabilities, kb.Capabilities)
}

func TestParseDescriptorRefusals(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"an anchor", minimal + "description: &a x\n", "an anchor"},
		{"an alias with its anchor", minimal + "description: &a x\nauthor: *a\n", "an anchor"},
		{"a merge key", minimal + "args:\n  b:\n    <<: {required: true}\n", "a merge key"},
		{"an include", minimal + "description: !include other.yaml\n", "a tag outside YAML's core schema"},
		{"a custom tag", minimal + "description: !!python/object x\n", "a tag outside YAML's core schema"},
		{"a null", minimal + "description:\n", "a null value"},
		{"a duplicate key", minimal + "description: a\ndescription: b\n", "written twice"},
		{"a key that is not a string", minimal + "1: a\n", "a key that is not a string"},
		{"an unknown key", minimal + "colour: red\n", "unknown"},
		{"an unknown key below a capability", string(withCaps("  - type: com.docker.sandbox/port@1\n    config: {container: 80, host: 8080}\n")), "unknown"},
		{"an unknown key of a capability", string(withCaps("  - type: com.docker.sandbox/port@1\n    mode: x\n    config: {container: 80}\n")), "unknown"},
		{"an unknown capability type", string(withCaps("  - type: com.docker.sandbox/teleport@1\n")), "does not model"},
		{"an unknown capability type of another namespace", string(withCaps("  - type: example.com/thing@1\n")), "does not model"},
		{"an optional unknown capability type", string(withCaps("  - type: com.docker.sandbox/teleport@1\n    optional: true\n")), "does not model"},
		{"an unknown capability type in a group", string(withCaps("  - group:\n      capabilities:\n        - type: com.docker.sandbox/teleport@2\n")), "does not model"},
		{"a config for a type that takes none", string(withCaps("  - type: com.docker.sandbox/privileged@1\n    config: {}\n")), "takes no config"},
		{"a missing schema version", "kind: mixin\n", "schemaVersion"},
		{"another schema version", "schemaVersion: \"2\"\nkind: mixin\n", "schemaVersion"},
		{"a kind that is not one", "schemaVersion: \"3\"\nkind: thing\n", "kind"},
		{"two documents", minimal + "---\n" + minimal, "more than one document"},
		{"a list", "- a\n", "not one mapping"},
		{"an empty file", "", "empty"},
		{"text that is not UTF-8", minimal + "description: \xff\n", "not UTF-8"},
		{"a file over the cap", minimal + "description: " + strings.Repeat("a", MaxDescriptorSize) + "\n", "over the cap"},
		{"a nesting too deep", minimal + "args: " + strings.Repeat("[", 80) + strings.Repeat("]", 80) + "\n", "deeper"},
		{"a mixin with sbx@1", string(withCaps("  - type: com.docker.sandbox/sbx@1\n")), "workload"},
		{"a set without kits", "schemaVersion: \"3\"\nkind: set\n", "kits"},
		{"build and dockerfile both", minimal + "build: FROM x\ndockerfile: a\n", "build"},
		{"an arg with env and buildArg", minimal + "args:\n  a: {env: A, buildArg: A}\n", "env"},
		{"a non-https icon", minimal + "iconUrl: http://example.com/a.png\n", "https"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDescriptor([]byte(tt.doc))

			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseDescriptorMessageNeverEchoesAValue(t *testing.T) {
	_, err := ParseDescriptor([]byte(minimal + "hunter2: secret-value\n"))

	require.ErrorIs(t, err, ErrInvalid)
	// The key is hostile text: it is quoted, and cut when long.
	assert.NotContains(t, err.Error(), "secret-value")
	_, err = ParseDescriptor([]byte(minimal + "\"\x1b[31mred\": x\n"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\x1b")
}

func TestParseAnnotationReadsCompactJSON(t *testing.T) {
	doc := `{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/lifecycle@1","config":{"files":[{"path":"/a","content":"x\/y\u00e9\n"}]}}]}`

	kit, err := ParseAnnotation([]byte(doc))

	require.NoError(t, err)
	require.NotNil(t, kit.Capabilities.Lifecycle)
	assert.Equal(t, "x/yé\n", kit.Capabilities.Lifecycle.Files[0].Content)
}

func FuzzParseDescriptor(f *testing.F) {
	seed, err := os.ReadFile("../../kits/git-ssh-sign/git-ssh-sign.yaml")
	require.NoError(f, err)
	f.Add(seed)
	f.Add([]byte(minimal))
	f.Add([]byte(`{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/port@1","config":{"container":80}}]}`))
	f.Add([]byte(minimal + "x: &a [*a]\n"))
	f.Add([]byte(minimal + "capabilities:\n  - type: com.docker.sandbox/credential@1\n    config: {service: a, phase: [install], apiKey: {name: A}}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		kit, err := ParseDescriptor(data)
		if err != nil {
			require.ErrorIs(t, err, ErrInvalid, "a refusal is always ErrInvalid")
			return
		}
		// What the grammar accepts is a descriptor of the version it reads.
		assert.Contains(t, []string{"workload", "mixin", "set"}, kit.Capabilities.Kind)
		again, err := ParseDescriptor(data)
		require.NoError(t, err)
		assert.Equal(t, kit, again, "parsing twice gives the same set")
	})
}

func TestParseAnnotationRefusals(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"a duplicate member", `{"schemaVersion":"3","kind":"mixin","kind":"workload"}`},
		{"a null", `{"schemaVersion":"3","kind":"mixin","description":null}`},
		{"an unknown member", `{"schemaVersion":"3","kind":"mixin","colour":"red"}`},
		{"an unknown capability type", `{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/teleport@1"}]}`},
		{"text that is not an object", `["a"]`},
		{"text that is not JSON", `schemaVersion: "3"`},
		{"a nesting too deep", `{"schemaVersion":"3","kind":"mixin","args":` + strings.Repeat("[", 80) + strings.Repeat("]", 80) + `}`},
		{"a document over the cap", `{"schemaVersion":"3","kind":"mixin","description":"` + strings.Repeat("a", MaxDescriptorSize) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAnnotation([]byte(tt.doc))

			require.ErrorIs(t, err, ErrInvalid)
		})
	}
}

func FuzzParseAnnotation(f *testing.F) {
	f.Add([]byte(`{"schemaVersion":"3","kind":"mixin"}`))
	f.Add([]byte(`{"schemaVersion":"3","kind":"workload","capabilities":[{"type":"com.docker.sandbox/sbx@1"},{"group":{"capabilities":[{"type":"com.docker.sandbox/long-running@1"}]}}]}`))
	f.Add([]byte(`{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/network-policy@2","config":{"runtime":{"allow":["a.example",{"hosts":["b.example"],"methods":["GET"],"paths":["/x"]}]}}}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		kit, err := ParseAnnotation(data)
		if err != nil {
			require.ErrorIs(t, err, ErrInvalid)
			return
		}
		assert.Contains(t, []string{"workload", "mixin", "set"}, kit.Capabilities.Kind)
	})
}

func TestNumbersPastTwoToTheFiftyThreeAreRefused(t *testing.T) {
	port := func(n string) string {
		return `{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/port@1","config":{"container":` + n + `}}]}`
	}
	yamlPort := func(n string) []byte {
		return []byte(minimal + "capabilities:\n  - type: com.docker.sandbox/port@1\n    config: {container: " + n + "}\n")
	}
	for _, n := range []string{"9007199254740993", "-9007199254740993", "18446744073709551616", "123456789012345678901234567890"} {
		t.Run(n, func(t *testing.T) {
			_, err := ParseAnnotation([]byte(port(n)))
			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), "2^53")

			_, err = ParseDescriptor(yamlPort(n))
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
	t.Run("two to the fifty-three is read exactly", func(t *testing.T) {
		kit, err := ParseAnnotation([]byte(port("9007199254740992")))
		require.NoError(t, err)
		assert.Equal(t, "9007199254740992", kit.Capabilities.Ports[0].Container)
		kit, err = ParseDescriptor(yamlPort("9007199254740992"))
		require.NoError(t, err)
		assert.Equal(t, "9007199254740992", kit.Capabilities.Ports[0].Container)
	})
	t.Run("a number keeps its digits", func(t *testing.T) {
		kit, err := ParseDescriptor(capability(typeResources, "{cpu: 0.1}"))
		require.NoError(t, err)
		assert.Equal(t, "0.1", kit.Capabilities.Resources.CPU)
		kit, err = ParseAnnotation([]byte(`{"schemaVersion":"3","kind":"mixin","capabilities":[{"type":"com.docker.sandbox/resources@1","config":{"cpu":0.1}}]}`))
		require.NoError(t, err)
		assert.Equal(t, "0.1", kit.Capabilities.Resources.CPU)
	})
}

func TestSSHAgentWithoutAPhase(t *testing.T) {
	const cfg = "{unrestricted: false, sign: [git]}"
	t.Run("a local file is read as the widest phases", func(t *testing.T) {
		// The sitting M0 descriptor of git-ssh-sign names no phase and sbx
		// v0.48.0 accepted it; the kit schema of the pinned frontend
		// requires one. The set shows both phases, the widest reading.
		kit, err := ParseDescriptor(capability(typeSSHAgent, cfg))

		require.NoError(t, err)
		assert.Equal(t, []string{"install", "runtime"}, kit.Capabilities.SSHAgent[0].Phases)
	})
	t.Run("the sitting fixture", func(t *testing.T) {
		data, err := os.ReadFile(fixturePath)
		require.NoError(t, err)

		kit, err := ParseDescriptor(data)

		require.NoError(t, err)
		assert.Equal(t, []string{"install", "runtime"}, kit.Capabilities.SSHAgent[0].Phases)
	})
	t.Run("an annotation is refused", func(t *testing.T) {
		doc := `{"schemaVersion":"3","kind":"workload","capabilities":[{"type":"com.docker.sandbox/ssh-agent@1","config":{"unrestricted":false,"sign":["git"]}}]}`

		_, err := ParseAnnotation([]byte(doc))

		require.ErrorIs(t, err, ErrInvalid)
		assert.Contains(t, err.Error(), "no phase")
	})
}
