// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// The verbs of the salvage capture of julieta (08 8.5 step 2): Changes
// and Ignored list what a worktree holds beyond its HEAD, Gitlinks the
// submodules its index records, and CaptureCommit records the working
// tree in a commit through a temporary index, leaving the worktree and
// its index as they were.

// The operations Changes reports in progress, each named by the file or
// directory git keeps in the worktree's git dir while it runs.
const (
	OperationRebase     = "rebase"
	OperationMerge      = "merge"
	OperationCherryPick = "cherry-pick"
	OperationRevert     = "revert"
	OperationBisect     = "bisect"
)

// operationFiles maps the names of the git dir that mark an operation
// in progress to the operation, in the order Changes looks for them.
var operationFiles = []struct{ name, operation string }{
	{"rebase-merge", OperationRebase},
	{"rebase-apply", OperationRebase},
	{"MERGE_HEAD", OperationMerge},
	{"CHERRY_PICK_HEAD", OperationCherryPick},
	{"REVERT_HEAD", OperationRevert},
	{"BISECT_LOG", OperationBisect},
}

// maxCapturePaths bounds each list of Changes, Ignored and Gitlinks: a
// worktree that lists more is refused rather than held in memory. A
// variable, so a test can lower it.
var maxCapturePaths = 1_000_000

// Changes is what a worktree holds beyond its HEAD, as git status
// --porcelain=v2 reads it with every untracked file listed.
type Changes struct {
	// OID is the commit HEAD names; empty on an unborn branch.
	OID string
	// Branch is the full ref name HEAD points at; empty when detached.
	Branch string
	// Operation is the operation in progress in the worktree (rebase,
	// merge, cherry-pick, revert, bisect), or empty.
	Operation string
	// Tracked are the paths whose index entry or working tree file
	// differs from HEAD, a conflict's included, in git's order.
	Tracked []string
	// Added are the paths among Tracked that the index adds, or marks
	// intent-to-add, and HEAD lacks.
	Added []string
	// Untracked are the files that are neither tracked nor ignored, in
	// git's order. A path that ends with a slash is a directory git
	// does not descend into: a nested repository.
	Untracked []string
}

// Changes reads HEAD, the changes and the untracked files of the
// worktree whose root is dir, and the operation in progress there,
// under the flags of the mode and with no filter (as Status runs git
// status), with git rev-parse --absolute-git-dir and git status
// --porcelain=v2 -z --untracked-files=all --no-renames. It writes
// nothing: no refresh of the index, no index.lock. A path is repository
// content: it is kept as git gives it and escaped where it is shown.
func (r *Runner) Changes(ctx context.Context, dir string) (Changes, error) {
	if err := checkRepo(dir); err != nil {
		return Changes{}, err
	}
	gitDir, err := r.readLine(ctx, dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return Changes{}, err
	}
	if !filepath.IsAbs(gitDir) {
		return Changes{}, fmt.Errorf("%w: rev-parse named the git dir %q", ErrRefused, termsafe.Escape(gitDir))
	}
	noFilters, err := r.filterOverrides(ctx, dir)
	if err != nil {
		return Changes{}, err
	}
	var c Changes
	sawOID := false
	keep := func(list *[]string, path string) error {
		if len(*list) == maxCapturePaths {
			return fmt.Errorf("%w: more than %d paths of a kind", ErrRefused, maxCapturePaths)
		}
		*list = append(*list, path)
		return nil
	}
	pathOf := func(rec string, fields int) (string, error) {
		parts := strings.SplitN(rec, " ", fields+1)
		if len(parts) != fields+1 || parts[fields] == "" {
			return "", fmt.Errorf("%w: git status record %q", ErrRefused, termsafe.Escape(rec))
		}
		return parts[fields], nil
	}
	w := &records{limit: maxStatusRecord, each: func(rec string) error {
		switch {
		case strings.HasPrefix(rec, "# branch.oid "):
			sawOID = true
			if oid := strings.TrimPrefix(rec, "# branch.oid "); oid != "(initial)" {
				if err := checkObjectID(oid); err != nil {
					return err
				}
				c.OID = oid
			}
		case strings.HasPrefix(rec, "# branch.head "):
			if head := strings.TrimPrefix(rec, "# branch.head "); head != "(detached)" {
				c.Branch = "refs/heads/" + head
			}
		case strings.HasPrefix(rec, "# "):
		case strings.HasPrefix(rec, "1 ") && len(rec) > 4:
			path, err := pathOf(rec, ordinaryFields)
			if err != nil {
				return err
			}
			// Added in the index (A.) or intent-to-add (.A): HEAD lacks
			// the path, so the capture adds it by name.
			if rec[2] == 'A' || rec[3] == 'A' {
				if err := keep(&c.Added, path); err != nil {
					return err
				}
			}
			return keep(&c.Tracked, path)
		case strings.HasPrefix(rec, "u "):
			path, err := pathOf(rec, unmergedFields)
			if err != nil {
				return err
			}
			return keep(&c.Tracked, path)
		case strings.HasPrefix(rec, "? "):
			path, err := pathOf(rec, 1)
			if err != nil {
				return err
			}
			return keep(&c.Untracked, path)
		default:
			return fmt.Errorf("%w: git status record %q", ErrRefused, termsafe.Escape(rec))
		}
		return nil
	}}
	if err := r.read(ctx, dir, noFilters, w, "status", "--porcelain=v2", "-z", "--branch", "--no-ahead-behind",
		"--untracked-files=all", "--ignore-submodules=dirty", "--no-renames"); err != nil {
		return Changes{}, err
	}
	if err := w.close(); err != nil {
		return Changes{}, err
	}
	if !sawOID {
		return Changes{}, fmt.Errorf("%w: git status named no branch.oid", ErrRefused)
	}
	if c.Branch != "" {
		if err := checkRefName(c.Branch, false); err != nil {
			return Changes{}, err
		}
	}
	for _, f := range operationFiles {
		if _, err := os.Lstat(filepath.Join(gitDir, f.name)); err == nil {
			c.Operation = f.operation
			break
		} else if !errors.Is(err, fs.ErrNotExist) {
			return Changes{}, err
		}
	}
	return c, nil
}

