// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package actions

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// The clock of the tests (10 10.4) and the generation of their
// manifest.
var (
	clock      = time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)
	generation = "01j9zc2q8x7w4t6y3r5e1v9k0p"
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

// git runs plain git, to build fixtures and to check bundles, with no
// system or global config and a fixed identity and date, and returns
// its standard output, trimmed. Each commit of a fixture has a message
// of its own, since the fixed date makes two equal commits one.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitErr(t, dir, args...)
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	return out
}

// gitErr is git, returning the error and, on one, the standard error.
func gitErr(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir, Stdout: &stdout, Stderr: &stderr,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		}})
	if err != nil {
		return stderr.String(), err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// commit adds an empty commit with the message msg to what is checked
// out in dir and returns it.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	git(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

// fixture is a repo, its memory dir and the HOME of julieta.
type fixture struct {
	repo, memory, home string
	base               string
}

// newFixture returns a repo whose main holds the commit base, which the
// snapshot's bases name, an empty memory dir 0700 and a HOME.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{repo: filepath.Join(root, "api"), memory: filepath.Join(root, "memory"), home: filepath.Join(root, "home")}
	require.NoError(t, os.Mkdir(f.memory, 0o700))
	require.NoError(t, os.Mkdir(f.home, 0o700))
	git(t, root, "init", "-q", f.repo)
	f.base = commit(t, f.repo, "base")
	return f
}

// config returns the snapshot config of the fixture, with the clock at
// now.
func (f *fixture) config(t *testing.T, now time.Time) SnapshotConfig {
	t.Helper()
	g, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Sandbox, Git: gitPath(t), Home: f.home})
	require.NoError(t, err)
	return SnapshotConfig{
		Git: g, Dir: f.repo, Memory: f.memory, Lock: write.LockPath(f.home, "api"),
		Work: filepath.Join(f.home, ".local/state/julieta/snapshot/api"), Generation: generation, Bases: []string{f.base},
		Now: func() time.Time { return now }, ULIDs: canon.NewULIDSource(rand.Reader),
	}
}

// snapshotDir returns the snapshot dir of the fixture's memory dir.
func (f *fixture) snapshotDir() string { return filepath.Join(f.memory, "snapshot") }

// heads returns the decoded snapshot/heads.json of the fixture.
func (f *fixture) heads(t *testing.T) salvage.Heads {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)
	h, err := salvage.DecodeHeads(data)
	require.NoError(t, err)
	return h
}

// bundleHeads returns the refs of the fixture's bundle, after git bundle
// verify passes on it in the repo, which holds its prerequisites.
func (f *fixture) bundleHeads(t *testing.T) map[string]string {
	t.Helper()
	file := filepath.Join(f.snapshotDir(), salvage.BundleFile)
	git(t, f.repo, "bundle", "verify", "-q", file)
	out := map[string]string{}
	for line := range strings.Lines(git(t, f.repo, "bundle", "list-heads", file)) {
		oid, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		require.True(t, ok, line)
		out[name] = oid
	}
	return out
}

// names returns the names in dir, sorted.
func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	slices.Sort(out)
	return out
}
