// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

var (
	passing = step{name: "passes", argv: []string{"go", "version"}}
	failing = step{name: "fails", argv: []string{"go", "no-such-command"}}
)

// TestFast covers fast without arguments: the steps and hygiene, and no
// range. The case with a made-up entry is the exit 0 the task's Verify
// line names; the case without the denylist is its non-zero half.
func TestFast(t *testing.T) {
	tests := []struct {
		name   string
		steps  []step
		change func(t *testing.T, r *gittest.Repo)
		code   int
		want   []string
	}{
		{
			name:  "a denylist with an entry, every step passing",
			steps: []step{passing},
			code:  exitOK,
			want:  []string{"ok    passes", "ok    hygiene"},
		},
		{
			name:  "a missing denylist",
			steps: []step{passing},
			change: func(t *testing.T, r *gittest.Repo) {
				if err := os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))); err != nil {
					t.Fatal(err)
				}
			},
			code: exitFail,
			want: []string{"ok    passes", "FAIL  hygiene", "denylist: the denylist is missing"},
		},
		{
			name:  "an empty denylist",
			steps: []step{passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(names.Path, string(denylist(t)))
			},
			code: exitFail,
			want: []string{"FAIL  hygiene", "denylist: the denylist has no entry"},
		},
		{
			name:  "a failing step does not hide the ones after it",
			steps: []step{failing, passing},
			code:  exitFail,
			want:  []string{"FAIL  fails", "ok    passes", "ok    hygiene"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			if tt.change != nil {
				tt.change(t, r)
			}
			r.Commit("fixture")
			code, out := runCI(t, r, nil, tt.steps, "fast")
			if code != tt.code {
				t.Errorf("exit = %d, want %d", code, tt.code)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("the output lacks %q", w)
				}
			}
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastPushedRange covers fast as the pre-push hook calls it: with
// the remote's name and URL, and one line per pushed ref on stdin.
func TestFastPushedRange(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	origin := gittest.NewBare(t)
	r := newTree(t)
	r.Git("remote", "add", "origin", origin.Dir)
	base := r.Commit("fixture")
	r.Git("push", "--quiet", "origin", "main")

	// Each branch leaves base with one commit, and main stays clean, so
	// hygiene passes in every case and only the range decides.
	branch := func(name string, commit func() string) string {
		t.Helper()
		r.Git("checkout", "--quiet", "-b", name, base)
		sha := commit()
		r.Git("checkout", "--quiet", "main")
		return sha
	}
	clean := branch("clean", func() string {
		r.Write("docs/page.md", "plain\n")
		return r.Commit("docs: add a page")
	})
	message := branch("message", func() string {
		return r.Commit("docs: a subject\n\nabout Zorvex Quimby")
	})
	author := branch("author", func() string {
		return r.WithEnv("GIT_AUTHOR_NAME=Zorvex Quimby").Commit("docs: by another author")
	})
	committer := branch("committer", func() string {
		return r.WithEnv("GIT_COMMITTER_EMAIL=zorvex.quimby@example.invalid").Commit("docs: by another committer")
	})
	path := branch("path", func() string {
		r.Write("docs/zorvex-quimby.md", "plain\n")
		return r.Commit("docs: add a path")
	})
	line := branch("line", func() string {
		r.Write("docs/page.md", "one\nzorvex_quimby\n")
		return r.Commit("docs: add a line")
	})
	var removed string
	branch("removed", func() string {
		r.Write("docs/page.md", "one\nzorvexquimby\n")
		removed = r.Commit("docs: add a line")
		r.Write("docs/page.md", "one\n")
		return r.Commit("docs: remove it again")
	})
	removedTip := r.Git("rev-parse", "removed")
	pathLine := branch("pathline", func() string {
		r.Write("docs/zorvex-quimby.md", "zorvexquimby\n")
		return r.Commit("docs: add a path and a line")
	})
	// known holds the name and is, as far as this clone knows, on
	// origin already; child is a clean commit on top of it.
	known := branch("known", func() string {
		return r.Commit("docs: a known subject\n\nabout Zorvex Quimby")
	})
	r.Git("update-ref", "refs/remotes/origin/known", known)
	r.Git("checkout", "--quiet", "-b", "child", known)
	r.Write("docs/page.md", "plain\n")
	child := r.Commit("docs: add a page")
	r.Git("checkout", "--quiet", "main")

	push := func(local, remoteRef, remoteSHA string) string {
		return fmt.Sprintf("refs/heads/x %s %s %s\n", local, remoteRef, remoteSHA)
	}
	tests := []struct {
		name   string
		stdin  string
		code   int
		want   []string
		remote string // the hook's first argument; origin when empty
	}{
		{name: "a line in a path that is withheld", stdin: push(pathLine, "refs/heads/pathline", zero), code: exitFail,
			want: []string{"commit " + pathLine + ": added path (withheld): name:", "commit " + pathLine + ": (path withheld):1: name:"}},
		{name: "the remote's name selects its tracking refs: a commit origin has is not read",
			stdin: push(child, "refs/heads/child", zero), code: exitOK},
		{name: "the URL is not the remote's name: read as a push to no remote",
			stdin: push(child, "refs/heads/child", zero), code: exitFail, remote: origin.Dir,
			want: []string{"commit " + known + ": message line 3: name:"}},
		{"a clean push", push(clean, "refs/heads/clean", zero), exitOK, []string{"ok    pushed range"}, ""},
		{"nothing to push", "", exitOK, []string{"ok    pushed range"}, ""},
		{"a remote ref name", push(clean, "refs/heads/zorvex-quimby", zero), exitFail,
			[]string{"FAIL  pushed range", "pushed ref 1: name: the remote ref name holds a name listed at HEAD"}, ""},
		{"a ref name with a slash between the halves", push(clean, "refs/heads/zorvex/quimby", zero), exitOK, nil, ""},
		{"a commit message", push(message, "refs/heads/message", zero), exitFail,
			[]string{"commit " + message + ": message line 3: name:"}, ""},
		{"an author", push(author, "refs/heads/author", zero), exitFail,
			[]string{"commit " + author + ": author: name:"}, ""},
		{"a committer", push(committer, "refs/heads/committer", zero), exitFail,
			[]string{"commit " + committer + ": committer: name:"}, ""},
		{"an added path", push(path, "refs/heads/path", zero), exitFail,
			[]string{"commit " + path + ": added path (withheld): name:"}, ""},
		{"an added line", push(line, "refs/heads/line", zero), exitFail,
			[]string{"commit " + line + ": docs/page.md:2: name:"}, ""},
		{"a line one commit adds and the next removes", push(removedTip, "refs/heads/removed", zero), exitFail,
			[]string{"commit " + removed + ": docs/page.md:2: name:"}, ""},
		{"a deleted ref with a listed name", fmt.Sprintf("(delete) %s refs/heads/zorvex-quimby %s\n", zero, base), exitOK, nil, ""},
		{"input that is not a hook's", "one two\n", exitError, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote := tt.remote
			if remote == "" {
				remote = "origin"
			}
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []step{passing}, "fast", remote, origin.Dir)
			if code != tt.code {
				t.Errorf("exit = %d, want %d", code, tt.code)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("the output lacks %q", w)
				}
			}
			if strings.Contains(strings.ToLower(out), "quimby") {
				t.Error("the output repeats the matched text")
			}
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastPushedRangeDenylists covers which denylist judges a push: the
// one at HEAD together with the one at each pushed tip. A push of a
// branch that is not checked out is judged by the entries that branch
// adds, and a pushed annotated tag by its own text.
func TestFastPushedRangeDenylists(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	const other = "blorptang wixelfrum" // a second made-up name, listed on a branch only
	origin := gittest.NewBare(t)
	r := newTree(t)
	r.Git("remote", "add", "origin", origin.Dir)
	base := r.Commit("fixture")
	r.Git("push", "--quiet", "origin", "main")

	branch := func(name string, commit func() string) string {
		t.Helper()
		r.Git("checkout", "--quiet", "-b", name, base)
		sha := commit()
		r.Git("checkout", "--quiet", "main")
		return sha
	}
	// listing adds an entry to the denylist and, in the same commit, a
	// line that holds the name the entry lists.
	listing := branch("listing", func() string {
		r.Write(names.Path, string(denylist(t, madeUp, other)))
		r.Write("docs/b.txt", "x blorptang-wixelfrum y\n")
		return r.Commit("docs: add a page and an entry")
	})
	r.Git("tag", "--annotate", "--message", "a tag", "v-listing", listing)
	listingTag := r.Git("rev-parse", "refs/tags/v-listing")
	// unlisted has no denylist; one of its commits holds the name main lists.
	unlisted := branch("unlisted", func() string {
		r.Git("rm", "--quiet", names.Path)
		return r.Commit("ci: drop the denylist")
	})
	unlistedLine := branch("unlistedline", func() string {
		r.Git("rm", "--quiet", names.Path)
		r.Write("docs/b.txt", "x zorvex-quimby y\n")
		return r.Commit("docs: add a page, drop the denylist")
	})
	broken := branch("broken", func() string {
		r.Write(names.Path, "entries: [\n")
		r.Write("docs/b.txt", "x zorvex-quimby y\n")
		return r.Commit("ci: damage the denylist")
	})
	// The text of an annotated tag: its message, its tagger and the
	// name it was created with, which the push may not use.
	clean := branch("clean", func() string {
		r.Write("docs/page.md", "plain\n")
		return r.Commit("docs: add a page")
	})
	tagID := func(name string, env []string, args ...string) string {
		t.Helper()
		r.WithEnv(env...).Git(append([]string{"tag", "--annotate"}, append(args, name, clean)...)...)
		return r.Git("rev-parse", "refs/tags/"+name)
	}
	tagMessage := tagID("v-message", nil, "--message", "release\n\nnotes about Zorvex Quimby")
	tagTagger := tagID("v-tagger", []string{"GIT_COMMITTER_NAME=Zorvex Quimby"}, "--message", "release")
	tagName := tagID("zorvex-quimby", nil, "--message", "release")
	tagClean := tagID("v-clean", nil, "--message", "release")
	tagSigned := tagID("v-signed", nil, "--message", "release\n-----BEGIN SSH SIGNATURE-----\nU1NIU0lHzorvexquimbyAAAA\n-----END SSH SIGNATURE-----")
	// inrange lists a second name in one commit and drops it in the next;
	// the line that holds it stays.
	inRange := branch("inrange", func() string {
		r.Write(names.Path, string(denylist(t, madeUp, other)))
		r.Write("docs/c.txt", "x blorptang-wixelfrum y\n")
		r.Commit("docs: add a page and an entry")
		r.Write(names.Path, string(denylist(t, madeUp)))
		return r.Commit("ci: drop the entry again")
	})
	// merged embeds a signed tag, which the push does not send as a ref.
	side := branch("side", func() string {
		r.Write("docs/side.md", "plain\n")
		return r.Commit("docs: add a side page")
	})
	merged := r.WriteObject("commit", "tree "+r.Git("rev-parse", side+"^{tree}")+"\n"+
		"parent "+base+"\nparent "+side+"\n"+
		"author Test Author <author@example.invalid> 1767323045 +0000\n"+
		"committer Test Committer <committer@example.invalid> 1767323045 +0000\n"+
		"mergetag object "+side+"\n type commit\n tag v-side\n tagger Test Committer <committer@example.invalid> 1767323045 +0000\n \n"+
		" release\n \n about Zorvex Quimby\n -----BEGIN SSH SIGNATURE-----\n U1NIU0lHAAAA\n -----END SSH SIGNATURE-----\n"+
		"\nmerge the side\n")
	// escaped and newline add a line with the name to a path that holds
	// an escape sequence or a newline: the path is printed, quoted.
	escaped := branch("escaped", func() string {
		r.Write("docs/a\x1b[2Jb.md", "zorvexquimby\n")
		return r.Commit("docs: add a page")
	})
	newline := branch("newline", func() string {
		r.Write("docs/a\nok    pushed range\n.md", "zorvexquimby\n")
		return r.Commit("docs: add a page")
	})
	// replaced holds the name, and a replace ref shows a clean commit in
	// its place; the push sends replaced itself.
	replaced := branch("replaced", func() string {
		r.Write("docs/r.md", "x zorvex-quimby y\n")
		return r.Commit("docs: add a page")
	})
	standIn := branch("standin", func() string {
		r.Write("docs/r.md", "clean\n")
		return r.Commit("docs: add a page")
	})
	r.Git("replace", replaced, standIn)
	// quoted adds a path git quotes in a patch, with a byte outside ASCII
	// between the two halves of the name.
	quoted := branch("quoted", func() string {
		r.Write("docs/zorvex\u00e9quimby.md", "zorvexquimby\n")
		return r.Commit("docs: add a page")
	})

	push := func(local, remoteRef string) string {
		return fmt.Sprintf("refs/heads/x %s %s %s\n", local, remoteRef, zero)
	}
	tests := []struct {
		name  string
		head  string // the branch checked out; main when empty
		stdin string
		code  int
		want  []string
		lacks []string
	}{
		{name: "an entry the pushed tip adds, with HEAD on another branch",
			stdin: push(listing, "refs/heads/listing"), code: exitFail,
			want: []string{"ok    hygiene", "FAIL  pushed range",
				"commit " + listing + ": docs/b.txt:1: name: the added line holds a name listed at a pushed tip"}},
		{name: "the same tip pushed as an annotated tag",
			stdin: push(listingTag, "refs/tags/v-listing"), code: exitFail,
			want: []string{"commit " + listing + ": docs/b.txt:1: name: the added line holds a name listed at a pushed tip"}},
		{name: "the same tip after a clean ref: every tip is read before the walk",
			stdin: push(clean, "refs/heads/clean") + push(listing, "refs/heads/listing"), code: exitFail,
			want: []string{"commit " + listing + ": docs/b.txt:1: name:"}},
		{name: "a tip without a denylist adds nothing",
			stdin: push(unlisted, "refs/heads/unlisted"), code: exitOK,
			want: []string{"ok    pushed range"}},
		{name: "a tip without a denylist is judged by the one at HEAD",
			stdin: push(unlistedLine, "refs/heads/unlistedline"), code: exitFail,
			want: []string{"commit " + unlistedLine + ": docs/b.txt:1: name: the added line holds a name listed at HEAD"}},
		{name: "a tip with a denylist that cannot be read stops the push",
			stdin: push(broken, "refs/heads/broken"), code: exitError,
			want: []string{"ci: " + names.Path + " at pushed tip " + broken + ": "}, lacks: []string{"pushed range"}},
		{name: "a deleted ref names no tip",
			stdin: fmt.Sprintf("(delete) %s refs/heads/listing %s\n", zero, listing), code: exitOK,
			want: []string{"ok    pushed range"}},
		{name: "no denylist at HEAD: the tip's still judges the range", head: "unlisted",
			stdin: push(listing, "refs/heads/listing"), code: exitFail,
			want: []string{"FAIL  hygiene", "FAIL  pushed range", "commit " + listing + ": docs/b.txt:1: name:"}},
		{name: "no denylist at HEAD or at the tip: the range is not checked", head: "unlisted",
			stdin: push(unlistedLine, "refs/heads/unlistedline"), code: exitFail,
			want: []string{"FAIL  hygiene", "--    pushed range: not checked"}, lacks: []string{"ok    pushed range"}},
		{name: "a tag message",
			stdin: push(tagMessage, "refs/tags/v-message"), code: exitFail,
			want: []string{"tag " + tagMessage + ": message line 3: name: the line holds a name listed at HEAD"}},
		{name: "a tagger",
			stdin: push(tagTagger, "refs/tags/v-tagger"), code: exitFail,
			want: []string{"tag " + tagTagger + ": tagger: name: the name and email hold a name listed at HEAD"}},
		{name: "the name a tag was created with, pushed under another",
			stdin: push(tagName, "refs/tags/v-other"), code: exitFail,
			want:  []string{"tag " + tagName + ": tag name: name: the tag's name holds a name listed at HEAD"},
			lacks: []string{"pushed ref 1"}},
		{name: "a signature line of a tag",
			stdin: push(tagSigned, "refs/tags/v-signed"), code: exitFail,
			want: []string{"tag " + tagSigned + ": message line 3: name: the line holds a name listed at HEAD"}},
		{name: "an entry listed inside the range and dropped at its tip is not applied",
			stdin: push(inRange, "refs/heads/inrange"), code: exitOK,
			want: []string{"ok    pushed range"}},
		{name: "a signed tag a merge commit embeds",
			stdin: push(merged, "refs/heads/merged"), code: exitFail,
			want: []string{"commit " + merged + ": mergetag 1 message line 3: name: the line holds a name listed at HEAD"}},
		{name: "a path git quotes is withheld",
			stdin: push(quoted, "refs/heads/quoted"), code: exitFail,
			want: []string{"commit " + quoted + ": added path (withheld): name:", "commit " + quoted + ": (path withheld):1: name:"}},
		{name: "a path with an escape sequence is quoted",
			stdin: push(escaped, "refs/heads/escaped"), code: exitFail,
			want: []string{"commit " + escaped + ": \"docs/a\\x1b[2Jb.md\":1: name:"}, lacks: []string{"\x1b"}},
		{name: "a path with a newline is quoted and forges no line",
			stdin: push(newline, "refs/heads/newline"), code: exitFail,
			want:  []string{"commit " + newline + ": \"docs/a\\nok    pushed range\\n.md\":1: name:", "FAIL  pushed range"},
			lacks: []string{"\nok    pushed range"}},
		{name: "a replace ref does not hide a commit",
			stdin: push(replaced, "refs/heads/replaced"), code: exitFail,
			want: []string{"commit " + replaced + ": docs/r.md:1: name:"}},
		{name: "a clean tag",
			stdin: push(tagClean, "refs/tags/v-clean"), code: exitOK,
			want: []string{"ok    pushed range"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.head != "" {
				r.Git("checkout", "--quiet", tt.head)
				defer r.Git("checkout", "--quiet", "main")
			}
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []step{passing}, "fast", "origin", origin.Dir)
			if code != tt.code {
				t.Errorf("exit = %d, want %d", code, tt.code)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("the output lacks %q", w)
				}
			}
			for _, w := range tt.lacks {
				if strings.Contains(out, w) {
					t.Errorf("the output holds %q", w)
				}
			}
			if low := strings.ToLower(out); strings.Contains(low, "quimby") || strings.Contains(low, "wixelfrum") {
				t.Error("the output repeats the matched text")
			}
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastSteps pins the steps fast runs to the In fast column of 10
// 10.2, for the steps that have code to check today.
func TestFastSteps(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := fastSteps(root)
	want := []step{
		{name: "format", argv: []string{"gofmt", "-l", "."}, quiet: true},
		{name: "vet", argv: []string{"go", "vet", "./..."}},
		{name: "unit", argv: []string{"go", "test", "./tools/..."}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("steps = %v, want %v", got, want)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	unit := fastSteps(root)[2].argv
	if want := []string{"go", "test", "./internal/...", "./tools/..."}; !reflect.DeepEqual(unit, want) {
		t.Errorf("unit = %v, want %v", unit, want)
	}
}

func TestStepRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nvar   X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		step step
		ok   bool
	}{
		{"a command that exits 0", passing, true},
		{"a command that exits non-zero", failing, false},
		{"a command that is not installed", step{name: "x", argv: []string{"no-such-program-here"}}, false},
		{"gofmt lists a file: output is a failure", step{name: "format", argv: []string{"gofmt", "-l", "."}, quiet: true}, false},
		{"a quiet command that fails", step{name: "x", argv: []string{"gofmt", "-l", "nothing.go"}, quiet: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.step.run(t.Context(), dir, nil); (err == nil) != tt.ok {
				t.Errorf("run error = %v, want ok %v", err, tt.ok)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nvar X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (step{name: "format", argv: []string{"gofmt", "-l", "."}, quiet: true}).run(t.Context(), dir, nil); err != nil {
		t.Errorf("gofmt on a formatted tree: %v", err)
	}
}
