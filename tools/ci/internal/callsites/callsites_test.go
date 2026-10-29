// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

import (
	"go/ast"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

const fixtureModule = "example.com/fixture"

// writeTree writes the files, by slash path, into a temporary directory
// and returns it. A Go file under testdata would escape the linters, so
// the fixtures of this package live in the code.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return root
}

// whereMsgs returns the findings as "where: message", sorted.
func whereMsgs(findings []check.Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Where+": "+f.Msg)
	}
	slices.Sort(out)
	return out
}

// binaries are the two entry packages every fixture has: romeu links
// the packages in romeuImports, julieta those in julietaImports.
func binaries(romeuImports, julietaImports []string) map[string]string {
	entry := func(imports []string) string {
		var b strings.Builder
		b.WriteString("package main\n\nimport (\n")
		for _, i := range imports {
			b.WriteString("\t_ \"" + fixtureModule + "/" + i + "\"\n")
		}
		b.WriteString(")\n\nfunc main() {}\n")
		return b.String()
	}
	return map[string]string{
		"cmd/romeu/main.go":   entry(romeuImports),
		"cmd/julieta/main.go": entry(julietaImports),
	}
}

// siteOnly keeps the findings of the scan against the table. The scan
// of os/exec and raw system calls (procgroup.go) reports some of the
// same calls under its own rule.
func siteOnly(findings []check.Finding) []check.Finding {
	return slices.DeleteFunc(slices.Clone(findings), func(f check.Finding) bool { return f.Rule != ruleSite })
}

// scanFixture scans the files plus the two entry packages.
func scanFixture(t *testing.T, files map[string]string, romeuImports, julietaImports []string, table []Site) []check.Finding {
	t.Helper()
	all := binaries(romeuImports, julietaImports)
	maps.Copy(all, files)
	findings, err := Scan(writeTree(t, all), fixtureModule, table)
	require.NoError(t, err)
	return findings
}

// TestCallsitesListedCallsOutsideTheTable is the positive control of
// the scan of I16 (REQ-0479): one package romeu links calls each listed
// kind, and each call fails because the table names no function of that
// package. The admitted cases then prove the scan is not a rule that
// fails everything: the same call passes in the function a row names
// and fails beside it.
func TestCallsitesListedCallsOutsideTheTable(t *testing.T) {
	tests := []struct {
		name    string
		imports string
		body    string
		kind    Kind
	}{
		{"os.Remove", `"os"`, `_ = os.Remove("x")`, KindRemove},
		{"os.RemoveAll", `"os"`, `_ = os.RemoveAll("x")`, KindRemove},
		{"unix.Unlink", `"golang.org/x/sys/unix"`, `_ = unix.Unlink("x")`, KindRemove},
		{"Root.RemoveAll", `"os"`, `var r *os.Root; _ = r.RemoveAll("x")`, KindRemove},
		{"os.Remove as a value", `"os"`, `f := os.Remove; _ = f`, KindRemove},
		{"os.Rename", `"os"`, `_ = os.Rename("a", "b")`, KindRename},
		{"Root.Rename", `"os"`, `var r *os.Root; _ = r.Rename("a", "b")`, KindRename},
		{"renameio.WriteFile", `"github.com/google/renameio/v2"`, `_ = renameio.WriteFile("a", nil, 0o600)`, KindRename},
		{"CloseAtomicallyReplace", `"github.com/google/renameio/v2"`, `var p *renameio.PendingFile; _ = p.CloseAtomicallyReplace()`, KindRename},
		{"O_TRUNC", `"os"`, `_, _ = os.OpenFile("a", os.O_WRONLY|os.O_TRUNC, 0o600)`, KindTruncate},
		{"update-ref -d", ``, `run("update-ref", "-d", "refs/x")`, KindUpdateRefDelete},
		{"update-ref --delete", ``, `run("-C", "d", "update-ref", "--delete", "refs/x")`, KindUpdateRefDelete},
		{"branch -D", ``, `run("branch", "-D", "x")`, KindBranchDelete},
		{"update-ref cluster", ``, `run("update-ref", "-md", "refs/x")`, KindUpdateRefDelete},
		{"branch cluster", ``, `run("branch", "-fD", "x")`, KindBranchDelete},
		{"branch cluster with r", ``, `run("branch", "-Dr", "x")`, KindBranchDelete},
		{"os.Truncate", `"os"`, `_ = os.Truncate("a", 0)`, KindTruncate},
		{"syscall.Ftruncate", `"syscall"`, `_ = syscall.Ftruncate(3, 0)`, KindTruncate},
		{"unix.Truncate", `"golang.org/x/sys/unix"`, `_ = unix.Truncate("a", 0)`, KindTruncate},
		{"renameio v1 WriteFile", `"github.com/google/renameio"`, `_ = renameio.WriteFile("a", nil, 0o600)`, KindRename},
		{"syscall.Syscall", `"syscall"`, `_, _, _ = syscall.Syscall(87, 0, 0, 0)`, KindSyscall},
		{"syscall.RawSyscall6", `"syscall"`, `_, _, _ = syscall.RawSyscall6(87, 0, 0, 0, 0, 0, 0)`, KindSyscall},
		{"unix.Syscall", `"golang.org/x/sys/unix"`, `_, _, _ = unix.Syscall(unix.SYS_UNLINK, 0, 0, 0)`, KindSyscall},
		{"unix.RawSyscallNoError", `"golang.org/x/sys/unix"`, `_, _ = unix.RawSyscallNoError(0, 0, 0, 0)`, KindSyscall},
		{"gc --prune", ``, `_ = []string{"gc", "--prune=now"}`, KindGCPrune},
		{"sbx env rm", ``, `run("env", "rm", "--force", "dir")`, KindEnvRm},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package x\n\nimport " + tt.imports + "\n\nfunc run(args ...string) {}\n\nfunc Do() { " + tt.body + " }\n\nfunc Other() { " + tt.body + " }\n"
			if tt.imports == "" {
				src = strings.Replace(src, "import \n\n", "", 1)
			}
			files := map[string]string{"internal/x/x.go": src}
			findings := siteOnly(scanFixture(t, files, []string{"internal/x"}, nil, nil))
			require.Len(t, findings, 2, "both functions fail with an empty table: %v", whereMsgs(findings))
			for _, f := range findings {
				assert.Equal(t, ruleSite, f.Rule)
				assert.Contains(t, f.Msg, string(tt.kind))
				assert.Contains(t, f.Msg, "which the admitted-call-site table does not name")
			}

			admitted := siteOnly(scanFixture(t, files, []string{"internal/x"}, nil, []Site{{tt.kind, "internal/x", "Do"}}))
			require.Len(t, admitted, 1, "only Other is left: %v", whereMsgs(admitted))
			assert.Contains(t, admitted[0].Msg, "in Other of internal/x")
		})
	}
}

