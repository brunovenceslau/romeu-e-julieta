// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// fsOpsEnv holds the operations that TestHelperFsOps runs, one on each
// line: "write <path> <text>", "chmod <path> <octal mode>" or "remove
// <path>", in its working
// directory. A step that runs this test binary again is a generator
// whose effect on the tree the test chooses.
const fsOpsEnv = "CI_TEST_FSOPS"

// TestHelperFsOps is not a test: it is the generator of the tests below,
// when fsOpsEnv is set.
func TestHelperFsOps(t *testing.T) {
	ops := os.Getenv(fsOpsEnv)
	if ops == "" {
		return
	}
	for _, line := range strings.Split(ops, "\n") {
		f := strings.SplitN(line, " ", 3)
		switch f[0] {
		case "write":
			if err := os.WriteFile(f[1], []byte(f[2]), 0o644); err != nil {
				os.Exit(2)
			}
		case "chmod":
			mode, err := strconv.ParseUint(f[2], 8, 32)
			if err != nil || os.Chmod(f[1], os.FileMode(mode)) != nil {
				os.Exit(2)
			}
		case "remove":
			if err := os.Remove(f[1]); err != nil {
				os.Exit(2)
			}
		case "fail":
			_, _ = os.Stdout.WriteString("the generator says no\n")
			os.Exit(3)
		}
	}
	os.Exit(0)
}

// fsOps returns a generator step that runs the operations.
func fsOps(ops ...string) step {
	return step{
		name:    "go generate",
		argv:    []string{os.Args[0], "-test.run=^TestHelperFsOps$"},
		environ: []string{"PATH=" + os.Getenv("PATH"), fsOpsEnv + "=" + strings.Join(ops, "\n")},
	}
}

// generatedFixture is a repository with a committed generated file and
// the other kinds of file a tree holds: an untracked one, an ignored one,
// a symbolic link, and a tracked file that is deleted from the disk, which
// no snapshot holds.
func generatedFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Write(".gitignore", "ignored.txt\n")
	r.Write("gen.txt", "generated")
	r.Write("gone.txt", "gone\n")
	require.NoError(t, os.Symlink("gen.txt", filepath.Join(r.Dir, "link")))
	r.Commit("fixture")
	require.NoError(t, os.Remove(filepath.Join(r.Dir, "gone.txt")))
	r.Write("untracked.txt", "u\n")
	r.Write("ignored.txt", "i\n")
	return r
}

// TestGenerated is the acceptance case of 10 10.2: a generator that
// changes, creates or removes a file fails, one that changes nothing
// passes, and the comparison reads content, so a rewrite with the same
// bytes passes.
func TestGenerated(t *testing.T) {
	tests := []struct {
		name string
		gen  step
		want []string // "<path>: generated: <message start>"
	}{
		{"nothing changes", fsOps(), nil},
		{"a rewrite with the same content passes", fsOps("write gen.txt generated"), nil},
		{"a changed tracked file fails", fsOps("write gen.txt hand edited"), []string{"gen.txt: generated: go generate changed this file"}},
		{"a new file fails", fsOps("write new.txt x"), []string{"new.txt: generated: go generate created this file"}},
		{"a chmod of a tracked file fails", fsOps("chmod gen.txt 755"), []string{"gen.txt: generated: go generate changed this file"}},
		{"a removed file fails", fsOps("remove gen.txt"), []string{"gen.txt: generated: go generate removed this file"}},
		{"a changed untracked file fails", fsOps("write untracked.txt v2"), []string{"untracked.txt: generated: go generate changed this file"}},
		{"a removed untracked file fails", fsOps("remove untracked.txt"), []string{"untracked.txt: generated: go generate removed this file"}},
		{"an ignored file is not compared", fsOps("write ignored.txt changed"), nil},
		{"a deleted tracked file that the generator writes again is new", fsOps("write gone.txt back"), []string{"gone.txt: generated: go generate created this file"}},
		{"findings are sorted by path", fsOps("write z.txt 1", "write gen.txt x", "write a.txt 1"), []string{
			"a.txt: generated: go generate created this file",
			"gen.txt: generated: go generate changed this file",
			"z.txt: generated: go generate created this file",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := generatedFixture(t)
			findings, err := generated(context.Background(), r.Repo, tt.gen, nil)
			require.NoError(t, err)
			require.Len(t, findings, len(tt.want))
			for i, w := range tt.want {
				assert.True(t, strings.HasPrefix(findings[i].String(), w), "%q starts with %q", findings[i], w)
			}
		})
	}
}

