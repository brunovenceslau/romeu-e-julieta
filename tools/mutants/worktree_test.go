// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// summaryCommit returns the commit id that ends the summary line.
func summaryCommit(t *testing.T, stdout string) string {
	t.Helper()
	out := strings.TrimSuffix(stdout, "\n")
	i := strings.LastIndex(out, " at ")
	require.GreaterOrEqual(t, i, 0, stdout)
	return out[i+len(" at "):]
}

// TestMutantsWorktree is the case -worktree exists for: a builder who
// has not committed. The test that kills double-plus is in an untracked
// file, which calls a helper added to a tracked file without staging
// it, which reads a constant of a file that is staged only and one of a
// tracked file that an ignore rule matches. Without -worktree the run
// sees none of them and the mutant survives; with it, the mutant is
// killed. An ignored test that would fail the baseline is left out.
// Neither run changes the real index, HEAD, a ref, the stash, a file of
// .git or of the working tree; the -worktree run adds loose objects
// only. Run again, -worktree makes the same snapshot commit.
func TestMutantsWorktree(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: double-plus, file: calc/calc.go, old: "x * 2", new: "x + 2", package: ./calc, run: ^TestDouble}
`)
	f.repo.Write("calc/kept.go", "package calc\n\nconst kept = 1\n")
	f.repo.Commit("kept")
	f.repo.Write(".gitignore", "calc/ignored_test.go\ncalc/kept.go\n")
	f.repo.Commit("ignore")
	f.repo.Write("calc/three.go", "package calc\n\n// Three is 3.\nconst Three = 3\n")
	f.repo.Git("add", "calc/three.go")
	f.repo.Write("calc/calc_test.go", fixtureCalcTest+"\nfunc three() int { return Three * kept }\n")
	f.repo.Write("calc/double_test.go", `package calc

import "testing"

func TestDoubleThree(t *testing.T) {
	if Double(three()) != 6 {
		t.Error("Double(3) is not 6")
	}
}
`)
	f.repo.Write("calc/ignored_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestDoubleIgnored(t *testing.T) { t.Fatal(\"an ignored file is in the run\") }\n")
	before := f.state(t)
	require.Contains(t, before.status, "?? calc/double_test.go")
	require.Contains(t, before.status, "A  calc/three.go")
	require.Contains(t, before.status, " M calc/calc_test.go")
	require.Contains(t, before.status, "!! calc/ignored_test.go")

	assert.Equal(t, exitFail, f.runArgs(), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "double-plus survived")
	assert.Contains(t, f.stderr.String(), "the run uses the commit "+before.head+" without them; -worktree runs them too")
	f.assertUntouched(t, before)

	f.stdout.Reset()
	f.stderr.Reset()
	assert.Equal(t, exitOK, f.runArgs("-worktree"), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "double-plus killed ./calc -run ^TestDouble fails")
	assert.Empty(t, f.stderr.String())
	snap := summaryCommit(t, f.stdout.String())
	assert.NotEqual(t, before.head, snap, "the run is at the snapshot, not HEAD")
	f.assertOnlyObjectsAdded(t, before)

	// The snapshot is a commit object no ref reaches, child of HEAD.
	assert.Equal(t, before.head, f.repo.Git("rev-parse", snap+"^"))
	assert.Empty(t, f.repo.Git("for-each-ref", "--contains", snap))

	f.stdout.Reset()
	assert.Equal(t, exitOK, f.runArgs("-worktree"), f.stderr.String())
	assert.Equal(t, snap, summaryCommit(t, f.stdout.String()), "the same tree on the same HEAD is the same snapshot")
}

// TestMutantsWorktreeClean checks that -worktree on a working tree that
// equals HEAD runs HEAD itself and makes no snapshot commit.
func TestMutantsWorktreeClean(t *testing.T) {
	f := newFixture(t, threeMutants)
	before := f.state(t)
	assert.Equal(t, exitFail, f.runArgs("-worktree"))
	assert.Contains(t, f.stdout.String(), "of 3 mutants at "+before.head+"\n")
	f.assertUntouched(t, before)
}

// TestMutantsWorktreeDeletion checks that a tracked file deleted on
// disk is not in the snapshot: HEAD holds a test that fails, so the run
// of HEAD stops at its baseline, and the run of the working tree, where
// the file is gone, does not.
func TestMutantsWorktreeDeletion(t *testing.T) {
	f := newFixture(t, threeMutants)
	f.repo.Write("calc/red_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestDoubleRed(t *testing.T) { t.Fatal(\"red\") }\n")
	f.repo.Commit("red")
	require.NoError(t, os.Remove(filepath.Join(f.repo.Dir, "calc", "red_test.go")))
	f.writeMutants(t, strings.ReplaceAll(threeMutants, "^TestDouble$", "^TestDouble"))
	before := f.state(t)
	assert.Equal(t, exitError, f.runArgs())
	assert.Contains(t, f.stderr.String(), "fails with no mutant")
	f.stdout.Reset()
	f.stderr.Reset()
	assert.Equal(t, exitFail, f.runArgs("-worktree"), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "clamp-low killed")
	f.assertOnlyObjectsAdded(t, before)
}

// TestMutantsWorktreeOnlyFile checks a mutant of a file that only the
// working tree holds, with its test: killed with -worktree, and a run
// that cannot proceed without it, as its target runs no test there.
func TestMutantsWorktreeOnlyFile(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: triple-plus, file: calc/triple.go, old: "x * 3", new: "x + 3", package: ./calc, run: ^TestTriple$}
`)
	f.repo.Write("calc/triple.go", "package calc\n\n// Triple is three times x.\nfunc Triple(x int) int {\n\treturn x * 3\n}\n")
	f.repo.Write("calc/triple_test.go", "package calc\n\nimport \"testing\"\n\nfunc TestTriple(t *testing.T) {\n\tif Triple(2) != 6 {\n\t\tt.Error(\"Triple(2) is not 6\")\n\t}\n}\n")
	before := f.state(t)
	assert.Equal(t, exitError, f.runArgs())
	assert.Contains(t, f.stderr.String(), "./calc -run ^TestTriple$ runs no test")
	f.stdout.Reset()
	f.stderr.Reset()
	assert.Equal(t, exitOK, f.runArgs("-worktree"), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "triple-plus killed ./calc -run ^TestTriple$ fails")
	f.assertOnlyObjectsAdded(t, before)
}

