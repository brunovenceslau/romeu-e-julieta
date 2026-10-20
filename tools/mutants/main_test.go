// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// pinnedGo is the go command the tests run in their clones, found once
// as main finds it, from the directory of this package.
var pinnedGo = sync.OnceValues(func() (string, error) {
	return resolveGo(context.Background(), ".")
})

// The module of every fixture: a package with one function its tests
// check (Clamp) and one they check too weakly (Double, where 2*2 and
// 2+2 agree). The go line is old, so any go builds it, and the module
// requires nothing, so it needs no go.sum and no module cache.
const (
	fixtureMod  = "module example.com/m\n\ngo 1.22\n"
	fixtureCalc = `package calc

// Clamp holds x between lo and hi.
func Clamp(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// Double is twice x.
func Double(x int) int {
	return x * 2
}
`
	fixtureCalcTest = `package calc

import "testing"

func TestClamp(t *testing.T) {
	for _, c := range [][4]int{{5, 0, 10, 5}, {-1, 0, 10, 0}, {11, 0, 10, 10}} {
		if got := Clamp(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("Clamp(%d, %d, %d) = %d, want %d", c[0], c[1], c[2], got, c[3])
		}
	}
}

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Error("Double(2) is not 4")
	}
}
`
)

// fixture is a repository with the module above committed, a mutants
// file outside it, and the env that runs against it.
type fixture struct {
	repo           *gittest.Repo
	e              env
	mutants        string // the path of the mutants file
	stdout, stderr *bytes.Buffer
}

func newFixture(t *testing.T, mutants string) fixture {
	t.Helper()
	goBin, err := pinnedGo()
	require.NoError(t, err)
	r := gittest.New(t)
	r.Write("go.mod", fixtureMod)
	r.Write("calc/calc.go", fixtureCalc)
	r.Write("calc/calc_test.go", fixtureCalcTest)
	r.Commit("calc")
	f := fixture{repo: r, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	f.mutants = filepath.Join(t.TempDir(), "mutants.yaml")
	require.NoError(t, os.WriteFile(f.mutants, []byte(mutants), 0o644))
	f.e = env{dir: r.Dir, gitEnv: r.Env, goBin: goBin, tmpDir: t.TempDir(), stdout: f.stdout, stderr: f.stderr}
	return f
}

// repoState is what a run must leave as it found it in the source
// repository: HEAD, every ref, the entries of the real index, what git
// status shows, every file below .git by its content (the index, the
// configuration and the packs among them) but the loose objects, which
// are apart, and every file of the working tree by its content. Status
// runs with --no-optional-locks, so reading the state writes nothing.
type repoState struct {
	head, refs, staged, status string
	gitDir                     map[string]string
	loose                      []string
	tree                       map[string]string
}

// looseObject matches the path of a loose object below .git.
var looseObject = regexp.MustCompile(`^objects/[0-9a-f]{2}/[0-9a-f]+$`)

func (f fixture) state(t *testing.T) repoState {
	t.Helper()
	s := repoState{
		head:   f.repo.Git("rev-parse", "HEAD"),
		refs:   f.repo.Git("for-each-ref"),
		staged: f.repo.Git("ls-files", "--stage"),
		status: f.repo.Git("--no-optional-locks", "status", "--porcelain", "--ignored", "--untracked-files=all"),
		gitDir: map[string]string{},
		tree:   map[string]string{},
	}
	gitDir := filepath.Join(f.repo.Dir, ".git")
	walkFiles(t, gitDir, func(rel, sum string) {
		if looseObject.MatchString(rel) {
			s.loose = append(s.loose, rel)
		} else {
			s.gitDir[rel] = sum
		}
	})
	walkFiles(t, f.repo.Dir, func(rel, sum string) {
		if rel != ".git" && !strings.HasPrefix(rel, ".git/") {
			s.tree[rel] = sum
		}
	})
	return s
}

// walkFiles calls fn with the slash path below root and the content of
// every file and symbolic link there: a hash of a file's bytes, or the
// target of a link, which is not followed.
func walkFiles(t *testing.T, root string, fn func(rel, sum string)) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			fn(filepath.ToSlash(rel), "link:"+target)
			return err
		}
		b, err := os.ReadFile(p)
		fn(filepath.ToSlash(rel), fmt.Sprintf("%x", sha256.Sum256(b)))
		return err
	}))
}

