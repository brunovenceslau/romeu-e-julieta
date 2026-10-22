// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package catalog_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
)

// head opens every fixture file.
const head = "schema: catalog/v1\n"

// family is a family file that holds one tool and the domains it needs.
const family = `schema: catalog/v1
domains:
  nodejs.org: {upload: false}
  registry.npmjs.org: {upload: true}
tools:
  core:node: [nodejs.org, registry.npmjs.org]
  aqua:jqlang/jq: []
`

// TestDecode reads a family file strictly (REQ-0212, ADR 0041: the
// catalog is a file a person writes, so an unknown key is refused).
func TestDecode(t *testing.T) {
	f, err := catalog.Decode([]byte(family))
	require.NoError(t, err)
	assert.Equal(t, []string{"nodejs.org", "registry.npmjs.org"}, f.Tools["core:node"])
	assert.NotNil(t, f.Tools["aqua:jqlang/jq"], "an empty list is a tool that needs no domain")
	require.NotNil(t, f.Domains["registry.npmjs.org"].Upload)
	assert.True(t, *f.Domains["registry.npmjs.org"].Upload)
}

func TestDecode_refuses(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"unknown key", head + "domain: {}\n", "a key the format does not have"},
		{"unknown key in an entry", head + "domains:\n  a.example: {upload: true, note: x}\n", "a key the format does not have"},
		{"duplicate domain", head + "domains:\n  a.example: {upload: true}\n  a.example: {upload: false}\n", "a key written twice"},
		{"duplicate tool", head + "tools:\n  core:go: []\n  core:go: []\n", "a key written twice"},
		{"no schema", "domains: {}\n", "schema"},
		{"other schema", "schema: catalog/v2\n", "schema"},
		{"no upload flag", head + "domains:\n  a.example: {}\n", "upload"},
		{"null upload flag", head + "domains:\n  a.example: {upload: null}\n", "null"},
		{"upload not a boolean", head + "domains:\n  a.example: {upload: maybe}\n", "upload"},
		{"null tool", head + "tools:\n  core:go:\n", "null"},
		{"null meta", head + "meta:\n", "null"},
		{"null domains", head + "domains: ~\n", "null"},
		{"yes for upload", head + "domains:\n  a.example: {upload: yes}\n", "upload"},
		{"on for upload", head + "domains:\n  a.example: {upload: on}\n", "upload"},
		{"True for upload", head + "domains:\n  a.example: {upload: True}\n", "upload"},
		{"quoted true for upload", head + "domains:\n  a.example: {upload: 'true'}\n", "upload"},
		{"an anchor", head + "meta: &m [a.example]\n", "anchor"},
		{"an anchor and an alias", head + "meta: &m [a.example]\nbase: *m\n", "anchor"},
		{"a merge key", head + "domains:\n  a.example: &e {upload: true}\n  b.example:\n    <<: *e\n", "anchor"},
		{"a merge key alone", head + "domains:\n  a.example:\n    <<: {upload: true}\n", "merge"},
		{"a custom tag", head + "meta: [!foo a.example]\n", "tag"},
		{"not one mapping", "- a\n", "mapping"},
		{"two documents", head + "---\n" + head, "more than one document"},
		{"not UTF-8", head + "meta: [\xff]\n", "UTF-8"},
		{"one byte over the cap", head + strings.Repeat("#", catalog.MaxFileSize-len(head)+1), "cap"},
		{"domain with a wildcard", head + "domains:\n  '*.example.com': {upload: false}\n", "domain"},
		{"domain with a port", head + "meta: ['a.example:443']\n", "domain"},
		{"domain in base", head + "base: [Example.COM]\n", "domain"},
		{"domain of a tool", head + "tools:\n  core:go: [10.0.0.1]\n", "domain"},
		{"key without a backend", head + "tools:\n  go: []\n", "backend:tool"},
		{"key with an empty tool", head + "tools:\n  'aqua:': []\n", "backend:tool"},
		{"key with a space", head + "tools:\n  'aqua:a b': []\n", "backend:tool"},
		{"key with a capital backend", head + "tools:\n  'Aqua:cli/cli': []\n", "backend:tool"},
		{"key with a second colon", head + "tools:\n  'aqua:a:b': []\n", "backend:tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := catalog.Decode([]byte(tt.doc))
			require.ErrorIs(t, err, catalog.ErrInvalid)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestDecode_cap: a file of exactly MaxFileSize bytes is read, one more
// byte is refused.
func TestDecode_cap(t *testing.T) {
	exact := head + strings.Repeat("#", catalog.MaxFileSize-len(head))
	require.Len(t, exact, catalog.MaxFileSize)
	_, err := catalog.Decode([]byte(exact))
	require.NoError(t, err)
	_, err = catalog.Decode([]byte(exact + "#"))
	require.ErrorIs(t, err, catalog.ErrInvalid)
}

func TestValidTool(t *testing.T) {
	for _, k := range []string{"core:go", "aqua:jqlang/jq", "go:golang.org/x/tools/gopls", "npm:@angular/cli", "pipx:black"} {
		require.NoError(t, catalog.ValidTool(k), k)
	}
	for _, k := range []string{"", "go", ":go", "core:", "core:g o", "core:go\n", "core:go:1"} {
		assert.Error(t, catalog.ValidTool(k), k)
	}
}

const base = `schema: catalog/v1
meta: [github.com]
base: [github.com, codeload.github.com]
domains:
  github.com: {upload: true}
  codeload.github.com: {upload: false}
`

func source(name, doc string) catalog.Source { return catalog.Source{Name: name, Data: []byte(doc)} }

// TestBuild merges the files of the embedded directory: the lookups
// read the union, and the base set lives in base.yaml alone.
func TestBuild(t *testing.T) {
	c, err := catalog.Build([]catalog.Source{source("node.yaml", family), source("base.yaml", base)})
	require.NoError(t, err)
	assert.Equal(t, []string{"github.com"}, c.Meta())
	assert.Equal(t, []string{"codeload.github.com", "github.com"}, c.Base())
	got, ok := c.Tool("core:node")
	assert.True(t, ok)
	assert.Equal(t, []string{"nodejs.org", "registry.npmjs.org"}, got)
	got, ok = c.Tool("aqua:jqlang/jq")
	assert.True(t, ok)
	assert.Empty(t, got)
	_, ok = c.Tool("core:go")
	assert.False(t, ok)
	assert.True(t, c.Gated("github.com"), "an upload domain is gated")
	assert.False(t, c.Gated("nodejs.org"), "a declared domain without upload is not")
	assert.True(t, c.Gated("unknown.example"), "a domain without an entry is gated (07 7.5, step 7)")
	assert.Equal(t, []string{"aqua:jqlang/jq", "core:node"}, c.Tools())
	assert.Empty(t, c.Undeclared())
}

// TestBuild_conflict is the decode error of 03 3.5: a domain two files
// declare with different entries.
func TestBuild_conflict(t *testing.T) {
	other := "schema: catalog/v1\ndomains:\n  nodejs.org: {upload: true}\n"
	_, err := catalog.Build([]catalog.Source{source("base.yaml", base), source("node.yaml", family), source("other.yaml", other)})
	require.ErrorIs(t, err, catalog.ErrInvalid)
	assert.Contains(t, err.Error(), "nodejs.org")
	assert.Contains(t, err.Error(), "node.yaml")
	assert.Contains(t, err.Error(), "other.yaml")

	same := "schema: catalog/v1\ndomains:\n  nodejs.org: {upload: false}\n"
	_, err = catalog.Build([]catalog.Source{source("base.yaml", base), source("node.yaml", family), source("same.yaml", same)})
	assert.NoError(t, err, "the same entry twice is no conflict")
}

func TestBuild_toolConflict(t *testing.T) {
	other := "schema: catalog/v1\ntools:\n  core:node: [nodejs.org]\n"
	_, err := catalog.Build([]catalog.Source{source("base.yaml", base), source("node.yaml", family), source("other.yaml", other)})
	require.ErrorIs(t, err, catalog.ErrInvalid)
	assert.Contains(t, err.Error(), "core:node")

	same := "schema: catalog/v1\ntools:\n  core:node: [registry.npmjs.org, nodejs.org]\n"
	_, err = catalog.Build([]catalog.Source{source("base.yaml", base), source("node.yaml", family), source("same.yaml", same)})
	assert.NoError(t, err, "the same domains in another order are the same entry")
}

func TestBuild_baseFile(t *testing.T) {
	_, err := catalog.Build([]catalog.Source{source("node.yaml", family)})
	require.ErrorIs(t, err, catalog.ErrInvalid)
	assert.Contains(t, err.Error(), "base.yaml")

	for _, set := range []string{"meta", "base"} {
		stray := "schema: catalog/v1\n" + set + ": [x.example]\n"
		_, err = catalog.Build([]catalog.Source{source("base.yaml", base), source("node.yaml", stray)})
		require.ErrorIs(t, err, catalog.ErrInvalid, set)
		assert.Contains(t, err.Error(), "node.yaml", set)
		assert.Contains(t, err.Error(), "meta and base belong in base.yaml", set)
	}
}

// TestBuild_dedupe: a domain listed twice is one in the sets.
func TestBuild_dedupe(t *testing.T) {
	doc := "schema: catalog/v1\nmeta: [a.example, a.example]\nbase: [b.example, b.example]\ndomains:\n  a.example: {upload: false}\n  b.example: {upload: false}\ntools:\n  core:go: [a.example, a.example]\n"
	c, err := catalog.Build([]catalog.Source{source("base.yaml", doc)})
	require.NoError(t, err)
	assert.Equal(t, []string{"a.example"}, c.Meta())
	assert.Equal(t, []string{"b.example"}, c.Base())
	got, _ := c.Tool("core:go")
	assert.Equal(t, []string{"a.example"}, got)
}

func TestBuild_namesTheFileOfADecodeError(t *testing.T) {
	_, err := catalog.Build([]catalog.Source{source("base.yaml", base), source("bad.yaml", "schema: catalog/v1\nnope: 1\n")})
	require.ErrorIs(t, err, catalog.ErrInvalid)
	assert.Contains(t, err.Error(), "bad.yaml")
}

// TestCatalog_Undeclared is the data REQ-0212 and REQ-0641 check: every
// domain used by meta, base or a tool needs a domains entry.
func TestCatalog_Undeclared(t *testing.T) {
	use := "schema: catalog/v1\nmeta: [m.example]\nbase: [b.example]\ntools:\n  core:go: [t.example, github.com]\n"
	c, err := catalog.Build([]catalog.Source{source("base.yaml", use), source("go.yaml", "schema: catalog/v1\ndomains:\n  github.com: {upload: true}\n")})
	require.NoError(t, err)
	assert.Equal(t, []string{"b.example", "m.example", "t.example"}, c.Undeclared())
}

// TestEmbedded reads the catalog the binaries embed (REQ-0638, REQ-0129):
// the examples of 03 3.5, with nothing undeclared.
func TestEmbedded(t *testing.T) {
	c, err := catalog.Embedded()
	require.NoError(t, err)
	assert.Equal(t, []string{"api.github.com", "github.com", "mise-versions.jdx.dev", "objects.githubusercontent.com", "tuf-repo-cdn.sigstore.dev"}, c.Meta())
	assert.Equal(t, []string{"api.github.com", "codeload.github.com", "github.com"}, c.Base())
	for tool, want := range map[string][]string{
		"core:go":      {"dl.google.com", "proxy.golang.org", "storage.googleapis.com", "sum.golang.org"},
		"core:node":    {"nodejs.org", "registry.npmjs.org"},
		"core:python":  {"files.pythonhosted.org", "pypi.org"},
		"aqua:cli/cli": {},
	} {
		got, ok := c.Tool(tool)
		assert.True(t, ok, tool)
		assert.ElementsMatch(t, want, got, tool)
	}
	for domain, want := range map[string]bool{
		"github.com": true, "api.github.com": true, "storage.googleapis.com": true, "registry.npmjs.org": true,
		"dl.google.com": false, "proxy.golang.org": false, "tuf-repo-cdn.sigstore.dev": false, "nodejs.org": false, "pypi.org": false, "mise-versions.jdx.dev": false,
	} {
		up, declared := c.UploadFlag(domain)
		assert.True(t, declared, domain)
		assert.Equal(t, want, up, domain)
		assert.Equal(t, want, c.Gated(domain), domain)
	}
	assert.Empty(t, c.Undeclared())
}
