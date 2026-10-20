// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package imports is the imports check of 10 10.2 (REQ-0839): the
// table of module boundaries of 10 10.7 over the packages that exist,
// the rule of REQ-0971 for tools/** and e2e/**, and the guards of I1,
// I2, I11 and I24 of 05 5.2. It reads the import graph from
// "go list -json", once for each target of lint.Targets, without build
// tags and with the tags of the tagged suites (tagSets), and judges the
// union, so a file that builds on one platform alone, or only in the
// e2e or host suite, is judged too.
//
// What it does not see: a file that builds only with cgo, since every
// listing runs with CGO_ENABLED=0, as the release build does; a file
// that builds only for a GOOS or GOARCH outside lint.Targets; and a
// build tag other than those of tagSets.
package imports

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// Package is one package of the module as the check reads it, every
// path relative to the root of the module ("internal/cli", "cmd/romeu");
// a package outside the module keeps its import path ("text/template").
type Package struct {
	Path string
	// Imports are the packages its non-test files import.
	Imports []string
	// TestImports are the packages its test files import, those of the
	// external test package included.
	TestImports []string
	// Deps are the packages it links, directly or through another,
	// test files left out: what "go list -deps" lists.
	Deps []string
}

// listed is the part of the output of "go list -json" the check reads.
type listed struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
	Deps         []string
	Module       *struct{ Path string }
	// Error and DepsErrors say that the package or one it imports does
	// not load: in JSON mode go list reports them here and exits 0.
	Error      *listError
	DepsErrors []*listError
}

// listError is one error of a package go list could not load.
type listError struct{ Err string }

// listFields are the fields of listed, for "go list -json=...".
const listFields = "ImportPath,Imports,TestImports,XTestImports,Deps,Module,Error,DepsErrors"

// tagSets are the build tags each listing runs with: none, and those of
// the tagged suites of 10 10.2, e2e and host, whose test files a listing
// without tags leaves out.
var tagSets = [][]string{nil, {"-tags", "e2e,host"}}

// Load lists the packages of the module at dir with the go command
// goBin, once for each of lint.Targets and each of tagSets, and returns
// their union, sorted by path. A package that does not load is an error.
func Load(ctx context.Context, goBin, dir string) ([]Package, error) {
	byPath := map[string]*Package{}
	for _, tags := range tagSets {
		for _, target := range lint.Targets {
			what := strings.Join(slices.Concat([]string{"go list"}, tags, []string{"for", target.String()}), " ")
			var stdout, stderr bytes.Buffer
			err := procgroup.Run(ctx, procgroup.Command{
				Class:  procgroup.ToolStep,
				Argv:   slices.Concat([]string{goBin, "list"}, tags, []string{"-json=" + listFields, "./..."}),
				Dir:    dir,
				Env:    lint.Env(target, filepath.Dir(goBin)),
				Stdout: &stdout,
				Stderr: &stderr,
			})
			if err != nil {
				return nil, fmt.Errorf("%s: %w\n%s", what, err, bytes.TrimSpace(stderr.Bytes()))
			}
			if err := merge(byPath, &stdout); err != nil {
				return nil, fmt.Errorf("%s: %w", what, err)
			}
		}
	}
	var pkgs []Package
	for _, p := range slices.Sorted(maps.Keys(byPath)) {
		pkgs = append(pkgs, *byPath[p])
	}
	return pkgs, nil
}

// merge decodes the stream of JSON objects "go list -json" prints and
// adds each package to byPath, the union of what each target lists.
func merge(byPath map[string]*Package, r io.Reader) error {
	dec := jsontext.NewDecoder(r)
	for {
		var l listed
		if err := json.UnmarshalDecode(dec, &l); errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		if l.Error != nil {
			return fmt.Errorf("%s: %s", l.ImportPath, l.Error.Err)
		}
		if len(l.DepsErrors) > 0 {
			return fmt.Errorf("%s: %s", l.ImportPath, l.DepsErrors[0].Err)
		}
		if l.Module == nil {
			return fmt.Errorf("%s belongs to no module", l.ImportPath)
		}
		rel := func(paths ...[]string) []string {
			var out []string
			for _, p := range slices.Concat(paths...) {
				out = append(out, relative(l.Module.Path, p))
			}
			return out
		}
		path := relative(l.Module.Path, l.ImportPath)
		p, ok := byPath[path]
		if !ok {
			p = &Package{Path: path}
			byPath[path] = p
		}
		p.Imports = union(p.Imports, rel(l.Imports))
		p.TestImports = union(p.TestImports, rel(l.TestImports, l.XTestImports))
		p.Deps = union(p.Deps, rel(l.Deps))
	}
}