// assertUntouched checks that the source repository is as before, its
// object store included, has no stash, and that no temporary clone is
// left.
func (f fixture) assertUntouched(t *testing.T, before repoState) {
	t.Helper()
	assert.Equal(t, before, f.state(t))
	f.assertNoStashNoClone(t)
}

// assertOnlyObjectsAdded is assertUntouched for a -worktree run, which
// writes the objects of its snapshot: loose objects may be added, and
// nothing else changes, no pack and no file of .git included.
func (f fixture) assertOnlyObjectsAdded(t *testing.T, before repoState) {
	t.Helper()
	after := f.state(t)
	assert.Subset(t, after.loose, before.loose, "no loose object is removed")
	after.loose = before.loose
	assert.Equal(t, before, after)
	f.assertNoStashNoClone(t)
}

func (f fixture) assertNoStashNoClone(t *testing.T) {
	t.Helper()
	_, err := f.repo.Run(context.Background(), nil, "rev-parse", "--verify", "--quiet", "refs/stash")
	require.Error(t, err, "refs/stash must not exist")
	entries, err := os.ReadDir(f.e.tmpDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the clone is removed")
}

// writeMutants replaces the mutants file.
func (f fixture) writeMutants(t *testing.T, text string) {
	t.Helper()
	require.NoError(t, os.WriteFile(f.mutants, []byte(text), 0o644))
}

// runArgs runs the command against the fixture with args before the
// mutants file.
func (f fixture) runArgs(args ...string) int {
	return run(context.Background(), f.e, append(args, f.mutants))
}

const threeMutants = `mutants:
  - id: clamp-low
    file: calc/calc.go
    old: "x < lo"
    new: "x > lo"
    package: ./calc
    run: ^TestClamp$
  - id: double-plus
    file: calc/calc.go
    old: "x * 2"
    new: "x + 2"
    package: ./calc
    run: ^TestDouble$
  - id: every-return
    file: calc/calc.go
    old: "return"
    new: "return 0 +"
    package: ./calc
    run: ^TestClamp$
`

// TestMutantsEndToEnd is the acceptance case: one mutant killed, one
// survived and one invalid, in that order, a summary, exit 1, and the
// source repository untouched.
func TestMutantsEndToEnd(t *testing.T) {
	f := newFixture(t, threeMutants)
	before := f.state(t)
	code := run(context.Background(), f.e, []string{f.mutants})
	assert.Equal(t, exitFail, code, f.stderr.String())
	want := "clamp-low killed ./calc -run ^TestClamp$ fails\n" +
		"double-plus survived ./calc -run ^TestDouble$ passes\n" +
		"every-return invalid old occurs 4 times in calc/calc.go, not once\n" +
		"mutants: 1 killed, 1 survived, 1 invalid, of 3 mutants at " + before.head + "\n"
	assert.Equal(t, want, f.stdout.String())
	assert.Empty(t, f.stderr.String())
	f.assertUntouched(t, before)
}

// TestMutantsEveryMutantKilled checks exit 0, from a subdirectory of the
// repository and with the mutants file named relative to it.
func TestMutantsEveryMutantKilled(t *testing.T) {
	f := newFixture(t, threeMutants)
	sub := filepath.Join(f.repo.Dir, "calc")
	src, err := os.ReadFile(f.mutants)
	require.NoError(t, err)
	killedOnly, _, _ := strings.Cut(string(src), "  - id: double-plus")
	rel, err := filepath.Rel(sub, f.mutants)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(f.mutants, []byte(killedOnly), 0o644))
	f.e.dir = sub
	before := f.state(t)
	assert.Equal(t, exitOK, run(context.Background(), f.e, []string{rel}), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "mutants: 1 killed, 0 survived, 0 invalid, of 1 mutants")
	f.assertUntouched(t, before)
}

