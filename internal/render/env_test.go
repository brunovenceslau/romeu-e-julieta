// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// shopInput is the render input of shopSpec, as sync passes it.
func shopInput(t *testing.T) render.EnvInput {
	t.Helper()
	return render.EnvInput{
		Project: project(t, shopSpec), Host: host(t, hostSettings), Home: "/home/agent",
		Kits: shopKits(t),
	}
}

// portsSpec is a project with one repo, no kit, no secret and the
// three forms of a port spec of 03 3.1.
const portsSpec = `schema: project/v1
name: ports
repos:
  - url: https://github.com/example/ports
    primary: true
workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5
agent: sbx-kit-claude
ports: ["3000", "8080:80/tcp", "127.0.0.1:5353:53/udp"]
`

// The goldens of sbxenv.yaml (03 3.3, REQ-0732): the project of 03 3.2
// with every kind of kit, and a project with ports. Each is the raw
// output of the marshaller; A5 hashes its normalized form (T019).
func TestEnvGolden(t *testing.T) {
	got, err := render.Env(shopInput(t))
	require.NoError(t, err)
	golden(t, "env/shop.sbxenv.yaml", got)

	ports := render.EnvInput{Project: project(t, portsSpec), Host: host(t, hostSettings), Home: "/home/agent"}
	got, err = render.Env(ports)
	require.NoError(t, err)
	golden(t, "env/ports.sbxenv.yaml", got)
}

// The shape rules of 03 3.3 on the decoded file: the keys of the sbx
// env-file schema alone, no x-* key and no lifecycle hook (REQ-0200),
// the fixed read-only mount of ./.romeu/bin (REQ-0583), paths of the
// form ./x (REQ-0142), and a secret that is a command alone (I8).
//
//romeu:invariant I8 test
func TestEnvShape(t *testing.T) {
	got, err := render.Env(shopInput(t))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(got, &doc))
	var keys []string
	for k := range doc {
		keys = append(keys, k)
		assert.False(t, strings.HasPrefix(k, "x-"), "the key %s", k)
	}
	assert.ElementsMatch(t, []string{"schemaVersion", "name", "agent", "workspace", "additionalWorkspaces", "kits", "sandboxOptions", "secrets", "ports"}, keys)
	assert.NotContains(t, string(got), "lifecycle")
	assert.Equal(t, "1", doc["schemaVersion"])
	assert.Equal(t, "./shop-api", doc["workspace"])
	assert.Equal(t, []any{
		"./memory/shop-api",
		"./memory/shop-web",
		map[string]any{"path": "./.romeu/bin", "readOnly": true},
	}, doc["additionalWorkspaces"])
	for _, k := range doc["kits"].([]any) {
		src := k.(map[string]any)["source"].(string)
		assert.True(t, strings.HasPrefix(src, "./.romeu/kits/") || strings.HasPrefix(src, "docker.io/"), "the source %s", src)
	}
	assert.Equal(t, map[string]any{"github": map[string]any{
		"command": "/usr/bin/env -i HOME=/home/agent PATH=/usr/bin:/bin /usr/bin/security find-generic-password -s romeu/shop/github -w",
	}}, doc["secrets"])
	assert.Equal(t, []any{}, doc["ports"])
}

// The same input renders the same bytes.
func TestEnvIsDeterministic(t *testing.T) {
	a, err := render.Env(shopInput(t))
	require.NoError(t, err)
	for range 20 {
		b, err := render.Env(shopInput(t))
		require.NoError(t, err)
		require.Equal(t, a, b)
	}
}

// A string that YAML would read as another type is quoted, so the file
// reads back as what was meant.
//
//romeu:invariant I11 test
func TestEnvQuotesStringsThatLookTyped(t *testing.T) {
	in := shopInput(t)
	in.Project.Kits[2].Args = map[string]string{"tz": "yes", "port": "8080", "empty": "", "null": "null", "comment": "# x"}
	got, err := render.Env(in)
	require.NoError(t, err)
	var doc struct {
		Kits []struct {
			Args map[string]string `yaml:"args"`
		} `yaml:"kits"`
	}
	require.NoError(t, yaml.Unmarshal(got, &doc))
	assert.Equal(t, in.Project.Kits[2].Args, doc.Kits[3].Args)
}

