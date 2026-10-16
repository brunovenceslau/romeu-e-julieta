// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/prose"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/pushed"
)

// The strings below are put together at run time, so this file does
// not hold what the check it tests would report in it.
const (
	todoWord = "TO" + "DO"
	usersDir = "/Users" + "/"
	homeDir  = "/home" + "/"
)

func TestHygienePasses(t *testing.T) {
	r := citest.NewTree(t)
	r.Write("docs/page.md", strings.Join([]string{
		"The word `glimmerous` is listed, so a page quotes it in a code span.",
		"A path like " + usersDir + "<name>/ is a placeholder, and " + homeDir + "agent/ is the sandbox user.",
		"A " + todoWord + " in a page is not a Go comment.",
		"zorvex/quimby is a path of another product.",
		"",
	}, "\n"))
	r.Write("tools/x/x.go", strings.Join([]string{
		"package x",
		"",
		"import \"context\"",
		"",
		"// " + todoWord + "(#12): tracked work.",
		"var _ = context." + todoWord + "()",
		"",
		"const s = \"" + todoWord + " in a string is not a comment\"",
		"",
	}, "\n"))
	r.Write("pkg/vendor/kept.txt", "a vendor directory below the root is not the module's\n")
	r.Write("docs/PROLOGUE.md.txt", "only the exact name is the file\n")
	r.Write("docs/NOT-PROLOGUE.md", "a longer name is another file\n")
	r.Write("docs/PROLOGUE.md/page.txt", "a directory of that name holds files, not the file\n")
	r.Commit("fixture")
	code, out := runCI(t, r, nil, nil, "hygiene")
	assert.Equal(t, check.ExitOK, code, "exit status\n%s", out)
}

