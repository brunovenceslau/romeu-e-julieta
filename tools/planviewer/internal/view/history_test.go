// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package view

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// plainGit runs git without gitsafe to build a fixture repository, with
// no global or system config so the host's signing cannot reach it.
func plainGit(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	git, err := findGit(os.Getenv("PATH"))
	require.NoError(t, err, "the tests need a git on the PATH")
	var stderr bytes.Buffer
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.JulietaGit,
		Argv:  append([]string{git, "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...),
		Dir:   dir,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=" + date,
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=" + date,
		},
		Stderr: &stderr,
	})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
}

func commitPlan(t *testing.T, dir, date, msg string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	plainGit(t, dir, date, "add", "-A")
	plainGit(t, dir, date, "commit", "-q", "-m", msg)
}

// The history of a real repository goes through internal/gitsafe: the
// commits that touched the task directory, a commit elsewhere left out.
func TestHistoryTrendsRealGit(t *testing.T) {
	dir := t.TempDir()
	plainGit(t, dir, "2026-02-01T00:00:00Z", "init", "-q")
	commitPlan(t, dir, "2026-02-01T00:00:00Z", "plan: first", map[string]string{
		"docs/plan/tasks/T001-a.md": "---\nid: T001\nfate: kept\n---\n",
		"docs/plan/tasks/T002-b.md": "---\nid: T002\nfate: kept\nneeds:\n  - T001\n---\n",
	})
	commitPlan(t, dir, "2026-02-02T00:00:00Z", "elsewhere", map[string]string{"README": "x\n"})
	commitPlan(t, dir, "2026-02-03T00:00:00Z", "plan: <b>merge</b> T001", map[string]string{
		"docs/plan/tasks/T001-a.md": "---\nid: T001\nfate: merged\n---\n",
	})

	tr := historyTrends(t.Context(), Options{History: true, HistoryLimit: 10, Repo: dir})
	require.Empty(t, tr.Note)
	require.Len(t, tr.Snapshots, 2)
	assert.Equal(t, 2, tr.Total)
	a, b := tr.Snapshots[0], tr.Snapshots[1]
	assert.Equal(t, "plan: first", a.Subject)
	assert.Equal(t, "plan: <b>merge</b> T001", b.Subject)
	assert.Equal(t, "2026-02-03T00:00:00Z", b.Time)
	assert.Len(t, b.OID, 40)
	assert.Equal(t, Counts{Ready: 1, Waiting: 1, Scope: 2, Critical: 2}, a.Counts)
	assert.Equal(t, Counts{Merged: 1, Ready: 1, Scope: 2, Critical: 1}, b.Counts)
	require.NotNil(t, b.Delta)
	assert.Equal(t, []string{"T001"}, b.Delta.Merged)
	assert.Equal(t, []string{"T001"}, b.Delta.Left)
	assert.Equal(t, 2, b.Delta.CritFrom)
	assert.Equal(t, 1, b.Delta.CritTo)
}

// A repository whose tasks never existed has no snapshot and no error.
func TestHistoryTrendsNoCommitTouchedTasks(t *testing.T) {
	dir := t.TempDir()
	plainGit(t, dir, "2026-02-01T00:00:00Z", "init", "-q")
	commitPlan(t, dir, "2026-02-01T00:00:00Z", "other", map[string]string{"README": "x\n"})
	tr := historyTrends(t.Context(), Options{History: true, HistoryLimit: 10, Repo: dir})
	assert.Empty(t, tr.Note)
	assert.Empty(t, tr.Snapshots)
}

// A repository gitsafe refuses (shallow, here by a missing HEAD) is a note.
func TestHistoryTrendsGitFailure(t *testing.T) {
	dir := t.TempDir()
	plainGit(t, dir, "2026-02-01T00:00:00Z", "init", "-q") // no commit: HEAD names nothing
	tr := historyTrends(t.Context(), Options{History: true, HistoryLimit: 10, Repo: dir})
	assert.Contains(t, tr.Note, "history cannot be read")
	assert.Empty(t, tr.Snapshots)
}

func TestFindGit(t *testing.T) {
	_, err := findGit("relative:/nonexistent-dir")
	require.ErrorContains(t, err, "no git")
}

// A git that lies under the repository is refused: a checkout could plant it.
func TestGitSourceRefusesGitInsideRepo(t *testing.T) {
	repo := t.TempDir()
	bin := filepath.Join(repo, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\n"), 0o700))
	t.Setenv("PATH", bin)
	_, err := gitSource(repo)
	require.ErrorContains(t, err, "under the repository")
	assert.True(t, inside(repo, filepath.Join(bin, "git")))
	assert.False(t, inside(repo, filepath.Join(filepath.Dir(repo), "git")))
}

// With no usable HOME git still runs, with the temporary directory as its home.
func TestGitSourceHome(t *testing.T) {
	t.Setenv("HOME", "relative")
	dir := t.TempDir()
	plainGit(t, dir, "2026-02-01T00:00:00Z", "init", "-q")
	commitPlan(t, dir, "2026-02-01T00:00:00Z", "plan", map[string]string{"docs/plan/tasks/T001-a.md": "---\nid: T001\nfate: kept\n---\n"})
	tr := historyTrends(t.Context(), Options{History: true, HistoryLimit: 5, Repo: dir})
	assert.Empty(t, tr.Note)
	assert.Len(t, tr.Snapshots, 1)
}
