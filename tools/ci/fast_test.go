// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

var (
	passing = step{name: "passes", argv: []string{"go", "version"}}
	failing = step{name: "fails", argv: []string{"go", "no-such-command"}}
)

// TestFast covers fast without arguments: the steps and hygiene, and no
// range. The case with a made-up entry is the exit 0 the task's Verify
// line names; the case without the denylist is its non-zero half. The
// probe case shows that a step gets its own environment and nothing of
// this process's, the dirty case that fast by hand runs on the working
// tree as it is, the go.mod cases that a nested module stops fast before
// any check, and the last case that a machine without mise stops fast
// before any check, rather than running a step with no tool.
func TestFast(t *testing.T) {
	probe := step{name: "probe", argv: []string{"env"}, quiet: true, environ: []string{"PROBE=from-the-step"}}
	tests := []struct {
		name   string
		steps  []step
		setup  func(t *testing.T)
		change func(t *testing.T, r *gittest.Repo) // before the commit
		after  func(t *testing.T, r *gittest.Repo) // after it
		code   int
		want   []string
		lacks  []string
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
				require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
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
		{
			name:  "a step runs in its own environment, not in this process's",
			steps: []step{probe},
			setup: func(t *testing.T) {
				t.Setenv("GOFLAGS", "-run=^$")
			},
			code:  exitFail,
			want:  []string{"FAIL  probe", "PROBE=from-the-step", "ok    hygiene"},
			lacks: []string{"GOFLAGS"},
		},
		{
			name:  "by hand, a working tree that differs from HEAD runs every check",
			steps: []step{passing},
			after: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/ci/x_test.go", "package main\n")
				r.Write("README.md", "# A fixture\n\nChanged.\n")
				r.Write("docs/staged.md", "plain\n")
				r.Git("add", "docs/staged.md")
				r.Write("docs/intent.md", "plain\n")
				r.Git("add", "--intent-to-add", "docs/intent.md")
			},
			code: exitOK,
			want: []string{"ok    passes", "ok    hygiene"},
		},
		{
			name:  "a tracked go.mod below the module root",
			steps: []step{passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/inner/go.mod", "module example.invalid/inner\n")
			},
			code:  exitError,
			want:  []string{"ci: fast checks one module", "\ntools/inner/go.mod"},
			lacks: []string{"ok  ", "FAIL"},
		},
		{
			name:  "an ignored go.mod below the module root",
			steps: []step{passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(".gitignore", "inner/\n")
			},
			after: func(t *testing.T, r *gittest.Repo) {
				r.Write("inner/go.mod", "module example.invalid/inner\n")
			},
			code:  exitError,
			want:  []string{"ci: fast checks one module", "\ninner/go.mod"},
			lacks: []string{"ok  ", "FAIL"},
		},
		{
			name: "no mise on the search path: fast stops before any check",
			setup: func(t *testing.T) {
				gitPath, err := exec.LookPath("git")
				require.NoError(t, err)
				dir := t.TempDir()
				require.NoError(t, os.Symlink(gitPath, filepath.Join(dir, "git")))
				t.Setenv("PATH", dir)
			},
			code:  exitError,
			want:  []string{"ci: mise is not on the search path"},
			lacks: []string{"ok  ", "FAIL", "hygiene"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTree(t)
			if tt.change != nil {
				tt.change(t, r)
			}
			r.Commit("fixture")
			if tt.after != nil {
				tt.after(t, r)
			}
			if tt.setup != nil {
				tt.setup(t)
			}
			code, out := runCI(t, r, nil, tt.steps, "fast")
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			for _, w := range tt.lacks {
				assert.NotContains(t, out, w)
			}
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastJudgesTheCommit covers what fast refuses as the pre-push
// hook calls it, before any check: a pushed tip that is not HEAD, and a
// working tree that differs from HEAD in a file the checks read. Each
// case starts from a clean checkout of main, the one commit pushed.
func TestFastJudgesTheCommit(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	const refused = "ci: fast judges the commit a push sends"
	tests := []struct {
		name  string
		dirty func(t *testing.T, r *gittest.Repo) []string // returns more environment for git, if any
		stdin func(head, other string) string              // the hook's input; a push of main when nil
		code  int
		want  []string
		lacks []string
	}{
		{name: "a clean tree, HEAD pushed", code: exitOK, want: []string{"ok    passes", "ok    hygiene", "ok    pushed range"}},
		{name: "a staged file", code: exitError, want: []string{refused, "\ndocs/staged.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/staged.md", "plain\n")
				r.Git("add", "docs/staged.md")
				return nil
			}},
		{name: "an intent-to-add entry", code: exitError, want: []string{refused, "\ndocs/intent.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/intent.md", "plain\n")
				r.Git("add", "--intent-to-add", "docs/intent.md")
				return nil
			}},
		{name: "a modified tracked file", code: exitError, want: []string{refused, "\nREADME.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("README.md", "# A fixture\n\nChanged.\n")
				return nil
			}},
		{name: "an alternate GIT_INDEX_FILE that matches HEAD does not hide the index", code: exitError,
			want: []string{refused, "\ndocs/staged.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/staged.md", "plain\n")
				r.Git("add", "docs/staged.md")
				alt := "GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")
				r.WithEnv(alt).Git("read-tree", "HEAD")
				return []string{alt}
			}},
		{name: "an untracked test with a TestMain that exits 0", code: exitError,
			want: []string{refused, "\ntools/ci/x_test.go"}, lacks: []string{"os.Exit"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("tools/ci/x_test.go", "package main\n\nimport (\"os\"; \"testing\")\n\nfunc TestMain(*testing.M) { os.Exit(0) }\n")
				return nil
			}},
		{name: "an ignored Go file, an untracked vendor directory and mise configuration", code: exitError,
			want:  []string{"\ntools/ci/ignored.go", "\nvendor/modules.txt", "\ngo.work", "\nmise.local.toml", "\n.tool-versions"},
			lacks: []string{"notes.txt"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write(".git/info/exclude", "*.go\n")
				r.Write("tools/ci/ignored.go", "package main\n")
				r.Write("vendor/modules.txt", "# vendored\n")
				r.Write("go.work", "go 1.27.0\n")
				r.Write("mise.local.toml", "[tools]\n")
				r.Write(".tool-versions", "go 1.27.0\n")
				r.Write("docs/notes.txt", "an untracked file fast does not read\n")
				return nil
			}},
		{name: "an untracked file fast does not read", code: exitOK, want: []string{"ok    passes", "ok    hygiene"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/notes.txt", "plain\n")
				return nil
			}},
		{name: "an untracked nested repository", code: exitError, want: []string{refused, "\ntools/ci/evil/"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("tools/ci/evil/evil_test.go", "package evil\n")
				r.Git("-C", "tools/ci/evil", "init", "--quiet")
				return nil
			}},
		{name: "a pushed tip that is not HEAD", code: exitError,
			stdin: func(head, other string) string {
				return "refs/heads/other " + other + " refs/heads/other " + zero + "\n"
			},
			want: []string{"ci: fast tests the commit checked out", "check out the ref you push", "\n{other}"}},
		{name: "two pushed tips, one of them not HEAD", code: exitError,
			stdin: func(head, other string) string {
				return "refs/heads/main " + head + " refs/heads/main " + zero + "\nrefs/heads/other " + other + " refs/heads/other " + zero + "\n"
			},
			want: []string{"ci: fast tests the commit checked out"}},
		{name: "an annotated tag on HEAD", code: exitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string { return "refs/tags/v1 {tag} refs/tags/v1 " + zero + "\n" }},
		{name: "a tag of the tag on HEAD", code: exitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string { return "refs/tags/v2 {tagtag} refs/tags/v2 " + zero + "\n" }},
		{name: "a lightweight tag on HEAD", code: exitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string {
				return "refs/tags/v-light " + head + " refs/tags/v-light " + zero + "\n"
			}},
		{name: "an annotated tag on a commit that is not HEAD", code: exitError,
			stdin: func(head, other string) string {
				return "refs/tags/v-other {othertag} refs/tags/v-other " + zero + "\n"
			},
			want: []string{"ci: fast tests the commit checked out", "\n{othertag}"}},
		{name: "a tag of a tree", code: exitError, want: []string{"ci: pushed tip {treetag}: "},
			stdin: func(head, other string) string { return "refs/tags/v-tree {treetag} refs/tags/v-tree " + zero + "\n" }},
		{name: "a tag of a blob", code: exitError, want: []string{"ci: pushed tip {blobtag}: "},
			stdin: func(head, other string) string { return "refs/tags/v-blob {blobtag} refs/tags/v-blob " + zero + "\n" }},
		{name: "a staged deletion", code: exitError, want: []string{refused, "\nREADME.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Git("rm", "--quiet", "README.md")
				return nil
			}},
		{name: "an untracked go.mod below the module root", code: exitError, want: []string{"ci: fast checks one module", "\ninner/go.mod"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("inner/go.mod", "module example.invalid/inner\n")
				return nil
			}},
		{name: "a deleted ref names no tip", code: exitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string { return "(delete) " + zero + " refs/heads/other " + other + "\n" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := gittest.NewBare(t)
			r := newTree(t)
			r.Git("remote", "add", "origin", origin.Dir)
			r.Git("checkout", "--quiet", "-b", "other")
			r.Write("docs/other.md", "plain\n")
			other := r.Commit("docs: add a page")
			r.Git("checkout", "--quiet", "--orphan", "main")
			r.Git("rm", "--quiet", "-r", "--cached", "docs/other.md")
			require.NoError(t, os.Remove(filepath.Join(r.Dir, "docs", "other.md")))
			head := r.Commit("fixture")
			tag := func(name, object string) string {
				r.Git("tag", "--annotate", "--message", "a tag", name, object)
				return r.Git("rev-parse", "refs/tags/"+name)
			}
			ids := strings.NewReplacer(
				"{tag}", tag("v1", head),
				"{tagtag}", tag("v2", "v1"),
				"{othertag}", tag("v-other", other),
				"{treetag}", tag("v-tree", head+"^{tree}"),
				"{blobtag}", tag("v-blob", head+":README.md"),
			)
			r.Git("tag", "v-light", head)
			r.Git("push", "--quiet", "origin", "main")
			r.Git("fetch", "--quiet", "origin") // creates refs/remotes/origin/HEAD
			stdin := "refs/heads/main " + head + " refs/heads/main " + zero + "\n"
			if tt.stdin != nil {
				stdin = ids.Replace(tt.stdin(head, other))
			}
			fixture := r
			if tt.dirty != nil {
				fixture = r.WithEnv(tt.dirty(t, r)...)
			}
			code, out := runCI(t, fixture, strings.NewReader(stdin), []step{passing}, "fast", "origin", origin.Dir)
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, ids.Replace(strings.ReplaceAll(w, "{other}", other)))
			}
			for _, w := range tt.lacks {
				assert.NotContains(t, out, w)
			}
			if tt.code == exitError {
				assert.NotContains(t, out, "ok  ", "a refusal comes before any check")
				assert.NotContains(t, out, "git add", "a refusal never suggests staging")
			}
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastJudgesAgainAfterTheSteps covers a working tree that changes
// while the steps of a pre-push run take their minutes: a step stub
// changes it, and fast refuses the push after the last step, before
// hygiene and the range, since the steps may have tested something
// other than the commit judged. The last case pushes nothing, so no
// tip ties HEAD down and only the comparison of the two commits sees
// that HEAD moved.
func TestFastJudgesAgainAfterTheSteps(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	sh, err := exec.LookPath("sh")
	require.NoError(t, err)
	tests := []struct {
		name   string
		script string // what the step stub runs, in the top of the working tree
		push   bool   // push main; nothing is pushed when false
		want   string
	}{
		{"a tracked file is modified", "echo changed >> README.md", true, "\nREADME.md"},
		{"an untracked Go file appears", "printf 'package main\\n' > tools/ci/late_test.go", true, "\ntools/ci/late_test.go"},
		{"a go.mod appears below the module root", "mkdir inner && printf 'module example.invalid/inner\\n' > inner/go.mod", true, "\ninner/go.mod"},
		{"HEAD moves to a commit that is not the pushed tip", "git commit --quiet --allow-empty --message late", true, "fast tests the commit checked out"},
		{"HEAD moves with nothing pushed", "git commit --quiet --allow-empty --message late", false, "HEAD is now "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := gittest.NewBare(t)
			r := newTree(t)
			r.Git("remote", "add", "origin", origin.Dir)
			head := r.Commit("fixture")
			r.Git("push", "--quiet", "origin", "main")
			r.Git("fetch", "--quiet", "origin")
			stdin := ""
			if tt.push {
				stdin = "refs/heads/main " + head + " refs/heads/main " + zero + "\n"
			}
			// The stub runs with the fixture's git environment, so a
			// commit it makes has an identity and reads no configuration
			// of the machine.
			mutate := step{name: "mutates", argv: []string{sh, "-c", tt.script}, environ: r.Env}
			code, out := runCI(t, r, strings.NewReader(stdin), []step{mutate}, "fast", "origin", origin.Dir)
			assert.Equal(t, exitError, code, "exit status")
			assert.Contains(t, out, "ok    mutates")
			assert.Contains(t, out, "ci: the working tree changed while the steps ran, so they may not have tested commit "+head)
			assert.Contains(t, out, tt.want)
			assert.NotContains(t, out, "hygiene")
			assert.NotContains(t, out, "pushed range")
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestHookGitEnv pins the variables that judgeCommit's git commands go
// without: the list git itself gives for a repository's location,
// every GIT_CONFIG variable, and a GIT_OPTIONAL_LOCKS of the caller,
// which it replaces with 0.
func TestHookGitEnv(t *testing.T) {
	out, err := exec.CommandContext(t.Context(), "git", "rev-parse", "--local-env-vars").Output()
	require.NoError(t, err)
	assert.ElementsMatch(t, strings.Fields(string(out)), repoLocalEnv, "git rev-parse --local-env-vars")

	base := []string{"PATH=/bin", "HOME=/h", "GIT_AUTHOR_NAME=a", "GIT_OPTIONAL_LOCKS=1",
		"GIT_CONFIG_KEY_0=core.worktree", "GIT_CONFIG_VALUE_0=/elsewhere", "GIT_CONFIG_GLOBAL=/g", "GIT_CONFIG_NOSYSTEM=1"}
	for _, key := range repoLocalEnv {
		base = append(base, key+"=/x")
	}
	assert.Equal(t, []string{"PATH=/bin", "HOME=/h", "GIT_AUTHOR_NAME=a", "GIT_OPTIONAL_LOCKS=0"}, hookGitEnv(base))

	t.Setenv("GIT_INDEX_FILE", "/from/the/process")
	assert.NotContains(t, hookGitEnv(nil), "GIT_INDEX_FILE=/from/the/process", "nil is this process's environment")
	assert.Contains(t, hookGitEnv(nil), "GIT_OPTIONAL_LOCKS=0")
}

// checkoutTip checks out, detached and for the time of the test, the
// first pushed tip of a pre-push input, or head when it is not empty:
// fast refuses a push whose tip is not HEAD (TestFastJudgesTheCommit).
// An input that pushes nothing leaves main checked out.
func checkoutTip(t *testing.T, r *gittest.Repo, stdin, head string) {
	t.Helper()
	if head == "" {
		for line := range strings.Lines(stdin) {
			if f := strings.Fields(line); len(f) == 4 && strings.Trim(f[1], "0") != "" {
				head = f[1]
				break
			}
		}
	}
	if head == "" {
		return
	}
	sub := r.For(t)
	sub.Git("checkout", "--quiet", "--detach", head)
	t.Cleanup(func() { sub.Git("checkout", "--quiet", "main") })
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
	r.Git("fetch", "--quiet", "origin") // creates refs/remotes/origin/HEAD

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
	prologue := branch("prologue", func() string {
		r.Write("docs/Prologue.md", "plain\n")
		return r.Commit("docs: add a page")
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
		{name: "the URL is not the remote's name: no default branch is known, and the push stops",
			stdin: push(child, "refs/heads/child", zero), code: exitError, remote: origin.Dir,
			want: []string{"ci: the push names no configured remote"}},
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
		{"an added PROLOGUE.md in another case", push(prologue, "refs/heads/prologue", zero), exitFail,
			[]string{"FAIL  pushed range", "commit " + prologue + ": added path docs/Prologue.md: prologue:"}, ""},
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
			checkoutTip(t, r, tt.stdin, "")
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []step{passing}, "fast", remote, origin.Dir)
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			assert.NotContains(t, strings.ToLower(out), "quimby", "the output repeats the matched text")
			assert.NotContains(t, out, origin.Dir, "the output repeats the URL")
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastPushedRangeDefaultBranch covers the denylist at origin's
// default branch, which judges a push together with the one at HEAD:
// main lists two names, and each pushed commit drops the second entry
// and holds that name, in a file, in its message or in its identity,
// so only the default branch's list refuses it. The default branch is
// refs/remotes/origin/HEAD, which a case points at another commit or
// deletes; without it, or with a list there that cannot be read, the
// push stops.
func TestFastPushedRangeDefaultBranch(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	const other = "blorptang wixelfrum" // the entry each pushed commit drops
	origin := gittest.NewBare(t)
	r := newTree(t)
	r.Write(names.Path, string(denylist(t, madeUp, other)))
	r.Git("remote", "add", "origin", origin.Dir)
	base := r.Commit("fixture")
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin") // creates refs/remotes/origin/HEAD
	require.Equal(t, "refs/remotes/origin/main", r.Git("symbolic-ref", "refs/remotes/origin/HEAD"))

	branch := func(name string, commit func() string) string {
		t.Helper()
		r.Git("checkout", "--quiet", "-b", name, base)
		sha := commit()
		r.Git("checkout", "--quiet", "main")
		return sha
	}
	drop := func() { r.Write(names.Path, string(denylist(t, madeUp))) }
	line := branch("line", func() string {
		drop()
		r.Write("docs/b.txt", "x blorptang-wixelfrum y\n")
		return r.Commit("ci: drop an entry")
	})
	message := branch("message", func() string {
		drop()
		return r.Commit("ci: drop an entry\n\nabout Blorptang Wixelfrum")
	})
	identity := branch("identity", func() string {
		drop()
		return r.WithEnv("GIT_AUTHOR_NAME=Blorptang Wixelfrum").Commit("ci: drop an entry")
	})
	clean := branch("clean", func() string {
		drop()
		return r.Commit("ci: drop an entry")
	})
	// Commits the default branch is pointed at.
	broken := branch("broken", func() string {
		r.Write(names.Path, "entries: [\n")
		return r.Commit("ci: damage the denylist")
	})
	unlisted := branch("unlisted", func() string {
		r.Git("rm", "--quiet", names.Path)
		return r.Commit("ci: drop the denylist")
	})
	none := branch("none", func() string {
		r.Git("rm", "--quiet", names.Path)
		r.Write("docs/b.txt", "x zorvex-quimby y\n")
		return r.Commit("docs: add a page, drop the denylist")
	})

	push := func(local, remoteRef string) string {
		return fmt.Sprintf("refs/heads/x %s %s %s\n", local, remoteRef, zero)
	}
	const deleted = "-" // the default branch is unknown
	tests := []struct {
		name string
		// defaultBranch is the commit origin's default branch is pointed
		// at for the case; main when empty, none when deleted.
		defaultBranch string
		stdin         string
		code          int
		want          []string
	}{
		{name: "a line that holds a name the pushed commit drops",
			stdin: push(line, "refs/heads/line"), code: exitFail,
			want: []string{"ok    hygiene", "FAIL  pushed range",
				"commit " + line + ": docs/b.txt:1: name: the added line holds a name listed at the remote's default branch"}},
		{name: "a message",
			stdin: push(message, "refs/heads/message"), code: exitFail,
			want: []string{"commit " + message + ": message line 3: name: the line holds a name listed at the remote's default branch"}},
		{name: "an identity",
			stdin: push(identity, "refs/heads/identity"), code: exitFail,
			want: []string{"commit " + identity + ": author: name: the name and email hold a name listed at the remote's default branch"}},
		{name: "a clean commit that drops the entry",
			stdin: push(clean, "refs/heads/clean"), code: exitOK,
			want: []string{"ok    pushed range"}},
		{name: "a default branch without a denylist adds nothing", defaultBranch: unlisted,
			stdin: push(line, "refs/heads/line"), code: exitOK,
			want: []string{"ok    hygiene", "ok    pushed range"}},
		{name: "no entry at HEAD or at the default branch: the range is not checked", defaultBranch: unlisted,
			stdin: push(none, "refs/heads/none"), code: exitFail,
			want: []string{"FAIL  hygiene", "--    pushed range: not checked"}},
		{name: "an unknown default branch stops the push", defaultBranch: deleted,
			stdin: push(line, "refs/heads/line"), code: exitError,
			want: []string{"ci: the default branch of origin is not known here", `run "git fetch origin"`}},
		{name: "a denylist at the default branch that cannot be read stops the push", defaultBranch: broken,
			stdin: push(clean, "refs/heads/clean"), code: exitError,
			want: []string{"ci: " + names.Path + " at the default branch of origin: "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := r.For(t)
			switch tt.defaultBranch {
			case "":
			case deleted:
				sub.Git("symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
			default:
				sub.Git("update-ref", "refs/remotes/origin/stand-in", tt.defaultBranch)
				sub.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/stand-in")
			}
			t.Cleanup(func() {
				sub.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
				sub.Git("update-ref", "-d", "refs/remotes/origin/stand-in")
			})
			checkoutTip(t, r, tt.stdin, "")
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []step{passing}, "fast", "origin", origin.Dir)
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			if tt.code == exitError {
				assert.NotContains(t, out, "pushed range")
			}
			low := strings.ToLower(out)
			assert.NotContains(t, low, "quimby", "the output repeats the matched text")
			assert.NotContains(t, low, "wixelfrum", "the output repeats the matched text")
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastPushedRangeDenylists covers which denylist judges a push: the
// one at HEAD, which is the one at each pushed tip, since fast refuses
// a push of a branch that is not checked out, together with the one at
// origin's default branch, here main, which lists the made-up name
// (TestFastPushedRangeDefaultBranch). A pushed annotated tag is judged
// by its own text too.
func TestFastPushedRangeDenylists(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	const other = "blorptang wixelfrum" // a second made-up name, listed on a branch only
	origin := gittest.NewBare(t)
	r := newTree(t)
	r.Git("remote", "add", "origin", origin.Dir)
	base := r.Commit("fixture")
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin") // creates refs/remotes/origin/HEAD

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
	// unlistedLine has no denylist, and adds a line that holds the name
	// main lists.
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
		{name: "an entry a pushed branch adds, with HEAD on another branch: refused", head: "main",
			stdin: push(listing, "refs/heads/listing"), code: exitError,
			want: []string{"ci: fast tests the commit checked out", "\n" + listing}, lacks: []string{"hygiene", "pushed range"}},
		{name: "an entry the pushed tip adds",
			stdin: push(listing, "refs/heads/listing"), code: exitFail,
			want: []string{"FAIL  pushed range",
				"commit " + listing + ": docs/b.txt:1: name: the added line holds a name listed at HEAD"}},
		{name: "the same tip pushed as an annotated tag",
			stdin: push(listingTag, "refs/tags/v-listing"), code: exitFail,
			want: []string{"commit " + listing + ": docs/b.txt:1: name: the added line holds a name listed at HEAD"}},
		{name: "a clean ref and the checked-out one: every tip must be HEAD", head: listing,
			stdin: push(clean, "refs/heads/clean") + push(listing, "refs/heads/listing"), code: exitError,
			want: []string{"ci: fast tests the commit checked out", "\n" + clean}, lacks: []string{"\n" + listing, "pushed range"}},
		{name: "a denylist that cannot be read at HEAD stops the push",
			stdin: push(broken, "refs/heads/broken"), code: exitError,
			want: []string{"ci: " + names.Path + ": "}, lacks: []string{"pushed range"}},
		{name: "a deleted ref names no tip",
			stdin: fmt.Sprintf("(delete) %s refs/heads/listing %s\n", zero, listing), code: exitOK,
			want: []string{"ok    pushed range"}},
		{name: "a commit that drops the denylist is judged by the one at the default branch",
			stdin: push(unlistedLine, "refs/heads/unlistedline"), code: exitFail,
			want: []string{"FAIL  hygiene", "FAIL  pushed range",
				"commit " + unlistedLine + ": docs/b.txt:1: name: the added line holds a name listed at the remote's default branch"}},
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
			checkoutTip(t, r, tt.stdin, tt.head)
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []step{passing}, "fast", "origin", origin.Dir)
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			for _, w := range tt.lacks {
				assert.NotContains(t, out, w)
			}
			low := strings.ToLower(out)
			assert.NotContains(t, low, "quimby", "the output repeats the matched text")
			assert.NotContains(t, low, "wixelfrum", "the output repeats the matched text")
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
}

// TestFastSteps pins the steps fast runs to the In fast column of 10
// 10.2, for the steps that have code to check today. Every step runs a
// pinned tool by its path, in stepEnv; the lint steps run the resolved
// linter itself, never "mise exec", which would add the [env] table of
// a mise configuration to their environment.
func TestFastSteps(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tools"), 0o755))
	goDir := filepath.Join("/pinned", "go", "bin")
	goCmd := filepath.Join(goDir, "go")
	tools := lintTools{linter: "/pinned/golangci-lint", goDir: goDir}
	lintArgv := []string{"/pinned/golangci-lint", "run", "--config", ".golangci.yml"}
	want := []step{
		{name: "format", argv: []string{filepath.Join(goDir, "gofmt"), "-l", "."}, quiet: true, environ: stepEnv(goDir)},
		{name: "vet", argv: []string{goCmd, "vet", "./..."}, environ: stepEnv(goDir)},
		{name: "lint linux/amd64", argv: lintArgv, environ: lintEnv(lintTarget{"linux", "amd64"}, goDir)},
		{name: "lint linux/arm64", argv: lintArgv, environ: lintEnv(lintTarget{"linux", "arm64"}, goDir)},
		{name: "lint darwin/amd64", argv: lintArgv, environ: lintEnv(lintTarget{"darwin", "amd64"}, goDir)},
		{name: "lint darwin/arm64", argv: lintArgv, environ: lintEnv(lintTarget{"darwin", "arm64"}, goDir)},
		{name: "unit", argv: []string{goCmd, "test", "-count=1", "./tools/..."}, environ: stepEnv(goDir)},
	}
	assert.Equal(t, want, fastSteps(root, tools))
	for _, s := range fastSteps(root, tools) {
		assert.Contains(t, s.environ, "GOPROXY=off", "%s never reaches the network", s.name)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o755))
	var unit []string
	for _, s := range fastSteps(root, tools) {
		if s.name == "unit" {
			unit = s.argv
		}
	}
	assert.Equal(t, []string{goCmd, "test", "-count=1", "./internal/...", "./tools/..."}, unit, "unit")
}

// pinnedLintTools returns resolveLintTools for this repository, run in
// a mise state directory of the test's own, where the test first
// trusts the repository's mise configuration. mise keeps its trust
// records in the state directory (https://mise.jdx.dev/directories.html),
// so the tests do not depend on a "mise trust" of the person who runs
// them, and leave no trust behind.
func pinnedLintTools(t *testing.T) lintTools {
	t.Helper()
	root := moduleRoot(t)
	t.Setenv("MISE_STATE_DIR", t.TempDir())
	cmd := exec.CommandContext(t.Context(), "mise", "-C", root, "trust")
	cmd.Env = passThroughEnv()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "mise trust\n%s", out)
	tools, err := resolveLintTools(t.Context(), root)
	require.NoError(t, err)
	return tools
}

// TestResolveLintTools checks that the tools of fast are the ones
// mise.toml pins: the linter and the go command, installed by mise.
func TestResolveLintTools(t *testing.T) {
	tools := pinnedLintTools(t)
	assert.Equal(t, "golangci-lint", filepath.Base(tools.linter))
	assert.True(t, filepath.IsAbs(tools.linter), "an absolute path: %s", tools.linter)
	assert.FileExists(t, filepath.Join(tools.goDir, "go"))
	assert.FileExists(t, filepath.Join(tools.goDir, "gofmt"))
}

// fakeMise is a home directory with the mise installs directory of
// miseInstalls, and a directory on PATH that holds a mise of its own,
// a script, in place of the real one. HOME and PATH point at them, and
// MISE_DATA_DIR and XDG_DATA_HOME are cleared.
type fakeMise struct {
	home, bin string
}

func newFakeMise(t *testing.T) fakeMise {
	t.Helper()
	f := fakeMise{home: t.TempDir(), bin: t.TempDir()}
	require.NoError(t, os.MkdirAll(f.installs(), 0o755))
	t.Setenv("HOME", f.home)
	t.Setenv("PATH", f.bin)
	t.Setenv("MISE_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	return f
}

func (f fakeMise) installs() string {
	return filepath.Join(f.home, ".local", "share", "mise", "installs")
}

// tool creates an executable file below the installs directory.
func (f fakeMise) tool(t *testing.T, rel ...string) string {
	t.Helper()
	return executable(t, filepath.Join(append([]string{f.installs()}, rel...)...))
}

func executable(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700))
	return path
}

// script writes the fake mise: it runs prelude, then prints linter for
// "mise -C <dir> which golangci-lint" and goCmd for "... which go".
func (f fakeMise) script(t *testing.T, prelude, linter, goCmd string) {
	t.Helper()
	script := "#!/bin/sh\n" + prelude + "case \"$4\" in\ngolangci-lint) echo '" + linter + "' ;;\ngo) echo '" + goCmd + "' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte(script), 0o700))
}

// TestResolveLintToolsFailsClosed runs resolveLintTools against a fake
// mise (newFakeMise). Every case that names an error fails, and none
// falls back to a tool on the search path.
func TestResolveLintToolsFailsClosed(t *testing.T) {
	const fixed = "{installs}" // the resolved installs directory, in want
	tests := []struct {
		name string
		// setup prepares the case and returns what the fake mise prints
		// for golangci-lint and for go; no fake mise is written when
		// noMise is set, and one that exits 1 when fails is set.
		setup  func(t *testing.T, f fakeMise) (linter, goCmd string)
		noMise bool
		fails  bool
		want   string
	}{
		{name: "no mise on the search path", noMise: true, want: "mise is not on the search path"},
		{
			name:  "mise which fails, with what mise wrote to standard error",
			fails: true,
			want:  "mise which golangci-lint: exit status 1; the tool is not installed or the configuration is not trusted: run \"mise trust\" and \"mise install\" in ",
		},
		{
			name: "MISE_DATA_DIR is set",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				t.Setenv("MISE_DATA_DIR", filepath.Join(f.home, ".local", "share", "mise"))
				return f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "MISE_DATA_DIR is set: fast runs only the tools mise installs below HOME",
		},
		{
			name: "XDG_DATA_HOME is set",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				t.Setenv("XDG_DATA_HOME", filepath.Join(f.home, ".local", "share"))
				return f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "XDG_DATA_HOME is set: fast runs only the tools mise installs below HOME",
		},
		{
			name: "HOME is not set",
			setup: func(t *testing.T, _ fakeMise) (string, string) {
				t.Setenv("HOME", "")
				return "", ""
			},
			want: "HOME is not set",
		},
		{
			name: "no installs directory",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				require.NoError(t, os.Remove(f.installs()))
				return "", ""
			},
			want: "the mise installs directory " + fixed + ": ",
		},
		{
			name: "a linter outside the installs directory",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				return executable(t, filepath.Join(t.TempDir(), "golangci-lint")), f.tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "golangci-lint is outside " + filepath.Join(fixed, "golangci-lint") + ", where mise installs it: the mise configuration names a tool mise did not install",
		},
		{
			name: "a linter in the directory of another tool",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				return f.tool(t, "go", "1.27.0", "bin", "golangci-lint"), f.tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "is outside " + filepath.Join(fixed, "golangci-lint") + ",",
		},
		{
			name: "a linter in a sibling directory that shares the tool's prefix",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				return f.tool(t, "golangci-lint-evil", "2.14.0", "golangci-lint"), f.tool(t, "go", "1.27.0", "bin", "go")
			},
			want: "is outside " + filepath.Join(fixed, "golangci-lint") + ",",
		},
		{
			name: "a go below the installs directory that links outside it",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				link := filepath.Join(f.installs(), "go", "1.27.0", "bin", "go")
				require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
				require.NoError(t, os.Symlink(executable(t, filepath.Join(t.TempDir(), "go")), link))
				return f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), link
			},
			want: "mise which go: ",
		},
		{
			name: "both below the installs directory",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				return f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.tool(t, "go", "1.27.0", "bin", "go")
			},
		},
		{
			// As on macOS, where the temporary directory is below /var, a
			// link to /private/var.
			name: "both below an installs directory reached through a link",
			setup: func(t *testing.T, f fakeMise) (string, string) {
				link := filepath.Join(t.TempDir(), "home")
				require.NoError(t, os.Symlink(f.home, link))
				t.Setenv("HOME", link)
				linter := f.tool(t, "golangci-lint", "2.14.0", "golangci-lint")
				goCmd := f.tool(t, "go", "1.27.0", "bin", "go")
				return strings.Replace(linter, f.home, link, 1), strings.Replace(goCmd, f.home, link, 1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeMise(t)
			var linter, goCmd string
			if tt.setup != nil {
				linter, goCmd = tt.setup(t, f)
			}
			switch {
			case tt.fails:
				require.NoError(t, os.WriteFile(filepath.Join(f.bin, "mise"), []byte("#!/bin/sh\necho 'mise ERROR not trusted' >&2\nexit 1\n"), 0o700))
			case !tt.noMise:
				f.script(t, "", linter, goCmd)
			}
			tools, err := resolveLintTools(t.Context(), t.TempDir())
			if tt.want != "" {
				installs, evalErr := filepath.EvalSymlinks(f.installs())
				if evalErr != nil {
					installs = f.installs()
				}
				require.ErrorContains(t, err, strings.ReplaceAll(tt.want, fixed, installs))
				if tt.fails {
					require.ErrorContains(t, err, "\nmise ERROR not trusted", "what mise wrote to standard error")
				}
				assert.Equal(t, lintTools{}, tools)
				return
			}
			require.NoError(t, err)
			realLinter, err := filepath.EvalSymlinks(linter)
			require.NoError(t, err)
			realGo, err := filepath.EvalSymlinks(goCmd)
			require.NoError(t, err)
			assert.Equal(t, lintTools{linter: realLinter, goDir: filepath.Dir(realGo)}, tools)
		})
	}
}

// TestResolveLintToolsEnvironment checks that "mise which" runs in
// passThroughEnv: a fake mise writes its environment to a file, and
// every variable of passThrough reaches it and nothing else, with the
// ones a shell adds itself, whatever else this process holds. Each
// variable of passThrough is set here, so the result does not depend
// on the caller's environment (a TMPDIR or a MISE_STATE_DIR of its own).
func TestResolveLintToolsEnvironment(t *testing.T) {
	envPath, err := exec.LookPath("env")
	require.NoError(t, err)
	f := newFakeMise(t) // sets HOME and PATH
	for _, key := range []string{"GOFLAGS", "MISE_INSTALLS_DIR", "MISE_TRUSTED_CONFIG_PATHS", "CI", "MISE_ENV"} {
		t.Setenv(key, "from-the-process")
	}
	for _, key := range passThrough {
		if key != "HOME" && key != "PATH" {
			t.Setenv(key, "/from/"+key)
		}
	}
	dump := filepath.Join(t.TempDir(), "env")
	f.script(t, "'"+envPath+"' > '"+dump+"'\n", f.tool(t, "golangci-lint", "2.14.0", "golangci-lint"), f.tool(t, "go", "1.27.0", "bin", "go"))
	_, err = resolveLintTools(t.Context(), t.TempDir())
	require.NoError(t, err)

	data, err := os.ReadFile(dump)
	require.NoError(t, err)
	var keys []string
	for line := range strings.Lines(string(data)) {
		key, _, _ := strings.Cut(line, "=")
		if !slices.Contains([]string{"PWD", "OLDPWD", "SHLVL", "_"}, key) {
			keys = append(keys, key)
		}
	}
	assert.Equal(t, slices.Sorted(slices.Values(passThrough)), slices.Sorted(slices.Values(keys)), "the variables mise gets")
}

// TestMiseInstalls holds miseInstalls to the data directory that mise
// itself reports in the environment it runs mise in, and pins it on
// the variables it reads.
func TestMiseInstalls(t *testing.T) {
	t.Setenv("MISE_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	cmd := exec.CommandContext(t.Context(), "mise", "-C", t.TempDir(), "doctor", "--json")
	cmd.Env = passThroughEnv()
	out, err := cmd.Output()
	require.NoError(t, err, "mise doctor")
	var doctor struct{ Dirs struct{ Data string } }
	require.NoError(t, json.Unmarshal(out, &doctor))
	require.NotEmpty(t, doctor.Dirs.Data, "the data directory mise reports")
	installs, err := miseInstalls()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(doctor.Dirs.Data, "installs"), installs, "the installs directory of mise")

	tests := []struct {
		name            string
		data, xdg, home string
		want            string // the directory, or the start of the error
	}{
		{"below HOME", "", "", "/h", "/h/.local/share/mise/installs"},
		{"MISE_DATA_DIR set", "/m", "", "/h", "MISE_DATA_DIR is set"},
		{"XDG_DATA_HOME set", "", "/x", "/h", "XDG_DATA_HOME is set"},
		{"HOME not set", "", "", "", "HOME is not set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MISE_DATA_DIR", tt.data)
			t.Setenv("XDG_DATA_HOME", tt.xdg)
			t.Setenv("HOME", tt.home)
			got, err := miseInstalls()
			if !strings.HasPrefix(tt.want, "/") {
				require.ErrorContains(t, err, tt.want)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.FromSlash(tt.want), got)
		})
	}
}