// TestGeneratedGeneratorFails checks that a generator that exits non-zero
// is an error that carries its output, not a finding.
func TestGeneratedGeneratorFails(t *testing.T) {
	r := generatedFixture(t)
	_, err := generated(context.Background(), r.Repo, fsOps("fail"), nil)
	require.ErrorContains(t, err, "go generate")
	require.ErrorContains(t, err, "the generator says no")
}

// TestGeneratedNotAGitRepository checks that a directory outside a
// repository is an error, not a pass.
func TestGeneratedNotAGitRepository(t *testing.T) {
	r := generatedFixture(t)
	r.Dir = t.TempDir()
	_, err := generated(context.Background(), r.Repo, fsOps("write a x"), nil)
	require.Error(t, err)
}

// TestRunGeneratedTakesNoArgument checks the command line of the
// subcommand.
func TestRunGeneratedTakesNoArgument(t *testing.T) {
	var out, errOut bytes.Buffer
	e := env{dir: ".", stdout: &out, stderr: &errOut}
	assert.Equal(t, exitError, run(context.Background(), e, []string{"generated", "x"}))
	assert.Contains(t, errOut.String(), "generated takes no argument")
	assert.Equal(t, exitError, run(context.Background(), e, []string{"generate", "x"}))
	assert.Contains(t, errOut.String(), "generate takes no argument")
}

// goodAttributes is the list of the outputs of generate in the
// .gitattributes of a clean checkout.
const goodAttributes = ".github/CODEOWNERS -text\ndocs/reference/ask-first.md -text\ndocs/adr/README.md -text\n"

// committedGenerateFixture is a repository whose generated files are
// written by generate and committed: the state a clean checkout holds.
func committedGenerateFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := generateFixture(t)
	r.Write(attributesPath, goodAttributes)
	code, _, errOut := runGen(t, r)
	require.Equal(t, exitOK, code, errOut)
	r.Commit("fixture")
	return r
}

// gateOf runs the gate over r with a generator that does nothing, since
// the generator's effect is the subject of the tests above, and with
// the outputs of generate as the files that must be tracked.
func gateOf(t *testing.T, r *gittest.Repo) []string {
	t.Helper()
	findings, err := generated(context.Background(), r.Repo, fsOps(), generateOutputs)
	require.NoError(t, err)
	got := make([]string, len(findings))
	for i, f := range findings {
		got[i] = f.String()
	}
	return got
}

