// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capability returns a mixin with the one capability of the type given
// and the config text given, written in YAML flow form.
func capability(typ, config string) []byte {
	doc := minimal + "capabilities:\n  - type: " + typ + "\n"
	if config != "" {
		doc += "    config: " + config + "\n"
	}
	return []byte(doc)
}

func TestNormalizeEachCapabilityType(t *testing.T) {
	tests := []struct {
		name   string
		typ    string
		config string
		check  func(t *testing.T, c Capabilities)
	}{
		{"network-policy@1", typeNetwork1, `{install: {allow: [b.example, a.example]}, runtime: {deny: ["*"]}}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.NotNil(t, c.Network)
			assert.Equal(t, []NetEntry{{Hosts: []string{"a.example"}}, {Hosts: []string{"b.example"}}}, c.Network.Install.Allow)
			assert.Equal(t, []NetEntry{{Hosts: []string{"*"}}}, c.Network.Runtime.Deny)
		}},
		{"network-policy@2", typeNetwork2, `{runtime: {allow: [a.example, {hosts: [c.example, b.example], methods: [POST, GET], paths: [/y, /x]}]}}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []NetEntry{
				{Hosts: []string{"a.example"}},
				{Hosts: []string{"b.example", "c.example"}, Methods: []string{"GET", "POST"}, Paths: []string{"/x", "/y"}},
			}, c.Network.Runtime.Allow)
		}},
		{"credential@1 api key", typeCredential, `{service: github, phase: [runtime, install], apiKey: {name: GH_TOKEN, proxyManaged: true, inject: [{domain: api.github.com, header: Authorization, format: "Bearer %s"}]}}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			require.Len(t, c.Credentials, 1)
			cr := c.Credentials[0]
			assert.Equal(t, "github", cr.Service)
			assert.Equal(t, []string{"install", "runtime"}, cr.Phases)
			assert.Equal(t, "true", cr.APIKey.ProxyManaged)
			assert.Equal(t, "api.github.com", cr.APIKey.Inject[0].Domain)
		}},
		{"credential@1 oauth", typeCredential, `{service: x, phase: runtime, oauth: {tokenEndpoint: {host: auth.example, path: /t}, resourceHosts: [b, a], credentialFile: {path: /home/a.json, format: json, structure: {z: 1, a: {k: v}}}, passthrough: false}}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			o := c.Credentials[0].OAuth
			assert.Equal(t, []string{"a", "b"}, o.ResourceHosts)
			assert.Equal(t, map[string]any{"a": map[string]any{"k": "v"}, "z": float64(1)}, o.CredentialFile.Structure)
			assert.Equal(t, "false", o.Passthrough)
		}},
		{"ssh-agent@1", typeSSHAgent, `{phase: runtime, unrestricted: false, authenticate: [github.com], sign: [git, file]}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []string{"file", "git"}, c.SSHAgent[0].Sign)
			assert.Equal(t, []string{"github.com"}, c.SSHAgent[0].Authenticate)
		}},
		{"agent-skills@1", typeSkills, `{path: /home/agent/.claude/skills, mode: readonly}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Skill{{Discovery: true, Path: "/home/agent/.claude/skills", Mode: "readonly"}}, c.Skills)
		}},
		{"agent-skill@1", typeSkill, `{path: /opt/skills/a, name: a}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Skill{{Path: "/opt/skills/a", Name: "a"}}, c.Skills)
		}},
		{"volume@1", typeVolume, `{path: /data, size: 1G, tmpfs: false, mode: "0700"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Volume{{Kind: "volume", Path: "/data", Size: "1G", Tmpfs: "false", Mode: "0700"}}, c.Volumes)
		}},
		{"host-mount@1", typeHostMount, `{path: /shared}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Volume{{Kind: "host-mount", Path: "/shared"}}, c.Volumes)
		}},
		{"port@1", typePort, `{container: 8080, transport: tcp, name: web}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Port{{Name: "web", Container: "8080", Transport: "tcp"}}, c.Ports)
		}},
		{"usb-device@1", typeUSB, `{vendorId: "1d6b", productId: "0002"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Device{{VendorID: "1d6b", ProductID: "0002"}}, c.Devices)
		}},
		{"resources@1", typeResources, `{cpu: 1.5, memory: 2GiB}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, &Resources{CPU: "1.5", Memory: "2GiB"}, c.Resources)
		}},
		{"agent-sessions@1", typeSessions, `{prompt: ["-p", "{{.Prompt}}"], list: "ls sessions"}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []string{"-p", "{{.Prompt}}"}, c.Sessions.Prompt)
			assert.Equal(t, &Command{Shell: "ls sessions"}, c.Sessions.List)
		}},
		{"lifecycle@1", typeLifecycle, `{install: [{command: "echo a"}, {command: [echo, b], user: agent, env: [B, A]}], startup: [{command: [serve], background: true}], files: [{path: /z, content: z}, {path: /a, content: a, mode: "0600", overwrite: false}], interactive: [--tty]}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			l := c.Lifecycle
			assert.Equal(t, Command{Shell: "echo a"}, l.Install[0].Command, "hooks keep the order they run in")
			assert.Equal(t, []string{"A", "B"}, l.Install[1].Env)
			assert.Equal(t, "true", l.Startup[0].Background)
			assert.Equal(t, "/a", l.Files[0].Path, "files are sorted by path")
			assert.Equal(t, "false", l.Files[0].Overwrite)
			assert.Equal(t, []string{"--tty"}, l.Interactive)
		}},
		{"agent-context@1", typeContext, `{filename: CLAUDE.md, directory: /home/agent/.claude, content: hello}`, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Context{{Filename: "CLAUDE.md", Directory: "/home/agent/.claude", Content: "hello"}}, c.Context)
		}},
		{"privileged@1", typePrivileged, ``, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Flag{{Type: typePrivileged}}, c.Flags)
		}},
		{"kit-registry@1", typeKitRegistry, ``, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Flag{{Type: typeKitRegistry}}, c.Flags)
		}},
		{"long-running@1", typeLongRunning, ``, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Flag{{Type: typeLongRunning}}, c.Flags)
		}},
		{"git-identity@1", typeGitIdentity, ``, func(t *testing.T, c Capabilities) {
			t.Helper()
			assert.Equal(t, []Flag{{Type: typeGitIdentity}}, c.Flags)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kit, err := ParseDescriptor(capability(tt.typ, tt.config))

			require.NoError(t, err)
			tt.check(t, kit.Capabilities)
		})
	}
}

