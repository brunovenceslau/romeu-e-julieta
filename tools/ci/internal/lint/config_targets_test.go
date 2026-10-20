// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// lintContext is what decides whether a Go file builds for one of
// Targets: the target, cgo, the compiler and the tags of the
// toolchain, and the lists of the operating systems and architectures
// the toolchain knows, which say which file-name suffixes are
// constraints.
type lintContext struct {
	goos, goarch string
	cgo          bool
	compiler     string
	toolTags     []string // the instruction set level and the goexperiment tags
	releaseTags  []string // go1.1 to the version of the toolchain
	knownOS      map[string]bool
	knownArch    map[string]bool
	unixOS       map[string]bool
}

// lintTargetContexts returns the build context of each of Targets,
// the list the lint step runs for, as the pinned go command builds it
// in the environment of the lint step (Env). It asks "go list" for
// the context (the template function "context" of "go help list"), so
// the tool tags are the toolchain's own: the instruction set level
// (amd64.v1, arm64.v8.0) and the goexperiment tags, which differ from
// one target to another (cfg.go and internal/buildcfg in
// cmd/go, Go 1.27). Cgo is off, and there are no tags of our own. The
// operating systems and architectures that a file-name suffix can name
// are the lists of internal/syslist in that toolchain's GOROOT
// (syslistOf).
func lintTargetContexts(t *testing.T) []lintContext {
	t.Helper()
	tools := citest.PinnedLintTools(t, mise.Path, mise.Environ, mise.ResolveLintTools)
	format := `{{context.CgoEnabled}} {{context.Compiler}} {{join context.ToolTags ","}} {{join context.ReleaseTags ","}} {{context.GOROOT}}`
	var contexts []lintContext
	for _, target := range Targets {
		var stdout strings.Builder
		err := procgroup.Run(t.Context(), procgroup.Command{
			Class:  procgroup.ToolProbe,
			Argv:   []string{filepath.Join(tools.GoDir, "go"), "list", "-f", format, "runtime"},
			Dir:    t.TempDir(),
			Env:    Env(target, tools.GoDir),
			Stdout: &stdout,
		})
		require.NoError(t, err, "go list for %s", target)
		out := stdout.String()
		fields := strings.Fields(out)
		require.Len(t, fields, 5, "go list for %s: %q", target, out)
		known := syslistOf(t, fields[4])
		contexts = append(contexts, lintContext{
			goos:        target.GOOS,
			goarch:      target.GOARCH,
			cgo:         fields[0] == "true",
			compiler:    fields[1],
			toolTags:    strings.Split(fields[2], ","),
			releaseTags: strings.Split(fields[3], ","),
			knownOS:     known["KnownOS"],
			knownArch:   known["KnownArch"],
			unixOS:      known["UnixOS"],
		})
	}
	return contexts
}

// syslistOf reads the maps KnownOS, KnownArch and UnixOS of
// internal/syslist in goroot, the lists go/build matches file names and
// the "unix" tag against, so this test knows the systems of the
// toolchain the lint step runs and keeps no copy of them.
func syslistOf(t *testing.T, goroot string) map[string]map[string]bool {
	t.Helper()
	file := filepath.Join(goroot, "src", "internal", "syslist", "syslist.go")
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	require.NoError(t, err)
	lists := map[string]map[string]bool{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			lit, ok := value.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			names := map[string]bool{}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				require.True(t, ok, "%s: an entry of %s", file, value.Names[0].Name)
				key, ok := kv.Key.(*ast.BasicLit)
				require.True(t, ok, "%s: a key of %s", file, value.Names[0].Name)
				name, err := strconv.Unquote(key.Value)
				require.NoError(t, err)
				names[name] = true
			}
			lists[value.Names[0].Name] = names
		}
	}
	for _, name := range []string{"KnownOS", "KnownArch", "UnixOS"} {
		require.NotEmpty(t, lists[name], "%s: %s", file, name)
	}
	return lists
}

// matchTag reports whether the tag holds in the context, as go/build
// matches one: cgo when cgo is on, the target, the compiler, an
// operating system that the target's counts as (android as linux,
// illumos as solaris, ios as darwin), unix on a unix system, and the
// tool and release tags; boringcrypto is the older name of
// goexperiment.boringcrypto.
func (c lintContext) matchTag(name string) bool {
	switch {
	case c.cgo && name == "cgo":
		return true
	case name == c.goos || name == c.goarch || name == c.compiler:
		return true
	case c.goos == "android" && name == "linux", c.goos == "illumos" && name == "solaris", c.goos == "ios" && name == "darwin":
		return true
	case name == "unix" && c.unixOS[c.goos]:
		return true
	case slices.Contains(strings.Split(BuildTags, ","), name):
		return true
	case name == "boringcrypto":
		name = "goexperiment.boringcrypto"
	}
	return slices.Contains(c.toolTags, name) || slices.Contains(c.releaseTags, name)
}

