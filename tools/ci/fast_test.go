// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/lint"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/pushedrange"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
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
	probe := check.Step{Name: "probe", Argv: []string{"env"}, Quiet: true, Environ: []string{"PROBE=from-the-step"}}
	tests := []struct {
		name   string
		steps  []check.Step
		setup  func(t *testing.T)
		change func(t *testing.T, r *gittest.Repo) // before the commit
		after  func(t *testing.T, r *gittest.Repo) // after it
		code   int
		want   []string
		lacks  []string
	}{
		{
			name:  "a denylist with an entry, every step passing",
			steps: []check.Step{citest.Passing},
			code:  check.ExitOK,
			want:  []string{"ok    passes", "ok    hygiene"},
		},
		{
			name:  "a missing denylist",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
			},
			code: check.ExitFail,
			want: []string{"ok    passes", "FAIL  hygiene", "denylist: the denylist is missing"},
		},
		{
			name:  "an empty denylist",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(names.Path, string(citest.Denylist(t)))
			},
			code: check.ExitFail,
			want: []string{"FAIL  hygiene", "denylist: the denylist has no entry"},
		},
		{
			name:  "a failing step does not hide the ones after it",
			steps: []check.Step{citest.Failing, citest.Passing},
			code:  check.ExitFail,
			want:  []string{"FAIL  fails", "ok    passes", "ok    hygiene"},
		},
		{
			name:  "a step runs in its own environment, not in this process's",
			steps: []check.Step{probe},
			setup: func(t *testing.T) {
				t.Setenv("GOFLAGS", "-run=^$")
			},
			code:  check.ExitFail,
			want:  []string{"FAIL  probe", "PROBE=from-the-step", "ok    hygiene"},
			lacks: []string{"GOFLAGS"},
		},
		{
			name:  "by hand, a working tree that differs from HEAD runs every check",
			steps: []check.Step{citest.Passing},
			after: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/ci/x_test.go", "package main\n")
				r.Write("README.md", "# A fixture\n\nChanged.\n")
				r.Write("docs/staged.md", "plain\n")
				r.Git("add", "docs/staged.md")
				r.Write("docs/intent.md", "plain\n")
				r.Git("add", "--intent-to-add", "docs/intent.md")
			},
			code: check.ExitOK,
			want: []string{"ok    passes", "ok    hygiene"},
		},
		{
			name:  "a tracked go.mod below the module root",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/inner/go.mod", "module example.invalid/inner\n")
			},
			code:  check.ExitError,
			want:  []string{"ci: the checks read one module", "\ntools/inner/go.mod"},
			lacks: []string{"ok  ", "FAIL"},
		},
		{
			name:  "an ignored go.mod below the module root",
			steps: []check.Step{citest.Passing},
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(".gitignore", "inner/\n")
			},
			after: func(t *testing.T, r *gittest.Repo) {
				r.Write("inner/go.mod", "module example.invalid/inner\n")
			},
			code:  check.ExitError,
			want:  []string{"ci: the checks read one module", "\ninner/go.mod"},
			lacks: []string{"ok  ", "FAIL"},
		},
		{
			name: "no mise on the search path: fast stops before any check",
			setup: func(t *testing.T) {
				gitPath, err := procgroup.LookPath("git")
				require.NoError(t, err)
				dir := t.TempDir()
				require.NoError(t, os.Symlink(gitPath, filepath.Join(dir, "git")))
				t.Setenv("PATH", dir)
			},
			code:  check.ExitError,
			want:  []string{"ci: mise is not on the search path"},
			lacks: []string{"ok  ", "FAIL", "hygiene"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := citest.NewTree(t)
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
		{name: "a clean tree, HEAD pushed", code: check.ExitOK, want: []string{"ok    passes", "ok    hygiene", "ok    pushed range"}},
		{name: "a staged file", code: check.ExitError, want: []string{refused, "\ndocs/staged.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/staged.md", "plain\n")
				r.Git("add", "docs/staged.md")
				return nil
			}},
		{name: "an intent-to-add entry", code: check.ExitError, want: []string{refused, "\ndocs/intent.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/intent.md", "plain\n")
				r.Git("add", "--intent-to-add", "docs/intent.md")
				return nil
			}},
		{name: "a modified tracked file", code: check.ExitError, want: []string{refused, "\nREADME.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("README.md", "# A fixture\n\nChanged.\n")
				return nil
			}},
		{name: "an alternate GIT_INDEX_FILE that matches HEAD does not hide the index", code: check.ExitError,
			want: []string{refused, "\ndocs/staged.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/staged.md", "plain\n")
				r.Git("add", "docs/staged.md")
				alt := "GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")
				r.WithEnv(alt).Git("read-tree", "HEAD")
				return []string{alt}
			}},
		{name: "an untracked test with a TestMain that exits 0", code: check.ExitError,
			want: []string{refused, "\ntools/ci/x_test.go"}, lacks: []string{"os.Exit"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("tools/ci/x_test.go", "package main\n\nimport (\"os\"; \"testing\")\n\nfunc TestMain(*testing.M) { os.Exit(0) }\n")
				return nil
			}},
		{name: "an ignored Go file, an untracked vendor directory and mise configuration", code: check.ExitError,
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
		{name: "an untracked file fast does not read", code: check.ExitOK, want: []string{"ok    passes", "ok    hygiene"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("docs/notes.txt", "plain\n")
				return nil
			}},
		{name: "an untracked nested repository", code: check.ExitError, want: []string{refused, "\ntools/ci/evil/"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("tools/ci/evil/evil_test.go", "package evil\n")
				r.Git("-C", "tools/ci/evil", "init", "--quiet")
				return nil
			}},
		{name: "a pushed tip that is not HEAD", code: check.ExitError,
			stdin: func(head, other string) string {
				return "refs/heads/other " + other + " refs/heads/other " + zero + "\n"
			},
			want: []string{"ci: fast tests the commit checked out", "check out the ref you push", "\n{other}"}},
		{name: "two pushed tips, one of them not HEAD", code: check.ExitError,
			stdin: func(head, other string) string {
				return "refs/heads/main " + head + " refs/heads/main " + zero + "\nrefs/heads/other " + other + " refs/heads/other " + zero + "\n"
			},
			want: []string{"ci: fast tests the commit checked out"}},
		{name: "an annotated tag on HEAD", code: check.ExitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string { return "refs/tags/v1 {tag} refs/tags/v1 " + zero + "\n" }},
		{name: "a tag of the tag on HEAD", code: check.ExitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string { return "refs/tags/v2 {tagtag} refs/tags/v2 " + zero + "\n" }},
		{name: "a lightweight tag on HEAD", code: check.ExitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string {
				return "refs/tags/v-light " + head + " refs/tags/v-light " + zero + "\n"
			}},
		{name: "an annotated tag on a commit that is not HEAD", code: check.ExitError,
			stdin: func(head, other string) string {
				return "refs/tags/v-other {othertag} refs/tags/v-other " + zero + "\n"
			},
			want: []string{"ci: fast tests the commit checked out", "\n{othertag}"}},
		{name: "a tag of a tree", code: check.ExitError, want: []string{"ci: pushed tip {treetag}: "},
			stdin: func(head, other string) string { return "refs/tags/v-tree {treetag} refs/tags/v-tree " + zero + "\n" }},
		{name: "a tag of a blob", code: check.ExitError, want: []string{"ci: pushed tip {blobtag}: "},
			stdin: func(head, other string) string { return "refs/tags/v-blob {blobtag} refs/tags/v-blob " + zero + "\n" }},
		{name: "a staged deletion", code: check.ExitError, want: []string{refused, "\nREADME.md"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Git("rm", "--quiet", "README.md")
				return nil
			}},
		{name: "an untracked go.mod below the module root", code: check.ExitError, want: []string{"ci: the checks read one module", "\ninner/go.mod"},
			dirty: func(t *testing.T, r *gittest.Repo) []string {
				r.Write("inner/go.mod", "module example.invalid/inner\n")
				return nil
			}},
		{name: "a deleted ref names no tip", code: check.ExitOK, want: []string{"ok    pushed range"},
			stdin: func(head, other string) string { return "(delete) " + zero + " refs/heads/other " + other + "\n" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := gittest.NewBare(t)
			r := citest.NewTree(t)
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
			code, out := runCI(t, fixture, strings.NewReader(stdin), []check.Step{citest.Passing}, "fast", "origin", origin.Dir)
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, ids.Replace(strings.ReplaceAll(w, "{other}", other)))
			}
			for _, w := range tt.lacks {
				assert.NotContains(t, out, w)
			}
			if tt.code == check.ExitError {
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
// other than the commit judged. The case "HEAD moves with nothing
// pushed" pushes nothing, so no tip ties HEAD down and only the
// comparison of the state before and after the steps (changedSince)
// sees that HEAD moved; the last two cases are seen by it alone too.
func TestFastJudgesAgainAfterTheSteps(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	sh, err := procgroup.LookPath("sh")
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
		// judgeCommit refuses neither of the two below (an untracked
		// file that is no gate input, a change git status hides);
		// changedSince does.
		{"an untracked file that is no gate input appears", "echo late > notes.txt", true, "\nnotes.txt"},
		{"a tracked file changes behind skip-worktree", "git update-index --skip-worktree README.md && echo changed >> README.md", true, "\nREADME.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origin := gittest.NewBare(t)
			r := citest.NewTree(t)
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
			mutate := check.Step{Name: "mutates", Argv: []string{sh, "-c", tt.script}, Environ: r.Env}
			code, out := runCI(t, r, strings.NewReader(stdin), []check.Step{mutate}, "fast", "origin", origin.Dir)
			assert.Equal(t, check.ExitError, code, "exit status")
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
	r := citest.NewTree(t)
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
	prologueNamed := branch("prologuenamed", func() string {
		r.Write("docs/zorvex-quimby/PROLOGUE.md", "plain\n")
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
		{name: "a line in a path that is withheld", stdin: push(pathLine, "refs/heads/pathline", zero), code: check.ExitFail,
			want: []string{"commit " + pathLine + ": added path (withheld): name:", "commit " + pathLine + ": (path withheld):1: name:"}},
		{name: "the remote's name selects its tracking refs: a commit origin has is not read",
			stdin: push(child, "refs/heads/child", zero), code: check.ExitOK},
		{name: "the URL is not the remote's name: no default branch is known, and the push stops",
			stdin: push(child, "refs/heads/child", zero), code: check.ExitError, remote: origin.Dir,
			want: []string{"ci: the push names no configured remote"}},
		{"a clean push", push(clean, "refs/heads/clean", zero), check.ExitOK, []string{"ok    pushed range"}, ""},
		{"nothing to push", "", check.ExitOK, []string{"ok    pushed range"}, ""},
		{"a remote ref name", push(clean, "refs/heads/zorvex-quimby", zero), check.ExitFail,
			[]string{"FAIL  pushed range", "pushed ref 1: name: the remote ref name holds a name listed at HEAD"}, ""},
		{"a ref name with a slash between the halves", push(clean, "refs/heads/zorvex/quimby", zero), check.ExitOK, nil, ""},
		{"a commit message", push(message, "refs/heads/message", zero), check.ExitFail,
			[]string{"commit " + message + ": message line 3: name:"}, ""},
		{"an author", push(author, "refs/heads/author", zero), check.ExitFail,
			[]string{"commit " + author + ": author: name:"}, ""},
		{"a committer", push(committer, "refs/heads/committer", zero), check.ExitFail,
			[]string{"commit " + committer + ": committer: name:"}, ""},
		{"an added path", push(path, "refs/heads/path", zero), check.ExitFail,
			[]string{"commit " + path + ": added path (withheld): name:"}, ""},
		{"an added PROLOGUE.md in another case", push(prologue, "refs/heads/prologue", zero), check.ExitFail,
			[]string{"FAIL  pushed range", "commit " + prologue + ": added path docs/Prologue.md: prologue:"}, ""},
		{"an added PROLOGUE.md under a listed name keeps the path withheld", push(prologueNamed, "refs/heads/prologuenamed", zero), check.ExitFail,
			[]string{"commit " + prologueNamed + ": added path (withheld): prologue:"}, ""},
		{"an added line", push(line, "refs/heads/line", zero), check.ExitFail,
			[]string{"commit " + line + ": docs/page.md:2: name:"}, ""},
		{"a line one commit adds and the next removes", push(removedTip, "refs/heads/removed", zero), check.ExitFail,
			[]string{"commit " + removed + ": docs/page.md:2: name:"}, ""},
		{"a deleted ref with a listed name", fmt.Sprintf("(delete) %s refs/heads/zorvex-quimby %s\n", zero, base), check.ExitOK, nil, ""},
		{"input that is not a hook's", "one two\n", check.ExitError, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote := tt.remote
			if remote == "" {
				remote = "origin"
			}
			checkoutTip(t, r, tt.stdin, "")
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []check.Step{citest.Passing}, "fast", remote, origin.Dir)
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
	r := citest.NewTree(t)
	r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp, other)))
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
	drop := func() { r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp))) }
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
			stdin: push(line, "refs/heads/line"), code: check.ExitFail,
			want: []string{"ok    hygiene", "FAIL  pushed range",
				"commit " + line + ": docs/b.txt:1: name: the added line holds a name listed at the remote's default branch"}},
		{name: "a message",
			stdin: push(message, "refs/heads/message"), code: check.ExitFail,
			want: []string{"commit " + message + ": message line 3: name: the line holds a name listed at the remote's default branch"}},
		{name: "an identity",
			stdin: push(identity, "refs/heads/identity"), code: check.ExitFail,
			want: []string{"commit " + identity + ": author: name: the name and email hold a name listed at the remote's default branch"}},
		{name: "a clean commit that drops the entry",
			stdin: push(clean, "refs/heads/clean"), code: check.ExitOK,
			want: []string{"ok    pushed range"}},
		{name: "a default branch without a denylist adds nothing", defaultBranch: unlisted,
			stdin: push(line, "refs/heads/line"), code: check.ExitOK,
			want: []string{"ok    hygiene", "ok    pushed range"}},
		{name: "no entry at HEAD or at the default branch: the range is not checked", defaultBranch: unlisted,
			stdin: push(none, "refs/heads/none"), code: check.ExitFail,
			want: []string{"FAIL  hygiene", "--    pushed range: not checked"}},
		{name: "an unknown default branch stops the push", defaultBranch: deleted,
			stdin: push(line, "refs/heads/line"), code: check.ExitError,
			want: []string{"ci: the default branch of origin is not known here", `run "git fetch origin"`}},
		{name: "a denylist at the default branch that cannot be read stops the push", defaultBranch: broken,
			stdin: push(clean, "refs/heads/clean"), code: check.ExitError,
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
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []check.Step{citest.Passing}, "fast", "origin", origin.Dir)
			assert.Equal(t, tt.code, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			if tt.code == check.ExitError {
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
	r := citest.NewTree(t)
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
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp, other)))
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
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp, other)))
		r.Write("docs/c.txt", "x blorptang-wixelfrum y\n")
		r.Commit("docs: add a page and an entry")
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp)))
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
			stdin: push(listing, "refs/heads/listing"), code: check.ExitError,
			want: []string{"ci: fast tests the commit checked out", "\n" + listing}, lacks: []string{"hygiene", "pushed range"}},
		{name: "an entry the pushed tip adds",
			stdin: push(listing, "refs/heads/listing"), code: check.ExitFail,
			want: []string{"FAIL  pushed range",
				"commit " + listing + ": docs/b.txt:1: name: the added line holds a name listed at HEAD"}},
		{name: "the same tip pushed as an annotated tag",
			stdin: push(listingTag, "refs/tags/v-listing"), code: check.ExitFail,
			want: []string{"commit " + listing + ": docs/b.txt:1: name: the added line holds a name listed at HEAD"}},
		{name: "a clean ref and the checked-out one: every tip must be HEAD", head: listing,
			stdin: push(clean, "refs/heads/clean") + push(listing, "refs/heads/listing"), code: check.ExitError,
			want: []string{"ci: fast tests the commit checked out", "\n" + clean}, lacks: []string{"\n" + listing, "pushed range"}},
		{name: "a denylist that cannot be read at HEAD fails hygiene and the pushed range",
			stdin: push(broken, "refs/heads/broken"), code: check.ExitFail,
			want: []string{"FAIL  hygiene\n" + names.Path + ": ", "FAIL  pushed range\n" + names.Path + ": "}},
		{name: "a deleted ref names no tip",
			stdin: fmt.Sprintf("(delete) %s refs/heads/listing %s\n", zero, listing), code: check.ExitOK,
			want: []string{"ok    pushed range"}},
		{name: "a commit that drops the denylist is judged by the one at the default branch",
			stdin: push(unlistedLine, "refs/heads/unlistedline"), code: check.ExitFail,
			want: []string{"FAIL  hygiene", "FAIL  pushed range",
				"commit " + unlistedLine + ": docs/b.txt:1: name: the added line holds a name listed at the remote's default branch"}},
		{name: "a tag message",
			stdin: push(tagMessage, "refs/tags/v-message"), code: check.ExitFail,
			want: []string{"tag " + tagMessage + ": message line 3: name: the line holds a name listed at HEAD"}},
		{name: "a tagger",
			stdin: push(tagTagger, "refs/tags/v-tagger"), code: check.ExitFail,
			want: []string{"tag " + tagTagger + ": tagger: name: the name and email hold a name listed at HEAD"}},
		{name: "the name a tag was created with, pushed under another",
			stdin: push(tagName, "refs/tags/v-other"), code: check.ExitFail,
			want:  []string{"tag " + tagName + ": tag name: name: the tag's name holds a name listed at HEAD"},
			lacks: []string{"pushed ref 1"}},
		{name: "a signature line of a tag",
			stdin: push(tagSigned, "refs/tags/v-signed"), code: check.ExitFail,
			want: []string{"tag " + tagSigned + ": message line 3: name: the line holds a name listed at HEAD"}},
		{name: "an entry listed inside the range and dropped at its tip is not applied",
			stdin: push(inRange, "refs/heads/inrange"), code: check.ExitOK,
			want: []string{"ok    pushed range"}},
		{name: "a signed tag a merge commit embeds",
			stdin: push(merged, "refs/heads/merged"), code: check.ExitFail,
			want: []string{"commit " + merged + ": mergetag 1 message line 3: name: the line holds a name listed at HEAD"}},
		{name: "a path git quotes is withheld",
			stdin: push(quoted, "refs/heads/quoted"), code: check.ExitFail,
			want: []string{"commit " + quoted + ": added path (withheld): name:", "commit " + quoted + ": (path withheld):1: name:"}},
		{name: "a path with an escape sequence is quoted",
			stdin: push(escaped, "refs/heads/escaped"), code: check.ExitFail,
			want: []string{"commit " + escaped + ": \"docs/a\\x1b[2Jb.md\":1: name:"}, lacks: []string{"\x1b"}},
		{name: "a path with a newline is quoted and forges no line",
			stdin: push(newline, "refs/heads/newline"), code: check.ExitFail,
			want:  []string{"commit " + newline + ": \"docs/a\\nok    pushed range\\n.md\":1: name:", "FAIL  pushed range"},
			lacks: []string{"\nok    pushed range"}},
		{name: "a replace ref does not hide a commit",
			stdin: push(replaced, "refs/heads/replaced"), code: check.ExitFail,
			want: []string{"commit " + replaced + ": docs/r.md:1: name:"}},
		{name: "a clean tag",
			stdin: push(tagClean, "refs/tags/v-clean"), code: check.ExitOK,
			want: []string{"ok    pushed range"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkoutTip(t, r, tt.stdin, tt.head)
			code, out := runCI(t, r, strings.NewReader(tt.stdin), []check.Step{citest.Passing}, "fast", "origin", origin.Dir)
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
	tools := check.Tools{Linter: "/pinned/golangci-lint", GoDir: goDir}
	// Spelled out, not read from lintArgs: this test pins the list.
	wantLint := []string{"/pinned/golangci-lint", "run", "--config", ".golangci.yml", "-j", "1", "--allow-parallel-runners"}
	want := []check.Step{
		{Name: "format", Argv: []string{filepath.Join(goDir, "gofmt"), "-l", "."}, Quiet: true, Environ: check.StepEnv(goDir)},
		{Name: "vet", Argv: []string{goCmd, "vet", "./..."}, Environ: check.StepEnv(goDir)},
		{Name: "generated", Argv: []string{goCmd, "run", "./tools/ci", "generated"}, Environ: check.StepEnv(goDir)},
		{Name: "lint linux/amd64", Argv: wantLint, Environ: lint.Env(lint.Target{GOOS: "linux", GOARCH: "amd64"}, goDir)},
		{Name: "lint linux/arm64", Argv: wantLint, Environ: lint.Env(lint.Target{GOOS: "linux", GOARCH: "arm64"}, goDir)},
		{Name: "lint darwin/amd64", Argv: wantLint, Environ: lint.Env(lint.Target{GOOS: "darwin", GOARCH: "amd64"}, goDir)},
		{Name: "lint darwin/arm64", Argv: wantLint, Environ: lint.Env(lint.Target{GOOS: "darwin", GOARCH: "arm64"}, goDir)},
		{Name: "unit", Argv: []string{goCmd, "test", "-count=1", "./tools/..."}, Environ: check.StepEnv(goDir)},
	}
	assert.Equal(t, want, fastSteps(root, tools))
	for _, s := range fastSteps(root, tools) {
		assert.Contains(t, s.Environ, "GOPROXY=off", "%s never reaches the network", s.Name)
	}

	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o755))
	var unit []string
	for _, s := range fastSteps(root, tools) {
		if s.Name == "unit" {
			unit = s.Argv
		}
	}
	assert.Equal(t, []string{goCmd, "test", "-count=1", "./internal/...", "./tools/..."}, unit, "unit")
}

