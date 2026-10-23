// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Staged returns which of the names the index of the repository in dir
// changes against HEAD: a path added, modified or deleted there, in the
// order git lists them, which is by path
// (the pre-commit action of julieta asks whether a mise file is staged,
// 04 4.3, REQ-0408). It runs git diff --cached --name-only under the
// flags of the mode, on an index that may belong to a repository with no
// commit yet. indexFile is the index to read, the GIT_INDEX_FILE a
// commit that stages for itself (git commit -a, git commit <path>) hands
// its hook, an absolute and clean path the caller has checked to lie in
// the git dir; "" is the repository's own index. Each name is a literal
// path of the work tree, relative and clean: an empty name, an absolute
// one, one with a ".." element, with a glob character, a leading colon or
// a control character is refused (ErrRefused), and so is a call with no
// name, so no pathspec magic reaches git.
func (r *Runner) Staged(ctx context.Context, dir, indexFile string, names ...string) ([]string, error) {
	env, err := indexEnv(dir, indexFile, names)
	if err != nil {
		return nil, err
	}
	var staged []string
	w := &records{limit: maxStatusRecord, each: func(path string) error {
		if slices.Contains(names, path) {
			staged = append(staged, path)
		}
		return nil
	}}
	args := append([]string{"diff", "--cached", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--"}, names...)
	if err := r.read(ctx, dir, env, w, args...); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return staged, nil
}

// IndexFile returns the content the index of the repository in dir holds
// for the path name, and whether it holds one: what a commit would
// record, which for git commit -a and git commit <path> is in the index
// the hook is given (indexFile, as Staged says). The entry must be a
// regular file at stage 0, a link, a submodule or an unmerged entry is
// refused (ErrRefused), and a blob over limit bytes is refused, so the
// read is bounded. It runs git ls-files -s and git cat-file blob under
// the flags of the mode.
func (r *Runner) IndexFile(ctx context.Context, dir, indexFile, name string, limit int) ([]byte, bool, error) {
	env, err := indexEnv(dir, indexFile, []string{name})
	if err != nil {
		return nil, false, err
	}
	var entries []string
	w := &records{limit: maxStatusRecord, each: func(rec string) error {
		entries = append(entries, rec)
		return nil
	}}
	if err := r.read(ctx, dir, env, w, "ls-files", "-s", "-z", "--", name); err != nil {
		return nil, false, err
	}
	if err := w.close(); err != nil {
		return nil, false, err
	}
	var oid string
	for _, rec := range entries {
		meta, path, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 || path != name {
			return nil, false, fmt.Errorf("%w: ls-files entry %q", ErrRefused, rec)
		}
		if f[2] != "0" {
			return nil, false, fmt.Errorf("%w: %s is unmerged in the index", ErrRefused, name)
		}
		if f[0] != "100644" && f[0] != "100755" {
			return nil, false, fmt.Errorf("%w: %s is not a regular file in the index (mode %s)", ErrRefused, name, f[0])
		}
		if err := checkObjectID(f[1]); err != nil {
			return nil, false, err
		}
		oid = f[1]
	}
	if oid == "" {
		return nil, false, nil
	}
	out := limitedBuffer{limit: limit + 1}
	if err := r.read(ctx, dir, env, &out, "cat-file", "blob", oid); err != nil {
		return nil, false, err
	}
	if out.truncated || out.buf.Len() > limit {
		return nil, false, fmt.Errorf("%w: %s is more than %d bytes in the index", ErrRefused, name, limit)
	}
	return slices.Clone(out.buf.Bytes()), true, nil
}

// indexEnv checks the repository, the names and the index file, and
// returns the environment that points git at the index file, none for
// the repository's own.
func indexEnv(dir, indexFile string, names []string) ([]string, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: no path to look for in the index", ErrRefused)
	}
	for _, name := range names {
		if err := checkLiteralPath(name); err != nil {
			return nil, err
		}
	}
	if indexFile == "" {
		return nil, nil
	}
	if !filepath.IsAbs(indexFile) || filepath.Clean(indexFile) != indexFile || strings.IndexFunc(indexFile, isControl) >= 0 {
		return nil, fmt.Errorf("%w: the index file %q is not an absolute, clean path", ErrRefused, indexFile)
	}
	return []string{"GIT_INDEX_FILE=" + indexFile}, nil
}

// checkLiteralPath refuses a name that is not a literal, relative, clean
// path of a work tree.
func checkLiteralPath(name string) error {
	switch {
	case name == "", strings.HasPrefix(name, "/"), strings.HasPrefix(name, ":"),
		strings.ContainsAny(name, "*?[\\"), strings.IndexFunc(name, isControl) >= 0:
		return fmt.Errorf("%w: %q is not a literal path of a work tree", ErrRefused, name)
	}
	for part := range strings.SplitSeq(name, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: %q is not a clean relative path", ErrRefused, name)
		}
	}
	return nil
}
