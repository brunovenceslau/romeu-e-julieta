// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pushedrange

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/hygiene"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/prose"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

// TestPushedRangeVerdicts pins the verdicts of Range, case by
// case: a verdict with findings whenever a denylist is in force or a
// finding exists, a "not checked" note whenever neither list has an
// entry, a verdict with its error for a denylist at HEAD that cannot be
// read, and an error for a remote that is not configured.
func TestPushedRangeVerdicts(t *testing.T) {
	const (
		zero       = "0000000000000000000000000000000000000000"
		name       = "pushed range"
		notChecked = "not checked against names, no denylist entry at HEAD or at the remote's default branch"
	)
	tests := []struct {
		name        string
		baseList    bool              // the default branch holds a denylist with an entry
		tip         map[string]string // files of the pushed commit; "" removes one
		noRemote    bool
		want        func(tip string) []check.Verdict // nil for the rows that end in an error
		wantErr     string
		wantVerdict string // the error of a verdict
	}{
		{
			name: "an entry at HEAD, nothing found", baseList: true,
			tip:  map[string]string{"docs/page.md": "plain\n"},
			want: func(string) []check.Verdict { return []check.Verdict{{Name: name}} },
		},
		{
			name: "an entry at HEAD, a listed name in an added line", baseList: true,
			tip: map[string]string{"docs/page.md": "see " + citest.MadeUp + "\n"},
			want: func(tip string) []check.Verdict {
				return []check.Verdict{{Name: name, Findings: []check.Finding{{Where: "commit " + tip + ": docs/page.md:1", Rule: "name", Msg: "the added line holds a name listed at HEAD"}}}}
			},
		},
		{
			name: "an entry only at the default branch", baseList: true,
			tip: map[string]string{names.Path: "", "docs/page.md": "see " + citest.MadeUp + "\n"},
			want: func(tip string) []check.Verdict {
				return []check.Verdict{{Name: name, Findings: []check.Finding{{Where: "commit " + tip + ": docs/page.md:1", Rule: "name", Msg: "the added line holds a name listed at the remote's default branch"}}}}
			},
		},
		{
			name: "no entry anywhere, nothing found",
			tip:  map[string]string{"docs/page.md": "plain\n"},
			want: func(string) []check.Verdict { return []check.Verdict{{Name: name, Note: notChecked}} },
		},
		{
			name: "no entry anywhere, an added prologue file",
			tip:  map[string]string{"sub/prologue.md": "text\n"},
			want: func(tip string) []check.Verdict {
				return []check.Verdict{
					{Name: name, Findings: []check.Finding{{Where: "commit " + tip + ": added path sub/prologue.md", Rule: "prologue", Msg: hygiene.PrologueMsg}}},
					{Name: name, Note: notChecked},
				}
			},
		},
		{
			name: "a denylist at HEAD that cannot be read", baseList: true,
			tip:         map[string]string{names.Path: "{not a denylist\n"},
			wantVerdict: names.Path,
		},
		{
			name: "a remote that is not configured", baseList: true, noRemote: true,
			tip:     map[string]string{"docs/page.md": "plain\n"},
			wantErr: "no configured remote",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := citest.NewTree(t)
			if !tt.baseList {
				require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
			}
			r.Commit("base")
			if !tt.noRemote {
				r.Git("remote", "add", "origin", gittest.NewBare(t).Dir)
				r.Git("push", "--quiet", "origin", "main")
				r.Git("fetch", "--quiet", "origin")
			}
			for path, content := range tt.tip {
				if content == "" {
					require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(path))))
					continue
				}
				r.Write(path, content)
			}
			tip := r.Commit("tip")
			push, err := pushed.Parse(strings.NewReader("refs/heads/main " + tip + " refs/heads/main " + zero + "\n"))
			require.NoError(t, err)
			got, err := Range(t.Context(), r.Repo, "origin", push, tip)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantVerdict != "" {
				require.Len(t, got, 1)
				assert.Equal(t, name, got[0].Name)
				require.ErrorContains(t, got[0].Err, tt.wantVerdict)
				return
			}
			assert.Equal(t, tt.want(tip), got)
		})
	}
}

