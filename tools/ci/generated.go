// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
)

// runGenerated is the "generated" subcommand (10 10.2): it runs
// "go generate ./..." with the go command that mise.lock locks and
// fails when that changed, created or removed a file of the working
// tree.
func runGenerated(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generated takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	goBin, err := whichPinned(ctx, root, "go", "go", "go")
	if err != nil {
		return false, err
	}
	goDir := filepath.Dir(goBin)
	gen := step{name: "go generate", argv: []string{goBin, "generate", "./..."}, environ: stepEnv(goDir)}
	repo := e.repo()
	repo.Dir = root
	findings, err := generated(ctx, repo, gen)
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("generated", len(findings)))
	return len(findings) == 0, nil
}

// generated runs gen in the root of repo and compares the working tree
// before and after, by content. The tree is the tracked files and the
// untracked ones that are not ignored, so a changed, a new and a removed
// file each fail, and a tree with uncommitted work can be checked. A
// generated file that exists and is not tracked is the same in both
// snapshots and passes: the workflow's checkout holds no untracked file,
// so it fails there.
func generated(ctx context.Context, repo git.Repo, gen step) ([]finding, error) {
	before, err := snapshot(ctx, repo)
	if err != nil {
		return nil, err
	}
	if out, err := gen.run(ctx, repo.Dir); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", gen.name, err, out)
	}
	after, err := snapshot(ctx, repo)
	if err != nil {
		return nil, err
	}
	var findings []finding
	for _, path := range slices.Sorted(maps.Keys(before)) {
		switch had, has := before[path], after[path]; {
		case has == "":
			findings = append(findings, finding{path, "generated", "go generate removed this file; commit the removal"})
		case has != had:
			findings = append(findings, finding{path, "generated", "go generate changed this file; run it and commit the result"})
		}
	}
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if _, had := before[path]; !had {
			findings = append(findings, finding{path, "generated", "go generate created this file; run it and commit the result"})
		}
	}
	slices.SortStableFunc(findings, func(a, b finding) int { return strings.Compare(a.where, b.where) })
	return findings, nil
}

// snapshot maps each file of the working tree, tracked or untracked and
// not ignored, to a digest of its type, mode and content. A tracked file
// deleted from disk is not in it.
func snapshot(ctx context.Context, repo git.Repo) (map[string]string, error) {
	out, err := repo.Run(ctx, nil, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, name := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(name) == 0 {
			continue
		}
		digest, ok, err := fileDigest(filepath.Join(repo.Dir, string(name)))
		if err != nil {
			return nil, err
		}
		if ok {
			files[string(name)] = digest
		}
	}
	return files, nil
}

// fileDigest returns the digest of the regular file or symbolic link at
// path, and false for anything else, a missing file and a directory (a
// submodule) included.
func fileDigest(path string) (string, bool, error) {
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	h := sha256.New()
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return "", false, err
		}
		_, _ = io.WriteString(h, "link "+target)
	case info.Mode().IsRegular():
		f, err := os.Open(path)
		if err != nil {
			return "", false, err
		}
		defer func() { _ = f.Close() }()
		_, _ = io.WriteString(h, "file "+info.Mode().Perm().String()+" ")
		if _, err := io.Copy(h, f); err != nil {
			return "", false, err
		}
	default:
		return "", false, nil
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}