// TestGeneratedRunsBeforeTheTests holds the order the threat model of
// tools/ci/misefiles.go relies on: in fast and in all, the generated
// step, which judges the index, runs before every step that runs code
// of the change (its tests), so that code cannot stage a good copy of
// a generated file before it is judged.
func TestGeneratedRunsBeforeTheTests(t *testing.T) {
	tools := check.Tools{Linter: "/pinned/golangci-lint", GoDir: "/pinned/go/bin"}
	for name, steps := range map[string][]check.Step{
		"fast": fastSteps(citest.ModuleRoot(t), tools),
		"all":  allSteps(t.Context(), citest.ModuleRoot(t), tools, filepath.Join(t.TempDir(), coverFile)),
	} {
		names := make([]string, len(steps))
		for i, s := range steps {
			names[i] = s.Name
		}
		generated := slices.Index(names, "generated")
		require.GreaterOrEqual(t, generated, 0, "%s: %v", name, names)
		for _, tests := range []string{"unit", "race"} {
			if i := slices.Index(names, tests); i >= 0 {
				assert.Less(t, generated, i, "%s: generated runs before %s: %v", name, tests, names)
			}
		}
	}
}

// TestIsGateInput pins the untracked files a pre-push run refuses, a
// nested repository among them, which git lists as its directory.
func TestIsGateInput(t *testing.T) {
	for _, path := range []string{"x.go", "tools/ci/x_test.go", "go.work", "go.work.sum", "vendor/modules.txt",
		"vendor/example.com/m/m.go", "mise.toml", ".mise.toml", "mise.local.toml", ".mise.local.toml", "mise.ci.toml",
		".config/mise.toml", ".config/mise/config.toml", ".config/mise/conf.d/x.toml", ".mise/config.toml",
		"mise/config.toml", ".tool-versions", "tools/.tool-versions", "tools/ci/evil/", "nested/",
		"mise.lock", "Mise.Local.toml", ".TOOL-VERSIONS", ".miserc.toml", ".miserc.local.toml", ".config/miserc.toml",
		"tools/ci/.miserc.toml", "tools/ci/.config/mise/conf.d/x.toml"} {
		assert.True(t, isGateInput(path), path)
	}
	for _, path := range []string{"README.md", "go.mod", "go.sum", "docs/promise.toml", "REUSE.toml",
		"tools/vendor/notes.md", "docs/go.md", "x.gox"} {
		assert.False(t, isGateInput(path), path)
	}
}

