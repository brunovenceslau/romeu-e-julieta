// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package procgroup

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The call-site scan is a test of this package until T039 moves it into
// the callsites check of tools/ci, beside the I16 scan (decided on
// 2026-10-13).

// thisPackage is the directory of this package below the module root.
// Only files directly in it are admitted: a subpackage or a sibling
// whose name starts the same is not this package.
const thisPackage = "internal/procgroup"

// admittedTools are the directories, below the module root, whose files
// may still import os/exec and os/signal (toolImports): tools/ci and
// tools/mutants,
// whose own copy of the group rule and signal handler lives in
// tools/ci/procgroup and their mains until T105 switches them to this
// package and removes these two entries.
var admittedTools = []string{"tools/ci", "tools/mutants"}

// guardedImports are the packages whose import outside this package
// starts a subprocess or maps a signal around it: os/exec and its
// wrapper execabs, net/http/cgi, which runs a CGI program, plugin,
// which loads code the scan cannot read, and os/signal.
var guardedImports = []string{"os/exec", "golang.org/x/sys/execabs", "net/http/cgi", "plugin", "os/signal"}

// toolImports are the guarded imports the admitted tools may still use.
var toolImports = []string{"os/exec", "os/signal"}

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

// TestCallSites is the call-site scan of 04 4.1, Subprocesses, item 4,
// over this module: every subprocess starts here and nothing else maps
// a signal, and only a test file sets the seam of the timeout table.
func TestCallSites(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	module := modulePath(data)
	require.NotEmpty(t, module, "the module line of go.mod")
	findings, err := callSiteFindings(root, module, admittedTools)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// callSiteFixture is the tree of the positive control, written to a
// temporary directory, since a Go file under testdata would escape the
// linters. Each file outside procgroup tries one way around the rule;
// the rest is what the scan admits or leaves out.
var callSiteFixture = map[string]string{
	"internal/procgroup/run.go": `package procgroup

import (
	"os/exec"
	"os/signal"
	"syscall"
)

func Start(name string) error { return exec.Command(name).Start() }

func Notify() { signal.Ignore(); _ = syscall.Exec }

func WithTestTimeout() {}
`,
	"internal/fixture/exec.go": `package fixture

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"internal/fixture/seam.go": `package fixture

import pg "example.com/fixture/internal/procgroup"

func Shorten() { pg.WithTestTimeout() }
`,
	"internal/fixture/seam_test.go": `package fixture

import (
	"os/exec"
	"testing"

	"example.com/fixture/internal/procgroup"
)

func TestShorten(t *testing.T) {
	procgroup.WithTestTimeout()
	_ = exec.Command("true").Run()
}
`,
	"internal/fixture/other.go": `package fixture

import "example.com/fixture/internal/other"

func Other() { other.WithTestTimeout() }
`,
	"internal/fixture/signal.go": `package fixture

import "os/signal"

func Ignore() { signal.Ignore() }
`,
	"internal/fixture/start.go": `package fixture

import (
	sys "syscall"
	"os"

	"golang.org/x/sys/unix"
)

func Start() {
	_, _ = os.StartProcess("/bin/true", nil, nil)
	_, _ = sys.ForkExec("/bin/true", nil, nil)
	_ = sys.Exec("/bin/true", nil, nil)
	_ = unix.Exec("/bin/true", nil, nil)
	f := os.StartProcess
	_ = f
}
`,
	"internal/fixture/dot.go": `package fixture

import . "example.com/fixture/internal/procgroup"

func Shorten() { WithTestTimeout() }
`,
	"internal/fixture/cgo.go": `package fixture

// int system(const char*);
import "C"

func Run() { C.system(nil) }
`,
	"internal/fixture/linkname.go": `package fixture

import _ "unsafe"

//go:linkname forkExec syscall.forkExec
func forkExec()
`,
	"internal/procgroup/sub/sub.go": `package sub

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"internal/fixture/imports.go": `package fixture

import (
	_ "golang.org/x/sys/execabs"
	_ "net/http/cgi"
	_ "plugin"
)
`,
	"internal/fixture/raw.go": `package fixture

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func Raw(trap uintptr) {
	_, _, _ = syscall.Syscall(syscall.SYS_EXECVE, 0, 0, 0)
	_, _, _ = syscall.RawSyscall6(59, 0, 0, 0, 0, 0, 0)
	_, _, _ = unix.Syscall(unix.SYS_CLONE3, 0, 0, 0)
	_, _, _ = unix.RawSyscall(trap, 0, 0, 0)
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, 0, 0, 0)
	_ = unix.ForkExec
	f := syscall.Syscall6
	_ = f
}
`,
	"tools/cix/x.go": `package cix

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"tools/ci/http.go": `package ci

import _ "net/http/cgi"
`,
	"internal/procgroupx/x.go": `package procgroupx

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"tools/ci/ci.go": `package ci

import (
	"os/exec"
	"os/signal"
)

func Start(name string) error { signal.Ignore(); return exec.Command(name).Start() }
`,
	"internal/fixture/testdata/ignored.go": `package ignored

import "os/exec"
`,
	"internal/.hidden/ignored.go": `package ignored

import "os/exec"
`,
	"internal/_ignored/ignored.go": `package ignored

import "os/exec"
`,
}

// TestCallSiteFindings is the positive control of the scan, over
// callSiteFixture.
func TestCallSiteFindings(t *testing.T) {
	root := t.TempDir()
	for name, content := range callSiteFixture {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	findings, err := callSiteFindings(root, "example.com/fixture", admittedTools)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"internal/fixture/cgo.go: imports C, whose calls start programs past this package",
		"internal/fixture/dot.go: dot-imports example.com/fixture/internal/procgroup, which hides its calls from this scan",
		"internal/fixture/exec.go: imports os/exec outside internal/procgroup",
		"internal/fixture/imports.go: imports golang.org/x/sys/execabs outside internal/procgroup",
		"internal/fixture/imports.go: imports net/http/cgi outside internal/procgroup",
		"internal/fixture/imports.go: imports plugin outside internal/procgroup",
		"internal/fixture/linkname.go: has a go:linkname directive, which reaches past this package",
		"internal/fixture/raw.go: calls golang.org/x/sys/unix.ForkExec outside internal/procgroup",
		"internal/fixture/raw.go: calls golang.org/x/sys/unix.RawSyscall with a trap the scan cannot name",
		"internal/fixture/raw.go: calls golang.org/x/sys/unix.Syscall with SYS_CLONE3",
		"internal/fixture/raw.go: calls syscall.RawSyscall6 with a trap the scan cannot name",
		"internal/fixture/raw.go: calls syscall.Syscall with SYS_EXECVE",
		"internal/fixture/raw.go: takes syscall.Syscall6 as a value, which hides its trap",
		"internal/fixture/seam.go: sets procgroup.WithTestTimeout outside a test file",
		"internal/fixture/seam_test.go: imports os/exec outside internal/procgroup",
		"internal/fixture/signal.go: imports os/signal outside internal/procgroup",
		"internal/fixture/start.go: calls golang.org/x/sys/unix.Exec outside internal/procgroup",
		"internal/fixture/start.go: calls os.StartProcess outside internal/procgroup",
		"internal/fixture/start.go: calls os.StartProcess outside internal/procgroup",
		"internal/fixture/start.go: calls syscall.Exec outside internal/procgroup",
		"internal/fixture/start.go: calls syscall.ForkExec outside internal/procgroup",
		"internal/procgroup/sub/sub.go: imports os/exec outside internal/procgroup",
		"internal/procgroupx/x.go: imports os/exec outside internal/procgroup",
		"tools/ci/http.go: imports net/http/cgi outside internal/procgroup",
		"tools/cix/x.go: imports os/exec outside internal/procgroup",
	}, findings)
}

