// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// helperScript writes an executable helper, mode 0700, in a new
// directory of the user's, mode 0700, and returns its path.
func helperScript(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.Mkdir(dir, 0o700))
	p := filepath.Join(dir, "git-credential-test")
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o700))
	return p
}

// TestCloneWithCredentialHelperArgv: the helper reaches the git clone
// command alone, as a credential.https://<host>.helper after the empty
// credential.helper of host mode, never as a helper for every host; the
// resolution of the URL, the read of HEAD, the listing of the filters
// and the checkout carry no helper.
func TestCloneWithCredentialHelperArgv(t *testing.T) {
	for name, helper := range map[string]string{"a name": "osxkeychain", "a path": helperScript(t)} {
		t.Run(name, func(t *testing.T) {
			r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
			require.NoError(t, err)
			var rec recorder
			rec.install(r)
			dest := filepath.Join(t.TempDir(), "r")

			require.NoError(t, r.CloneWithCredentialHelper(t.Context(), "https://GitHub.com/owner/cfg", dest, helper))

			require.Len(t, rec.commands, 5)
			for i, c := range rec.commands {
				has := slices.Contains(c.Argv, "credential.https://github.com.helper="+helper)
				assert.Equal(t, i == 1, has, "command %d: %v", i, c.Argv)
				assert.NotContains(t, c.Argv, "credential.helper="+helper)
			}
			clone := rec.commands[1].Argv
			assert.Equal(t, "clone", clone[1+len(hostFlags)+2])
			reset := slices.Index(clone, "credential.helper=")
			scoped := slices.Index(clone, "credential.https://github.com.helper="+helper)
			assert.Less(t, reset, scoped, "the scoped helper comes after the reset of the list")
		})
	}
}

// TestCloneWithCredentialHelperScopesTheResolvedHost: the key names the
// host git connects to, the URL as git resolves it, with its port, since
// git matches the credential key against that URL.
func TestCloneWithCredentialHelperScopesTheResolvedHost(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	record := r.start
	r.start = func(ctx context.Context, c procgroup.Command) error {
		if slices.Contains(c.Argv, "--get-url") {
			_, err := io.WriteString(c.Stdout, "https://mirror.example.com:8443/owner/cfg\n")
			return err
		}
		return record(ctx, c)
	}

	require.NoError(t, r.CloneWithCredentialHelper(t.Context(), "https://github.com/owner/cfg", filepath.Join(t.TempDir(), "r"), "osxkeychain"))

	assert.Contains(t, rec.commands[0].Argv, "credential.https://mirror.example.com:8443.helper=osxkeychain")
}

// TestCloneWithCredentialHelperRefuses: a helper git would run as a
// shell snippet, a value with a shell character, a relative or unclean
// path, one over the cap, and any helper in sandbox mode are refused
// before git runs.
func TestCloneWithCredentialHelperRefuses(t *testing.T) {
	for name, tc := range map[string]struct {
		mode   Mode
		helper string
	}{
		"the shell form":             {Host, "!gh auth git-credential"},
		"the shell form of a path":   {Host, "!/opt/homebrew/bin/gh auth git-credential"},
		"a path with a space":        {Host, "/opt/my helper"},
		"a command substitution":     {Host, "/bin/$(id)"},
		"a semicolon":                {Host, "store;id"},
		"a newline":                  {Host, "osxkeychain\nstore"},
		"a NUL":                      {Host, "osxkeychain\x00"},
		"a relative path":            {Host, "bin/git-credential-x"},
		"a path that is not clean":   {Host, "/opt/../bin/x"},
		"a path over the cap":        {Host, "/" + strings.Repeat("a", MaxCredentialHelper)},
		"a helper in sandbox mode":   {Sandbox, "osxkeychain"},
		"a name starting with a dot": {Host, ".hidden"},
		"an upper-case name":         {Host, "OSXKeychain"},
		"an option":                  {Host, "-x"},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := New(Config{Mode: tc.mode, Git: "/usr/bin/git", Home: "/h"})
			require.NoError(t, err)
			var rec recorder
			rec.install(r)

			err = r.CloneWithCredentialHelper(t.Context(), "https://github.com/owner/cfg", filepath.Join(t.TempDir(), "r"), tc.helper)

			require.ErrorIs(t, err, ErrRefused)
			assert.Empty(t, rec.commands, "git does not run")
		})
	}
}