// relative returns importPath relative to the module path, or itself
// when it is outside the module.
func relative(module, importPath string) string {
	if rest, ok := strings.CutPrefix(importPath, module+"/"); ok {
		return rest
	}
	return importPath
}

// union returns the sorted union of a and b without repeats.
func union(a, b []string) []string {
	out := slices.Concat(a, b)
	slices.Sort(out)
	return slices.Compact(out)
}

// internalName returns p below internal/, and false for a path outside it.
func internalName(p string) (string, bool) {
	return strings.CutPrefix(p, "internal/")
}

// rule names the findings of the check, one name per rule, each
// "imports <rule>".
const (
	ruleUnnamed  = "imports unnamed"
	ruleLink     = "imports link"
	ruleBinary   = "imports binary"
	ruleMustNot  = "imports must-not"
	ruleTools    = "imports tools"
	ruleTestOnly = "imports test-only"
	ruleI1       = "imports I1"
	ruleI2       = "imports I2"
	ruleI24      = "imports I24"
	ruleI11      = "imports I11"
)

// binaryPaths are the packages of the two binaries, by binary.
var binaryPaths = []struct {
	path string
	bin  Binaries
}{{"cmd/romeu", Romeu}, {"cmd/julieta", Julieta}}

// Judge returns the findings of the check over pkgs, sorted by place:
// the rows of 10 10.7 (rowFindings, binaryFindings, testOnlyFindings),
// the tools rule of REQ-0971 (toolFindings), and the guards of I1, I2,
// I24 and I11.
func Judge(pkgs []Package) []check.Finding {
	findings := slices.Concat(
		rowFindings(pkgs),
		binaryFindings(pkgs),
		testOnlyFindings(pkgs),
		toolFindings(pkgs),
		hostTerminalFindings(pkgs),
		hostHooksFindings(pkgs),
		hostMemWriteFindings(pkgs),
		renderFindings(pkgs),
	)
	slices.SortStableFunc(findings, func(a, b check.Finding) int { return strings.Compare(a.Where, b.Where) })
	return findings
}

// linkedBy returns the binaries that may link the package at p: those
// of its row for a package below internal/, and the binary itself for
// cmd/romeu and cmd/julieta. ok is false for any other package, which
// the rows do not govern, and for a package no row names.
func linkedBy(p string) (Binaries, bool) {
	switch p {
	case "cmd/romeu":
		return Romeu, true
	case "cmd/julieta":
		return Julieta, true
	}
	name, ok := internalName(p)
	if !ok {
		return 0, false
	}
	row, ok := RowOf(name)
	return row.LinkedBy, ok
}

// rowFindings holds every package below internal/ to the table of 10
// 10.7, column by column:
//   - the first: a package no row names fails (REQ-0972), a command
//     package following the row of its binary (REQ-0970, RowOf);
//   - the second: a package, cmd/romeu and cmd/julieta among them,
//     imports only packages that every binary that may link it may link
//     too, so the binary that links it links nothing its rows forbid
//     ("a package of the last row imports none of the second: romeu
//     links it"). Each import obeying this is what makes the whole
//     chain obey it, so direct imports are enough;
//   - the third: a package of a row links none of the packages the row
//     names, directly or through another.
//
// Test files are left out: no binary links them.
func rowFindings(pkgs []Package) []check.Finding {
	var findings []check.Finding
	for _, p := range pkgs {
		name, internal := internalName(p.Path)
		row, named := RowOf(name)
		if internal && !named {
			findings = append(findings, check.Finding{Where: p.Path, Rule: ruleUnnamed,
				Msg: "no row of the table of 10 10.7 names this package; place it in a row before it builds (REQ-0972)"})
			continue
		}
		mine, governed := linkedBy(p.Path)
		if !governed {
			continue
		}
		for _, imp := range p.Imports {
			theirs, ok := linkedBy(imp)
			if _, internalImp := internalName(imp); !ok || !internalImp {
				continue
			}
			if missing := mine &^ theirs; missing != 0 {
				findings = append(findings, check.Finding{Where: p.Path, Rule: ruleLink,
					Msg: fmt.Sprintf("imports %s, which %s may not link, and %s may link this package (10 10.7, second column)",
						imp, strings.Join(missing.Names(), " and "), strings.Join(mine.Names(), " and "))})
			}
		}
		if !internal {
			continue
		}
		for _, m := range row.MustNot {
			if slices.Contains(p.Deps, "internal/"+m) {
				findings = append(findings, check.Finding{Where: p.Path, Rule: ruleMustNot,
					Msg: fmt.Sprintf("links internal/%s, which the row of %s must not import (10 10.7, third column)", m, row.Req)})
			}
		}
	}
	return findings
}