// TestCallsitesAdmissionIsByPackageAndFunction holds the shape of the
// table (REQ-0481): the package, the function and the kind must all
// match, a method is named "Type.name", and a comment admits nothing.
func TestCallsitesAdmissionIsByPackageAndFunction(t *testing.T) {
	files := map[string]string{
		"internal/x/x.go": `package x

import "os"

type T struct{}

func (t *T) Drop() { _ = os.Remove("a") }

//callsites:admit I16
func Marked() { _ = os.Remove("a") }

var level = os.Remove

func Keep() { _ = os.Rename("a", "b") }
`,
		"internal/y/y.go": `package y

import "os"

func Drop() { _ = os.Remove("a") }
`,
	}
	table := []Site{
		{KindRemove, "internal/x", "T.Drop"},
		{KindRemove, "internal/y", "Drop"},
		{KindRemove, "internal/x", "Keep"}, // the kind differs
		{KindRemove, "internal/x", ""},     // a package-level call is never admitted
	}
	findings := scanFixture(t, files, []string{"internal/x", "internal/y"}, nil, table)
	assert.Equal(t, []string{
		"internal/x/x.go:10: os.Remove (os.Remove*) in Marked of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:12: os.Remove (os.Remove*) in package level of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:14: os.Rename (rename-over) in Keep of internal/x, which the admitted-call-site table does not name",
	}, whereMsgs(findings))

	// The same name in another package is another row.
	findings = scanFixture(t, files, []string{"internal/x", "internal/y"}, nil, []Site{{KindRemove, "internal/y", "Drop"}})
	assert.Len(t, findings, 4)
}