// Each input render refuses, with the reason it names.
func TestEnvErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(in *render.EnvInput)
		want   string
	}{
		{"a spec secret with no host-settings entry", func(in *render.EnvInput) {
			in.Project.Secrets = []string{"npm", "gitlab"}
		}, `the secret "gitlab" has no host-settings entry "gitlab@shop"`},
		{"an entry of another project never satisfies the spec (REQ-0491)", func(in *render.EnvInput) {
			s := in.Host.Secrets["github@shop"]
			delete(in.Host.Secrets, "github@shop")
			in.Host.Secrets["github@other"] = s
		}, `the secret "github" has no host-settings entry "github@shop"`},
		{"two secrets of the spec on one sbx service", func(in *render.EnvInput) {
			e := in.Host.Secrets["npm@shop"]
			e.Service = "github"
			in.Host.Secrets["npm@shop"] = e
			in.Project.Secrets = []string{"github", "npm"}
		}, `two secrets of the spec name the sbx service "github"`},
		{"a host-settings entry with no service", func(in *render.EnvInput) {
			e := in.Host.Secrets["github@shop"]
			e.Service = ""
			in.Host.Secrets["github@shop"] = e
		}, `the host-settings entry "github@shop" names no sbx service`},
		{"a secret argv that is not absolute", func(in *render.EnvInput) {
			e := in.Host.Secrets["github@shop"]
			e.Argv = []string{"security", "-w"}
			in.Host.Secrets["github@shop"] = e
		}, "does not start with an absolute path"},
		{"a relative HOME", func(in *render.EnvInput) { in.Home = "home/dev" }, `HOME "home/dev" is not an absolute clean path`},
		{"an unclean HOME", func(in *render.EnvInput) { in.Home = "/home/agent/" }, `HOME "/home/agent/" is not an absolute clean path`},
		{"fewer kits materialized than the spec has", func(in *render.EnvInput) { in.Kits = in.Kits[:4] }, "the spec has 5 kits and 4 were materialized"},
		{"a materialized kit out of the spec's order", func(in *render.EnvInput) {
			in.Kits[0], in.Kits[1] = in.Kits[1], in.Kits[0]
		}, `kit 1 of the spec is "julieta", and the materialized kit there is "julieta-claude"`},
		{"a spec arg that romeu renders", func(in *render.EnvInput) {
			in.Project.Kits[1].Args = map[string]string{"julietaBin": "/elsewhere"}
		}, `the spec sets the arg "julietaBin", which romeu renders`},
		{"a repo dir that is not one element", func(in *render.EnvInput) { in.Project.Repos[1].Dir = "a/b" }, `"a/b" is not one path element`},
		{"a repo dir romeu reserves", func(in *render.EnvInput) { in.Project.Repos[1].Dir = "Memory" }, `"Memory" is a name romeu reserves`},
		{"a kit id romeu reserves", func(in *render.EnvInput) {
			in.Project.Kits[0].Product = "review"
			in.Kits[0].ID = "review"
		}, `"review" is a name romeu reserves`},
		{"no primary repo", func(in *render.EnvInput) { in.Project.Repos[0].Primary = false }, "the spec has 0 primary repos"},
		{"two primary repos", func(in *render.EnvInput) { in.Project.Repos[1].Primary = true }, "the spec has 2 primary repos"},
		{"a workload that is no digest reference", func(in *render.EnvInput) {
			in.Project.Workload = "docker.io/docker/sbx-kit-claude:latest"
		}, "render: "},
		{"a port spec of another address", func(in *render.EnvInput) {
			in.Project.Ports = []string{"0.0.0.0:80:80"}
		}, "binds a host address other than 127.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := shopInput(t)
			tt.change(&in)
			_, err := render.Env(in)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// The secret command of 03 3.3 (REQ-0201, REQ-0208, REQ-0490): the argv
// of host settings under /usr/bin/env -i with HOME and PATH alone, each
// word quoted by internal/shquote, so no word of the argv reaches sh as
// anything but itself.
//
//romeu:invariant I8 test
func TestSecretCommand(t *testing.T) {
	tests := []struct {
		name string
		home string
		argv []string
		want string
	}{
		{"the Keychain form", "/Users/agent", []string{"/usr/bin/security", "find-generic-password", "-s", "romeu/shop/github", "-w"},
			"/usr/bin/env -i HOME=/Users/agent PATH=/usr/bin:/bin /usr/bin/security find-generic-password -s romeu/shop/github -w"},
		{"a word with a space and a quote", "/home/agent", []string{"/opt/bin/op", "read", "op://dev/it's a token"},
			`/usr/bin/env -i HOME=/home/agent PATH=/usr/bin:/bin /opt/bin/op read 'op://dev/it'\''s a token'`},
		{"a word that sh would expand", "/home/agent", []string{"/bin/echo", "$(id)", "`id`", "$HOME", "a;b", "*"},
			"/usr/bin/env -i HOME=/home/agent PATH=/usr/bin:/bin /bin/echo '$(id)' '`id`' '$HOME' 'a;b' '*'"},
		{"a HOME with a space", "/home/a b", []string{"/bin/true"},
			"/usr/bin/env -i HOME='/home/a b' PATH=/usr/bin:/bin /bin/true"},
		{"an empty argument stays one word", "/home/agent", []string{"/bin/printf", ""},
			"/usr/bin/env -i HOME=/home/agent PATH=/usr/bin:/bin /bin/printf ''"},
		{"an equals sign past argv[0] is quoted", "/home/agent", []string{"/bin/tool", "A=b"},
			"/usr/bin/env -i HOME=/home/agent PATH=/usr/bin:/bin /bin/tool 'A=b'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render.SecretCommand(tt.home, tt.argv)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The argv SecretCommand refuses.
func TestSecretCommandErrors(t *testing.T) {
	tests := []struct {
		name string
		home string
		argv []string
		want string
	}{
		{"no argv", "/home/agent", nil, "does not start with an absolute path"},
		{"a relative argv[0]", "/home/agent", []string{"security"}, "does not start with an absolute path"},
		{"an argv[0] env reads as a variable", "/home/agent", []string{"/bin/A=b"}, "argv[0] holds an ="},
		{"a NUL in an argument", "/home/agent", []string{"/bin/true", "a\x00b"}, "argv: word 2"},
		{"a NUL in HOME", "/home/\x00", []string{"/bin/true"}, "HOME: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := render.SecretCommand(tt.home, tt.argv)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// Every string of the spec that reaches sbxenv.yaml reads back as
// itself, whatever its bytes: Env refuses or writes a file that decodes
// to what was meant, never one that means another thing (I11, REQ-0494).
//
//romeu:invariant I11 test
func FuzzEnvRoundTrip(f *testing.F) {
	for _, s := range []string{"Etc/UTC", "yes", "8080", "", "null", "# x", "a: b", "- a", "'\"", "{x}", " ", "\x00", "~"} {
		f.Add(s, s)
	}
	f.Fuzz(func(t *testing.T, arg, agent string) {
		in := shopInput(t)
		in.Project.Kits[2].Args = map[string]string{"tz": arg}
		in.Project.Agent = agent
		got, err := render.Env(in)
		if err != nil {
			return
		}
		var doc struct {
			Agent string `yaml:"agent"`
			Kits  []struct {
				Args map[string]string `yaml:"args"`
			} `yaml:"kits"`
		}
		require.NoError(t, yaml.Unmarshal(got, &doc))
		assert.Equal(t, agent, doc.Agent)
		assert.Equal(t, arg, doc.Kits[3].Args["tz"])
	})
}
