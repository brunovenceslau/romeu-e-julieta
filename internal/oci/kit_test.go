// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	jsonv2 "encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestArgDeclarationsAreInTheSet(t *testing.T) {
	// A network allow that names an arg: what it reaches depends on the
	// arg's default, so two descriptors that differ only there are two
	// sets (a widening a reviewer must see).
	doc := func(def string) []byte {
		return []byte(minimal + "args:\n  host:\n    default: " + def + "\n" +
			"capabilities:\n  - type: com.docker.sandbox/network-policy@1\n    config:\n      runtime:\n        allow: ['${{ kit.args.host }}']\n")
	}

	narrow, err := ParseDescriptor(doc("a.example"))
	require.NoError(t, err)
	wide, err := ParseDescriptor(doc("evil.example"))
	require.NoError(t, err)

	assert.NotEqual(t, narrow.Capabilities, wide.Capabilities)
	require.Contains(t, wide.Capabilities.Args, "host")
	assert.Equal(t, "evil.example", *wide.Capabilities.Args["host"].Default)
}

// richDescriptors hold every member of the kit schema the grammar reads,
// between them, so the test below reaches each one.
var richDescriptors = []string{`
schemaVersion: "3"
kind: mixin
displayName: n
author: a
description: d
sourceUrl: https://example.com/s
iconUrl: https://example.com/i.png
version: 1.2.3
licenses: [MIT]
provides: [p@1]
requires: [r >= 1]
integrates: [i]
conflicts: [c]
build: FROM scratch
args:
  host:
    default: a.example
    description: shown to the installer
    env: HOST
  mode:
    required: true
    enum: [a, b]
    buildArg: MODE
  name:
    pattern: '^[a-z]+$'
capabilities:
  - name: label
    description: why
    optional: true
    source: {kit: k, path: p}
    type: com.docker.sandbox/port@1
    config: {container: 80, transport: tcp, name: web}
  - group:
      name: g
      description: gd
      optional: true
      capabilities:
        - type: com.docker.sandbox/volume@1
          name: member
          description: md
          source: {kit: k2, path: p2}
          config: {path: /data, size: 1G, tmpfs: false, mode: "0700"}
  - type: com.docker.sandbox/network-policy@2
    config:
      install: {allow: [a.example], deny: [b.example]}
      runtime:
        allow: [{hosts: [c.example], methods: [GET], paths: [/x]}]
  - type: com.docker.sandbox/credential@1
    config:
      service: svc
      phase: [install, runtime]
      apiKey:
        name: KEY
        proxyManaged: true
        inject: [{domain: d.example, header: H, format: "B %s", scheme: basic, username: u}]
  - type: com.docker.sandbox/credential@1
    config:
      service: svc2
      phase: runtime
      oauth:
        tokenEndpoint: {host: h.example, path: /t}
        resourceHosts: [r.example]
        sentinels: {accessToken: A, refreshToken: R}
        credentialFile: {path: /f.json, format: json, structure: {a: {b: c}}}
        responseFields: {accessToken: at, refreshToken: rt, expiresIn: ei}
        passthrough: false
  - type: com.docker.sandbox/ssh-agent@1
    config: {phase: runtime, unrestricted: false, sign: [git], authenticate: [github.com]}
  - type: com.docker.sandbox/lifecycle@1
    config:
      install: [{command: [a, b], user: agent, env: [X], description: hd}]
      startup: [{command: "run", background: true, user: agent, env: [Y], description: sd}]
      files: [{path: /f, content: c, mode: "0600", overwrite: false, description: fd}]
      interactive: [--tty]
  - type: com.docker.sandbox/usb-device@1
    config: {vendorId: "1d6b", productId: "0002"}
  - type: com.docker.sandbox/resources@1
    config: {cpu: 2, memory: 1G, gpu: all}
  - type: com.docker.sandbox/agent-context@1
    config: {filename: F.md, contentFile: c, content: x, directory: /d}
  - type: com.docker.sandbox/agent-skills@1
    config: {path: /skills, mode: readwrite}
  - type: com.docker.sandbox/agent-skill@1
    config: {path: /skill, name: s}
  - type: com.docker.sandbox/agent-sessions@1
    config: {prompt: [p], resume: [r], continue: [c], list: [ls]}
  - type: com.docker.sandbox/privileged@1
  - type: com.docker.sandbox/host-mount@1
    config: {path: /m, mode: "0755"}
`, `
schemaVersion: "3"
kind: set
kits:
  - ref: example.com/org/kit:1
    digest: sha256:` + strings.Repeat("a", 64) + `
    args: {x: y}
`, `
schemaVersion: "3"
kind: workload
dockerfile: kit.dockerfile
capabilities:
  - type: com.docker.sandbox/sbx@1
`}

