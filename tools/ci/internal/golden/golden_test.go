// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package golden_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/golden"
)

// specRow is a test levels table with the Golden row, as 10 10.1 has it.
const specRow = "# 10\n\n| Level | Where | Runs | What |\n|---|---|---|---|\n" +
	"| Unit | `internal/**` | CI | units |\n" +
	"| Golden | `internal/render/testdata/`, `internal/gate/testdata/` | CI, dev | `REQ-0732` goldens |\n"

// tree returns files as the tree of a commit.
func tree(files map[string]string) []check.TreeFile {
	var out []check.TreeFile
	for p, c := range files {
		out = append(out, check.TreeFile{Path: p, Mode: "100644", Content: []byte(c)})
	}
	return out
}

// The Golden row of 10 10.1 names the golden directories, and a spec
// without it, or without the file, is an error rather than an empty
// list.
func TestDirs(t *testing.T) {
	got, err := golden.Dirs(tree(map[string]string{golden.SpecPath: specRow}))
	require.NoError(t, err)
	assert.Equal(t, []string{"internal/render/testdata/", "internal/gate/testdata/"}, got)

	_, err = golden.Dirs(tree(map[string]string{golden.SpecPath: "# 10\n"}))
	require.ErrorContains(t, err, "has no Golden row")
	_, err = golden.Dirs(tree(map[string]string{golden.SpecPath: "| Golden | none | CI | x |\n"}))
	require.ErrorContains(t, err, "names no directory")
	_, err = golden.Dirs(nil)
	require.ErrorContains(t, err, "is not in the tree")
}

// The committed spec holds the Golden row, with the render goldens in
// it.
func TestDirsOfThisRepository(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", filepath.FromSlash(golden.SpecPath)))
	require.NoError(t, err)
	got, err := golden.Dirs(tree(map[string]string{golden.SpecPath: string(data)}))
	require.NoError(t, err)
	assert.Contains(t, got, "internal/render/testdata/")
}

// A CI invocation that passes -update is a finding at its line; the
// same word in a test of tools/ci, in another file, or as part of a
// longer word is not.
func TestUpdateFindings(t *testing.T) {
	files := tree(map[string]string{
		".github/workflows/ci.yml": "jobs:\n  a:\n    steps:\n      - run: go test ./... -update\n      - run: go test -run X --update=true ./x\n" +
			"      - run: ./update-checker -updated\n      # never pass -update here\n      - run: go test -test.update ./x\n",
		".github/workflows/x64.yaml":       "      - run: 'go test \"-update\" ./...'\n",
		".github/workflows/README.md":      "go test -update\n",
		".github/workflows/env.yml":        "env:\n  FLAGS: x=-update\n  ARGS: [test,-update]\n  - run: go test $(echo -x)(-test.update)\n",
		".github/actions/gen/action.yml":   "runs:\n  steps:\n    - run: go test -update ./...\n      # -update is never passed\n",
		".github/actions/gen/run.sh":       "#!/bin/sh\n# no -update here\nexec go test --update\n",
		"tools/ci/internal/race/race.go":   "package race\n\nvar argv = []string{\"test\", \"-update\"}\nvar ok = []string{\"-updated\"}\n",
		"tools/ci/internal/race/x.go":      "package race\n\n// go test -update is never run here.\nvar v = []string{\"--update=1\"}\nvar line = \"go test -update ./...\"\nvar raw = `-test.update`\n",
		"tools/ci/internal/race/bad.go":    "package race\n\nfunc {\n",
		"tools/ci/internal/race/x_test.go": "package race\n\nvar argv = []string{\"-update\"}\n",
		"internal/render/render_test.go":   "var update = flag.Bool(\"update\", false, \"\")\n// go test -update\n",
	})
	var where []string
	for _, f := range golden.UpdateFindings(files) {
		where = append(where, f.Where)
		assert.Equal(t, "update-in-ci", f.Rule)
	}
	assert.ElementsMatch(t, []string{
		".github/workflows/ci.yml:4",
		".github/workflows/ci.yml:5",
		".github/workflows/ci.yml:8",
		".github/workflows/x64.yaml:1",
		".github/workflows/env.yml:2",
		".github/workflows/env.yml:3",
		".github/workflows/env.yml:4",
		".github/actions/gen/action.yml:3",
		".github/actions/gen/run.sh:3",
		"tools/ci/internal/race/race.go:3",
		"tools/ci/internal/race/x.go:4",
		"tools/ci/internal/race/x.go:5",
		"tools/ci/internal/race/x.go:6",
		"tools/ci/internal/race/bad.go:1",
	}, where)
}