// TestScopedHelperAnswersItsHostOnly runs the real git's credential
// lookup with the setting the clone carries: the helper answers the
// config repo's host, and git never asks it for another host or another
// port of that host, where hardened git, with no helper and no prompt,
// fails instead.
func TestScopedHelperAnswersItsHostOnly(t *testing.T) {
	r := newRunner(t, Host)
	asked := filepath.Join(t.TempDir(), "asked")
	helper := filepath.Join(t.TempDir(), "git-credential-test")
	script := "#!/bin/sh\necho \"$1\" >> '" + asked + "'\n[ \"$1\" = get ] && printf 'username=operator\\npassword=s3cret\\n'\nexit 0\n"
	require.NoError(t, os.WriteFile(helper, []byte(script), 0o700))
	s, err := scopedHelper("https://git.example.com/owner/cfg", helper)
	require.NoError(t, err)
	fill := func(host string) (string, error) {
		var out bytes.Buffer
		err := r.runInput(t.Context(), "/", []setting{s}, nil, strings.NewReader("protocol=https\nhost="+host+"\n\n"), &out, "credential", "fill")
		return out.String(), err
	}

	out, err := fill("git.example.com")
	require.NoError(t, err)
	assert.Contains(t, out, "password=s3cret")

	for _, other := range []string{"other.example.com", "git.example.com:8443", "example.com", "sub.git.example.com"} {
		_, err := fill(other)
		require.Error(t, err, other)
		assert.Contains(t, err.Error(), "terminal prompts disabled", other)
	}
	data, err := os.ReadFile(asked)
	require.NoError(t, err)
	assert.Equal(t, "get\n", string(data), "the helper was asked once, for its own host")
}

// TestScopedHelperRefusesAHostOutsideThePattern: a host the key cannot
// carry as it is (an =, a space, a bracket, an empty one) is refused, so
// the -c argument never names another key than the host's.
func TestScopedHelperRefusesAHostOutsideThePattern(t *testing.T) {
	for _, raw := range []string{
		"https://exa=mple.com/o/r", "https://exa%20mple.com/o/r", "https://[::1]/o/r", "https:///o/r",
		"https://u@example.com/o/r", "http://example.com/o/r", "git://127.0.0.1/o/r", "https://example.com.helper=x/o",
	} {
		_, err := scopedHelper(raw, "osxkeychain")
		require.ErrorIs(t, err, ErrRefused, raw)
	}
	s, err := scopedHelper("https://Git.Example.com:8443/o/r", "osxkeychain")
	require.NoError(t, err)
	assert.Equal(t, setting{"credential.https://git.example.com:8443.helper", "osxkeychain"}, s)
}

