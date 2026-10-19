// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package drift

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

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/schema"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
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
			// The step's output is stdout and stderr together; println
			// writes to stderr and names no os file.
			println("the generator says no")
			os.Exit(3)
		}
	}
	os.Exit(0)
}

// fsOps returns a generator step that runs the operations.
func fsOps(ops ...string) check.Step {
	return check.Step{
		Name:    "go generate",
		Argv:    []string{os.Args[0], "-test.run=^TestHelperFsOps$"},
		Environ: []string{"PATH=" + os.Getenv("PATH"), fsOpsEnv + "=" + strings.Join(ops, "\n")},
	}
}

// generatedFixture is a repository with a committed generated file and
// the other kinds of file a tree holds: an untracked one, an ignored one,
// a symbolic link, and a tracked file that is deleted from the disk, which
// no Snapshot holds.
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
		gen  check.Step
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
	e := check.Env{Dir: ".", Stdout: &out, Stderr: &errOut}
	assert.Equal(t, check.ExitError, citest.Exit(context.Background(), e, Run, "x"))
	assert.Contains(t, errOut.String(), "generated takes no argument")
	assert.Equal(t, check.ExitError, citest.Exit(context.Background(), e, Generate, "x"))
	assert.Contains(t, errOut.String(), "generate takes no argument")
}

// indexPath is the index of the decision records, one of the two
// outputs of generate in the fixture.
const indexPath = "docs/adr/README.md"

// goodAttributes is the list of the outputs of generate in the
// .gitattributes of a clean checkout.
const goodAttributes = ".github/CODEOWNERS -text\ndocs/adr/README.md -text\n"

