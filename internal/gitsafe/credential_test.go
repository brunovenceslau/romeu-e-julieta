// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"bytes"
	"context"
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

// TestCloneWithCredentialHelperArgv: the helper reaches the git clone
// command alone, as a credential.https://<host>.helper after the empty
// credential.helper of host mode, never as a helper for every host; the
// resolution of the URL, the read of HEAD, the listing of the filters
// and the checkout carry no helper.
func TestCloneWithCredentialHelperArgv(t *testing.T) {
	for name, helper := range map[string]string{"a name": "osxkeychain", "a path": "/opt/homebrew/bin/git-credential-gh"} {
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
		"a path over the cap":        {Host, "/" + strings.Repeat("a", maxHelper)},
		"a helper in sandbox mode":   {Sandbox, "osxkeychain"},
		"a name starting with a dot": {Host, ".hidden"},
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

	for _, other := range []string{"other.example.com", "git.example.com:8443", "example.com"} {
		_, err := fill(other)
		require.Error(t, err, other)
		assert.Contains(t, err.Error(), "terminal prompts disabled", other)
	}
	data, err := os.ReadFile(asked)
	require.NoError(t, err)
	assert.Equal(t, "get\n", string(data), "the helper was asked once, for its own host")
}
