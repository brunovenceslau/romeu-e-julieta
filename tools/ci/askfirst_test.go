// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureList is an ask-first list whose owner is the <owner> of
// fixtureModule.
const fixtureList = `version: 1
owner: "@someone-1"
surfaces:
  - id: gates
    globs: [internal/gate/**, internal/spec/project.go]
    reason: the gates | a pipe, a \ and [brackets]
  - id: ask-first
    globs: [.github/ask-first.yaml]
    reason: this list
`

func TestParseAskFirst(t *testing.T) {
	got, err := parseAskFirst([]byte(fixtureList), fixtureModule)
	require.NoError(t, err)
	crlf, err := parseAskFirst([]byte(strings.ReplaceAll(fixtureList, "\n", "\r\n")), fixtureModule)
	require.NoError(t, err)
	assert.Equal(t, got, crlf, "a list with CR LF line ends reads the same")
	assert.Equal(t, askFirst{
		Version: 1,
		Owner:   "@someone-1",
		Surfaces: []surface{
			{ID: "gates", Globs: []string{"internal/gate/**", "internal/spec/project.go"}, Reason: `the gates | a pipe, a \ and [brackets]`},
			{ID: "ask-first", Globs: []string{askFirstPath}, Reason: "this list"},
		},
	}, got)
}