// committedGenerateFixture is a repository whose generated files are
// written by generate and committed: the state a clean checkout holds.
func committedGenerateFixture(t *testing.T) *gittest.Repo {
	t.Helper()
	r := generateFixture(t)
	r.Write(attributesPath, goodAttributes)
	code, _, errOut := runGen(t, r)
	require.Equal(t, check.ExitOK, code, errOut)
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
			t.Helper()
			r.Git("rm", "--quiet", "--cached", codeowners)
			r.Write(".gitignore", codeowners+"\n")
			r.Commit("hide it")
			r.Git("clean", "-fdx", "--quiet")
		}, []string{codeowners + ": generated: is not tracked"}},
		{"untracked but not ignored", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Git("rm", "--quiet", "--cached", codeowners)
			r.Git("commit", "--quiet", "--message", "untrack it")
		}, []string{codeowners + ": generated: is not tracked"}},
		{"tracked and ignored", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(".gitignore", codeowners+"\n")
			r.Commit("ignore it")
		}, []string{codeowners + ": generated: is tracked and matches an ignore rule"}},
		{"B: a link to an identical copy", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			require.NoError(t, os.Rename(filepath.Join(r.Dir, codeowners), filepath.Join(r.Dir, "copy")))
			require.NoError(t, os.Symlink("../copy", filepath.Join(r.Dir, codeowners)))
			r.Commit("link it")
		}, []string{codeowners + ": generated: is tracked with mode 120000"}},
		{"F: a link to a path outside the tree", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			page := filepath.Join(r.Dir, filepath.FromSlash(indexPath))
			require.NoError(t, os.Remove(page))
			require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "outside.md"), page))
			r.Commit("link it out")
		}, []string{indexPath + ": generated: is tracked with mode 120000"}},
		{"executable", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			require.NoError(t, os.Chmod(filepath.Join(r.Dir, codeowners), 0o755))
			r.Commit("make it executable")
		}, []string{codeowners + ": generated: is tracked with mode 100755"}},
		{"committed with other bytes", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(codeowners, "# a hand edit\n")
			r.Commit("edit by hand")
		}, []string{codeowners + ": generated: is tracked with other bytes"}},
		{"changed in the tree and not staged", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(codeowners, "# a hand edit\n")
		}, nil}, // the index still holds the right bytes; the snapshots see no change either
		{"an output missing from the list", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, ".github/CODEOWNERS -text\n")
			r.Commit("drop a line")
		}, []string{indexPath + ": generated: .gitattributes does not list this file"}},
		{"no .gitattributes at all", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Git("rm", "--quiet", attributesPath)
			r.Commit("drop the file")
		}, []string{
			codeowners + ": generated: .gitattributes does not list this file",
			"docs/adr/README.md: generated: .gitattributes does not list this file",
		}},
		{"a listed path that generate does not write", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"stale.md -text\n")
			r.Commit("list a stranger")
		}, []string{`.gitattributes: generated: lists "stale.md" with -text`}},
		{"a glob with -text is not the list", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"*.bin -text\n")
			r.Commit("add a glob")
		}, []string{`.gitattributes: generated: lists "*.bin" with -text`}},
		{"a comment that mentions -text is not an entry", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"# stale.md -text\n")
			r.Commit("comment")
		}, nil},
		{"a macro line with -text is not an entry", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"[attr]plain -text\n")
			r.Commit("macro")
		}, nil},
		{"another attribute on a stranger is not the list", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"stale.md text\n")
			r.Commit("text, not -text")
		}, nil},
		{"-text after another attribute still lists", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"stale.md eol=lf -text\n")
			r.Commit("two attributes")
		}, []string{`.gitattributes: generated: lists "stale.md" with -text`}},
		{"the index is judged, not HEAD", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, ".github/CODEOWNERS -text\n")
			r.Git("add", attributesPath)
		}, []string{indexPath + ": generated: .gitattributes does not list this file"}},
		{"a .gitattributes with CRLF line endings", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, strings.ReplaceAll(goodAttributes, "\n", "\r\n"))
			r.Commit("crlf")
		}, nil},
		{"a later !text resets the attribute", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" !text\n")
			r.Commit("reset")
		}, []string{codeowners + ": generated: has the text attribute \"unspecified\""}},
		{"text=unset is not -text", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" text=unset\n")
			r.Commit("value")
		}, []string{`.gitattributes: generated: gives ".github/CODEOWNERS" the value text=unset`}},
		{"text=auto before the list is not a state", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, "* text=auto\n"+goodAttributes)
			r.Commit("auto")
		}, nil},
		{"text=unset before the list is flagged", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, "* text=unset\n"+goodAttributes)
			r.Commit("unset")
		}, []string{`.gitattributes: generated: gives "*" the value text=unset`}},
		{"text=set and text=unspecified are flagged too", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, "* text=set\n* text=unspecified\n"+goodAttributes)
			r.Commit("states")
		}, []string{`.gitattributes: generated: gives "*" the value text=set`, `.gitattributes: generated: gives "*" the value text=unspecified`}},
		{"a nested file with text=unset", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(".github/.gitattributes", "CODEOWNERS text=unset\n")
			r.Commit("nested")
		}, []string{`.github/.gitattributes: generated: gives "CODEOWNERS" the value text=unset`}},
		{"a macro with text=unset used on an output", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, "[attr]m text=unset\n"+goodAttributes+codeowners+" m\n")
			r.Commit("macro")
		}, []string{`.gitattributes: generated: gives "[attr]m" the value text=unset`}},
		{"a nested file that is not staged is not judged for text=unset", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(".github/.gitattributes", "CODEOWNERS text=unset\n")
		}, nil},
		{"a quoted pattern", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"\"a b.md\" -text\n")
			r.Commit("quoted")
		}, []string{`.gitattributes: generated: quotes a pattern in "\"a b.md\" -text"; write the path unquoted`}},
		{"a quoted pattern with an escape is printed quoted", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"\"a\x1b[31m\" -text\n")
			r.Commit("quoted escape")
		}, []string{`.gitattributes: generated: quotes a pattern in "\"a\x1b[31m\" -text"; write the path unquoted`}},
		{"eol=unspecified reads like a state", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" eol=unspecified\n")
			r.Commit("eol string")
		}, []string{`.gitattributes: generated: gives ".github/CODEOWNERS" the value eol=unspecified`}},
		{"filter, ident and encoding strings read like a state", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+"none.md filter=unspecified ident=unspecified working-tree-encoding=unspecified\n")
			r.Commit("strings")
		}, []string{
			`.gitattributes: generated: gives "none.md" the value ident=unspecified`,
			`.gitattributes: generated: gives "none.md" the value filter=unspecified`,
			`.gitattributes: generated: gives "none.md" the value working-tree-encoding=unspecified`,
		}},
		{"ident", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" ident\n")
			r.Commit("ident")
		}, []string{codeowners + ": generated: has the ident attribute \"set\""}},
		{"a filter", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" filter=foo\n")
			r.Commit("filter")
		}, []string{codeowners + ": generated: has the filter attribute \"foo\""}},
		{"a working tree encoding", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" working-tree-encoding=UTF-8\n")
			r.Commit("encoding")
		}, []string{codeowners + ": generated: has the working-tree-encoding attribute \"UTF-8\""}},
		{"an eol", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, ".github/CODEOWNERS eol=lf -text\ndocs/adr/README.md -text\n")
			r.Commit("eol")
		}, []string{codeowners + ": generated: has the eol attribute \"lf\""}},
		{"a later line sets text again", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, goodAttributes+codeowners+" text\n")
			r.Commit("override")
		}, []string{codeowners + ": generated: has the text attribute \"set\""}},
		{"a nested .gitattributes sets text again", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(".github/.gitattributes", "CODEOWNERS text\n")
			r.Commit("override below")
		}, []string{codeowners + ": generated: has the text attribute \"set\""}},
		{"a nested .gitattributes that is not staged is not judged", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(".github/.gitattributes", "CODEOWNERS text\n")
		}, nil},
		{"info/attributes of the clone sets text again", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			info := filepath.Join(r.Dir, ".git", "info")
			require.NoError(t, os.MkdirAll(info, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(codeowners+" text\n"), 0o644))
		}, []string{codeowners + ": generated: has the text attribute \"set\""}},
		{"info/attributes of the clone with text=unset", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			info := filepath.Join(r.Dir, ".git", "info")
			require.NoError(t, os.MkdirAll(info, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(codeowners+" text=unset\n"), 0o644))
		}, []string{`info/attributes: generated: gives ".github/CODEOWNERS" the value text=unset`}},
		{"a .gitattributes named in other case is scanned", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write("sub/.GitAttributes", "x text=unset\n")
			r.Commit("case")
		}, []string{`sub/.GitAttributes: generated: gives "x" the value text=unset`}},
		{"an executable .gitattributes is scanned", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write("sub/.gitattributes", "x text=unset\n")
			require.NoError(t, os.Chmod(filepath.Join(r.Dir, "sub", attributesPath), 0o755))
			r.Commit("exec")
		}, []string{`sub/.gitattributes: generated: gives "x" the value text=unset`}},
		{"a nested -text line is not on the list", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write("sub/.gitattributes", "x -text\n")
			r.Commit("nested -text")
		}, nil},
		{"the message names a root file in other case", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Git("mv", attributesPath, ".GitAttributes")
			r.Write(".GitAttributes", ".github/CODEOWNERS -text\n")
			r.Commit("rename")
		}, []string{
			codeowners + `: generated: has the text attribute "unspecified"`,
			indexPath + ": generated: .GitAttributes does not list this file with -text; rename it to .gitattributes",
		}},
		{"a root file named in other case is the list", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Git("mv", attributesPath, ".GitAttributes")
			r.Commit("rename")
		}, []string{
			codeowners + `: generated: has the text attribute "unspecified"`,
			indexPath + `: generated: has the text attribute "unspecified"`,
		}},
		{"info/attributes does not list an output", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, ".github/CODEOWNERS -text\n")
			r.Commit("drop a line")
			info := filepath.Join(r.Dir, ".git", "info")
			require.NoError(t, os.MkdirAll(info, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(info, "attributes"), []byte(indexPath+" -text\n"), 0o644))
		}, []string{indexPath + ": generated: .gitattributes does not list this file"}},
		{"a nested file in other case does not list an output", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, ".github/CODEOWNERS -text\n")
			r.Write("sub/.GitAttributes", indexPath+" -text\n")
			r.Commit("nested")
		}, []string{indexPath + ": generated: .gitattributes does not list this file"}},
		{"a nested .gitattributes that is unmerged", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
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
			t.Helper()
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
		}},
		{"a .gitattributes that is a link is not scanned", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			require.NoError(t, os.MkdirAll(filepath.Join(r.Dir, "sub"), 0o755))
			require.NoError(t, os.Symlink("x text=unset", filepath.Join(r.Dir, "sub", attributesPath)))
			r.Commit("link")
		}, nil},
		{"a conflicted .gitattributes is a finding, not an error", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
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
		}},
		{"an oversized root .gitattributes is a finding, and lists nothing", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
			r.Write(attributesPath, sized(attributesMaxSize+1, goodAttributes))
			r.Commit("oversized")
		}, []string{
			".gitattributes: generated: is " + strconv.Itoa(attributesMaxSize+1) + " bytes",
			codeowners + ": generated: .gitattributes does not list this file",
			"docs/adr/README.md: generated: .gitattributes does not list this file",
		}},
		{"unmerged", func(t *testing.T, r *gittest.Repo) {
			t.Helper()
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
				t.Helper()
				r.Write(attributesPath, goodAttributes+line)
				r.Commit("string")
			}, []string{`.gitattributes: generated: gives "none.md" the value ` + attr + "=" + state +
				"; git prints that string as it prints a state, so write the state as " + attr + ", -" + attr + " or !" + attr}})
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
	assert.Equal(t, `info/attributes: generated: gives ".github/CODEOWNERS" the value text=unset; git prints that string as it prints a state, so write the state as text, -text or !text`, findings[0].String())
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

