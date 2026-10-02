// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-or-later

package pushed

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

const zero = "0000000000000000000000000000000000000000"

// fixture is a repository with one remote, origin, that holds commit a
// on main. Locally main goes on to b and c, feature leaves b with d,
// and m merges feature into main.
//
//	a - b - c - m   main
//	     \     /
//	      d ---     feature
type fixture struct {
	repo          *gittest.Repo
	a, b, c, d, m string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	origin := gittest.NewBare(t)
	r := gittest.New(t)
	f := fixture{repo: r}
	r.Git("remote", "add", "origin", origin.Dir)
	r.Write("a.txt", "a\n")
	f.a = r.Commit("a")
	r.Git("push", "--quiet", "origin", "main")
	r.Write("b.txt", "b\n")
	f.b = r.Commit("b")
	r.Git("branch", "feature")
	r.Write("c.txt", "c\n")
	f.c = r.Commit("c")
	r.Git("checkout", "--quiet", "feature")
	r.Write("d.txt", "d one\nd two\n")
	f.d = r.Commit("d")
	r.Git("checkout", "--quiet", "main")
	r.Git("merge", "--quiet", "--no-ff", "--message", "m", "feature")
	f.m = r.Git("rev-parse", "HEAD")
	return f
}

// walk runs Walk and returns the commits it read, sorted, and the ref
// names it read, in order. A commit is read once, however many pushed
// refs reach it; the author reading comes once per read, so it counts
// them.
func walk(t *testing.T, f fixture, remote, stdin string) (commits, refs []string, err error) {
	t.Helper()
	reads := map[string]int{}
	err = Walk(context.Background(), f.repo.Repo, remote, strings.NewReader(stdin), func(rd Reading) {
		switch rd.Field {
		case FieldRef:
			refs = append(refs, string(rd.Text))
		case FieldAuthor:
			reads[rd.Commit]++
		}
	})
	for c, n := range reads {
		if n != 1 {
			t.Errorf("commit %s was read %d times, want once", c, n)
		}
		commits = append(commits, c)
	}
	slices.Sort(commits)
	return commits, refs, err
}

func sorted(s ...string) []string {
	slices.Sort(s)
	return s
}

// TestWalkRanges is the fixture-repository table of 10 10.2: which
// commits a push is read for, case by case.
func TestWalkRanges(t *testing.T) {
	f := newFixture(t)
	missing := strings.Repeat("12", 20) // a commit id the fixture does not hold
	f.repo.Git("tag", "--annotate", "--message", "a tag", "v-test", f.c)
	tag := f.repo.Git("rev-parse", "refs/tags/v-test")
	tests := []struct {
		name    string
		remote  string
		stdin   string
		commits []string
		refs    []string
	}{
		{
			name:    "a ref the remote has: the commits after the remote sha",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.c, f.a),
			commits: sorted(f.b, f.c),
			refs:    []string{"refs/heads/main"},
		},
		{
			name:    "a new ref: the commits no remote-tracking ref reaches",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, zero),
			commits: sorted(f.b, f.d),
			refs:    []string{"refs/heads/feature"},
		},
		{
			name:    "a remote sha the local repository lacks: the same fallback",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.c, missing),
			commits: sorted(f.b, f.c),
			refs:    []string{"refs/heads/main"},
		},
		{
			name:    "a URL that is no remote: every commit the local sha reaches",
			remote:  "https://example.invalid/some/repository.git",
			stdin:   fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, zero),
			commits: sorted(f.a, f.b, f.d),
			refs:    []string{"refs/heads/feature"},
		},
		{
			name:    "a URL that is no remote, with a remote sha the repository holds",
			remote:  "https://example.invalid/some/repository.git",
			stdin:   fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, f.b),
			commits: sorted(f.d),
			refs:    []string{"refs/heads/feature"},
		},
		{
			name:    "a merge commit: the merge and both sides",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.m, f.a),
			commits: sorted(f.b, f.c, f.d, f.m),
			refs:    []string{"refs/heads/main"},
		},
		{
			name:    "a merge commit whose parents the remote has: the merge alone",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.m, f.c) + fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, f.d),
			commits: sorted(f.d, f.m),
			refs:    []string{"refs/heads/main", "refs/heads/feature"},
		},
		{
			name:    "a deleted ref: no commit and no name",
			remote:  "origin",
			stdin:   fmt.Sprintf("(delete) %s refs/heads/gone %s\n", zero, f.a),
			commits: nil,
			refs:    nil,
		},
		{
			name:    "two refs in one push",
			remote:  "origin",
			stdin:   fmt.Sprintf("(delete) %s refs/heads/gone %s\nrefs/heads/main %s refs/heads/renamed %s\n", zero, f.a, f.b, zero),
			commits: sorted(f.b),
			refs:    []string{"refs/heads/renamed"},
		},
		{
			name:   "two refs that share a commit: it is read once",
			remote: "origin",
			stdin: fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.c, f.a) +
				fmt.Sprintf("refs/heads/feature %s refs/heads/feature %s\n", f.d, zero),
			commits: sorted(f.b, f.c, f.d),
			refs:    []string{"refs/heads/main", "refs/heads/feature"},
		},
		{
			name:    "an annotated tag: the commits it leads to",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/tags/v-test %s refs/tags/v-test %s\n", tag, zero),
			commits: sorted(f.b, f.c),
			refs:    []string{"refs/tags/v-test"},
		},
		{
			name:    "nothing pushed",
			remote:  "origin",
			stdin:   "",
			commits: nil,
			refs:    nil,
		},
		{
			name:    "up to date",
			remote:  "origin",
			stdin:   fmt.Sprintf("refs/heads/main %s refs/heads/main %s\n", f.a, f.a),
			commits: nil,
			refs:    []string{"refs/heads/main"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commits, refs, err := walk(t, f, tt.remote, tt.stdin)
			if err != nil {
				t.Fatalf("Walk: %v", err)
			}
			if !reflect.DeepEqual(commits, tt.commits) {
				t.Errorf("commits = %v, want %v", commits, tt.commits)
			}
			if !reflect.DeepEqual(refs, tt.refs) {
				t.Errorf("refs = %v, want %v", refs, tt.refs)
			}
		})
	}
}