// TestParseAskFirstRefuses covers what the list must not hold. Each
// refusal stops go generate, so no consumer is written from a list
// that its readers could read in two ways.
func TestParseAskFirstRefuses(t *testing.T) {
	self := "  - id: ask-first\n    globs: [.github/ask-first.yaml]\n    reason: this list\n"
	head := "version: 1\nowner: \"@someone-1\"\nsurfaces:\n"
	one := func(id, globs, reason string) string {
		return head + "  - id: " + id + "\n    globs: " + globs + "\n    reason: " + reason + "\n" + self
	}
	tests := []struct {
		name, list, module, want string
	}{
		{"not YAML", "version: [", fixtureModule, "ask-first.yaml"},
		{"an unknown key", head + self + "extra: 1\n", fixtureModule, "field extra not found"},
		{"a key written twice", "version: 1\nversion: 1\nowner: \"@someone-1\"\nsurfaces:\n" + self, fixtureModule, "already defined"},
		{"another version", strings.Replace(head, "version: 1", "version: 2", 1) + self, fixtureModule, "version 2"},
		{"no owner", "version: 1\nsurfaces:\n" + self, fixtureModule, "the owner"},
		{"an owner that is not a handle", strings.Replace(head, "@someone-1", "someone-1", 1) + self, fixtureModule, "the owner"},
		{"an owner other than the module's", strings.Replace(head, "@someone-1", "@someone-2", 1) + self, fixtureModule, `the owner "@someone-2" is not "@someone-1"`},
		{"a module outside github.com", head + self, "example.invalid/m", "github.com/<owner>/"},
		{"no surface", head, fixtureModule, "lists itself"},
		{"a list that does not list itself", head + "  - id: gates\n    globs: [internal/gate/**]\n    reason: the gates\n", fixtureModule, "lists itself"},
		{"an empty id", one(`""`, "[a]", "r"), fixtureModule, `surface 1: the id "" is not`},
		{"an id with a capital", one("Gates", "[a]", "r"), fixtureModule, `the id "Gates" is not`},
		{"an id written twice", one("ask-first", "[a]", "r"), fixtureModule, `the id "ask-first" is used twice`},
		{"no glob", one("gates", "[]", "r"), fixtureModule, "surface gates: no glob"},
		{"no reason", one("gates", "[a]", `""`), fixtureModule, "surface gates: no reason"},
		{"a reason of spaces", one("gates", "[a]", `"   "`), fixtureModule, "surface gates: no reason"},
		{"an owner that is not a handle, though the module names it", strings.Replace(head, "@someone-1", "@-x", 1) + self, "github.com/-x/y", `the owner "@-x" is not a GitHub handle`},
		{"a glob two surfaces share", one("gates", "[.github/ask-first.yaml]", "r"), fixtureModule, `surface ask-first: glob ".github/ask-first.yaml" is a glob of gates too`},
		{"a reason on two lines", one("gates", "[a]", `"one\ntwo"`), fixtureModule, "surface gates: the reason is one line"},
		{"an absolute glob", one("gates", "[/a]", "r"), fixtureModule, `glob "/a"`},
		{"a glob with an empty segment", one("gates", "[a//b]", "r"), fixtureModule, `glob "a//b"`},
		{"a glob that ends in a slash", one("gates", "[a/]", "r"), fixtureModule, `glob "a/"`},
		{"a dot segment", one("gates", "[a/./b]", "r"), fixtureModule, `glob "a/./b"`},
		{"a dot-dot segment", one("gates", "[../a]", "r"), fixtureModule, `glob "../a"`},
		{"a double star inside a segment", one("gates", "[a/b**]", "r"), fixtureModule, `glob "a/b**"`},
		{"a question mark", one("gates", "[\"a?\"]", "r"), fixtureModule, `glob "a?"`},
		{"a bracket", one("gates", "[\"a[b]\"]", "r"), fixtureModule, `glob "a[b]"`},
		{"a space", one("gates", "[\"a b\"]", "r"), fixtureModule, `glob "a b"`},
		{"a negation", one("gates", "[\"!a\"]", "r"), fixtureModule, `glob "!a"`},
		{"a hash", one("gates", "[\"#a\"]", "r"), fixtureModule, `glob "#a"`},
		{"a backslash", one("gates", `["a\\b"]`, "r"), fixtureModule, `glob "a\\b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAskFirst([]byte(tt.list), tt.module)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestParseAskFirstGlobs lists the globs the list takes: the shapes 12
// 12.4 defines, written in the characters that mean the same thing to
// its matcher and to CODEOWNERS.
func TestParseAskFirstGlobs(t *testing.T) {
	for _, glob := range []string{"go.mod", "a/b/**", "**/x.go", "a/**/b", "*.go", "a/*_test.go", ".github/workflows/**", "a-b_c.d/e"} {
		t.Run(glob, func(t *testing.T) {
			list := "version: 1\nowner: \"@someone-1\"\nsurfaces:\n  - id: ask-first\n    globs: [.github/ask-first.yaml, \"" + glob + "\"]\n    reason: r\n"
			_, err := parseAskFirst([]byte(list), fixtureModule)
			assert.NoError(t, err)
		})
	}
}

// TestAskFirstList reads the committed list against 05 5.3: it parses,
// and it holds each surface of the spec's block with the spec's globs,
// in the spec's order.
func TestAskFirstList(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(askFirstPath)))
	require.NoError(t, err)
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	module, err := modulePath(goMod)
	require.NoError(t, err)
	list, err := parseAskFirst(data, module)
	require.NoError(t, err)

	spec, err := os.ReadFile(filepath.Join(root, "docs", "spec", "05-security.md"))
	require.NoError(t, err)
	_, block, ok := strings.Cut(string(spec), "## 5.3 Ask-first surfaces")
	require.True(t, ok, "05 has section 5.3")
	_, block, ok = strings.Cut(block, "```yaml\n")
	require.True(t, ok, "5.3 has a yaml block")
	block, _, ok = strings.Cut(block, "```")
	require.True(t, ok, "the yaml block ends")
	want, err := parseAskFirst([]byte(strings.Replace(block, `"@<owner>"`, `"`+list.Owner+`"`, 1)), module)
	require.NoError(t, err, "the block of 05 5.3")
	assert.Equal(t, want, list, "the committed list is the block of 05 5.3")
}

// TestCodeowners pins the generated CODEOWNERS: each glob anchored at
// the top of the repository, so that a glob without a slash, such as
// go.mod, names the root file only, as it does for the matcher of 12
// 12.4, and not a file of that name at any depth.
func TestCodeowners(t *testing.T) {
	list, err := parseAskFirst([]byte(fixtureList), fixtureModule)
	require.NoError(t, err)
	assert.Equal(t, "# "+generatedFrom(askFirstPath)+"\n"+
		"#\n"+
		"# Each path of an ask-first surface (05 5.3) asks for the review of\n"+
		"# its owner.\n"+
		"\n"+
		"# gates\n"+
		"/internal/gate/** @someone-1\n"+
		"/internal/spec/project.go @someone-1\n"+
		"\n"+
		"# ask-first\n"+
		"/.github/ask-first.yaml @someone-1\n", string(codeowners(list)))
}

// TestAskFirstPage pins the generated reference page: its front matter
// (ADR 0001, rules 1 and 10), its header, and a table row per surface
// whose cells keep a pipe of the reason inside the cell.
func TestAskFirstPage(t *testing.T) {
	list, err := parseAskFirst([]byte(fixtureList), fixtureModule)
	require.NoError(t, err)
	page := string(askFirstPage(list))
	assert.True(t, strings.HasPrefix(page, "---\ntype: reference\nreader: "), "front matter first")
	assert.Contains(t, page, "\n<!-- "+generatedFrom(askFirstPath)+" -->\n")
	assert.Contains(t, page, "| `gates` | `internal/gate/**`, `internal/spec/project.go` | the gates \\| a pipe, a \\\\ and \\[brackets\\] |\n")
	assert.Contains(t, page, "| `ask-first` | `.github/ask-first.yaml` | this list |\n")
	assert.Contains(t, page, "`@someone-1`")
}

// TestMiseFilesAskFirst holds miseFiles and the dependencies surface of
// the ask-first list to one list: the surface's globs, without go.mod,
// go.sum and the workflows, are miseFiles, in order. So a file mise
// reads cannot be refused by hygiene and left off the list, or the
// reverse.
func TestMiseFilesAskFirst(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(askFirstPath)))
	require.NoError(t, err)
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	module, err := modulePath(goMod)
	require.NoError(t, err)
	list, err := parseAskFirst(data, module)
	require.NoError(t, err)
	i := slices.IndexFunc(list.Surfaces, func(s surface) bool { return s.ID == "dependencies" })
	require.GreaterOrEqual(t, i, 0, "the dependencies surface")
	got := slices.DeleteFunc(slices.Clone(list.Surfaces[i].Globs), func(g string) bool {
		return g == "go.mod" || g == "go.sum" || g == ".github/workflows/**"
	})
	assert.Equal(t, miseFiles, got)
}

// TestGlobMatch pins the matcher of 12 12.4 that isMiseFile reads with:
// "**" is zero or more whole segments, "*" stays inside one, and a
// glob is anchored at the top of the tree.
func TestGlobMatch(t *testing.T) {
	tests := []struct {
		glob, name string
		want       bool
	}{
		{"go.mod", "go.mod", true},
		{"go.mod", "tools/go.mod", false},
		{"mise.*.toml", "mise.local.toml", true},
		{"mise.*.toml", "mise.toml", false},
		{"mise.*.toml", "tools/mise.local.toml", false},
		{".config/mise/**", ".config/mise/conf.d/x.toml", true},
		{".config/mise/**", ".config/mise", true},
		{".config/mise/**", ".config/mise.toml", false},
		{"a/**/b", "a/b", true},
		{"a/**/b", "a/x/y/b", true},
		{"a/**/b", "a/x/y/c", false},
		{"a/*/b", "a/x/y/b", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, globMatch(tt.glob, tt.name), "%s ~ %s", tt.glob, tt.name)
	}
}