// TestCallsitesI16ReadsPackagesRomeuLinks holds the reach of the scan of
// I16: a package romeu links through another is read, a package only
// julieta links is not, and a test file is not.
func TestCallsitesI16ReadsPackagesRomeuLinks(t *testing.T) {
	files := map[string]string{
		"internal/shared/shared.go":      "package shared\n\nimport \"os\"\n\nfunc F() { _ = os.Remove(\"a\") }\n",
		"internal/shared/shared_test.go": "package shared\n\nimport \"os\"\n\nfunc G() { _ = os.Remove(\"a\") }\n",
		"internal/via/via.go":            "package via\n\nimport _ \"example.com/fixture/internal/shared\"\n",
		"internal/jonly/jonly.go":        "package jonly\n\nimport \"os\"\n\nfunc F() { _ = os.Remove(\"a\") }\n",
		"internal/orphan/orphan.go":      "package orphan\n\nimport \"os\"\n\nfunc F() { _ = os.Remove(\"a\") }\n",
	}
	findings := scanFixture(t, files, []string{"internal/via"}, []string{"internal/jonly"}, nil)
	assert.Equal(t, []string{
		"internal/shared/shared.go:5: os.Remove (os.Remove*) in F of internal/shared, which the admitted-call-site table does not name",
	}, whereMsgs(findings))
}

// TestCallsitesDotImports holds that an identifier of a dot-imported
// package counts as a call of that package, and that a selector on a
// value with the same name does not count twice.
func TestCallsitesDotImports(t *testing.T) {
	files := map[string]string{
		"internal/x/x.go": `package x

import (
	. "os"
	. "syscall"
)

func Drop() { _ = Remove("a"); _ = Rename("a", "b"); _ = Truncate("a", 0) }

func Flag() int { return O_TRUNC }

func Raw() { _, _, _ = Syscall(87, 0, 0, 0) }

func Quiet(r interface{ Name() string }) string { return r.Name() }
`,
	}
	findings := siteOnly(scanFixture(t, files, []string{"internal/x"}, nil, nil))
	assert.ElementsMatch(t, []string{
		"internal/x/x.go:10: os.O_TRUNC (dot-imported) (truncate) in Flag of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:12: syscall.Syscall (dot-imported) (raw syscall) in Raw of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:8: os.Remove (dot-imported) (os.Remove*) in Drop of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:8: os.Rename (dot-imported) (rename-over) in Drop of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:8: os.Truncate (dot-imported) (truncate) in Drop of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:8: syscall.Rename (dot-imported) (rename-over) in Drop of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:8: syscall.Truncate (dot-imported) (truncate) in Drop of internal/x, which the admitted-call-site table does not name",
		"internal/x/x.go:10: syscall.O_TRUNC (dot-imported) (truncate) in Flag of internal/x, which the admitted-call-site table does not name",
	}, whereMsgs(findings))
}

// TestCallsitesGitKindsOutsideTheSpec holds the edge of the listed git
// kinds: 05 5.2 lists update-ref -d, branch -D and gc --prune, so a tag
// delete, a push delete, a symbolic-ref delete, reflog expire, worktree
// remove and prune are not seen (the doc of the package says so), and a
// flag cluster that holds no deleting letter is not a finding.
func TestCallsitesGitKindsOutsideTheSpec(t *testing.T) {
	files := map[string]string{
		"internal/x/x.go": `package x

func run(args ...string) {}

func Unlisted() {
	run("tag", "-d", "v1")
	run("push", "--delete", "origin", "x")
	run("symbolic-ref", "--delete", "HEAD")
	run("reflog", "expire", "--all")
	run("worktree", "remove", "w")
	run("prune")
	run("branch", "-vv")
	run("branch", "--list", "-r")
	run("update-ref", "-m", "msg", "refs/x", "oid")
	run("gc", "--auto")
}
`,
	}
	assert.Empty(t, scanFixture(t, files, []string{"internal/x"}, nil, nil))
}

// TestCallsitesMethodsOfOtherTypesAreKnownFalseFindings holds a limit
// the scan states: it reads no types, so the Remove method of any value
// counts as a Remove of an os.Root. The fix for the code is a name that
// differs.
func TestCallsitesMethodsOfOtherTypesAreKnownFalseFindings(t *testing.T) {
	files := map[string]string{
		"internal/x/x.go": `package x

import "container/list"

func Drop(l *list.List, e *list.Element) { l.Remove(e) }
`,
	}
	findings := scanFixture(t, files, []string{"internal/x"}, nil, nil)
	assert.Equal(t, []string{
		"internal/x/x.go:5: .Remove (os.Remove*) in Drop of internal/x, which the admitted-call-site table does not name",
	}, whereMsgs(findings))
}

