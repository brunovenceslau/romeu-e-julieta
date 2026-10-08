// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adr"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// TestGenerated covers the generated step of 10 10.2: each generated
// file of the commit HEAD names is a regular file that holds what its
// generator writes from the sources of that same commit. Each case
// starts from a committed fixture whose generated files are current,
// and changes it after that commit; a case whose change is not
// committed shows that the working tree and the index are not read.
func TestGenerated(t *testing.T) {
	codeownersFile := func(r *gittest.Repo) string { return filepath.Join(r.Dir, filepath.FromSlash(codeownersPath)) }
	const record2 = "docs/adr/0002-pick-another.md"
	const record2Text = "# 2. Pick another\n\n## Status\n\nProposed\n"
	// regenerate writes the generated files from the working tree, as
	// "go generate ./..." does.
	regenerate := func(t *testing.T, r *gittest.Repo) {
		t.Helper()
		files, err := generateFiles(os.DirFS(r.Dir))
		require.NoError(t, err)
		for _, f := range files {
			r.Write(f.path, string(f.data))
		}
	}
	tests := []struct {
		name   string
		change func(t *testing.T, r *gittest.Repo)
		code   int
		want   []string
		lacks  []string
	}{
		{
			name: "current files",
			code: exitOK,
			want: []string{"generated: ok\n"},
		},
		{
			name: "uncommitted work elsewhere",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("README.md", "# A fixture\n\nEdited.\n")
				r.Write("wip.md", "not added yet\n")
			},
			code: exitOK,
			want: []string{"generated: ok\n"},
		},
		{
			name: "a hand edit, a deletion and a broken source, none committed",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(codeownersPath, "# nothing asks first\n")
				r.Git("add", codeownersPath)
				require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(askFirstPagePath))))
				r.Write(askFirstPath, "version: [\n")
			},
			code: exitOK,
			want: []string{"generated: ok\n"},
		},
		{
			name: "a hand edit, committed",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(codeownersPath, "# nothing asks first\n")
				r.Commit("weaken")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the file differs from what its generator writes from the sources in the commit; edit the source, not this file, then run \"go generate ./...\", review the result and commit it; this step reads the commit, not the working tree\n", "generated: 1 finding(s)\n"},
		},
		{
			name: "a source committed without its consumers",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(askFirstPath, strings.Replace(fixtureList, "reason: this list", "reason: this list, edited", 1))
				r.Commit("edit the list")
			},
			code:  exitFail,
			want:  []string{"docs/reference/ask-first.md: generated: the file differs", "generated: 1 finding(s)\n"},
			lacks: []string{".github/CODEOWNERS"},
		},
		{
			name: "a generated file deleted in a commit",
			change: func(t *testing.T, r *gittest.Repo) {
				require.NoError(t, os.Remove(codeownersFile(r)))
				r.Commit("drop it")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the commit does not hold the file;"},
		},
		{
			name: "a generated file removed from git and ignored",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Git("rm", "--quiet", codeownersPath)
				r.Write(".gitignore", "CODEOWNERS\n")
				r.Commit("drop it")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the commit does not hold the file;"},
		},
		{
			name: "a generated file kept on disk, untracked and ignored",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Git("rm", "--quiet", "--cached", codeownersPath)
				r.Write(".gitignore", "CODEOWNERS\n")
				r.Commit("untrack it")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the commit does not hold the file;"},
		},
		{
			name: "a stale file regenerated and staged, not committed",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(codeownersPath, "# stale\n")
				r.Commit("stale")
				regenerate(t, r)
				r.Git("add", "--all")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the file differs", "this step reads the commit, not the working tree"},
		},
		{
			name: "every generated file stale, in the order of the generators",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/adr/README.md", "x\n")
				r.Write(askFirstPagePath, "x\n")
				r.Write(codeownersPath, "x\n")
				r.Commit("stale")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the file differs from what its generator writes from the sources in the commit; edit the source, not this file, then run \"go generate ./...\", review the result and commit it; this step reads the commit, not the working tree\n" +
				"docs/reference/ask-first.md: generated: the file differs from what its generator writes from the sources in the commit; edit the source, not this file, then run \"go generate ./...\", review the result and commit it; this step reads the commit, not the working tree\n" +
				"docs/adr/README.md: generated: the file differs from what its generator writes from the sources in the commit; edit the source, not this file, then run \"go generate ./...\", review the result and commit it; this step reads the commit, not the working tree\n" +
				"generated: 3 finding(s)\n"},
		},
		{
			name: "a generated file committed as a symbolic link whose target is its content",
			change: func(t *testing.T, r *gittest.Repo) {
				data, err := os.ReadFile(codeownersFile(r))
				require.NoError(t, err)
				blob := r.WriteObject("blob", string(data))
				r.Git("update-index", "--cacheinfo", "120000,"+blob+","+codeownersPath)
				r.Git("commit", "--quiet", "--message", "a link")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the commit holds the file with mode 120000, and a generated file is a regular file of mode 100644;", "generated: 1 finding(s)\n"},
		},
		{
			name: "a generated file committed as executable",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Git("update-index", "--chmod=+x", codeownersPath)
				r.Git("commit", "--quiet", "--message", "executable")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the commit holds the file with mode 100755"},
		},
		{
			name: "a record committed without the index",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(record2, record2Text)
				r.Commit("a record")
			},
			code:  exitFail,
			want:  []string{"docs/adr/README.md: generated: the file differs", "generated: 1 finding(s)\n"},
			lacks: []string{"CODEOWNERS"},
		},
		{
			name: "a record written and regenerated, nothing committed",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(record2, record2Text)
				regenerate(t, r)
			},
			code: exitOK,
			want: []string{"generated: ok\n"},
		},
		{
			name: "the index committed without the record it lists",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(record2, record2Text)
				regenerate(t, r)
				r.Git("add", adr.Dir+"/"+adr.Index)
				r.Git("commit", "--quiet", "--message", "the index alone")
			},
			code: exitFail,
			want: []string{"docs/adr/README.md: generated: the file differs", "generated: 1 finding(s)\n"},
		},
		{
			name: "a working copy with CRLF line endings, the commit with LF",
			change: func(t *testing.T, r *gittest.Repo) {
				for _, p := range []string{codeownersPath, askFirstPagePath, askFirstPath} {
					data, err := os.ReadFile(filepath.Join(r.Dir, filepath.FromSlash(p)))
					require.NoError(t, err)
					r.Write(p, strings.ReplaceAll(string(data), "\n", "\r\n"))
				}
			},
			code: exitOK,
			want: []string{"generated: ok\n"},
		},
		{
			name: "a generated file committed with CRLF line endings",
			change: func(t *testing.T, r *gittest.Repo) {
				data, err := os.ReadFile(codeownersFile(r))
				require.NoError(t, err)
				r.Write(codeownersPath, strings.ReplaceAll(string(data), "\n", "\r\n"))
				r.Commit("crlf")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the file differs"},
		},
		{
			name: "a source committed as a link to a file outside the tree",
			change: func(t *testing.T, r *gittest.Repo) {
				outside := filepath.Join(t.TempDir(), "ask-first.yaml")
				require.NoError(t, os.WriteFile(outside, []byte(fixtureList), 0o644))
				blob := r.WriteObject("blob", outside)
				r.Git("update-index", "--cacheinfo", "120000,"+blob+","+askFirstPath)
				r.Git("commit", "--quiet", "--message", "a link out")
			},
			code: exitError,
			want: []string{"ci: .github/ask-first.yaml is not a regular file; a source of the generators is never a link"},
		},
		{
			name: "a source committed as a link to itself",
			change: func(t *testing.T, r *gittest.Repo) {
				blob := r.WriteObject("blob", "ask-first.yaml")
				r.Git("update-index", "--cacheinfo", "120000,"+blob+","+askFirstPath)
				r.Git("commit", "--quiet", "--message", "a loop")
			},
			code: exitError,
			want: []string{"ci: .github/ask-first.yaml is not a regular file"},
		},
		{
			name: "a weakened file behind a replace ref to the good one",
			change: func(t *testing.T, r *gittest.Repo) {
				good := r.Git("rev-parse", "HEAD:"+codeownersPath)
				r.Write(codeownersPath, "# nothing asks first\n")
				r.Commit("weaken")
				r.Git("replace", r.Git("rev-parse", "HEAD:"+codeownersPath), good)
				// A git that honours replace refs, as git does unless told
				// otherwise, reads the good file at HEAD.
				plain := exec.CommandContext(t.Context(), "git", "cat-file", "blob", "HEAD:"+codeownersPath)
				plain.Dir, plain.Env = r.Dir, r.Env
				replaced, err := plain.Output()
				require.NoError(t, err)
				require.Equal(t, r.Git("cat-file", "blob", good)+"\n", string(replaced), "the replace ref is in force")
			},
			code: exitFail,
			want: []string{".github/CODEOWNERS: generated: the file differs"},
		},
		{
			name: "a source that cannot be read, committed",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(askFirstPath, "version: [\n")
				r.Commit("break the list")
			},
			code: exitError,
			want: []string{"ci: .github/ask-first.yaml: yaml:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			r.Commit("fixture")
			if tt.change != nil {
				tt.change(t, r.For(t))
			}
			code, out := runCI(t, r, nil, nil, "generated")
			assert.Equal(t, tt.code, code, out)
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			for _, w := range tt.lacks {
				assert.NotContains(t, out, w)
			}
		})
	}
}