// TestIsGateInput pins the untracked files a pre-push run refuses, a
// nested repository among them, which git lists as its directory.
func TestIsGateInput(t *testing.T) {
	for _, path := range []string{"x.go", "tools/ci/x_test.go", "go.work", "go.work.sum", "vendor/modules.txt",
		"vendor/example.com/m/m.go", "mise.toml", ".mise.toml", "mise.local.toml", ".mise.local.toml", "mise.ci.toml",
		".config/mise.toml", ".config/mise/config.toml", ".config/mise/conf.d/x.toml", ".mise/config.toml",
		"mise/config.toml", ".tool-versions", "tools/.tool-versions", "tools/ci/evil/", "nested/"} {
		assert.True(t, isGateInput(path), path)
	}
	for _, path := range []string{"README.md", "go.mod", "go.sum", "mise.lock", "docs/promise.toml", "REUSE.toml",
		"tools/vendor/notes.md", "docs/go.md", "x.gox"} {
		assert.False(t, isGateInput(path), path)
	}
}

// TestStepEnv pins the environment of the steps, written out in full:
// PATH with the directory of the pinned go first, the other variables
// of passThrough that are set, and the fixed ones, in that order and
// nothing else; the lint step adds its target. Every variable of the
// process that is outside that list stays out, whatever its name.
func TestStepEnv(t *testing.T) {
	// The fixed variables, and others that change what a step checks,
	// each set in the process to a value no step may take.
	for _, key := range []string{"GOENV", "GOOS", "GOARCH", "CGO_ENABLED", "GOTOOLCHAIN", "GOWORK", "GOPROXY",
		"GOFLAGS", "GOLANGCI_DIFF_PROCESSOR_PATCH", "GOEXPERIMENT", "MISE_INSTALLS_DIR", "MISE_DATA_DIR", "XDG_DATA_HOME",
		"CI", "MISE_TRUSTED_CONFIG_PATHS"} {
		t.Setenv(key, "from-the-process")
	}
	for _, key := range passThrough {
		t.Setenv(key, "/from/"+key)
	}
	// Two that are empty and must be left out.
	t.Setenv("TMPDIR", "")
	t.Setenv("MISE_CACHE_DIR", "")

	sep := string(os.PathListSeparator)
	located := []string{
		"HOME=/from/HOME",
		"XDG_CACHE_HOME=/from/XDG_CACHE_HOME",
		"XDG_CONFIG_HOME=/from/XDG_CONFIG_HOME",
		"XDG_STATE_HOME=/from/XDG_STATE_HOME",
		"MISE_CONFIG_DIR=/from/MISE_CONFIG_DIR",
		"MISE_STATE_DIR=/from/MISE_STATE_DIR",
		"GOPATH=/from/GOPATH",
		"GOCACHE=/from/GOCACHE",
		"GOMODCACHE=/from/GOMODCACHE",
	}
	fixed := []string{"GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOFLAGS=-mod=readonly"}
	target := []string{"GOOS=darwin", "GOARCH=amd64", "CGO_ENABLED=0"}

	assert.Equal(t, slices.Concat([]string{"PATH=/pinned/go/bin" + sep + "/from/PATH"}, located, fixed),
		stepEnv("/pinned/go/bin"), "stepEnv")
	assert.Equal(t, slices.Concat([]string{"PATH=/pinned/go/bin" + sep + "/from/PATH"}, located, fixed, target),
		lintEnv(lintTarget{"darwin", "amd64"}, "/pinned/go/bin"), "lintEnv")
	assert.Equal(t, slices.Concat([]string{"PATH=/from/PATH"}, located), passThroughEnv(), "passThroughEnv")

	// Without PATH in the process, PATH is the go directory alone.
	require.NoError(t, os.Unsetenv("PATH"))
	assert.Equal(t, slices.Concat([]string{"PATH=/pinned/go/bin"}, located, fixed), stepEnv("/pinned/go/bin"), "PATH unset")
	assert.Equal(t, located, passThroughEnv(), "passThroughEnv, PATH unset")
}