// provenanceOnly are the members of a descriptor that the set does not
// hold, because they say who made the kit or label what it asks for and
// change nothing it can do: this is the explicit allow list. A member
// that is in neither the set nor this list is a way to widen a kit
// without a reviewer seeing it, and the test below fails on it.
var provenanceOnly = []string{
	"displayName", "author", "description", "sourceUrl", "iconUrl", "licenses",
	"args.*.description",
	"capabilities[].name", "capabilities[].description", "capabilities[].source",
	"capabilities[].group.name", "capabilities[].group.description",
	"capabilities[].group.capabilities[].name", "capabilities[].group.capabilities[].description",
	"capabilities[].group.capabilities[].source",
}

// isProvenance reports whether the path is a provenance-only member or
// below one.
func isProvenance(path string) bool {
	return slices.ContainsFunc(provenanceOnly, func(p string) bool {
		return path == p || strings.HasPrefix(path, p+".") || strings.HasPrefix(path, p+"[]")
	})
}

// leaf is a scalar of a tree and how to change it.
type leaf struct {
	path string // with [] for a list index and * for an arg name
	set  func(root any, v any)
	cut  func(root any) // removes the member, if it is a map member
	val  any
}

// leaves lists the scalars of the tree v.
func leaves(v any, path string, parent func() any, key any) []leaf {
	var out []leaf
	switch x := v.(type) {
	case map[string]any:
		keys := slices.Sorted(maps.Keys(x))
		for _, k := range keys {
			c := x[k]
			p := path
			if p != "" {
				p += "."
			}
			name := k
			if strings.HasPrefix(path, "args") && path == "args" {
				name = "*"
			}
			out = append(out, leaves(c, p+name, func() any { return x }, k)...)
		}
	case []any:
		for i, c := range x {
			out = append(out, leaves(c, path+"[]", func() any { return x }, i)...)
		}
	default:
		l := leaf{path: path, val: x}
		switch p := parent().(type) {
		case map[string]any:
			k := key.(string)
			l.set = func(_ any, nv any) { p[k] = nv }
			l.cut = func(any) { delete(p, k) }
		case []any:
			i := key.(int)
			l.set = func(_ any, nv any) { p[i] = nv }
		}
		out = append(out, l)
	}
	return out
}

// deepCopy returns a copy of a tree of maps and lists.
func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, c := range x {
			m[k] = deepCopy(c)
		}
		return m
	case []any:
		l := make([]any, len(x))
		for i, c := range x {
			l[i] = deepCopy(c)
		}
		return l
	}
	return v
}

// TestEveryMemberOfADescriptorIsInTheSetOrProvenanceOnly changes each
// scalar of descriptors that hold every member, one at a time, and
// removes each map member: the set must change, or the descriptor be
// refused, unless the member is in provenanceOnly, and it must not
// change if it is.
func TestEveryMemberOfADescriptorIsInTheSetOrProvenanceOnly(t *testing.T) {
	for i, text := range richDescriptors {
		var tree map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(text), &tree))
		raw, err := jsonv2.Marshal(tree)
		require.NoError(t, err)
		base, err := ParseAnnotation(raw)
		require.NoError(t, err, "descriptor %d", i)

		var reached int
		for at, l := range leaves(tree, "", func() any { return nil }, nil) {
			allowed := isProvenance(l.path)
			for _, how := range []string{"change", "remove"} {
				if how == "remove" && l.cut == nil {
					continue
				}
				t.Run(fmt.Sprintf("%d/%s/%s", i, how, l.path), func(t *testing.T) {
					work := deepCopy(tree)
					// The same leaf of the copy: the walk is in sorted order.
					target := leaves(work, "", func() any { return nil }, nil)[at]
					require.Equal(t, l.path, target.path)
					if how == "remove" {
						target.cut(work)
					} else {
						switch v := l.val.(type) {
						case string:
							target.set(work, v+"x")
						case bool:
							target.set(work, !v)
						case int:
							target.set(work, v+1)
						default:
							t.Fatalf("a scalar of a kind the test does not change: %T", v)
						}
					}
					raw, err := jsonv2.Marshal(work)
					require.NoError(t, err)
					got, err := ParseAnnotation(raw)
					if err != nil {
						require.False(t, allowed, "the provenance member %s is refused when changed: %v", l.path, err)
						return
					}
					if allowed {
						assert.Equal(t, base.Capabilities, got.Capabilities, "%s is provenance only, so the set must not move", l.path)
						return
					}
					assert.NotEqual(t, base.Capabilities, got.Capabilities, "%s is in neither the set nor the provenance list: a widening a reviewer cannot see", l.path)
				})
			}
			reached++
		}
		assert.Positive(t, reached)
	}
	paths := map[string]bool{}
	for _, text := range richDescriptors {
		var tree map[string]any
		require.NoError(t, yaml.Unmarshal([]byte(text), &tree))
		for _, l := range leaves(tree, "", func() any { return nil }, nil) {
			paths[l.path] = true
		}
	}
	var missing []string
	for _, p := range provenanceOnly {
		if !slices.ContainsFunc(slices.Collect(maps.Keys(paths)), func(path string) bool { return isProvenance(path) && (path == p || strings.HasPrefix(path, p)) }) {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "every entry of the provenance list is reached by a fixture")
}
