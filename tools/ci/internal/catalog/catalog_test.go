// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package catalog_test

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// tree is a fixture tree: path to content.
type tree map[string]string

func (tr tree) files() []check.TreeFile {
	var files []check.TreeFile
	for p, c := range tr {
		files = append(files, check.TreeFile{Path: p, Mode: "100644", Content: []byte(c)})
	}
	return files
}

const head = "schema: catalog/v1\n"

// good is a small catalog that passes: the base file and one family.
func good() tree {
	return tree{
		catalog.Dir + "base.yaml": head + "meta: [github.com]\nbase: [github.com]\ndomains:\n  github.com: {upload: true}\n",
		catalog.Dir + "go.yaml":   head + "domains:\n  proxy.golang.org: {upload: false}\ntools:\n  core:go: [proxy.golang.org]\n  aqua:cli/cli: []\n",
	}
}

func messages(findings []check.Finding) string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Where+": "+f.Msg)
	}
	return strings.Join(out, "\n")
}

// TestCatalogRepositoryPasses runs the rules on the catalog files of this
// repository, which every commit keeps passing.
func TestCatalogRepositoryPasses(t *testing.T) {
	root := citest.ModuleRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "internal", "catalog", "egress"))
	require.NoError(t, err)
	tr := tree{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(root, "internal", "catalog", "egress", e.Name()))
		require.NoError(t, err)
		tr[catalog.Dir+e.Name()] = string(b)
	}
	require.NotEmpty(t, tr)
	assert.Empty(t, messages(catalog.Of(tr.files())))
}

func TestCatalogGoodTreePasses(t *testing.T) {
	assert.Empty(t, messages(catalog.Of(good().files())))
}

// TestCatalogNoDirectoryNothingToJudge: a tree without the catalog
// directory is no catalog, as a tree without kits has none to judge.
func TestCatalogNoDirectoryNothingToJudge(t *testing.T) {
	assert.Empty(t, catalog.Of(tree{"README.md": "x"}.files()))
}

// TestCatalogPackageWithoutFiles: the package is there and the embed
// pattern matches nothing, which is a finding and not "no catalog".
func TestCatalogPackageWithoutFiles(t *testing.T) {
	got := catalog.Of(tree{"internal/catalog/catalog.go": "package catalog\n"}.files())
	require.Len(t, got, 1)
	assert.Contains(t, messages(got), catalog.Dir)
	assert.Equal(t, "catalog", got[0].Rule)
}

// TestCatalogDirIsTheEmbedPattern ties Dir and the extension the check
// reads to the pattern internal/catalog embeds, so a move of the pattern
// fails here and not silently in the check.
func TestCatalogDirIsTheEmbedPattern(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "internal", "catalog", "catalog.go"))
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^//go:embed (\S+)$`).FindAllStringSubmatch(string(b), -1)
	require.Len(t, m, 1, "one embed directive")
	pattern := m[0][1]
	assert.Equal(t, catalog.Dir, "internal/catalog/"+path.Dir(pattern)+"/")
	assert.Equal(t, catalog.Ext, path.Ext(pattern))
}

// TestCatalogOneFindingPerFault: each fault is one finding, so a run
// that has two is not read as one.
func TestCatalogOneFindingPerFault(t *testing.T) {
	tr := good()
	tr[catalog.Dir+"go.yaml"] = head + "tools:\n  core:go: [a.example, b.example]\n"
	assert.Len(t, catalog.Of(tr.files()), 2, "two undeclared domains")

	tr = good()
	tr[catalog.Dir+"node.yaml"] = head + "nope: 1\n"
	tr[catalog.Dir+"python.yaml"] = head + "nada: 1\n"
	assert.Len(t, catalog.Of(tr.files()), 2, "two files that do not decode")

	tr = good()
	tr[catalog.Dir+"node.yml"] = head
	tr[catalog.Dir+"more/node.yaml"] = head
	assert.Len(t, catalog.Of(tr.files()), 2, "two files the glob does not embed")
}

