// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
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

// TestSetupLinksClonesAndMovesNoBranch is the run of setup on a new
// sandbox, then again (04 4.3, REQ-0399): the PATH link to the running
// binary, the missing secondary cloned, every repo fetched, and the
// primary's clean default branch, behind origin's, left at its commit
// and reported behind, with no warning; the second run changes nothing.
func TestSetupLinksClonesAndMovesNoBranch(t *testing.T) {
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
	assert.Empty(t, rep.Warnings)
	assert.Empty(t, stderr)
	require.Len(t, rep.Repos, 2)
	assert.Equal(t, setup.Repo{Dir: "api", Path: s.primary(), Fetched: true, DefaultBranch: "main",
		BranchState: "behind"}, rep.Repos[0], "fastForward, from and to stay empty")
	assert.Equal(t, setup.Repo{Dir: "web", Path: s.m.Repos[1].Path, Cloned: true, Fetched: true, DefaultBranch: "main",
		BranchState: "current"}, rep.Repos[1])
	assert.Equal(t, old, git(t, s.primary(), "rev-parse", "HEAD"), "the branch stays at its commit")
	assert.Equal(t, tip, git(t, s.primary(), "rev-parse", "origin/main"), "origin's tip is fetched")
	assert.FileExists(t, filepath.Join(s.m.Repos[1].Path, ".git", "HEAD"))

	state, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, clock, state.At)
	assert.Equal(t, "v0.1.0-test", state.Version)
	assert.Empty(t, state.Warnings)

	code, rep, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
	assert.False(t, rep.Link.Changed, "the link is kept")
	assert.Empty(t, rep.Warnings)
	assert.Equal(t, "behind", rep.Repos[0].BranchState)
	assert.Equal(t, "current", rep.Repos[1].BranchState)
	assert.False(t, rep.Repos[1].Cloned)
	assert.Equal(t, old, git(t, s.primary(), "rev-parse", "HEAD"))
}

