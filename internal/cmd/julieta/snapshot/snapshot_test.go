// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package snapshot_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/snapshot"
	ihooks "github.com/brunovenceslau/romeu-e-julieta/internal/hooks"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

var update = flag.Bool("update", false, "rewrite the golden files of testdata")

// testdata is the absolute path of testdata/, read before a test
// changes its working directory.
var testdata, _ = filepath.Abs("testdata")

// clock is the time the tests give julieta (10 10.4).
var clock = time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)

// TestSnapshotHelp is the help test of julieta snapshot (12 12.3): its help
// exits 0 and opens with its usage line.
func TestSnapshotHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr)}
	assert.Equal(t, 0, cli.Main(t.Context(), []string{"snapshot", "--help"}, env))
	assert.Empty(t, stderr.String())
	assert.True(t, strings.HasPrefix(stdout.String(), "Usage: julieta snapshot [flags]\n"), stdout.String())
}

// sandbox is a HOME, a manifest of two repos, api (primary, a real
// repository, the working directory) and web (a secondary not cloned
// yet), and their memory dirs.
type sandbox struct {
	home string
	m    manifest.Manifest
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir()}
	primary := filepath.Join(t.TempDir(), "api")
	mem := t.TempDir()
	for _, p := range []string{filepath.Join(mem, "api"), filepath.Join(mem, "web")} {
		require.NoError(t, os.MkdirAll(p, 0o700))
	}
	git(t, filepath.Dir(primary), "init", "-q", primary)
	base := commit(t, primary, "base")
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	s.m = manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Base: []string{base}, Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: primary, Memory: filepath.Join(mem, "api")},
			{Base: []string{}, Dir: "web", URL: "git://127.0.0.1:9418/web.git", Ref: "main", Path: filepath.Join(s.home, "src", "shop", "web"), Memory: filepath.Join(mem, "web")},
		},
	}
	t.Setenv("HOME", s.home)
	v, err := manifest.EncodeEnv(s.m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
	t.Chdir(primary)
	return s
}

// primary returns the path of api.
func (s *sandbox) primary() string { return s.m.Repos[0].Path }

// snapshotDir returns the snapshot dir of api's memory dir.
func (s *sandbox) snapshotDir() string { return filepath.Join(s.m.Repos[0].Memory, "snapshot") }

// julieta runs julieta with args and returns the exit status, stdout and
// stderr.
func julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: func() time.Time { return clock }, ULIDs: canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 1<<12)))}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// git runs plain git, to build fixtures, with no system or global config
// and a fixed identity and date, so a fixture's commits have the same
// ids on every run, and returns its standard output, trimmed.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{"/usr/bin/git", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir, Stdout: &stdout, Stderr: &stderr,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		}})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// commit adds an empty commit with the message msg and returns it.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	git(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

// golden compares got with testdata/name, or rewrites it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join(testdata, name)
	if *update {
		require.NoError(t, os.WriteFile(p, got, 0o644))
	}
	want, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

// TestSnapshotWritesHeadsJSON (03 3.13, REQ-0261; 08 8.4, REQ-0689):
// julieta snapshot in the primary writes snapshot/heads.json, pinned by
// a golden, which romeu's reader decodes strictly, after the bundle,
// and prints both paths, the createdAt and the repo.
func TestSnapshotWritesHeadsJSON(t *testing.T) {
	s := newSandbox(t)
	c1 := commit(t, s.primary(), "one")
	git(t, s.primary(), "commit", "-q", "--amend", "--allow-empty", "-m", "one amended")
	base := s.m.Repos[0].Base[0]
	git(t, s.primary(), "tag", "v1", base)

	code, stdout, stderr := julieta(t, "snapshot")
	require.Equal(t, 0, code, stderr)
	heads := filepath.Join(s.snapshotDir(), salvage.HeadsFile)
	assert.Equal(t, "wrote "+filepath.Join(s.snapshotDir(), salvage.BundleFile)+" repo api\n"+
		"wrote "+heads+" createdAt 2026-10-21T09:30:00Z repo api\n", stdout)
	data, err := os.ReadFile(heads)
	require.NoError(t, err)
	golden(t, "heads.json.golden", data)
	h, err := salvage.DecodeHeads(data)
	require.NoError(t, err)
	assert.Equal(t, base, h.Refs["refs/tags/v1"], "a ref the base reaches is listed, though the bundle leaves it out")
	assert.Equal(t, c1, h.Refs["refs/salvage-reflog/main/1"], "the amended commit, in main's reflog only")
}

// TestSnapshotNamesTheRepos: --repo names one repo, --all every one; a
// repo whose snapshot fails (web, not cloned) is said and exits 1 while
// the others are written; --repo with --all, or an unknown repo, is
// refused.
func TestSnapshotNamesTheRepos(t *testing.T) {
	s := newSandbox(t)
	commit(t, s.primary(), "one")
	t.Chdir(t.TempDir())
	code, _, stderr := julieta(t, "snapshot")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "is in no repo of the manifest")

	code, stdout, stderr := julieta(t, "snapshot", "--repo", "api")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "repo api\n")

	code, stdout, stderr = julieta(t, "snapshot", "--all")
	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "createdAt 2026-10-21T09:30:00Z repo api\n")
	assert.NotContains(t, stdout, "repo web")
	assert.Contains(t, stderr, "the snapshot of the repo web")

	code, _, _ = julieta(t, "snapshot", "--repo", "api", "--all", "--json")
	assert.Equal(t, 2, code)
	code, _, _ = julieta(t, "snapshot", "--repo", "nope")
	assert.Equal(t, 1, code)
}