// TestAttributeProblemsQuoteHostilePaths checks that an output path with a
// newline and an escape (the path comes from .adr-dir) is quoted in the
// message and in the error, so neither carries a raw control character.
func TestAttributeProblemsQuoteHostilePaths(t *testing.T) {
	r := committedGenerateFixture(t).For(t)
	hostile := "d\n::error::forged\x1b[2J/README.md"
	findings, err := attributeProblems(t.Context(), r.Repo, []output{{path: hostile}})
	require.NoError(t, err)
	require.NotEmpty(t, findings)
	for _, f := range findings {
		assert.NotContains(t, f.Msg, "\n")
		assert.NotContains(t, f.Msg, "\x1b")
	}
	assert.Contains(t, findings[0].Msg, `add the line "d\n::error::forged\x1b[2J/README.md -text"`)
	var other string
	for _, attr := range checkoutAttrs {
		other += "b\x00" + attr + "\x00unspecified\x00"
	}
	_, err = parseCheckAttr([]byte(other), []output{{path: hostile}})
	require.ErrorContains(t, err, "gave no text for")
	assert.NotContains(t, err.Error(), "\n")
	assert.NotContains(t, err.Error(), "\x1b")
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
	_, _, err := fileDigest(citest.RootOf(t, dir), "sub/secret")
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
	r.Write(Site, "package main\n\n"+generateDirective+"\n")
	r.Write("zz/z.go", "package z\n\nvar s = `\n//go:generate touch marker\n`\n")
	var out, errOut bytes.Buffer
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdout: &out, Stderr: &errOut}
	assert.Equal(t, check.ExitFail, citest.Exit(t.Context(), e, Run), errOut.String())
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
	require.ErrorContains(t, err, `gave no text for "a"`)
}

