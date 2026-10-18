// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// rawEntry is one entry of a tree object as git stores it: the mode
// without a leading zero, the name and the object.
type rawEntry struct {
	mode, name, oid string
}

// writeTree stores a tree object of the entries in the repository in
// dir with hash-object --literally, which checks nothing, so a hostile
// tree that no git command would write can be built.
func writeTree(t *testing.T, dir string, entries ...rawEntry) string {
	t.Helper()
	var b bytes.Buffer
	for _, e := range entries {
		raw, err := hex.DecodeString(e.oid)
		require.NoError(t, err)
		b.WriteString(e.mode + " " + e.name)
		b.WriteByte(0)
		b.Write(raw)
	}
	file := filepath.Join(t.TempDir(), "tree")
	require.NoError(t, os.WriteFile(file, b.Bytes(), 0o600))
	return plainGit(t, dir, "hash-object", "-w", "-t", "tree", "--literally", file)
}

// writeBlob stores a blob of content in the repository in dir.
func writeBlob(t *testing.T, dir, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "blob")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
	return plainGit(t, dir, "hash-object", "-w", file)
}

// commitTree stores a commit of tree in the repository in dir.
func commitTree(t *testing.T, dir, tree string) string {
	t.Helper()
	return plainGit(t, dir, "commit-tree", tree, "-m", "tree")
}

// TestWalkTree lists the files of a tree I27 accepts, in git's order,
// each with its mode and blob, and no directory.
func TestWalkTree(t *testing.T) {
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, _ := newRepo(t)
			blob := writeBlob(t, dir, "x\n")
			script := writeBlob(t, dir, "#!/bin/sh\n")
			docs := writeTree(t, dir, rawEntry{"100644", "guide.md", blob})
			bin := writeTree(t, dir, rawEntry{"100755", "run", script})
			root := writeTree(t, dir, rawEntry{"100644", "README", blob}, rawEntry{"40000", "bin", bin}, rawEntry{"40000", "docs", docs}, rawEntry{"100644", "\u00e9t\u00e9.txt", blob})
			got, err := newRunner(t, mode).WalkTree(t.Context(), dir, commitTree(t, dir, root))
			require.NoError(t, err)
			assert.Equal(t, []TreeEntry{
				{"README", modeFile, blob},
				{"bin/run", modeExecutable, script},
				{"docs/guide.md", modeFile, blob},
				{"\u00e9t\u00e9.txt", modeFile, blob},
			}, got)
		})
	}
}

