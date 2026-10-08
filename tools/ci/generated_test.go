// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// fsOpsEnv holds the operations that TestHelperFsOps runs, one on each
// line: "write <path> <text>" or "remove <path>", in its working
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
			findings, err := generated(context.Background(), r.Repo, tt.gen)
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
	_, err := generated(context.Background(), r.Repo, fsOps("fail"))
	require.ErrorContains(t, err, "go generate")
	require.ErrorContains(t, err, "the generator says no")
}

// TestGeneratedNotAGitRepository checks that a directory outside a
// repository is an error, not a pass.
func TestGeneratedNotAGitRepository(t *testing.T) {
	r := generatedFixture(t)
	r.Dir = t.TempDir()
	_, err := generated(context.Background(), r.Repo, fsOps("write a x"))
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