// moduleRoot returns the directory of the go.mod above this package.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "a go.mod above %s", dir)
		dir = parent
	}
}

// modulePath returns the path of the module line of a go.mod, or ""
// when it has none. A quoted path is not used in this repository.
func modulePath(gomod []byte) string {
	for line := range strings.Lines(string(gomod)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// callSiteFindings walks the Go files of the tree at root, a module
// whose path is module, and returns one line per breach, sorted. It
// leaves out this package's own files and what the go command leaves
// out: testdata, and directories whose name starts with "." or "_".
func callSiteFindings(root, module string, tools []string) ([]string, error) {
	var findings []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if path.Dir(rel) == thisPackage {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		findings = append(findings, fileFindings(rel, file, module+"/"+thisPackage, tools)...)
		return nil
	})
	slices.Sort(findings)
	return findings, err
}

// fileFindings returns the breaches of one file outside this package,
// whose path below the module root is rel and which imports this
// package as self.
func fileFindings(rel string, file *ast.File, self string, tools []string) []string {
	var findings []string
	names := map[string]string{} // the name a file gives an import, to its path
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(imported)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = imported
		_, starts := startCalls[imported]
		switch {
		case imported == "C":
			findings = append(findings, rel+": imports C, whose calls start programs past this package")
		case name == "." && (imported == self || starts || slices.Contains(guardedImports, imported)):
			findings = append(findings, rel+": dot-imports "+imported+", which hides its calls from this scan")
		case slices.Contains(guardedImports, imported) && !admittedTool(rel, imported, tools):
			findings = append(findings, rel+": imports "+imported+" outside "+thisPackage)
		}
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if strings.HasPrefix(c.Text, "//go:linkname") {
				findings = append(findings, rel+": has a go:linkname directive, which reaches past this package")
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
					findings = append(findings, rel+": calls "+imported+"."+fun.Sel.Name+" with "+finding)
				}
			}
			return true
		}
		sel, imported, ok := selectorOf(n, names)
		if !ok {
			return true
		}
		switch {
		case imported == self && sel.Sel.Name == "WithTestTimeout" && !isTest:
			findings = append(findings, rel+": sets procgroup.WithTestTimeout outside a test file")
		case slices.Contains(startCalls[imported], sel.Sel.Name):
			findings = append(findings, rel+": calls "+imported+"."+sel.Sel.Name+" outside "+thisPackage)
		case slices.Contains(rawCalls[imported], sel.Sel.Name) && !judged[sel]:
			findings = append(findings, rel+": takes "+imported+"."+sel.Sel.Name+" as a value, which hides its trap")
		}
		return true
	})
	return findings
}

// selectorOf returns n as a selector on an imported package, with the
// path of that package.
func selectorOf(n ast.Node, names map[string]string) (*ast.SelectorExpr, string, bool) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, "", false
	}
	imported, ok := names[x.Name]
	return sel, imported, ok
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

// admittedTool reports whether rel, a file of an admitted tool, may
// still import the package.
func admittedTool(rel, imported string, tools []string) bool {
	return slices.Contains(toolImports, imported) && inside(rel, tools)
}

// inside reports whether the slash path rel is in one of the dirs.
func inside(rel string, dirs []string) bool {
	return slices.ContainsFunc(dirs, func(dir string) bool { return strings.HasPrefix(rel, dir+"/") })
}
