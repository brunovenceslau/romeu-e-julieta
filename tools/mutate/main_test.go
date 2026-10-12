// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
// repository: HEAD, every ref, the real index (its bytes and its
// entries) and the state of the working tree. It reads status with
// --no-optional-locks, so reading it does not refresh the index.
type repoState struct {
	head, refs, index, staged, status string
}

func (f fixture) state() repoState {
	index, err := os.ReadFile(filepath.Join(f.repo.Dir, ".git", "index"))
	if err != nil {
		index = nil // a fixture with no index yet
	}
	return repoState{
		head:   f.repo.Git("rev-parse", "HEAD"),
		refs:   f.repo.Git("for-each-ref"),
		index:  string(index),
		staged: f.repo.Git("ls-files", "--stage"),
		status: f.repo.Git("--no-optional-locks", "status", "--porcelain", "--ignored", "--untracked-files=all"),
	}
}

// assertUntouched checks that the source repository is as before and
// has no stash, and that no temporary clone is left.
func (f fixture) assertUntouched(t *testing.T, before repoState) {
	t.Helper()
	assert.Equal(t, before, f.state())
	_, err := f.repo.Run(context.Background(), nil, "rev-parse", "--verify", "--quiet", "refs/stash")
	require.Error(t, err, "refs/stash must not exist")
	entries, err := os.ReadDir(f.e.tmpDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the clone is removed")
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

// TestMutateEndToEnd is the acceptance case: one mutant killed, one
// survived and one invalid, in that order, a summary, exit 1, and the
// source repository untouched.
func TestMutateEndToEnd(t *testing.T) {
	f := newFixture(t, threeMutants)
	before := f.state()
	code := run(context.Background(), f.e, []string{f.mutants})
	assert.Equal(t, exitFail, code, f.stderr.String())
	want := "clamp-low killed ./calc -run ^TestClamp$ fails\n" +
		"double-plus survived ./calc -run ^TestDouble$ passes\n" +
		"every-return invalid old occurs 4 times in calc/calc.go, not once\n" +
		"mutate: 1 killed, 1 survived, 1 invalid, of 3 mutants at " + before.head + "\n"
	assert.Equal(t, want, f.stdout.String())
	assert.Empty(t, f.stderr.String())
	f.assertUntouched(t, before)
}

// TestMutateEveryMutantKilled checks exit 0, from a subdirectory of the
// repository and with the mutants file named relative to it.
func TestMutateEveryMutantKilled(t *testing.T) {
	f := newFixture(t, threeMutants)
	sub := filepath.Join(f.repo.Dir, "calc")
	src, err := os.ReadFile(f.mutants)
	require.NoError(t, err)
	killedOnly := strings.SplitN(string(src), "  - id: double-plus", 2)[0]
	rel, err := filepath.Rel(sub, f.mutants)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(f.mutants, []byte(killedOnly), 0o644))
	f.e.dir = sub
	before := f.state()
	assert.Equal(t, exitOK, run(context.Background(), f.e, []string{rel}), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "mutate: 1 killed, 0 survived, 0 invalid, of 1 mutants")
	f.assertUntouched(t, before)
}

// TestMutateInvalid covers each kind of invalid mutant: a build failure
// is no kill, and the file is restored before the next mutant runs, so
// the killable one after it is still killed.
func TestMutateInvalid(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: no-build, file: calc/calc.go, old: "x * 2", new: "x *", package: ./calc, run: ^TestDouble$}
  - {id: missing-old, file: calc/calc.go, old: "x / 2", new: "x", package: ./calc, run: ^TestDouble$}
  - {id: no-file, file: calc/none.go, old: "a", new: "b", package: ./calc, run: ^TestDouble$}
  - {id: a-directory, file: calc, old: "a", new: "b", package: ./calc, run: ^TestDouble$}
  - {id: other-package, file: other/other.go, old: "1", new: "x", package: ./calc, run: ^TestDouble$}
  - {id: clamp-low, file: calc/calc.go, old: "x < lo", new: "x > lo", package: ./calc, run: ^TestClamp$}
`)
	f.repo.Write("other/other.go", "package other\n\n// One is one.\nconst One = 1\n")
	f.repo.Commit("other")
	before := f.state()
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}))
	lines := strings.Split(strings.TrimSuffix(f.stdout.String(), "\n"), "\n")
	require.Len(t, lines, 7, f.stdout.String())
	assert.Equal(t, "no-build invalid the mutated code does not build", lines[0])
	assert.Equal(t, "missing-old invalid old occurs 0 times in calc/calc.go, not once", lines[1])
	assert.Equal(t, "no-file invalid calc/none.go is not a file of the commit", lines[2])
	assert.Equal(t, "a-directory invalid calc is not a file of the commit", lines[3])
	assert.Equal(t, "other-package invalid the mutated code does not build", lines[4], "the package of the file is built too")
	assert.Equal(t, "clamp-low killed ./calc -run ^TestClamp$ fails", lines[5])
	assert.Contains(t, f.stderr.String(), "no-build: the build fails:")
	assert.Contains(t, f.stderr.String(), "other-package: the build fails:")
	f.assertUntouched(t, before)
}

// TestMutateOutsideAPackage checks that a Go file in a directory that
// holds no package, such as testdata, is judged by the tests that read
// it and not as a build failure, and that a target of ./... runs.
func TestMutateOutsideAPackage(t *testing.T) {
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
	before := f.state()
	assert.Equal(t, exitOK, run(context.Background(), f.e, []string{f.mutants}), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "data killed ./... -run ^TestData$ fails")
	f.assertUntouched(t, before)
}

// TestMutateWorktree is the case -worktree exists for: a builder who
// has not committed. The test that kills double-plus is in an untracked
// file, which calls a helper added to a tracked file without staging
// it, which reads a constant of a file that is staged only and one of a
// tracked file that an ignore rule matches. Without
// -worktree the run sees none of them and the mutant survives; with it,
// the mutant is killed. An ignored test that would fail the baseline is
// left out. Neither run changes the real index, HEAD, a ref, the stash
// or what git status shows.
func TestMutateWorktree(t *testing.T) {
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
	before := f.state()
	require.Contains(t, before.status, "?? calc/double_test.go")
	require.Contains(t, before.status, "A  calc/three.go")
	require.Contains(t, before.status, " M calc/calc_test.go")
	require.Contains(t, before.status, "!! calc/ignored_test.go")

	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "double-plus survived")
	assert.Contains(t, f.stderr.String(), "the run uses the commit "+before.head+" without them; -worktree runs them too")
	f.assertUntouched(t, before)

	f.stdout.Reset()
	f.stderr.Reset()
	assert.Equal(t, exitOK, run(context.Background(), f.e, []string{"-worktree", f.mutants}), f.stderr.String())
	assert.Contains(t, f.stdout.String(), "double-plus killed ./calc -run ^TestDouble fails")
	assert.NotContains(t, f.stdout.String(), "at "+before.head, "the run is at the snapshot, not HEAD")
	assert.Empty(t, f.stderr.String())
	f.assertUntouched(t, before)

	// The snapshot is a commit object no ref reaches, child of HEAD.
	snap := strings.TrimSuffix(f.stdout.String()[strings.LastIndex(f.stdout.String(), " ")+1:], "\n")
	assert.Equal(t, before.head, f.repo.Git("rev-parse", snap+"^"))
	assert.Empty(t, f.repo.Git("for-each-ref", "--contains", snap))
}

// TestMutateWorktreeClean checks that -worktree on a working tree that
// equals HEAD runs HEAD itself and makes no snapshot commit.
func TestMutateWorktreeClean(t *testing.T) {
	f := newFixture(t, threeMutants)
	before := f.state()
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{"-worktree", f.mutants}))
	assert.Contains(t, f.stdout.String(), "of 3 mutants at "+before.head+"\n")
	f.assertUntouched(t, before)
}

// TestMutateLinkIsInvalid checks that a tracked symbolic link, whose
// blob is its target and not the content a mutant would change, is an
// invalid mutant.
func TestMutateLinkIsInvalid(t *testing.T) {
	f := newFixture(t, "mutants:\n  - {id: link, file: calc/calc.lnk, old: \"x * 2\", new: \"x + 2\", package: ./calc, run: ^TestDouble$}\n")
	require.NoError(t, os.Symlink("calc.go", filepath.Join(f.repo.Dir, "calc", "calc.lnk")))
	f.repo.Commit("link")
	before := f.state()
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}))
	assert.Contains(t, f.stdout.String(), "link invalid calc/calc.lnk is not checked out as its blob")
	f.assertUntouched(t, before)
}

// TestMutateRedBaseline checks that a target that fails with no mutant
// stops the run with exit 2 before any mutant is judged.
func TestMutateRedBaseline(t *testing.T) {
	f := newFixture(t, threeMutants)
	f.repo.Write("calc/calc_test.go", strings.Replace(fixtureCalcTest, "!= 4", "!= 5", 1))
	f.repo.Commit("red")
	before := f.state()
	assert.Equal(t, exitError, run(context.Background(), f.e, []string{f.mutants}))
	assert.Empty(t, f.stdout.String())
	assert.Contains(t, f.stderr.String(), "the baseline of ./calc -run ^TestDouble$ fails with no mutant")
	assert.Contains(t, f.stderr.String(), "Double(2) is not 4")
	f.assertUntouched(t, before)
}

// TestMutateUsesTheCommit checks that an uncommitted change is not part
// of the run, and that the run says so.
func TestMutateUsesTheCommit(t *testing.T) {
	f := newFixture(t, threeMutants)
	// Uncommitted, this test would fail the baseline of TestDouble.
	f.repo.Write("calc/calc_test.go", strings.Replace(fixtureCalcTest, "!= 4", "!= 5", 1))
	before := f.state()
	assert.Equal(t, exitFail, run(context.Background(), f.e, []string{f.mutants}), f.stderr.String())
	assert.Contains(t, f.stderr.String(), "the working tree has uncommitted changes (git status: 1); the run uses the commit "+before.head+" without them; -worktree runs them too")
	assert.Contains(t, f.stdout.String(), "double-plus survived")
	f.assertUntouched(t, before)
}

// TestMutateRefuses covers the errors that stop a run before any
// mutant: each exits 2, says why, and leaves no clone.
func TestMutateRefuses(t *testing.T) {
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
			require.NoError(t, os.WriteFile(f.mutants, []byte("mutants: []\n"), 0o644))
		}, "lists no mutant"},
		{"not a repository", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, ".git")))
		}, "git rev-parse"},
		{"no commit", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, ".git")))
			f.repo.Git("init", "--quiet", "--initial-branch=main")
		}, "has no commit HEAD to run"},
		{"a go that does not run", func(f fixture) []string { return []string{f.mutants} }, func(_ *testing.T, f *fixture) {
			f.e.goBin = filepath.Join(f.e.tmpDir, "none", "go")
		}, "none/go test"},
		{"a package that matches nothing", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			require.NoError(t, os.WriteFile(f.mutants, []byte(strings.ReplaceAll(threeMutants, "package: ./calc", "package: ./nope")), 0o644))
		}, "the baseline of ./nope -run ^TestClamp$ fails with no mutant"},
		{"a run that matches no test", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
			require.NoError(t, os.WriteFile(f.mutants, []byte(strings.ReplaceAll(threeMutants, "run: ^TestClamp$", "run: ^TestNope$")), 0o644))
		}, "./calc -run ^TestNope$ runs no test"},
		{"no temporary directory", func(f fixture) []string { return []string{f.mutants} }, func(t *testing.T, f *fixture) {
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

// helperEnv names the role of this test binary when a test runs it as a
// helper process; an empty value means it is the test run itself.
const helperEnv = "MUTATE_TEST_HELPER"

// TestHelperProcess is not a test: TestMutateInterrupted runs this
// binary again with helperEnv set, to get a process with the signal
// handling of main that it can interrupt.
func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "main" {
		return
	}
	e := env{
		dir:    os.Getenv("MUTATE_TEST_DIR"),
		goBin:  os.Getenv("MUTATE_TEST_GO"),
		tmpDir: os.Getenv("MUTATE_TEST_TMP"),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	os.Exit(runSignalled(e, []string{os.Getenv("MUTATE_TEST_MUTANTS")}))
}

// TestMutateInterrupted sends SIGINT to a run while a mutant runs a
// test that never ends: the run kills it, removes the clone, leaves the
// source as it was, and exits 130.
func TestMutateInterrupted(t *testing.T) {
	f := newFixture(t, `mutants:
  - {id: hang, file: calc/calc.go, old: "return x * 2", new: "for {\n\t}", package: ./calc, run: ^TestDouble$}
`)
	before := f.state()
	gocache, err := exec.Command(f.e.goBin, "env", "GOCACHE").Output()
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(append([]string{}, f.repo.Env...),
		"GOCACHE="+strings.TrimSpace(string(gocache)),
		helperEnv+"=main",
		"MUTATE_TEST_DIR="+f.e.dir,
		"MUTATE_TEST_GO="+f.e.goBin,
		"MUTATE_TEST_TMP="+f.e.tmpDir,
		"MUTATE_TEST_MUTANTS="+f.mutants,
	)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// The bin directory appears when the mutant is built, after the
	// baseline, so the handler of main is in place by then.
	require.Eventually(t, func() bool {
		matches, _ := filepath.Glob(filepath.Join(f.e.tmpDir, "mutate-*", "bin"))
		return len(matches) == 1
	}, time.Minute, 50*time.Millisecond)
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))
	select {
	case err := <-done:
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		assert.Equal(t, exitSignalBase+int(syscall.SIGINT), exit.ExitCode())
	case <-time.After(time.Minute):
		require.FailNow(t, "the run did not stop")
	}
	assert.Contains(t, stderr.String(), "mutate: interrupted")
	assert.Empty(t, stdout.String())
	f.assertUntouched(t, before)
}

// TestRunSignalledWithoutASignal checks that a run no signal touches
// keeps its own status.
func TestRunSignalledWithoutASignal(t *testing.T) {
	f := newFixture(t, threeMutants)
	assert.Equal(t, exitFail, runSignalled(f.e, []string{f.mutants}))
	assert.NotContains(t, f.stderr.String(), "interrupted")
}

// TestFinish checks the status of a run that no signal touched, and of
// one a signal cut short.
func TestFinish(t *testing.T) {
	var b bytes.Buffer
	assert.Equal(t, exitFail, finish(&b, exitFail, 0))
	assert.Empty(t, b.String())
	assert.Equal(t, exitSignalBase+int(syscall.SIGTERM), finish(&b, exitOK, syscall.SIGTERM))
	assert.Equal(t, "mutate: interrupted\n", b.String())
}

// TestCheckGoVersion checks that a go of another version is refused.
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
}

// TestGoEnvIsBuiltFromNothing checks that a variable of this process
// that would change the build does not reach the go command, and that
// the pinned values are set.
func TestGoEnvIsBuiltFromNothing(t *testing.T) {
	t.Setenv("GOFLAGS", "-run=Nothing")
	t.Setenv("GOTOOLCHAIN", "go1.0")
	t.Setenv("GOCACHE", "/cache")
	got := goEnv("/pinned/bin/go")
	assert.True(t, strings.HasPrefix(got[0], "PATH=/pinned/bin"+string(os.PathListSeparator)), got[0])
	assert.Contains(t, got, "GOCACHE=/cache")
	assert.Contains(t, got, "GOFLAGS=-mod=readonly")
	assert.Contains(t, got, "GOTOOLCHAIN=local")
	assert.NotContains(t, got, "GOFLAGS=-run=Nothing")
	assert.NotContains(t, got, "GOTOOLCHAIN=go1.0")
}
