// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// refsFixture makes a repository with two branches, a lightweight and
// an annotated tag, a note, two generations of refs/salvage/ and a
// remote-tracking ref, and returns it with its commits.
func refsFixture(t *testing.T) (dir, one, two string) {
	t.Helper()
	dir, one = newRepo(t)
	two = commit(t, dir, "two")
	plainGit(t, dir, "branch", "side", one)
	plainGit(t, dir, "tag", "light", one)
	plainGit(t, dir, "tag", "-a", "-m", "annotated", "heavy", two)
	plainGit(t, dir, "notes", "add", "-m", "a note", two)
	plainGit(t, dir, "update-ref", "refs/salvage/gen1/heads/main", one)
	plainGit(t, dir, "update-ref", "refs/salvage/gen1/heads/side", two)
	plainGit(t, dir, "update-ref", "refs/salvage/gen2/heads/main", two)
	plainGit(t, dir, "update-ref", "refs/remotes/origin/main", one)
	return dir, one, two
}

// TestListRefsRealGit lists the named namespaces only, sorted by name,
// with their object ids: the heads, the tags with the annotated tag's
// own object, the notes, and one generation of refs/salvage/ at a time.
func TestListRefsRealGit(t *testing.T) {
	dir, one, two := refsFixture(t)
	heavy := plainGit(t, dir, "rev-parse", "refs/tags/heavy")
	notes := plainGit(t, dir, "rev-parse", "refs/notes/commits")
	check := noWrite(t, dir)
	r := newRunner(t, Sandbox)

	got, _, err := r.ListRefs(t.Context(), dir, []string{"refs/tags/", "refs/heads/", "refs/notes/"})
	require.NoError(t, err)
	assert.Equal(t, []Ref{
		{Name: "refs/heads/main", OID: two},
		{Name: "refs/heads/side", OID: one},
		{Name: "refs/notes/commits", OID: notes},
		{Name: "refs/tags/heavy", OID: heavy},
		{Name: "refs/tags/light", OID: one},
	}, got)

	got, _, err = r.ListRefs(t.Context(), dir, []string{"refs/salvage/gen1/"})
	require.NoError(t, err)
	assert.Equal(t, []Ref{
		{Name: "refs/salvage/gen1/heads/main", OID: one},
		{Name: "refs/salvage/gen1/heads/side", OID: two},
	}, got, "one generation, not the other")

	got, _, err = r.ListRefs(t.Context(), dir, []string{"refs/salvage/gen3/"})
	require.NoError(t, err)
	assert.Empty(t, got)
	check()
}

// TestListRefsRefuses: a namespace outside the closed list, the whole
// of refs/salvage/, two generations in one call, a namespace named
// twice and no namespace are refused before git starts.
func TestListRefsRefuses(t *testing.T) {
	var rec recorder
	r := newRunner(t, Sandbox)
	rec.install(r)
	for _, ns := range [][]string{
		nil,
		{"refs/remotes/"},
		{"refs/"},
		{"refs/heads"},
		{"refs/hea"},
		{"refs/salvage/"},
		{"refs/salvage-reflog/"},
		{"refs/salvage/gen1"},
		{"refs/salvage/gen1/heads/"},
		{"refs/salvage/*/"},
		{"refs/salvage/../"},
		{"refs/salvage/gen1/", "refs/salvage/gen2/"},
		{"refs/heads/", "refs/heads/"},
	} {
		_, _, err := r.ListRefs(t.Context(), stubRepo, ns)
		require.ErrorIs(t, err, ErrRefused, "%q", ns)
	}
	assert.Empty(t, rec.commands)
}

