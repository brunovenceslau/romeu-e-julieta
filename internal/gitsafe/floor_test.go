// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestFloor holds the initial floor of 05 5.1.
func TestFloor(t *testing.T) {
	assert.Equal(t, "2.45.4", Floor())
}

// TestParseVersion reads the first line of git version as the git
// builds print it.
func TestParseVersion(t *testing.T) {
	tests := []struct {
		out  string
		want version
		ok   bool
	}{
		{"git version 2.53.0\n", version{2, 53, 0, false}, true},
		{"git version 2.45.4", version{2, 45, 4, false}, true},
		{"git version 2.39.5 (Apple Git-154)\n", version{2, 39, 5, false}, true},
		{"git version 2.45.4.windows.1\n", version{2, 45, 4, false}, true},
		{"git version 2.46.0.rc1\n", version{2, 46, 0, true}, true},
		{"git version 2.46.0-rc1\n", version{2, 46, 0, true}, true},
		{"git version 2.45\n", version{}, false},
		{"git version 2.x.4\n", version{}, false},
		{"git version 2.45.-1\n", version{}, false},
		{"git version 99999999999999999999.0.0\n", version{}, false},
		{"hub version 2.45.4\n", version{}, false},
		{"", version{}, false},
	}
	for _, tt := range tests {
		got, err := parseVersion(tt.out)
		if !tt.ok {
			require.Error(t, err, "%q", tt.out)
			continue
		}
		require.NoError(t, err, "%q", tt.out)
		assert.Equal(t, tt.want, got, "%q", tt.out)
	}
}

// TestCheckFloor: a git below the floor, or one whose version cannot
// be read, is the error git-floor (RJ-344, exit 2); at or above it
// passes.
func TestCheckFloor(t *testing.T) {
	tests := []struct {
		out string
		ok  bool
	}{
		{"git version 2.45.4\n", true},
		{"git version 2.53.0\n", true},
		{"git version 3.0.0\n", true},
		{"git version 2.45.5\n", true},
		{"git version 2.46.0\n", true},
		{"git version 2.45.3\n", false},
		{"git version 2.44.9\n", false},
		{"git version 1.99.99\n", false},
		{"git version 2.45.4.rc0\n", false},
		{"git version 2.39.5 (Apple Git-154)\n", false},
		{"not git\n", false},
	}
	for _, tt := range tests {
		r, err := New(Config{Mode: Host, Git: "/usr/bin/git", Home: "/h"})
		require.NoError(t, err)
		r.start = func(_ context.Context, c procgroup.Command) error {
			assert.Equal(t, "version", c.Argv[len(c.Argv)-1])
			_, err := io.WriteString(c.Stdout, tt.out)
			return err
		}
		err = r.CheckFloor(t.Context())
		if tt.ok {
			require.NoError(t, err, tt.out)
			continue
		}
		floor, ok := errors.AsType[*FloorError](err)
		require.True(t, ok, "%q: %v", tt.out, err)
		assert.Equal(t, "RJ-344", floor.ID())
		assert.Equal(t, "git-floor", floor.Slug())
		assert.Equal(t, 2, floor.ExitCode())
		assert.Equal(t, "decide: install git at or above the floor of 05 5.1", floor.Fix())
		assert.Contains(t, floor.Details(), "2.45.4")
		assert.Contains(t, floor.Error(), "below the floor 2.45.4")
	}
}

// TestCheckFloorRealGit runs the floor check on the git of the test
// process; whether it passes depends on that git, so the test asserts
// only that the check ran and answered in one of its two ways.
func TestCheckFloorRealGit(t *testing.T) {
	err := newRunner(t, Host).CheckFloor(t.Context())
	if err != nil {
		_, ok := errors.AsType[*FloorError](err)
		assert.True(t, ok, "%v", err)
	}
}

// TestCheckFloorRunError: a git that does not run is not a floor
// finding.
func TestCheckFloorRunError(t *testing.T) {
	r, err := New(Config{Mode: Host, Git: "/nonexistent/git", Home: t.TempDir()})
	require.NoError(t, err)
	err = r.CheckFloor(t.Context())
	require.ErrorIs(t, err, procgroup.ErrNotFound)
	_, ok := errors.AsType[*FloorError](err)
	assert.False(t, ok)
}

// fakeGit writes an executable script that stands in for git and
// returns its path.
func fakeGit(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "git")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o700))
	return path
}

// TestGitOutputInErrorsIsEscaped: what git writes, part of which a
// server controls, reaches an error only escaped (I29), and the
// version a FloorError quotes is its first line, cut short.
func TestGitOutputInErrorsIsEscaped(t *testing.T) {
	t.Run("standard error", func(t *testing.T) {
		git := fakeGit(t, `printf 'remote: \033[2J\033]0;title\007\nfatal: \233bad\n' >&2; exit 128`+"\n")
		r, err := New(Config{Mode: Host, Git: git, Home: t.TempDir()})
		require.NoError(t, err)
		err = r.CheckFloor(t.Context())
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "\x1b")
		assert.NotContains(t, err.Error(), "\a")
		assert.NotContains(t, err.Error(), "\x9b")
		assert.Contains(t, err.Error(), `remote: \u001b[2J`)
		assert.Contains(t, err.Error(), `fatal: \x9bbad`)
	})
	t.Run("an unreadable version", func(t *testing.T) {
		git := fakeGit(t, `printf 'git version \033[31mbanana\n'; i=0; while [ $i -lt 2000 ]; do echo filler; i=$((i+1)); done`+"\n")
		r, err := New(Config{Mode: Host, Git: git, Home: t.TempDir()})
		require.NoError(t, err)
		floorErr, ok := errors.AsType[*FloorError](r.CheckFloor(t.Context()))
		require.True(t, ok)
		assert.Equal(t, `git version \u001b[31mbanana`, floorErr.Found)
	})
}