// TestHygieneRules has one fixture for each rule of 10 10.2: each case
// changes a tree that passes, and names the finding it expects.
func TestHygieneRules(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, r *gittest.Repo)
		want   []string // each must be in the output
	}{
		{
			name: "U+2014 in a file",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/page.md", "one\ntwo "+emDash+" three\n")
			},
			want: []string{"docs/page.md:2: em-dash:"},
		},
		{
			name: "U+2014 in a code span and in a fenced block: the check skips nothing",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/page.md", "a `"+emDash+"` span\n\n```\n"+emDash+"\n```\n")
			},
			want: []string{"docs/page.md:1: em-dash:", "docs/page.md:4: em-dash:"},
		},
		{
			name: "U+2014 in a path",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/a"+emDash+"b.md", "text\n")
			},
			// The byte rule of the path reports it too.
			want: []string{"em-dash: the path holds U+2014", "path: the path holds a byte outside printable ASCII"},
		},
		{
			name: "a listed prose word",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/page.md", "one\n\nA Glimmerous tool.\n")
			},
			want: []string{"docs/page.md:3: prose: shiny"},
		},
		{
			name: "a to-do without an issue",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/x/x.go", "package x\n\n// "+todoWord+": later.\nvar X = 1\n")
			},
			want: []string{"tools/x/x.go:3: todo:"},
		},
		{
			name: "a to-do with a name instead of an issue, in a block comment",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/x/x.go", "package x\n\n/*\n   one\n   "+todoWord+"(someone) later\n*/\nvar X = 1\n")
			},
			want: []string{"tools/x/x.go:5: todo:"},
		},
		{
			name: "a personal absolute path, macOS form",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/page.md", "run it in "+usersDir+"someone/src\n")
			},
			want: []string{"docs/page.md:1: personal-path:"},
		},
		{
			name: "a personal absolute path, Linux form",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/x/x.go", "package x\n\nconst p = \""+homeDir+"someone/x\"\n")
			},
			want: []string{"tools/x/x.go:3: personal-path:"},
		},
		{
			name: "a tracked go.work",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("go.work", "go 1.27\n")
				r.Write("go.work.sum", "\n")
			},
			want: []string{"go.work: workspace:", "go.work.sum: workspace:"},
		},
		{
			name: "a tracked vendor directory",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("vendor/modules.txt", "\n")
			},
			want: []string{"vendor/modules.txt: workspace:"},
		},
		{
			name: "a tracked PROLOGUE.md at the root",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("PROLOGUE.md", "text\n")
			},
			want: []string{"PROLOGUE.md: prologue: the user's agreement file lives outside product repositories"},
		},
		{
			name: "a tracked PROLOGUE.md in a subdirectory",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/notes/PROLOGUE.md", "text\n")
			},
			want: []string{"docs/notes/PROLOGUE.md: prologue:"},
		},
		{
			name: "a tracked prologue.md in another case",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("Docs/Prologue.MD", "text\n")
			},
			want: []string{"Docs/Prologue.MD: prologue:"},
		},
		{
			name: "a second file in .githooks",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(".githooks/pre-commit", "#!/bin/sh\n")
			},
			want: []string{".githooks/pre-commit: githooks:"},
		},
		{
			name: "a file in .githooks that is no hook",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(".githooks/other", "text\n")
			},
			want: []string{".githooks/other: githooks:"},
		},
		{
			name: "a file in .githooks whose path holds a listed name",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(".githooks/zorvex-quimby", "text\n")
			},
			want: []string{
				"(path withheld, tree entry 2): name: the path holds a listed name",
				"(path withheld, tree entry 2): githooks:",
			},
		},
		{
			name: "a to-do with an empty issue number",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("tools/x/x.go", "package x\n\n// "+todoWord+"(#): later.\nvar X = 1\n")
			},
			want: []string{"tools/x/x.go:3: todo:"},
		},
		{
			name: "a hook that is not executable",
			change: func(t *testing.T, r *gittest.Repo) {
				require.NoError(t, os.Chmod(filepath.Join(r.Dir, ".githooks", "pre-push"), 0o644))
			},
			want: []string{".githooks/pre-push: githooks: the mode is 100644, want 100755"},
		},
		{
			name: "no hook",
			change: func(t *testing.T, r *gittest.Repo) {
				require.NoError(t, os.Remove(filepath.Join(r.Dir, ".githooks", "pre-push")))
			},
			want: []string{".githooks/pre-push: githooks: the hook is not tracked"},
		},
		{
			name: "a mise configuration other than mise.toml, in any case",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("mise.local.toml", "[settings]\n")
				r.Write("Mise.Ci.toml", "[settings]\n")
				r.Write(".TOOL-VERSIONS", "go 1.27.0\n")
				r.Write(".config/mise/conf.d/x.toml", "[settings]\n")
				r.Write(".miserc.toml", "env = []\n")
				// The pinned files are exempt by their exact name only.
				r.Write("MISE.TOML", "[settings]\n")
				r.Write("Mise.lock", "\n")
			},
			want: []string{
				".TOOL-VERSIONS: mise:", ".config/mise/conf.d/x.toml: mise:", ".miserc.toml: mise:",
				"MISE.TOML: mise:", "Mise.Ci.toml: mise:", "Mise.lock: mise:", "mise.local.toml: mise:",
			},
		},
		{
			name: "a path with a byte outside printable ASCII",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("mi\u017fe.local.toml", "[settings]\n")
				r.Write("docs/tab\tname.md", "text\n")
				r.Write("docs/del\x7fname.md", "text\n")
			},
			want: []string{`"docs/del\x7fname.md": path:`, `"docs/tab\tname.md": path:`, "mi\u017fe.local.toml: path:"},
		},
		{
			name: "a symbolic link",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("realconf/mise/conf.d/x.md", "text\n")
				require.NoError(t, os.Symlink("realconf", filepath.Join(r.Dir, ".config")))
			},
			want: []string{".config: mode: the mode is 120000"},
		},
		{
			name: "a submodule",
			change: func(t *testing.T, r *gittest.Repo) {
				// The directory keeps "git add --all" from staging the
				// removal of the entry.
				require.NoError(t, os.Mkdir(filepath.Join(r.Dir, "lib"), 0o755))
				r.Git("update-index", "--add", "--cacheinfo", "160000,0123456789abcdef0123456789abcdef01234567,lib")
			},
			want: []string{"lib: mode: the mode is 160000"},
		},
		{
			name: "a missing denylist",
			change: func(t *testing.T, r *gittest.Repo) {
				require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
			},
			want: []string{names.Path + ": denylist: the denylist is missing"},
		},
		{
			name: "a denylist with no entry",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write(names.Path, string(citest.Denylist(t)))
			},
			want: []string{names.Path + ": denylist: the denylist has no entry"},
		},
		{
			name: "a listed name in a file",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/page.md", "one\nsee Zorvex-Quimby\n")
				r.Write("e2e/testdata/sbx/v1/session.jsonl", "{\"out\":\"zorvexquimby\"}\n")
			},
			want: []string{"docs/page.md:2: name:", "e2e/testdata/sbx/v1/session.jsonl:1: name:"},
		},
		{
			name: "a listed name in a path",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/zorvex_quimby/page.md", "see zorvex.quimby\n")
			},
			want: []string{"(path withheld, tree entry 3): name: the path holds a listed name", "(path withheld, tree entry 3):1: name:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := citest.NewTree(t)
			tt.change(t, r)
			r.Commit("fixture")
			code, out := runCI(t, r, nil, nil, "hygiene")
			assert.Equal(t, check.ExitFail, code, "exit status")
			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			assert.Equal(t, len(tt.want)+1, strings.Count(out, "\n"),
				"lines of output, want %d findings and a summary", len(tt.want))
			// A hit is reported by its location; what matched stays out
			// of a log that is public.
			low := strings.ToLower(out)
			for _, form := range []string{"zorvex", "quimby", "glimmerous", "someone"} {
				assert.NotContains(t, low, form, "the output repeats the matched text")
			}
			if t.Failed() {
				t.Logf("output:\n%s", out)
			}
		})
	}
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
	got, checked, err := rangeOf(pushedRange(t.Context(), r.Repo, "origin", push, tip))
	require.NoError(t, err)
	assert.False(t, checked, "no denylist entry: names are not checked")
	assert.Equal(t, "commit "+added+": added path sub/prologue.md: prologue: the user's agreement file lives outside product repositories\n", check.Lines(got))

	// The run reports the finding too, and not only "not checked".
	code, out := runCI(t, r, strings.NewReader(stdin), []check.Step{citest.Passing}, "fast", "origin", origin.Dir)
	assert.Equal(t, check.ExitFail, code, "exit status")
	assert.Contains(t, out, "FAIL  pushed range\ncommit "+added+": added path sub/prologue.md: prologue:")
	assert.Contains(t, out, "pushed range: not checked against names")
}

