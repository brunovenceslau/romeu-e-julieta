// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

import (
	"go/ast"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// The scan of this file is the call-site scan of 04 4.1 (Subprocesses,
// item 4), which T137 landed as a test of internal/procgroup and T039
// moved here, beside the scan of I16 (decided on 2026-10-13). Its
// admitted package is internal/procgroup alone.

// ruleExec names the findings of this scan.
const ruleExec = "callsites os/exec"

// thisPackage is the directory of this package below the module root.
// Only files directly in it are admitted: a subpackage or a sibling
// whose name starts the same is not this package.
const thisPackage = "internal/procgroup"

// guardedImports are the packages whose import outside this package
// starts a subprocess or maps a signal around it: os/exec and its
// wrapper execabs, net/http/cgi, which runs a CGI program, go/build and
// go/importer, which start the go command with this process's
// environment, plugin, which loads code the scan cannot read, and
// os/signal.
var guardedImports = []string{
	"os/exec",
	"golang.org/x/sys/execabs",
	"net/http/cgi",
	"go/build",
	"go/importer",
	"plugin",
	"os/signal",
}

// toolsTree is the directory, below the module root, of the tools. Its
// packages start programs with the environment and the classes of the
// tools, as tools/ci/git does, and the scan reads a file's own imports
// only, so a file outside it that imported one would carry those rules
// into the product past the scan. A tool inside it may import one:
// tools/new runs git through tools/ci/git.
const toolsTree = "tools"

// testSeams are the functions of this package that only tests call:
// the timeout table's seam and the hook of Main's handler.
var testSeams = []string{"WithTestTimeout", "SetBeforeStepAside"}

// startCalls are the functions, by package path, that start or replace
// a process without os/exec; no file outside this package calls them.
var startCalls = map[string][]string{
	"os":                    {"StartProcess"},
	"syscall":               {"Exec", "ForkExec", "StartProcess"},
	"golang.org/x/sys/unix": {"Exec", "ForkExec"},
}

// rawCalls are the raw system calls, by package path, whose trap the
// scan reads: one named by a constant of startTraps starts a process,
// and one the scan cannot name, a number or a variable, may.
var rawCalls = map[string][]string{
	"syscall":               {"Syscall", "Syscall6", "RawSyscall", "RawSyscall6"},
	"golang.org/x/sys/unix": {"Syscall", "Syscall6", "RawSyscall", "RawSyscall6", "RawSyscallNoError", "SyscallNoError"},
}

// startTraps are the system calls that start or replace a process.
var startTraps = []string{"SYS_EXECVE", "SYS_EXECVEAT", "SYS_FORK", "SYS_VFORK", "SYS_CLONE", "SYS_CLONE3", "SYS_POSIX_SPAWN", "SYS_POSIX_SPAWNP"}

// fileFindings returns the breaches of one file outside this package,
// whose path below the module root is rel, in the module whose path is
// module.
func fileFindings(rel string, file *ast.File, module string) []string {
	var findings []string
	self := module + "/" + thisPackage
	toolsPackage := module + "/" + toolsTree
	inTools := inside(rel, []string{toolsTree})
	names := map[string]string{} // the name a file gives an import, to its path
	for _, spec := range file.Imports {
		imported, name, ok := importName(spec)
		if !ok {
			continue
		}
		names[name] = imported
		_, starts := startCalls[imported]
		switch {
		case imported == "C":
			findings = append(findings, "imports C, whose calls start programs past this package")
		case name == "." && (imported == self || starts || slices.Contains(guardedImports, imported)):
			findings = append(findings, "dot-imports "+imported+", which hides its calls from this scan")
		case slices.Contains(guardedImports, imported):
			findings = append(findings, "imports "+imported+" outside "+thisPackage)
		case !inTools && (imported == toolsPackage || inside(imported, []string{toolsPackage})):
			findings = append(findings,
				"imports "+imported+" outside "+toolsTree+"/, whose packages start programs past this package")
		}
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:linkname") {
				findings = append(findings, "has a go:linkname directive, which reaches past this package")
			}
		}
	}
	isTest := strings.HasSuffix(rel, "_test.go")
	judged := map[*ast.SelectorExpr]bool{} // the raw calls whose trap was read
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fun, imported, ok := selectorOf(call.Fun, names); ok && slices.Contains(rawCalls[imported], fun.Sel.Name) {
				judged[fun] = true
				if finding := trapFinding(call, names); finding != "" {
					findings = append(findings, "calls "+imported+"."+fun.Sel.Name+" with "+finding)
				}
			}
			return true
		}
		sel, imported, ok := selectorOf(n, names)
		if !ok {
			return true
		}
		switch {
		case imported == self && slices.Contains(testSeams, sel.Sel.Name) && !isTest:
			findings = append(findings, "sets procgroup."+sel.Sel.Name+" outside a test file")
		case slices.Contains(startCalls[imported], sel.Sel.Name):
			findings = append(findings, "calls "+imported+"."+sel.Sel.Name+" outside "+thisPackage)
		case slices.Contains(rawCalls[imported], sel.Sel.Name) && !judged[sel]:
			findings = append(findings, "takes "+imported+"."+sel.Sel.Name+" as a value, which hides its trap")
		}
		return true
	})
	return findings
}

// trapFinding judges the trap of a raw system call: "" for a constant
// of syscall or unix that starts no process, the constant's name for
// one that does, and a note for a trap the scan cannot name.
func trapFinding(call *ast.CallExpr, names map[string]string) string {
	if len(call.Args) > 0 {
		if sel, imported, ok := selectorOf(call.Args[0], names); ok && rawCalls[imported] != nil && strings.HasPrefix(sel.Sel.Name, "SYS_") {
			if slices.Contains(startTraps, sel.Sel.Name) {
				return sel.Sel.Name
			}
			return ""
		}
	}
	return "a trap the scan cannot name"
}

// execFindings judges every Go file of the module outside this
// package's own directory and returns one finding per breach.
func execFindings(t *tree) []check.Finding {
	var findings []check.Finding
	for dir, files := range t.dirs {
		if dir == thisPackage {
			continue
		}
		for _, s := range files {
			for _, msg := range fileFindings(s.rel, s.file, t.module) {
				findings = append(findings, check.Finding{Where: s.rel, Rule: ruleExec, Msg: msg})
			}
		}
	}
	return findings
}
