// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// TestReflogRealGit reads the reflog of a branch moved by commit, reset
// and amend, oldest first, and of HEAD; an entry whose commit is gone
// is left out; a ref with no reflog and a missing ref are empty lists.
func TestReflogRealGit(t *testing.T) {
	dir, one := newRepo(t)
	two := commit(t, dir, "two")
	plainGit(t, dir, "reset", "-q", "--hard", one)
	three := commit(t, dir, "three")
	plainGit(t, dir, "commit", "-q", "--amend", "--allow-empty", "-m", "three, amended")
	amended := plainGit(t, dir, "rev-parse", "HEAD")
	require.NotEqual(t, three, amended)
	plainGit(t, dir, "-c", "core.logAllRefUpdates=false", "update-ref", "refs/tags/nolog", one)
	check := noWrite(t, dir)
	r := newRunner(t, Sandbox)

	got, err := r.Reflog(t.Context(), dir, "refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, []string{one, two, one, three, amended}, got, "commit, commit, reset, commit, amend")
	head, err := r.Reflog(t.Context(), dir, "HEAD")
	require.NoError(t, err)
	assert.Equal(t, got, head)

	for _, ref := range []string{"refs/tags/nolog", "refs/heads/missing"} {
		got, err = r.Reflog(t.Context(), dir, ref)
		require.NoError(t, err)
		assert.Empty(t, got, ref)
	}
	check()

	// The commit two is reachable from the reflog alone: once it is
	// gone, its entry is left out.
	require.NoError(t, os.Remove(filepath.Join(dir, ".git", "objects", two[:2], two[2:])))
	got, err = r.Reflog(t.Context(), dir, "refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, []string{one, one, three, amended}, got)
}

// TestReflogRefuses: a name that is neither HEAD nor a full ref name,
// and one checkDisplayRef refuses, are refused before git starts; an
// answer over the bound or not an object id is refused.
func TestReflogRefuses(t *testing.T) {
	var rec recorder
	r := newRunner(t, Sandbox)
	rec.install(r)
	for _, ref := range []string{"", "main", "-v", "refs/heads/a..b", "refs/heads/a\u202eb", "HEAD@{1}", strings.Repeat("a", 40)} {
		_, err := r.Reflog(t.Context(), stubRepo, ref)
		require.ErrorIs(t, err, ErrRefused, "%q", ref)
	}
	assert.Empty(t, rec.commands)

	oid := strings.Repeat("a", 40)
	var commands []procgroup.Command
	answer := func(out string) func(context.Context, procgroup.Command) error {
		return func(_ context.Context, c procgroup.Command) error {
			commands = append(commands, c)
			if c.Stdout == nil {
				return nil // reflog exists
			}
			_, err := io.WriteString(c.Stdout, out)
			return err
		}
	}
	r.start = answer(oid + "\n" + oid + "\n")
	got, err := r.Reflog(t.Context(), stubRepo, "refs/heads/main")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	flags := 1 + 2*(len(r.settings)+len(historySettings))
	require.Len(t, commands, 2)
	assert.Equal(t, []string{"reflog", "exists", "refs/heads/main"}, commands[0].Argv[flags:])
	assert.Equal(t, []string{"log", "-g", "--format=%H", "--end-of-options", "refs/heads/main", "--"}, commands[1].Argv[flags:])
	for _, c := range commands {
		assert.Contains(t, c.Argv, "core.hooksPath=/dev/null")
		assert.Contains(t, c.Argv, "log.showSignature=false")
	}

	old := maxReflogEntries
	maxReflogEntries = 1
	_, err = r.Reflog(t.Context(), stubRepo, "refs/heads/main")
	maxReflogEntries = old
	require.ErrorIs(t, err, ErrRefused, "over the bound")

	r.start = answer("abc\n")
	_, err = r.Reflog(t.Context(), stubRepo, "refs/heads/main")
	require.ErrorIs(t, err, ErrRefused)
}
