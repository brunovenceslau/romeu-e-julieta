// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// fsOpsEnv holds the operations that TestHelperFsOps runs, one on each
// line: "write <path> <text>", "chmod <path> <octal mode>" or "remove
// <path>", in its working
// directory. A step that runs this test binary again is a generator
// whose effect on the tree the test chooses.
const fsOpsEnv = "CI_TEST_FSOPS"

// TestHelperFsOps is not a test: it is the generator of the tests below,
// when fsOpsEnv is set.
func TestHelperFsOps(t *testing.T) {
	ops := os.Getenv(fsOpsEnv)
	if ops == "" {
		return
	}
	for _, line := range strings.Split(ops, "\n") {
		f := strings.SplitN(line, " ", 3)
		switch f[0] {
		case "write":
			if err := os.WriteFile(f[1], []byte(f[2]), 0o644); err != nil {
				os.Exit(2)
			}
		case "chmod":
			mode, err := strconv.ParseUint(f[2], 8, 32)
			if err != nil || os.Chmod(f[1], os.FileMode(mode)) != nil {
				os.Exit(2)
			}
		case "remove":
			if err := os.Remove(f[1]); err != nil {
				os.Exit(2)
			}
		case "fail":
			_, _ = os.Stdout.WriteString("the generator says no\n")
			os.Exit(3)
		}
	}
	os.Exit(0)
}

// fsOps returns a generator step that runs the operations.
func fsOps(ops ...string) step {
	return step{
		name:    "go generate",
		argv:    []string{os.Args[0], "-test.run=^TestHelperFsOps$"},
		environ: []string{"PATH=" + os.Getenv("PATH"), fsOpsEnv + "=" + strings.Join(ops, "\n")},
	}
}

// generatedFixture is a repository with a committed generated file and
// the other kinds of file a tree holds: an untracked one, an ignored one,
// a symbolic link, and a tracked file that is deleted from the disk, which
// no snapshot holds.
func generatedFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(".gitignore", "ignored.txt\n")
	r.Write("gen.txt", "generated")
	r.Write("gone.txt", "gone\n")
	require.NoError(t, os.Symlink("gen.txt", filepath.Join(r.Dir, "link")))
	r.Commit("fixture")
	require.NoError(t, os.Remove(filepath.Join(r.Dir, "gone.txt")))
	r.Write("untracked.txt", "u\n")
	r.Write("ignored.txt", "i\n")
	return r
}

// TestGenerated is the acceptance case of 10 10.2: a generator that
// changes, creates or removes a file fails, one that changes nothing
// passes, and the comparison reads content, so a rewrite with the same
// bytes passes.
func TestGenerated(t *testing.T) {
	tests := []struct {
		name string
		gen  step
		want []string // "<path>: generated: <message start>"
	}{
		{"nothing changes", fsOps(), nil},
		{"a rewrite with the same content passes", fsOps("write gen.txt generated"), nil},
		{"a changed tracked file fails", fsOps("write gen.txt hand edited"), []string{"gen.txt: generated: go generate changed this file"}},
		{"a new file fails", fsOps("write new.txt x"), []string{"new.txt: generated: go generate created this file"}},
		{"a chmod of a tracked file fails", fsOps("chmod gen.txt 755"), []string{"gen.txt: generated: go generate changed this file"}},
		{"a removed file fails", fsOps("remove gen.txt"), []string{"gen.txt: generated: go generate removed this file"}},
		{"a changed untracked file fails", fsOps("write untracked.txt v2"), []string{"untracked.txt: generated: go generate changed this file"}},
		{"a removed untracked file fails", fsOps("remove untracked.txt"), []string{"untracked.txt: generated: go generate removed this file"}},
		{"an ignored file is not compared", fsOps("write ignored.txt changed"), nil},
		{"a deleted tracked file that the generator writes again is new", fsOps("write gone.txt back"), []string{"gone.txt: generated: go generate created this file"}},
		{"findings are sorted by path", fsOps("write z.txt 1", "write gen.txt x", "write a.txt 1"), []string{
			"a.txt: generated: go generate created this file",
			"gen.txt: generated: go generate changed this file",
			"z.txt: generated: go generate created this file",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := generatedFixture(t)
			findings, err := generated(context.Background(), r.Repo, tt.gen, nil)
			require.NoError(t, err)
			require.Len(t, findings, len(tt.want))
			for i, w := range tt.want {
				assert.True(t, strings.HasPrefix(findings[i].String(), w), "%q starts with %q", findings[i], w)
			}
		})
	}
}

// TestGeneratedGeneratorFails checks that a generator that exits non-zero
// is an error that carries its output, not a finding.
func TestGeneratedGeneratorFails(t *testing.T) {
	r := generatedFixture(t)
	_, err := generated(context.Background(), r.Repo, fsOps("fail"), nil)
	require.ErrorContains(t, err, "go generate")
	require.ErrorContains(t, err, "the generator says no")
}

// TestGeneratedNotAGitRepository checks that a directory outside a
// repository is an error, not a pass.
func TestGeneratedNotAGitRepository(t *testing.T) {
	r := generatedFixture(t)
	r.Dir = t.TempDir()
	_, err := generated(context.Background(), r.Repo, fsOps("write a x"), nil)
	require.Error(t, err)
}