// TestFastHookIgnoresCallerGitEnv shows that hygiene and the pushed
// range read this repository when the caller sets GIT_DIR and
// GIT_ALTERNATE_OBJECT_DIRECTORIES to another one: a pre-push run
// clears the variables of git.LocalEnv for every git command it runs.
func TestFastHookIgnoresCallerGitEnv(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	r := citest.NewTree(t)
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
	code, out := runCI(t, hooked, strings.NewReader(stdin), []check.Step{citest.Passing}, "fast", "origin", "https://example.invalid/x.git")
	assert.Equal(t, check.ExitOK, code, "exit status\n%s", out)
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
	r := citest.NewTree(t)
	r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp, other)))
	r.Git("remote", "add", "origin", origin.Dir)
	base := r.Commit("fixture")
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin")
	r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp)))
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
		e := check.Env{Dir: r.Dir, Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &out, Steps: []check.Step{citest.Passing}}
		return run(t.Context(), e, []string{"fast", "origin", origin.Dir}), out.String()
	}

	r.Write("inner/go.mod", "module example.invalid/inner\n")
	code, out := run()
	assert.Equal(t, check.ExitError, code, "exit status\n%s", out)
	assert.Contains(t, out, "ci: the checks read one module")
	assert.Contains(t, out, "\ninner/go.mod")

	require.NoError(t, os.Remove(filepath.Join(r.Dir, "inner", "go.mod")))
	code, out = run()
	assert.Equal(t, check.ExitFail, code, "exit status\n%s", out)
	assert.Contains(t, out, "FAIL  pushed range")
	assert.Contains(t, out, "commit "+tip+": docs/b.txt:1: name: the added line holds a name listed at the remote's default branch")
	assert.NotContains(t, strings.ToLower(out), "wixelfrum", "the output repeats the matched text")
}

