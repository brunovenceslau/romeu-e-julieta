// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/drift"
)

// allowedDirectives are the directives a comment of any Go file of this
// repository may hold, as "tool:name". go:build is the build
// constraint (tools/ci/internal/hygiene/add_test.go); romeu:invariant tags a guard or its test
// with the invariant it holds (05 5.2), read by the invariants check
// when it lands. Every other directive, of any tool, is a decision to
// review here first, because the readers of directives include the
// linters, and theirs turn findings off.
var allowedDirectives = []string{"go:build", "romeu:invariant"}

// embedSites are the files that may hold a go:embed directive (02 2.1,
// REQ-0129; ADR 0007: a new directive is a reviewed line here), each
// named by path, and no other file may: kits/embed.go is the embed.FS of
// the product kits. The catalog's file joins this list in T031. A
// pattern in a site may not take the all: prefix, which would embed
// dotfiles and files whose names begin with an underscore.
var embedSites = []string{"kits/embed.go"}

// embedPatterns returns the patterns of the //go:embed lines of src,
// each line split on spaces.
func embedPatterns(src []byte) []string {
	var out []string
	for line := range strings.SplitSeq(string(src), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "//go:embed "); ok {
			out = append(out, strings.Fields(rest)...)
		}
	}
	return out
}

// toolDirective matches the start of a directive of the "tool:name"
// form after directiveText has normalised the line: the shape go/ast
// reads (isDirective in go/ast/ast.go, Go 1.27: "[a-z0-9]+:[a-z0-9]"),
// which revive's "revive:disable" and staticcheck's "lint:ignore" have.
// The match is the "tool:name" word.
var toolDirective = regexp.MustCompile(`^[a-z0-9]+:[a-z0-9]\S*`)

// nolintDirective matches golangci-lint's own directive after
// directiveText. golangci-lint matches "^nolint( |:|$)"
// (NewNolintFilter, pkg/result/processors/nolint_filter.go, v2.14.0);
// this one also takes the word followed by any other character that
// ends it, which is more than the linter reads. It is built in two
// parts so that this file does not hold what it looks for.
var nolintDirective = regexp.MustCompile(`^no` + `lint\b`)

// goLineDirectives are the directives without a colon that the Go
// toolchain reads when they follow "//" or "/*" with no space between
// (isDirective in go/ast/ast.go): a line directive, which moves the
// position of what follows (and golangci-lint drops a finding moved to
// a file that is not Go), gccgo's extern and cgo's export.
var goLineDirectives = []string{"line", "extern", "export"}

// directiveText normalises one line of a comment as the most lenient
// reader of directives does. golangci-lint applies
// strings.TrimLeft(text, "/ ") to the text of the comment with its "//"
// or "/*" (extractInlineRangeFromComment of nolint_filter.go), and
// revive accepts "\s", any of tab, line feed, form feed, carriage return
// and space, after "//" (directiveRegexp in lint/file.go, v1.17.0). This
// one trims "/", "*" and every rune for which unicode.IsSpace holds, a
// superset of both sets, so it finds at least what each finds.
func directiveText(line string) string {
	return strings.TrimLeftFunc(line, func(r rune) bool {
		return r == '/' || r == '*' || unicode.IsSpace(r)
	})
}

// directives returns the directives that the comments of the Go source
// hold, as "tool:name" or the bare name, in any position: every line of
// every comment read leniently (directiveText) for the "tool:name" form
// and golangci-lint's, and the start of each comment read as the Go
// toolchain reads it for the forms of goLineDirectives.
func directives(src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, group := range f.Comments {
		for _, c := range group.List {
			// c.Text starts with "//" or "/*", and the toolchain reads
			// the word right after it.
			for _, name := range goLineDirectives {
				if strings.HasPrefix(c.Text[2:], name+" ") {
					found = append(found, name)
				}
			}
			for line := range strings.SplitSeq(c.Text, "\n") {
				text := directiveText(line)
				if m := toolDirective.FindString(text); m != "" {
					found = append(found, m)
				} else if m := nolintDirective.FindString(text); m != "" {
					found = append(found, m)
				}
			}
		}
	}
	return found, nil
}

// repoFiles lists the files of the repository that match the pathspecs,
// the tracked ones and the ones not yet added, as paths below root. The
// ones that a .gitignore names are listed too: the linter reads them
// all the same.
func repoFiles(t *testing.T, root string, pathspecs ...string) []string {
	t.Helper()
	args := append([]string{"-C", root, "ls-files", "-z", "--cached", "--others", "--"}, pathspecs...)
	var out strings.Builder
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolGit,
		Argv:   append([]string{"git"}, args...),
		Env:    os.Environ(),
		Stdout: &out,
	})
	require.NoError(t, err, "git ls-files")
	var files []string
	for f := range strings.SplitSeq(out.String(), "\x00") {
		if f == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, f)); errors.Is(err, os.ErrNotExist) {
			continue // listed and deleted in the working tree
		}
		files = append(files, f)
	}
	return files
}

