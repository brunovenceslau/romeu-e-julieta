// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

import (
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
			findings := scanFixture(t, files, []string{"internal/x"}, nil, nil)
			require.Len(t, findings, 2, "both functions fail with an empty table: %v", whereMsgs(findings))
			for _, f := range findings {
				assert.Equal(t, ruleSite, f.Rule)
				assert.Contains(t, f.Msg, string(tt.kind))
				assert.Contains(t, f.Msg, "which the admitted-call-site table does not name")
			}

			admitted := scanFixture(t, files, []string{"internal/x"}, nil, []Site{{tt.kind, "internal/x", "Do"}})
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

// TestCallsitesTree runs the check over this module with the admitted
// table. The scan is proved not inert on the real code first: with an
// empty table it must find a call of every kind this module makes today.
func TestCallsitesTree(t *testing.T) {
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