// TestCheckCredentialHelperFile: an absolute-path helper is a regular,
// executable file that the user or root owns and that neither the
// group nor others can write, in directories of the same kind up to /
// (a root-owned sticky one, as /tmp, admitted above the parent); a link
// is followed and both the link's path and the target's are checked. A
// helper name is exempt. Each rule refuses on its own.
func TestCheckCredentialHelperFile(t *testing.T) {
	uid := os.Getuid()
	t.Run("a helper name is exempt", func(t *testing.T) {
		require.NoError(t, checkHelperFile("osxkeychain", uid))
	})
	t.Run("a script of the user's", func(t *testing.T) {
		require.NoError(t, checkHelperFile(helperScript(t), uid))
	})
	t.Run("a link to a script of the user's", func(t *testing.T) {
		p := helperScript(t)
		link := filepath.Join(filepath.Dir(p), "link")
		require.NoError(t, os.Symlink(p, link))
		require.NoError(t, checkHelperFile(link, uid))
	})
	for _, tc := range []struct {
		name, rule string
		change     func(t *testing.T, p string) string
	}{
		{"missing", "cannot be resolved", func(t *testing.T, p string) string {
			t.Helper()
			return p + "-missing"
		}},
		{"a directory", "is not a regular file", func(t *testing.T, p string) string {
			t.Helper()
			return filepath.Dir(p)
		}},
		{"not executable", "is not executable", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(p, 0o600))
			return p
		}},
		{"writable by the group", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(p, 0o770))
			return p
		}},
		{"writable by others", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(p, 0o702))
			return p
		}},
		{"a parent writable by others", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(filepath.Dir(p), 0o707))
			return p
		}},
		{"a sticky parent writable by others", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(filepath.Dir(p), os.ModeSticky|0o777))
			return p
		}},
		{"a grandparent writable by the group", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(filepath.Dir(filepath.Dir(p)), 0o770))
			return p
		}},
		{"a link through a directory others can write", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			open := filepath.Join(t.TempDir(), "open")
			require.NoError(t, os.Mkdir(open, 0o700))
			require.NoError(t, os.Chmod(open, 0o777))
			link := filepath.Join(open, "link")
			require.NoError(t, os.Symlink(p, link))
			return link
		}},
		{"a target in a directory others can write", "is writable by its group or others", func(t *testing.T, p string) string {
			t.Helper()
			require.NoError(t, os.Chmod(filepath.Dir(p), 0o777))
			link := filepath.Join(t.TempDir(), "link")
			require.NoError(t, os.Symlink(p, link))
			return link
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.change(t, helperScript(t))

			err := checkHelperFile(p, uid)

			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), tc.rule)
		})
	}
	t.Run("owned by another user", func(t *testing.T) {
		p := helperScript(t)
		other := uid + 1
		if uid == 0 { // root owns what it makes: give the file away
			require.NoError(t, os.Chown(p, 65534, 65534))
			other = 0
		}

		err := checkHelperFile(p, other)

		if uid == 0 {
			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), "is owned by uid 65534, neither the user nor root")
			return
		}
		require.ErrorIs(t, err, ErrRefused)
		assert.Contains(t, err.Error(), fmt.Sprintf("is owned by uid %d, neither the user nor root", uid))
	})
}

// TestCheckCredentialHelperFileNamesTheFix: a write bit of the group, of
// others or of both is refused, a group that holds the user alone
// included (umask 002), and the message names the path and the chmod
// that fixes it.
func TestCheckCredentialHelperFileNamesTheFix(t *testing.T) {
	for mode, fix := range map[os.FileMode]string{0o770: "chmod g-w ", 0o707: "chmod o-w ", 0o777: "chmod go-w "} {
		t.Run(fix, func(t *testing.T) {
			p := helperScript(t)
			require.NoError(t, os.Chmod(p, mode))

			err := checkHelperFile(p, os.Getuid())

			require.ErrorIs(t, err, ErrRefused)
			assert.Contains(t, err.Error(), "fix it with: "+fix+p)
		})
	}
	t.Run("a directory", func(t *testing.T) {
		p := helperScript(t)
		require.NoError(t, os.Chmod(filepath.Dir(p), 0o770))

		err := checkHelperFile(p, os.Getuid())

		require.ErrorIs(t, err, ErrRefused)
		assert.Contains(t, err.Error(), "fix it with: chmod g-w "+filepath.Dir(p))
	})
}

// TestCloneMarksTheRemotesContent: once git clone has run, a HEAD that
// gitsafe refuses is an error of the remote's content, and the clone is
// removed; a refusal of the input, before git runs, is not.
func TestCloneMarksTheRemotesContent(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	record := r.start
	r.start = func(ctx context.Context, c procgroup.Command) error {
		if slices.Contains(c.Argv, "HEAD^{commit}") {
			_, err := io.WriteString(c.Stdout, "not-an-object-id\n")
			return err
		}
		if slices.Contains(c.Argv, "clone") {
			return os.MkdirAll(c.Argv[len(c.Argv)-1], 0o700)
		}
		return record(ctx, c)
	}
	dest := filepath.Join(t.TempDir(), "r")

	err = r.Clone(t.Context(), "https://github.com/owner/cfg", dest)

	require.ErrorIs(t, err, ErrRemoteContent)
	require.ErrorIs(t, err, ErrRefused)
	assert.NoDirExists(t, dest)

	err = r.CloneWithCredentialHelper(t.Context(), "https://github.com/owner/cfg", filepath.Join(t.TempDir(), "r"), "!sh")
	require.ErrorIs(t, err, ErrRefused)
	assert.NotErrorIs(t, err, ErrRemoteContent)
}