// TestDataAtHEAD shows that the denylist and the word lists are read
// from the tree at HEAD, by every check: a denylist that is written
// and not committed protects no push, so it must not turn a check
// green. Each subtest builds its own tree, so each runs alone.
func TestDataAtHEAD(t *testing.T) {
	const (
		zero = "0000000000000000000000000000000000000000"
		url  = "https://example.invalid/x.git"
	)
	// tree returns a fixture whose HEAD holds no denylist, and pushed to
	// origin, whose default branch then holds none either.
	tree := func(t *testing.T) *gittest.Repo {
		t.Helper()
		r := citest.NewTree(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
		r.Commit("no denylist")
		r.Git("remote", "add", "origin", gittest.NewBare(t).Dir)
		r.Git("push", "--quiet", "origin", "main")
		r.Git("fetch", "--quiet", "origin")
		return r
	}
	pushOf := func(r *gittest.Repo) string {
		return "refs/heads/main " + r.Git("rev-parse", "HEAD") + " refs/heads/main " + zero + "\n"
	}
	// runAll runs every check and wants the status want, except fast as
	// the hook when hook is not 0: a working tree that differs from HEAD
	// stops it before any check (TestFastJudgesTheCommit).
	runAll := func(t *testing.T, r *gittest.Repo, want, hook int) {
		t.Helper()
		clean := filepath.Join(t.TempDir(), "clean.txt")
		require.NoError(t, os.WriteFile(clean, []byte("plain\n"), 0o644))
		checks := []struct {
			name  string
			stdin string
			args  []string
		}{
			{"hygiene", "", []string{"hygiene"}},
			{"hygiene --file", "", []string{"hygiene", "--file", clean}},
			{"fast", "", []string{"fast"}},
			{"fast as the hook", pushOf(r), []string{"fast", "origin", url}},
		}
		for _, c := range checks {
			code, out := runCI(t, r, strings.NewReader(c.stdin), []check.Step{citest.Passing}, c.args...)
			if c.stdin != "" && hook != 0 {
				assert.Equal(t, hook, code, "%s: exit status\n%s", c.name, out)
				assert.Contains(t, out, "ci: fast judges the commit a push sends", c.name)
				continue
			}
			assert.Equal(t, want, code, "%s: exit status\n%s", c.name, out)
			if want == check.ExitFail {
				assert.Contains(t, out, "denylist: the denylist is missing", c.name)
			}
		}
	}

	t.Run("a denylist that is not committed fails every check", func(t *testing.T) {
		r := tree(t)
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp)))
		r.Git("add", "--all") // staged is not committed either
		runAll(t, r, check.ExitFail, check.ExitError)
	})
	t.Run("once committed, every check passes", func(t *testing.T) {
		r := tree(t)
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp)))
		r.Commit("the denylist")
		runAll(t, r, check.ExitOK, 0)
	})
	t.Run("later changes that are not committed are not read", func(t *testing.T) {
		r := tree(t)
		r.Write(names.Path, string(citest.Denylist(t, citest.MadeUp)))
		r.Commit("the denylist")
		r.Write(names.Path, "entries: [\n")
		r.Write("tools/ci/prose.yaml", "rules: []\n")
		r.Git("add", "--all")
		runAll(t, r, check.ExitOK, check.ExitError)
	})
}

