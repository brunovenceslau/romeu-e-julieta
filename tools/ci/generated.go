// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"testing/fstest"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// generated is the generated step of 10 10.2. It judges one commit,
// named by its id: it reads the sources from the tree of that commit,
// runs the generators of generateFiles over them in this process, and
// compares what they write with each generated file of the same tree.
// A file the commit does not hold, holds with another mode than a
// regular, non-executable file (a symbolic link among them) or holds
// with other bytes is a finding. So a hand edit of a generated file
// fails, and so does a source changed without its consumers, a
// generated file deleted or ignored, and a record committed without
// the index or the index without the record.
//
// It reads the commit from the object store, never the working tree
// or the index. fast and all read it (commitChecks) before they start
// mise or any step: nothing a step writes to the working tree, the
// index, HEAD or the object store changes what it judges. git and the
// object store are trusted up to that read, which in the hosted
// workflow comes after mise has run (misefiles.go). The step runs
// no go:generate line either. It does not judge the generators
// themselves: a pull request that changes tools/ci runs its own
// generators and its own checks, which is why tools/ci/** is on the
// checks surface of the ask-first list (05 5.3).
func generated(ctx context.Context, repo git.Repo, commit string) ([]finding, error) {
	files, err := headTree(ctx, repo, commit)
	if err != nil {
		return nil, err
	}
	return generatedOf(files)
}

// generatedOf judges the generated files of files, the tree of one
// commit as headTree reads it.
func generatedOf(files []treeFile) ([]finding, error) {
	tree, err := commitTree(files)
	if err != nil {
		return nil, err
	}
	generated, err := generateFiles(tree)
	if err != nil {
		return nil, err
	}
	var findings []finding
	for _, f := range generated {
		if msg := judgeGenerated(tree, f); msg != "" {
			findings = append(findings, finding{f.path, "generated", msg})
		}
	}
	return findings, nil
}

// regenerate is the advice every finding of generated ends with. The
// step reads the commit, so a file regenerated and not yet committed
// still fails until it is committed.
const regenerate = `run "go generate ./...", review the result and commit it; this step reads the commit, not the working tree`

// regularMode is the git mode of a generated file: a regular file that
// is not executable.
const regularMode = "100644"

// judgeGenerated returns what is wrong with the generated file f in the
// tree of the judged commit, or "" when the tree holds f.data at f.path
// as a regular file.
func judgeGenerated(tree fstest.MapFS, f generatedFile) string {
	entry := tree[f.path]
	switch {
	case entry == nil:
		return "the commit does not hold the file; " + regenerate
	case entry.Sys != regularMode:
		return fmt.Sprintf("the commit holds the file with mode %v, and a generated file is a regular file of mode %s; ", entry.Sys, regularMode) + regenerate
	case !bytes.Equal(entry.Data, f.data):
		return "the file differs from what its generator writes from the sources in the commit; edit the source, not this file, then " + regenerate
	}
	return ""
}

// commitTree returns files, the tree of one commit, as a file system
// whose root is the top of the repository. Each entry holds the content
// git stores and the type its mode gives: a regular file (with the
// execute bit when git records one), a directory for a submodule, and
// for a symbolic link an irregular file whose content is its target.
// Sys holds the git mode itself, as ls-tree prints it. The file system
// is a testing/fstest.MapFS, the standard library's file system in
// memory. It follows only entries of type fs.ModeSymlink, and without
// a depth limit (resolveSymlinks in testing/fstest/mapfs.go, Go 1.27),
// so a link is never of that type here: a link to itself would
// otherwise exhaust the stack, and no link is followed at all.
func commitTree(files []treeFile) (fstest.MapFS, error) {
	tree := fstest.MapFS{}
	for _, f := range files {
		var mode fs.FileMode
		switch f.mode {
		case "100644":
			mode = 0o644
		case "100755":
			mode = 0o755
		case "120000":
			mode = fs.ModeIrregular | 0o777
		case "160000":
			mode = fs.ModeDir | 0o755
		default:
			return nil, fmt.Errorf("read tree: %s has the unknown mode %s", git.Printable(f.path), f.mode)
		}
		tree[f.path] = &fstest.MapFile{Data: f.content, Mode: mode, Sys: f.mode}
	}
	return tree, nil
}

// runGenerated runs the generated step on the commit HEAD names in the
// repository the command starts in, and prints what it finds.
func runGenerated(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generated takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	repo := e.repo()
	repo.Dir = root
	head, err := commitOf(ctx, repo, "HEAD")
	if err != nil {
		return false, err
	}
	findings, err := generated(ctx, repo, head)
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("generated", len(findings)))
	return len(findings) == 0, nil
}
