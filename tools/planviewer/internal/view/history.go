// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
)

// This file, like internal/gitsafe, builds on unix systems only, and so
// does tools/planviewer.

// gitSource reads the history of the repository at repo (an absolute
// path) through internal/gitsafe, which builds on unix systems only, as
// every target of this repository is, in host mode: a fresh environment, no
// global or system git config, no hook, and only the read-only verbs
// History and ReadDir. It finds git on the PATH of the process, as the
// product's host settings name it in an absolute path.
func gitSource(repo string) (Source, error) {
	git, err := findGit(os.Getenv("PATH"))
	if err != nil {
		return nil, err
	}
	// git runs with a fresh environment; HOME is the user's, or the
	// temporary directory, and never the repository.
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		home = os.TempDir()
	}
	if !filepath.IsAbs(home) {
		return nil, fmt.Errorf("no absolute home directory for git")
	}
	if inside(repo, git) {
		return nil, fmt.Errorf("git (%s) lies under the repository, which a checkout could plant; history skipped", git)
	}
	r, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: git, Home: home})
	if err != nil {
		return nil, err
	}
	return gitLog{r: r, repo: repo}, nil
}

// inside reports whether path, with symbolic links resolved, is dir or
// lies below it.
func inside(dir, path string) bool {
	d, err1 := filepath.EvalSymlinks(dir)
	p, err2 := filepath.EvalSymlinks(path)
	if err1 != nil || err2 != nil {
		d, p = dir, path
	}
	rel, err := filepath.Rel(d, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// findGit returns the absolute path of the first executable git on path.
func findGit(path string) (string, error) {
	for dir := range strings.SplitSeq(path, string(os.PathListSeparator)) {
		if !filepath.IsAbs(dir) {
			continue
		}
		p := filepath.Join(dir, "git")
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("no git on the PATH")
}

type gitLog struct {
	r    *gitsafe.Runner
	repo string
}

func (g gitLog) Log(ctx context.Context) ([]Commit, error) {
	cs, err := g.r.History(ctx, g.repo, "HEAD", TasksDir)
	if err != nil {
		return nil, err
	}
	out := make([]Commit, len(cs))
	for i, c := range cs {
		out[i] = Commit{OID: c.OID, Time: c.Time, Subject: c.Subject}
	}
	return out, nil
}

func (g gitLog) Tasks(ctx context.Context, oid string) ([]File, error) {
	fs, err := g.r.ReadDir(ctx, g.repo, oid, TasksDir)
	if err != nil {
		return nil, err
	}
	out := make([]File, len(fs))
	for i, f := range fs {
		out[i] = File{Name: f.Name, Data: f.Data}
	}
	return out, nil
}
