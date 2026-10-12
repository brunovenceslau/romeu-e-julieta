// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// snapshotIdentity is the author and committer of a snapshot commit,
// with a fixed date, so the same tree on the same HEAD gives the same
// commit id, and no identity of the machine is needed or written.
var snapshotIdentity = []string{
	"GIT_AUTHOR_NAME=mutants",
	"GIT_AUTHOR_EMAIL=mutants@localhost.invalid",
	"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
	"GIT_COMMITTER_NAME=mutants",
	"GIT_COMMITTER_EMAIL=mutants@localhost.invalid",
	"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
}

// privateIndex are the settings of every git command that reads or
// writes the private index: no split index, whose shared part git would
// write as .git/sharedindex.* in the source repository, no file system
// monitor and no untracked cache, which a configuration could turn on
// and which would keep their state next to the index or in a daemon.
var privateIndex = []string{
	"-c", "core.splitIndex=false",
	"-c", "core.fsmonitor=false",
	"-c", "core.untrackedCache=false",
}

// snapshot records the working tree of source as a commit whose parent
// is head, moves the clone to it, and returns its id. The working tree
// is what "git add --all" stages: the tracked files as they are on
// disk (a file deleted on disk is not in the snapshot), and the
// untracked files that are not ignored. A file that is tracked and also
// matches an ignore rule stays, as the index starts from head.
//
// The real index is never written, nor read past checkIndexFlags: git
// stages into a private index in tmp (GIT_INDEX_FILE), with the
// settings of privateIndex. In the source repository the snapshot
// writes objects only, the blobs, trees and the commit, and no ref, so
// nothing changes there but the object store; the commit is left
// dangling, and "git gc" prunes it once it is older than gc.pruneExpire
// (two weeks by default). When the working tree equals head, there is
// no snapshot commit and head is returned.
//
// The objects reach the clone as a pack, streamed from pack-objects in
// the source to unpack-objects in the clone (sendPack), since a fetch
// would not be served an object that no ref reaches. Then "git
// checkout --detach" moves the clone to the snapshot, with the hooks
// off. A checkout is allowed there, and nowhere else: the clone is a
// repository of its own in tmp, made by this run, checked to be there
// (newRunner), and shares no index, ref, stash or working tree with the
// source repository or any worktree linked to it.
func snapshot(ctx context.Context, source, clone git.Repo, head, tmp string) (string, error) {
	if err := checkIndexFlags(ctx, source); err != nil {
		return "", err
	}
	private := source
	private.Env = slices.Concat(source.Env, []string{"GIT_INDEX_FILE=" + filepath.Join(tmp, "index")}, snapshotIdentity)
	run := func(args ...string) (string, error) {
		out, err := gitLine(ctx, private, slices.Concat(privateIndex, args)...)
		if err != nil {
			return "", fmt.Errorf("snapshot: git %s: %w", args[0], err)
		}
		return out, nil
	}
	if _, err := run("read-tree", head); err != nil {
		return "", err
	}
	if _, err := run("add", "--all"); err != nil {
		return "", err
	}
	tree, err := run("write-tree")
	if err != nil {
		return "", err
	}
	headTree, err := gitLine(ctx, source, "rev-parse", "--verify", head+"^{tree}")
	if err != nil {
		return "", err
	}
	if tree == headTree {
		return head, nil
	}
	commit, err := run("commit-tree", "--no-gpg-sign", "-p", head, "-m", "mutants: a snapshot of the working tree", tree)
	if err != nil {
		return "", err
	}
	if err := sendPack(ctx, source, clone, commit, head); err != nil {
		return "", err
	}
	if _, err := clone.Run(ctx, nil, "-c", noHooks, "checkout", "--quiet", "--detach", commit); err != nil {
		return "", fmt.Errorf("snapshot: git checkout in the clone: %w", err)
	}
	got, err := gitLine(ctx, clone, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", err
	}
	if got != commit {
		return "", fmt.Errorf("the clone holds the commit %s, not the snapshot %s", got, commit)
	}
	return commit, nil
}

// checkIndexFlags refuses a real index with an entry marked
// skip-worktree (a sparse checkout marks every file it leaves out) or
// assume-unchanged: such a file is not on disk as the commit has it, so
// the private index, which starts from head without the marks, would
// record it as deleted or miss its change. "git ls-files -v" tags such
// an entry with S or with a lower-case letter.
func checkIndexFlags(ctx context.Context, source git.Repo) error {
	out, err := source.Run(ctx, nil, "ls-files", "-v", "-z")
	if err != nil {
		return err
	}
	for entry := range strings.SplitSeq(string(out), "\x00") {
		if entry == "" {
			continue
		}
		tag, name, _ := strings.Cut(entry, " ")
		if tag == "S" || (len(tag) == 1 && unicode.IsLower(rune(tag[0]))) {
			return fmt.Errorf("-worktree: %s is marked skip-worktree or assume-unchanged in the index (a sparse checkout, say), so the working tree does not show it as it is; run without -worktree, or clear the mark", git.Printable(name))
		}
	}
	return nil
}

// sendPack streams the objects of want that have does not reach from
// source to clone: pack-objects writes a pack to a pipe that
// unpack-objects reads, so no pack is held in memory or on disk. Both
// run as git.Repo.Run runs git, without replace refs.
func sendPack(ctx context.Context, source, clone git.Repo, want, have string) error {
	pack := exec.CommandContext(ctx, "git", "--no-replace-objects", "pack-objects", "--revs", "--stdout", "--quiet")
	pack.Dir, pack.Env = source.Dir, source.Env
	pack.Stdin = strings.NewReader(want + "\n^" + have + "\n")
	var packErr, unpackErr bytes.Buffer
	pack.Stderr = &packErr
	pipe, err := pack.StdoutPipe()
	if err != nil {
		return err
	}
	unpack := exec.CommandContext(ctx, "git", "--no-replace-objects", "unpack-objects", "-q")
	unpack.Dir, unpack.Env = clone.Dir, clone.Env
	unpack.Stdin = pipe
	unpack.Stderr = &unpackErr
	if err := pack.Start(); err != nil {
		return fmt.Errorf("snapshot: git pack-objects: %w", err)
	}
	unpackRun := unpack.Run()
	if unpackRun != nil {
		_ = pack.Process.Kill()
	}
	packRun := pack.Wait()
	var errs []error
	if packRun != nil {
		errs = append(errs, fmt.Errorf("snapshot: git pack-objects: %w: %s", packRun, git.Printable(strings.TrimSpace(packErr.String()))))
	}
	if unpackRun != nil {
		errs = append(errs, fmt.Errorf("snapshot: git unpack-objects: %w: %s", unpackRun, git.Printable(strings.TrimSpace(unpackErr.String()))))
	}
	return errors.Join(errs...)
}