// TestSnapshotNeedsAManifest: no manifest is exit 2 (REQ-0425).
func TestSnapshotNeedsAManifest(t *testing.T) {
	newSandbox(t)
	t.Setenv(setup.ManifestEnv, "")
	code, _, _ := julieta(t, "snapshot")
	assert.Equal(t, 2, code)
}

// TestTheDispatcherTakesTheSnapshot (04 4.3, REQ-0408, REQ-0409; 08 8.4,
// REQ-0686, REQ-0691): julieta hooks run post-commit, post-rewrite and
// post-merge each take the snapshot of the repo the hook runs in, from
// the primary and from a linked worktree of it, before the repo's own
// hook; a snapshot that cannot be written keeps the previous bundle,
// leaves no temporary file, is recorded as a hook failure with its
// repo, and the repo's own post-commit still runs; a repo of no
// manifest takes no snapshot and records nothing.
func TestTheDispatcherTakesTheSnapshot(t *testing.T) {
	s := newSandbox(t)
	out := filepath.Join(t.TempDir(), "out")
	require.NoError(t, os.MkdirAll(filepath.Join(s.primary(), ".githooks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(s.primary(), ".githooks", "post-commit"), []byte("#!/bin/sh\necho tracked >>"+out+"\n"), 0o755))
	failures := filepath.Join(s.home, ".local/state/julieta/hook-failures")

	c1 := commit(t, s.primary(), "one")
	for _, hook := range []string{"post-commit", "post-rewrite", "post-merge"} {
		require.NoError(t, os.RemoveAll(s.snapshotDir()))
		code, _, stderr := julieta(t, "hooks", "run", hook)
		require.Equal(t, 0, code, stderr)
		assert.Empty(t, stderr, hook)
		data, err := os.ReadFile(filepath.Join(s.snapshotDir(), salvage.HeadsFile))
		require.NoError(t, err, hook)
		h, err := salvage.DecodeHeads(data)
		require.NoError(t, err)
		assert.Equal(t, c1, h.Refs["refs/heads/main"], hook)
	}

	wt := filepath.Join(t.TempDir(), "wt")
	git(t, s.primary(), "worktree", "add", "-q", "--detach", wt)
	d1 := commit(t, wt, "in the worktree")
	t.Chdir(wt)
	code, _, stderr := julieta(t, "hooks", "run", "post-commit")
	require.Equal(t, 0, code, stderr)
	data, err := os.ReadFile(filepath.Join(s.snapshotDir(), salvage.HeadsFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), d1, "the worktree's HEAD, in the primary's snapshot")

	got, err := ihooks.Failures(failures)
	require.NoError(t, err)
	assert.Empty(t, got)

	if os.Geteuid() != 0 { // root writes a read-only directory
		t.Chdir(s.primary())
		bundle, err := os.ReadFile(filepath.Join(s.snapshotDir(), salvage.BundleFile))
		require.NoError(t, err)
		commit(t, s.primary(), "two")
		require.NoError(t, os.Chmod(s.snapshotDir(), 0o500))
		t.Cleanup(func() { _ = os.Chmod(s.snapshotDir(), 0o700) })
		code, _, stderr = julieta(t, "hooks", "run", "post-commit")
		assert.Equal(t, 0, code, "a failed snapshot does not fail the commit")
		assert.Contains(t, stderr, "julieta hooks: post-commit stage julieta (snapshot) exited 1\n")
		require.NoError(t, os.Chmod(s.snapshotDir(), 0o700))
		now, err := os.ReadFile(filepath.Join(s.snapshotDir(), salvage.BundleFile))
		require.NoError(t, err)
		assert.Equal(t, bundle, now, "the previous bundle stays")
		entries, err := os.ReadDir(s.snapshotDir())
		require.NoError(t, err)
		assert.Len(t, entries, 2, "no .tmp- file of this run remains")
		got, err := ihooks.Failures(failures)
		require.NoError(t, err)
		require.Len(t, got, 1)
		real, err := filepath.EvalSymlinks(s.primary())
		require.NoError(t, err)
		assert.Equal(t, ihooks.Failure{At: clock, ExitCode: 1, Hook: "post-commit", Path: "snapshot", Repo: real, Stage: ihooks.StageJulieta}, got[0])
		log, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat("tracked\n", 2), string(log), "the repo's own post-commit ran each time")
	}

	other := t.TempDir()
	git(t, other, "init", "-q")
	commit(t, other, "scratch")
	t.Chdir(other)
	require.NoError(t, os.RemoveAll(failures))
	code, _, stderr = julieta(t, "hooks", "run", "post-commit")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr, "a repo of no manifest takes no snapshot")
	_, err = os.Stat(failures)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
