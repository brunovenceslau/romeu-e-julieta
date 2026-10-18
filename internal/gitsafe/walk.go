// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The modes a materialized tree may hold (I27): a regular file, an
// executable file and a directory. A symlink (120000) and a gitlink
// (160000) are refused, and so is any other mode. git itself reads any
// mode of a regular file as 100644 or 100755 (canon_mode), so a stored
// 100664 lists as 100644, the mode the file is written with; Fsck fails
// on the stored one.
const (
	modeFile       = "100644"
	modeExecutable = "100755"
	modeTree       = "040000"
)

// The bounds of a walk, each far above a kit's: the entries of the
// tree, directories included; the length of one path and of one name,
// the name being the limit of the file systems of both hosts (APFS and
// ext4); and the bytes of all the paths together, which bounds what the
// walk holds in memory.
const (
	maxTreeEntries = 100_000
	maxTreePath    = 4096
	maxTreeName    = 255
	maxTreeBytes   = 16 << 20
)

// TreeEntry is one file of a tree WalkTree accepted: its path from the
// root of the tree, with / between the names, its mode, modeFile or
// modeExecutable, and its blob.
type TreeEntry struct {
	Path, Mode, OID string
}

// WalkTree lists the tree of commit in the repository in dir with git
// ls-tree, under the flags of the mode, and checks it by the rules of
// I27 in Go, without relying on fsck (05 5.1, REQ-0465): only the modes
// 100644, 100755 and 040000; no name ".", ".." or ".git", whatever its
// case and with any character HFS+ ignores, such as U+200C, taken out;
// and no two paths that are one under case-insensitive comparison. It
// also refuses a name that holds a control character, a character that
// does not show or one that turns the direction of text, which no kit
// needs and a listing could be spoofed with. It returns the files in the
// order git lists them, or ErrRefused, wrapped with the first rule the
// tree breaks; a git that fails, as ls-tree does on an empty name,
// gives its own error. Two names that are one only under Unicode
// normalization (NFC and NFD), which APFS also treats as one, are not
// caught here: that needs golang.org/x/text, which the dependency list
// of 12 12.1 (REQ-1049) does not hold. Neither are the NTFS names fsck
// refuses (".git.", "git~1"), which are plain names on both hosts. commit is a full object id.
//
//romeu:invariant I27 guard
func (r *Runner) WalkTree(ctx context.Context, dir, commit string) ([]TreeEntry, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	if err := checkObjectID(commit); err != nil {
		return nil, err
	}
	w := &treeWalker{seen: map[string]string{}}
	if err := r.runWith(ctx, dir, readSettings, w, "ls-tree", "-r", "-t", "-z", "--full-tree", commit); err != nil {
		return nil, err
	}
	if w.err == nil && w.partial.Len() > 0 {
		w.err = fmt.Errorf("%w: ls-tree output ends inside an entry", ErrRefused)
	}
	if w.err != nil {
		return nil, w.err
	}
	return w.files, nil
}

// treeWalker reads the NUL-terminated entries of ls-tree -z as git
// writes them and checks each one. After the first refusal it keeps
// reading and drops the rest, so git never blocks on a full pipe.
type treeWalker struct {
	partial bytes.Buffer
	entries int
	bytes   int
	files   []TreeEntry
	seen    map[string]string // folded path -> the path that holds it
	err     error
}

// Write takes the next part of the output.
func (w *treeWalker) Write(p []byte) (int, error) {
	if w.err != nil {
		return len(p), nil
	}
	rest := p
	for len(rest) > 0 {
		i := bytes.IndexByte(rest, 0)
		end := i
		if i < 0 {
			end = len(rest)
		}
		if w.partial.Len()+end > maxTreePath+128 {
			w.err = fmt.Errorf("%w: a tree entry longer than %d bytes", ErrRefused, maxTreePath)
			return len(p), nil
		}
		if i < 0 {
			w.partial.Write(rest)
			break
		}
		w.partial.Write(rest[:i])
		rest = rest[i+1:]
		w.err = w.entry(w.partial.String())
		w.partial.Reset()
		if w.err != nil {
			break
		}
	}
	return len(p), nil
}