// TestListRefsArgvAndBound pins the command, the namespaces as plain
// prefixes after --end-of-options, and refuses an answer over the
// bound, a name outside the namespaces asked for, and a short id.
func TestListRefsArgvAndBound(t *testing.T) {
	oid := strings.Repeat("a", 40)
	answer := func(out string, commands *[]procgroup.Command) func(context.Context, procgroup.Command) error {
		return func(_ context.Context, c procgroup.Command) error {
			*commands = append(*commands, c)
			_, err := io.WriteString(c.Stdout, out)
			return err
		}
	}
	var commands []procgroup.Command
	r := newRunner(t, Sandbox)
	r.start = answer(oid+" refs/heads/a\n"+oid+" refs/heads/b\n", &commands)
	got, _, err := r.ListRefs(t.Context(), stubRepo, []string{"refs/heads/", "refs/salvage/g/"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
	require.Len(t, commands, 1)
	flags := 1 + 2*(len(r.settings)+len(historySettings))
	assert.Equal(t, []string{"for-each-ref", "--format=%(objectname) %(refname)", "--end-of-options", "refs/heads/", "refs/salvage/g/"}, commands[0].Argv[flags:])
	assert.Contains(t, commands[0].Argv, "core.hooksPath=/dev/null")
	assert.Subset(t, commands[0].Env, []string{"GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1"})

	old := maxBranches
	maxBranches = 1
	t.Cleanup(func() { maxBranches = old })
	_, _, err = r.ListRefs(t.Context(), stubRepo, []string{"refs/heads/"})
	require.ErrorIs(t, err, ErrRefused, "over the bound")
	maxBranches = old

	for _, out := range []string{oid + " refs/tags/a\n", "abc refs/heads/a\n", oid + "refs/heads/a\n"} {
		r.start = answer(out, &commands)
		_, _, err = r.ListRefs(t.Context(), stubRepo, []string{"refs/heads/"})
		require.ErrorIs(t, err, ErrRefused, "%q", out)
	}
	assert.True(t, slices.IsSortedFunc(got, func(a, b Ref) int { return strings.Compare(a.Name, b.Name) }))
}

// TestListRefsReturnsTheNamesItRefuses: a name checkDisplayRef refuses
// is left out of the refs and returned, escaped, in the second list, so
// a caller keeps every other ref (decided while building T060); the
// refused names count against the bound too.
func TestListRefsReturnsTheNamesItRefuses(t *testing.T) {
	dir, one, two := refsFixture(t)
	plainGit(t, dir, "update-ref", "refs/tags/bad\u202etag", one)
	r := newRunner(t, Sandbox)
	got, refused, err := r.ListRefs(t.Context(), dir, []string{"refs/heads/", "refs/tags/"})
	require.NoError(t, err)
	assert.Equal(t, Refused{Names: []string{`refs/tags/bad\u202etag`}}, refused, "escaped")
	assert.Contains(t, got, Ref{Name: "refs/heads/main", OID: two})
	assert.Contains(t, got, Ref{Name: "refs/tags/light", OID: one})
	assert.Len(t, got, 4)

	old := maxBranches
	maxBranches = 4
	t.Cleanup(func() { maxBranches = old })
	_, _, err = r.ListRefs(t.Context(), dir, []string{"refs/heads/", "refs/tags/"})
	require.ErrorIs(t, err, ErrRefused, "the refused name is the fifth")
}

// TestListRefsBoundsTheNamesItRefuses: a refused name is clipped to
// maxRefusedName bytes before it is escaped, at most maxRefusedNames are
// kept and the rest counted, and one record over maxRefRecord bytes
// fails the list without git's output being read further.
func TestListRefsBoundsTheNamesItRefuses(t *testing.T) {
	oid := strings.Repeat("a", 40)
	answer := func(out string) func(context.Context, procgroup.Command) error {
		return func(_ context.Context, c procgroup.Command) error {
			_, err := io.WriteString(c.Stdout, out)
			return err
		}
	}
	r := newRunner(t, Sandbox)
	long := "refs/heads/\u202e" + strings.Repeat("x", 1000)
	var b strings.Builder
	b.WriteString(oid + " " + long + "\n")
	for i := range maxRefusedNames + 5 {
		b.WriteString(oid + " refs/heads/n\u202e" + strconv.Itoa(1000+i) + "\n")
	}
	b.WriteString(oid + " refs/heads/good\n")
	r.start = answer(b.String())
	got, refused, err := r.ListRefs(t.Context(), stubRepo, []string{"refs/heads/"})
	require.NoError(t, err)
	assert.Equal(t, []Ref{{Name: "refs/heads/good", OID: oid}}, got)
	require.Len(t, refused.Names, maxRefusedNames)
	assert.Equal(t, 6, refused.More)
	assert.Equal(t, `refs/heads/\u202e`+strings.Repeat("x", maxRefusedName-len("refs/heads/\u202e"))+"...", refused.Names[0], "clipped, then escaped")

	r.start = answer(oid + " refs/heads/" + strings.Repeat("y", maxRefRecord) + "\n")
	_, _, err = r.ListRefs(t.Context(), stubRepo, []string{"refs/heads/"})
	require.ErrorIs(t, err, ErrRefused, "a record over the bound")
}