// TestCallsitesGhFixtures holds REQ-0500 and REQ-1060: a fixture package
// that starts gh fails, whether it calls os/exec or the runner, and a
// fixture package that calls os/exec outside procgroup fails.
func TestCallsitesGhFixtures(t *testing.T) {
	procgroupRun := `package procgroup

import (
	"context"
	"os/exec"
)

type Command struct{ Argv []string }

func Run(ctx context.Context, c Command) error {
	return exec.CommandContext(ctx, c.Argv[0]).Run()
}

func Elsewhere(ctx context.Context) error { return exec.CommandContext(ctx, "gh").Run() }
`
	files := map[string]string{
		"internal/procgroup/procgroup.go": procgroupRun,
		"internal/ghexec/ghexec.go":       "package ghexec\n\nimport \"os/exec\"\n\nfunc Start() error { return exec.Command(\"gh\").Run() }\n",
		"internal/ghrun/ghrun.go": `package ghrun

import (
	"context"

	"example.com/fixture/internal/procgroup"
)

func Start(ctx context.Context) error {
	return procgroup.Run(ctx, procgroup.Command{Argv: []string{"gh", "pr", "list"}})
}
`,
		"internal/sbxdrv/sbxdrv.go": `package sbxdrv

import (
	"context"

	"example.com/fixture/internal/procgroup"
)

func Start(ctx context.Context) error { return procgroup.Run(ctx, procgroup.Command{Argv: []string{"sbx"}}) }
`,
		"internal/mise/mise.go": `package mise

import (
	"context"

	"example.com/fixture/internal/procgroup"
)

func Start(ctx context.Context) error { return procgroup.Run(ctx, procgroup.Command{Argv: []string{"mise"}}) }
`,
	}
	table := []Site{
		{KindExec, "internal/procgroup", "Run"},
		{KindRunner, "internal/sbxdrv", AnyFunc},
		{KindRunner, "internal/mise", AnyFunc},
	}
	imports := []string{"internal/procgroup", "internal/ghexec", "internal/ghrun", "internal/sbxdrv"}
	findings := scanFixture(t, files, imports, []string{"internal/mise"}, table)
	assert.Equal(t, []string{
		"internal/ghexec/ghexec.go: imports os/exec outside internal/procgroup",
		"internal/ghexec/ghexec.go:5: os/exec.Command (exec.Command) in Start of internal/ghexec, which the admitted-call-site table does not name",
		"internal/ghrun/ghrun.go:10: procgroup.Run (procgroup runner) in Start of internal/ghrun, which the admitted-call-site table does not name",
		"internal/procgroup/procgroup.go:14: os/exec.CommandContext (exec.Command) in Elsewhere of internal/procgroup, which the admitted-call-site table does not name",
	}, whereMsgs(findings))
}

// TestCallsitesIgnoresWhatTheGoCommandIgnores holds the walk: testdata
// and directories that start with "." or "_" are left out.
func TestCallsitesIgnoresWhatTheGoCommandIgnores(t *testing.T) {
	src := "package x\n\nimport \"os/exec\"\n\nfunc F() { _ = exec.Command(\"gh\") }\n"
	files := map[string]string{
		"internal/x/testdata/x.go": src,
		"internal/.h/x.go":         src,
		"internal/_u/x.go":         src,
	}
	assert.Empty(t, scanFixture(t, files, nil, nil, nil))
}

// TestCallsitesNeedsBothBinaries holds that a module without the two
// entry packages is an error, not a pass.
func TestCallsitesNeedsBothBinaries(t *testing.T) {
	_, err := Scan(writeTree(t, map[string]string{"internal/x/x.go": "package x\n"}), fixtureModule, nil)
	assert.Error(t, err)
}

// TestCallsitesTreeIsNotInert proves the scan of the real code reads
// it: with a table that names only the os/exec call, it finds a call of
// every kind this module makes today.
func TestCallsitesTreeIsNotInert(t *testing.T) {
	root := citest.ModuleRoot(t)
	module, err := modulePath(root)
	require.NoError(t, err)

	bare, err := Scan(root, module, []Site{{KindExec, "internal/procgroup", "Run"}})
	require.NoError(t, err)
	for _, want := range []string{"(os.Remove*)", "(rename-over)", "(sbx env rm)", "(procgroup runner)"} {
		assert.True(t, slices.ContainsFunc(bare, func(f check.Finding) bool { return strings.Contains(f.Msg, want) }),
			"an empty table must fail a call of kind %s", want)
	}
}