// TestDefaultBranchDenylistOfAPartialClone is the case of a clone made
// with --filter=blob:none: after a fetch, the denylist of the default
// branch is in its tree and its blob is not in the object store. That
// is an error that says how to fetch the blob, never an empty list
// that would pass every push; once fetched as it says, the list is
// read.
func TestDefaultBranchDenylistOfAPartialClone(t *testing.T) {
	const other = "blorptang wixelfrum"
	upstream := citest.NewTree(t)
	upstream.Commit("fixture")
	clone := citest.PartialClone(t, upstream)
	upstream.Write(names.Path, string(citest.Denylist(t, citest.MadeUp, other)))
	upstream.Commit("ci: list a second name")
	_, err := clone.Run(t.Context(), nil, "fetch", "--quiet", "origin")
	require.NoError(t, err)
	list, err := defaultBranchDenylist(t.Context(), clone, "origin")
	require.Error(t, err, "a list that may be empty: %v", list)
	oid := upstream.Git("rev-parse", "HEAD:"+names.Path)
	assert.Contains(t, err.Error(), `"git cat-file -p `+oid+`"`)
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolGit,
		Argv:  []string{"git", "cat-file", "-p", oid},
		Dir:   clone.Dir,
		Env:   clone.Env,
	})
	require.NoError(t, err, "%s", out)
	list, err = defaultBranchDenylist(t.Context(), clone, "origin")
	require.NoError(t, err)
	assert.Equal(t, 2, list.Len())
}

// TestJudgedCommitIsPinned shows that hygiene.At and Range read the
// commit they are given and not HEAD: X is the commit judged, and HEAD
// has moved on to a Y that adds a listed name, or drops the denylist or
// the word lists. Each callee is asked for X and for "HEAD", and the
// two answers differ.
func TestJudgedCommitIsPinned(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	// moved returns a fixture whose commit X is clean, and the id of X;
	// the test then commits the Y that makes HEAD differ.
	moved := func(t *testing.T) (*gittest.Repo, string) {
		t.Helper()
		r := citest.NewTree(t)
		return r, r.Commit("X")
	}
	t.Run("hygiene: a name that Y adds", func(t *testing.T) {
		r, x := moved(t)
		r.Write("docs/page.md", "see "+citest.MadeUp+"\n")
		r.Commit("Y")
		got, err := hygiene.At(t.Context(), r.Repo, x)
		require.NoError(t, err)
		assert.Empty(t, got, "X is clean")
		got, err = hygiene.At(t.Context(), r.Repo, "HEAD")
		require.NoError(t, err)
		assert.Contains(t, check.Lines(got), "docs/page.md:1: name:")
	})
	t.Run("hygiene: a denylist that Y drops", func(t *testing.T) {
		r, x := moved(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
		r.Commit("Y")
		got, err := hygiene.At(t.Context(), r.Repo, x)
		require.NoError(t, err)
		assert.Empty(t, got, "X has its denylist")
		got, err = hygiene.At(t.Context(), r.Repo, "HEAD")
		require.NoError(t, err)
		assert.Contains(t, check.Lines(got), "denylist: the denylist is missing")
	})
	t.Run("hygiene: word lists that Y drops", func(t *testing.T) {
		r, x := moved(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(prose.Path))))
		r.Commit("Y")
		got, err := hygiene.At(t.Context(), r.Repo, x)
		require.NoError(t, err)
		assert.Empty(t, got)
		_, err = hygiene.At(t.Context(), r.Repo, "HEAD")
		assert.Error(t, err)
	})
	t.Run("pushedRange: a denylist that Y drops", func(t *testing.T) {
		r := citest.NewTree(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
		r.Commit("base, no denylist")
		r.Git("remote", "add", "origin", gittest.NewBare(t).Dir)
		r.Git("push", "--quiet", "origin", "main")
		r.Git("fetch", "--quiet", "origin")
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp)))
		r.Write("docs/page.md", "see "+citest.MadeUp+"\n")
		x := r.Commit("X")
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
		r.Commit("Y")
		push, err := pushed.Parse(strings.NewReader("refs/heads/main " + x + " refs/heads/main " + zero + "\n"))
		require.NoError(t, err)
		got, checked, err := citest.RangeOf(Range(t.Context(), r.Repo, "origin", push, x))
		require.NoError(t, err)
		assert.True(t, checked, "X holds an entry")
		assert.Contains(t, check.Lines(got), "the added line holds a name listed at HEAD")
		_, checked, err = citest.RangeOf(Range(t.Context(), r.Repo, "origin", push, "HEAD"))
		require.NoError(t, err)
		assert.False(t, checked, "Y has none, and neither has the default branch")
	})
}

// TestJudge checks that the pushed range judges nothing outside a
// pre-push run, and the push of the hook in one, as Range does.
func TestJudge(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	r := citest.NewTree(t)
	head := r.Commit("fixture")
	r.Git("remote", "add", "origin", gittest.NewBare(t).Dir)
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin")
	got, err := Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head})
	require.NoError(t, err)
	assert.Nil(t, got, "no hook, no verdict")
	push, err := pushed.Parse(strings.NewReader("refs/heads/main " + head + " refs/heads/main " + zero + "\n"))
	require.NoError(t, err)
	got, err = Judge(t.Context(), check.Commit{Repo: r.Repo, ID: head, Hook: &check.Hook{Remote: "origin", Push: push}})
	require.NoError(t, err)
	want, err := Range(t.Context(), r.Repo, "origin", push, head)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	require.Len(t, got, 1)
	assert.Equal(t, "pushed range", got[0].Name)
}