// TestCatalogRefuses are the fixture lines: one change to the good
// tree, and the finding it draws (REQ-0212, REQ-0641, REQ-0842).
func TestCatalogRefuses(t *testing.T) {
	tests := []struct {
		name string
		edit func(tree)
		want string
	}{
		{"a tool domain without an upload entry", func(tr tree) {
			tr[catalog.Dir+"go.yaml"] = head + "tools:\n  core:go: [proxy.golang.org, sum.golang.org]\n"
		}, "sum.golang.org"},
		{"a meta domain without an upload entry", func(tr tree) {
			tr[catalog.Dir+"base.yaml"] = head + "meta: [github.com, mise.example.org]\nbase: [github.com]\ndomains:\n  github.com: {upload: true}\n"
		}, "mise.example.org"},
		{"a base domain without an upload entry", func(tr tree) {
			tr[catalog.Dir+"base.yaml"] = head + "meta: [github.com]\nbase: [github.com, git.example.org]\ndomains:\n  github.com: {upload: true}\n"
		}, "git.example.org"},
		{"an entry without the flag", func(tr tree) {
			tr[catalog.Dir+"go.yaml"] = head + "domains:\n  proxy.golang.org: {}\ntools:\n  core:go: [proxy.golang.org]\n"
		}, "upload"},
		{"a malformed tool key", func(tr tree) {
			tr[catalog.Dir+"go.yaml"] = head + "domains:\n  proxy.golang.org: {upload: false}\ntools:\n  golang: [proxy.golang.org]\n"
		}, "backend:tool"},
		{"two files with different entries for a domain", func(tr tree) {
			tr[catalog.Dir+"node.yaml"] = head + "domains:\n  proxy.golang.org: {upload: true}\n"
		}, "proxy.golang.org"},
		{"an unknown key", func(tr tree) {
			tr[catalog.Dir+"node.yaml"] = head + "domain: {}\n"
		}, "node.yaml"},
		{"a domain that is not one", func(tr tree) {
			tr[catalog.Dir+"node.yaml"] = head + "domains:\n  '*.nodejs.org': {upload: false}\n"
		}, "domain"},
		{"a file the glob does not embed", func(tr tree) {
			tr[catalog.Dir+"node.yml"] = head
		}, "node.yml"},
		{"a subdirectory", func(tr tree) {
			tr[catalog.Dir+"more/node.yaml"] = head
		}, "more/node.yaml"},
		{"no base file", func(tr tree) {
			delete(tr, catalog.Dir+"base.yaml")
		}, "base.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := good()
			tt.edit(tr)
			got := catalog.Of(tr.files())
			require.NotEmpty(t, got)
			assert.Contains(t, messages(got), tt.want)
			for _, f := range got {
				assert.Equal(t, "catalog", f.Rule)
			}
		})
	}
}

// TestCatalogRun is the subcommand: it judges the committed tree.
func TestCatalogRun(t *testing.T) {
	r := citest.NewTree(t)
	for p, c := range good() {
		r.Write(p, c)
	}
	r.Commit("a good catalog")
	code, out := citest.Run(t, r, nil, catalog.Run)
	assert.Equal(t, check.ExitOK, code, out)
	assert.Contains(t, out, "ok")

	r.Write(catalog.Dir+"go.yaml", head+"tools:\n  core:go: [sum.golang.org]\n")
	code, out = citest.Run(t, r, nil, catalog.Run)
	assert.Equal(t, check.ExitOK, code, "the working tree is not the commit: "+out)
	r.Commit("an undeclared domain")
	code, out = citest.Run(t, r, nil, catalog.Run)
	assert.NotEqual(t, check.ExitOK, code, out)
	assert.Contains(t, out, "sum.golang.org")

	code, out = citest.Run(t, r, nil, catalog.Run, "extra")
	assert.NotEqual(t, check.ExitOK, code)
	assert.Contains(t, out, "takes no argument")
}

func TestCatalogJudge(t *testing.T) {
	verdicts, err := catalog.Judge(t.Context(), check.Commit{Files: good().files()})
	require.NoError(t, err)
	require.Len(t, verdicts, 1)
	assert.Equal(t, "catalog", verdicts[0].Name)
	assert.Empty(t, verdicts[0].Findings)
}