// TestPushedRangeVerdicts pins the verdicts of pushedRange, case by
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
					{Name: name, Findings: []check.Finding{{Where: "commit " + tip + ": added path sub/prologue.md", Rule: "prologue", Msg: prologueMsg}}},
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
			got, err := pushedRange(t.Context(), r.Repo, "origin", push, tip)
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

// TestHygienePrintsPathsSafely shows that a path is printed quoted when
// it holds a control character, so it cannot move the terminal or add a
// line of its own to the output.
func TestHygienePrintsPathsSafely(t *testing.T) {
	tests := []struct{ name, path, want string }{
		{"an escape sequence", "docs/a\x1b[2Jb.md", `"docs/a\x1b[2Jb.md":1: em-dash:`},
		{"a newline", "docs/a\nhygiene: ok\n.md", `"docs/a\nhygiene: ok\n.md":1: em-dash:`},
		{"a zero-width space", "docs/a\u200bb.md", `"docs/a\u200bb.md":1: em-dash:`},
		{"a plain path outside ASCII", "docs/caf\u00e9.md", "docs/caf\u00e9.md:1: em-dash:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := citest.NewTree(t)
			r.Write(tt.path, "a "+emDash+" b\n")
			r.Commit("fixture")
			code, out := runCI(t, r, nil, nil, "hygiene")
			assert.Equal(t, check.ExitFail, code, "exit status")
			assert.Contains(t, out, tt.want)
			for _, raw := range []string{"\x1b", "\nhygiene: ok", "\u200b"} {
				assert.NotContains(t, out, raw, "the output holds a control character of the path")
			}
			if t.Failed() {
				t.Logf("output:\n%q", out)
			}
		})
	}
}