// TestGeneratedRequiresTrackedRegularOutputs is the attack of the
// round-1 audit: a generated file that the before and after snapshots
// cannot tell from a right one, because it is ignored, removed from the
// index, a link to a copy, a link out of the tree, executable or stale.
// Each one must fail, and a clean checkout must pass.
func TestGeneratedRequiresTrackedRegularOutputs(t *testing.T) {
	const codeowners = ".github/CODEOWNERS"
	tests := []struct {
		name  string
		setup func(t *testing.T, r *gittest.Repo)
		want  []string // "<path>: generated: <message start>"
	}{
		{"a clean checkout passes", func(*testing.T, *gittest.Repo) {}, nil},
		{"A: removed from the index and ignored", func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "--quiet", "--cached", codeowners)
			r.Write(".gitignore", codeowners+"\n")
			r.Commit("hide it")
			r.Git("clean", "-fdx", "--quiet")
		}, []string{codeowners + ": generated: is not tracked"}},
		{"untracked but not ignored", func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "--quiet", "--cached", codeowners)
			r.Git("commit", "--quiet", "--message", "untrack it")
		}, []string{codeowners + ": generated: is not tracked"}},
		{"tracked and ignored", func(t *testing.T, r *gittest.Repo) {
			r.Write(".gitignore", codeowners+"\n")
			r.Commit("ignore it")
		}, []string{codeowners + ": generated: is tracked and matches an ignore rule"}},
		{"B: a link to an identical copy", func(t *testing.T, r *gittest.Repo) {
			require.NoError(t, os.Rename(filepath.Join(r.Dir, codeowners), filepath.Join(r.Dir, "copy")))
			require.NoError(t, os.Symlink("../copy", filepath.Join(r.Dir, codeowners)))
			r.Commit("link it")
		}, []string{codeowners + ": generated: is tracked with mode 120000"}},
		{"F: a link to a path outside the tree", func(t *testing.T, r *gittest.Repo) {
			page := filepath.Join(r.Dir, filepath.FromSlash(askFirstPagePath))
			require.NoError(t, os.Remove(page))
			require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "outside.md"), page))
			r.Commit("link it out")
		}, []string{askFirstPagePath + ": generated: is tracked with mode 120000"}},
		{"executable", func(t *testing.T, r *gittest.Repo) {
			require.NoError(t, os.Chmod(filepath.Join(r.Dir, codeowners), 0o755))
			r.Commit("make it executable")
		}, []string{codeowners + ": generated: is tracked with mode 100755"}},
		{"committed with other bytes", func(t *testing.T, r *gittest.Repo) {
			r.Write(codeowners, "# a hand edit\n")
			r.Commit("edit by hand")
		}, []string{codeowners + ": generated: is tracked with other bytes"}},
		{"changed in the tree and not staged", func(t *testing.T, r *gittest.Repo) {
			r.Write(codeowners, "# a hand edit\n")
		}, nil}, // the index still holds the right bytes; the snapshots see no change either
		{"an output missing from the list", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, ".github/CODEOWNERS -text\ndocs/adr/README.md -text\n")
			r.Commit("drop a line")
		}, []string{askFirstPagePath + ": generated: .gitattributes does not list this file"}},
		{"no .gitattributes at all", func(t *testing.T, r *gittest.Repo) {
			r.Git("rm", "--quiet", attributesPath)
			r.Commit("drop the file")
		}, []string{
			codeowners + ": generated: .gitattributes does not list this file",
			"docs/adr/README.md: generated: .gitattributes does not list this file",
			askFirstPagePath + ": generated: .gitattributes does not list this file",
		}},
		{"a listed path that generate does not write", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"stale.md -text\n")
			r.Commit("list a stranger")
		}, []string{`.gitattributes: generated: lists "stale.md" with -text`}},
		{"a glob with -text is not the list", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"*.bin -text\n")
			r.Commit("add a glob")
		}, []string{`.gitattributes: generated: lists "*.bin" with -text`}},
		{"a comment that mentions -text is not an entry", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"# stale.md -text\n")
			r.Commit("comment")
		}, nil},
		{"a macro line with -text is not an entry", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"[attr]plain -text\n")
			r.Commit("macro")
		}, nil},
		{"another attribute on a stranger is not the list", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"stale.md text\n")
			r.Commit("text, not -text")
		}, nil},
		{"-text after another attribute still lists", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"stale.md eol=lf -text\n")
			r.Commit("two attributes")
		}, []string{`.gitattributes: generated: lists "stale.md" with -text`}},
		{"the index is judged, not HEAD", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, ".github/CODEOWNERS -text\ndocs/adr/README.md -text\n")
			r.Git("add", attributesPath)
		}, []string{askFirstPagePath + ": generated: .gitattributes does not list this file"}},
		{"a .gitattributes with CRLF line endings", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, strings.ReplaceAll(goodAttributes, "\n", "\r\n"))
			r.Commit("crlf")
		}, nil},
		{"a later !text resets the attribute", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" !text\n")
			r.Commit("reset")
		}, []string{codeowners + ": generated: has the text attribute \"unspecified\""}},
		{"text=unset is not -text", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" text=unset\n")
			r.Commit("value")
		}, []string{`.gitattributes: generated: gives ".github/CODEOWNERS" the value text=unset`}},
		{"text=auto before the list is not a state", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, "* text=auto\n"+goodAttributes)
			r.Commit("auto")
		}, nil},
		{"text=unset before the list is flagged", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, "* text=unset\n"+goodAttributes)
			r.Commit("unset")
		}, []string{`.gitattributes: generated: gives "*" the value text=unset`}},
		{"text=set and text=unspecified are flagged too", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, "* text=set\n* text=unspecified\n"+goodAttributes)
			r.Commit("states")
		}, []string{`.gitattributes: generated: gives "*" the value text=set`, `.gitattributes: generated: gives "*" the value text=unspecified`}},
		{"a nested file with text=unset", func(t *testing.T, r *gittest.Repo) {
			r.Write(".github/.gitattributes", "CODEOWNERS text=unset\n")
			r.Commit("nested")
		}, []string{`.github/.gitattributes: generated: gives "CODEOWNERS" the value text=unset`}},
		{"a macro with text=unset used on an output", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, "[attr]m text=unset\n"+goodAttributes+codeowners+" m\n")
			r.Commit("macro")
		}, []string{`.gitattributes: generated: gives "[attr]m" the value text=unset`}},
		{"a nested file that is not staged is not judged for text=unset", func(t *testing.T, r *gittest.Repo) {
			r.Write(".github/.gitattributes", "CODEOWNERS text=unset\n")
		}, nil},
		{"a quoted pattern", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"\"a b.md\" -text\n")
			r.Commit("quoted")
		}, []string{`.gitattributes: generated: quotes a pattern in "\"a b.md\" -text"; write the path unquoted`}},
		{"a quoted pattern with an escape is printed quoted", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"\"a\x1b[31m\" -text\n")
			r.Commit("quoted escape")
		}, []string{`.gitattributes: generated: quotes a pattern in "\"a\x1b[31m\" -text"; write the path unquoted`}},
		{"eol=unspecified reads like a state", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" eol=unspecified\n")
			r.Commit("eol string")
		}, []string{`.gitattributes: generated: gives ".github/CODEOWNERS" the value eol=unspecified`}},
		{"filter, ident and encoding strings read like a state", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+"none.md filter=unspecified ident=unspecified working-tree-encoding=unspecified\n")
			r.Commit("strings")
		}, []string{
			`.gitattributes: generated: gives "none.md" the value ident=unspecified`,
			`.gitattributes: generated: gives "none.md" the value filter=unspecified`,
			`.gitattributes: generated: gives "none.md" the value working-tree-encoding=unspecified`,
		}},
		{"ident", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" ident\n")
			r.Commit("ident")
		}, []string{codeowners + ": generated: has the ident attribute \"set\""}},
		{"a filter", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" filter=foo\n")
			r.Commit("filter")
		}, []string{codeowners + ": generated: has the filter attribute \"foo\""}},
		{"a working tree encoding", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" working-tree-encoding=UTF-8\n")
			r.Commit("encoding")
		}, []string{codeowners + ": generated: has the working-tree-encoding attribute \"UTF-8\""}},
		{"an eol", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, ".github/CODEOWNERS eol=lf -text\ndocs/reference/ask-first.md -text\ndocs/adr/README.md -text\n")
			r.Commit("eol")
		}, []string{codeowners + ": generated: has the eol attribute \"lf\""}},
		{"a later line sets text again", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, goodAttributes+codeowners+" text\n")
			r.Commit("override")
		}, []string{codeowners + ": generated: has the text attribute \"set\""}},
		{"a nested .gitattributes sets text again", func(t *testing.T, r *gittest.Repo) {
			r.Write(".github/.gitattributes", "CODEOWNERS text\n")
			r.Commit("override below")
		}, []string{codeowners + ": generated: has the text attribute \"set\""}},
		{"a nested .gitattributes that is not staged is not judged", func(t *testing.T, r *gittest.Repo) {
			r.Write(".github/.gitattributes", "CODEOWNERS text\n")
		}, nil},
		{"info/attributes of the clone sets text again", func(t *testing.T, r *gittest.Repo) {
			info := filepath.Join(r.Dir, ".git", "info")
			require.NoError(t, os.MkdirAll(info, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(codeowners+" text\n"), 0o644))
		}, []string{codeowners + ": generated: has the text attribute \"set\""}},
		{"info/attributes of the clone with text=unset", func(t *testing.T, r *gittest.Repo) {
			info := filepath.Join(r.Dir, ".git", "info")
			require.NoError(t, os.MkdirAll(info, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(codeowners+" text=unset\n"), 0o644))
		}, []string{`info/attributes: generated: gives ".github/CODEOWNERS" the value text=unset`}},
		{"a .gitattributes named in other case is scanned", func(t *testing.T, r *gittest.Repo) {
			r.Write("sub/.GitAttributes", "x text=unset\n")
			r.Commit("case")
		}, []string{`sub/.GitAttributes: generated: gives "x" the value text=unset`}},
		{"an executable .gitattributes is scanned", func(t *testing.T, r *gittest.Repo) {
			r.Write("sub/.gitattributes", "x text=unset\n")
			require.NoError(t, os.Chmod(filepath.Join(r.Dir, "sub", attributesPath), 0o755))
			r.Commit("exec")
		}, []string{`sub/.gitattributes: generated: gives "x" the value text=unset`}},
		{"a nested -text line is not on the list", func(t *testing.T, r *gittest.Repo) {
			r.Write("sub/.gitattributes", "x -text\n")
			r.Commit("nested -text")
		}, nil},
		{"the message names a root file in other case", func(t *testing.T, r *gittest.Repo) {
			r.Git("mv", attributesPath, ".GitAttributes")
			r.Write(".GitAttributes", ".github/CODEOWNERS -text\ndocs/adr/README.md -text\n")
			r.Commit("rename")
		}, []string{
			codeowners + `: generated: has the text attribute "unspecified"`,
			`docs/adr/README.md: generated: has the text attribute "unspecified"`,
			askFirstPagePath + ": generated: .GitAttributes does not list this file",
		}},
		{"a root file named in other case is the list", func(t *testing.T, r *gittest.Repo) {
			r.Git("mv", attributesPath, ".GitAttributes")
			r.Commit("rename")
		}, []string{
			codeowners + `: generated: has the text attribute "unspecified"`,
			`docs/adr/README.md: generated: has the text attribute "unspecified"`,
			askFirstPagePath + `: generated: has the text attribute "unspecified"`,
		}},
		{"info/attributes does not list an output", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, ".github/CODEOWNERS -text\ndocs/adr/README.md -text\n")
			r.Commit("drop a line")
			info := filepath.Join(r.Dir, ".git", "info")
			require.NoError(t, os.MkdirAll(info, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(askFirstPagePath+" -text\n"), 0o644))
		}, []string{askFirstPagePath + ": generated: .gitattributes does not list this file"}},
		{"a nested file in other case does not list an output", func(t *testing.T, r *gittest.Repo) {
			r.Write(attributesPath, ".github/CODEOWNERS -text\ndocs/adr/README.md -text\n")
			r.Write("sub/.GitAttributes", askFirstPagePath+" -text\n")
			r.Commit("nested")
		}, []string{askFirstPagePath + ": generated: .gitattributes does not list this file"}},
		{"a nested .gitattributes that is unmerged", func(t *testing.T, r *gittest.Repo) {
			r.Write("sub/.gitattributes", "a -text\n")
			r.Commit("base")
			r.Git("checkout", "--quiet", "-b", "other")
			r.Write("sub/.gitattributes", "b -text\n")
			r.Commit("other")
			r.Git("checkout", "--quiet", "main")
			r.Write("sub/.gitattributes", "c -text\n")
			r.Commit("main")
			_, err := r.Run(t.Context(), nil, "merge", "other")
			require.Error(t, err)
		}, []string{"sub/.gitattributes: generated: has an unmerged entry"}},
		{"a .gitattributes deleted on one side and edited on the other", func(t *testing.T, r *gittest.Repo) {
			r.Git("checkout", "--quiet", "-b", "other")
			r.Git("rm", "--quiet", attributesPath)
			r.Git("commit", "--quiet", "--message", "delete")
			r.Git("checkout", "--quiet", "main")
			r.Write(attributesPath, goodAttributes+"a -text\n")
			r.Commit("edit")
			_, err := r.Run(t.Context(), nil, "merge", "other")
			require.Error(t, err)
		}, []string{
			".gitattributes: generated: has an unmerged entry",
			codeowners + ": generated: .gitattributes does not list this file",
			"docs/adr/README.md: generated: .gitattributes does not list this file",
			askFirstPagePath + ": generated: .gitattributes does not list this file",
		}},
		{"a .gitattributes that is a link is not scanned", func(t *testing.T, r *gittest.Repo) {
			require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, "sub"), 0o755))
			require.NoError(t, os.Symlink("x text=unset", filepath.Join(r.Dir, "sub", attributesPath)))
			r.Commit("link")
		}, nil},
		{"a conflicted .gitattributes is a finding, not an error", func(t *testing.T, r *gittest.Repo) {
			r.Git("checkout", "--quiet", "-b", "other")
			r.Write(attributesPath, goodAttributes+"a -text\n")
			r.Commit("other")
			r.Git("checkout", "--quiet", "main")
			r.Write(attributesPath, goodAttributes+"b -text\n")
			r.Commit("main")
			_, err := r.Run(t.Context(), nil, "merge", "other")
			require.Error(t, err)
		}, []string{
			".gitattributes: generated: has an unmerged entry",
			codeowners + ": generated: .gitattributes does not list this file",
			"docs/adr/README.md: generated: .gitattributes does not list this file",
			askFirstPagePath + ": generated: .gitattributes does not list this file",
		}},
		{"unmerged", func(t *testing.T, r *gittest.Repo) {
			r.Git("checkout", "--quiet", "-b", "other")
			r.Write(codeowners, "# other\n")
			r.Commit("other")
			r.Git("checkout", "--quiet", "main")
			r.Write(codeowners, "# main\n")
			r.Commit("main")
			_, err := r.Run(t.Context(), nil, "merge", "other") // conflicts: that is the fixture
			require.Error(t, err)
		}, []string{codeowners + ": generated: has an unmerged entry"}},
	}
	for _, attr := range []string{"eol", "filter", "ident", "working-tree-encoding"} {
		for _, state := range []string{"set", "unset", "unspecified"} {
			line := "none.md " + attr + "=" + state + "\n"
			tests = append(tests, struct {
				name  string
				setup func(t *testing.T, r *gittest.Repo)
				want  []string
			}{attr + "=" + state + " reads like a state", func(t *testing.T, r *gittest.Repo) {
				r.Write(attributesPath, goodAttributes+line)
				r.Commit("string")
			}, []string{`.gitattributes: generated: gives "none.md" the value ` + attr + "=" + state}})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := committedGenerateFixture(t).For(t)
			tt.setup(t, r)
			got := gateOf(t, r)
			require.Len(t, got, len(tt.want), "%v", got)
			for i, w := range tt.want {
				assert.True(t, strings.HasPrefix(got[i], w), "%q starts with %q", got[i], w)
			}
		})
	}
}

