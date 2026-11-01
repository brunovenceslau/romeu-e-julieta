// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
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
// head of the config ref or a --from commit that a fetched branch of the
// origin reaches, the config ref or another (REQ-0336).
func TestFetchConfig(t *testing.T) {
	type fixture struct {
		c                               *config
		first, head, own, tree, feature string
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
		{name: "a commit only another branch of the origin reaches", from: func(f fixture) string { return f.feature }, want: func(f fixture) string { return f.feature }},
		{name: "a commit of the clone the origin never had", from: func(f fixture) string { return f.own }, slug: "from-unreachable"},
		{name: "a commit the clone does not hold", from: func(fixture) string { return strings.Repeat("1", 40) }, slug: "from-unreachable"},
		{name: "the tree of the head", from: func(f fixture) string { return f.tree }, slug: "from-unreachable"},
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
			c := newConfig(t, map[string]string{"projects/cfg.yaml": "first"})
			f := fixture{c: c, first: gitIn(t, c.origin, "rev-parse", "HEAD")}
			f.head = commitFiles(t, c.origin, map[string]string{"projects/cfg.yaml": "second"})
			gitIn(t, c.origin, "checkout", "-q", "-b", "feature", f.first)
			f.feature = commitFiles(t, c.origin, map[string]string{"projects/cfg.yaml": "on a branch"})
			gitIn(t, c.origin, "checkout", "-q", "main")
			f.own = commitFiles(t, c.clone, map[string]string{"local": "never pushed"})
			f.tree = gitIn(t, c.origin, "rev-parse", f.head+"^{tree}")
			if tt.arrange != nil {
				tt.arrange(t, f)
			}
			r := c.run(flags{from: tt.from(f)})

			err := fetchConfig(t.Context(), r)

			if tt.slug != "" {
				require.Error(t, err)
				assert.Equal(t, 2, cli.ExitCode(err))
				assert.Contains(t, err.Error(), tt.slug+": ")
				assert.Empty(t, r.specCommit)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want(f), r.specCommit)
			assert.Equal(t, c.clone, r.configClone)
			assert.NotNil(t, r.git)
			assert.Equal(t, [][]string{{"https://github.com/owner/cfg", "+refs/heads/*:refs/romeu/origin/cfg/*"}}, c.fetched)
			assert.Equal(t, f.head, gitIn(t, c.clone, "rev-parse", "refs/romeu/origin/cfg/main"))
		})
	}
}

// TestConfigMarker: step 2 finds the config repo's clone by the marker
// <cfg>-env/.romeu-config (02 2.3), with two clones there; a marker
// that is malformed, a link, or names no clone is exit 2 (config-marker);
// with no marker and one clone the guess of before stands and warns
// config-marker-missing.
func TestConfigMarker(t *testing.T) {
	const slug = "config-marker"
	tests := []struct {
		name string
		// marker is the content of the marker; "" writes none. link
		// makes the marker a symbolic link to a file of that content.
		marker string
		link   bool
		// other adds a second clone dir under <cfg>-env/.
		other bool
		// linkedDir adds a clone dir of that name under <cfg>-env/, with
		// a .git; viaLink makes it a symbolic link to such a dir outside.
		linkedDir string
		viaLink   bool
		want      string // the dir found; "" when step 2 refuses with slug
		warn      string
	}{
		{name: "the marker names the clone among two", marker: "cfg\n", other: true, want: "cfg"},
		{name: "no marker and one clone warns", want: "cfg", warn: "config-marker-missing"},
		{name: "no newline", marker: "cfg"},
		{name: "a parent path", marker: "../cfg\n"},
		{name: "a path", marker: "x/cfg\n"},
		{name: "a reserved name", marker: ".romeu\n"},
		{name: "two lines", marker: "cfg\nother\n"},
		{name: "a dir that is not there", marker: "other\n"},
		{name: "a dir without a .git", marker: "plain\n"},
		{name: "a link", marker: "cfg\n", link: true},
		{name: "over the limit", marker: strings.Repeat("c", markerLimit) + "\n"},
		{name: "a name of 63 bytes is held to the pattern, not the size", marker: strings.Repeat("c", 63) + "\n", linkedDir: strings.Repeat("c", 63), want: strings.Repeat("c", 63)},
		{name: "a name of 64 bytes", marker: strings.Repeat("c", 64) + "\n"},
		{name: "a clone dir that is a link", marker: "linked\n", linkedDir: "linked", viaLink: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newConfig(t, map[string]string{"projects/cfg.yaml": "first"})
			env := filepath.Join(c.root, "cfg-env")
			require.NoError(t, os.MkdirAll(filepath.Join(env, "plain"), 0o700))
			if tt.other {
				require.NoError(t, os.MkdirAll(filepath.Join(env, "other", ".git"), 0o700))
			}
			if tt.linkedDir != "" {
				target := filepath.Join(env, tt.linkedDir)
				if tt.viaLink {
					target = filepath.Join(c.home, "elsewhere")
				}
				require.NoError(t, os.MkdirAll(target, 0o700))
				gitIn(t, target, "init", "-q", "-b", "main")
				if tt.viaLink {
					require.NoError(t, os.Symlink(target, filepath.Join(env, tt.linkedDir)))
				}
			}
			marker := filepath.Join(env, markerName)
			switch {
			case tt.link:
				target := filepath.Join(c.home, "marker")
				require.NoError(t, os.WriteFile(target, []byte(tt.marker), 0o600))
				require.NoError(t, os.Symlink(target, marker))
			case tt.marker != "":
				require.NoError(t, os.WriteFile(marker, []byte(tt.marker), 0o600))
			}
			r := c.run(flags{from: fromOrigin})

			err := fetchConfig(t.Context(), r)

			if tt.want == "" {
				require.Error(t, err)
				assert.Equal(t, 2, cli.ExitCode(err))
				assert.Contains(t, err.Error(), slug+": ")
				assert.Empty(t, c.fetched, "nothing is fetched before the clone is found")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(env, tt.want), r.configClone)
			var out bytes.Buffer
			for _, w := range r.warnings {
				w(cli.NewWriter(&out))
			}
			if tt.warn == "" {
				assert.Empty(t, out.String())
				return
			}
			assert.Contains(t, out.String(), "warning "+tt.warn+": "+env+" has no "+markerName)
			assert.Contains(t, out.String(), "write cfg as the only line of "+marker)
		})
	}
}
