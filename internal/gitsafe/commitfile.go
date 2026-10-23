// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrFileRefused is wrapped, with ErrRefused, by CommitFile for a refusal
// of the file itself (not a regular file, over the limit), as opposed to
// one of its arguments (the repository, the commit, the path or the
// limit). A caller that reads a file a repo may or may not shape as
// expected tells the two apart with it.
var ErrFileRefused = errors.New("gitsafe: the file is refused")

// fileRefused is the error of a file CommitFile refuses.
func fileRefused(format string, args ...any) error {
	return fmt.Errorf("%w: %w: %s", ErrRefused, ErrFileRefused, fmt.Sprintf(format, args...))
}

// CommitFile returns the bytes of the regular file name in the tree of
// commit in the repository in dir, and whether the commit holds one: a
// name the commit lacks gives no file and no error. It is what egress
// reads of a repo at its egressCommit (07 7.5, step 1), under the flags
// of the mode, with one git ls-tree and one git cat-file blob. name is
// a literal path from the root of the tree, taken as IndexFile takes
// it; commit is a full object id. A file that is a symbolic link, a
// gitlink or a directory is refused, and so is one of more than limit
// bytes, which is refused by the size ls-tree reports, before git reads
// the blob; each is ErrRefused, and ErrFileRefused as well.
func (r *Runner) CommitFile(ctx context.Context, dir, commit, name string, limit int) ([]byte, bool, error) {
	if err := checkRepo(dir); err != nil {
		return nil, false, err
	}
	if err := checkObjectID(commit); err != nil {
		return nil, false, err
	}
	if limit < 0 {
		return nil, false, fmt.Errorf("%w: a negative limit", ErrRefused)
	}
	if err := checkLiteralPath(name); err != nil {
		return nil, false, err
	}
	if _, err := checkTreePath(name); err != nil {
		return nil, false, err
	}
	var entries []string
	w := &records{limit: maxTreePath + 128, each: func(rec string) error {
		entries = append(entries, rec)
		return nil
	}}
	if err := r.runInput(ctx, dir, historySettings, historyEnv, nil, w, "ls-tree", "-l", "-z", "--full-tree", "--end-of-options", commit, "--", ":(literal)"+name); err != nil {
		return nil, false, err
	}
	if err := w.close(); err != nil {
		return nil, false, err
	}
	var oid string
	for _, rec := range entries {
		meta, path, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 4 || path != name {
			return nil, false, fmt.Errorf("%w: ls-tree entry %q", ErrRefused, rec)
		}
		if (f[0] != modeFile && f[0] != modeExecutable) || f[1] != "blob" {
			return nil, false, fileRefused("%s is not a regular file in the commit (mode %s %s)", name, f[0], f[1])
		}
		if err := checkObjectID(f[2]); err != nil {
			return nil, false, err
		}
		size, err := strconv.Atoi(f[3])
		if err != nil || size < 0 {
			return nil, false, fileRefused("%s has the size %q", name, f[3])
		}
		if size > limit {
			return nil, false, fileRefused("%s is more than %d bytes in the commit", name, limit)
		}
		oid = f[2]
	}
	if len(entries) > 1 {
		return nil, false, fmt.Errorf("%w: ls-tree answered %d entries for %s", ErrRefused, len(entries), name)
	}
	if oid == "" {
		return nil, false, nil
	}
	out := limitedBuffer{limit: limit + 1}
	if err := r.runInput(ctx, dir, historySettings, historyEnv, nil, &out, "cat-file", "blob", oid); err != nil {
		return nil, false, err
	}
	if out.truncated || out.buf.Len() > limit {
		return nil, false, fileRefused("%s is more than %d bytes in the commit", name, limit)
	}
	return slices.Clone(out.buf.Bytes()), true, nil
}