// TestHygieneReadsHEAD shows which tree is checked: the one of HEAD.
// What is committed is what a push publishes.
func TestHygieneReadsHEAD(t *testing.T) {
	r := citest.NewTree(t)
	r.Write("docs/page.md", "see zorvex quimby\n")
	r.Commit("with the name")
	r.Write("docs/page.md", "clean again, and not committed\n")
	code, out := runCI(t, r, nil, nil, "hygiene")
	assert.Equal(t, check.ExitFail, code, "the committed name\n%s", out)
	r.Commit("without the name")
	r.Write("docs/page.md", "see zorvex quimby, not committed\n")
	code, out = runCI(t, r, nil, nil, "hygiene")
	assert.Equal(t, check.ExitOK, code, "the name that is not committed\n%s", out)
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

func TestHygieneBrokenData(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
	}{
		{"a denylist that does not parse", names.Path, "entries: [\n"},
		{"word lists whose self-test fails", "tools/ci/prose.yaml", "rules:\n  - id: a\n    phrases: [zappy]\n    fails: [plain]\n    passes: [plain]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := citest.NewTree(t)
			r.Write(tt.path, tt.content)
			r.Commit("fixture")
			code, out := runCI(t, r, nil, nil, "hygiene")
			assert.Equal(t, check.ExitError, code, "exit status\n%s", out)
		})
	}
}

// TestHygieneFile covers the --file flag: the name matcher over one
// file outside the tree.
func TestHygieneFile(t *testing.T) {
	r := citest.NewTree(t)
	r.Commit("fixture")
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}
	with := write("with.txt", "one\nhost zorvex-quimby.example\nthree\n")
	without := write("without.txt", "one\nhost plain.example, with "+emDash+" and glimmerous\n")

	tests := []struct {
		name string
		file string
		code int
		want string
	}{
		{"a file that holds a listed name", with, check.ExitFail, ":2: name:"},
		{"a file that holds none: the other rules do not apply", without, check.ExitOK, ""},
		{"a file that does not exist", filepath.Join(dir, "nonesuch"), check.ExitError, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out := runCI(t, r, nil, nil, "hygiene", "--file", tt.file)
			assert.Equal(t, tt.code, code, "exit status\n%s", out)
			assert.Contains(t, out, tt.want)
			assert.NotContains(t, strings.ToLower(out), "zorvex", "the output repeats the matched text")
		})
	}

	t.Run("without a denylist entry the flag fails", func(t *testing.T) {
		r.Write(names.Path, string(citest.Denylist(t)))
		r.Commit("an empty denylist")
		code, out := runCI(t, r, nil, nil, "hygiene", "--file", without)
		assert.Equal(t, check.ExitFail, code, "exit status\n%s", out)
	})
}

// TestJudgedCommitIsPinned shows that hygiene and pushedRange read the
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
		got, err := hygiene(t.Context(), r.Repo, x)
		require.NoError(t, err)
		assert.Empty(t, got, "X is clean")
		got, err = hygiene(t.Context(), r.Repo, "HEAD")
		require.NoError(t, err)
		assert.Contains(t, check.Lines(got), "docs/page.md:1: name:")
	})
	t.Run("hygiene: a denylist that Y drops", func(t *testing.T) {
		r, x := moved(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(names.Path))))
		r.Commit("Y")
		got, err := hygiene(t.Context(), r.Repo, x)
		require.NoError(t, err)
		assert.Empty(t, got, "X has its denylist")
		got, err = hygiene(t.Context(), r.Repo, "HEAD")
		require.NoError(t, err)
		assert.Contains(t, check.Lines(got), "denylist: the denylist is missing")
	})
	t.Run("hygiene: word lists that Y drops", func(t *testing.T) {
		r, x := moved(t)
		require.NoError(t, os.Remove(filepath.Join(r.Dir, filepath.FromSlash(prose.Path))))
		r.Commit("Y")
		got, err := hygiene(t.Context(), r.Repo, x)
		require.NoError(t, err)
		assert.Empty(t, got)
		_, err = hygiene(t.Context(), r.Repo, "HEAD")
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
		got, checked, err := rangeOf(pushedRange(t.Context(), r.Repo, "origin", push, x))
		require.NoError(t, err)
		assert.True(t, checked, "X holds an entry")
		assert.Contains(t, check.Lines(got), "the added line holds a name listed at HEAD")
		_, checked, err = rangeOf(pushedRange(t.Context(), r.Repo, "origin", push, "HEAD"))
		require.NoError(t, err)
		assert.False(t, checked, "Y has none, and neither has the default branch")
	})
}