// TestFindingPathCannotStartACommand checks that a path that is printable
// and starts like a workflow command does not begin a line of the log
// as one.
func TestFindingPathCannotStartACommand(t *testing.T) {
	for _, where := range []string{"::error::x/.gitattributes", "  ::stop-commands::tok/f", "##[error]x/f", "\t::error::x/f", "\u00a0::error::x/f", "\u200b::error::x/f", "a/##[error]x"} {
		line := check.Finding{Where: where, Rule: "generated", Msg: "m"}.String()
		assert.False(t, strings.HasPrefix(strings.TrimSpace(line), "::"), "%q", line)
		assert.NotContains(t, line, "##[")
		assert.Equal(t, line, git.SafeLines(line), "already safe: %q", line)
	}
	assert.Equal(t, "a/b: generated: m", check.Finding{Where: "a/b", Rule: "generated", Msg: "m"}.String())
}

// TestGeneratedOutputsAreNotConverted runs attributeProblems on this
// repository: the .gitattributes of the index lists each file that
// generate writes with -text and nothing else, and no attribute that
// changes a checkout is set on them. A "text eol=crlf" rule would
// otherwise make a clean checkout differ from the bytes that generate
// gives, and "generated" would fail with no defect to fix. The cases of
// TestGeneratedRequiresTrackedRegularOutputs hold the failing side.
func TestGeneratedOutputsAreNotConverted(t *testing.T) {
	root := citest.ModuleRoot(t)
	r, err := os.OpenRoot(root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	// The files the generators of the registry of this test write, the
	// inventory of the checks, and the schemas of the formats registered
	// with tools/schemagen.
	outs, err := generateOutputs(r)
	require.NoError(t, err)
	checks, err := check.ChecksPageOutput(r)
	require.NoError(t, err)
	outs = append(outs, output{checks[0].Path, checks[0].Content})
	schemas, err := schema.Outputs(schemagen.Registered())
	require.NoError(t, err)
	for _, o := range schemas {
		outs = append(outs, output{o.Path, o.Content})
	}
	findings, err := attributeProblems(t.Context(), git.Repo{Dir: root}, outs)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// scanFixture is a repository with the given .gitattributes files
// committed, each path to its content.
func scanFixture(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	for name, content := range files {
		r.Write(name, content)
	}
	r.Commit("attributes")
	return r
}

// scanFindings runs scanAttributes over repo and returns its findings
// as strings.
func scanFindings(t *testing.T, repo git.Repo) []string {
	t.Helper()
	_, _, problems, err := scanAttributes(t.Context(), repo)
	require.NoError(t, err)
	got := make([]string, len(problems))
	for i, f := range problems {
		got[i] = f.String()
	}
	return got
}

// sized returns attributes text of exactly n bytes that ends with line:
// comment padding, then line.
func sized(n int, line string) string {
	pad := n - len(line) - 1
	return "#" + strings.Repeat("x", pad-1) + "\n" + line
}

// TestScanAttributesSizeCap checks the bound on what scanAttributes
// reads: a file at the cap is read, one byte more is a finding and is
// not read, and a file after an oversized one is still read whole.
func TestScanAttributesSizeCap(t *testing.T) {
	const problem = "x text=unset\n"
	over := `: generated: is ` + strconv.Itoa(attributesMaxSize+1) + ` bytes`
	stringFinding := `: generated: gives "x" the value text=unset`
	tests := []struct {
		name  string
		files map[string]string
		want  []string // "<path><message start>"
	}{
		{"a file at the cap is read", map[string]string{"a/.gitattributes": sized(attributesMaxSize, problem)},
			[]string{"a/.gitattributes" + stringFinding}},
		{"a file one byte over the cap is a finding, not a read", map[string]string{"a/.gitattributes": sized(attributesMaxSize+1, problem)},
			[]string{"a/.gitattributes" + over}},
		{"a file after an oversized one is read", map[string]string{
			"a/.gitattributes": sized(attributesMaxSize+1, problem),
			"b/.gitattributes": problem,
		}, []string{"a/.gitattributes" + over, "b/.gitattributes" + stringFinding}},
		{"an oversized root file is a finding", map[string]string{attributesPath: sized(attributesMaxSize+1, problem)},
			[]string{attributesPath + over}},
		{"files with the same content are each read", map[string]string{
			"a/.gitattributes": problem,
			"b/.gitattributes": problem,
		}, []string{"a/.gitattributes" + stringFinding, "b/.gitattributes" + stringFinding}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scanFindings(t, scanFixture(t, tt.files).Repo)
			require.Len(t, got, len(tt.want), "%v", got)
			for i, w := range tt.want {
				assert.True(t, strings.HasPrefix(got[i], w), "%q starts with %q", got[i], w)
			}
		})
	}
}

// gitCall is one run of git that recordGit saw: its arguments, and its
// standard input when it is a cat-file.
type gitCall struct{ args, stdin string }

// recordGit puts first on PATH a git that records each run of the real
// one, and returns a function that reads the runs so far. Run looks git
// up in the PATH of this process, not in the environment of the command,
// so it sets PATH with t.Setenv, which also keeps a test that calls it
// from running in parallel with another.
func recordGit(t *testing.T) func() []gitCall {
	t.Helper()
	real, err := procgroup.LookPath("git")
	require.NoError(t, err)
	bin, calls := t.TempDir(), t.TempDir()
	wrapper := "#!/bin/sh\n" +
		"f=$(mktemp " + strconv.Quote(calls+"/call.XXXXXX") + ")\n" +
		"printf '%s' \"$*\" > \"$f\"\n" +
		"case \" $* \" in *\" cat-file \"*) cat > \"$f.in\"; exec " + strconv.Quote(real) + " \"$@\" < \"$f.in\";; esac\n" +
		"exec " + strconv.Quote(real) + " \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []gitCall {
		names, err := filepath.Glob(filepath.Join(calls, "call.*"))
		require.NoError(t, err)
		var got []gitCall
		for _, name := range names {
			if strings.HasSuffix(name, ".in") {
				continue
			}
			args, err := os.ReadFile(name)
			require.NoError(t, err)
			stdin, err := os.ReadFile(name + ".in")
			if !os.IsNotExist(err) {
				require.NoError(t, err)
			}
			got = append(got, gitCall{string(args), string(stdin)})
		}
		return got
	}
}