// TestMutantsInvalid covers each kind of invalid mutant: a build failure
// is no kill, and the file is restored before the next mutant runs, so
// the killable one after it is still killed. Old that overlaps itself
// ("xx" in "xxx") occurs twice, not once.
func TestMutantsInvalid(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: no-build, file: calc/calc.go, old: "x * 2", new: "x *", package: ./calc, run: ^TestDouble$}
  - {id: missing-old, file: calc/calc.go, old: "x / 2", new: "x", package: ./calc, run: ^TestDouble$}
  - {id: no-file, file: calc/none.go, old: "a", new: "b", package: ./calc, run: ^TestDouble$}
  - {id: a-directory, file: calc, old: "a", new: "b", package: ./calc, run: ^TestDouble$}
  - {id: other-package, file: other/other.go, old: "1", new: "x", package: ./calc, run: ^TestDouble$}
  - {id: overlapping, file: calc/testdata/x.txt, old: "xx", new: "y", package: ./calc, run: ^TestDouble$}
  - {id: clamp-low, file: calc/calc.go, old: "x < lo", new: "x > lo", package: ./calc, run: ^TestClamp$}
`)
	f.repo.Write("other/other.go", "package other\n\n// One is one.\nconst One = 1\n")
	f.repo.Write("calc/testdata/x.txt", "xxx\n")
	f.repo.Commit("other")
	before := f.state(t)
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}))
	lines := strings.Split(strings.TrimSuffix(f.stdout.String(), "\n"), "\n")
	require.Len(t, lines, 8, f.stdout.String())
	assert.Equal(t, "no-build invalid the mutated code does not build", lines[0])
	assert.Equal(t, "missing-old invalid old occurs 0 times in calc/calc.go, not once", lines[1])
	assert.Equal(t, "no-file invalid calc/none.go is not a file of the commit", lines[2])
	assert.Equal(t, "a-directory invalid calc is not a file of the commit", lines[3])
	assert.Equal(t, "other-package invalid the mutated code does not build", lines[4], "the package of the file is built too")
	assert.Equal(t, "overlapping invalid old occurs 2 times in calc/testdata/x.txt, not once", lines[5])
	assert.Equal(t, "clamp-low killed ./calc -run ^TestClamp$ fails", lines[6])
	assert.Contains(t, f.stderr.String(), "no-build: the build fails:")
	assert.Contains(t, f.stderr.String(), "other-package: the build fails:")
	f.assertUntouched(t, before)
}

// TestMutantsOutsideAPackage checks that a Go file in a directory that
// holds no package, such as testdata, is judged by the tests that read
// it and not as a build failure, and that a target of ./... runs.
func TestMutantsOutsideAPackage(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: data, file: calc/testdata/four.go, old: "4", new: "5", package: ./..., run: ^TestData$}
`)
	f.repo.Write("calc/testdata/four.go", "4\n")
	f.repo.Write("calc/data_test.go", `package calc

import (
	"os"
	"testing"
)

func TestData(t *testing.T) {
	b, err := os.ReadFile("testdata/four.go")
	if err != nil || string(b) != "4\n" {
		t.Error("testdata/four.go is not 4")
	}
}
`)
	f.repo.Commit("testdata")
	before := f.state(t)
	assert.Equal(t, exitOK, run(context.Background(), f.e, []string{f.mutants}), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "data killed ./... -run ^TestData$ fails")
	f.assertUntouched(t, before)
}

// TestMutantsLinkIsInvalid checks that a tracked symbolic link, whose
// blob is its target and not the content a mutant would change, is an
// invalid mutant.
func TestMutantsLinkIsInvalid(t *testing.T) {
	f := newFixture(t, "mutants:\n  - {id: link, file: calc/calc.lnk, old: \"x * 2\", new: \"x + 2\", package: ./calc, run: ^TestDouble$}\n")
	require.NoError(t, os.Symlink("calc.go", filepath.Join(f.repo.Dir, "calc", "calc.lnk")))
	f.repo.Commit("link")
	before := f.state(t)
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}))
	assert.Contains(t, f.stdout.String(), "link invalid calc/calc.lnk is not checked out as its blob")
	f.assertUntouched(t, before)
}