// matchName reports whether the file name, by its suffixes, builds in
// the context: the part from the first "_" up to the first ".", without
// a last "_test", ends in _GOOS_GOARCH, _GOOS or _GOARCH of a known
// system and architecture, which must then match, or names none.
func (c lintContext) matchName(name string) bool {
	name, _, _ = strings.Cut(name, ".")
	i := strings.Index(name, "_")
	if i < 0 {
		return true
	}
	l := strings.Split(name[i:], "_")
	if n := len(l); l[n-1] == "test" {
		l = l[:n-1]
	}
	n := len(l)
	switch {
	case n >= 2 && c.knownOS[l[n-2]] && c.knownArch[l[n-1]]:
		return c.matchTag(l[n-1]) && c.matchTag(l[n-2])
	case n >= 1 && (c.knownOS[l[n-1]] || c.knownArch[l[n-1]]):
		return c.matchTag(l[n-1])
	}
	return true
}

// buildConstraint returns the build constraint of a Go source, as
// go/build reads it: the "//go:build" line among the comments before
// the first text that is not a comment, and without one, the "// +build"
// lines of the leading run of blank lines and line comments that a
// blank line ends, all of which must hold. Nil means no constraint. Two
// "//go:build" lines are an error.
func buildConstraint(src []byte) (constraint.Expr, error) {
	var goBuild string
	var plusBuild []string
	var pending []string // the +build lines since the last blank line
	isLeading := true    // only blank lines and line comments so far
	isInBlock := false   // inside a /* comment
	for line := range strings.Lines(string(src)) {
		line = strings.TrimSpace(line)
		if line == "" && isLeading {
			plusBuild, pending = append(plusBuild, pending...), nil
			continue
		}
		if !strings.HasPrefix(line, "//") {
			isLeading = false
		}
		if !isInBlock && constraint.IsGoBuild(line) {
			if goBuild != "" {
				return nil, errors.New("two //go:build lines")
			}
			goBuild = line
		}
		if isLeading && constraint.IsPlusBuild(line) {
			pending = append(pending, line)
		}
		if !skipComments(line, &isInBlock) {
			break
		}
	}
	if goBuild != "" {
		return constraint.Parse(goBuild)
	}
	var expr constraint.Expr
	for _, line := range plusBuild {
		x, err := constraint.Parse(line)
		if err != nil {
			continue // go/build ignores a +build line it cannot parse
		}
		if expr == nil {
			expr = x
		} else {
			expr = &constraint.AndExpr{X: expr, Y: x}
		}
	}
	return expr, nil
}

// skipComments reports whether the line holds nothing but comments,
// a line comment or /* */ comments, and keeps track of a /* comment
// that the line leaves open.
func skipComments(line string, isInBlock *bool) bool {
	for line != "" {
		if *isInBlock {
			_, rest, ok := strings.Cut(line, "*/")
			if !ok {
				return true
			}
			*isInBlock = false
			line = strings.TrimSpace(rest)
			continue
		}
		switch {
		case strings.HasPrefix(line, "//"):
			return true
		case strings.HasPrefix(line, "/*"):
			*isInBlock = true
			line = strings.TrimSpace(line[len("/*"):])
		default:
			return false
		}
	}
	return true
}

// buildsForLintTarget reports whether the Go file builds for at least
// one of the contexts of lintTargetContexts, as go/build's MatchFile
// tells it: a file that begins with "_" or "." does not, the file-name
// suffixes (_darwin, _arm64, _test) must match (matchName), and so must
// the build constraint (buildConstraint). MatchFile leaves the import of
// "C" to Import, so this reads the imports too: a file that imports "C"
// builds only with cgo, which every target has off.
func buildsForLintTarget(contexts []lintContext, dir, name string) (bool, error) {
	if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		return false, nil
	}
	src, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return false, err
	}
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), src, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return false, nil
		}
	}
	expr, err := buildConstraint(src)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	for _, c := range contexts {
		if c.matchName(name) && (expr == nil || expr.Eval(c.matchTag)) {
			return true, nil
		}
	}
	return false, nil
}