// pendingTree are the call sites of this module that the table does not
// admit and 05 5.2 has no row for, each as "file message" without its
// line. They are reported to the operator, not admitted (T039); the
// list only shrinks, and the test below fails both on a new call site
// and on one of these that is gone or admitted, so it must then leave
// the list. An empty list is the end state: the tree with Admitted
// yields no finding.
var pendingTree = []string{
	"internal/cmd/romeu/init/init.go os.RemoveAll (os.Remove*) in run of internal/cmd/romeu/init, which the admitted-call-site table does not name",
	"internal/cmd/romeu/init/init.go .Rename (rename-over) in cloneConfig of internal/cmd/romeu/init, which the admitted-call-site table does not name",
	"internal/cmd/romeu/init/init.go .Remove (os.Remove*) in made.undo of internal/cmd/romeu/init, which the admitted-call-site table does not name",
	"internal/cmd/romeu/init/init.go os.Remove (os.Remove*) in made.undo of internal/cmd/romeu/init, which the admitted-call-site table does not name",
	"internal/gitsafe/clone.go os.RemoveAll (os.Remove*) in Runner.clone of internal/gitsafe, which the admitted-call-site table does not name",
	"internal/render/promote.go .Rename (rename-over) in ensureKitDir of internal/render, which the admitted-call-site table does not name",
	"internal/state/write.go .CloseAtomicallyReplace (rename-over) in writeFile of internal/state, which the admitted-call-site table does not name",
}

// TestCallsitesTreeWithAdmitted runs the check over this module with the
// admitted table: it yields no finding but the pending ones.
func TestCallsitesTreeWithAdmitted(t *testing.T) {
	root := citest.ModuleRoot(t)
	module, err := modulePath(root)
	require.NoError(t, err)

	findings, err := Scan(root, module, Admitted)
	require.NoError(t, err)
	var got []string
	for _, f := range findings {
		file, _, _ := strings.Cut(f.Where, ":")
		got = append(got, file+" "+f.Msg)
	}
	assert.ElementsMatch(t, pendingTree, got)
}

// rowExists reports whether the row names a function of a non-test file
// of its package in the tree; a row of AnyFunc needs the package to have
// one such file.
func rowExists(tr *tree, row Site) bool {
	for _, s := range tr.dirs[row.Package] {
		if s.isTest {
			continue
		}
		if row.Func == AnyFunc {
			return true
		}
		if slices.ContainsFunc(s.file.Decls, func(d ast.Decl) bool { return funcName(d) == row.Func }) {
			return true
		}
	}
	return false
}

// TestCallsitesAdmittedRowsExist holds that each row of the table names
// a function that exists, so a row outlives neither its function nor
// its package.
func TestCallsitesAdmittedRowsExist(t *testing.T) {
	root := citest.ModuleRoot(t)
	module, err := modulePath(root)
	require.NoError(t, err)
	tr, err := parseModule(root, module)
	require.NoError(t, err)

	for _, row := range Admitted {
		assert.True(t, rowExists(tr, row), "%s: %s in %s names no function of the tree", row.Kind, row.Func, row.Package)
	}
}

// TestCallsitesStaleRowFails is the positive control of the test above:
// a row for a function or a package that does not exist is found
// missing, and one that exists is found.
func TestCallsitesStaleRowFails(t *testing.T) {
	root := writeTree(t, map[string]string{
		"internal/x/x.go":      "package x\n\nfunc Real() {}\n\ntype T struct{}\n\nfunc (t *T) Method() {}\n",
		"internal/x/x_test.go": "package x\n\nfunc OnlyInTests() {}\n",
	})
	tr, err := parseModule(root, fixtureModule)
	require.NoError(t, err)
	assert.True(t, rowExists(tr, Site{KindRemove, "internal/x", "Real"}))
	assert.True(t, rowExists(tr, Site{KindRemove, "internal/x", "T.Method"}))
	assert.True(t, rowExists(tr, Site{KindRunner, "internal/x", AnyFunc}))
	assert.False(t, rowExists(tr, Site{KindRemove, "internal/x", "Gone"}))
	assert.False(t, rowExists(tr, Site{KindRemove, "internal/x", "OnlyInTests"}))
	assert.False(t, rowExists(tr, Site{KindRemove, "internal/gone", "Real"}))
}
