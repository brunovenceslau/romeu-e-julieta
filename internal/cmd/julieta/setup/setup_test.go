// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// TestSetupHelp is the help test of julieta setup (12 12.3): its help
// exits 0 and opens with its usage line.
func TestSetupHelp(t *testing.T) {
	code, stdout, stderr := julieta(t.Context(), "setup", "--help")
	assert.Equal(t, 0, code)
	assert.Empty(t, stderr)
	assert.True(t, strings.HasPrefix(stdout, "Usage: julieta setup [flags]\n"), stdout)
}

// TestSetupWithoutManifest exits 2 with manifest-protocol when
// JULIETA_MANIFEST is unset and none is cached (04 4.3, REQ-0425).
func TestSetupWithoutManifest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv(setup.ManifestEnv, "")
	code, stdout, stderr := julieta(t.Context(), "setup", "--json")
	assert.Equal(t, 2, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "RJ-342", errorID(t, stderr))
}

// TestSetupProtocols accepts a manifest of protocol N and refuses one
// of protocol 0 and one of N+1 with manifest-protocol, through the
// decoder of internal/manifest (03 3.6, REQ-0219).
func TestSetupProtocols(t *testing.T) {
	s := newSandbox(t)
	for _, p := range []int{0, s.m.Protocol + 1} {
		m := s.m
		m.Protocol = p
		(&sandbox{m: m}).setManifest(t)
		code, stdout, stderr := julieta(t.Context(), "setup", "--json")
		assert.Equal(t, 2, code, "protocol %d", p)
		assert.Empty(t, stdout)
		assert.Equal(t, "RJ-342", errorID(t, stderr), "protocol %d", p)
	}
	s.setManifest(t)
	code, _, stderr := setupJSON(t)
	assert.Equal(t, 0, code, stderr)
}

// TestSetupLinksClonesAndFastForwards is the run of setup on a new
// sandbox, then again (04 4.3): the PATH link to the running binary,
// the missing secondary cloned, the primary's clean default branch
// fast-forwarded and the move recorded in julieta state with the
// injected clock and version; the second run changes nothing and keeps
// the move for the next SessionStart.
func TestSetupLinksClonesAndFastForwards(t *testing.T) {
	s := newSandbox(t)
	old := git(t, s.primary(), "rev-parse", "HEAD")
	tip := push(t, filepath.Join(s.base, "api.git"), "two")

	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	exe, err := os.Executable()
	require.NoError(t, err)
	link := filepath.Join(s.home, ".local", "bin", "julieta")
	assert.Equal(t, setup.Link{Changed: true, Path: link, Target: exe}, rep.Link)
	have, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, exe, have)
	assert.Equal(t, "v0.1.0-test", rep.Version)
	assert.Empty(t, rep.Unusable)
	moved := setup.Warning{Slug: setup.DefaultBranchMoved, Message: "api " + old + ".." + tip}
	assert.Equal(t, []setup.Warning{moved}, rep.Warnings)
	assert.Equal(t, "warning default-branch-moved: api "+old+".."+tip+"\n", stderr)
	require.Len(t, rep.Repos, 2)
	assert.Equal(t, setup.Repo{Dir: "api", Path: s.primary(), Fetched: true, DefaultBranch: "main",
		FastForward: "moved", From: old, To: tip}, rep.Repos[0])
	assert.Equal(t, setup.Repo{Dir: "web", Path: s.m.Repos[1].Path, Cloned: true, Fetched: true, DefaultBranch: "main",
		FastForward: "current"}, rep.Repos[1])
	assert.Equal(t, tip, git(t, s.primary(), "rev-parse", "HEAD"))
	assert.FileExists(t, filepath.Join(s.m.Repos[1].Path, ".git", "HEAD"))

	state, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, setup.State{At: clock, Version: "v0.1.0-test", Warnings: []setup.Warning{moved}}, state)

	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
	assert.False(t, rep.Link.Changed, "the link is kept")
	assert.Empty(t, rep.Warnings)
	for _, r := range rep.Repos {
		assert.Equal(t, "current", r.FastForward, r.Dir)
		assert.False(t, r.Cloned, r.Dir)
	}
	state, err = setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, []setup.Warning{moved}, state.Warnings, "the move holds until a SessionStart shows it")
}

// TestSetupText prints one line for the link and one per repo.
func TestSetupText(t *testing.T) {
	s := newSandbox(t)
	code, stdout, stderr := julieta(t.Context(), "setup")
	require.Equal(t, 0, code, stderr)
	exe, err := os.Executable()
	require.NoError(t, err)
	assert.Equal(t, "link  "+filepath.Join(s.home, ".local", "bin", "julieta")+" -> "+exe+" (made)\n"+
		"repo  api  fetched, main current\n"+
		"repo  web  cloned, fetched, main current\n", stdout)
}