// TestMutantsWorktreeSplitIndex checks that a source repository with a
// split index gets no .git/sharedindex file from the private index.
func TestMutantsWorktreeSplitIndex(t *testing.T) {
	f := newFixture(t, threeMutants)
	f.repo.Git("config", "core.splitIndex", "true")
	f.repo.Git("update-index", "--split-index")
	f.repo.Write("calc/extra_test.go", "package calc\n")
	before := f.state(t)
	require.NotEmpty(t, sharedIndexFiles(before), "the fixture has a split index")
	assert.Equal(t, exitFail, f.runArgs("-worktree"), f.stderr.String())
	f.assertOnlyObjectsAdded(t, before)
}

func sharedIndexFiles(s repoState) []string {
	var out []string
	for name := range s.gitDir {
		if strings.HasPrefix(name, "sharedindex.") {
			out = append(out, name)
		}
	}
	return out
}

// TestMutantsWorktreeRefuses covers the trees a snapshot cannot record
// as they are: an entry marked skip-worktree, as a sparse checkout
// makes, one marked assume-unchanged, and an embedded repository, which
// git add records as a submodule. Each exits 2 and changes nothing but
// objects.
func TestMutantsWorktreeRefuses(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, f fixture)
		want  string
	}{
		{"skip-worktree", func(_ *testing.T, f fixture) {
			f.repo.Git("update-index", "--skip-worktree", "calc/calc_test.go")
		}, "calc/calc_test.go is marked skip-worktree or assume-unchanged"},
		{"assume-unchanged", func(_ *testing.T, f fixture) {
			f.repo.Git("update-index", "--assume-unchanged", "calc/calc_test.go")
		}, "calc/calc_test.go is marked skip-worktree or assume-unchanged"},
		{"an embedded repository", func(_ *testing.T, f fixture) {
			f.repo.Git("init", "--quiet", "calc/inner")
			f.repo.Git("-C", "calc/inner", "commit", "--quiet", "--allow-empty", "--message", "inner")
		}, "holds a submodule (or an embedded repository) at calc/inner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, threeMutants)
			tt.setup(t, f)
			before := f.state(t)
			assert.Equal(t, exitError, f.runArgs("-worktree"))
			assert.Contains(t, f.stderr.String(), tt.want)
			assert.Empty(t, f.stdout.String())
			f.assertOnlyObjectsAdded(t, before)
		})
	}
}

// TestMutantsHooksOff checks that no hook runs in the clone, not even
// one that a global core.hooksPath names: the clone and the checkout of
// a snapshot would run post-checkout.
func TestMutantsHooksOff(t *testing.T) {
	f := newFixture(t, threeMutants)
	home := ""
	for _, kv := range f.repo.Env {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			home = v
		}
	}
	require.NotEmpty(t, home)
	hooks := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	require.NoError(t, os.WriteFile(filepath.Join(hooks, "post-checkout"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[core]\n\thooksPath = "+hooks+"\n"), 0o600))
	f.repo.Write("calc/extra_test.go", "package calc\n")
	assert.Equal(t, exitFail, f.runArgs("-worktree"), f.stderr.String())
	assert.NoFileExists(t, marker, "a hook ran in the clone")
}