// TestSetupWritesTheDeprecatedMembersEmpty: every repo of the --json
// document keeps fastForward, from and to, the members of
// julieta-setup/v1 that setup no longer fills, each present and empty,
// and carries branchState instead (REQ-0163, REQ-1310).
func TestSetupWritesTheDeprecatedMembersEmpty(t *testing.T) {
	s := newSandbox(t)
	push(t, filepath.Join(s.base, "api.git"), "two")
	code, stdout, stderr := julieta(t.Context(), "setup", "--json")
	require.Equal(t, 0, code, stderr)
	var doc struct {
		Repos []map[string]any `json:"repos"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc))
	require.Len(t, doc.Repos, 2)
	for _, r := range doc.Repos {
		for _, m := range []string{"fastForward", "from", "to"} {
			v, ok := r[m]
			assert.True(t, ok, "%v holds %s", r["dir"], m)
			assert.Empty(t, v, "%v: %s", r["dir"], m)
		}
		assert.NotEmpty(t, r["branchState"], r["dir"])
	}
}

// TestSetupDropsTheRetiredSlugOfAnOldState: a setup.json written before
// setup moved no branch may hold default-branch-moved; it is read with
// that entry dropped, and the next setup writes the file without it.
func TestSetupDropsTheRetiredSlugOfAnOldState(t *testing.T) {
	s := newSandbox(t)
	path := filepath.Join(setup.StateDir(s.home), "setup.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(`{"at":"2026-10-27T00:00:00Z","version":"v0","warnings":[`+
		`{"message":"api a..b","slug":"default-branch-moved"},{"message":"web: offline","slug":"fetch-failed"}]}`), 0o600))
	state, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, []setup.Warning{{Slug: setup.FetchFailed, Message: "web: offline"}}, state.Warnings)

	code, _, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "default-branch-moved")
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
		dirty  bool
		noPush bool
		args   []string
		want   string
	}{
		"dirty":    {dirty: true, want: "not-clean"},
		"ahead":    {noPush: true, args: []string{"commit", "-q", "--allow-empty", "-m", "mine"}, want: "ahead"},
		"branch":   {args: []string{"switch", "-q", "-c", "topic"}, want: "not-checked-out"},
		"detached": {args: []string{"switch", "-q", "--detach"}, want: "not-checked-out"},
		"diverged": {args: []string{"commit", "-q", "--allow-empty", "-m", "mine"}, want: "diverged"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t)
			if !tc.noPush {
				push(t, filepath.Join(s.base, "api.git"), "two")
			}
			if tc.dirty {
				require.NoError(t, os.WriteFile(filepath.Join(s.primary(), "new"), nil, 0o600))
			} else {
				git(t, s.primary(), tc.args...)
			}
			head := git(t, s.primary(), "rev-parse", "HEAD")
			code, rep, stderr := setupJSON(t)
			require.Equal(t, 0, code, stderr)
			assert.Equal(t, tc.want, rep.Repos[0].BranchState)
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
	assert.Equal(t, "not-fetched", rep.Repos[0].BranchState)
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
	t.Run("not a repository", func(t *testing.T) {
		s := newSandbox(t)
		require.NoError(t, os.MkdirAll(s.m.Repos[1].Path, 0o700))
		code, rep, _ := setupJSON(t)
		assert.Equal(t, 1, code)
		require.Len(t, rep.Unusable, 1)
		assert.Contains(t, rep.Unusable[0], "the secondary repo web")
		assert.False(t, rep.Repos[1].Fetched)
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

// TestDecodeIgnoresAnUnknownMember reads a document to which a newer
// julieta added a member, at every level, as the document without it
// (03, opening, REQ-0163).
func TestDecodeIgnoresAnUnknownMember(t *testing.T) {
	valid, err := os.ReadFile(filepath.Join("testdata", "valid.json"))
	require.NoError(t, err)
	want, err := setup.Decode(valid)
	require.NoError(t, err)
	// Every object of the document gains a member, as a newer julieta
	// that added one would write it.
	newer := strings.ReplaceAll(strings.ReplaceAll(string(valid), "{", `{"added":{"a":[1]},`), ",}", "}")
	got, err := setup.Decode([]byte(newer))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestDecodeHostileUnknownMembers holds what a member this build skips
// may not do, whoever wrote it (03, opening, REQ-0163): change what a
// known member says (a name of another case is another name, in either
// order), pass a duplicate or a nest too deep unseen, or put its own text
// in an error.
func TestDecodeHostileUnknownMembers(t *testing.T) {
	valid, err := os.ReadFile(filepath.Join("testdata", "valid.json"))
	require.NoError(t, err)
	want, err := setup.Decode(valid)
	require.NoError(t, err)
	doc := strings.TrimSpace(string(valid))
	after := func(members string) []byte { return []byte(strings.TrimSuffix(doc, "}") + "," + members + "}") }
	before := func(members string) []byte { return []byte("{" + members + "," + strings.TrimPrefix(doc, "{")) }
	for name, members := range map[string]string{
		"a case variant of a known name":  `"Schema":"x","SCHEMA":"y"`,
		"an escape in a name and a value": `"\u001b[31mred":"\u0007\u001b[0m"`,
	} {
		for pos, build := range map[string]func(string) []byte{"after": after, "before": before} {
			got, err := setup.Decode(build(members))
			require.NoError(t, err, name+" "+pos)
			assert.Equal(t, want, got, name+" "+pos)
		}
	}
	for name, members := range map[string]string{
		"a duplicate in an unknown member":        `"x":{"a":1,"a":2}`,
		"a nest of 20000 arrays":                  `"x":` + strings.Repeat("[", 20000) + strings.Repeat("]", 20000),
		"a duplicate known name beside an escape": `"\u001b[31mred":1,"schema":"julieta-setup/v1"`,
	} {
		_, err := setup.Decode(after(members))
		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), "\x1b", name)
		assert.NotContains(t, err.Error(), "red", name)
	}
	// The name of a duplicate is in the error, as jsontext quotes it, with
	// the escape written out and no raw control character.
	_, err = setup.Decode(after(`"\u001b[31mred":1,"\u001b[31mred":2`))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\x1b")
}

// TestDecodeIsStrict reads the --json document strictly: the fixture
// with a duplicate member is refused, the valid one is read, and a
// document of another schema is refused.
func TestDecodeIsStrict(t *testing.T) {
	for name, valid := range map[string]bool{"valid.json": true, "invalid-duplicate-member.json": false} {
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
// a link to another binary, with the link to the running binary, and
// leaves the running binary itself where it is when it is the file at
// the path.
func TestSetupReplacesWhatIsAtTheLink(t *testing.T) {
	s := newSandbox(t)
	link := filepath.Join(s.home, ".local", "bin", "julieta")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o700))
	// The running binary is a regular file at the path itself, as when a
	// copy was installed there.
	require.NoError(t, os.WriteFile(link, []byte("binary"), 0o700))
	restore := setup.SetExecutable(func() (string, error) { return link, nil })
	code, rep, stderr := setupJSON(t)
	restore()
	require.Equal(t, 0, code, stderr)
	assert.False(t, rep.Link.Changed, "the running binary is the file at the path")
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "the binary is kept, not replaced by a link to itself")
	require.NoError(t, os.Remove(link))

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
	good := manifest.Repo{Dir: "web", Path: "/sandbox/src/shop/web", URL: "https://example.com/web"}
	require.NoError(t, setup.CheckRepo("/sandbox", "shop", good))
	for name, edit := range map[string]func(r *manifest.Repo, project *string){
		"dir":      func(r *manifest.Repo, _ *string) { r.Dir = "../web" },
		"reserved": func(r *manifest.Repo, _ *string) { r.Dir = "Memory" },
		"path":     func(r *manifest.Repo, _ *string) { r.Path = "/sandbox/src/shop/../web" },
		"place":    func(r *manifest.Repo, _ *string) { r.Path = "/tmp/web" },
		"project":  func(_ *manifest.Repo, p *string) { *p = "Shop" },
		"url":      func(r *manifest.Repo, _ *string) { r.URL = "file:///tmp/web" },
	} {
		r, project := good, "shop"
		edit(&r, &project)
		assert.Error(t, setup.CheckRepo("/sandbox", project, r), name)
	}
}

// TestPrintWarning prints a slug julieta does not record as what it is.
func TestPrintWarning(t *testing.T) {
	var b strings.Builder
	setup.PrintWarning(cli.NewWriter(&b), setup.Warning{Slug: "other", Message: "m"})
	assert.Equal(t, "julieta state holds a warning this julieta does not know, \"other\": m\n", b.String())
}

// TestSetupReportsAnUnreadableState reports a julieta state file it
// cannot read, goes on, and replaces it with this run's state.
func TestSetupReportsAnUnreadableState(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, os.MkdirAll(setup.StateDir(s.home), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(setup.StateDir(s.home), "setup.json"), []byte("{"), 0o600))
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	require.Len(t, rep.Errors, 1)
	assert.Contains(t, stderr, "error  julieta state setup.json: ")
	state, err := setup.ReadState(s.home)
	require.NoError(t, err)
	assert.Equal(t, clock, state.At)
}

// TestEmptyRunDigest holds the digest of kind run of the empty run, {},
// that e2e/julieta_setup_test.go writes into its manifests as a
// literal, since e2e may not import internal/canon (REQ-0971). When
// canon changes the digest, this test fails with the new value, which
// replaces emptyRunDigest there and here.
func TestEmptyRunDigest(t *testing.T) {
	sum, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	assert.Equal(t, "d33e4b621d0923ab86c327af8e7819ddba70d80740f12b580bf056748a01ff25", sum.String())
}