// TestWalkTreeRefusesHostileTrees is the unit table of I27: each tree is
// refused by the walk alone, with no fsck run before it.
//
//romeu:invariant I27 test
func TestWalkTreeRefusesHostileTrees(t *testing.T) {
	dir, head := newRepo(t)
	blob := writeBlob(t, dir, "x\n")
	sub := func(entries ...rawEntry) string { return writeTree(t, dir, entries...) }
	file := func(name string) rawEntry { return rawEntry{"100644", name, blob} }
	tests := []struct {
		name    string
		entries []rawEntry
		want    string
	}{
		{"symlink", []rawEntry{{"120000", "link", blob}}, `"link" is a symbolic link`},
		{"gitlink", []rawEntry{{"160000", "module", head}}, `"module" is a gitlink`},
		{"dot", []rawEntry{file(".")}, `holds the name "."`},
		{"dot dot", []rawEntry{file("..")}, `holds the name ".."`},
		{".git", []rawEntry{file(".git")}, `holds the name ".git"`},
		{".GIT", []rawEntry{file(".GIT")}, `holds the name ".GIT"`},
		{".Git directory", []rawEntry{{"40000", ".Git", sub(file("config"))}}, `holds the name ".Git"`},
		{".git below a directory", []rawEntry{{"40000", "a", sub(rawEntry{"40000", ".git", sub(file("hooks"))})}}, `"a/.git" holds the name ".git"`},
		{"zero width non-joiner after the dot", []rawEntry{file(".\u200cgit")}, `holds the name ".\u200cgit"`},
		{"zero width joiner inside", []rawEntry{file(".gi\u200dt")}, `holds the name ".gi\u200dt"`},
		{"no-break space at the end", []rawEntry{file(".git\ufeff")}, `holds the name ".git\ufeff"`},
		{"direction mark before the dot", []rawEntry{file("\u200e.GiT")}, `holds the name "\u200e.GiT"`},
		{"ignorable dot dot", []rawEntry{file(".\u206a.")}, `holds the name ".\u206a."`},
		{"only ignorable characters", []rawEntry{file("\u200c\u200d")}, `holds an empty name`},
		{"empty name, which git ls-tree refuses itself", []rawEntry{file("")}, `gitsafe: git ls-tree`},
		{"case collision", []rawEntry{file("README"), file("readme")}, `"README" and "readme" are one path`},
		{"directory and file collide", []rawEntry{{"40000", "Docs", sub(file("x"))}, file("docs")}, `are one path under case-insensitive comparison`},
		{"collision below a directory", []rawEntry{{"40000", "d", sub(file("X"), file("x"))}}, `"d/X" and "d/x" are one path`},
		{"an ignorable character inside a name", []rawEntry{file("ab"), file("a\u200cb")}, `"a\u200cb" holds the character U+200C`},
		{"escape character", []rawEntry{file("a\x1b[2J")}, `holds the character U+001B`},
		{"newline", []rawEntry{file("a\nb")}, `holds the character U+000A`},
		{"right-to-left override", []rawEntry{file("a\u202eb")}, `holds the character U+202E`},
		{"first strong isolate", []rawEntry{file("a\u2068b")}, `holds the character U+2068`},
		{"line separator", []rawEntry{file("a\u2028b")}, `holds the character U+2028`},
		{"a name of 256 bytes", []rawEntry{file(strings.Repeat("n", 256))}, `holds a name longer than 255 bytes`},
		{"precomposed then decomposed e acute", []rawEntry{file("caf\u00e9"), file("cafe\u0301")}, "\"caf\u00e9\" and \"cafe\u0301\" are one path under case-insensitive comparison or Unicode normalization"},
		{"decomposed then precomposed e acute", []rawEntry{file("cafe\u0301"), file("caf\u00e9")}, "\"cafe\u0301\" and \"caf\u00e9\" are one path"},
		{"capital E acute against decomposed e acute", []rawEntry{file("CAF\u00c9"), file("cafe\u0301")}, `are one path`},
		{"directories e acute in both forms", []rawEntry{{"40000", "\u00e9", sub(file("x"))}, {"40000", "e\u0301", sub(file("y"))}}, `are one path`},
		{"Kelvin sign", []rawEntry{file("k"), file("\u212a")}, `are one path`},
		{"not UTF-8", []rawEntry{file("a\xff")}, `is not valid UTF-8`},
	}
	r := newRunner(t, Host)
	for _, tt := range tests {
		c := commitTree(t, dir, writeTree(t, dir, tt.entries...))
		got, err := r.WalkTree(t.Context(), dir, c)
		require.Error(t, err, tt.name)
		if !strings.HasPrefix(tt.want, "gitsafe: git") {
			require.ErrorIs(t, err, ErrRefused, tt.name)
		}
		assert.Contains(t, err.Error(), tt.want, tt.name)
		assert.Nil(t, got, tt.name)
	}
}

// TestWalkTreeIgnoresReplaceRefs: a replace ref planted for the tree of
// a commit changes neither what the walk lists nor what fsck checks.
func TestWalkTreeIgnoresReplaceRefs(t *testing.T) {
	dir, _ := newRepo(t)
	blob := writeBlob(t, dir, "x\n")
	good := writeTree(t, dir, rawEntry{"100644", "a", blob})
	hostile := writeTree(t, dir, rawEntry{"120000", "link", blob})
	c := commitTree(t, dir, good)
	plainGit(t, dir, "update-ref", "refs/replace/"+good, hostile)
	assert.Contains(t, plainGit(t, dir, "ls-tree", c), "link", "plain git follows the replace ref")
	r := newRunner(t, Host)
	files, err := r.WalkTree(t.Context(), dir, c)
	require.NoError(t, err)
	assert.Equal(t, []TreeEntry{{"a", modeFile, blob}}, files)
	require.NoError(t, r.Fsck(t.Context(), dir, c))
}

