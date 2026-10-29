// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package approval

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
)

// Invariant (b) of T005's acceptance (05 5.3, REQ-0540): the code the pr
// job builds, cmd/ask-first, imports only the standard library and packages
// under the globs of the approvals surface, so no change a grant covers
// changes the code that judges a pull request. TestPRImports holds it
// over the committed tree; the fixtures below hold the rule itself.

// prCommand is the package the ask-first job builds (REQ-1150).
const prCommand = "./tools/ci/internal/askfirst/cmd/ask-first"

// listedPackage is what the rule reads of one package that
// "go list -deps -test -json" prints.
type listedPackage struct {
	ImportPath string
	Dir        string
	// Standard is the go command's own reading of the standard library:
	// a package of GOROOT. A path without a dot is not read as one, since
	// a module under a local replace may have such a path.
	Standard bool
	Module   *listedModule
	// The files the package builds from, each relative to Dir, the test
	// files of the listed package included (-test).
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles []string
	SwigFiles, SwigCXXFiles, SysoFiles, EmbedFiles, TestGoFiles         []string
	XTestGoFiles, TestEmbedFiles, XTestEmbedFiles                       []string
	Error                                                               *struct{ Err string }
}

// listedModule is the module of a package, and what replaces it.
type listedModule struct {
	Path, Version string
	Main          bool
	Replace       *listedModule
}

// files returns every file the package builds from.
func (p listedPackage) files() []string {
	return slices.Concat(p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles,
		p.SwigFiles, p.SwigCXXFiles, p.SysoFiles, p.EmbedFiles, p.TestGoFiles, p.XTestGoFiles,
		p.TestEmbedFiles, p.XTestEmbedFiles)
}

// outsideApprovals applies the rule of invariant (b) to the packages
// that a listing of target prints for the tree at root, and returns one
// line per package it does not admit:
//
//   - a package of the standard library, by its Standard field, passes;
//   - a package whose directory is inside the tree (of the main module,
//     a vendor directory or a local replace) passes when every file it
//     builds from matches a glob of globs;
//   - a package whose directory is outside the tree passes when its
//     module, or the module that replaces it, has a version and go.sum
//     pins that version's code (a "<path> <version> h1:" line, not only
//     the go.mod line): go.sum is on the approvals surface, and the go
//     command checks a download against it. A local replace outside the
//     tree has no version, so no pin, and fails.
//
// The test main the go command writes for -test is no file of the tree
// and is left out (testMain).
func outsideApprovals(pkgs []listedPackage, root string, globs []string, goSum []byte) []string {
	var out []string
	root = resolved(root)
	for _, p := range pkgs {
		rel, err := filepath.Rel(root, resolved(p.Dir))
		inside := err == nil && filepath.IsLocal(rel)
		switch {
		case p.Error != nil:
			out = append(out, fmt.Sprintf("%s: does not load: %s", p.ImportPath, p.Error.Err))
		case p.Standard, testMain(p):
		case inside:
			for _, f := range p.files() {
				name := filepath.ToSlash(filepath.Join(rel, f))
				if !slices.ContainsFunc(globs, func(g string) bool { return GlobMatch(g, name) }) {
					out = append(out, fmt.Sprintf("%s: %s is on no approvals glob", p.ImportPath, name))
				}
			}
		default:
			m := p.Module
			if m != nil && m.Replace != nil {
				m = m.Replace
			}
			if m == nil || m.Main || m.Version == "" || !pinned(goSum, m.Path, m.Version) {
				out = append(out, fmt.Sprintf("%s: outside the tree, and go.sum pins no version of its module", p.ImportPath))
			}
		}
	}
	return out
}

// resolved returns path with its symbolic links resolved, or path itself
// when it cannot be: go list prints the directories it resolves (the
// temporary directory of macOS, /var, is /private/var), so both sides of
// a comparison are resolved first.
func resolved(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return path
}

// testMain reports whether p is the test main the go command writes for
// -test: a package "<path>.test" built from one file of its build cache,
// outside the tree.
func testMain(p listedPackage) bool {
	return strings.HasSuffix(p.ImportPath, ".test") && len(p.files()) == 1 && len(p.GoFiles) == 1 && filepath.IsAbs(p.GoFiles[0])
}

// pinned reports whether go.sum holds the hash of the code of the module
// path at version: a line "<path> <version> h1:<hash>".
func pinned(goSum []byte, path, version string) bool {
	for line := range strings.SplitSeq(string(goSum), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[0] == path && f[1] == version && strings.HasPrefix(f[2], "h1:") {
			return true
		}
	}
	return false
}