// disallowed returns what the Go file at path holds that would make the
// linters skip a finding: each directive that is not one of allowed. A
// generated-code header is no longer one, since the linters read
// generated files (ADR 0016, TestLintConfigLowersNothing).
func disallowed(path string, allowed []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	found, err := directives(data)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, name := range found {
		if !slices.Contains(allowed, name) {
			bad = append(bad, "directive "+name)
		}
	}
	return bad, nil
}

// TestDisallowed pins disallowed on files written to disk, the way
// TestOnlyAllowedDirectives reads the files of the repository. The
// directives are built in parts so that this file holds none.
func TestDisallowed(t *testing.T) {
	nl, rv := "no"+"lint", "revive:dis"+"able"
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"clean", "//go:build linux\n\n// Package p is it.\npackage p\n", nil},
		{"a nolint", "package p\n\nvar _ = 1 //" + nl + "\n", []string{"directive " + nl}},
		{"revive after a form feed", "package p\n\n// Doc.\n// \f" + rv + "\nfunc F() {}\n", []string{"directive " + rv}},
		{"a stray go:generate", "package p\n\n//go:generate echo hi\nvar _ = 1\n", []string{"directive go:generate"}},
		{"an invariant tag", "package p\n\n//romeu:invariant I29 guard\nfunc F() {}\n", nil},
		{"a misspelt invariant tag", "package p\n\n//romeu:invariants I29 guard\nfunc F() {}\n", []string{"directive romeu:invariants"}},
		{"go:embed", "package p\n\n//go:embed data.txt\nvar data string\n", []string{"directive go:embed"}},
		{"go:linkname", "package p\n\n//go:linkname f runtime.f\nfunc f()\n", []string{"directive go:linkname"}},
		{"a generated header", generatedHeader + "package p\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.go")
			require.NoError(t, os.WriteFile(path, []byte(tt.src), 0o644))
			got, err := disallowed(path, allowedDirectives)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEmbedSites holds the embed allowance both ways: a go:embed in a
// file that is not an embed site is reported, and the all: prefix is
// read from the line.
func TestEmbedSites(t *testing.T) {
	src := "package p\n\nimport \"embed\"\n\n//go:embed julieta\nvar FS embed.FS\n"
	path := filepath.Join(t.TempDir(), "elsewhere.go")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	got, err := disallowed(path, allowedDirectives)
	require.NoError(t, err)
	assert.Equal(t, []string{"directive go:embed"}, got, "go:embed outside embedSites is reported")
	assert.Equal(t, []string{"julieta"}, embedPatterns([]byte(src)))
	assert.Equal(t, []string{"all:julieta", "x"}, embedPatterns([]byte("//go:embed all:julieta x\n")))
	assert.Empty(t, embedPatterns([]byte("// go:embed julieta\n")), "only the directive form is a pattern line")
	for _, site := range embedSites {
		assert.False(t, strings.HasPrefix(site, "/") || strings.Contains(site, "*"), "%s is a path, not a pattern", site)
	}
}

// TestOnlyAllowedDirectives checks that each directive in the Go files
// of the repository, the tracked ones and the ones not yet added, is one
// of allowedDirectives (ADR 0007, rule 8): a finding is fixed in the
// code, and a directive that makes the linters skip it hides findings.
func TestOnlyAllowedDirectives(t *testing.T) {
	root := citest.ModuleRoot(t)
	files := repoFiles(t, root, "*.go")
	require.NotEmpty(t, files, "the Go files of the repository")
	for _, f := range files {
		allowed := allowedDirectives
		if f == drift.Site {
			allowed = append(slices.Clone(allowed), "go:generate")
		}
		if slices.Contains(embedSites, f) {
			allowed = append(slices.Clone(allowed), "go:embed")
			src, err := os.ReadFile(filepath.Join(root, f))
			require.NoError(t, err)
			for _, pattern := range embedPatterns(src) {
				assert.False(t, strings.HasPrefix(pattern, "all:"), "%s embeds %s: the all: prefix takes dotfiles and underscore files", f, pattern)
			}
		}
		bad, err := disallowed(filepath.Join(root, f), allowed)
		require.NoError(t, err, "read %s", f)
		assert.Empty(t, bad, "%s holds what makes the linters skip a finding", f)
	}
	// The raw lines that go generate runs: exactly one in the repository,
	// at its site (directives.go of the drift package). The comments are read above for the
	// other directives and here for the lenient forms of this one.
	problems, err := drift.CheckDirectives(root)
	require.NoError(t, err)
	assert.Empty(t, problems, "the go:generate lines of the repository")
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(drift.Site)))
	require.NoError(t, err)
	found, err := directives(src)
	require.NoError(t, err)
	assert.Equal(t, 1, countEntries(found, "go:generate"), "%s", drift.Site)
}

// countEntries returns how many entries of found are exactly name.
func countEntries(found []string, name string) int {
	n := 0
	for _, f := range found {
		if f == name {
			n++
		}
	}
	return n
}

