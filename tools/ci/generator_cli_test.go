// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
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
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// module is the path of the module.
const module = "github.com/brunovenceslau/romeu-e-julieta"

// goList runs the pinned go list with args in the module root for the
// target and returns its lines.
func goList(t *testing.T, goDir, root string, target lint.Target, args ...string) []string {
	t.Helper()
	var stdout, stderr strings.Builder
	err := procgroup.Run(t.Context(), procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   append([]string{filepath.Join(goDir, "go"), "list"}, args...),
		Dir:    root,
		Env:    lint.Env(target, goDir),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "go list for %s: %s", target, stderr.String())
	return strings.Split(strings.TrimSpace(stdout.String()), "\n")
}

// TestCommandLineImportBoundary holds the import boundary of REQ-1246 and
// REQ-1247 on each target: cobra is imported only by internal/cli,
// internal/cli/clidoc and the packages under internal/cmd/, cobra/doc
// only by internal/cli/clidoc, and that only by tools/ci; and what the
// binaries link holds none of internal/cli/clidoc, cobra/doc, go-md2man,
// blackfriday and mousetrap. Until cmd/romeu and cmd/julieta land (T009)
// the linked set is read from the packages their main.go imports:
// internal/cli and internal/cmd/<binary>/all, which links every command
// in. It reads the imports of the packages, not of their tests, which
// link into no binary: the tests of the generated check import
// internal/cli/clidoc to register the generators as tools/ci does.
func TestCommandLineImportBoundary(t *testing.T) {
	tools := citest.PinnedLintTools(t, mise.Path, mise.Environ, mise.ResolveLintTools)
	root := citest.ModuleRoot(t)
	allowed := map[string]func(string) bool{
		"github.com/spf13/cobra": func(p string) bool {
			return p == module+"/internal/cli" || p == module+"/internal/cli/clidoc" || strings.HasPrefix(p, module+"/internal/cmd/")
		},
		"github.com/spf13/cobra/doc":    func(p string) bool { return p == module+"/internal/cli/clidoc" },
		module + "/internal/cli/clidoc": func(p string) bool { return p == module+"/tools/ci" },
	}
	binaries := []string{"./internal/cli", "./internal/cmd/romeu/all", "./internal/cmd/julieta/all"}
	for _, dir := range []string{"cmd/romeu", "cmd/julieta"} {
		if _, err := os.Stat(filepath.Join(root, dir)); err == nil {
			binaries = append(binaries, "./"+dir)
		}
	}
	forbidden := []string{module + "/internal/cli/clidoc", "github.com/spf13/cobra/doc", "github.com/cpuguy83/go-md2man", "github.com/russross/blackfriday", "github.com/inconshreveable/mousetrap"}
	for _, target := range lint.Targets {
		var importers int
		for _, line := range goList(t, tools.GoDir, root, target, "-deps", "-f", "{{.ImportPath}}{{range .Imports}} {{.}}{{end}}", "./...") {
			fields := strings.Fields(line)
			pkg, imports := fields[0], fields[1:]
			if !strings.HasPrefix(pkg, module+"/") {
				// The rule governs the packages of this module.
				continue
			}
			for imported, ok := range allowed {
				if slices.Contains(imports, imported) {
					importers++
					assert.True(t, ok(pkg), "%s: %s imports %s (REQ-1246, REQ-1247)", target, pkg, imported)
				}
			}
		}
		assert.Positive(t, importers, "%s: the scan saw the importers of cobra", target)
		linked := goList(t, tools.GoDir, root, target, append([]string{"-deps", "-f", "{{.ImportPath}}"}, binaries...)...)
		require.Contains(t, linked, module+"/internal/cli", "%s: the binaries link internal/cli", target)
		require.Contains(t, linked, "github.com/spf13/cobra", "%s: the binaries link cobra", target)
		for _, pkg := range linked {
			for _, f := range forbidden {
				assert.False(t, pkg == f || strings.HasPrefix(pkg, f+"/"), "%s: a binary links %s (REQ-1246)", target, pkg)
			}
		}
	}
}