// fixture returns a repository whose main holds the spec and one golden,
// with a branch that changes, adds and deletes goldens and a file
// outside them.
func fixture(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	r := gittest.New(t)
	r.Write(golden.SpecPath, specRow)
	r.Write("internal/render/testdata/env/a.yaml", "a: 1\n")
	r.Write("internal/gate/testdata/old.golden", "old\n")
	r.Commit("base")
	r.Git("checkout", "--quiet", "-b", "change")
	r.Write("internal/render/testdata/env/a.yaml", "a: 2\n")
	r.Write("internal/render/testdata/workspace/dev.code-workspace", "{}\n")
	require.NoError(t, os.Remove(filepath.Join(r.Dir, "internal", "gate", "testdata", "old.golden")))
	r.Write("internal/render/render.go", "package render\n")
	r.Commit("one")
	r.Write("internal/render/env.go", "package render\n")
	head := r.Commit("two")
	return r, head
}

// The goldens a change touches are listed against the merge base with
// origin's default branch, every commit of the branch counted.
func TestChangedAgainstTheMergeBase(t *testing.T) {
	r, head := fixture(t)
	r.Git("update-ref", "refs/remotes/origin/main", "main")
	got, note, err := golden.Changed(t.Context(), r.Repo, head, []string{"internal/render/testdata/", "internal/gate/testdata/"})
	require.NoError(t, err)
	assert.Empty(t, note)
	assert.Equal(t, []golden.Change{
		{Status: "D", Path: "internal/gate/testdata/old.golden"},
		{Status: "M", Path: "internal/render/testdata/env/a.yaml"},
		{Status: "A", Path: "internal/render/testdata/workspace/dev.code-workspace"},
	}, got)
}

// With no ref of origin, the base is the commit's parent; with no parent
// either, a note says that nothing is listed.
func TestChangedWithoutOrigin(t *testing.T) {
	r, head := fixture(t)
	got, note, err := golden.Changed(t.Context(), r.Repo, head, []string{"internal/render/testdata/"})
	require.NoError(t, err)
	assert.Contains(t, note, "listed against the parent only")
	assert.Empty(t, got, "the last commit touches no golden")

	r.Write("internal/render/testdata/env/a.yaml", "a: 3\n")
	head = r.Commit("three")
	got, note, err = golden.Changed(t.Context(), r.Repo, head, []string{"internal/render/testdata/"})
	require.NoError(t, err)
	assert.Contains(t, note, "listed against the parent only")
	assert.Equal(t, []golden.Change{{Status: "M", Path: "internal/render/testdata/env/a.yaml"}}, got)

	root := r.Git("rev-list", "--max-parents=0", "HEAD")
	got, note, err = golden.Changed(t.Context(), r.Repo, root, []string{"internal/render/testdata/"})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Contains(t, note, "the changed goldens are not listed")
}

// A commit that is its own merge base with origin, a push to main, has
// no base to compare against, and the note says so rather than "no
// golden changed".
func TestChangedOnTheDefaultBranch(t *testing.T) {
	r, head := fixture(t)
	r.Git("update-ref", "refs/remotes/origin/main", head)
	got, note, err := golden.Changed(t.Context(), r.Repo, head, []string{"internal/render/testdata/"})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Contains(t, note, "no base to compare against")
	assert.NotContains(t, golden.Info(got, note), "no golden changed")
}