// TestPrintableASCII pins the bounds of the path rule: 0x20 and 0x7E
// are in, 0x1F and 0x7F are out. A Windows file system reads a name
// with a trailing dot as the name without it, and the rule passes such
// a name: the accepted gap the threat model of tools/ci/misefiles.go
// names, here to be seen.
func TestPrintableASCII(t *testing.T) {
	for _, tt := range []struct {
		path string
		want bool
	}{
		{"a\x1fb", false},
		{"a b", true},
		{"a~b", true},
		{"a\x7fb", false},
		{"caf\u00e9", false},
		{"mise.local.toml.", true},
	} {
		assert.Equal(t, tt.want, printableASCII(tt.path), "%q", tt.path)
	}
	assert.False(t, mise.IsMiseFile("mise.local.toml."), "nor does isMiseFile match it")
}

// rangeOf reads the verdicts of pushedRange back as its findings, and
// whether names were matched: no verdict carries the "not checked"
// note. A verdict that could not judge is returned as its error.
func rangeOf(verdicts []check.Verdict, err error) (findings []check.Finding, checked bool, _ error) {
	checked = true
	for _, v := range verdicts {
		switch {
		case v.Err != nil:
			return nil, false, v.Err
		case v.Note != "":
			checked = false
		}
		findings = append(findings, v.Findings...)
	}
	return findings, checked, err
}

// TestBlobAt checks how blobAt tells a file that is there from one
// that is not: by the tree of rev, so a file the tree lacks is not
// found, a file it holds is read, and a directory or a submodule at the
// path is an error.
func TestBlobAt(t *testing.T) {
	r := citest.NewTree(t)
	head := r.Commit("fixture")
	data, found, err := blobAt(t.Context(), r.Repo, head, "README.md")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "# A fixture\n\nPlain text.\n", string(data))
	_, found, err = blobAt(t.Context(), r.Repo, head, "absent.md")
	require.NoError(t, err)
	assert.False(t, found)
	_, found, err = blobAt(t.Context(), r.Repo, head, "docs/absent.md")
	require.NoError(t, err)
	assert.False(t, found, "under a directory the tree lacks")
	_, _, err = blobAt(t.Context(), r.Repo, head, "tools")
	require.ErrorContains(t, err, "not a file")
	// A submodule whose commit this repository does not hold, as is usual.
	r.Git("update-index", "--add", "--cacheinfo", "160000,0123456789abcdef0123456789abcdef01234567,sub")
	r.Git("commit", "--quiet", "--message", "a submodule")
	_, _, err = blobAt(t.Context(), r.Repo, "HEAD", "sub")
	require.ErrorContains(t, err, "read sub at HEAD: not a file")
}

// partialClone clones upstream with --filter=blob:none and no checkout,
// so the clone holds every commit and tree and no blob; a later fetch
// brings no blob either. It returns the clone. The fixture's git runs
// with GIT_NO_LAZY_FETCH=1 (git.Repo.Run), so a checkout could not
// fetch the blobs it needs.
func partialClone(t *testing.T, upstream *gittest.Repo) git.Repo {
	t.Helper()
	upstream.Git("config", "uploadpack.allowFilter", "true")
	dir := filepath.Join(t.TempDir(), "clone")
	upstream.Git("clone", "--quiet", "--filter=blob:none", "--no-checkout", "file://"+upstream.Dir, dir)
	return git.Repo{Dir: dir, Env: upstream.Env}
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
	clone := partialClone(t, upstream)
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

// TestHeadTreeOfAPartialClone checks that a tree whose blob the object
// store lacks is an error that names the file and says how to fetch the
// blob.
func TestHeadTreeOfAPartialClone(t *testing.T) {
	upstream := citest.NewTree(t)
	upstream.Commit("fixture")
	clone := partialClone(t, upstream)
	upstream.Write("docs/new.md", "new\n")
	upstream.Commit("docs: add a page")
	_, err := clone.Run(t.Context(), nil, "fetch", "--quiet", "origin")
	require.NoError(t, err)
	_, err = check.HeadTree(t.Context(), clone, "refs/remotes/origin/HEAD")
	// The clone has no blob at all: the first entry of the tree is said.
	first := upstream.Git("rev-parse", "HEAD:.githooks/pre-push")
	require.ErrorContains(t, err, "read .githooks/pre-push: the object store lacks its blob "+first+`; in a partial clone (one made with --filter), fetch it with "git cat-file -p `+first+`"`)
}