// entry checks one "<mode> <type> <oid>\t<path>" entry.
func (w *treeWalker) entry(line string) error {
	w.entries++
	if w.entries > maxTreeEntries {
		return fmt.Errorf("%w: the tree holds more than %d entries", ErrRefused, maxTreeEntries)
	}
	meta, path, ok := strings.Cut(line, "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 3 {
		return fmt.Errorf("%w: ls-tree entry %q", ErrRefused, line)
	}
	mode, kind, oid := fields[0], fields[1], fields[2]
	if err := checkObjectID(oid); err != nil {
		return err
	}
	switch {
	case (mode == modeFile || mode == modeExecutable) && kind == "blob":
	case mode == modeTree && kind == "tree":
	case mode == "120000":
		return fmt.Errorf("%w: %q is a symbolic link", ErrRefused, path)
	case mode == "160000":
		return fmt.Errorf("%w: %q is a gitlink", ErrRefused, path)
	default:
		return fmt.Errorf("%w: %q has the mode %s %s", ErrRefused, path, mode, kind)
	}
	if err := checkTreePath(path); err != nil {
		return err
	}
	w.bytes += len(path)
	if w.bytes > maxTreeBytes {
		return fmt.Errorf("%w: the paths of the tree hold more than %d bytes", ErrRefused, maxTreeBytes)
	}
	key := foldPath(path)
	if other, dup := w.seen[key]; dup {
		return fmt.Errorf("%w: %q and %q are one path under case-insensitive comparison", ErrRefused, other, path)
	}
	w.seen[key] = path
	if kind == "blob" {
		w.files = append(w.files, TreeEntry{Path: path, Mode: mode, OID: oid})
	}
	return nil
}

// checkTreePath refuses a path that is too long, is not UTF-8, or holds
// an empty name or a name that is ".", ".." or ".git" once folded
// (foldName).
func checkTreePath(path string) error {
	if len(path) > maxTreePath {
		return fmt.Errorf("%w: a path longer than %d bytes", ErrRefused, maxTreePath)
	}
	if !utf8.ValidString(path) {
		return fmt.Errorf("%w: %q is not valid UTF-8", ErrRefused, path)
	}
	for name := range strings.SplitSeq(path, "/") {
		switch foldName(name) {
		case "":
			return fmt.Errorf("%w: %q holds an empty name", ErrRefused, path)
		case ".", "..", foldedDotGit:
			return fmt.Errorf("%w: %q holds the name %q", ErrRefused, path, name)
		}
		if len(name) > maxTreeName {
			return fmt.Errorf("%w: %q holds a name longer than %d bytes", ErrRefused, path, maxTreeName)
		}
		if i := strings.IndexFunc(name, unsafeRune); i >= 0 {
			r, _ := utf8.DecodeRuneInString(name[i:])
			return fmt.Errorf("%w: %q holds the character %U", ErrRefused, path, r)
		}
	}
	return nil
}

// unsafeRune reports whether r may not stand in a name of a tree: a
// control character, a character HFS+ ignores, a format character (Cf,
// which holds the direction overrides and isolates, U+202A to U+202E and
// U+2066 to U+2069) and the line and paragraph separators.
func unsafeRune(r rune) bool {
	return unicode.IsControl(r) || hfsIgnorable(r) || unicode.Is(unicode.Cf, r) || r == 0x2028 || r == 0x2029
}

// foldedDotGit is ".git" folded as every name is (foldName).
var foldedDotGit = foldName(".git")

// hfsIgnorable reports whether HFS+ ignores r in a file name, the list
// git itself checks (next_hfs_char in utf8.c): the zero width joiners,
// the direction marks and embeddings, the deprecated format characters
// U+206A to U+206F and the zero width no-break space.
func hfsIgnorable(r rune) bool {
	return r == 0x200c || r == 0x200d || r == 0x200e || r == 0x200f ||
		(r >= 0x202a && r <= 0x202e) || (r >= 0x206a && r <= 0x206f) || r == 0xfeff
}

// foldName returns name with every character HFS+ ignores taken out and
// each letter folded to one case, so ".git", ".GIT" and ".g\u200cit"
// all give one value.
func foldName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if hfsIgnorable(r) {
			continue
		}
		b.WriteRune(foldRune(r))
	}
	return b.String()
}

// foldPath folds each name of path (foldName).
func foldPath(path string) string {
	names := strings.Split(path, "/")
	for i, n := range names {
		names[i] = foldName(n)
	}
	return strings.Join(names, "/")
}

// foldRune returns the smallest rune of the case-folding orbit of r, one
// value for every case of a letter (unicode.SimpleFold).
func foldRune(r rune) rune {
	least := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		least = min(least, f)
	}
	return least
}