// TestGeneratedInfoAttributesOfALinkedWorktree checks that the
// info/attributes of the common directory is read from a linked
// worktree, where git prints its path as an absolute one.
func TestGeneratedInfoAttributesOfALinkedWorktree(t *testing.T) {
	r := committedGenerateFixture(t).For(t)
	wt := filepath.Join(t.TempDir(), "wt")
	r.Git("worktree", "add", "--quiet", "--detach", wt)
	info := filepath.Join(r.Dir, ".git", "info")
	require.NoError(t, os.MkdirAll(info, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(".github/CODEOWNERS text=unset\n"), 0o644))
	findings, err := generated(t.Context(), git.Repo{Dir: wt, Env: r.Env}, fsOps(), generateOutputs)
	require.NoError(t, err)
	require.Len(t, findings, 1, "%v", findings)
	assert.Equal(t, `info/attributes: generated: gives ".github/CODEOWNERS" the value text=unset; git prints that string as it prints a state, so use -text`, findings[0].String())
}

// TestGeneratedFindingsPrintHostilePaths checks that a tracked path with
// a newline and an escape reaches a finding quoted, so it cannot forge a
// workflow command or move the terminal.
func TestGeneratedFindingsPrintHostilePaths(t *testing.T) {
	r := committedGenerateFixture(t).For(t)
	r.Write("d\n::error::forged\x1b[2J/.gitattributes", "x text=unset\n")
	r.Commit("hostile")
	got := gateOf(t, r)
	require.Len(t, got, 1, "%v", got)
	assert.NotContains(t, got[0], "\n")
	assert.NotContains(t, got[0], "\x1b")
	assert.True(t, strings.HasPrefix(got[0], `"d\n::error::forged\x1b[2J/.gitattributes": generated: gives "x"`), got[0])
}