// TestSetupReportsWhatItDoesNotMove leaves a default branch that is not
// clean, one that is not checked out, and one with commits of its own
// where they are, and reports each; none is a warning.
func TestSetupReportsWhatItDoesNotMove(t *testing.T) {
	for name, tc := range map[string]struct {
		dirty bool
		args  []string
		want  string
	}{
		"dirty":    {dirty: true, want: "not-clean"},
		"branch":   {args: []string{"switch", "-q", "-c", "topic"}, want: "not-checked-out"},
		"detached": {args: []string{"switch", "-q", "--detach"}, want: "not-checked-out"},
		"diverged": {args: []string{"commit", "-q", "--allow-empty", "-m", "mine"}, want: "diverged"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			push(t, filepath.Join(s.base, "api.git"), "two")
			if tc.dirty {
				require.NoError(t, os.WriteFile(filepath.Join(s.primary(), "new"), nil, 0o600))
			} else {
				git(t, s.primary(), tc.args...)
			}
			head := git(t, s.primary(), "rev-parse", "HEAD")
			code, rep, stderr := setupJSON(t)
			require.Equal(t, 0, code, stderr)
			assert.Equal(t, tc.want, rep.Repos[0].FastForward)
			assert.Empty(t, rep.Warnings)
			assert.Equal(t, head, git(t, s.primary(), "rev-parse", "HEAD"))
		})
	}
}

// TestSetupFetchFailed is a fetch that fails: the warning fetch-failed,
// naming the repo and git's error, recorded in julieta state, and exit
// 0 (04 4.3, REQ-0399).
func TestSetupFetchFailed(t *testing.T) {
	s := newSandbox(t)
	s.m.Repos[0].URL = s.url + "gone.git"
	s.setManifest(t)
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rep.Warnings, 1)
	assert.Equal(t, setup.FetchFailed, rep.Warnings[0].Slug)
	assert.True(t, strings.HasPrefix(rep.Warnings[0].Message, "api: "), rep.Warnings[0].Message)
	assert.Contains(t, stderr, "warning fetch-failed: api: ")
	assert.Equal(t, "not-fetched", rep.Repos[0].FastForward)
	assert.NotEmpty(t, rep.Repos[0].Error)
	state, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, rep.Warnings, state.Warnings)

	// The next setup whose fetch works drops the warning.
	s.m.Repos[0].URL = s.url + "api.git"
	s.setManifest(t)
	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Warnings)
	state, err = setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Empty(t, state.Warnings)
}

// TestSetupUnusable exits 1 when the PATH link or a secondary clone
// cannot be made, after the other steps ran (04 4.3).
func TestSetupUnusable(t *testing.T) {
	t.Run("link", func(t *testing.T) {
		s := newSandbox(t)
		require.NoError(t, os.MkdirAll(filepath.Join(s.home, ".local", "bin", "julieta"), 0o700))
		code, rep, _ := setupJSON(t)
		assert.Equal(t, 1, code)
		require.Len(t, rep.Unusable, 1)
		assert.Contains(t, rep.Unusable[0], "the PATH link")
		assert.True(t, rep.Repos[1].Cloned, "the other steps ran")
	})
	t.Run("clone", func(t *testing.T) {
		s := newSandbox(t)
		s.m.Repos[1].URL = s.url + "gone.git"
		s.setManifest(t)
		code, rep, _ := setupJSON(t)
		assert.Equal(t, 1, code)
		require.Len(t, rep.Unusable, 1)
		assert.Contains(t, rep.Unusable[0], "the secondary repo web")
		assert.NoDirExists(t, s.m.Repos[1].Path)
	})
	t.Run("secondary path", func(t *testing.T) {
		s := newSandbox(t)
		s.m.Repos[1].Path = filepath.Join(t.TempDir(), "web")
		s.setManifest(t)
		code, rep, _ := setupJSON(t)
		assert.Equal(t, 1, code)
		require.Len(t, rep.Unusable, 1)
		assert.NoDirExists(t, s.m.Repos[1].Path)
	})
}