// TestRunGeneratedTakesNoArgument checks the command line of the
// subcommand.
func TestRunGeneratedTakesNoArgument(t *testing.T) {
	var out, errOut bytes.Buffer
	e := env{dir: ".", stdout: &out, stderr: &errOut}
	assert.Equal(t, exitError, run(context.Background(), e, []string{"generated", "x"}))
	assert.Contains(t, errOut.String(), "generated takes no argument")
	assert.Equal(t, exitError, run(context.Background(), e, []string{"generate", "x"}))
	assert.Contains(t, errOut.String(), "generate takes no argument")
}

// committedGenerateFixture is a repository whose generated files are
// written by generate and committed: the state a clean checkout holds.
func committedGenerateFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := generateFixture(t)
	code, _, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	r.Commit("fixture")
	return r
}

// gateOf runs the gate over r with a generator that does nothing, since
// the generator's effect is the subject of the tests above, and with
// the outputs of generate as the files that must be tracked.
func gateOf(t *testing.T, r *gittest.Repo) []string {
	t.Helper()
	findings, err := generated(context.Background(), r.Repo, fsOps(), generateOutputs)
	require.NoError(t, err)
	got := make([]string, len(findings))
	for i, f := range findings {
		got[i] = f.String()
	}
	return got
}

// TestGeneratedRequiresTrackedRegularOutputs is the attack of the
// round-1 audit: a generated file that the before and after snapshots
// cannot tell from a right one, because it is ignored, removed from the
// index, a link to a copy, a link out of the tree, executable or stale.
// Each one must fail, and a clean checkout must pass.
func TestGeneratedRequiresTrackedRegularOutputs(t *testing.T) {
	const codeowners = ".github/CODEOWNERS"
	tests := []struct {
		name  string
		setup func(t *testing.T, r *gittest.Repo)
		want  []string // "<path>: generated: <message start>"
	}{
		{"a clean checkout passes", func(*testing.T, *gittest.Repo) {}, nil},
		{"A: removed from the index and ignored", func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "--quiet", "--cached", codeowners)
			r.Write(".gitignore", codeowners+"\n")
			r.Commit("hide it")
			r.Git("clean", "-fdx", "--quiet")
		}, []string{codeowners + ": generated: is not tracked"}},
		{"untracked but not ignored", func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "--quiet", "--cached", codeowners)
			r.Git("commit", "--quiet", "--message", "untrack it")
		}, []string{codeowners + ": generated: is not tracked"}},
		{"tracked and ignored", func(t *testing.T, r *gittest.Repo) {
			r.Write(".gitignore", codeowners+"\n")
			r.Commit("ignore it")
		}, []string{codeowners + ": generated: is tracked and matches an ignore rule"}},
		{"B: a link to an identical copy", func(t *testing.T, r *gittest.Repo) {
			require.NoError(t, os.Rename(filepath.Join(r.Dir, codeowners), filepath.Join(r.Dir, "copy")))
			require.NoError(t, os.Symlink("../copy", filepath.Join(r.Dir, codeowners)))
			r.Commit("link it")
		}, []string{codeowners + ": generated: is tracked with mode 120000"}},
		{"F: a link to a path outside the tree", func(t *testing.T, r *gittest.Repo) {
			page := filepath.Join(r.Dir, filepath.FromSlash(askFirstPagePath))
			require.NoError(t, os.Remove(page))
			require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "outside.md"), page))
			r.Commit("link it out")
		}, []string{askFirstPagePath + ": generated: is tracked with mode 120000"}},
		{"executable", func(t *testing.T, r *gittest.Repo) {
			require.NoError(t, os.Chmod(filepath.Join(r.Dir, codeowners), 0o755))
			r.Commit("make it executable")
		}, []string{codeowners + ": generated: is tracked with mode 100755"}},
		{"committed with other bytes", func(t *testing.T, r *gittest.Repo) {
			r.Write(codeowners, "# a hand edit\n")
			r.Commit("edit by hand")
		}, []string{codeowners + ": generated: is tracked with other bytes"}},
		{"changed in the tree and not staged", func(t *testing.T, r *gittest.Repo) {
			r.Write(codeowners, "# a hand edit\n")
		}, nil}, // the index still holds the right bytes; the snapshots see no change either
		{"unmerged", func(t *testing.T, r *gittest.Repo) {
			r.Git("checkout", "--quiet", "-b", "other")
			r.Write(codeowners, "# other\n")
			r.Commit("other")
			r.Git("checkout", "--quiet", "main")
			r.Write(codeowners, "# main\n")
			r.Commit("main")
			_, err := r.Run(t.Context(), nil, "merge", "other") // conflicts: that is the fixture
			require.Error(t, err)
		}, []string{codeowners + ": generated: has an unmerged entry"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := committedGenerateFixture(t).For(t)
			tt.setup(t, r)
			got := gateOf(t, r)
			require.Len(t, got, len(tt.want), "%v", got)
			for i, w := range tt.want {
				assert.True(t, strings.HasPrefix(got[i], w), "%q starts with %q", got[i], w)
			}
		})
	}
}

// TestFileDigestStaysInsideTheRoot checks that a path that goes through
// a link out of the tree is an error, not a read of what lies there.
func TestFileDigestStaysInsideTheRoot(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "sub")))
	_, _, err := fileDigest(rootOf(t, dir), "sub/secret")
	require.Error(t, err)
}