// listFor lists target with its dependencies and test dependencies in the
// module at dir, for one lint target, with the go command of the tests.
func listFor(t *testing.T, dir, target string, lt lint.Target) []listedPackage {
	t.Helper()
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   []string{goBin, "list", "-e", "-deps", "-test", "-json", target},
		Dir:    dir,
		Env:    lint.Env(lt, filepath.Dir(goBin)),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "go list for %s: %s", lt, stderr.String())
	var pkgs []listedPackage
	dec := jsontext.NewDecoder(&stdout)
	for {
		var p listedPackage
		err := json.UnmarshalDecode(dec, &p)
		if errors.Is(err, io.EOF) {
			return pkgs
		}
		require.NoError(t, err)
		pkgs = append(pkgs, p)
	}
}

// judgeImports lists target once for each of lint.Targets, so an import
// behind a build constraint of another target is read too, and returns
// what outsideApprovals finds in each listing.
func judgeImports(t *testing.T, root, target string, globs []string) []string {
	t.Helper()
	goSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if errors.Is(err, os.ErrNotExist) {
		goSum = nil
	} else {
		require.NoError(t, err)
	}
	var out []string
	for _, lt := range lint.Targets {
		for _, f := range outsideApprovals(listFor(t, root, target, lt), root, globs, goSum) {
			out = append(out, lt.String()+": "+f)
		}
	}
	return out
}

// approvalsGlobs returns the globs of the approvals surface of the list
// at root.
func approvalsGlobs(t *testing.T, root string) []string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, Path))
	require.NoError(t, err)
	list, err := Parse(src)
	require.NoError(t, err)
	i := slices.IndexFunc(list.Surfaces, func(s Surface) bool { return s.ID == "approvals" })
	require.GreaterOrEqual(t, i, 0, "the approvals surface")
	return list.Surfaces[i].Globs
}

// TestPRImports holds invariant (b) over the committed tree: every
// package cmd/ask-first builds, on every lint target, is of the standard
// library, on an approvals glob, or of a module go.sum pins.
func TestPRImports(t *testing.T) {
	root := citest.ModuleRoot(t)
	assert.Empty(t, judgeImports(t, root, prCommand, approvalsGlobs(t, root)))
}

// TestPRImportsRule holds the rule of invariant (b) with fixture modules,
// whose main module path has a dot, as this repository's does:
// an import outside the globs, a module path without a dot under a local
// replace, which the dot rule would read as the standard library, and an
// outside import that builds only for darwin, which a listing of linux
// alone misses.
func TestPRImportsRule(t *testing.T) {
	globs := []string{"cmd/**", "ok/**", "go.sum"}
	base := map[string]string{
		"go.mod":                "module example.org/fix\n\ngo 1.24\n",
		"cmd/ask-first/main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.org/fix/ok\"\n)\n\nfunc main() { fmt.Println(ok.X) }\n",
		"ok/ok.go":              "package ok\n\nconst X = 1\n",
		"outside/o.go":          "package outside\n\nconst Y = 2\n",
	}
	cases := []struct {
		name   string
		change map[string]string
		want   []string // a part of a finding, for each target that has one
	}{
		{"every package on a glob", nil, nil},
		{"an import outside the globs", map[string]string{
			"ok/ok.go": "package ok\n\nimport \"example.org/fix/outside\"\n\nconst X = outside.Y\n",
		}, []string{"outside/o.go is on no approvals glob"}},
		{"a test file of the command that imports outside the globs", map[string]string{
			"cmd/ask-first/main_test.go": "package main\n\nimport (\n\t\"testing\"\n\n\t\"example.org/fix/outside\"\n)\n\nfunc TestX(t *testing.T) { _ = outside.Y }\n",
		}, []string{"outside/o.go is on no approvals glob"}},
		{"a dotless module under a local replace inside the tree", map[string]string{
			"go.mod":         "module example.org/fix\n\ngo 1.24\n\nrequire example v0.0.0\n\nreplace example => ./example\n",
			"example/go.mod": "module example\n\ngo 1.24\n",
			"example/e.go":   "package example\n\nconst Z = 3\n",
			"ok/ok.go":       "package ok\n\nimport \"example\"\n\nconst X = example.Z\n",
		}, []string{"example/e.go is on no approvals glob"}},
		{"a dotless module under a local replace outside the tree", map[string]string{
			"go.mod":   "module example.org/fix\n\ngo 1.24\n\nrequire example v0.0.0\n\nreplace example => ../example\n",
			"ok/ok.go": "package ok\n\nimport \"example\"\n\nconst X = example.Z\n",
		}, []string{"example: outside the tree, and go.sum pins no version"}},
		{"an outside import that builds for darwin alone", map[string]string{
			"ok/x_darwin.go": "package ok\n\nimport \"example.org/fix/outside\"\n\nvar _ = outside.Y\n",
		}, []string{"darwin/amd64: example.org/fix/outside", "darwin/arm64: example.org/fix/outside"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The fixture is reached through a symbolic link, as the
			// temporary directory of macOS is (/var is /private/var), so a
			// rule that compares the paths go list resolves with the
			// path it was given turns red here too.
			top := filepath.Join(t.TempDir(), "link")
			require.NoError(t, os.Symlink(t.TempDir(), top))
			root := filepath.Join(top, "fix")
			for name, src := range base {
				writeFile(t, filepath.Join(root, name), src)
			}
			for name, src := range c.change {
				writeFile(t, filepath.Join(root, name), src)
			}
			writeFile(t, filepath.Join(top, "example", "go.mod"), "module example\n\ngo 1.24\n")
			writeFile(t, filepath.Join(top, "example", "e.go"), "package example\n\nconst Z = 3\n")
			got := judgeImports(t, root, "./cmd/ask-first", globs)
			if c.want == nil {
				assert.Empty(t, got)
				return
			}
			for _, w := range c.want {
				assert.True(t, slices.ContainsFunc(got, func(f string) bool { return strings.Contains(f, w) }), "%q in %q", w, got)
			}
			if strings.Contains(c.name, "darwin") {
				for _, f := range got {
					assert.True(t, strings.HasPrefix(f, "darwin/"), "a linux listing finds nothing: %q", f)
				}
			}
		})
	}
}