// TestMutantsRedBaseline checks that a target that fails with no mutant
// stops the run with exit 2 before any mutant is judged.
func TestMutantsRedBaseline(t *testing.T) {
	f := newFixture(t, threeMutants)
	f.repo.Write("calc/calc_test.go", strings.Replace(fixtureCalcTest, "!= 4", "!= 5", 1))
	f.repo.Commit("red")
	before := f.state(t)
	assert.Equal(t, exitError, run(context.Background(), f.e, []string{f.mutants}))
	assert.Empty(t, f.stdout.String())
	assert.Contains(t, f.stderr.String(), "the baseline of ./calc -run ^TestDouble$ fails with no mutant")
	assert.Contains(t, f.stderr.String(), "Double(2) is not 4")
	f.assertUntouched(t, before)
}

// TestMutantsUsesTheCommit checks that an uncommitted change is not part
// of the run, and that the run says so.
func TestMutantsUsesTheCommit(t *testing.T) {
	f := newFixture(t, threeMutants)
	// Uncommitted, this test would fail the baseline of TestDouble.
	f.repo.Write("calc/calc_test.go", strings.Replace(fixtureCalcTest, "!= 4", "!= 5", 1))
	before := f.state(t)
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}), f.stderr.String())
	assert.Contains(t, f.stderr.String(), "the working tree has uncommitted changes (git status: 1); the run uses the commit "+before.head+" without them; -worktree runs them too")
	assert.Contains(t, f.stdout.String(), "double-plus survived")
	f.assertUntouched(t, before)
}

// TestMutantsRefuses covers the errors that stop a run before any
// mutant: each exits 2, says why, and leaves no clone.
func TestMutantsRefuses(t *testing.T) {
	tests := []struct {
		name  string
		args  func(f fixture) []string
		setup func(t *testing.T, f *fixture)
		want  string
	}{
		{"no argument", func(fixture) []string { return nil }, nil, "usage:"},
		{"two arguments", func(f fixture) []string { return []string{f.mutants, f.mutants} }, nil, "usage:"},
		{"a flag", func(fixture) []string { return []string{"-h"} }, nil, "usage:"},
		{"an unknown flag", func(f fixture) []string { return []string{"-nope", f.mutants} }, nil, "usage:"},
		{"-worktree and no file", func(fixture) []string { return []string{"-worktree"} }, nil, "usage:"},
		{"a missing mutants file", func(f fixture) []string { return []string{f.mutants + ".none"} }, nil, "no such file"},
		{"a bad mutants file", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			require.NoError(t, os.WriteFile(f.mutants, []byte("mutants: []\n"), 0o644))
		}, "lists no mutant"},
		{"not a repository", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, ".git")))
		}, "git rev-parse"},
		{"no commit", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, ".git")))
			f.repo.Git("init", "--quiet", "--initial-branch=main")
		}, "has no commit HEAD to run"},
		{"a go that does not run", func(f fixture) []string { return []string{f.mutants} }, func(_ *testing.T, f *fixture) {
			f.e.goBin = filepath.Join(f.e.tmpDir, "none", "go")
		}, "none/go test"},
		{"a package that matches nothing", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			require.NoError(t, os.WriteFile(f.mutants, []byte(strings.ReplaceAll(threeMutants, "package: ./calc", "package: ./nope")), 0o644))
		}, "the baseline of ./nope ./calc does not build"},
		{"a baseline that does not build", func(f fixture) []string { return []string{f.mutants} }, func(_ *testing.T, f *fixture) {
			f.repo.Write("calc/broken.go", "package calc\n\nfunc (\n")
			f.repo.Commit("broken")
		}, "the baseline of ./calc does not build"},
		{"a package of a mutated file that does not build", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			f.repo.Write("other/other.go", "package other\n\nfunc (\n")
			f.repo.Commit("broken other")
			require.NoError(t, os.WriteFile(f.mutants, []byte(threeMutants+"  - {id: other, file: other/other.go, old: \"func (\", new: \"func ((\", package: ./calc, run: ^TestDouble$}\n"), 0o644))
		}, "the baseline of ./calc ./other does not build"},
		{"a submodule", func(f fixture) []string { return []string{f.mutants} }, func(_ *testing.T, f *fixture) {
			head := f.repo.Git("rev-parse", "HEAD")
			f.repo.Git("update-index", "--add", "--cacheinfo", "160000,"+head+",sub")
			f.repo.Git("commit", "--quiet", "--message", "submodule")
		}, "holds a submodule (or an embedded repository) at sub"},
		{"a run that matches no test", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			require.NoError(t, os.WriteFile(f.mutants, []byte(strings.ReplaceAll(threeMutants, "run: ^TestClamp$", "run: ^TestNope$")), 0o644))
		}, "./calc -run ^TestNope$ runs no test"},
		{"no temporary directory", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			t.Helper()
			f.e.tmpDir = filepath.Join(t.TempDir(), "none")
		}, "make the temporary directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, threeMutants)
			if tt.setup != nil {
				tt.setup(t, &f)
			}
			tmp := f.e.tmpDir
			assert.Equal(t, exitError, run(context.Background(), f.e, tt.args(f)))
			assert.Empty(t, f.stdout.String())
			assert.Contains(t, f.stderr.String(), tt.want)
			if entries, err := os.ReadDir(tmp); err == nil {
				assert.Empty(t, entries, "no clone is left")
			}
		})
	}
}

