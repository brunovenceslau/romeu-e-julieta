// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/handoff"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	_ "github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/status"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// The version stamp the tests give julieta (10 10.4); the clock is the
// sandbox's.
var stamp = cli.Version{Version: "v0.1.0-test", Commit: "abc1234", Date: "2026-10-20T00:00:00Z", Protocol: manifest.Protocol}

// narrative is a body with the headings of a clear handoff, and final
// the one of a final handoff.
const (
	narrative = "## Goal\nShip the cart.\n## State\nTests pass.\n## Decisions\nUse a map.\n## Next steps\nOpen the PR.\n## Open questions\nNone.\n"
	final     = narrative + "## What would be lost\nNothing.\n"
)

// sandbox is a HOME with a manifest of the project shop: the primary
// repo api and the secondary repo web, each a git repository with one
// commit and a memory dir, and the clock and the ULID source every
// julieta run of the test shares.
type sandbox struct {
	home  string
	m     manifest.Manifest
	ulids *canon.ULIDSource
	now   time.Time
}

// newSandbox makes a sandbox, sets HOME and JULIETA_MANIFEST for the
// test, and works in the primary repo.
func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir(), ulids: canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 1<<16))),
		now: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	primary := filepath.Join(t.TempDir(), "api")
	web := filepath.Join(s.home, "src", "shop", "web")
	mem := t.TempDir()
	for _, p := range []string{primary, web} {
		require.NoError(t, os.MkdirAll(p, 0o700))
		git(t, p, "init", "-q", "-b", "main")
		git(t, p, "commit", "-q", "--allow-empty", "-m", "one")
	}
	for _, d := range []string{"api", "web"} {
		require.NoError(t, os.MkdirAll(filepath.Join(mem, d), 0o700))
	}
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	s.m = manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: primary, Memory: filepath.Join(mem, "api")},
			{Dir: "web", URL: "git://127.0.0.1:9418/web.git", Ref: "main", Path: web, Memory: filepath.Join(mem, "web")},
		},
	}
	t.Setenv("HOME", s.home)
	v, err := manifest.EncodeEnv(s.m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
	t.Chdir(primary)
	return s
}

// git runs git in dir for a test, with an identity and no signing, and
// returns its output.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{"/usr/bin/git", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir,
		Env: []string{"HOME=" + t.TempDir(), "PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1"}, Stdout: &stdout, Stderr: &stderr})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// mem returns the memory dir of the repo i, and repo its path.
func (s *sandbox) mem(i int) string  { return s.m.Repos[i].Memory }
func (s *sandbox) repo(i int) string { return s.m.Repos[i].Path }

// advance moves the clock by d.
func (s *sandbox) advance(d time.Duration) { s.now = s.now.Add(d) }

// julieta runs julieta with args, with stdin as its standard input
// (empty for none), and returns the exit status, stdout and stderr.
func (s *sandbox) julieta(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	return s.run(t.Context(), stdin, args...)
}

// run is julieta for a goroutine other than the test's, which may not
// fail the test.
func (s *sandbox) run(ctx context.Context, stdin string, args ...string) (int, string, string) {
	in, err := os.CreateTemp("", "stdin")
	if err != nil {
		return -1, "", err.Error()
	}
	defer func() { _ = os.Remove(in.Name()); _ = in.Close() }()
	if _, err := in.WriteString(stdin); err != nil {
		return -1, "", err.Error()
	}
	if _, err := in.Seek(0, 0); err != nil {
		return -1, "", err.Error()
	}
	old := os.Stdin
	os.Stdin = in
	defer func() { os.Stdin = old }()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: func() time.Time { return s.now }, ULIDs: s.ulids, Version: stamp}
	code := cli.Main(ctx, args, env)
	return code, stdout.String(), stderr.String()
}

// ok runs julieta with args and requires exit 0.
func (s *sandbox) ok(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	code, stdout, stderr := s.julieta(t, stdin, args...)
	require.Equal(t, 0, code, stderr)
	return stdout
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

// names lists the file names of the handoff/ of the memory dir i.
func (s *sandbox) names(t *testing.T, i int) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(s.mem(i), "handoff"))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}
