// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestVersion: git version is read with the hardened argv and
// environment of host mode, and its first line is parsed strictly: a
// version line gives git's own token, escaped and cut, and anything
// else is the error git-floor (RJ-344), whatever the floor says of the
// version.
func TestVersion(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "a release", out: "git version 2.53.0\n", want: "2.53.0"},
		{name: "below the floor is still a version, the suffix after the space left out", out: "git version 2.39.5 (Apple Git-154)\n", want: "2.39.5"},
		{name: "a build suffix is git's own token", out: "git version 2.45.4.windows.1\n", want: "2.45.4.windows.1"},
		{name: "a release candidate", out: "git version 2.46.0.rc1\n", want: "2.46.0.rc1"},
		{name: "a release candidate with a hyphen", out: "git version 2.46.0-rc1\n", want: "2.46.0-rc1"},
		{name: "a suffix of I29's code points is escaped", out: "git version 2.53.0.\u202e\u3164\x9bx\n", want: `2.53.0.\u202e\u3164\x9bx`},
		{name: "a long suffix is cut", out: "git version 2.53.0." + strings.Repeat("a", 300) + "\n", want: "2.53.0." + strings.Repeat("a", foundLimit-len("2.53.0."))},
		{name: "two fields", out: "git version 2.45\n"},
		{name: "another program", out: "hub version 2.45.4\n"},
		{name: "an escape in the line", out: "git version \x1b[31m2.53.0\n"},
		{name: "nothing", out: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
			require.NoError(t, err)
			var ran procgroup.Command
			r.start = func(_ context.Context, c procgroup.Command) error {
				ran = c
				_, err := io.WriteString(c.Stdout, tt.out)
				return err
			}
			got, err := r.Version(t.Context())
			assert.Equal(t, "/usr/bin/git", ran.Argv[0])
			assert.Equal(t, "version", ran.Argv[len(ran.Argv)-1])
			assert.Equal(t, procgroup.HostGitFetch, ran.Class, "the class of every git call of host mode")
			assert.Contains(t, ran.Env, "GIT_CONFIG_GLOBAL=/dev/null")
			assert.Contains(t, ran.Env, "PATH=/usr/bin:/bin")
			assert.Nil(t, ran.Stdin, "git reads no input")
			if tt.want != "" {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			floor, ok := errors.AsType[*FloorError](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, "RJ-344", floor.ID())
			assert.Empty(t, got)
			assert.False(t, strings.ContainsRune(floor.Found, '\x1b'), "the output is quoted escaped: %q", floor.Found)
		})
	}
}

// TestVersionRunError: a git that does not run is the error of the run,
// not git-floor, as CheckFloor reports it.
func TestVersionRunError(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/nonexistent/git", Home: t.TempDir()})
	require.NoError(t, err)
	got, err := r.Version(t.Context())
	require.ErrorIs(t, err, procgroup.ErrNotFound)
	_, ok := errors.AsType[*FloorError](err)
	assert.False(t, ok)
	assert.Empty(t, got)
}

// TestVersionOutputBound: an output past the bound of gitsafe's reads
// is an error, never a version read from a cut output.
func TestVersionOutputBound(t *testing.T) {
	git := fakeGit(t, `echo 'git version 2.53.0'; i=0; while [ $i -lt 20000 ]; do echo filler-filler; i=$((i+1)); done`+"\n")
	r, err := New(Config{Mode: Host, Git: git, Home: t.TempDir()})
	require.NoError(t, err)
	got, err := r.Version(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output longer than")
	assert.Empty(t, got)
}

// TestVersionRealGit reads the version of the git of the test process.
func TestVersionRealGit(t *testing.T) {
	got, err := newRunner(t, Host).Version(t.Context())
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9]+\.[0-9]+\.[0-9]+[0-9A-Za-z.-]*$`, got)
}