// TestGeneratedInfoAttributesUnreadable checks that an info/attributes
// that cannot be read is an error, not a pass.
func TestGeneratedInfoAttributesUnreadable(t *testing.T) {
	r := committedGenerateFixture(t).For(t)
	require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, ".git", "info", "attributes"), 0o755))
	_, err := generated(t.Context(), r.Repo, fsOps(), generateOutputs)
	assert.Error(t, err)
}

// TestFileDigestStaysInsideTheRoot checks that a path that goes through
// a link out of the tree is an error, not a read of what lies there.
func TestFileDigestStaysInsideTheRoot(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "sub")))
	_, _, err := fileDigest(rootOf(t, dir), "sub/secret")
	require.Error(t, err)
}

// TestRunGeneratedRefusesAStrayDirectiveBeforeRunningAnything is the
// round-2 attack: a go:generate line that only go generate reads, in a
// raw string, must stop "generated" before the go command runs, so the
// command on the line never runs. The fixture holds no go.mod and no
// pinned go command, so a run that got past the refusal would end in an
// error (exit 2), not in findings (exit 1).
func TestRunGeneratedRefusesAStrayDirectiveBeforeRunningAnything(t *testing.T) {
	r := gittest.New(t)
	r.Write(generateSite, "package main\n\n"+generateDirective+"\n")
	r.Write("zz/z.go", "package z\n\nvar s = `\n//go:generate touch marker\n`\n")
	var out, errOut bytes.Buffer
	e := env{dir: r.Dir, gitEnv: r.Env, stdout: &out, stderr: &errOut}
	assert.Equal(t, exitFail, run(t.Context(), e, []string{"generated"}), errOut.String())
	assert.Contains(t, out.String(), "zz/z.go: generate-directive: ")
	assert.NoFileExists(t, filepath.Join(r.Dir, "marker"))
}