// TestBuildsForLintTarget pins buildsForLintTarget on constraints that
// no lint target satisfies and on those that one does.
func TestBuildsForLintTarget(t *testing.T) {
	tests := []struct {
		name string
		file string
		src  string
		want bool
	}{
		{"no constraint", "a.go", "package p\n", true},
		{"darwin only by name", "a_darwin.go", "package p\n", true},
		{"darwin arm64 test by name", "a_darwin_arm64_test.go", "package p\n", true},
		{"amd64 by name", "a_amd64.go", "package p\n", true},
		{"amd64 by tag", "a.go", "//go:build amd64\n\npackage p\n", true},
		{"cgo only", "a.go", "//go:build cgo\n\npackage p\n", false},
		{"not cgo", "a.go", "//go:build !cgo\n\npackage p\n", true},
		{"imports C", "a.go", "package p\n\nimport \"C\"\n", false},
		{"riscv64 by name", "a_riscv64.go", "package p\n", false},
		{"windows by name", "a_windows.go", "package p\n", false},
		{"linux 386 by name", "a_linux_386.go", "package p\n", false},
		{"ignore tag", "a.go", "//go:build ignore\n\npackage p\n", false},
		{"custom tag", "a.go", "//go:build never\n\npackage p\n", false},
		{"unix", "a.go", "//go:build unix\n\npackage p\n", true},
		{"darwin or windows", "a.go", "//go:build darwin || windows\n\npackage p\n", true},
		{"windows and not linux", "a.go", "//go:build windows && !linux\n\npackage p\n", false},
		{"plus build line", "a.go", "// +build ignore\n\npackage p\n", false},
		{"name and tag disagree", "a_windows.go", "//go:build linux\n\npackage p\n", false},
		{"begins with underscore", "_a.go", "package p\n", false},
		{"amd64.v1 tag", "a.go", "//go:build amd64.v1\n\npackage p\n", true},
		{"arm64.v8.0 tag", "a.go", "//go:build arm64.v8.0\n\npackage p\n", true},
		{"no default instruction set level", "a.go", "//go:build !amd64.v1 && !arm64.v8.0\n\npackage p\n", false},
		{"a level above the default", "a.go", "//go:build amd64.v3 || arm64.v9.0\n\npackage p\n", false},
		{"a default experiment off", "a.go", "//go:build !goexperiment.regabiargs\n\npackage p\n", false},
	}
	contexts := lintTargetContexts(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, tt.file), []byte(tt.src), 0o644))
			got, err := buildsForLintTarget(contexts, dir, tt.file)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// skippedLocation reports why "go list ./..." and so golangci-lint skip
// the Go file at rel (a slash path below the module root), or "" when
// they do not: a directory or file name that begins with "_" or ".",
// a directory named testdata or vendor, or a directory under another
// go.mod. moduleDirs are the directories of the other go.mod files.
func skippedLocation(rel string, moduleDirs []string) string {
	parts := strings.SplitSeq(rel, "/")
	for p := range parts {
		switch {
		case strings.HasPrefix(p, "_") || strings.HasPrefix(p, "."):
			return "a path element that begins with _ or ."
		case p == "testdata" || p == "vendor":
			return "a " + p + " directory"
		}
	}
	for _, d := range moduleDirs {
		if strings.HasPrefix(rel, d+"/") {
			return "a directory of its own module (" + d + "/go.mod)"
		}
	}
	return ""
}

// TestSkippedLocation pins skippedLocation.
func TestSkippedLocation(t *testing.T) {
	mods := []string{"sub/mod"}
	tests := []struct {
		name string
		rel  string
		want bool
	}{
		{"file at the root", "main.go", false},
		{"file in a package", "tools/ci/fast.go", false},
		{"nested testdata", "tools/ci/testdata/x.go", true},
		{"testdata at the root", "testdata/x.go", true},
		{"vendor", "vendor/x/y.go", true},
		{"underscore directory", "_x/y.go", true},
		{"dot directory", "tools/.x/y.go", true},
		{"underscore file", "tools/_y.go", true},
		{"nested module", "sub/mod/x.go", true},
		{"sibling of a nested module", "sub/other/x.go", false},
		{"name that extends a nested module", "sub/module/x.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, skippedLocation(tt.rel, mods) != "")
		})
	}
}

// TestEveryGoFileIsLinted checks that the linter reaches each Go file
// of the repository, the tracked ones and the ones not yet added: the
// lint step lints each of Targets with cgo off (fastSteps), so a
// file must build for one of them, and it must sit where "go list ./..."
// looks (ADR 0007, rule 8). A file that is skipped is a file whose
// findings nobody sees.
func TestEveryGoFileIsLinted(t *testing.T) {
	root := citest.ModuleRoot(t)
	var moduleDirs []string
	for _, f := range repoFiles(t, root, "*go.mod") {
		if filepath.Base(f) == "go.mod" && filepath.Dir(f) != "." {
			moduleDirs = append(moduleDirs, filepath.ToSlash(filepath.Dir(f)))
		}
	}
	files := repoFiles(t, root, "*.go")
	require.NotEmpty(t, files, "the Go files of the repository")
	contexts := lintTargetContexts(t)
	for _, f := range files {
		if why := skippedLocation(filepath.ToSlash(f), moduleDirs); why != "" {
			assert.Failf(t, "not linted", "%s is in %s, which go list skips", f, why)
			continue
		}
		ok, err := buildsForLintTarget(contexts, filepath.Join(root, filepath.Dir(f)), filepath.Base(f))
		require.NoError(t, err, "build constraints of %s", f)
		assert.True(t, ok, "%s builds for none of %v with cgo off, so the lint steps skip it", f, Targets)
	}
}