// TestWalkStops shows that input Walk cannot read, and a git command
// that fails, are errors: either one stops the push.
func TestWalkStops(t *testing.T) {
	f := newFixture(t)
	missing := strings.Repeat("12", 20)
	blob := f.repo.Git("rev-parse", f.c+":c.txt")
	tree := f.repo.Git("rev-parse", f.c+"^{tree}")
	tests := []struct {
		name  string
		stdin string
	}{
		{"a ref that points at a blob", "refs/tags/b " + blob + " refs/tags/b " + zero + "\n"},
		{"a ref that points at a tree", "refs/tags/t " + tree + " refs/tags/t " + zero + "\n"},
		{"three fields", "refs/heads/main " + f.c + " refs/heads/main\n"},
		{"a local sha that is no object id", "refs/heads/main --all refs/heads/main " + f.a + "\n"},
		{"a remote sha that is no object id", "refs/heads/main " + f.c + " refs/heads/main ^" + f.a[:39] + "\n"},
		{"a local sha the repository lacks", "refs/heads/main " + missing + " refs/heads/main " + f.a + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := walk(t, f, "origin", tt.stdin); err == nil {
				t.Error("Walk succeeded, want an error")
			}
		})
	}
}

// readings returns what Walk reads of one commit, as "field|path|line|text".
func readings(t *testing.T, r *gittest.Repo, sha string) []string {
	t.Helper()
	var out []string
	err := Readings(context.Background(), r.Repo, sha, func(rd Reading) {
		if rd.Commit != sha {
			t.Errorf("reading of commit %s, want %s", rd.Commit, sha)
		}
		out = append(out, fmt.Sprintf("%s|%s|%d|%s", rd.Field, rd.Path, rd.Line, rd.Text))
	})
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	return out
}

