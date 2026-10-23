// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/shquote"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// EnvSchemaVersion is the schemaVersion of the sbx env file, which
// belongs to sbx (03 3.3).
const EnvSchemaVersion = "1"

// SecretPath is the PATH a secret argv runs with (REQ-0208).
const SecretPath = "/usr/bin:/bin"

// EnvInput is what sbxenv.yaml is rendered from: the input of a render
// and the kits the Materializer wrote for it.
type EnvInput struct {
	Input
	// Kits are the kits the Materializer wrote, in the spec's order.
	Kits []MaterializedKit
}

// envFile is sbxenv.yaml, in the order of 03 3.3. The keys follow the
// documented env-file schema of sbx
// (https://docs.docker.com/ai/sandboxes/configuration/environment-files/):
// no x-* key, since the metadata lives in render.json, and no lifecycle
// hook (REQ-0200).
type envFile struct {
	SchemaVersion        string               `yaml:"schemaVersion"`
	Name                 string               `yaml:"name"`
	Agent                string               `yaml:"agent"`
	Workspace            string               `yaml:"workspace"`
	AdditionalWorkspaces []any                `yaml:"additionalWorkspaces"`
	Kits                 []envKit             `yaml:"kits"`
	SandboxOptions       *envOptions          `yaml:"sandboxOptions,omitempty"`
	Secrets              map[string]envSecret `yaml:"secrets,omitempty"`
	Ports                []Port               `yaml:"ports"`
}

// mount is an additional workspace with options: the fixed mount of
// .romeu/bin, read-only (REQ-0583).
type mount struct {
	Path     string `yaml:"path"`
	ReadOnly bool   `yaml:"readOnly"`
}

// envKit is one kit of the env file: the workload by its reference, or
// a local kit dir by its path, with its args.
type envKit struct {
	Source string            `yaml:"source"`
	Args   map[string]string `yaml:"args,omitempty"`
}