// TestWalkTreeRefuses: only a full object id of a repository root
// reaches the argv, and the argv is ls-tree with the flags of the mode.
func TestWalkTreeRefuses(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	for _, c := range []string{"HEAD", "--all", strings.Repeat("a", 39)} {
		_, err := r.WalkTree(t.Context(), stubRepo, c)
		require.ErrorIs(t, err, ErrRefused, c)
	}
	_, err = r.WalkTree(t.Context(), "relative", strings.Repeat("a", 40))
	require.ErrorIs(t, err, ErrRefused)
	assert.Empty(t, rec.commands)

	oid := strings.Repeat("a", 40)
	got, err := r.WalkTree(t.Context(), stubRepo, oid)
	require.NoError(t, err)
	assert.Empty(t, got)
	require.Len(t, rec.commands, 1)
	assert.Equal(t, []string{"-c", "core.useReplaceRefs=false", "-c", "fsck.badFilemode=error", "ls-tree", "-r", "-t", "-z", "--full-tree", oid}, rec.commands[0].Argv[1+len(hostFlags):])
}

// TestTreeWalkerBounds: the walker refuses an entry longer than a path
// may be, a tree of more entries than the bound, an entry of another
// shape and output that ends inside an entry, and keeps taking what git
// writes after a refusal.
func TestTreeWalkerBounds(t *testing.T) {
	oid := strings.Repeat("b", 40)
	entry := func(path string) string { return "100644 blob " + oid + "\t" + path + "\x00" }

	w := &treeWalker{seen: map[string]string{}}
	n, err := w.Write([]byte(strings.Repeat("x", maxTreePath+200)))
	require.NoError(t, err)
	assert.Equal(t, maxTreePath+200, n)
	require.ErrorIs(t, w.err, ErrRefused)
	first := w.err
	n, err = w.Write([]byte(entry("after")))
	require.NoError(t, err)
	assert.Equal(t, len(entry("after")), n, "the rest is read and dropped")
	assert.Equal(t, first, w.err, "the first refusal stays")

	w = &treeWalker{seen: map[string]string{}}
	_, _ = w.Write([]byte(entry(strings.Repeat("y", maxTreePath+200))))
	require.ErrorIs(t, w.err, ErrRefused, "an entry with its NUL in the same write is bounded too")

	var paths strings.Builder
	name := strings.Repeat("p", 200)
	for i := range maxTreeBytes/(len(name)+8) + 1 {
		paths.WriteString(entry(fmt.Sprintf("%s%08d", name, i)))
	}
	w = &treeWalker{seen: map[string]string{}}
	_, _ = w.Write([]byte(paths.String()))
	require.ErrorIs(t, w.err, ErrRefused)
	assert.Contains(t, w.err.Error(), "the paths of the tree hold more than")

	var b strings.Builder
	for i := range maxTreeEntries + 1 {
		b.WriteString(entry(fmt.Sprintf("f%d", i)))
	}
	w = &treeWalker{seen: map[string]string{}}
	_, _ = w.Write([]byte(b.String()))
	require.ErrorIs(t, w.err, ErrRefused)
	assert.Contains(t, w.err.Error(), "more than 100000 entries")

	for _, bad := range []string{"100644 blob\tx\x00", "100644 blob " + oid + " x\x00", "100644 blob nothex\tx\x00", "100644 blob " + oid + "\t" + strings.Repeat("y", maxTreePath+1) + "\x00"} {
		w = &treeWalker{seen: map[string]string{}}
		_, _ = w.Write([]byte(bad))
		require.ErrorIs(t, w.err, ErrRefused, "%q", bad)
	}

	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	r.start = func(_ context.Context, c procgroup.Command) error {
		_, err := c.Stdout.Write([]byte("100644 blob " + oid + "\tcut"))
		return err
	}
	_, err = r.WalkTree(t.Context(), stubRepo, oid)
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "ends inside an entry")
}

