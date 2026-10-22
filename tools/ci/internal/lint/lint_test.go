// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks the lint steps: one per target, in the order of
// Targets, each the linter with Args in the environment of its target;
// and, when a package is outside Allowed, the run of forbidigo over it
// after each one.
func TestSteps(t *testing.T) {
	root := t.TempDir()
	steps := Steps(t.Context(), check.Plan{Root: root, Tools: check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/bin"}})
	var want []check.Step
	for _, target := range Targets {
		want = append(want, check.Step{
			Name:    "lint " + target.GOOS + "/" + target.GOARCH,
			Argv:    slices.Concat([]string{"/pinned/golangci-lint"}, Args),
			Environ: slices.Concat(check.StepEnv("/pinned/bin"), []string{"GOOS=" + target.GOOS, "GOARCH=" + target.GOARCH, "CGO_ENABLED=0"}),
		})
	}
	require.Len(t, steps, 8, "a root with no package to judge keeps the boundary steps, failing")
	for i, target := range Targets {
		assert.Equal(t, want[i], steps[2*i])
		assert.Equal(t, "lint boundaries "+target.String(), steps[2*i+1].Name)
		require.Error(t, steps[2*i+1].Unavailable, "an empty list fails the step and does not drop it")
		assert.Nil(t, steps[2*i+1].Argv)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "cli"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "cli", "cli.go"), []byte("package cli\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "other"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "other", "other.go"), []byte("package other\n"), 0o600))
	steps = Steps(t.Context(), check.Plan{Root: root, Tools: check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/bin"}})
	require.Len(t, steps, 8)
	for i, target := range Targets {
		assert.Equal(t, "lint "+target.String(), steps[2*i].Name)
		assert.Equal(t, "lint boundaries "+target.String(), steps[2*i+1].Name)
		assert.Equal(t, []string{"/pinned/golangci-lint", "run", "--config", "tools/ci/internal/lint/forbidigo.yml", "-j", "1", "--allow-parallel-runners", "--build-tags", "e2e", "./internal/other"}, steps[2*i+1].Argv)
		assert.Equal(t, steps[2*i].Environ, steps[2*i+1].Environ)
	}

	steps = Steps(t.Context(), check.Plan{Root: filepath.Join(root, "missing"), Tools: check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/bin"}})
	require.Len(t, steps, 8, "a root that cannot be read leaves the boundary steps failing, not absent")
	require.Error(t, steps[1].Unavailable)
}

// TestBoundaryDirs holds the packages the second run judges: those with
// a Go file that are not in Allowed, and no testdata, hidden or nested
// module directory.
func TestBoundaryDirs(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"internal/cli/a.go", "internal/procgroup/a_test.go", "cmd/romeu/main.go", "tools/ci/main.go",
		"tools/ci/internal/lint/a.go", "internal/gitsafe/a.go", "internal/gitsafe/testdata/x/a.go",
		".hidden/a.go", "docs/readme.md", "other/go.mod", "other/a.go", "internal/cli/sub/a.go",
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(f)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte("package x\n"), 0o600))
	}
	dirs, err := BoundaryDirs(root)
	require.NoError(t, err)
	assert.Equal(t, []string{"./internal/cli/sub", "./internal/gitsafe", "./tools/ci/internal/lint"}, dirs)
}