// envOptions are the sandbox options of the spec, as sbx names them.
type envOptions struct {
	CPUs   int    `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// envSecret is how sbx fetches one secret: a command, never a value
// (REQ-0201, I8).
type envSecret struct {
	Command string `yaml:"command"`
}

// RenderEnv returns sbxenv.yaml for in (03 3.3), written through the
// YAML marshaller alone (REQ-0198, I11). The paths in it are ./x,
// relative to the file: the primary repo's host clone is the workspace,
// and each repo's memory dir and the fixed mount ./.romeu/bin,
// read-only, are the additional workspaces (REQ-0142). The kits are the
// workload first, then each kit of the spec in its order from its
// content-addressed dir. Each secret is a command that runs the
// host-settings argv under /usr/bin/env -i with HOME and PATH alone,
// built by internal/shquote; no secret value is ever rendered (I8).
// What it writes it reads back, and a document that does not read back
// as what was meant is an error, never a file.
//
//romeu:invariant I8 guard
//romeu:invariant I11 guard
func RenderEnv(in EnvInput) ([]byte, error) {
	f, err := envOf(in)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(f); err != nil {
		return nil, fmt.Errorf("render: encode sbxenv.yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("render: encode sbxenv.yaml: %w", err)
	}
	if err := readsBack(b.Bytes(), f); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// envOf returns the env file of in.
func envOf(in EnvInput) (envFile, error) {
	p := in.Project
	f := envFile{SchemaVersion: EnvSchemaVersion, Name: p.Name, Agent: p.Agent, Ports: []Port{}}
	primary := 0
	for _, r := range p.Repos {
		if err := oneElement(r.Dir); err != nil {
			return envFile{}, fmt.Errorf("render: the repo %q: %w", r.URL, err)
		}
		if r.Primary {
			primary++
			f.Workspace = "./" + r.Dir
		}
		f.AdditionalWorkspaces = append(f.AdditionalWorkspaces, "./"+path.Join("memory", r.Dir))
	}
	if primary != 1 {
		return envFile{}, fmt.Errorf("render: the spec has %d primary repos; it is validated first, and has one", primary)
	}
	f.AdditionalWorkspaces = append(f.AdditionalWorkspaces, mount{Path: "./" + BinDir, ReadOnly: true})
	kits, err := kitsOf(p, in.Kits)
	if err != nil {
		return envFile{}, err
	}
	f.Kits = kits
	if o := p.SandboxOptions; o != nil && *o != (spec.SandboxOptions{}) {
		f.SandboxOptions = &envOptions{CPUs: o.CPUs, Memory: o.Memory}
	}
	if f.Secrets, err = secretsOf(p, in.Host, in.Home); err != nil {
		return envFile{}, err
	}
	for _, s := range p.Ports {
		port, err := ParsePort(s)
		if err != nil {
			return envFile{}, fmt.Errorf("render: %w", err)
		}
		f.Ports = append(f.Ports, port)
	}
	return f, nil
}

// kitsOf returns the kits of the env file: the workload, then each
// materialized kit, which must be the spec's kit at the same place.
func kitsOf(p spec.Project, kits []MaterializedKit) ([]envKit, error) {
	if _, err := spec.ParseWorkload(p.Workload); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	out := []envKit{{Source: p.Workload}}
	if len(kits) != len(p.Kits) {
		return nil, fmt.Errorf("render: the spec has %d kits and %d were materialized", len(p.Kits), len(kits))
	}
	for i, k := range p.Kits {
		m := kits[i]
		if id := k.Product + k.Personal; m.ID != id || id == "" || (k.Product != "" && k.Personal != "") {
			return nil, fmt.Errorf("render: kit %d of the spec is %q, and the materialized kit there is %q", i+1, id, m.ID)
		}
		if err := oneElement(m.ID); err != nil {
			return nil, fmt.Errorf("render: the kit %q: %w", m.ID, err)
		}
		var args map[string]string
		if len(k.Args)+len(m.Rendered) > 0 {
			args = make(map[string]string, len(k.Args)+len(m.Rendered))
		}
		for name, v := range k.Args {
			args[name] = v
		}
		for name, v := range m.Rendered {
			if _, ok := k.Args[name]; ok {
				return nil, fmt.Errorf("render: the kit %q: the spec sets the arg %q, which romeu renders", m.ID, name)
			}
			args[name] = v
		}
		out = append(out, envKit{Source: "./" + path.Join(LiveKitsDir, m.Dir()), Args: args})
	}
	return out, nil
}

// secretsOf returns the secrets of the env file, by the sbx service
// each host-settings entry names: a spec secret with no entry
// <name>@<project> is an error naming the key, and an entry of another
// project never satisfies it (REQ-0207).
func secretsOf(p spec.Project, hs spec.HostSettings, home string) (map[string]envSecret, error) {
	if len(p.Secrets) == 0 {
		return nil, nil
	}
	if !path.IsAbs(home) || path.Clean(home) != home {
		return nil, fmt.Errorf("render: HOME %q is not an absolute clean path", home)
	}
	out := map[string]envSecret{}
	for _, name := range p.Secrets {
		key := name + "@" + p.Name
		entry, ok := hs.Secrets[key]
		if !ok {
			return nil, fmt.Errorf("render: the secret %q has no host-settings entry %q", name, key)
		}
		if _, dup := out[entry.Service]; dup {
			return nil, fmt.Errorf("render: two secrets of the spec name the sbx service %q", entry.Service)
		}
		cmd, err := SecretCommand(home, entry.Argv)
		if err != nil {
			return nil, fmt.Errorf("render: the secret %q: %w", key, err)
		}
		out[entry.Service] = envSecret{Command: cmd}
	}
	return out, nil
}

// SecretCommand returns the command sbx runs to fetch a secret
// (03 3.3, REQ-0201, REQ-0208): argv under /usr/bin/env -i, with
// exactly HOME and PATH=/usr/bin:/bin, each word quoted by
// internal/shquote. argv[0] is absolute, and holds no "=", which env
// would read as one more variable rather than the command.
//
//romeu:invariant I8 guard
func SecretCommand(home string, argv []string) (string, error) {
	if len(argv) == 0 || !path.IsAbs(argv[0]) {
		return "", errors.New("the argv does not start with an absolute path")
	}
	if strings.Contains(argv[0], "=") {
		return "", errors.New("argv[0] holds an =, which env reads as a variable")
	}
	h, err := shquote.Quote(home)
	if err != nil {
		return "", fmt.Errorf("HOME: %w", err)
	}
	words, err := shquote.Join(argv)
	if err != nil {
		return "", fmt.Errorf("argv: %w", err)
	}
	return "/usr/bin/env -i HOME=" + h + " PATH=" + SecretPath + " " + words, nil
}

// oneElement holds a name that render joins into a path: one element,
// not a name romeu reserves inside <name>-env/, so neither a repo dir
// nor a kit dir can reach a memory dir or .romeu (03 3.1).
func oneElement(name string) error {
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return fmt.Errorf("%q is not one path element", name)
	}
	if slices.ContainsFunc(reservedDirs(), func(r string) bool { return strings.EqualFold(r, name) }) {
		return fmt.Errorf("%q is a name romeu reserves inside <name>-env/", name)
	}
	return nil
}

// reservedDirs returns the names romeu reserves inside <name>-env/, the
// one list of the repo dir rule (03 3.1).
func reservedDirs() []string {
	for _, r := range spec.Rules() {
		if r.Slug == "repo-dir" {
			return r.Reserved
		}
	}
	panic("render: internal/spec has no repo-dir rule")
}

// readsBack decodes data and requires it to be want: the marshaller
// writes some strings in a form it reads back as another (03 3.3).
func readsBack(data []byte, want envFile) error {
	var got envFile
	if err := yaml.Unmarshal(data, &got); err != nil {
		return fmt.Errorf("render: sbxenv.yaml does not read back: %w", err)
	}
	for i, w := range got.AdditionalWorkspaces {
		if m, ok := w.(map[string]any); ok {
			p, _ := m["path"].(string)
			ro, _ := m["readOnly"].(bool)
			if len(m) == 2 {
				got.AdditionalWorkspaces[i] = mount{Path: p, ReadOnly: ro}
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		return errors.New("render: sbxenv.yaml reads back as another document")
	}
	return nil
}