// TestFsck passes a commit whose objects are well formed and fails one
// whose tree fsck --strict refuses, and one with a stored file mode of
// 100664, which fsck alone only warns about and ls-tree lists as 100644.
func TestFsck(t *testing.T) {
	dir, head := newRepo(t)
	blob := writeBlob(t, dir, "x\n")
	r := newRunner(t, Host)
	require.NoError(t, r.Fsck(t.Context(), dir, head))
	bad := commitTree(t, dir, writeTree(t, dir, rawEntry{"100644", ".GIT", blob}))
	require.Error(t, r.Fsck(t.Context(), dir, bad))
	require.Error(t, r.Fsck(t.Context(), dir, head), "fsck checks every object of the repository, the hostile tree included")

	dir, _ = newRepo(t)
	blob = writeBlob(t, dir, "x\n")
	mode := commitTree(t, dir, writeTree(t, dir, rawEntry{"100664", "a", blob}))
	_, err := plainGitErr(t, dir, nil, "fsck", "--strict", "--no-dangling", mode)
	require.NoError(t, err, "plain fsck --strict only warns")
	require.Error(t, r.Fsck(t.Context(), dir, mode))
	files, err := r.WalkTree(t.Context(), dir, mode)
	require.NoError(t, err)
	assert.Equal(t, []TreeEntry{{"a", modeFile, blob}}, files, "the mode git writes the file with")

	for _, c := range []string{"HEAD", "--all"} {
		require.ErrorIs(t, r.Fsck(t.Context(), dir, c), ErrRefused, c)
	}
	require.ErrorIs(t, r.Fsck(t.Context(), "relative", head), ErrRefused)
}

// TestFoldName: the folded name is one value for names that are one
// under case, the characters HFS+ ignores and Unicode normalization, and
// a different value for names that are not.
func TestFoldName(t *testing.T) {
	same := [][2]string{
		{"caf\u00e9", "cafe\u0301"},
		{"CAF\u00c9", "cafe\u0301"},
		{"\u212b", "a\u030a"},              // ANGSTROM SIGN, a singleton decomposition
		{"\u212a", "k"},                    // KELVIN SIGN folds to k
		{"s\u0323\u0307", "S\u0307\u0323"}, // two orders of marks on one base
		{"\u1e9b\u0323", "\u1e69"},         // long s with its marks, folded to s
		{".GIT", ".g\u200cit"},
		{"\u1fb3", "\u1fbc"}, // alpha with U+0345 and its capital variant
	}
	for _, p := range same {
		assert.Equal(t, foldName(p[0]), foldName(p[1]), "%q and %q", p[0], p[1])
	}
	differ := [][2]string{
		{"cafe", "caf\u00e9"},
		{"e\u0301", "e\u0300"},
		{"a", "b"},
		{"\u1fb3", "\u03b1"}, // the mark U+0345 is not dropped
	}
	for _, p := range differ {
		assert.NotEqual(t, foldName(p[0]), foldName(p[1]), "%q and %q", p[0], p[1])
	}
	// No name other than ".git" itself is known to have the NFD form of
	// ".git" once folded (no singleton decomposition gives g, i, t or the
	// dot), so the dot git check holds for them through foldedDotGit.
	assert.Equal(t, foldedDotGit, foldName(".G\u200cIT"))
}

// TestFoldASCIIAllocations: an ASCII name that is already folded
// allocates nothing, and one that changes allocates once.
func TestFoldASCIIAllocations(t *testing.T) {
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = foldName("README.MD") }))
	assert.LessOrEqual(t, testing.AllocsPerRun(100, func() { _ = foldName("readme.md") }), 2.0)
	assert.Equal(t, foldName("Readme.Md"), foldName("rEADME.mD"))
}

// TestUnicodeVersions: the normalization tables and the case folding
// must come from one Unicode version. A bump of either is a reason to
// re-check I27 and the fold of foldName.
func TestUnicodeVersions(t *testing.T) {
	require.Equal(t, unicode.Version, norm.Version, "golang.org/x/text and the standard library disagree on the Unicode version; re-check I27 and foldName")
}
