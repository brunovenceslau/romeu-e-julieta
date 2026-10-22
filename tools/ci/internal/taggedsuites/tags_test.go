// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package taggedsuites

import (
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// tagsOf returns, for each tag that a //go:build line of a .go file
// below root names, the files that name it (relative, sorted). The
// walk skips the directories the go command skips (testdata, and those
// that begin with "." or "_"), so a worktree or a fixture kept in the
// tree is not read.
func tagsOf(t *testing.T, root string) map[string][]string {
	t.Helper()
	found := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for line := range strings.Lines(string(src)) {
			line = strings.TrimSpace(line)
			// Constraints precede the package clause.
			if strings.HasPrefix(line, "package ") {
				break
			}
			if !constraint.IsGoBuild(line) {
				continue
			}
			expr, err := constraint.Parse(line)
			require.NoError(t, err, "%s", rel)
			expr.Eval(func(tag string) bool {
				if !slices.Contains(found[tag], filepath.ToSlash(rel)) {
					found[tag] = append(found[tag], filepath.ToSlash(rel))
				}
				return false
			})
		}
		return nil
	})
	require.NoError(t, err)
	for _, files := range found {
		sort.Strings(files)
	}
	return found
}

// syslistFile returns internal/syslist/syslist.go of the toolchain the
// test runs under: the GOROOT variable when set, else the root of the
// first go on PATH (the steps put the pinned go first), with the
// links of its binary resolved. runtime.GOROOT is deprecated, and no
// subprocess starts here (04 4.1).
func syslistFile(t *testing.T) string {
	t.Helper()
	var roots []string
	if r := os.Getenv("GOROOT"); r != "" {
		roots = append(roots, r)
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if bin, err := filepath.EvalSymlinks(filepath.Join(dir, "go")); err == nil {
			roots = append(roots, filepath.Dir(filepath.Dir(bin)))
		}
	}
	for _, r := range roots {
		file := filepath.Join(r, "src", "internal", "syslist", "syslist.go")
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}
	require.FailNow(t, "no internal/syslist found", "GOROOT and the go on PATH: %q", roots)
	return ""
}

// syslist reads the maps KnownOS, KnownArch and UnixOS of
// internal/syslist in the toolchain's GOROOT: the lists go/build matches
// file names and the "unix" tag against. go/build is not imported
// here, since the call-site scan of 04 4.1 admits it in
// internal/procgroup alone (it can start the go command), and no copy
// of the lists is kept.
func syslist(t *testing.T) map[string]bool {
	t.Helper()
	file := syslistFile(t)
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	require.NoError(t, err)
	names := map[string]bool{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Values) != 1 {
				continue
			}
			lit, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				require.True(t, ok, "%s: an entry of %s", file, value.Names[0].Name)
				key, ok := kv.Key.(*ast.BasicLit)
				require.True(t, ok, "%s: a key of %s", file, value.Names[0].Name)
				name, err := strconv.Unquote(key.Value)
				require.NoError(t, err)
				names[name] = true
			}
		}
	}
	require.NotEmpty(t, names, "%s: the system lists", file)
	return names
}

var releaseTag = regexp.MustCompile(`^go1\.[0-9]+$`)

// toolchainTag reports whether the toolchain names the tag as its own:
// a GOOS or GOARCH name (syslist), unix, cgo, a compiler, a release tag
// (go1.N) or an experiment (goexperiment.*).
func toolchainTag(tag string, known map[string]bool) bool {
	return known[tag] || tag == "unix" || tag == "cgo" || tag == "gc" || tag == "gccgo" ||
		releaseTag.MatchString(tag) || strings.HasPrefix(tag, "goexperiment.")
}

// declared returns the tags of found that are not in suites and are not
// the toolchain's.
func declared(suites []Suite, found map[string][]string, known map[string]bool) []string {
	var out []string
	for tag, files := range found {
		if toolchainTag(tag, known) || slices.ContainsFunc(suites, func(s Suite) bool { return s.Tag == tag }) {
			continue
		}
		out = append(out, tag+" ("+strings.Join(files, ", ")+")")
	}
	sort.Strings(out)
	return out
}

// carried returns the tags of suites that no file of found carries.
func carried(suites []Suite, found map[string][]string) []string {
	var out []string
	for _, s := range suites {
		if len(found[s.Tag]) == 0 {
			out = append(out, s.Tag)
		}
	}
	return out
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return root
}

// TestTagsOfFixtureTree holds both directions on a fixture tree: a tag
// of Suites that no file carries is found, and so is a //go:build tag
// outside Suites, the names of the toolchain left out.
func TestTagsOfFixtureTree(t *testing.T) {
	root := writeTree(t, map[string]string{
		"a/a.go":          "//go:build e2e\n\npackage a\n",
		"a/b.go":          "//go:build unix && !windows && go1.21\n\npackage a\n",
		"a/c.go":          "// Copyright\n\n//go:build (linux || darwin) && cgo && amd64\n\npackage a\n",
		"a/d.go":          "//go:build host\n\npackage a\n",
		"a/testdata/x.go": "//go:build hidden\n\npackage x\n",
		".claude/w/y.go":  "//go:build worktree\n\npackage y\n",
		"a/note.txt":      "//go:build text\n",
		"a/e.go":          "package a // //go:build not-a-constraint\n",
		"a/f.go":          "//go:build goexperiment.jsonv2 || ignore\n\npackage a\n",
	})
	found := tagsOf(t, root)
	known := syslist(t)

	assert.Equal(t, []string{"host (a/d.go)", "ignore (a/f.go)"}, declared([]Suite{{"e2e", "e2e"}}, found, known), "host is outside Suites; ignore is the generator tag, not the toolchain's")
	assert.Empty(t, declared([]Suite{{"e2e", "e2e"}, {"host", "e2e/host"}, {"ignore", "x"}}, found, known))
	assert.Equal(t, []string{"none"}, carried([]Suite{{"e2e", "e2e"}, {"none", "none"}}, found))
	assert.Empty(t, carried([]Suite{{"e2e", "e2e"}, {"host", "e2e/host"}}, found))
}

// TestSuitesTagsAreCarried fails when Suites names a tag that no file
// of the module carries: the suite would be vetted, linted and listed
// with a tag that selects nothing.
func TestSuitesTagsAreCarried(t *testing.T) {
	assert.Empty(t, carried(Suites, tagsOf(t, citest.ModuleRoot(t))))
}

// TestEveryTagIsInSuites fails when a //go:build line of the module
// names a tag outside Suites that the toolchain does not name: no step
// would pass it, and the file would be built by none.
func TestEveryTagIsInSuites(t *testing.T) {
	assert.Empty(t, declared(Suites, tagsOf(t, citest.ModuleRoot(t)), syslist(t)))
}