// catFileInput returns the standard input of each run of git cat-file
// with the option that starts with prefix.
func catFileInput(calls []gitCall, prefix string) []string {
	var got []string
	for _, c := range calls {
		if strings.Contains(" "+c.args, " cat-file "+prefix) {
			got = append(got, c.stdin)
		}
	}
	return got
}

// TestScanAttributesReadsManyFilesInOneBatch checks that many nested
// .gitattributes are each read, and that reading them takes one
// --batch-check and one --batch and not a git process for each file.
func TestScanAttributesReadsManyFilesInOneBatch(t *testing.T) {
	const n = 300
	files := map[string]string{}
	for i := range n {
		files["d"+strconv.Itoa(i)+"/.gitattributes"] = "x" + strconv.Itoa(i) + " text=unset\n"
	}
	r := scanFixture(t, files)
	calls := recordGit(t)
	got := scanFindings(t, r.Repo)
	assert.Len(t, got, n)
	var catFile int
	for _, c := range calls() {
		if strings.Contains(" "+c.args+" ", " cat-file ") {
			catFile++
		}
	}
	assert.Equal(t, 2, catFile, "cat-file runs")
	assert.Len(t, catFileInput(calls(), "--batch-check="), 1)
	assert.Len(t, catFileInput(calls(), "--batch="), 1)
}

