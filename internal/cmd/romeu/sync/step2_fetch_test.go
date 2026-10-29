// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The unit tests of steps 2 and 3 run real git on local repositories,
// with a fixture in place of the fetch (fetch): gitsafe fetches https
// URLs only, and the real fetch is the e2e's.

// gitIn runs plain git in dir, outside gitsafe, to build a fixture, and
// returns its trimmed output.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolGit, Argv: append([]string{git}, args...), Dir: dir,
		Env: []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=f", "GIT_AUTHOR_EMAIL=f@example.com", "GIT_COMMITTER_NAME=f", "GIT_COMMITTER_EMAIL=f@example.com"},
	})
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// commitFiles writes files, by path, into the work tree of the
// repository in dir, commits them all and returns the commit.
func commitFiles(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	for p, data := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, p), []byte(data), 0o600))
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "x")
	return gitIn(t, dir, "rev-parse", "HEAD")
}

// config is a config repo and its clone for steps 2 and 3: the origin,
// a local repository whose branch main the config ref names, and the
// clone romeu init made of it at <root>/cfg-env/cfg.
type config struct {
	root, home, origin, clone string
	settings                  spec.HostSettings
	// fetched are the URL and the refspecs of each fetch.
	fetched [][]string
}

// newConfig makes the origin with one commit of files on main, clones
// it and replaces fetch with a fetch from the origin.
func newConfig(t *testing.T, files map[string]string) *config {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	c := &config{root: filepath.Join(tmp, "root"), home: filepath.Join(tmp, "home"), origin: filepath.Join(tmp, "origin")}
	c.clone = filepath.Join(c.root, "cfg-env", "cfg")
	for _, d := range []string{c.home, c.origin, filepath.Dir(c.clone)} {
		require.NoError(t, os.MkdirAll(d, 0o700))
	}
	gitIn(t, c.origin, "init", "-q", "-b", "main")
	commitFiles(t, c.origin, files)
	gitIn(t, tmp, "clone", "-q", c.origin, c.clone)
	c.settings = spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   c.root,
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: "/usr/bin/false", Git: git},
	}
	old := fetch
	fetch = func(_ context.Context, _ *gitsafe.Runner, dir, url string, refspecs []string) error {
		c.fetched = append(c.fetched, append([]string{url}, refspecs...))
		gitIn(t, dir, append([]string{"fetch", "-q", c.origin}, refspecs...)...)
		return nil
	}
	t.Cleanup(func() { fetch = old })
	return c
}

// run returns a run whose step 1 is done, with the flags f.
func (c *config) run(f flags) *run {
	if f.from == "" {
		f.from = fromOrigin
	}
	r := newRun(&cli.Env{}, f)
	r.settings, r.home = c.settings, c.home
	return r
}

// TestFetchConfig is step 2 of 04 4.2: the fetch of the config repo's
// origin into refs/romeu/origin/<cfg-dir>/*, and the spec commit, the
// head of the config ref or a --from commit that it reaches.
func TestFetchConfig(t *testing.T) {
	type fixture struct {
		c                *config
		first, head, own string
	}
	tests := []struct {
		name string
		// from returns --from for the fixture.
		from    func(f fixture) string
		arrange func(t *testing.T, f fixture)
		// want returns the spec commit, "" when step 2 refuses with
		// slug.
		want func(f fixture) string
		slug string
	}{
		{name: "origin is the head of the config ref", from: func(fixture) string { return fromOrigin }, want: func(f fixture) string { return f.head }},
		{name: "the head itself", from: func(f fixture) string { return f.head }, want: func(f fixture) string { return f.head }},
		{name: "a commit the config ref reaches", from: func(f fixture) string { return f.first }, want: func(f fixture) string { return f.first }},
		{name: "a commit of the clone the origin never had", from: func(f fixture) string { return f.own }, slug: "from-unreachable"},
		{name: "a commit the clone does not hold", from: func(fixture) string { return strings.Repeat("1", 40) }, slug: "from-unreachable"},
		{name: "an abbreviated id", from: func(f fixture) string { return f.first[:12] }, slug: "from-unreachable"},
		{
			name:    "a config ref the origin lacks",
			from:    func(fixture) string { return fromOrigin },
			arrange: func(_ *testing.T, f fixture) { f.c.settings.Config.Ref = "trunk" },
			slug:    "config-ref-missing",
		},
		{
			name:    "no clone of the config repo",
			from:    func(fixture) string { return fromOrigin },
			arrange: func(_ *testing.T, f fixture) { f.c.settings.Config.Project = "other" },
			slug:    "config-clone-missing",
		},
		{
			name: "two clones that could be the config repo's",
			from: func(fixture) string { return fromOrigin },
			arrange: func(t *testing.T, f fixture) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(f.c.root, "cfg-env", "other", ".git"), 0o700))
			},
			slug: "config-clone-ambiguous",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			c := newConfig(t, map[string]string{"projects/cfg.yaml": "first"})
			f := fixture{c: c, first: gitIn(t, c.origin, "rev-parse", "HEAD")}
			f.head = commitFiles(t, c.origin, map[string]string{"projects/cfg.yaml": "second"})
			f.own = commitFiles(t, c.clone, map[string]string{"local": "never pushed"})
			if tt.arrange != nil {
				tt.arrange(t, f)
			}
			r := c.run(flags{from: tt.from(f)})

			err := fetchConfig(t.Context(), r)

			if tt.slug != "" {
				require.Error(t, err)
				is.Equal(2, cli.ExitCode(err))
				is.Contains(err.Error(), tt.slug+": ")
				is.Empty(r.specCommit)
				return
			}
			require.NoError(t, err)
			is.Equal(tt.want(f), r.specCommit)
			is.Equal(c.clone, r.configClone)
			is.NotNil(r.git)
			is.Equal([][]string{{"https://github.com/owner/cfg", "+refs/heads/*:refs/romeu/origin/cfg/*"}}, c.fetched)
			is.Equal(f.head, gitIn(t, c.clone, "rev-parse", "refs/romeu/origin/cfg/main"))
		})
	}
}

// TestFetchConfigLeavesTheClonesRefs: the fetch writes
// refs/romeu/origin/<cfg-dir>/* alone, so refs/heads, refs/remotes and
// HEAD of the clone are as they were (I5). The fetch is the fixture's,
// so this holds the refspec step 2 asks for; gitsafe's own tests hold
// its refusal of any other destination.
func TestFetchConfigLeavesTheClonesRefs(t *testing.T) {
	c := newConfig(t, map[string]string{"projects/cfg.yaml": "first"})
	commitFiles(t, c.origin, map[string]string{"projects/cfg.yaml": "second"})
	before := gitIn(t, c.clone, "for-each-ref", "refs/heads", "refs/remotes", "refs/tags")
	head := gitIn(t, c.clone, "symbolic-ref", "HEAD")

	require.NoError(t, fetchConfig(t.Context(), c.run(flags{})))

	assert.Equal(t, before, gitIn(t, c.clone, "for-each-ref", "refs/heads", "refs/remotes", "refs/tags"))
	assert.Equal(t, head, gitIn(t, c.clone, "symbolic-ref", "HEAD"))
}
