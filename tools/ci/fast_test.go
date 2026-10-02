// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
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
			[]string{"FAIL  pushed range", "pushed ref 1: name: the remote ref name holds a listed name"}, ""},
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
			if _, err := tt.step.run(context.Background(), dir, nil); (err == nil) != tt.ok {
				t.Errorf("run error = %v, want ok %v", err, tt.ok)
			}
		})
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nvar X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (step{name: "format", argv: []string{"gofmt", "-l", "."}, quiet: true}).run(context.Background(), dir, nil); err != nil {
		t.Errorf("gofmt on a formatted tree: %v", err)
	}
}