// TestScanAttributesSendsOnlyWhatItReads checks what each cat-file is
// asked: every distinct blob once to --batch-check, and to --batch only
// the blobs within the bounds, so an oversized blob never reaches the
// command that prints content.
func TestScanAttributesSendsOnlyWhatItReads(t *testing.T) {
	const same = "x text=unset\n"
	r := scanFixture(t, map[string]string{
		"a/.gitattributes": sized(attributesMaxSize+1, "x -text\n"),
		"b/.gitattributes": same,
		"c/.gitattributes": same,
		"d/.gitattributes": same,
	})
	big := r.Git("rev-parse", ":a/.gitattributes")
	small := r.Git("rev-parse", ":b/.gitattributes")
	calls := recordGit(t)
	got := scanFindings(t, r.Repo)
	require.Len(t, got, 4, "%v", got)
	assert.True(t, strings.HasPrefix(got[0], "a/.gitattributes: generated: is "), got[0])
	for i, dir := range []string{"b", "c", "d"} {
		assert.True(t, strings.HasPrefix(got[i+1], dir+`/.gitattributes: generated: gives "x" the value text=unset`), got[i+1])
	}
	check := catFileInput(calls(), "--batch-check=")
	require.Len(t, check, 1)
	assert.ElementsMatch(t, []string{big, small}, strings.Fields(check[0]), "each distinct blob once")
	read := catFileInput(calls(), "--batch=")
	require.Len(t, read, 1)
	assert.Equal(t, small+"\n", read[0], "only the blob within the bound")
}