func TestNormalizeSbxIsAWorkloadsAlone(t *testing.T) {
	doc := "schemaVersion: \"3\"\nkind: workload\ncapabilities:\n  - type: com.docker.sandbox/sbx@1\n"

	kit, err := ParseDescriptor([]byte(doc))

	require.NoError(t, err)
	assert.Equal(t, []Flag{{Type: typeSbx}}, kit.Capabilities.Flags)
}

func TestNormalizeGroupsAndOptional(t *testing.T) {
	doc := minimal + `capabilities:
  - group:
      optional: true
      capabilities:
        - type: com.docker.sandbox/port@1
          config: {container: 80}
  - type: com.docker.sandbox/port@1
    optional: true
    config: {container: 81}
  - type: com.docker.sandbox/port@1
    config: {container: 82}
`

	kit, err := ParseDescriptor([]byte(doc))

	require.NoError(t, err)
	assert.Equal(t, []Port{
		{Container: "80", Optional: true},
		{Container: "81", Optional: true},
		{Container: "82"},
	}, kit.Capabilities.Ports)
}

func TestNormalizeKeepsReferencesAsWritten(t *testing.T) {
	kit, err := ParseDescriptor(capability(typePort, `{container: "${{ kit.args.port }}", transport: "${{ kit.args.t }}"}`))

	require.NoError(t, err)
	assert.Equal(t, "${{ kit.args.port }}", kit.Capabilities.Ports[0].Container)
}