func TestStepRun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nvar   X = 1\n"), 0o644))
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
			_, err := tt.step.run(t.Context(), dir)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n\nvar X = 1\n"), 0o644))
	_, err := (step{name: "format", argv: []string{"gofmt", "-l", "."}, quiet: true}).run(t.Context(), dir)
	require.NoError(t, err, "gofmt on a formatted tree")

	// The command gets its environ and nothing else; a nil one is empty.
	t.Setenv("GOFLAGS", "-run=^$")
	out, err := (step{name: "env", argv: []string{"env"}, environ: []string{"A=1"}}).run(t.Context(), dir)
	require.NoError(t, err)
	assert.Equal(t, "A=1\n", string(out))
	out, err = (step{name: "env", argv: []string{"env"}}).run(t.Context(), dir)
	require.NoError(t, err)
	assert.Empty(t, string(out))
}

// TestFastHookIgnoresCallerGitEnv shows that hygiene and the pushed
// range read this repository when the caller sets GIT_DIR and
// GIT_ALTERNATE_OBJECT_DIRECTORIES to another one: a pre-push run
// clears the variables of repoLocalEnv for every git command it runs.
func TestFastHookIgnoresCallerGitEnv(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	r := newTree(t)
	r.Commit("fixture")
	r.Git("remote", "add", "origin", gittest.NewBare(t).Dir)
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin")
	other := gittest.New(t)
	other.Write("other.txt", "another repository\n")
	other.Commit("other")
	hooked := r.WithEnv(
		"GIT_DIR="+filepath.Join(other.Dir, ".git"),
		"GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(other.Dir, ".git", "objects"),
	)
	stdin := "refs/heads/main " + r.Git("rev-parse", "HEAD") + " refs/heads/main " + zero + "\n"
	code, out := runCI(t, hooked, strings.NewReader(stdin), []step{passing}, "fast", "origin", "https://example.invalid/x.git")
	assert.Equal(t, exitOK, code, "exit status\n%s", out)
	assert.Contains(t, out, "ok    hygiene")
	assert.Contains(t, out, "ok    pushed range")
}