// TestPRImportsPins holds the go.sum rule on listed packages: a module
// outside the tree passes with the hash of its code, and fails with the
// go.mod hash alone, with another version, without a version, and when
// its replacement is not pinned.
func TestPRImportsPins(t *testing.T) {
	const sum = "example.com/m v1.0.0 h1:abc=\nexample.com/m v1.0.0/go.mod h1:def=\nexample.com/g v1.0.0/go.mod h1:ghi=\nexample.com/r v2.0.0 h1:jkl=\n"
	outside := func(m *listedModule) listedPackage {
		return listedPackage{ImportPath: "p", Dir: "/elsewhere/p", Module: m, GoFiles: []string{"p.go"}}
	}
	cases := []struct {
		name string
		pkg  listedPackage
		ok   bool
	}{
		{"pinned", outside(&listedModule{Path: "example.com/m", Version: "v1.0.0"}), true},
		{"the go.mod hash alone", outside(&listedModule{Path: "example.com/g", Version: "v1.0.0"}), false},
		{"another version", outside(&listedModule{Path: "example.com/m", Version: "v1.0.1"}), false},
		{"no module", outside(nil), false},
		{"the main module outside the tree", outside(&listedModule{Path: "fix", Main: true}), false},
		{"a pinned replacement", outside(&listedModule{Path: "example.com/m", Version: "v1.0.0", Replace: &listedModule{Path: "example.com/r", Version: "v2.0.0"}}), true},
		{"a replacement not pinned", outside(&listedModule{Path: "example.com/r", Version: "v2.0.0", Replace: &listedModule{Path: "example.com/m", Version: "v9.0.0"}}), false},
		{"a local replacement", outside(&listedModule{Path: "example.com/m", Version: "v1.0.0", Replace: &listedModule{Path: "../m"}}), false},
		{"the standard library", listedPackage{ImportPath: "fmt", Dir: "/goroot/src/fmt", Standard: true}, true},
		{"a package that does not load", listedPackage{ImportPath: "x", Dir: "/root/x", Error: &struct{ Err string }{"no Go files"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := outsideApprovals([]listedPackage{c.pkg}, "/root", []string{"**"}, []byte(sum))
			assert.Equal(t, c.ok, len(got) == 0, "%q", got)
		})
	}
}

// writeFile writes src at name, making its directory.
func writeFile(t *testing.T, name, src string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o755))
	require.NoError(t, os.WriteFile(name, []byte(src), 0o644))
}
