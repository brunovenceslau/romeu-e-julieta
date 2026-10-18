// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package drift

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// TestTidy holds the findings of the tidy rule for a step that changes,
// creates or removes go.mod or go.sum, or leaves them as they were.
func TestTidy(t *testing.T) {
	tests := []struct {
		name string
		gen  check.Step
		want []string
	}{
		{"nothing changes", fsOps(), nil},
		{"the same content passes", fsOps("write go.mod module example.invalid/m"), nil},
		{"a changed go.mod fails", fsOps("write go.mod module example.invalid/other"), []string{"go.mod: tidy: go mod tidy changed this file"}},
		{"a changed go.sum fails", fsOps("write go.sum x"), []string{"go.sum: tidy: go mod tidy changed this file"}},
		{"a removed go.sum fails", fsOps("remove go.sum"), []string{"go.sum: tidy: go mod tidy removed this file"}},
		{"another file is no business of the rule", fsOps("write other.txt x"), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gittest.New(t)
			r.Write("go.mod", "module example.invalid/m")
			r.Write("go.sum", "")
			r.Git("add", ".")
			r.Git("commit", "-m", "fixture")
			findings, err := tidy(t.Context(), r.Repo, tt.gen)
			require.NoError(t, err)
			require.Len(t, findings, len(tt.want))
			for i, w := range tt.want {
				assert.Contains(t, findings[i].String(), w)
			}
		})
	}
}

// TestTidyCreatesGoSum holds that a go.sum that tidy writes is a finding.
func TestTidyCreatesGoSum(t *testing.T) {
	r := gittest.New(t)
	r.Write("go.mod", "module example.invalid/m")
	r.Git("add", ".")
	r.Git("commit", "-m", "fixture")
	findings, err := tidy(t.Context(), r.Repo, fsOps("write go.sum x"))
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].String(), "go.sum: tidy: go mod tidy created this file")
}

// TestTidyStepFails holds that a tidy that cannot run is an error, not a
// finding.
func TestTidyStepFails(t *testing.T) {
	r := gittest.New(t)
	_, err := tidy(t.Context(), r.Repo, fsOps("fail"))
	require.Error(t, err)
}

// TestTidyRunsTheRealCommand runs the pinned "go mod tidy", offline as in
// every step, in a module that requires a module nothing imports: the
// command drops the requirement, which is the finding, and a module that
// is tidy passes.
func TestTidyRunsTheRealCommand(t *testing.T) {
	root := citest.ModuleRoot(t)
	goSum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	require.NoError(t, err)
	tools := citest.PinnedLintTools(t, mise.Path, mise.Environ, mise.ResolveLintTools)
	step := check.Step{Name: "go mod tidy", Argv: []string{filepath.Join(tools.GoDir, "go"), "mod", "tidy"}, Environ: check.StepEnv(tools.GoDir)}

	for _, tt := range []struct {
		name, goMod, goSum string
		want               string // a finding that must exist; empty for none
	}{
		{"a tidy module", "module example.invalid/m\n\ngo 1.27.2\n", "", ""},
		{"an unused requirement", "module example.invalid/m\n\ngo 1.27.2\n\nrequire github.com/stretchr/testify v1.12.1\n", string(goSum), "go.mod: tidy: go mod tidy changed this file; run it and commit the result"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gittest.New(t)
			r.Write("go.mod", tt.goMod)
			if tt.goSum != "" {
				r.Write("go.sum", tt.goSum)
			}
			r.Write("m.go", "package m\n")
			r.Git("add", ".")
			r.Git("commit", "-m", "fixture")
			findings, err := tidy(t.Context(), r.Repo, step)
			require.NoError(t, err)
			if tt.want == "" {
				assert.Empty(t, findings)
				return
			}
			var got []string
			for _, f := range findings {
				got = append(got, f.String())
			}
			assert.Contains(t, got, tt.want)
		})
	}
}
