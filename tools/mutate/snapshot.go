// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// snapshotIdentity is the author and committer of a snapshot commit,
// with a fixed date, so the same tree on the same HEAD gives the same
// commit id, and no identity of the machine is needed or written.
var snapshotIdentity = []string{
	"GIT_AUTHOR_NAME=mutate",
	"GIT_AUTHOR_EMAIL=mutate@localhost.invalid",
	"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
	"GIT_COMMITTER_NAME=mutate",
	"GIT_COMMITTER_EMAIL=mutate@localhost.invalid",
	"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
}

// snapshot records the working tree of source as a commit whose parent
// is head, moves the clone to it, and returns its id. The working tree
// is what "git add --all" stages: the tracked files as they are on
// disk, and the untracked files that are not ignored. A file that is
// tracked and also matches an ignore rule stays, as the index starts
// from head.
//
// The real index is never read or written: git stages into a private
// index in tmp (GIT_INDEX_FILE). In the source repository the snapshot
// writes objects only, the blobs, trees and the commit, and no ref, so
// nothing changes there but the object store; the commit is left
// dangling, and "git gc" prunes it once it is older than gc.pruneExpire
// (two weeks by default). When the working tree equals head, there is
// no snapshot commit and head is returned.
//
// The objects reach the clone as a pack (rev-list of the snapshot not
// in head, through pack-objects and unpack-objects), since a fetch
// would not be served an object that no ref reaches. Then "git
// checkout --detach" moves the clone to the snapshot. A checkout is
// allowed there, and nowhere else: the clone is a repository of its own
// in tmp, made by this run, checked to be there (newRunner), and shares
// no index, ref, stash or working tree with the source repository or
// any worktree linked to it.
func snapshot(ctx context.Context, source, clone git.Repo, head, tmp string) (string, error) {
	base := source.Env
	if base == nil {
		base = os.Environ()
	}
	private := source
	private.Env = slices.Concat(base, []string{"GIT_INDEX_FILE=" + filepath.Join(tmp, "index")}, snapshotIdentity)
	if _, err := private.Run(ctx, nil, "read-tree", head); err != nil {
		return "", err
	}
	if _, err := private.Run(ctx, nil, "add", "--all"); err != nil {
		return "", err
	}
	tree, err := gitLine(ctx, private, "write-tree")
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
	commit, err := gitLine(ctx, private, "commit-tree", "--no-gpg-sign", "-p", head, "-m", "mutate: a snapshot of the working tree", tree)
	if err != nil {
		return "", err
	}
	pack, err := source.Run(ctx, []byte(commit+"\n^"+head+"\n"), "pack-objects", "--revs", "--stdout", "--quiet")
	if err != nil {
		return "", err
	}
	if _, err := clone.Run(ctx, pack, "unpack-objects", "-q"); err != nil {
		return "", err
	}
	if _, err := clone.Run(ctx, nil, "checkout", "--quiet", "--detach", commit); err != nil {
		return "", err
	}
	if got, err := gitLine(ctx, clone, "rev-parse", "--verify", "HEAD^{commit}"); err != nil {
		return "", err
	} else if got != commit {
		return "", fmt.Errorf("the clone holds the commit %s, not the snapshot %s", got, commit)
	}
	return commit, nil
}
