// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package vet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
)

// TestSteps checks the vet steps: go vet of the pinned go over the
// module once per target of lint.Targets, in the environment of the
// linter for that target.
func TestSteps(t *testing.T) {
	var want []check.Step
	for _, target := range lint.Targets {
		want = append(want, check.Step{
			Name:    "vet " + target.String(),
			Argv:    []string{"/pinned/bin/go", "vet", "./..."},
			Environ: lint.Env(target, "/pinned/bin"),
		})
	}
	assert.Equal(t, want, Steps(t.Context(), check.Plan{Root: "/src", Tools: check.Tools{GoDir: "/pinned/bin"}}))
}

// TestVetSeesCodeOfOneTargetAlone is the positive control for the targets:
// a file that builds on darwin alone and does not vet is found by the
// darwin step and by no linux one, with the real go command.
func TestVetSeesCodeOfOneTargetAlone(t *testing.T) {
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	dir := t.TempDir()
	for name, src := range map[string]string{
		"go.mod":      "module example.invalid/vetfixture\n\ngo 1.27\n",
		"a.go":        "package a\n",
		"b_darwin.go": "package a\n\nimport \"fmt\"\n\nfunc b() { fmt.Printf(\"%d\", \"not a number\") }\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600))
	}
	for _, step := range Steps(t.Context(), check.Plan{Root: dir, Tools: check.Tools{GoDir: filepath.Dir(goBin)}}) {
		out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
			Class: procgroup.ToolStep, Argv: step.Argv, Dir: dir, Env: step.Environ,
		})
		if strings.HasPrefix(step.Name, "vet darwin") {
			require.Error(t, err, "%s: %s", step.Name, out)
			assert.Contains(t, string(out), "b_darwin.go", step.Name)
		} else {
			require.NoError(t, err, "%s: %s", step.Name, out)
		}
	}
}