// TestDirectiveCountIsPerEntry checks that the count of the go:generate
// directives of a file is a count of entries, so a longer directive
// such as go:generated does not pass for one.
func TestDirectiveCountIsPerEntry(t *testing.T) {
	found, err := directives([]byte("package p\n\n//go:generated x\n//go:generate y\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"go:generated", "go:generate"}, found)
	assert.Equal(t, 1, countEntries(found, "go:generate"))
	found, err = directives([]byte("package p\n\n//go:generated x\n"))
	require.NoError(t, err)
	assert.Zero(t, countEntries(found, "go:generate"), "go:generated is another directive")
}

// TestOneLintConfig checks that .golangci.yml is the only golangci-lint
// configuration file in the repository, the tracked ones and the ones
// not yet added: golangci-lint prefers a .golangci.yaml, .toml or .json
// beside it, and the lint step and the fixture test both name
// .golangci.yml, so a second file would be one that nothing reads.
func TestOneLintConfig(t *testing.T) {
	root := citest.ModuleRoot(t)
	var found []string
	for _, f := range repoFiles(t, root, "*.golangci.*") {
		if strings.HasPrefix(filepath.Base(f), ".golangci.") {
			found = append(found, f)
		}
	}
	assert.Equal(t, []string{".golangci.yml"}, found, "the golangci-lint configuration files of the repository")
}

// TestDirectives pins directives on the spellings that a reader of
// directives accepts, and on the comments that are none. The
// directives are built in parts so that this file holds none.
func TestDirectives(t *testing.T) {
	nl, ig := "no"+"lint", "lint:ig"+"nore"
	rv, ln := "revive:dis"+"able", "li"+"ne"
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"plain", "package p\n\nvar _ = 1 //" + nl + "\n", []string{nl}},
		{"space", "package p\n\nvar _ = 1 // " + nl + ":errcheck\n", []string{nl + ":errcheck"}},
		{"slash then name", "package p\n\nvar _ = 1 // /" + nl + "\n", []string{nl}},
		{"slash space name with linter", "package p\n\nvar _ = 1 // / " + nl + ":errcheck\n", []string{nl + ":errcheck"}},
		{"many slashes", "package p\n\nvar _ = 1 /////" + nl + "\n", []string{nl}},
		{"tab", "package p\n\nvar _ = 1 //\t" + nl + "\n", []string{nl}},
		{"revive after a tab", "package p\n\n//\t" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive after a form feed", "package p\n\n// \f" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive after a vertical tab", "package p\n\n//\v" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive after a carriage return", "package p\n\n/*\r" + rv + " */\nvar _ = 1\n", []string{rv}},
		{"revive after a no-break space", "package p\n\n//\u00a0" + rv + "\nvar _ = 1\n", []string{rv}},
		{"block comment", "package p\n\nvar _ = 1 /* " + nl + " */\n", []string{nl}},
		{"second line of a block comment", "package p\n\n/*\nabout\n" + nl + ":all\n*/\nvar _ = 1\n", []string{nl + ":all"}},
		{"staticcheck", "package p\n\n//" + ig + " SA1000 reason\nvar _ = 1\n", []string{ig}},
		{"staticcheck file", "package p\n\n// lint:file-ig" + "nore SA1000 reason\n", []string{"lint:file-ig" + "nore"}},
		{"in a doc comment line", "package p\n\n// F is it.\n//" + nl + "\nfunc F() {}\n", []string{nl}},
		{"revive", "package p\n\n//" + rv + "\nvar _ = 1\n", []string{rv}},
		{"revive with rule and space", "package p\n\n// " + rv + "-line:exported reason\nvar _ = 1\n", []string{rv + "-line:exported"}},
		{"line directive", "package p\n\n//" + ln + " x.txt:1\nvar _ = 1\n", []string{ln}},
		{"block line directive", "package p\n\nvar _ = /*" + ln + " x.txt:1:1*/ 1\n", []string{ln}},
		{"cgo export", "package p\n\n//exp" + "ort F\nfunc F() {}\n", []string{"exp" + "ort"}},
		{"gccgo extern", "package p\n\n//ext" + "ern c_f\nfunc F()\n", []string{"ext" + "ern"}},
		{"build constraint", "//go:build linux\n\npackage p\n", []string{"go:build"}},
		{"generate", "package p\n\n//go:gen" + "erate stringer\n", []string{"go:gen" + "erate"}},
		{"prose about it", "package p\n\n// F never uses a " + nl + " directive.\nfunc F() {}\n", nil},
		{"prose line that starts with the word line", "package p\n\n// the first\n// " + ln + " of it.\nvar _ = 1\n", nil},
		{"a URL", "package p\n\n// https://go.dev/doc/comment\nvar _ = 1\n", nil},
		{"a string", "package p\n\nvar _ = \"//" + nl + "\"\n", nil},
		{"a longer word", "package p\n\n// " + nl + "x is a word.\nvar _ = 1\n", nil},
		{"no comment", "package p\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := directives([]byte(tt.src))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