// TestParseCheckAttr pins the reading of "git check-attr -z": a short or
// long answer, or one for another path or attribute, is an error.
func TestParseCheckAttr(t *testing.T) {
	outs := []output{{path: "a"}}
	record := func(path, attr, value string) string { return path + "\x00" + attr + "\x00" + value + "\x00" }
	var good string
	for _, attr := range checkoutAttrs {
		good += record("a", attr, "unspecified")
	}
	values, err := parseCheckAttr([]byte(good), outs)
	require.NoError(t, err)
	assert.Equal(t, "unspecified", values[[2]string{"a", "text"}])
	_, err = parseCheckAttr([]byte(good+record("a", "text", "set")), outs)
	require.ErrorContains(t, err, "fields")
	_, err = parseCheckAttr([]byte(""), outs)
	require.ErrorContains(t, err, "fields")
	_, err = parseCheckAttr([]byte(strings.ReplaceAll(good, "a\x00text", "b\x00text")), outs)
	require.ErrorContains(t, err, "gave no text for a")
}

// TestGeneratedOutputsAreNotConverted runs attributeProblems on this
// repository: the .gitattributes of the index lists each file that
// generate writes with -text and nothing else, and no attribute that
// changes a checkout is set on them. A "text eol=crlf" rule would
// otherwise make a clean checkout differ from the bytes that generate
// gives, and "generated" would fail with no defect to fix. The cases of
// TestGeneratedRequiresTrackedRegularOutputs hold the failing side.
func TestGeneratedOutputsAreNotConverted(t *testing.T) {
	root := moduleRoot(t)
	r, err := os.OpenRoot(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	outs, err := generateOutputs(r)
	require.NoError(t, err)
	findings, err := attributeProblems(t.Context(), git.Repo{Dir: root}, outs)
	require.NoError(t, err)
	assert.Empty(t, findings)
}
