// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// The lines that open a v3 kit descriptor under examples/: the frontend
// pin, then the modeline of the official kit schema at the vendored tag.
const (
	kitSyntax = "# syntax=docker/sandbox-kit:3.0.0-m.8@sha256:e6a397771e865625047cf84256a914c7974b3e749d858259b5bc9c653ddb6f0d\n"
	kitHint   = "# yaml-language-server: $schema=https://raw.githubusercontent.com/docker/sandbox-kit-spec/v3.0.0-m.8/schema/kit.schema.json\n"
	kitPins   = "frontend:\n  ref: docker/sandbox-kit:3.0.0-m.8@sha256:e6a397771e865625047cf84256a914c7974b3e749d858259b5bc9c653ddb6f0d\n"
	kitBody   = "schemaVersion: \"3\"\nkind: mixin\ndisplayName: k\ndescription: a kit\n"
)

// TestKitDescriptors judges the v3 kit descriptors under examples/: each
// carries the official schema's modeline at the vendored tag on its
// second line and validates against the vendored kit schema; a product
// kit under kits/ is not judged.
func TestKitDescriptors(t *testing.T) {
	good := kitSyntax + kitHint + kitBody
	m := withKit(t, tree(t))
	m["examples/kits/k/k.yaml"] = good
	m["kits/k/k.yaml"] = kitSyntax + "kind: nothing\n"
	got, err := Of(files(m), fixture)
	require.NoError(t, err)
	assert.Empty(t, got)

	for _, c := range []struct {
		name, path, content, rule, msg string
	}{
		{"no modeline", "examples/kits/k/k.yaml", kitSyntax + kitBody, "schema hint",
			"is a v3 kit descriptor, and its second line is not \"" + strings.TrimSuffix(kitHint, "\n") + "\" (10 10.2, REQ-0836)"},
		{"a modeline of another tag", "examples/kits/k/k.yaml", kitSyntax + strings.Replace(kitHint, "m.8", "m.7", 1) + kitBody, "schema hint",
			"is a v3 kit descriptor, and its second line is not \"" + strings.TrimSuffix(kitHint, "\n") + "\" (10 10.2, REQ-0836)"},
		{"a file of the two lines only", "examples/kits/k/k.yaml", kitSyntax + strings.TrimSuffix(kitHint, "\n"), "schema parse",
			"does not decode as one YAML document, or repeats a key"},
		{"an unknown key", "examples/kits/k/k.yaml", good + "extra: 1\n", "schema invalid",
			`does not validate against kit.schema.json at v3.0.0-m.8 at "/ (additionalProperties)"`},
		{"a missing member", "examples/kits/k/k.yaml", kitSyntax + kitHint + "kind: mixin\n", "schema invalid",
			`does not validate against kit.schema.json at v3.0.0-m.8 at "/ (required)"`},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			m := withKit(t, tree(t))
			m[c.path] = c.content
			got, err := Of(files(m), fixture)
			require.NoError(t, err)
			assert.Equal(t, []check.Finding{{Where: c.path, Rule: c.rule, Msg: c.msg}}, got)
		})
	}
}

// withKit returns m with the copy of the kit schema and a kits/pins.yaml
// whose frontend ref the descriptors' first line follows.
func withKit(t *testing.T, m map[string]string) map[string]string {
	t.Helper()
	m = withKitSchema(t, m)
	m[pinsFile] = kitPins
	return m
}

// withKitSchema returns m with the copy of the kit schema of this
// repository at its path, as a commit's tree holds it.
func withKitSchema(t *testing.T, m map[string]string) map[string]string {
	t.Helper()
	root := citest.ModuleRoot(t)
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(kitSpecRoot)), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		m[filepath.ToSlash(rel)] = string(data)
		return err
	})
	require.NoError(t, err)
	return m
}