// TestReadings covers the four readings of a commit: the message, the
// two identities, the paths it adds or renames to, and the lines it
// adds.
func TestReadings(t *testing.T) {
	r := gittest.New(t)
	r.Write("kept.txt", "one\ntwo\nthree\n")
	r.Write("moved.txt", "a line long enough to be seen as the same file after a rename\nand a second line\n")
	root := r.Commit("root subject\n\nroot body")

	r.Write("kept.txt", "one\ntwo changed\nthree\nfour\n")
	r.Write("dir/new file.txt", "++ looks like a header\n-- so does this\nno newline at the end")
	r.Git("mv", "moved.txt", "renamed.txt")
	second := r.WithEnv("GIT_AUTHOR_NAME=Other Author", "GIT_AUTHOR_EMAIL=other@example.invalid").Commit("second")

	r.Git("rm", "--quiet", "dir/new file.txt")
	third := r.Commit("third")

	tests := []struct {
		name string
		sha  string
		want []string
	}{
		{
			name: "a root commit is compared with the empty tree",
			sha:  root,
			want: []string{
				"message||1|root subject",
				"message||2|",
				"message||3|root body",
				"author||0|Test Author <author@example.invalid>",
				"committer||0|Test Committer <committer@example.invalid>",
				"path|kept.txt|0|kept.txt",
				"path|moved.txt|0|moved.txt",
				"line|kept.txt|1|one",
				"line|kept.txt|2|two",
				"line|kept.txt|3|three",
				"line|moved.txt|1|a line long enough to be seen as the same file after a rename",
				"line|moved.txt|2|and a second line",
			},
		},
		{
			name: "added and renamed paths, added lines with their numbers",
			sha:  second,
			want: []string{
				"message||1|second",
				"author||0|Other Author <other@example.invalid>",
				"committer||0|Test Committer <committer@example.invalid>",
				"path|dir/new file.txt|0|dir/new file.txt",
				"path|renamed.txt|0|renamed.txt",
				"line|dir/new file.txt|1|++ looks like a header",
				"line|dir/new file.txt|2|-- so does this",
				"line|dir/new file.txt|3|no newline at the end",
				"line|kept.txt|2|two changed",
				"line|kept.txt|4|four",
			},
		},
		{
			name: "a commit that only removes adds no path and no line",
			sha:  third,
			want: []string{
				"message||1|third",
				"author||0|Test Author <author@example.invalid>",
				"committer||0|Test Committer <committer@example.invalid>",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readings(t, r, tt.sha)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

// TestReadingsMerge shows that a merge commit is compared with its
// first parent: what the second parent brings counts as added by the
// merge, and what the first parent already had does not.
func TestReadingsMerge(t *testing.T) {
	f := newFixture(t)
	got := readings(t, f.repo, f.m)
	want := []string{
		"message||1|m",
		"author||0|Test Author <author@example.invalid>",
		"committer||0|Test Committer <committer@example.invalid>",
		"path|d.txt|0|d.txt",
		"line|d.txt|1|d one",
		"line|d.txt|2|d two",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestReadingsUnknownCommit(t *testing.T) {
	f := newFixture(t)
	err := Readings(context.Background(), f.repo.Repo, strings.Repeat("12", 20), func(Reading) {})
	if err == nil {
		t.Error("Readings of a commit the repository lacks succeeded")
	}
}

func TestIsObjectID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"a sha1 id", strings.Repeat("a1", 20), true},
		{"a sha256 id", strings.Repeat("a1", 32), true},
		{"all zeros", zero, true},
		{"39 digits", strings.Repeat("a", 39), false},
		{"41 digits", strings.Repeat("a", 41), false},
		{"63 digits", strings.Repeat("a", 63), false},
		{"upper case", strings.Repeat("A1", 20), false},
		{"not hex", strings.Repeat("g1", 20), false},
		{"an option", "--all", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isObjectID(tt.id); got != tt.want {
				t.Errorf("isObjectID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// TestPatchErrorsHoldNoContent shows that a patch the parser cannot
// read is reported by its hunk number: neither the path nor a line of
// the file reaches the output.
func TestPatchErrorsHoldNoContent(t *testing.T) {
	tests := []struct {
		name  string
		patch string
	}{
		{"a hunk cut short", "+++ b/zorvex-quimby.txt\n@@ -0,0 +1,2 @@\n+one\n"},
		{"a line that is no hunk line", "+++ b/zorvex-quimby.txt\n@@ -0,0 +1,2 @@\n+one\nzorvex quimby\n"},
		{"a header without ranges", "+++ b/zorvex-quimby.txt\n@@ zorvex quimby @@\n"},
		{"a range that is no number", "+++ b/zorvex-quimby.txt\n@@ -1 +zorvex,quimby @@\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addedLines([]byte(tt.patch), func(string, int, []byte) {})
			if err == nil {
				t.Fatal("addedLines succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "hunk 1") {
				t.Errorf("the error does not name the hunk: %v", err)
			}
			if strings.Contains(err.Error(), "zorvex") || strings.Contains(err.Error(), "quimby") {
				t.Errorf("the error repeats the patch: %v", err)
			}
		})
	}
}