// TestMutantsInvalidLinks checks that a path through a tracked link to
// a directory is no file of the commit, and that a tracked link whose
// target leaves the tree is not read: both mutants are invalid.
func TestMutantsInvalidLinks(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: through-a-link, file: calclink/calc.go, old: "x * 2", new: "x + 2", package: ./calc, run: ^TestDouble$}
  - {id: out-of-the-tree, file: out.go, old: "x", new: "y", package: ./calc, run: ^TestDouble$}
`)
	outside := filepath.Join(t.TempDir(), "outside.go")
	require.NoError(t, os.WriteFile(outside, []byte("x\n"), 0o644))
	require.NoError(t, os.Symlink("calc", filepath.Join(f.repo.Dir, "calclink")))
	require.NoError(t, os.Symlink(outside, filepath.Join(f.repo.Dir, "out.go")))
	f.repo.Commit("links")
	before := f.state(t)
	assert.Equal(t, exitFail, f.runArgs())
	assert.Contains(t, f.stdout.String(), "through-a-link invalid calclink/calc.go is not a file of the commit\n")
	assert.Contains(t, f.stdout.String(), "out-of-the-tree invalid out.go is not checked out as its blob")
	f.assertUntouched(t, before)
}

// TestMutantsSameNamePackages checks that two packages of the same name
// build in one run: the target b/util imports a/util, whose file the
// mutant changes, and the mutant is killed, not invalid.
func TestMutantsSameNamePackages(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: one-is-zero, file: a/util/util.go, old: "return 1", new: "return 0", package: ./b/util, run: ^TestTwo$}
`)
	f.repo.Write("a/util/util.go", "package util\n\n// One is 1.\nfunc One() int {\n\treturn 1\n}\n")
	f.repo.Write("b/util/util.go", "package util\n\nimport au \"example.com/m/a/util\"\n\n// Two is 2.\nfunc Two() int {\n\treturn au.One() + 1\n}\n")
	f.repo.Write("b/util/util_test.go", "package util\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) {\n\tif Two() != 2 {\n\t\tt.Error(\"Two is not 2\")\n\t}\n}\n")
	f.repo.Commit("util")
	before := f.state(t)
	assert.Equal(t, exitOK, f.runArgs(), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "one-is-zero killed ./b/util -run ^TestTwo$ fails")
	f.assertUntouched(t, before)
}

// TestMutantsTestOnlyDirectory checks a directory with test files only:
// as a target, and as the place of the mutated file under a target of
// ./....
func TestMutantsTestOnlyDirectory(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: double-plus, file: calc/calc.go, old: "x * 2", new: "x + 2", package: ./tests, run: ^TestDoubleThree$}
  - {id: want-seven, file: tests/calc_test.go, old: "!= 6", new: "!= 7", package: ./..., run: ^TestDoubleThree$}