// TestKitPin holds the first line of a descriptor and kitSpecTag to the
// frontend ref of kits/pins.yaml (REQ-0117).
func TestKitPin(t *testing.T) {
	const path = "examples/kits/k/k.yaml"
	first := strings.TrimSuffix(kitSyntax, "\n")
	pinsAt := func(ref string) string { return "frontend:\n  ref: " + ref + "\n" }
	for _, c := range []struct {
		name, pins string
		want       []check.Finding
	}{
		{"the pin and the tag agree", kitPins, nil},
		{"another digest", strings.Replace(kitPins, "e6a3", "e6a4", 1), []check.Finding{{Where: path, Rule: "schema pin",
			Msg: "is a v3 kit descriptor, and its first line is not \"" + strings.Replace(first, "e6a3", "e6a4", 1) + "\", the frontend ref of kits/pins.yaml (02 2.1)"}}},
		{"another tag", pinsAt("docker/sandbox-kit:3.0.0-m.9@sha256:aa"), []check.Finding{
			{Where: path, Rule: "schema pin", Msg: "is a v3 kit descriptor, and its first line is not \"# syntax=docker/sandbox-kit:3.0.0-m.9@sha256:aa\", the frontend ref of kits/pins.yaml (02 2.1)"},
			{Where: "kits/pins.yaml", Rule: "schema pin", Msg: "pins the frontend at tag \"3.0.0-m.9\", and the vendored kit schema is v3.0.0-m.8 (kitSpecTag, tools/ci/internal/schema/kits.go); copy the schema of the pinned tag and change kitSpecTag together"},
		}},
		{"no frontend ref", "workload:\n  ref: x\n", []check.Finding{{Where: "kits/pins.yaml", Rule: "schema pin",
			Msg: "kits/pins.yaml holds no frontend ref, so the vendored kit schema at v3.0.0-m.8 follows no pin"}}},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			m := withKit(t, tree(t))
			m["kits/pins.yaml"] = c.pins
			m[path] = kitSyntax + kitHint + kitBody
			got, err := Of(files(m), fixture)
			require.NoError(t, err)
			assert.ElementsMatch(t, c.want, got)
		})
	}
}

// TestKitSchemaCopyIsPinned holds the vendored copy to the digests of
// kitSpecDigests: a file edited, added or removed is a finding.
func TestKitSchemaCopyIsPinned(t *testing.T) {
	const port = kitSpecRoot + "/capabilities/com.docker.sandbox/port@1.schema.json"
	for _, c := range []struct {
		name   string
		change func(m map[string]string)
		want   []check.Finding
	}{
		{"an edited file", func(m map[string]string) { m[port] = strings.Replace(m[port], "\"type\"", "\"type\" ", 1) },
			[]check.Finding{{Where: port, Rule: "schema copy", Msg: "differs from the digest pinned for it in kits_digests.go: the copy of the kit schema at v3.0.0-m.8 changes only with its pin"}}},
		{"an added file", func(m map[string]string) { m[kitSpecRoot+"/extra.schema.json"] = "{}" },
			[]check.Finding{{Where: kitSpecRoot + "/extra.schema.json", Rule: "schema copy", Msg: "is no file of the copy of the kit schema at v3.0.0-m.8 that kits_digests.go pins"}}},
		{"a removed file", func(m map[string]string) { delete(m, port) },
			[]check.Finding{{Where: port, Rule: "schema copy", Msg: "is pinned in kits_digests.go and is not in the tree"}}},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			m := withKit(t, tree(t))
			c.change(m)
			got, err := Of(files(m), fixture)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestWithin stops waiting for work that does not return: a hostile
// schema or document that loops fails closed, in time. The library
// takes no context, and the vendored schema is pinned by digest, so no
// pathological schema is fed to it here; the timer is what is tested.
func TestWithin(t *testing.T) {
	got, ok := within(time.Minute, func() int { return 7 })
	assert.True(t, ok)
	assert.Equal(t, 7, got)

	stuck := make(chan struct{})
	defer close(stuck)
	start := time.Now()
	got, ok = within(10*time.Millisecond, func() int { <-stuck; return 7 })
	assert.False(t, ok)
	assert.Zero(t, got)
	assert.Less(t, time.Since(start), 5*time.Second)
}

// TestAKitWithoutTheSchemaCopy fails a kit descriptor of a tree that
// holds no copy of the kit schema to validate it against.
func TestAKitWithoutTheSchemaCopy(t *testing.T) {
	m := tree(t)
	m["examples/kits/k/k.yaml"] = kitSyntax + kitHint + kitBody
	got, err := Of(files(m), fixture)
	require.NoError(t, err)
	assert.Equal(t, []check.Finding{{Where: "examples/kits/k/k.yaml", Rule: ruleUnknown,
		Msg: "is a v3 kit descriptor, and the tree holds no copy of the kit schema at " + kitSpecRoot}}, got)
}

// TestTheKitSchemaIsTheTagsCopy pins the copy of the kit schema in this
// repository: the 21 files of schema/ at v3.0.0-m.8, the top one a
// draft-07 document, so a partial copy or another draft fails here.
func TestTheKitSchemaIsTheTagsCopy(t *testing.T) {
	m := withKitSchema(t, map[string]string{})
	var names []string
	for p := range m {
		names = append(names, strings.TrimPrefix(p, kitSpecRoot+"/"))
	}
	assert.Len(t, names, 21)
	assert.ElementsMatch(t, slices.Collect(maps.Keys(kitSpecDigests)), names)
	assert.Contains(t, names, "kit.schema.json")
	assert.Contains(t, names, "definitions/kit-arg.schema.json")
	assert.Contains(t, m[kitSpecRoot+"/kit.schema.json"], `"$schema": "http://json-schema.org/draft-07/schema#"`)
	sch, err := compileKit(files(m))
	require.NoError(t, err)
	assert.NotNil(t, sch)
}
