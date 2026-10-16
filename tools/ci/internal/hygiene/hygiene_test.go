// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package hygiene

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
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
	code, out := citest.Run(t, r, nil, Run)
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
				r.Write("docs/page.md", "one\ntwo "+EmDash+" three\n")
			},
			want: []string{"docs/page.md:2: em-dash:"},
		},
		{
			name: "U+2014 in a code span and in a fenced block: the check skips nothing",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/page.md", "a `"+EmDash+"` span\n\n```\n"+EmDash+"\n```\n")
			},
			want: []string{"docs/page.md:1: em-dash:", "docs/page.md:4: em-dash:"},
		},
		{
			name: "U+2014 in a path",
			change: func(t *testing.T, r *gittest.Repo) {
				r.Write("docs/a"+EmDash+"b.md", "text\n")
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
			code, out := citest.Run(t, r, nil, Run)
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
			r.Write(tt.path, "a "+EmDash+" b\n")
			r.Commit("fixture")
			code, out := citest.Run(t, r, nil, Run)
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
	code, out := citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitFail, code, "the committed name\n%s", out)
	r.Commit("without the name")
	r.Write("docs/page.md", "see zorvex quimby, not committed\n")
	code, out = citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitOK, code, "the name that is not committed\n%s", out)
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
			code, out := citest.Run(t, r, nil, Run)
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
	without := write("without.txt", "one\nhost plain.example, with "+EmDash+" and glimmerous\n")

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
			code, out := citest.Run(t, r, nil, Run, "--file", tt.file)
			assert.Equal(t, tt.code, code, "exit status\n%s", out)
			assert.Contains(t, out, tt.want)
			assert.NotContains(t, strings.ToLower(out), "zorvex", "the output repeats the matched text")
		})
	}

	t.Run("without a denylist entry the flag fails", func(t *testing.T) {
		r.Write(names.Path, string(citest.Denylist(t)))
		r.Commit("an empty denylist")
		code, out := citest.Run(t, r, nil, Run, "--file", without)
		assert.Equal(t, check.ExitFail, code, "exit status\n%s", out)
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