`)
	f.repo.Write("tests/calc_test.go", "package tests_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/calc\"\n)\n\nfunc TestDoubleThree(t *testing.T) {\n\tif calc.Double(3) != 6 {\n\t\tt.Error(\"Double(3) is not 6\")\n\t}\n}\n")
	f.repo.Commit("tests")
	before := f.state(t)
	assert.Equal(t, exitOK, f.runArgs(), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "double-plus killed ./tests -run ^TestDoubleThree$ fails")
	assert.Contains(t, f.stdout.String(), "want-seven killed ./... -run ^TestDoubleThree$ fails")
	f.assertUntouched(t, before)
}

// TestMutantsKilledWithoutAPassingTest covers the kills that are no
// failed assertion: a loop that the timeout ends, a panic in TestMain,
// and a test binary that panics as it starts.
func TestMutantsKilledWithoutAPassingTest(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: loops, file: calc/calc.go, old: "return x * 2", new: "for {\n\t}", package: ./calc, run: ^TestDouble$}
  - {id: main-panics, file: calc/main_test.go, old: "os.Exit(m.Run())", new: "panic(\"boom\")\n\tos.Exit(m.Run())", package: ./calc, run: ^TestDouble$}
  - {id: start-panics, file: calc/calc.go, old: "// Double is twice x.", new: "func init() { panic(\"start\") }\n\n// Double is twice x.", package: ./calc, run: ^TestDouble$}
`)
	f.repo.Write("calc/main_test.go", "package calc\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) {\n\tos.Exit(m.Run())\n}\n")
	f.repo.Commit("main")
	f.e.minTimeout = 2 * time.Second
	before := f.state(t)
	// A deadline of its own: should the loop not be ended by -timeout,
	// the run is cut short here, not at the timeout of the package.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	assert.Equal(t, exitOK, run(ctx, f.e, []string{f.mutants}), "the run must end before its deadline: %s", f.stderr.String())
	// The timeout grows with the baseline's duration, so a slow runner
	// sees more than the 2s floor.
	assert.Regexp(t, `(?m)^loops killed \./calc -run \^TestDouble\$ times out after [0-9]+s$`, f.stdout.String())
	assert.Contains(t, f.stdout.String(), "main-panics killed ./calc -run ^TestDouble$ fails\n")
	assert.Contains(t, f.stdout.String(), "start-panics killed ./calc -run ^TestDouble$ fails\n")
	f.assertUntouched(t, before)
}

// TestMutantsFewerTests checks that a mutant that renames or skips the
// test it is for is invalid, not survived: the target passes because
// the test no longer runs.
func TestMutantsFewerTests(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: renamed, file: calc/calc_test.go, old: "func TestDouble(", new: "func testDouble(", package: ./calc, run: ^TestDouble$}
  - {id: skipped, file: calc/calc_test.go, old: "if Double(2) != 4 {", new: "t.Skip()\n\tif Double(2) != 4 {", package: ./calc, run: ^TestDouble$}
