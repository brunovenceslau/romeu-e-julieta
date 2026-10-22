// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
)

// rule is the Rule of every finding of the mise check.
const rule = "mise"

// examplesDir is the directory whose mise files the check judges besides
// those at the top of the tree: each directory below it that holds a
// mise.toml or a mise.lock (07 7.3; REQ-0845).
const examplesDir = "examples"

// emptyRunDigest is the digest of kind run of the empty run, {} (03
// 3.12), a literal since tools/** may not import internal/canon (REQ-0971).
// TestEmptyRunDigest of internal/cmd/julieta/setup holds the same
// value, and a manifest with another one is refused by julieta, so a
// change of canon fails this check at once.
const emptyRunDigest = "d33e4b621d0923ab86c327af8e7819ddba70d80740f12b580bf056748a01ff25"

// The manifest the check gives julieta: one repo, the one directory
// judged.
const (
	repoDir    = "check"
	repoOrigin = "git://127.0.0.1:9418/check.git"
)

// Steps returns the step of all: the mise subcommand, run with the
// pinned go in the environment of the steps. The subcommand builds julieta from this
// tree, so it runs code of the change (RunsCode).
func Steps(_ context.Context, p check.Plan) []check.Step {
	return []check.Step{{
		Name:    "mise",
		Argv:    []string{filepath.Join(p.Tools.GoDir, "go"), "run", "./tools/ci", "mise"},
		Environ: check.StepEnv(p.Tools.GoDir),
	}}
}

// Run is the mise subcommand (10 10.2, REQ-0845): the logic of julieta
// lock --check, which it runs as the binary built from this tree, on the
// mise files at the top of the tree and in each directory below
// examples/ that holds one. tools/** reaches internal/mise only through
// the binary that links it (REQ-0971), so the check builds julieta with
// the pinned go and gives it a manifest of one repo for each directory.
// A finding is a line "finding <slug> <tool> [<platform>]" of julieta
// lock --check; any finding fails the check, and so does an exit of
// julieta that is neither 0 nor 6, or a 6 with no finding printed.
func Run(ctx context.Context, e check.Env, args []string) (bool, error) {
	if len(args) != 0 {
		return false, fmt.Errorf("mise takes no argument\n%s", check.Usage())
	}
	root, err := check.RepoRoot(ctx, e)
	if err != nil {
		return false, err
	}
	dirs, err := miseDirs(root)
	if err != nil {
		return false, err
	}
	if len(dirs) == 0 {
		check.Say(e.Stdout, "%s\n", check.Summary("mise", 0))
		return true, nil
	}
	goBin, err := WhichPinned(ctx, root, GoTool, GoTool, GoTool)
	if err != nil {
		return false, err
	}
	tmp, err := os.MkdirTemp("", "ci-mise-")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	bin := filepath.Join(tmp, "julieta")
	if err := build(ctx, goBin, root, bin); err != nil {
		return false, err
	}
	return report(ctx, e.Stdout, bin, root, tmp, dirs)
}

// report runs julieta lock --check of the binary bin on each directory,
// prints the findings and the summary to out, and says whether there were
// none.
func report(ctx context.Context, out io.Writer, bin, root, tmp string, dirs []string) (bool, error) {
	var findings []check.Finding
	for _, dir := range dirs {
		f, err := checkDir(ctx, bin, root, tmp, dir)
		if err != nil {
			return false, err
		}
		findings = append(findings, f...)
	}
	check.Say(out, "%s%s\n", check.Lines(findings), check.Summary("mise", len(findings)))
	return len(findings) == 0, nil
}

// miseDirs returns the directories to judge, relative to root with
// forward slashes, "." for the top of the tree: the top when it holds a
// mise.toml, and each directory below examples/ that holds one. A lock
// with no mise.toml names no tool to be missing, so a directory with
// only a lock is not judged.
func miseDirs(root string) ([]string, error) {
	var dirs []string
	if has(filepath.Join(root, "mise.toml")) {
		dirs = append(dirs, ".")
	}
	err := filepath.WalkDir(filepath.Join(root, examplesDir), func(p string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist) && p == filepath.Join(root, examplesDir):
			return fs.SkipAll
		case err != nil:
			return err
		case d.Name() == "mise.toml" && !d.IsDir():
			rel, err := filepath.Rel(root, filepath.Dir(p))
			if err != nil {
				return err
			}
			dirs = append(dirs, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(dirs)
	return dirs, err
}

// has reports whether a file or a link is at path.
func has(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// build builds julieta from the tree at root into bin, with the pinned
// go in the environment of the steps.
func build(ctx context.Context, goBin, root, bin string) error {
	var out bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   []string{goBin, "build", "-o", bin, "./cmd/julieta"},
		Dir:    root,
		Env:    check.StepEnv(filepath.Dir(goBin)),
		Stdout: &out,
		Stderr: &out,
	})
	if err != nil {
		return fmt.Errorf("go build ./cmd/julieta: %w\n%s", err, bytes.TrimSpace(out.Bytes()))
	}
	return nil
}

// checkDir runs "julieta lock --check" of the binary bin on the
// directory dir of the tree at root, and returns its findings, as
// findings of the check. julieta gets a home of its own below tmp and a
// manifest of one repo at the directory;
// exit 6 is findings, any other failure is an error of the check.
func checkDir(ctx context.Context, bin, root, tmp, dir string) ([]check.Finding, error) {
	abs := filepath.Join(root, filepath.FromSlash(dir))
	home, err := os.MkdirTemp(tmp, "home-")
	if err != nil {
		return nil, err
	}
	memory := filepath.Join(home, "memory")
	env, err := manifestEnv(abs, memory)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	err = procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.ToolStep,
		Argv:   []string{bin, "lock", "--check", "--repo", repoDir},
		Dir:    root,
		Env:    []string{"HOME=" + home, "PATH=/usr/bin:/bin", "JULIETA_MANIFEST=" + env},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if exit, ok := errors.AsType[*procgroup.ExitError](err); err != nil && (!ok || exit.Code != 6) {
		return nil, fmt.Errorf("julieta lock --check in %s: %w\n%s", git.Printable(dir), err, bytes.TrimSpace(stderr.Bytes()))
	}
	where := git.Printable(filepath.ToSlash(filepath.Join(dir, "mise.lock")))
	var findings []check.Finding
	for line := range strings.Lines(stdout.String()) {
		if msg, ok := strings.CutPrefix(strings.TrimSuffix(line, "\n"), "finding "); ok {
			findings = append(findings, check.Finding{Where: where, Rule: rule, Msg: msg})
		}
	}
	if err != nil && len(findings) == 0 {
		return nil, fmt.Errorf("julieta lock --check in %s exited 6 and printed no finding\n%s", git.Printable(dir), bytes.TrimSpace(stderr.Bytes()))
	}
	return findings, nil
}

// manifestEnv returns the value of JULIETA_MANIFEST for a sandbox of
// one primary repo at path, with the memory dir at memory (03 3.6).
func manifestEnv(path, memory string) (string, error) {
	return manifest.EncodeEnv(manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "check", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: emptyRunDigest, Schema: manifest.Field,
		Salvage:    manifest.Salvage{ExcludeIgnored: []string{}},
		SpecCommit: strings.Repeat("ab", 20),
		Repos:      []manifest.Repo{{Base: []string{}, Dir: repoDir, Memory: memory, Path: path, Primary: true, Ref: "main", URL: repoOrigin}},
	})
}
