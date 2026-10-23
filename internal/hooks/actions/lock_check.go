// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// julieta's action of pre-commit: julieta lock --check when a mise file
// is staged (04 4.3, REQ-0408; 07 7.3, REQ-0619). A failure stops the
// chain: the commit does not happen with a lock that is behind its
// mise.toml.
var _ = register("pre-commit", lockCheckAction)

// lockCheckAction is the action lock-check.
var lockCheckAction = Action{Name: "lock-check", Run: runLockCheck}

// misePaths are the files whose staging asks for the check, and the three
// the check reads.
var misePaths = []string{mise.ConfigFile, mise.LockFile, mise.ExceptionsFile}

// gitPaths are where this action looks for git, as every julieta git
// call does (the PATH of gitsafe's sandbox mode), so no PATH the agent
// sets chooses the program.
var gitPaths = []string{"/usr/bin/git", "/bin/git"}

// runLockCheck asks the index whether a mise file is staged and, when
// one is, runs the check of julieta lock --check on what the commit
// holds: the three files read from the index, with a size cap, not from
// the work tree. The index is the one git gives the hook in
// GIT_INDEX_FILE, which for git commit -a and git commit <path> is a
// temporary one, checked to lie in the git dir; so those commits are
// judged by what they record, and an unstaged change of the work tree
// neither blocks a good commit nor excuses a bad one. The findings go to
// stderr, one line each as the command prints them, and are the error
// check-findings (RJ-328, exit 6), a file that does not parse among
// them (REQ-0406); a file the index cannot give is an error that names
// it. The repo need not be in a manifest.
func runLockCheck(ctx context.Context, in Input) error {
	idx, err := indexFile(in)
	if err != nil {
		return fmt.Errorf("julieta lock --check: %w", err)
	}
	if !hasIndex(in.Dir, idx) {
		return nil // no index, so nothing staged, and no git to start
	}
	git, err := newGit(in.Env)
	if err != nil {
		return err
	}
	staged, err := git.Staged(ctx, in.Dir, idx, misePaths...)
	if err != nil {
		return fmt.Errorf("julieta lock --check: %w", err)
	}
	if len(staged) == 0 {
		return nil
	}
	raw, err := mise.RawFrom(func(name string) ([]byte, bool, error) {
		return git.IndexFile(ctx, in.Dir, idx, name, mise.MaxFileSize)
	})
	if err != nil {
		return fmt.Errorf("julieta lock --check: the mise files of %s in the index: %s", termsafe.Escape(in.Dir), termsafe.Escape(err.Error()))
	}
	rep := raw.Check()
	if len(rep.Findings) == 0 {
		return nil
	}
	out := cli.NewWriter(in.Stderr)
	var details []cli.Detail
	var slugs []string
	for _, f := range rep.Findings {
		if _, err := fmt.Fprintf(out, "finding %s\n", f); err != nil {
			return err
		}
		details = append(details, cli.Detail{Key: strings.TrimPrefix(f.String(), f.Slug+" "), Value: f.Slug})
		slugs = append(slugs, f.Slug)
	}
	return cli.CheckFindings.New(fmt.Sprintf("a mise file is staged and mise.lock is not fresh: %d findings: %s", len(details), strings.Join(slugs, ", ")), details...)
}

// indexFile returns the absolute path of the index the hook is given in
// GIT_INDEX_FILE, "" when git gives none (the repository's own index). A
// relative value is read from the hook's directory, where git runs it,
// and the path must lie below the git dir, GIT_DIR or .git of the hook's
// directory, so a variable the hook's caller sets cannot make this action
// read any other file.
func indexFile(in Input) (string, error) {
	idx := envValue(in.Env, "GIT_INDEX_FILE")
	if idx == "" {
		return "", nil
	}
	gitDir := envValue(in.Env, "GIT_DIR")
	if gitDir == "" {
		gitDir = ".git"
	}
	abs := func(p string) string {
		if !filepath.IsAbs(p) {
			p = filepath.Join(in.Dir, p)
		}
		return filepath.Clean(p)
	}
	idx, gitDir = abs(idx), abs(gitDir)
	if rel, err := filepath.Rel(gitDir, idx); err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("GIT_INDEX_FILE %q is not a file of the git dir %q", idx, gitDir)
	}
	return idx, nil
}

// hasIndex reports whether there is an index to read: the file idx, or,
// for the repository's own, a .git that is not a directory (a linked
// worktree's file) or holds an index. A .git directory with none has
// nothing staged and starts no git.
func hasIndex(dir, idx string) bool {
	if idx != "" {
		_, err := os.Lstat(idx)
		return err == nil
	}
	info, err := os.Lstat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	_, err = os.Lstat(filepath.Join(dir, ".git", "index"))
	return err == nil
}

// envValue returns the value of the last KEY=value of env, "" when none.
func envValue(env []string, key string) string {
	v := ""
	for _, kv := range env {
		if rest, ok := strings.CutPrefix(kv, key+"="); ok {
			v = rest
		}
	}
	return v
}

// newGit returns the gitsafe runner of the action, in sandbox mode,
// with the HOME the hook runs with.
func newGit(env []string) (*gitsafe.Runner, error) {
	home := envValue(env, "HOME")
	for _, p := range gitPaths {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return gitsafe.New(gitsafe.Config{Mode: gitsafe.Sandbox, Git: p, Home: home})
		}
	}
	return nil, fmt.Errorf("no git at %s", strings.Join(gitPaths, " or "))
}