// The listing names each change with its status, and says so when there
// is none or when it could not list.
func TestInfo(t *testing.T) {
	assert.Equal(t, "no golden changed\n", golden.Info(nil, ""))
	assert.Equal(t, "a note\n", golden.Info(nil, "a note"))
	assert.Equal(t, "a note\n1 changed golden(s), each to be listed under Evidence and reviewed:\n  M x\n",
		golden.Info([]golden.Change{{Status: "M", Path: "x"}}, "a note"))
	assert.Equal(t, "1 changed golden(s), each to be listed under Evidence and reviewed:\n  M \"a\\nb\"\n",
		golden.Info([]golden.Change{{Status: "M", Path: "a\nb"}}, ""))
}

// Judge lists the changed goldens under the verdict and, on a runner,
// in the job summary; a CI invocation with -update is a finding.
func TestJudge(t *testing.T) {
	r, head := fixture(t)
	r.Git("update-ref", "refs/remotes/origin/main", "main")
	summary := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summary)
	files, err := check.HeadTree(t.Context(), r.Repo, head)
	require.NoError(t, err)
	files = append(files, check.TreeFile{Path: ".github/workflows/ci.yml", Mode: "100644", Content: []byte("- run: go test -update ./...\n")})

	vs, err := golden.Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Files: files})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.NoError(t, vs[0].Err)
	assert.Len(t, vs[0].Findings, 1)
	assert.Contains(t, vs[0].Info, "3 changed golden(s)")
	got, err := os.ReadFile(summary)
	require.NoError(t, err)
	assert.Equal(t, "### Changed goldens\n\n"+
		"- D `internal/gate/testdata/old.golden`\n"+
		"- M `internal/render/testdata/env/a.yaml`\n"+
		"- A `internal/render/testdata/workspace/dev.code-workspace`\n", string(got))

	vs, err = golden.Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Files: files[len(files)-1:]})
	require.NoError(t, err)
	require.NoError(t, vs[0].Err)
	assert.Len(t, vs[0].Findings, 1, "a tree without the spec still has its invocations judged")
	assert.Contains(t, vs[0].Info, "is not in the tree")

	broken := append([]check.TreeFile{{Path: golden.SpecPath, Mode: "100644", Content: []byte("# 10\n")}}, files[len(files)-1:]...)
	vs, err = golden.Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Files: broken})
	require.NoError(t, err)
	require.ErrorContains(t, vs[0].Err, "has no Golden row")

	t.Setenv("GITHUB_STEP_SUMMARY", filepath.Join(t.TempDir(), "absent", "summary.md"))
	vs, err = golden.Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Files: files})
	require.NoError(t, err)
	require.ErrorContains(t, vs[0].Err, "the job summary")
}

// tools/ci golden judges the commit at HEAD: it passes and lists the
// changed goldens, fails on a workflow that passes -update, and takes no
// argument.
func TestRun(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	r, _ := fixture(t)
	r.Git("update-ref", "refs/remotes/origin/main", "main")
	var out bytes.Buffer
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdout: &out, Stderr: &out}
	ok, err := golden.Run(t.Context(), e, nil)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Contains(t, out.String(), "3 changed golden(s)")
	assert.Contains(t, out.String(), "golden: ok")

	r.Write(".github/workflows/ci.yml", "- run: go test -update ./...\n")
	r.Commit("rewrite in CI")
	out.Reset()
	ok, err = golden.Run(t.Context(), e, nil)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, out.String(), ".github/workflows/ci.yml:1: update-in-ci")

	_, err = golden.Run(t.Context(), e, []string{"x"})
	require.ErrorContains(t, err, "takes no argument")

	r.Write(golden.SpecPath, "# 10\n")
	r.Commit("no Golden row")
	_, err = golden.Run(t.Context(), e, nil)
	require.ErrorContains(t, err, "has no Golden row")
}

// The job summary carries the note too, so a list against the parent
// alone never reads as the whole change.
func TestJudgeSummaryCarriesTheNote(t *testing.T) {
	r, head := fixture(t)
	summary := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summary)
	files, err := check.HeadTree(t.Context(), r.Repo, head)
	require.NoError(t, err)
	vs, err := golden.Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Files: files})
	require.NoError(t, err)
	require.NoError(t, vs[0].Err)
	got, err := os.ReadFile(summary)
	require.NoError(t, err)
	assert.Contains(t, string(got), "listed against the parent only")
	assert.NotContains(t, string(got), "None.")
}