`)
	before := f.state(t)
	assert.Equal(t, exitFail, f.runArgs(), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "renamed invalid ./calc -run ^TestDouble$ passes 0 tests, fewer than the 1 of the baseline")
	assert.Contains(t, f.stdout.String(), "skipped invalid ./calc -run ^TestDouble$ passes 0 tests, fewer than the 1 of the baseline")
	f.assertUntouched(t, before)
}

// TestMutantTimeout pins the -timeout of a mutant run: ten times the
// baseline, never under the least.
func TestMutantTimeout(t *testing.T) {
	assert.Equal(t, defaultMinTimeout, mutantTimeout(time.Second, 0))
	assert.Equal(t, 3*time.Second, mutantTimeout(100*time.Millisecond, 3*time.Second))
	assert.Equal(t, 50*time.Second, mutantTimeout(5*time.Second, 3*time.Second))
}

// TestCheckGoVersion checks that a go of another version is refused,
// and that the probe of resolveGo runs with GOTOOLCHAIN=local and
// GOENV=off.
func TestCheckGoVersion(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "go")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\necho go0.0.1\n"), 0o700))
	err := checkGoVersion(context.Background(), fake)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is go0.0.1, not "+runtime.Version())
	broken := filepath.Join(t.TempDir(), "go")
	require.NoError(t, os.WriteFile(broken, []byte("#!/bin/sh\nexit 3\n"), 0o700))
	require.Error(t, checkGoVersion(context.Background(), broken))
	_, err = resolveGo(context.Background(), filepath.Join(t.TempDir(), "none"))
	require.Error(t, err)

	probe := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(probe, "go"), []byte("#!/bin/sh\necho \"/$GOTOOLCHAIN-$GOENV\"\n"), 0o700))
	t.Setenv("PATH", probe+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOTOOLCHAIN", "go1.0")
	t.Setenv("GOENV", "/elsewhere")
	_, err = resolveGo(context.Background(), probe)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/local-off/bin/go")
}

// TestGoEnvIsBuiltFromNothing checks that a variable of this process
// that would change the build does not reach the go command, and that
// the pinned values and GOTMPDIR are set.
func TestGoEnvIsBuiltFromNothing(t *testing.T) {
	t.Setenv("GOFLAGS", "-run=Nothing")
	t.Setenv("GOTOOLCHAIN", "go1.0")
	t.Setenv("GOCACHE", "/cache")
	got := goEnv("/pinned/bin/go", "/run/go")
	assert.True(t, strings.HasPrefix(got[0], "PATH=/pinned/bin"+string(os.PathListSeparator)), got[0])
	assert.Contains(t, got, "GOCACHE=/cache")
	assert.Contains(t, got, "GOFLAGS=-mod=readonly")
	assert.Contains(t, got, "GOTOOLCHAIN=local")
	assert.Contains(t, got, "GOPROXY=off")
	assert.Contains(t, got, "GOTMPDIR=/run/go")
	assert.NotContains(t, got, "GOFLAGS=-run=Nothing")
	assert.NotContains(t, got, "GOTOOLCHAIN=go1.0")
	assert.NotContains(t, goEnv("/pinned/bin/go", ""), "GOTMPDIR=")
}

// TestRestoreErrors covers what restore refuses: a file it cannot write,
// a clone that still has a change after the write, and a clone where
// git status does not run.
func TestRestoreErrors(t *testing.T) {
	f := newFixture(t, threeMutants)
	root, err := os.OpenRoot(f.repo.Dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	r := &runner{clone: f.repo.Repo, root: root}
	err = r.restore("none/file.go", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restore none/file.go:")
	err = r.restore("calc/calc.go", []byte("package calc\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "the clone still has changes after calc/calc.go was restored")
	r.clone.Dir = t.TempDir()
	require.Error(t, r.restore("calc/calc.go", []byte(fixtureCalc)))
}

// TestInside checks that a path below dir is inside it, and that one
// outside it, or a dir or a path that does not exist, is not.
func TestInside(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
	assert.True(t, inside(dir, filepath.Join(dir, "sub")))
	assert.False(t, inside(filepath.Join(dir, "sub"), dir))
	assert.False(t, inside(filepath.Join(dir, "none"), dir))
	assert.False(t, inside(dir, filepath.Join(dir, "none")))
}

// stub writes an executable shell script named name, with body after
// its first line, to a directory of its own and returns its path. A
// program the body names is named by its path, so the stub runs
// whatever PATH holds.
func stub(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700))
	return path
}

// stall returns the body of a stub that sleeps for 30 s.
func stall(t *testing.T) string {
	t.Helper()
	sleep, err := procgroup.LookPath("sleep")
	require.NoError(t, err)
	return "exec '" + sleep + "' 30\n"
}

// requireTimeoutOf fails unless err is subprocess-timeout of the class.
func requireTimeoutOf(t *testing.T, class procgroup.Class, err error) {
	t.Helper()
	timeout, ok := errors.AsType[*procgroup.TimeoutError](err)
	require.True(t, ok, "the error is subprocess-timeout: %v", err)
	assert.Equal(t, class, timeout.Class)
}

// classCtx returns a context under which the class has a limit of
// 100 ms, below a deadline of 10 s: a command of another class that
// stalls ends at that deadline, which is not subprocess-timeout.
func classCtx(t *testing.T, class procgroup.Class) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	return procgroup.WithTestTimeout(ctx, class, 100*time.Millisecond)
}

// TestGoProbesRunUnderTheMiseClass shows that "go env GOROOT"
// (resolveGo), which a mise shim may answer with an install, and "go
// env GOVERSION" (checkGoVersion) run under the class procgroup.Mise:
// a go that stalls ends at the limit of that class.
func TestGoProbesRunUnderTheMiseClass(t *testing.T) {
	goBin := stub(t, "go", stall(t))
	t.Run("resolveGo", func(t *testing.T) {
		t.Setenv("PATH", filepath.Dir(goBin))
		_, err := resolveGo(classCtx(t, procgroup.Mise), t.TempDir())
		requireTimeoutOf(t, procgroup.Mise, err)
	})
	t.Run("checkGoVersion", func(t *testing.T) {
		requireTimeoutOf(t, procgroup.Mise, checkGoVersion(classCtx(t, procgroup.Mise), goBin))
	})
}

// TestGoProbesCarryStandardError shows that a go probe that fails says
// what the go command wrote to standard error, made printable.
func TestGoProbesCarryStandardError(t *testing.T) {
	goBin := stub(t, "go", "echo 'go: broken\x1b[1m' >&2\nexit 1\n")
	t.Setenv("PATH", filepath.Dir(goBin))
	_, err := resolveGo(t.Context(), t.TempDir())
	require.ErrorContains(t, err, `exited 1: "go: broken\x1b[1m"`)
	require.ErrorContains(t, checkGoVersion(t.Context(), goBin), `exited 1: "go: broken\x1b[1m"`)
}

// TestGoListRunsUnderTheProbeClass shows that "go list -find"
// (packages) runs under the class procgroup.ToolProbe, through goCmd as
// a run calls it: a go that stalls ends at the limit of that class.
func TestGoListRunsUnderTheProbeClass(t *testing.T) {
	goBin := stub(t, "go", stall(t))
	r := &runner{clone: git.Repo{Dir: t.TempDir()}, goBin: goBin, goEnv: goEnv(goBin, "")}
	m := mutant{File: "calc/calc.go", Package: "./other"}
	_, err := r.packages(classCtx(t, procgroup.ToolProbe), m)
	requireTimeoutOf(t, procgroup.ToolProbe, err)
}

// TestBuildAndTestAreUnbounded shows that the build and the test runs
// of the go command (build and goTest) have no limit, the class
// procgroup.Unbounded: with every class of the tools at 50 ms, a go
// that takes 300 ms still ends on its own, and what it printed is read.
func TestBuildAndTestAreUnbounded(t *testing.T) {
	sleep, err := procgroup.LookPath("sleep")
	require.NoError(t, err)
	goBin := stub(t, "go", "'"+sleep+"' 0.3\necho '--- PASS: TestSlow'\n")
	r := &runner{clone: git.Repo{Dir: t.TempDir()}, goBin: goBin, goEnv: goEnv(goBin, "")}
	ctx := t.Context()
	for _, class := range []procgroup.Class{procgroup.ToolStep, procgroup.ToolRace, procgroup.ToolGit, procgroup.ToolProbe, procgroup.Mise} {
		ctx = procgroup.WithTestTimeout(ctx, class, 50*time.Millisecond)
	}
	built, out, err := r.build(ctx, []string{"./calc"})
	require.NoError(t, err)
	assert.True(t, built)
	assert.Contains(t, out, "--- PASS: TestSlow", "the stub ran to its end")
	failed, out, err := r.goTest(ctx, target{pkg: "./calc", run: "^TestSlow$"}, time.Minute)
	require.NoError(t, err)
	assert.False(t, failed)
	assert.Equal(t, 1, countPassed(out), "the stub ran to its end")
}