// TestScanAttributesTotalCap checks the bound on all that scanAttributes
// reads: blobs that fill attributesMaxTotal exactly are read, a file
// after them is a finding and is not read, and a smaller file after one
// that does not fit is still read.
func TestScanAttributesTotalCap(t *testing.T) {
	const problem = "x text=unset\n"
	stringFinding := `: generated: gives "x" the value text=unset`
	notRead := ": generated: is not read: the .gitattributes files read before it (the root one, then the others in path order) leave too little of the 8 MiB"
	fill := func(size int) map[string]string {
		files := map[string]string{}
		for i := range attributesMaxTotal / attributesMaxSize {
			// Distinct content, so each is a blob of its own.
			files["f"+strconv.Itoa(i)+"/.gitattributes"] = sized(size, "#"+strconv.Itoa(i)+"\n")
		}
		return files
	}
	t.Run("a file past the total is a finding", func(t *testing.T) {
		files := fill(attributesMaxSize)
		files["g/.gitattributes"] = problem
		got := scanFindings(t, scanFixture(t, files).Repo)
		require.Len(t, got, 1, "%v", got)
		assert.True(t, strings.HasPrefix(got[0], "g/.gitattributes"+notRead), got[0])
	})
	t.Run("blobs that fill the total exactly are read", func(t *testing.T) {
		files := fill(attributesMaxSize)
		files["f0/.gitattributes"] = sized(attributesMaxSize, problem)
		got := scanFindings(t, scanFixture(t, files).Repo)
		require.Len(t, got, 1, "%v", got)
		assert.True(t, strings.HasPrefix(got[0], "f0/.gitattributes"+stringFinding), got[0])
	})
	t.Run("the root file is read first", func(t *testing.T) {
		// '-' sorts before '.', so in path order these fill the total
		// before the root file.
		files := map[string]string{attributesPath: problem}
		for i := range attributesMaxTotal / attributesMaxSize {
			files["-"+strconv.Itoa(i)+"/.gitattributes"] = sized(attributesMaxSize, "#"+strconv.Itoa(i)+"\n")
		}
		got := scanFindings(t, scanFixture(t, files).Repo)
		require.Len(t, got, 2, "%v", got)
		assert.True(t, strings.HasPrefix(got[0], "-7/.gitattributes"+notRead), got[0])
		assert.True(t, strings.HasPrefix(got[1], attributesPath+stringFinding), got[1])
	})
	t.Run("a shared blob counts once and an oversized one not at all", func(t *testing.T) {
		files := fill(attributesMaxSize) // f0 to f7, the total exactly
		files["0/.gitattributes"] = sized(attributesMaxSize+1, problem)
		files["g/.gitattributes"] = files["f0/.gitattributes"]
		files["h/.gitattributes"] = files["f1/.gitattributes"]
		got := scanFindings(t, scanFixture(t, files).Repo)
		require.Len(t, got, 1, "%v", got)
		assert.True(t, strings.HasPrefix(got[0], "0/.gitattributes: generated: is "), got[0])
	})
	t.Run("a smaller file after one that does not fit is read", func(t *testing.T) {
		files := fill(attributesMaxSize - 2) // 16 bytes left
		files["g/.gitattributes"] = sized(attributesMaxSize, problem)
		files["h/.gitattributes"] = problem
		got := scanFindings(t, scanFixture(t, files).Repo)
		require.Len(t, got, 2, "%v", got)
		assert.True(t, strings.HasPrefix(got[0], "g/.gitattributes"+notRead), got[0])
		assert.True(t, strings.HasPrefix(got[1], "h/.gitattributes"+stringFinding), got[1])
	})
}

// TestScanAttributesMissingObject checks that an index entry whose blob
// the object store does not hold is an error, not a pass.
func TestScanAttributesMissingObject(t *testing.T) {
	r := scanFixture(t, map[string]string{"a/.gitattributes": "x -text\n"})
	const absent = "0123456789abcdef0123456789abcdef01234567"
	r.Git("update-index", "--add", "--cacheinfo", "100644,"+absent+",b/.gitattributes")
	_, _, _, err := scanAttributes(t.Context(), r.Repo)
	require.ErrorContains(t, err, "missing")
	assert.Contains(t, err.Error(), `"b/.gitattributes"`)
}

// TestScanAttributesMissingAmongOthers checks that a missing blob is an
// error that names its file when the index also holds a readable file
// and an oversized one, and when the same blob is missing for two files
// the first of them is named.
func TestScanAttributesMissingAmongOthers(t *testing.T) {
	r := scanFixture(t, map[string]string{
		"a/.gitattributes": "x -text\n",
		"z/.gitattributes": sized(attributesMaxSize+1, "x -text\n"),
	})
	const absent = "0123456789abcdef0123456789abcdef01234567"
	r.Git("update-index", "--add", "--cacheinfo", "100644,"+absent+",n/.gitattributes")
	r.Git("update-index", "--add", "--cacheinfo", "100644,"+absent+",p/.gitattributes")
	_, _, _, err := scanAttributes(t.Context(), r.Repo)
	require.ErrorContains(t, err, `reading "n/.gitattributes" of the index: git cat-file: blob `+absent+": the object store is missing it")
}

// TestScanAttributesFromALinkedWorktree checks that the nested files of
// the index, an oversized one among them, are read from a linked
// worktree as from the main one.
func TestScanAttributesFromALinkedWorktree(t *testing.T) {
	r := scanFixture(t, map[string]string{
		"a/.gitattributes": sized(attributesMaxSize+1, "x -text\n"),
		"b/.gitattributes": "x text=unset\n",
	})
	wt := filepath.Join(t.TempDir(), "wt")
	r.Git("worktree", "add", "--quiet", "--detach", wt)
	got := scanFindings(t, git.Repo{Dir: wt, Env: r.Env})
	require.Len(t, got, 2, "%v", got)
	assert.True(t, strings.HasPrefix(got[0], "a/.gitattributes: generated: is "), got[0])
	assert.True(t, strings.HasPrefix(got[1], `b/.gitattributes: generated: gives "x" the value text=unset`), got[1])
}

