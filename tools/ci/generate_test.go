// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adr"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// TestADRIndex pins the index: a row per record, the title and the
// status escaped for a table cell and a link's text, and the file name
// escaped as a path segment, so that no name makes Markdown of its own.
func TestADRIndex(t *testing.T) {
	got := string(adrIndex([]adr.Record{
		{Number: 1, Title: "Pick a thing", Status: "Accepted", File: "0001-pick-a-thing.md"},
		{Number: 2, Title: `Use [x] | y \ z`, Status: "Pro|posed", File: "0002-a (b) [c].md"},
	}))
	assert.True(t, strings.HasPrefix(got, "<!-- "+generatedFrom("the records in docs/adr")+" -->\n\n# Decision records\n"), got)
	assert.True(t, strings.HasSuffix(got, "| Record | Title | Status |\n|---|---|---|\n"+
		"| 1 | [Pick a thing](0001-pick-a-thing.md) | Accepted |\n"+
		`| 2 | [Use \[x\] \| y \\ z](0002-a%20%28b%29%20%5Bc%5D.md) | Pro\|posed |`+"\n"), got)
}

// TestGenerate runs generate in a fixture: it writes the three files
// from their sources, says which, and on a second run writes nothing.
func TestGenerate(t *testing.T) {
	r := newTree(t)
	for _, f := range []string{codeownersPath, askFirstPagePath, adr.Dir + "/" + adr.Index} {
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(f))))
	}
	code, out := runCI(t, r, nil, nil, "generate")
	require.Equal(t, exitOK, code, out)
	assert.Equal(t, "generate: wrote .github/CODEOWNERS\ngenerate: wrote docs/reference/ask-first.md\ngenerate: wrote docs/adr/README.md\n", out)

	list, err := parseAskFirst([]byte(fixtureList), fixtureModule)
	require.NoError(t, err)
	records, err := adr.Read(os.DirFS(r.Dir), adr.Dir)
	require.NoError(t, err)
	for path, want := range map[string][]byte{
		codeownersPath:            codeowners(list),
		askFirstPagePath:          askFirstPage(list),
		adr.Dir + "/" + adr.Index: adrIndex(records),
	} {
		got, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(path)))
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got), path)
		info, err := os.Stat(filepath.Join(r.Dir, filepath.FromSlash(path)))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), path)
	}

	code, out = runCI(t, r, nil, nil, "generate")
	require.Equal(t, exitOK, code, out)
	assert.Empty(t, out, "a second run finds every file current")
}

// TestGenerateRefuses checks that generate writes nothing when a
// source cannot be read: a list it refuses, a record it cannot index, a
// missing go.mod, and a stray argument.
func TestGenerateRefuses(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		change func(t *testing.T, r *gittest.Repo)
		want   string
	}{
		{"a stray argument", []string{"stray"}, nil, "generate takes no argument"},
		{"a list without itself", nil, func(t *testing.T, r *gittest.Repo) {
			r.Write(askFirstPath, strings.Replace(fixtureList, "[.github/ask-first.yaml]", "[x]", 1))
		}, "lists itself"},
		{"no list", nil, func(t *testing.T, r *gittest.Repo) {
			require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(askFirstPath))))
		}, "ask-first.yaml"},
		{"a glob without a star that names a directory", nil, func(t *testing.T, r *gittest.Repo) {
			r.Write(askFirstPath, strings.Replace(fixtureList, "internal/spec/project.go", "docs/adr", 1))
		}, `surface gates: glob "docs/adr" names a directory, which CODEOWNERS reads as its subtree and the matcher of 12 12.4 as one path; write "docs/adr/**"`},
		{"a record without a status", nil, func(t *testing.T, r *gittest.Repo) { r.Write("docs/adr/0002-b.md", "# 2. B\n") }, "0002-b.md"},
		{"a list that is a link to itself", nil, func(t *testing.T, r *gittest.Repo) {
			path := filepath.Join(r.Dir, filepath.FromSlash(askFirstPath))
			require.NoError(t, os.Remove(path))
			require.NoError(t, os.Symlink("ask-first.yaml", path))
		}, ".github/ask-first.yaml is not a regular file; a source of the generators is never a link"},
		{"no go.mod", nil, func(t *testing.T, r *gittest.Repo) { require.NoError(t, os.Remove(filepath.Join(r.Dir, "go.mod"))) }, "go.mod"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			before, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(codeownersPath)))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(r.Dir, filepath.FromSlash(codeownersPath)), append(before, "# stale\n"...), 0o644))
			if tt.change != nil {
				tt.change(t, r.For(t))
			}
			code, out := runCI(t, r, nil, nil, append([]string{"generate"}, tt.args...)...)
			assert.Equal(t, exitError, code, out)
			assert.Contains(t, out, tt.want)
			after, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(codeownersPath)))
			require.NoError(t, err)
			assert.Equal(t, string(before)+"# stale\n", string(after), "nothing written")
		})
	}
}

// TestGenerateGlobThatNamesAFile checks that a glob without a star
// that names a file, or a path that does not exist yet, passes the
// directory rule.
func TestGenerateGlobThatNamesAFile(t *testing.T) {
	r := newTree(t)
	r.Write("internal/spec/project.go", "package spec\n")
	code, out := runCI(t, r, nil, nil, "generate")
	assert.Equal(t, exitOK, code, out)
}

// TestGenerateRepairsTheMode checks that generate writes a generated
// file again when it holds the right bytes as a symbolic link or as an
// executable file, the two modes the generated step refuses, so that
// its advice repairs what it reports.
func TestGenerateRepairsTheMode(t *testing.T) {
	r := newTree(t)
	path := filepath.Join(r.Dir, filepath.FromSlash(codeownersPath))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	target := filepath.Join(r.Dir, "owners.txt")
	require.NoError(t, os.WriteFile(target, data, 0o644))
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Symlink(target, path))
	require.NoError(t, os.Chmod(filepath.Join(r.Dir, filepath.FromSlash(askFirstPagePath)), 0o755))

	code, out := runCI(t, r, nil, nil, "generate")
	require.Equal(t, exitOK, code, out)
	assert.Equal(t, "generate: wrote .github/CODEOWNERS\ngenerate: wrote docs/reference/ask-first.md\n", out)
	for _, p := range []string{codeownersPath, askFirstPagePath} {
		info, err := os.Lstat(filepath.Join(r.Dir, filepath.FromSlash(p)))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode(), p)
	}
	kept, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(kept), "the link's target is left as it was")
}
