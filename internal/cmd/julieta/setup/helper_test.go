// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The clock and the version stamp the tests give julieta (10 10.4).
var (
	clock = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	stamp = cli.Version{Version: "v0.1.0-test", Commit: "abc1234", Date: "2026-10-20T00:00:00Z", Protocol: manifest.Protocol}
)

// julieta runs julieta with args and returns the exit status, stdout and
// stderr.
func julieta(ctx context.Context, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: func() time.Time { return clock }, Version: stamp}
	code := cli.Main(ctx, args, env)
	return code, stdout.String(), stderr.String()
}

// errorID returns the id of the --json error report on stderr.
func errorID(t *testing.T, stderr string) string {
	t.Helper()
	var e struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &e), stderr)
	return e.ID
}

// gitPath is the git of the tests, the first one julieta looks at.
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

// fixtureEnv is the environment of the plain git that builds fixtures:
// no system or global config, and a fixed identity and date.
func fixtureEnv(home string) []string {
	return []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + home, "LANG=C",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	}
}

// git runs git without julieta, to build fixtures, and returns its
// standard output, trimmed.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir,
		Env: fixtureEnv(t.TempDir()), Stdout: &stdout, Stderr: &stderr})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// commit adds an empty commit to the branch checked out in dir and
// returns it.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	git(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return git(t, dir, "rev-parse", "HEAD")
}

// origins makes a bare origin per name below a new base, each with one
// commit on main, served over git:// on 127.0.0.1, and returns the base
// and the URL prefix of the server.
func origins(t *testing.T, names ...string) (base, url string) {
	t.Helper()
	base = t.TempDir()
	for _, name := range names {
		work := t.TempDir()
		git(t, work, "init", "-q")
		commit(t, work, "one")
		git(t, base, "clone", "-q", "--bare", work, name+".git")
	}
	return base, serveGit(t, base)
}

// push adds a commit to main of the bare origin at dir and returns it.
func push(t *testing.T, dir, msg string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "w")
	git(t, filepath.Dir(work), "clone", "-q", dir, work)
	head := commit(t, work, msg)
	git(t, work, "push", "-q", "origin", "main")
	return head
}

// serveGit serves the repositories below base over git:// on
// 127.0.0.1, one git daemon --inetd per connection, and returns the
// URL prefix of base, ending in a slash.
func serveGit(t *testing.T, base string) string {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	argv := []string{gitPath(t), "daemon", "--inetd", "--export-all", "--base-path=" + base}
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				// A failed session shows as a failed fetch in the test.
				_ = procgroup.Run(ctx, procgroup.Command{Class: procgroup.JulietaGit, Argv: argv,
					Env: fixtureEnv(base), Stdin: conn, Stdout: conn})
			})
		}
	})
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		wg.Wait()
	})
	return "git://" + ln.Addr().String() + "/"
}

// sandbox is a HOME with a manifest of the project shop: the primary
// repo api, cloned at a path of its own, and the secondary repo web,
// missing, whose origins are served at url.
type sandbox struct {
	home, base, url string
	m               manifest.Manifest
}

// newSandbox makes a sandbox, sets HOME and JULIETA_MANIFEST for the
// test, and returns it.
func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir()}
	s.base, s.url = origins(t, "api", "web")
	primary := filepath.Join(t.TempDir(), "api")
	git(t, filepath.Dir(primary), "clone", "-q", filepath.Join(s.base, "api.git"), primary)
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	mem := t.TempDir()
	for _, dir := range []string{"api", "web"} {
		require.NoError(t, os.Mkdir(filepath.Join(mem, dir), 0o700))
	}
	s.m = manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{"github.com", "dl.google.com"}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Dir: "api", URL: s.url + "api.git", Primary: true, Ref: "main", Path: primary, Memory: filepath.Join(mem, "api")},
			{Dir: "web", URL: s.url + "web.git", Ref: "main", Path: filepath.Join(s.home, "src", "shop", "web"), Memory: filepath.Join(mem, "web")},
		},
	}
	t.Setenv("HOME", s.home)
	s.setManifest(t)
	return s
}

// setManifest sets JULIETA_MANIFEST to the env value of s.m.
func (s *sandbox) setManifest(t *testing.T) {
	t.Helper()
	v, err := manifest.EncodeEnv(s.m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
}

// primary is the path of the primary repo.
func (s *sandbox) primary() string { return s.m.Repos[0].Path }

// setupJSON runs julieta setup --json and returns its exit status, its
// report, read with the strict decoder, and stderr.
func setupJSON(t *testing.T) (int, setup.Report, string) {
	t.Helper()
	code, stdout, stderr := julieta(t.Context(), "setup", "--json")
	rep, err := setup.Decode([]byte(stdout))
	require.NoError(t, err, "stdout %q, stderr %q", stdout, stderr)
	return code, rep, stderr
}