// TestParseBatch pins the reading of "git cat-file --batch": each
// record is framed by the size in its header, so content that looks
// like a header is content, and a short, missing or foreign record is
// an error.
func TestParseBatch(t *testing.T) {
	const a, b = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const empty = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	fake := b + " blob 5\nnope\n"
	good := a + " blob " + strconv.Itoa(len(fake)) + "\n" + fake + "\n" + empty + " blob 0\n\n" + b + " blob 3\nyes\n"
	got, err := parseBatch([]byte(good), []string{a, empty, b})
	require.NoError(t, err)
	assert.Equal(t, fake, string(got[a]))
	assert.Contains(t, got, empty)
	assert.Empty(t, got[empty])
	assert.Equal(t, "yes", string(got[b]))
	for name, tc := range map[string]struct{ out, want string }{
		"short content":             {a + " blob 9\nabc\n", "short"},
		"no room for the line feed": {a + " blob 4\nabc\n", "short"},
		"no line feed after":        {a + " blob 3\nabcX", "line feed"},
		"a size past the end":       {a + " blob 9223372036854775807\nabc\n", "short"},
		"missing":                   {a + " missing\n", "missing"},
		"not a blob":                {a + " tree 3\nabc\n", "tree"},
		"another object":            {b + " blob 3\nabc\n", b},
		"a record too many":         {a + " blob 1\nx\n" + a + " blob 1\nx\n", "after"},
		"no record":                 {"", "no header line"},
		"a header with no LF":       {a + " blob 3", "header"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseBatch([]byte(tc.out), []string{a})
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// TestParseBatchCheck pins the reading of "git cat-file --batch-check":
// a size for each blob, a missing object listed and not an error, and
// a short answer, one for another object or output after the last
// record an error.
func TestParseBatchCheck(t *testing.T) {
	const a, b = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	sizes, err := parseBatchCheck([]byte(a+" blob 7\n"+b+" blob 0\n"), []string{a, b})
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{a: 7, b: 0}, sizes)
	for name, tc := range map[string]struct{ out, want string }{
		"missing":             {a + " blob 7\n" + b + " missing\n", "blob " + b + ": the object store is missing it"},
		"short":               {a + " blob 7\n", "blob " + b + ": no header line"},
		"another object":      {a + " blob 7\n" + a + " blob 7\n", "the answer is for " + a},
		"a record too many":   {a + " blob 7\n" + b + " blob 1\n" + b + " blob 1\n", "after"},
		"a negative size":     {a + " blob -1\n" + b + " blob 1\n", `the size "-1"`},
		"a size not a number": {a + " blob 7x\n" + b + " blob 1\n", `the size "7x"`},
		"not a blob":          {a + " tree 7\n" + b + " blob 1\n", "a tree, not a blob"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseBatchCheck([]byte(tc.out), []string{a, b})
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// TestScanAttributesBlobGoneBeforeTheRead checks an object that
// --batch-check finds and --batch then does not, as when it is removed
// between the two: the error names the file that holds it. A git first
// on PATH answers "missing" to --batch.
func TestScanAttributesBlobGoneBeforeTheRead(t *testing.T) {
	r := scanFixture(t, map[string]string{"a/.gitattributes": "x -text\n"})
	oid := r.Git("rev-parse", ":a/.gitattributes")
	real, err := procgroup.LookPath("git")
	require.NoError(t, err)
	bin := t.TempDir()
	wrapper := "#!/bin/sh\n" +
		"case \" $* \" in *\" cat-file --batch=\"*) while read -r id; do printf '%s missing\\n' \"$id\"; done; exit 0;; esac\n" +
		"exec " + strconv.Quote(real) + " \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH")) // serial, as recordGit
	_, _, _, err = scanAttributes(t.Context(), r.Repo)
	require.ErrorContains(t, err, `reading "a/.gitattributes" of the index: git cat-file: blob `+oid+": the object store is missing it; ")
	assert.Contains(t, err.Error(), `"git cat-file -p `+oid+`"`)
}
