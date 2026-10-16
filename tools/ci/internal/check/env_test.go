// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package check_test

import (
	"bytes"
	"context"
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

// TestFindingLines checks how findings print: one per line, the path
// quoted when it does not print, and the summary line.
func TestFindingLines(t *testing.T) {
	findings := []check.Finding{
		{Where: "a.md:1", Rule: "em-dash", Msg: "a dash"},
		{Where: "b\x1b.md", Rule: "name", Msg: "a name"},
	}
	assert.Equal(t, "a.md:1: em-dash: a dash\n\"b\\x1b.md\": name: a name\n", check.Lines(findings))
	assert.Equal(t, "hygiene: ok", check.Summary("hygiene", 0))
	assert.Equal(t, "hygiene: 2 finding(s)", check.Summary("hygiene", len(findings)))
}

// TestFileInAndCommitOf checks the file of a tree read once, a
// submodule that is no file, and the commit a revision names.
func TestFileInAndCommitOf(t *testing.T) {
	files := []check.TreeFile{{Path: "a", Mode: "100644", Content: []byte("x")}, {Path: "sub", Mode: "160000"}}
	got, found, err := check.FileIn(files, "a")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "x", string(got))
	_, found, err = check.FileIn(files, "b")
	require.NoError(t, err)
	assert.False(t, found)
	_, _, err = check.FileIn(files, "sub")
	require.ErrorContains(t, err, "not a file")

	r := citest.NewTree(t)
	head := r.Commit("fixture")
	got2, err := check.CommitOf(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	assert.Equal(t, head, got2)
	_, err = check.CommitOf(t.Context(), r.Repo, "nonesuch")
	require.Error(t, err)
}

// TestTheRegistryOfTheBinary checks the registry the init of each
// check_<check>.go fills: what Register and RegisterCommand add is what
// Registered, Lookup and Usage give.
func TestTheRegistryOfTheBinary(t *testing.T) {
	run := func(context.Context, check.Env, []string) (bool, error) { return true, nil }
	check.RegisterCommand(check.Command{Name: "all", Usage: []string{"all"}, Run: run})
	check.Register(check.Check{Name: "format", Order: 10})
	check.Register(check.Check{Name: "reqs", Order: 120, Commands: []check.Command{{Name: "reqs", Usage: []string{"reqs [allocate]"}, Run: run}}})
	var names []string
	for _, c := range check.Registered() {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"format", "reqs"}, names)
	cmd, ok := check.Lookup("reqs")
	require.True(t, ok)
	assert.Equal(t, "reqs", cmd.Name)
	assert.Equal(t, "usage:\n  go run ./tools/ci all\n  go run ./tools/ci reqs [allocate]\n", check.Usage())
}
