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

// runGenerated is the "generated" subcommand (10 10.2): it first checks
// that the tree holds no go:generate line but the one of the repository
// (checkGenerateDirectives), then runs
// "go generate ./..." with the go command that mise.lock locks and
// fails when that changed, created or removed a file of the working
// tree, or when a file that generate writes is not tracked as a regular
// file with the bytes that generate gives.
func runGenerated(ctx context.Context, e env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("generated takes no argument\n%s", usage)
	}
	root, err := repoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	// Before anything runs: go generate runs every directive it finds, so
	// the tree must hold the one directive of the repository first.
	problems, err := checkGenerateDirectives(root)
	if err != nil {
		return false, err
	}
	if len(problems) != 0 {
		say(e.stdout, "%s%s\n", lines(problems), summary("generated", len(problems)))
		return false, nil
	}
	goBin, err := whichPinned(ctx, root, "go", "go", "go")
	if err != nil {
		return false, err
	}
	goDir := filepath.Dir(goBin)
	gen := step{name: "go generate", argv: []string{goBin, "generate", "./..."}, environ: stepEnv(goDir)}
	repo := e.repo()
	repo.Dir = root
	findings, err := generated(ctx, repo, gen, generateOutputs)
	if err != nil {
		return false, err
	}
	say(e.stdout, "%s%s\n", lines(findings), summary("generated", len(findings)))
	return len(findings) == 0, nil
}

// generated runs gen in the root of repo and compares the working tree
// before and after, by content. The tree is the tracked files and the
// untracked ones that are not ignored, so a changed, a new and a removed
// file each fail, and a tree with uncommitted work can be checked.
//
// That comparison cannot see a generated file that is the same before
// and after: one that is ignored, untracked, or a symbolic link to a copy
// of the right bytes. The workflow's checkout would hold something else,
// so wanted, when it is not nil, names the files that gen writes and the
// bytes it gives them, and each must be tracked in the index as a
// regular file (mode 100644) with exactly those bytes and match no ignore
// rule. The index is what a commit takes, so a file is judged as the
// merged tree holds it.
func generated(ctx context.Context, repo git.Repo, gen step, wanted func(*os.Root) ([]output, error)) ([]finding, error) {
	root, err := os.OpenRoot(repo.Dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	before, err := snapshot(ctx, repo, root)
	if err != nil {
		return nil, err
	}
	if out, err := gen.run(ctx, repo.Dir); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", gen.name, err, out)
	}
	after, err := snapshot(ctx, repo, root)
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
	if wanted != nil {
		outs, err := wanted(root)
		if err != nil {
			return nil, err
		}
		tracked, err := trackedOutputs(ctx, repo, outs)
		if err != nil {
			return nil, err
		}
		findings = append(findings, tracked...)
	}
	slices.SortStableFunc(findings, func(a, b finding) int { return strings.Compare(a.where, b.where) })
	return findings, nil
}

// regularMode is the index mode of a regular file without the execute
// bit, the one mode a generated file has.
const regularMode = "100644"

// trackedOutputs returns a finding for each of outs that the index does
// not hold as a regular file with the bytes of the output, or that
// matches an ignore rule.
func trackedOutputs(ctx context.Context, repo git.Repo, outs []output) ([]finding, error) {
	// ":(literal)" keeps a glob character of a path from being one.
	pathspec := []string{"--"}
	for _, o := range outs {
		pathspec = append(pathspec, ":(literal)"+o.path)
	}
	staged, err := repo.Run(ctx, nil, append([]string{"ls-files", "--stage", "-z"}, pathspec...)...)
	if err != nil {
		return nil, err
	}
	// "<mode> <object> <stage>\t<path>", one entry for each stage.
	entries := map[string][]string{}
	for _, rec := range strings.Split(strings.TrimSuffix(string(staged), "\x00"), "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		if ok {
			entries[path] = append(entries[path], meta)
		}
	}
	ignored, err := repo.Run(ctx, nil, append([]string{"ls-files", "--cached", "--ignored", "--exclude-standard", "-z"}, pathspec...)...)
	if err != nil {
		return nil, err
	}
	isIgnored := map[string]bool{}
	for _, path := range strings.Split(string(ignored), "\x00") {
		isIgnored[path] = true
	}
	var findings []finding
	for _, o := range outs {
		msg, err := trackedProblem(ctx, repo, o, entries[o.path])
		if err != nil {
			return nil, err
		}
		if msg == "" && isIgnored[o.path] {
			msg = "is tracked and matches an ignore rule; a generated file is not ignored"
		}
		if msg != "" {
			findings = append(findings, finding{o.path, "generated", msg})
		}
	}
	return findings, nil
}

// trackedProblem says what is wrong with the index entries of one
// output, or returns "" when they are right.
func trackedProblem(ctx context.Context, repo git.Repo, o output, entries []string) (string, error) {
	if len(entries) == 0 {
		return "is not tracked; run go generate, add the file and commit it", nil
	}
	want, err := repo.Run(ctx, o.content, "hash-object", "--stdin")
	if err != nil {
		return "", err
	}
	for _, meta := range entries {
		mode, rest, _ := strings.Cut(meta, " ")
		object, stage, _ := strings.Cut(rest, " ")
		switch {
		case stage != "0":
			return "has an unmerged entry in the index; resolve it", nil
		case mode != regularMode:
			return fmt.Sprintf("is tracked with mode %s; a generated file is a regular file (%s), not a link and not executable", mode, regularMode), nil
		case object != strings.TrimSpace(string(want)):
			return "is tracked with other bytes than go generate gives; run it, add the file and commit it", nil
		}
	}
	return "", nil
}

// snapshot maps each file of the working tree, tracked or untracked and
// not ignored, to a digest of its type, mode and content. A tracked file
// deleted from disk is not in it. Files are read through root, so a path
// that reaches outside it is an error and not a read.
func snapshot(ctx context.Context, repo git.Repo, root *os.Root) (map[string]string, error) {
	out, err := repo.Run(ctx, nil, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, name := range bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0}) {
		if len(name) == 0 {
			continue
		}
		digest, ok, err := fileDigest(root, string(name))
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
// path below root, and false for anything else, a missing file and a
// directory (a submodule) included. The mode of a regular file is part
// of the digest, so a chmod is a change.
func fileDigest(root *os.Root, path string) (string, bool, error) {
	info, err := root.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	h := sha256.New()
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := root.Readlink(path)
		if err != nil {
			return "", false, err
		}
		_, _ = io.WriteString(h, "link "+target)
	case info.Mode().IsRegular():
		f, err := root.Open(path)
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
