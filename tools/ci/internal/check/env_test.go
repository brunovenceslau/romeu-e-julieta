// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestStatus checks the three exit statuses, and that the error is
// printed made safe.
func TestStatus(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, check.ExitOK, check.Status(&out, true, nil))
	assert.Equal(t, check.ExitFail, check.Status(&out, false, nil))
	assert.Empty(t, out.String())
	assert.Equal(t, check.ExitError, check.Status(&out, true, fmt.Errorf("x\x1b[2J%s", "\n")))
	assert.Equal(t, "ci: x\\x1b[2J\n", out.String())
}

// TestStatusOfAnInfrastructureError checks that an error that wraps
// ErrInfrastructure exits with the status of its own (REQ-0798), and
// is printed as any other error.
func TestStatusOfAnInfrastructureError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, check.ExitInfra, check.Status(&out, false, fmt.Errorf("all: %w", check.ErrInfrastructure)))
	assert.Contains(t, out.String(), "ci: all: ")
	assert.NotContains(t, []int{check.ExitOK, check.ExitFail, check.ExitError}, check.ExitInfra)
}

// TestRepoRoot checks that the root of the working tree is found from a
// directory below it, through the repository of the environment.
func TestRepoRoot(t *testing.T) {
	r := citest.NewTree(t)
	sub := filepath.Join(r.Dir, "docs")
	require.NoError(t, os.Mkdir(sub, 0o755))
	e := check.Env{Dir: sub, GitEnv: r.Env}
	assert.Equal(t, sub, e.Repo().Dir)
	root, err := check.RepoRoot(t.Context(), e)
	require.NoError(t, err)
	assert.Equal(t, citest.MustEval(t, r.Dir), citest.MustEval(t, root))
	_, err = check.RepoRoot(t.Context(), check.Env{Dir: t.TempDir(), GitEnv: append(r.Env, "GIT_CEILING_DIRECTORIES="+os.TempDir())})
	require.Error(t, err, "a directory outside a repository")
}