func TestNormalizeRefusals(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		cfg  string
		want string
	}{
		{"a network key the type lacks", typeNetwork1, `{runtime: {allow: [a.example], maybe: [x]}}`, "unknown"},
		{"a network@1 entry that is an object", typeNetwork1, `{runtime: {allow: [{hosts: [a.example]}]}}`, "host patterns only"},
		{"an entry with paths and no methods", typeNetwork2, `{runtime: {allow: [{hosts: [a.example], paths: [/x]}]}}`, "no methods"},
		{"an entry without hosts", typeNetwork2, `{runtime: {allow: [{methods: [GET]}]}}`, "without hosts"},
		{"an empty host pattern", typeNetwork2, `{runtime: {allow: [""]}}`, "empty host"},
		{"an entry that is a number", typeNetwork2, `{runtime: {allow: [1]}}`, "not a host pattern"},
		{"a credential without a service", typeCredential, `{phase: runtime, apiKey: {name: A}}`, "no service"},
		{"a credential without a phase", typeCredential, `{service: a, apiKey: {name: A}}`, "no phase"},
		{"a credential with a phase of another name", typeCredential, `{service: a, phase: always, apiKey: {name: A}}`, "phase"},
		{"a credential with neither form", typeCredential, `{service: a, phase: runtime}`, "neither apiKey nor oauth"},
		{"an api key with neither name nor inject", typeCredential, `{service: a, phase: runtime, apiKey: {proxyManaged: true}}`, "neither name nor inject"},
		{"an ssh-agent restricted and unbounded", typeSSHAgent, `{phase: runtime, unrestricted: false}`, "bounds neither"},
		{"an ssh-agent unrestricted and bounded", typeSSHAgent, `{phase: runtime, sign: [git]}`, "unrestricted and bounds"},
		{"an ssh-agent with an empty list", typeSSHAgent, `{phase: runtime, unrestricted: false, sign: []}`, "empty"},
		{"an ssh-agent unrestricted that is a word", typeSSHAgent, `{phase: runtime, unrestricted: maybe}`, "not a boolean"},
		{"a volume without a path", typeVolume, `{size: 1G}`, "no path"},
		{"a host-mount without a config", typeHostMount, ``, "needs a config"},
		{"a host path in a host-mount", typeHostMount, `{path: /a, source: /etc}`, "unknown"},
		{"a port without a container port", typePort, `{name: a}`, "no container port"},
		{"a usb device with both shapes", typeUSB, `{vendorId: a, productId: b, class: c}`, "exactly one"},
		{"a usb device with a vendor alone", typeUSB, `{vendorId: a}`, "exactly one"},
		{"resources of a list", typeResources, `{cpu: [1]}`, "not a string, a number or a boolean"},
		{"agent-sessions of nothing", typeSessions, `{}`, "no verb"},
		{"a lifecycle of nothing", typeLifecycle, `{}`, "declares nothing"},
		{"a hook without a command", typeLifecycle, `{install: [{user: agent}]}`, "without a command"},
		{"a hook with an empty argv", typeLifecycle, `{install: [{command: []}]}`, "command"},
		{"a file without a path", typeLifecycle, `{files: [{content: x}]}`, "file path"},
		{"an agent-context of nothing", typeContext, `{}`, "declares nothing"},
		{"a directory without a filename", typeContext, `{directory: /a, content: x}`, "without a filename"},
		{"an agent-skills without a config", typeSkills, ``, "needs a config"},
		{"an agent-skill without a config", typeSkill, ``, "needs a config"},
		{"an agent-skill with a mode", typeSkill, `{path: /a, mode: readonly}`, "no mode"},
		{"an agent-skills with a name", typeSkills, `{path: /a, name: b}`, "no name"},
		{"an agent-skills with another mode", typeSkills, `{path: /a, mode: write}`, "readonly or readwrite"},
		{"a config on git-identity", typeGitIdentity, `{}`, "takes no config"},
		{"a config on sbx", typeKitRegistry, `{a: 1}`, "takes no config"},
		{"a type that is not a type", "nonsense", ``, "<namespace>/<name>@<version>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDescriptor(capability(tt.typ, tt.cfg))

			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestNormalizeSingletonsOnce(t *testing.T) {
	for _, typ := range []string{typeNetwork1, typeResources, typeLifecycle} {
		t.Run(typ, func(t *testing.T) {
			cfg := map[string]string{typeNetwork1: "{}", typeResources: "{cpu: 1}", typeLifecycle: "{interactive: [a]}"}[typ]
			doc := string(capability(typ, cfg)) + "  - type: " + typ + "\n    config: " + cfg + "\n"

			_, err := ParseDescriptor([]byte(doc))

			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), "singleton")
		})
	}
	t.Run("network-policy of both versions", func(t *testing.T) {
		doc := string(capability(typeNetwork1, "{}")) + "  - type: " + typeNetwork2 + "\n    config: {}\n"

		_, err := ParseDescriptor([]byte(doc))

		require.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("sessions", func(t *testing.T) {
		doc := string(capability(typeSessions, "{continue: [a]}")) + "  - type: " + typeSessions + "\n    config: {continue: [a]}\n"

		_, err := ParseDescriptor([]byte(doc))

		require.ErrorIs(t, err, ErrInvalid)
	})
}

func TestNormalizeGroupRefusals(t *testing.T) {
	tests := []struct {
		name string
		caps string
		want string
	}{
		{"a group with a type", "  - type: com.docker.sandbox/port@1\n    group: {capabilities: [{type: com.docker.sandbox/privileged@1}]}\n", "group and a capability"},
		{"an empty group", "  - group: {capabilities: []}\n", "no capabilities"},
		{"a member with optional", "  - group:\n      capabilities:\n        - type: com.docker.sandbox/privileged@1\n          optional: true\n", "optional or group"},
		{"a nested group", "  - group:\n      capabilities:\n        - group: {capabilities: []}\n", "optional or group"},
		{"a member with sbx in a mixin", "  - group:\n      capabilities:\n        - type: com.docker.sandbox/sbx@1\n", "workload"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDescriptor(withCaps(tt.caps))

			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseDescriptorArgsAndKits(t *testing.T) {
	ok := minimal + "args:\n  A:\n    required: true\n    pattern: '^[a-z]+$'\n    env: A_ENV\n  B:\n    default: x\n    enum: []\n"
	_, err := ParseDescriptor([]byte(ok))
	require.NoError(t, err)

	tests := []struct {
		name string
		doc  string
	}{
		{"an arg named like a flag", minimal + "args:\n  a-b: {}\n"},
		{"an arg with a bad pattern", minimal + "args:\n  A: {pattern: '['}\n"},
		{"an arg with enum and pattern", minimal + "args:\n  A: {enum: [a], pattern: a}\n"},
		{"an arg required with a default", minimal + "args:\n  A: {required: true, default: a}\n"},
		{"an arg with a bad env", minimal + "args:\n  A: {env: a-b}\n"},
		{"a kit by URL", "schemaVersion: \"3\"\nkind: set\nkits:\n  - ref: https://example.com/k\n"},
		{"a kit by git", "schemaVersion: \"3\"\nkind: set\nkits:\n  - ref: git+ssh://example.com/k\n"},
		{"a kit with a bad digest", "schemaVersion: \"3\"\nkind: set\nkits:\n  - ref: example.com/k\n    digest: sha256:abc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDescriptor([]byte(tt.doc))

			require.ErrorIs(t, err, ErrInvalid)
		})
	}
	set := "schemaVersion: \"3\"\nkind: set\nkits:\n  - ref: example.com/k\n    digest: sha256:" + strings.Repeat("a", 64) + "\n"
	_, err = ParseDescriptor([]byte(set))
	require.NoError(t, err)
}