// find returns the package at p in pkgs, or nil.
func find(pkgs []Package, p string) *Package {
	for i := range pkgs {
		if pkgs[i].Path == p {
			return &pkgs[i]
		}
	}
	return nil
}

// binaryFindings holds the link set of each binary to the second column
// of 10 10.7: every package below internal/ that cmd/romeu links, by any
// path, packages outside internal/ included, has romeu in the second
// column of its row, and the same for cmd/julieta. A row whose second
// column names no binary (tools/ci only, test packages only) is so
// refused in both. A binary the listing does not hold is a finding, so
// that no guard below passes because it had nothing to judge.
func binaryFindings(pkgs []Package) []check.Finding {
	var findings []check.Finding
	for _, b := range binaryPaths {
		host := find(pkgs, b.path)
		if host == nil {
			findings = append(findings, check.Finding{Where: b.path, Rule: ruleBinary,
				Msg: "the listing holds no such package, so its link set and the guards on it judge nothing (10 10.7)"})
			continue
		}
		for _, dep := range host.Deps {
			name, ok := internalName(dep)
			if !ok {
				continue
			}
			row, named := RowOf(name)
			if named && row.LinkedBy&b.bin == 0 {
				findings = append(findings, check.Finding{Where: b.path, Rule: ruleBinary,
					Msg: fmt.Sprintf("links %s, which the row of %s does not let %s link (10 10.7, second column)", dep, row.Req, b.bin.Names()[0])})
			}
		}
	}
	return findings
}

// testOnlyFindings holds every package of the module to the rows whose
// second column is "test packages only" (REQ-1248): it imports a
// package of such a row from a test file alone, never from a non-test
// file, so no binary and no tool links it.
func testOnlyFindings(pkgs []Package) []check.Finding {
	var findings []check.Finding
	for _, p := range pkgs {
		for _, imp := range p.Imports {
			name, ok := internalName(imp)
			if !ok {
				continue
			}
			if row, named := RowOf(name); named && row.TestOnly {
				findings = append(findings, check.Finding{Where: p.Path, Rule: ruleTestOnly,
					Msg: fmt.Sprintf("imports %s from a non-test file; only a test file imports it (%s)", imp, row.Req)})
			}
		}
	}
	return findings
}

// toolFindings holds tools/** and the packages under e2e/** to
// REQ-0971: each, its test files included, imports below internal/ only
// procgroup and internal/cmd/<binary>/all, plus what toolExtra lets its
// tree import. So tools/ci reaches a command package through
// internal/cmd/<binary>/all only.
func toolFindings(pkgs []Package) []check.Finding {
	var findings []check.Finding
	for _, p := range pkgs {
		if !underTree(p.Path, "tools") && !underTree(p.Path, "e2e") {
			continue
		}
		allowed := slices.Clone(toolImports)
		for tree, extra := range toolExtra {
			if underTree(p.Path, tree) {
				allowed = append(allowed, extra...)
			}
		}
		for _, imp := range union(p.Imports, p.TestImports) {
			name, ok := internalName(imp)
			if !ok || slices.Contains(allowed, name) {
				continue
			}
			findings = append(findings, check.Finding{Where: p.Path, Rule: ruleTools,
				Msg: fmt.Sprintf("imports %s directly; tools/** and e2e/** reach internal/ through internal/cmd/<binary>/all (REQ-0971)", imp)})
		}
	}
	return findings
}

// underTree reports whether p is tree or a package below it.
func underTree(p, tree string) bool {
	return p == tree || strings.HasPrefix(p, tree+"/")
}