// Ignored lists the ignored files of the worktree whose root is dir, as
// git ls-files --others --ignored --exclude-standard --directory -z
// lists them under the flags of the mode: a directory all of whose
// files are ignored is one path ending with a slash. It writes nothing.
func (r *Runner) Ignored(ctx context.Context, dir string) ([]string, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	var paths []string
	w := &records{limit: maxStatusRecord, each: func(rec string) error {
		if rec == "" {
			return fmt.Errorf("%w: an empty ls-files record", ErrRefused)
		}
		if len(paths) == maxCapturePaths {
			return fmt.Errorf("%w: more than %d ignored paths", ErrRefused, maxCapturePaths)
		}
		paths = append(paths, rec)
		return nil
	}}
	if err := r.read(ctx, dir, nil, w, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return paths, nil
}

// gitlinkMode is the mode of an index entry that records a submodule's
// commit.
const gitlinkMode = "160000"

// Gitlinks lists the paths the index of the worktree whose root is dir
// records as submodules (mode 160000), with git ls-files --stage -z
// under the flags of the mode. It writes nothing.
func (r *Runner) Gitlinks(ctx context.Context, dir string) ([]string, error) {
	if err := checkRepo(dir); err != nil {
		return nil, err
	}
	var paths []string
	seen := map[string]bool{}
	w := &records{limit: maxStatusRecord, each: func(rec string) error {
		// "<mode> <object> <stage>\t<path>"
		meta, path, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || path == "" || len(fields) != 3 {
			return fmt.Errorf("%w: ls-files record %q", ErrRefused, termsafe.Escape(rec))
		}
		if fields[0] != gitlinkMode || seen[path] {
			return nil
		}
		seen[path] = true
		if len(paths) == maxCapturePaths {
			return fmt.Errorf("%w: more than %d submodules", ErrRefused, maxCapturePaths)
		}
		paths = append(paths, path)
		return nil
	}}
	if err := r.read(ctx, dir, nil, w, "ls-files", "--stage", "-z"); err != nil {
		return nil, err
	}
	if err := w.close(); err != nil {
		return nil, err
	}
	return paths, nil
}

// Capture is one capture commit of a worktree (CaptureCommit).
type Capture struct {
	// Index is the temporary index file the capture builds, an
	// absolute and clean path outside the worktree that does not exist
	// yet; the caller removes it, since gitsafe, which romeu links,
	// deletes nothing outside the admitted call sites (I16).
	Index string
	// Parent is the commit HEAD names, or empty on an unborn branch.
	Parent string
	// Add are the paths the capture adds beyond the tracked files:
	// the files the index adds and the untracked files it keeps, each a
	// literal, clean, relative path of the worktree.
	Add []string
	// Name and Email are the author and committer of the commit.
	Name, Email string
	// Message is the commit message.
	Message string
	// When is the author and committer date.
	When time.Time
}

// captureSettings are the extra settings of CaptureCommit: no signature,
// whatever commit.gpgsign the repository sets (I18), no automatic gc or
// maintenance, which the objects it writes could start, and no replace
// ref.
var captureSettings = []setting{
	{"commit.gpgsign", "false"},
	{"gc.auto", "0"},
	{"maintenance.auto", "false"},
	{"core.useReplaceRefs", "false"},
}

// CaptureCommit records the working tree of the worktree whose root is
// dir in a commit whose parent is c.Parent (08 8.5 step 2, REQ-0700),
// and returns its id; empty when the working tree holds nothing beyond
// c.Parent, so no commit is made. It builds the tree in the temporary
// index c.Index (GIT_INDEX_FILE): git read-tree of the parent, git add
// --update for every tracked file, git add --force of c.Add, git
// write-tree; then git commit-tree --no-gpg-sign with the author and
// date of c. The worktree, its index and its refs are left as they
// were; the objects are written into the repository. No filter runs,
// as in Status, and the paths of c.Add are literal pathspecs.
//
//romeu:invariant I18 guard
func (r *Runner) CaptureCommit(ctx context.Context, dir string, c Capture) (string, error) {
	if err := checkRepo(dir); err != nil {
		return "", err
	}
	if err := c.check(); err != nil {
		return "", err
	}
	if _, err := os.Lstat(c.Index); err == nil {
		return "", fmt.Errorf("%w: the index file %s exists", ErrRefused, c.Index)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if c.Parent == "" && len(c.Add) == 0 {
		return "", nil
	}
	noFilters, err := r.filterOverrides(ctx, dir)
	if err != nil {
		return "", err
	}
	env := slices.Concat(noFilters, []string{"GIT_INDEX_FILE=" + c.Index, "GIT_LITERAL_PATHSPECS=1"})
	git := func(stdin string, args ...string) (string, error) {
		out := limitedBuffer{limit: maxLineBytes}
		if err := r.runInput(ctx, dir, captureSettings, env, strings.NewReader(stdin), &out, args...); err != nil {
			return "", err
		}
		if out.truncated {
			return "", fmt.Errorf("%w: git %s wrote more than %d bytes", ErrRefused, args[0], out.limit)
		}
		return strings.TrimSuffix(out.String(), "\n"), nil
	}
	if c.Parent == "" {
		_, err = git("", "read-tree", "--empty")
	} else {
		_, err = git("", "read-tree", c.Parent)
	}
	if err != nil {
		return "", err
	}
	if c.Parent != "" {
		if _, err := git("", "add", "--update"); err != nil {
			return "", err
		}
	}
	if len(c.Add) > 0 {
		list := strings.Join(c.Add, "\x00") + "\x00"
		if _, err := git(list, "add", "--force", "--pathspec-from-file=-", "--pathspec-file-nul"); err != nil {
			return "", err
		}
	}
	// A filter driver the config gained since the listing above could
	// have run during the adds: refuse the capture (05 5.1).
	if again, err := r.filterOverrides(ctx, dir); err != nil {
		return "", err
	} else if !slices.Equal(again, noFilters) {
		return "", fmt.Errorf("%w: the filter drivers of the config changed during the capture", ErrRefused)
	}
	tree, err := git("", "write-tree")
	if err != nil {
		return "", err
	}
	if err := checkObjectID(tree); err != nil {
		return "", err
	}
	args := []string{"commit-tree", "--no-gpg-sign", "-m", c.Message}
	if c.Parent != "" {
		parentTree, err := git("", "rev-parse", "--verify", "--end-of-options", c.Parent+"^{tree}")
		if err != nil {
			return "", err
		}
		if parentTree == tree {
			return "", nil
		}
		args = append(args, "-p", c.Parent)
	}
	date := "@" + strconv.FormatInt(c.When.Unix(), 10) + " +0000"
	env = append(env,
		"GIT_AUTHOR_NAME="+c.Name, "GIT_AUTHOR_EMAIL="+c.Email, "GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_NAME="+c.Name, "GIT_COMMITTER_EMAIL="+c.Email, "GIT_COMMITTER_DATE="+date)
	oid, err := git("", append(args, tree)...)
	if err != nil {
		return "", err
	}
	if err := checkObjectID(oid); err != nil {
		return "", err
	}
	return oid, nil
}

// check returns ErrRefused for a capture CaptureCommit does not run.
func (c Capture) check() error {
	if !filepath.IsAbs(c.Index) || filepath.Clean(c.Index) != c.Index || strings.IndexFunc(c.Index, isControl) >= 0 {
		return fmt.Errorf("%w: the index file %q is not an absolute, clean path", ErrRefused, c.Index)
	}
	if c.Parent != "" {
		if err := checkObjectID(c.Parent); err != nil {
			return err
		}
	}
	for _, p := range c.Add {
		if err := checkCapturePath(p); err != nil {
			return err
		}
	}
	for _, v := range []string{c.Name, c.Email} {
		if v == "" || strings.ContainsAny(v, "<>\n") || strings.IndexFunc(v, isControl) >= 0 {
			return fmt.Errorf("%w: the identity %q", ErrRefused, v)
		}
	}
	if c.Message == "" || c.When.IsZero() {
		return fmt.Errorf("%w: a capture needs a message and a date", ErrRefused)
	}
	return nil
}

// checkCapturePath refuses a path that is not a clean, relative path of
// a worktree: empty, absolute, with an empty, "." or ".." element, or a
// NUL, which would split the pathspec list. A glob character is
// allowed, since the pathspecs are literal.
func checkCapturePath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		return fmt.Errorf("%w: %q is not a path of a worktree", ErrRefused, termsafe.Escape(p))
	}
	for part := range strings.SplitSeq(p, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: %q is not a clean relative path", ErrRefused, termsafe.Escape(p))
		}
	}
	return nil
}