// TestFastHookReadsTheProcessGitEnv is the case of a real pre-push run:
// git exports GIT_DIR and its kin to the hook, and the hook gets no
// other environment to read, so the run starts from this process's. The
// fixture's own git commands run with their own environment, set up
// before the variables are; the run under test has a nil gitEnv, and
// still judges the repository of its directory: it refuses an untracked
// nested go.mod and a name that only the default branch lists. It is not
// parallel, because t.Setenv changes this process's environment.
func TestFastHookReadsTheProcessGitEnv(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	const other = "blorptang wixelfrum" // the entry the pushed commit drops
	origin := gittest.NewBare(t)
	r := newTree(t)
	r.Write(names.Path, string(denylist(t, madeUp, other)))
	r.Git("remote", "add", "origin", origin.Dir)
	base := r.Commit("fixture")
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin")
	r.Write(names.Path, string(denylist(t, madeUp)))
	r.Write("docs/b.txt", "x blorptang-wixelfrum y\n")
	tip := r.Commit("ci: drop an entry")
	r.Git("checkout", "--quiet", "--detach", tip)
	t.Cleanup(func() { r.Git("checkout", "--quiet", "main") })
	require.NotEqual(t, base, tip)
	decoy := gittest.New(t)
	decoy.Write("decoy.txt", "another repository\n")
	decoy.Commit("decoy")

	t.Setenv("GIT_DIR", filepath.Join(decoy.Dir, ".git"))
	shallow := filepath.Join(t.TempDir(), "shallow")
	require.NoError(t, os.WriteFile(shallow, []byte(tip+"\n"), 0o600))
	t.Setenv("GIT_SHALLOW_FILE", shallow)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "no-such-index"))
	run := func() (int, string) {
		t.Helper()
		var out strings.Builder
		stdin := "refs/heads/x " + tip + " refs/heads/x " + zero + "\n"
		e := env{dir: r.Dir, stdin: strings.NewReader(stdin), stdout: &out, stderr: &out, steps: []step{passing}}
		return run(t.Context(), e, []string{"fast", "origin", origin.Dir}), out.String()
	}

	r.Write("inner/go.mod", "module example.invalid/inner\n")
	code, out := run()
	assert.Equal(t, exitError, code, "exit status\n%s", out)
	assert.Contains(t, out, "ci: fast checks one module")
	assert.Contains(t, out, "\ninner/go.mod")

	require.NoError(t, os.Remove(filepath.Join(r.Dir, "inner", "go.mod")))
	code, out = run()
	assert.Equal(t, exitFail, code, "exit status\n%s", out)
	assert.Contains(t, out, "FAIL  pushed range")
	assert.Contains(t, out, "commit "+tip+": docs/b.txt:1: name: the added line holds a name listed at the remote's default branch")
	assert.NotContains(t, strings.ToLower(out), "wixelfrum", "the output repeats the matched text")
}