// hostLinks returns a finding at cmd/romeu for each package of names
// (below internal/) that it links, naming the packages of its link set
// that import it, under rule and with why.
//
// A listing without cmd/romeu gives none here: binaryFindings reports it.
func hostLinks(pkgs []Package, names []string, rule, why string) []check.Finding {
	host := find(pkgs, "cmd/romeu")
	if host == nil {
		return nil
	}
	linked := append(slices.Clone(host.Deps), host.Path)
	var findings []check.Finding
	for _, name := range names {
		target := "internal/" + name
		if !slices.Contains(host.Deps, target) {
			continue
		}
		var by []string
		for _, p := range pkgs {
			if slices.Contains(linked, p.Path) && slices.Contains(p.Imports, target) {
				by = append(by, p.Path)
			}
		}
		findings = append(findings, check.Finding{Where: host.Path, Rule: rule,
			Msg: fmt.Sprintf("links %s, imported by %s: %s", target, strings.Join(by, ", "), why)})
	}
	return findings
}

// hostTerminalFindings is the guard of I1: "go list -deps ./cmd/romeu"
// holds none of internal/layout, internal/hooks and internal/mise, so
// romeu has no multiplexer or terminal code (05 5.2).
//
//romeu:invariant I1 guard
func hostTerminalFindings(pkgs []Package) []check.Finding {
	return hostLinks(pkgs, hostTerminal, ruleI1, "no pane or shell ever runs on the host, so romeu links no multiplexer or terminal code (I1)")
}

// hostHooksFindings is the guard of I2: cmd/romeu does not link
// internal/hooks, so the host never configures hooks (05 5.2).
//
//romeu:invariant I2 guard
func hostHooksFindings(pkgs []Package) []check.Finding {
	return hostLinks(pkgs, hostHooks, ruleI2, "the host never configures hooks (I2)")
}

// hostMemWriteFindings is the guard of I24 that 10 10.7 names: no
// package romeu links imports internal/memstore/write, so romeu reads
// the memory dirs only through the readers of memstore (05 5.2).
//
//romeu:invariant I24 guard
func hostMemWriteFindings(pkgs []Package) []check.Finding {
	return hostLinks(pkgs, hostMemWrite, ruleI24, "romeu reads the memory dirs only through the readers of internal/memstore (I24)")
}

// renderPath is the package the I11 rule judges.
const renderPath = "internal/render"

// renderFindings holds internal/render to I11: it links no
// text/template, so YAML and JSON are written only through marshallers.
// The package does not exist before its task lands; Notes says so, as
// the rule then judges nothing.
func renderFindings(pkgs []Package) []check.Finding {
	if p := find(pkgs, renderPath); p != nil && slices.Contains(p.Deps, renderForbidden) {
		return []check.Finding{{Where: p.Path, Rule: ruleI11,
			Msg: "links " + renderForbidden + ": YAML and JSON are written only through marshallers (I11)"}}
	}
	return nil
}

// Notes returns the lines the subcommand prints above its summary for a
// rule that judged nothing because its package is not in the listing
// yet: the I11 rule before internal/render exists. It is not a finding,
// since the check runs over the packages that exist.
func Notes(pkgs []Package) []string {
	if find(pkgs, renderPath) == nil {
		return []string{"imports: " + renderPath + " does not exist yet, so the I11 rule judged nothing"}
	}
	return nil
}

// Run is the "imports" subcommand: it lists the packages of the module
// with the go command that mise.lock locks and prints the findings of
// Judge.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("imports takes no argument\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	goBin, err := mise.WhichPinned(ctx, root, "go", "go", "go")
	if err != nil {
		return false, err
	}
	return report(ctx, e.Stdout, goBin, root)
}

// report loads the packages at dir with goBin, prints the findings and
// the summary line to out, and reports whether there were none.
func report(ctx context.Context, out io.Writer, goBin, dir string) (bool, error) {
	pkgs, err := Load(ctx, goBin, dir)
	if err != nil {
		return false, err
	}
	findings := Judge(pkgs)
	notes := ""
	for _, n := range Notes(pkgs) {
		notes += n + "\n"
	}
	check.Say(out, "%s%s%s\n", check.Lines(findings), notes, check.Summary("imports", len(findings)))
	return len(findings) == 0, nil
}

// Steps returns the step of all: the imports subcommand, run with the
// pinned go in the environment of the steps.
func Steps(_ context.Context, p check.Plan) []check.Step {
	return []check.Step{{
		Name:    "imports",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "run", "./tools/ci", "imports"},
		Environ: check.StepEnv(p.Tools.GoDir),
	}}
}
