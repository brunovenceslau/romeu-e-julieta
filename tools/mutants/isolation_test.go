// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hookVars are the variables git exports to a hook, pointed at the
// fixture's own repository, as a pre-commit hook that starts the tool
// would pass them on. Before git.CleanEnv, they sent the clone's
// commands to the source: an empty commit that lost the staged change,
// the source's remote.origin.url overwritten, and packs written to its
// object store.
func hookVars(f fixture) []string {
	gitDir := filepath.Join(f.repo.Dir, ".git")
	return []string{
		"GIT_DIR=" + gitDir,
		"GIT_INDEX_FILE=" + filepath.Join(gitDir, "index"),
		"GIT_COMMON_DIR=" + gitDir,
		"GIT_OBJECT_DIRECTORY=" + filepath.Join(gitDir, "objects"),
		"GIT_WORK_TREE=" + f.repo.Dir,
	}
}

// TestMutantsIgnoresTheCallersGitVariables runs the command with the
// variables of a hook, given as its git environment and, with a nil git
// environment, as this process's own: the run is the run without them,
// and the source repository is untouched, its index, configuration,
// refs and packs included; a -worktree run adds loose objects only.
func TestMutantsIgnoresTheCallersGitVariables(t *testing.T) {
	for _, worktree := range []bool{false, true} {
		for _, fromProcess := range []bool{false, true} {
			name := map[bool]string{false: "given", true: "from the process"}[fromProcess]
			if worktree {
				name += " with -worktree"
			}
			t.Run(name, func(t *testing.T) {
				f := newFixture(t, threeMutants)
				f.repo.Write("calc/three.go", "package calc\n\nconst three = 3\n")
				f.repo.Git("add", "calc/three.go")
				before := f.state(t)
				if fromProcess {
					for _, kv := range hookVars(f) {
						k, v, _ := strings.Cut(kv, "=")
						t.Setenv(k, v)
					}
					f.e.gitEnv = nil
				} else {
					f.e.gitEnv = append(append([]string{}, f.repo.Env...), hookVars(f)...)
				}
				var args []string
				if worktree {
					args = []string{"-worktree"}
				}
				assert.Equal(t, exitFail, f.runArgs(args...), f.stderr.String())
				assert.Contains(t, f.stdout.String(), "clamp-low killed")
				assert.Contains(t, f.stdout.String(), "double-plus survived")
				if worktree {
					f.assertOnlyObjectsAdded(t, before)
				} else {
					f.assertUntouched(t, before)
				}
			})
		}
	}
}

// fakeGit puts first on PATH a git that logs each command line to log
// and fails a command that has fail among its arguments; any other
// command runs the real git.
func fakeGit(t *testing.T, fail string) (log string, env []string) {
	t.Helper()
	real, err := exec.LookPath("git")
	require.NoError(t, err)
	dir := t.TempDir()
	log = filepath.Join(dir, "log")
	script := `#!/bin/sh
if [ -n "$MUTANTS_GIT_LOG" ]; then
	echo "$*" >> "$MUTANTS_GIT_LOG"
fi
for a in "$@"; do
	if [ -n "$MUTANTS_FAIL_GIT" ] && [ "$a" = "$MUTANTS_FAIL_GIT" ]; then
		echo "fake failure of $a" >&2
		exit 1
	fi
done
exec '` + real + `' "$@"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log, []string{"MUTANTS_GIT_LOG=" + log, "MUTANTS_FAIL_GIT=" + fail}
}

// TestMutantsGitFails covers a git command that fails in each step the
// run cannot go on without: the clone and its checks, the snapshot,
// and the check after a restore. Each exits 2, names the command, and
// leaves no clone and the source as it was, but for the objects a
// snapshot wrote before the failure.
func TestMutantsGitFails(t *testing.T) {
	for _, tt := range []struct{ fail, want string }{
		{"clone", "git clone"},
		{"remote", "git remote"},
		{"ls-tree", "git ls-tree"},
		{"ls-files", "git ls-files"},
		{"read-tree", "snapshot: git read-tree"},
		{"add", "snapshot: git add"},
		{"write-tree", "snapshot: git write-tree"},
		{"commit-tree", "snapshot: git commit-tree"},
		{"pack-objects", "pack-objects"},
		{"unpack-objects", "snapshot: git unpack-objects"},
		{"checkout", "snapshot: git checkout in the clone"},
		{"status", "git status"},
	} {
		t.Run(tt.fail, func(t *testing.T) {
			f := newFixture(t, threeMutants)
			f.repo.Write("calc/extra_test.go", "package calc\n")
			before := f.state(t)
			_, vars := fakeGit(t, tt.fail)
			f.e.gitEnv = append(append([]string{}, f.repo.Env...), vars...)
			assert.Equal(t, exitError, f.runArgs("-worktree"))
			assert.Contains(t, f.stderr.String(), tt.want)
			assert.Contains(t, f.stderr.String(), "fake failure of "+tt.fail)
			f.assertOnlyObjectsAdded(t, before)
		})
	}
}

// TestMutantsCloneSteps checks, from the log of the git commands, that
// the clone runs with the hooks off and that its remote is removed
// right after it is made.
func TestMutantsCloneSteps(t *testing.T) {
	f := newFixture(t, threeMutants)
	log, vars := fakeGit(t, "none")
	f.e.gitEnv = append(append([]string{}, f.repo.Env...), vars...)
	assert.Equal(t, exitFail, f.runArgs(), f.stderr.String())
	b, err := os.ReadFile(log)
	require.NoError(t, err)
	lines := strings.Split(string(b), "\n")
	clone := -1
	for i, line := range lines {
		if strings.Contains(line, " clone ") {
			clone = i
		}
	}
	require.GreaterOrEqual(t, clone, 0, string(b))
	assert.Contains(t, lines[clone], "--config core.hooksPath="+os.DevNull)
	assert.Contains(t, lines[clone+2], "remote remove origin", "after the clone and the check of its git directory")
}