// TestSetupCachesTheManifest caches the manifest of the variable, which
// a later command without it reads.
func TestSetupCachesTheManifest(t *testing.T) {
	s := newSandbox(t)
	code, _, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	t.Setenv(setup.ManifestEnv, "")
	m, fromEnv, err := setup.LoadManifest(s.home)
	require.NoError(t, err)
	assert.False(t, fromEnv)
	assert.Equal(t, s.m.SpecCommit, m.SpecCommit)
	code, _, stderr = setupJSON(t)
	assert.Equal(t, 0, code, stderr)
}

// TestDecodeIsStrict reads the --json document strictly: the fixtures
// with a duplicate member and with an unknown one are refused, and the
// valid one is read.
func TestDecodeIsStrict(t *testing.T) {
	for name, valid := range map[string]bool{"valid.json": true, "invalid-duplicate-member.json": false, "invalid-unknown-member.json": false} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)
		_, err = setup.Decode(data)
		if valid {
			require.NoError(t, err, name)
		} else {
			require.Error(t, err, name)
		}
	}
	_, err := setup.Decode([]byte(`{"link":{"changed":false,"path":"","target":""},"repos":[],"schema":"julieta-setup/v2","unusable":[],"version":"","warnings":[]}`))
	require.Error(t, err, "another schema")
}

// TestSetupWithoutADefaultBranch reports a repo whose origin/HEAD is
// missing, and fast-forwards nothing in it.
func TestSetupWithoutADefaultBranch(t *testing.T) {
	s := newSandbox(t)
	git(t, s.primary(), "remote", "set-head", "origin", "-d")
	code, stdout, stderr := julieta(t.Context(), "setup")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "repo  api  fetched, no-default\n")
}

// TestSetupReplacesWhatIsAtTheLink replaces a file at the PATH link, and
// a link to another binary, with the link to the running binary.
func TestSetupReplacesWhatIsAtTheLink(t *testing.T) {
	s := newSandbox(t)
	link := filepath.Join(s.home, ".local", "bin", "julieta")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
	for _, plant := range []func() error{
		func() error { return os.WriteFile(link, []byte("old"), 0o600) },
		func() error { return errors.Join(os.Remove(link), os.Symlink("/usr/bin/true", link)) },
	} {
		require.NoError(t, plant())
		code, rep, stderr := setupJSON(t)
		require.Equal(t, 0, code, stderr)
		assert.True(t, rep.Link.Changed)
		have, err := os.Readlink(link)
		require.NoError(t, err)
		assert.Equal(t, rep.Link.Target, have)
	}
}

// TestSetupRefusesABadCache reads a cached manifest the decoder refuses
// as manifest-protocol, and a HOME that is not absolute as no place to
// work in.
func TestSetupRefusesABadCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(setup.ManifestEnv, "")
	require.NoError(t, os.MkdirAll(setup.StateDir(home), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(setup.StateDir(home), "manifest.json"), []byte(`{"schema":"manifest/v1"}`), 0o600))
	code, _, stderr := julieta(t.Context(), "setup", "--json")
	assert.Equal(t, 2, code)
	assert.Equal(t, "RJ-342", errorID(t, stderr))

	t.Setenv("HOME", "relative")
	code, _, _ = julieta(t.Context(), "setup")
	assert.Equal(t, 1, code)
}

// TestCheckRepo refuses the values of a repo that cannot become a path
// or a URL (03 3.1, REQ-0271).
func TestCheckRepo(t *testing.T) {
	good := manifest.Repo{Dir: "web", Path: "/home/a/src/shop/web", URL: "https://example.com/web"}
	require.NoError(t, setup.CheckRepo("/home/a", "shop", good))
	for name, edit := range map[string]func(r *manifest.Repo, project *string){
		"dir":      func(r *manifest.Repo, _ *string) { r.Dir = "../web" },
		"reserved": func(r *manifest.Repo, _ *string) { r.Dir = "Memory" },
		"path":     func(r *manifest.Repo, _ *string) { r.Path = "/home/a/src/shop/../web" },
		"place":    func(r *manifest.Repo, _ *string) { r.Path = "/tmp/web" },
		"project":  func(_ *manifest.Repo, p *string) { *p = "Shop" },
		"url":      func(r *manifest.Repo, _ *string) { r.URL = "file:///tmp/web" },
	} {
		r, project := good, "shop"
		edit(&r, &project)
		assert.Error(t, setup.CheckRepo("/home/a", project, r), name)
	}
}

// TestPrintWarning prints a slug julieta does not record as what it is.
func TestPrintWarning(t *testing.T) {
	var b strings.Builder
	setup.PrintWarning(cli.NewWriter(&b), setup.Warning{Slug: "other", Message: "m"})
	assert.Equal(t, "julieta state holds a warning this julieta does not know, \"other\": m\n", b.String())
}
