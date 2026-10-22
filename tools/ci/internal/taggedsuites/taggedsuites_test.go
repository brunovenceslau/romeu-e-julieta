// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package taggedsuites

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// TestSteps checks the go vet of each tagged suite of REQ-0832, with the
// pinned go and the environment of the steps, and that a suite whose
// directory does not exist is left out.
func TestSteps(t *testing.T) {
	root := t.TempDir()
	p := check.Plan{Root: root, Tools: check.Tools{GoDir: "/pinned/bin"}}
	assert.Empty(t, Steps(t.Context(), p), "no suite yet")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "e2e"), 0o755))
	e2e := check.Step{Name: "tagged suites e2e", Argv: []string{"/pinned/bin/go", "vet", "-tags", "e2e", "./e2e/..."}, Environ: check.StepEnv("/pinned/bin")}
	assert.Equal(t, []check.Step{e2e}, Steps(t.Context(), p))

	// A directory of a suite not in Suites gets no step.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "e2e", "host"), 0o755))
	assert.Equal(t, []check.Step{e2e}, Steps(t.Context(), p))
}

// TestTags pins the value the lint, fix, imports and license steps
// read: the tags of Suites, comma separated.
func TestTags(t *testing.T) {
	assert.Equal(t, "e2e", Tags())
	defer func(old []Suite) { Suites = old }(Suites)
	Suites = []Suite{{"a", "a"}, {"b", "b"}}
	assert.Equal(t, "a,b", Tags())
}