// TestGeneratedGitErrors covers the step when git cannot read the
// commit: an id that names no object, and a directory that is no
// repository. Each is an error, never a pass.
func TestGeneratedGitErrors(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	_, err := generated(t.Context(), r.Repo, "0123456789012345678901234567890123456789")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git ls-tree")

	outside := r.Repo
	outside.Dir = t.TempDir()
	outside.Env = append(slices.Clone(r.Env), "GIT_CEILING_DIRECTORIES="+filepath.Dir(outside.Dir))
	_, err = generated(t.Context(), outside, "HEAD")
	require.Error(t, err)

	empty := gittest.New(t)
	code, out := runCI(t, empty, nil, nil, "generated")
	assert.Equal(t, exitError, code, out)
	assert.Contains(t, out, "ci: git rev-parse", "a repository without a commit has nothing to judge")
}

// TestCommitTree checks the file system of a commit: the content and
// the type of each entry from its git mode, with the mode itself in
// Sys, a link as an irregular file that is never followed, and
// directories that hold only what the commit holds.
func TestCommitTree(t *testing.T) {
	r := newTree(t)
	r.Write("bin/run", "#!/bin/sh\n")
	require.NoError(t, os.Chmod(filepath.Join(r.Dir, "bin", "run"), 0o755))
	r.Commit("fixture")
	blob := r.WriteObject("blob", "README.md")
	r.Git("update-index", "--add", "--cacheinfo", "120000,"+blob+",link")
	r.Git("commit", "--quiet", "--message", "a link")
	head := r.Git("rev-parse", "HEAD")
	r.Write("untracked.md", "not in the commit\n")

	files, err := headTree(t.Context(), r.Repo, head)
	require.NoError(t, err)
	tree, err := commitTree(files)
	require.NoError(t, err)
	for path, want := range map[string]struct {
		sys  string
		mode fs.FileMode
	}{
		"README.md": {"100644", 0o644},
		"bin/run":   {"100755", 0o755},
		"link":      {"120000", fs.ModeIrregular | 0o777},
	} {
		require.Contains(t, tree, path)
		assert.Equal(t, want.sys, tree[path].Sys, path)
		assert.Equal(t, want.mode, tree[path].Mode, path)
	}
	assert.NotContains(t, tree, "untracked.md")
	info, err := fs.Lstat(tree, "link")
	require.NoError(t, err)
	assert.Equal(t, fs.ModeIrregular, info.Mode().Type(), "a link is never followed")
	data, err := fs.ReadFile(tree, "link")
	require.NoError(t, err)
	assert.Equal(t, "README.md", string(data), "a link reads as its target, not as the file it names")
	info, err = fs.Stat(tree, "docs/adr")
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestGeneratedFromASubdirectory checks that the step finds the top of
// the working tree, and so the sources, from a subdirectory.
func TestGeneratedFromASubdirectory(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	r.Dir = filepath.Join(r.Dir, "docs")
	code, out := runCI(t, r, nil, nil, "generated")
	assert.Equal(t, exitOK, code, out)
}

func TestGeneratedUsage(t *testing.T) {
	r := newTree(t)
	r.Commit("fixture")
	code, out := runCI(t, r, nil, nil, "generated", "stray")
	assert.Equal(t, exitError, code, out)
	assert.Contains(t, out, "generated takes no argument")
}
