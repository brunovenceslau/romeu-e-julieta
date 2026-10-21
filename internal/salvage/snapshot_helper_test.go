// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// gitPath is the git of the tests, the first of the two julieta looks
// at.
func gitPath(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"/usr/bin/git", "/bin/git"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	require.FailNow(t, "no git at /usr/bin/git or /bin/git")
	return ""
}

// runner returns a gitsafe runner in sandbox mode, the mode of julieta,
// with a HOME of its own.
func runner(t *testing.T) *gitsafe.Runner {
	t.Helper()
	r, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Sandbox, Git: gitPath(t), Home: t.TempDir()})
	require.NoError(t, err)
	return r
}

// git runs plain git, to build fixtures, with no system or global config
// and a fixed identity and date, and returns its standard output,
// trimmed. A fixed date makes two commits of one tree, parent and
// message one commit, so each commit of a fixture has a message of its
// own.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main", "-c", "tag.gpgsign=false"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir, Stdout: &stdout, Stderr: &stderr,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		}})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// commit adds an empty commit with the message msg to what is checked
// out in dir and returns it.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	git(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

// repo returns a new repository with core.logAllRefUpdates on, so every
// branch keeps a reflog as in a clone with a work tree.
func repo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	git(t, filepath.Dir(dir), "init", "-q", dir)
	return dir
}

// reflogIndex returns the smallest n for which <ref>@{n} is oid, as git
// names the entries of a reflog, newest 0.
func reflogIndex(t *testing.T, dir, ref, oid string) int {
	t.Helper()
	for n, line := range strings.Split(git(t, dir, "log", "-g", "--format=%H", ref), "\n") {
		if line == oid {
			return n
		}
	}
	require.FailNow(t, "no entry of the reflog holds the commit", "%s %s", ref, oid)
	return -1
}
