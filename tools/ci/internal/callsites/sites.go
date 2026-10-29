// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// ruleSite names the findings of the scan of call sites against the
// admitted-call-site table.
const ruleSite = "callsites table"

// procgroupPath is the directory of the runner below the module root.
const procgroupPath = thisPackage

// hit is one listed call a node makes.
type hit struct {
	kind Kind
	what string // the call as the finding names it
}

// removeNames, renameNames and truncNames are the functions, by package
// path, of the kinds that name a package function.
var (
	removeNames = map[string][]string{
		"os":                    {"Remove", "RemoveAll"},
		"syscall":               {"Unlink", "Unlinkat", "Rmdir"},
		"golang.org/x/sys/unix": {"Unlink", "Unlinkat", "Rmdir"},
	}
	renameNames = map[string][]string{
		"os":                            {"Rename"},
		"syscall":                       {"Rename", "Renameat"},
		"golang.org/x/sys/unix":         {"Rename", "Renameat", "Renameat2"},
		"github.com/google/renameio/v2": {"WriteFile", "Symlink"},
	}
	truncPackages = []string{"os", "syscall", "golang.org/x/sys/unix"}
)

// methodHits are the methods, on a receiver the scan cannot type, that
// do what a listed function does: the Remove, RemoveAll and Rename of
// an os.Root, and the atomic replace of a renameio pending file. A
// method of another type with the same name is a false finding, and the
// fix is a name that differs.
var methodHits = map[string]Kind{
	"Remove":                 KindRemove,
	"RemoveAll":              KindRemove,
	"Rename":                 KindRename,
	"CloseAtomicallyReplace": KindRename,
}

// runnerNames are the functions of internal/procgroup that start a
// subprocess.
var runnerNames = []string{"Run", "CombinedOutput"}

// execNames are the functions of os/exec the table of exec calls
// admits.
var execNames = []string{"Command", "CommandContext"}

// hitsOf returns the listed calls the node n makes. A selector counts
// whether it is called or taken as a value, so a listed function cannot
// hide in a variable; the string arguments of a call or a composite
// literal are read as an argv.
func hitsOf(n ast.Node, names map[string]string, module string) []hit {
	switch n := n.(type) {
	case *ast.SelectorExpr:
		return selectorHits(n, names, module)
	case *ast.CallExpr:
		return argvHits(n.Args)
	case *ast.CompositeLit:
		return argvHits(n.Elts)
	}
	return nil
}

// selectorHits judges one selector.
func selectorHits(sel *ast.SelectorExpr, names map[string]string, module string) []hit {
	name := sel.Sel.Name
	x, isIdent := sel.X.(*ast.Ident)
	imported, qualified := "", false
	if isIdent {
		imported, qualified = names[x.Name]
	}
	if !qualified {
		if kind, ok := methodHits[name]; ok {
			return []hit{{kind, "." + name}}
		}
		return nil
	}
	pkg := imported + "." + name
	var hits []hit
	switch {
	case slices.Contains(removeNames[imported], name):
		hits = append(hits, hit{KindRemove, pkg})
	case slices.Contains(renameNames[imported], name):
		hits = append(hits, hit{KindRename, pkg})
	case name == "O_TRUNC" && slices.Contains(truncPackages, imported):
		hits = append(hits, hit{KindTruncate, pkg})
	case imported == "os/exec" && slices.Contains(execNames, name):
		hits = append(hits, hit{KindExec, pkg})
	case imported == module+"/"+procgroupPath && slices.Contains(runnerNames, name):
		hits = append(hits, hit{KindRunner, "procgroup." + name})
	}
	return hits
}

// argvHits reads the string literals among args, in order, as the argv
// of a command: git or sbx. The literals are the direct children of
// the call or the literal only; an argv built from variables is not
// read.
func argvHits(args []ast.Expr) []hit {
	var words []string
	for _, a := range args {
		if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				words = append(words, s)
			}
		}
	}
	var hits []hit
	after := func(first string, match func(string) bool) bool {
		i := slices.Index(words, first)
		return i >= 0 && slices.ContainsFunc(words[i+1:], match)
	}
	if after("update-ref", func(w string) bool { return w == "-d" || w == "--delete" }) {
		hits = append(hits, hit{KindUpdateRefDelete, "update-ref -d"})
	}
	if after("branch", func(w string) bool { return w == "-D" || w == "-d" || w == "--delete" }) {
		hits = append(hits, hit{KindBranchDelete, "branch -D"})
	}
	if after("gc", func(w string) bool { return strings.HasPrefix(w, "--prune") }) {
		hits = append(hits, hit{KindGCPrune, "gc --prune"})
	}
	for i := 0; i+1 < len(words); i++ {
		if words[i] == "env" && words[i+1] == "rm" {
			hits = append(hits, hit{KindEnvRm, "sbx env rm"})
			break
		}
	}
	return hits
}

// funcName returns the name the table gives a declaration: "name", or
// "Type.name" for a method, and "" for a declaration that is no
// function.
func funcName(decl ast.Decl) string {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	typ := fn.Recv.List[0].Type
	for {
		switch t := typ.(type) {
		case *ast.StarExpr:
			typ = t.X
			continue
		case *ast.IndexExpr:
			typ = t.X
			continue
		case *ast.IndexListExpr:
			typ = t.X
			continue
		case *ast.Ident:
			return t.Name + "." + fn.Name.Name
		}
		return fn.Name.Name
	}
}

// admitted reports whether the table admits the kind in the function.
func admitted(table []Site, kind Kind, pkg, fn string) bool {
	return slices.ContainsFunc(table, func(s Site) bool {
		return s.Kind == kind && s.Package == pkg && fn != "" && (s.Func == AnyFunc || s.Func == fn)
	})
}

// siteFindings judges the non-test files of the directories in dirs
// for the kinds in kinds, against table.
func siteFindings(t *tree, dirs map[string]bool, kinds []Kind, table []Site) []check.Finding {
	var findings []check.Finding
	for dir := range dirs {
		for _, s := range t.dirs[dir] {
			if s.isTest {
				continue
			}
			names := importNames(s.file)
			for _, decl := range s.file.Decls {
				fn := funcName(decl)
				ast.Inspect(decl, func(n ast.Node) bool {
					for _, h := range hitsOf(n, names, t.module) {
						if !slices.Contains(kinds, h.kind) || admitted(table, h.kind, dir, fn) {
							continue
						}
						where := fn
						if where == "" {
							where = "package level"
						}
						findings = append(findings, check.Finding{
							Where: s.rel + ":" + strconv.Itoa(s.fset.Position(n.Pos()).Line),
							Rule:  ruleSite,
							Msg:   h.what + " (" + string(h.kind) + ") in " + where + " of " + dir + ", which the admitted-call-site table does not name",
						})
					}
					return true
				})
			}
		}
	}
	return findings
}