// TestPrologueInThePushedRange shows that a pushed commit which adds
// the file fails the pre-push run even when a later commit removes it,
// and that it does so with no denylist entry to match names with.
func TestPrologueInThePushedRange(t *testing.T) {
	const zero = "0000000000000000000000000000000000000000"
	r := citest.NewTree(t)
	require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
	r.Commit("base, no denylist")
	origin := gittest.NewBare(t)
	r.Git("remote", "add", "origin", origin.Dir)
	r.Git("push", "--quiet", "origin", "main")
	r.Git("fetch", "--quiet", "origin")
	r.Write("sub/prologue.md", "text\n")
	added := r.Commit("adds it")
	require.NoError(t, os.Remove(filepath.Join(r.Dir, "sub", "prologue.md")))
	tip := r.Commit("removes it again")
	stdin := "refs/heads/main " + tip + " refs/heads/main " + zero + "\n"
	push, err := pushed.Parse(strings.NewReader(stdin))
	require.NoError(t, err)
	got, checked, err := citest.RangeOf(pushedrange.Range(t.Context(), r.Repo, "origin", push, tip))
	require.NoError(t, err)
	assert.False(t, checked, "no denylist entry: names are not checked")
	assert.Equal(t, "commit "+added+": added path sub/prologue.md: prologue: the user's agreement file lives outside product repositories\n", check.Lines(got))

	// The run reports the finding too, and not only "not checked".
	code, out := runCI(t, r, strings.NewReader(stdin), []check.Step{citest.Passing}, "fast", "origin", origin.Dir)
	assert.Equal(t, check.ExitFail, code, "exit status")
	assert.Contains(t, out, "FAIL  pushed range\ncommit "+added+": added path sub/prologue.md: prologue:")
	assert.Contains(t, out, "pushed range: not checked against names")
}
