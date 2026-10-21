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
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// userState is what BundleOf must leave as it found it in the user's
// repository: every ref, objects/info/alternates and the config, read
// byte for byte from the common git dir.
type userState struct {
	refs       string
	alternates []byte
	config     []byte
}

// userStateOf reads the user state of the repository whose common git
// dir is common, from the work tree dir.
func userStateOf(t *testing.T, dir, common string) userState {
	t.Helper()
	s := userState{refs: plainGit(t, dir, "for-each-ref", "--format=%(refname) %(objectname) %(symref)")}
	var err error
	s.alternates, err = os.ReadFile(filepath.Join(common, "objects", "info", "alternates"))
	if !errors.Is(err, os.ErrNotExist) {
		require.NoError(t, err)
	}
	s.config, err = os.ReadFile(filepath.Join(common, "config"))
	require.NoError(t, err)
	return s
}

// outDir returns a new directory for the bundles of a test, and a check
// that it holds the named files and nothing else: no temporary
// repository remains.
func outDir(t *testing.T) (string, func(names ...string)) {
	t.Helper()
	out := t.TempDir()
	return out, func(names ...string) {
		t.Helper()
		entries, err := os.ReadDir(out)
		require.NoError(t, err)
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		assert.ElementsMatch(t, names, got, "what the bundle directory holds")
	}
}

// bundleHeads returns the "<oid> <ref>" lines git bundle list-heads
// prints for file, after git bundle verify passes on it in dir.
func bundleHeads(t *testing.T, dir, file string) []string {
	t.Helper()
	plainGit(t, dir, "bundle", "verify", "-q", file)
	return strings.Split(plainGit(t, dir, "bundle", "list-heads", file), "\n")
}

// TestBundleOfRealGit bundles pairs of names and object ids, one of
// them a commit that no ref reaches (as a reflog entry is), minus a
// base, and writes no ref into the user's repository: git bundle verify
// passes, the bundle names exactly the pairs, and Unbundle into an
// empty repository that holds the base restores them.
func TestBundleOfRealGit(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			plainGit(t, dir, "init", "-q", "--object-format="+format)
			base := commitFile(t, dir, "a", "base\n", "base", "2026-02-01T00:00:00Z")
			kept := commitFile(t, dir, "a", "kept\n", "kept", "2026-02-02T00:00:00Z")
			lost := commitFile(t, dir, "a", "lost\n", "lost", "2026-02-03T00:00:00Z")
			plainGit(t, dir, "reset", "-q", "--hard", kept)
			assert.Empty(t, plainGit(t, dir, "branch", "--contains", lost), "no ref reaches lost")
			before := userStateOf(t, dir, filepath.Join(dir, ".git"))

			out, holds := outDir(t)
			file := filepath.Join(out, "snapshot.bundle")
			pairs := []Ref{
				{Name: "refs/heads/main", OID: kept},
				{Name: "refs/salvage-reflog/heads/main/1", OID: lost},
				{Name: "refs/salvage/gen1/worktrees/main/HEAD", OID: kept},
			}
			require.NoError(t, newRunner(t, Sandbox).BundleOf(t.Context(), dir, file, pairs, []string{base}))
			assert.Equal(t, before, userStateOf(t, dir, filepath.Join(dir, ".git")), "the user's repository is unchanged")
			holds("snapshot.bundle")

			assert.ElementsMatch(t, []string{
				kept + " refs/heads/main",
				lost + " refs/salvage-reflog/heads/main/1",
				kept + " refs/salvage/gen1/worktrees/main/HEAD",
			}, bundleHeads(t, dir, file))
			assert.Contains(t, plainGit(t, dir, "bundle", "verify", file), base, "the base is a prerequisite")

			restore := t.TempDir()
			plainGit(t, restore, "init", "-q", "--object-format="+format)
			_, err := plainGitErr(t, restore, nil, "bundle", "verify", file)
			require.Error(t, err, "the empty repository lacks the base")
			plainGit(t, restore, "fetch", "-q", dir, base+":refs/base")
			heads, err := newRunner(t, Sandbox).Unbundle(t.Context(), restore, file)
			require.NoError(t, err)
			got := make([]Ref, 0, len(heads))
			for _, h := range heads {
				got = append(got, Ref{Name: h.Ref, OID: h.OID})
				assert.Equal(t, "commit", plainGit(t, restore, "cat-file", "-t", h.OID), "the object is restored")
			}
			assert.ElementsMatch(t, pairs, got)
		})
	}
}

// TestBundleOfFromALinkedWorktree bundles from a linked worktree: the
// temporary repository borrows the objects of the common git dir, and
// a repository that already borrows objects through its own alternates
// is read through them and left as it was.
func TestBundleOfFromALinkedWorktree(t *testing.T) {
	lender, _ := newRepo(t)
	lent := commit(t, lender, "lent")
	dir := filepath.Join(t.TempDir(), "main")
	plainGit(t, filepath.Dir(dir), "clone", "-q", "--shared", lender, dir)
	linked := filepath.Join(t.TempDir(), "linked")
	plainGit(t, dir, "worktree", "add", "-q", "-b", "side", linked)
	side := commit(t, linked, "side")
	common := filepath.Join(dir, ".git")
	before := userStateOf(t, linked, common)
	require.NotEmpty(t, before.alternates, "the clone borrows the lender's objects")
	linkedGitFile, err := os.ReadFile(filepath.Join(linked, ".git"))
	require.NoError(t, err)

	out, holds := outDir(t)
	file := filepath.Join(out, "b.bundle")
	pairs := []Ref{{Name: "refs/heads/side", OID: side}, {Name: "refs/tags/lent", OID: lent}}
	require.NoError(t, newRunner(t, Sandbox).BundleOf(t.Context(), linked, file, pairs, nil))
	assert.Equal(t, before, userStateOf(t, linked, common))
	again, err := os.ReadFile(filepath.Join(linked, ".git"))
	require.NoError(t, err)
	assert.Equal(t, linkedGitFile, again)
	holds("b.bundle")
	assert.ElementsMatch(t, []string{side + " refs/heads/side", lent + " refs/tags/lent"}, bundleHeads(t, linked, file))
}

// TestBundleOfRemovesTheTemporaryRepository: a forced failure (a pair
// whose object does not exist, a base that does not), a failure of git
// bundle create itself and a context cancelled while git runs each
// return an error, leave the user's repository as it was and leave no
// temporary repository behind.
func TestBundleOfRemovesTheTemporaryRepository(t *testing.T) {
	dir, head := newRepo(t)
	before := userStateOf(t, dir, filepath.Join(dir, ".git"))
	missing := strings.Repeat("e", 40)
	cases := map[string]struct {
		pairs []Ref
		bases []string
		start func(cancel context.CancelFunc) func(context.Context, procgroup.Command) error
	}{
		"missing object": {pairs: []Ref{{Name: "refs/heads/main", OID: missing}}},
		"missing base":   {pairs: []Ref{{Name: "refs/heads/main", OID: head}}, bases: []string{missing}},
		"empty bundle":   {pairs: []Ref{{Name: "refs/heads/main", OID: head}}, bases: []string{head}},
		"cancelled": {
			pairs: []Ref{{Name: "refs/heads/main", OID: head}},
			start: func(cancel context.CancelFunc) func(context.Context, procgroup.Command) error {
				return func(ctx context.Context, c procgroup.Command) error {
					if slices.Contains(c.Argv, "bundle") {
						cancel() // as the caller's deadline would, while the bundle is being written
					}
					return procgroup.Run(ctx, c)
				}
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r := newRunner(t, Sandbox)
			if tc.start != nil {
				r.start = tc.start(cancel)
			}
			out, holds := outDir(t)
			err := r.BundleOf(ctx, dir, filepath.Join(out, "x.bundle"), tc.pairs, tc.bases)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrNoRefs)
			if name == "cancelled" {
				require.ErrorIs(t, err, context.Canceled, "the bundle was cut short by the cancellation")
			}
			holds()
			assert.Equal(t, before, userStateOf(t, dir, filepath.Join(dir, ".git")))
		})
	}
}

// TestBundleOfNoPair: no pair is an error of its own, ErrNoRefs, which
// the caller tells apart from git's, and nothing is made.
func TestBundleOfNoPair(t *testing.T) {
	var rec recorder
	r := newRunner(t, Sandbox)
	rec.install(r)
	out, holds := outDir(t)
	err := r.BundleOf(t.Context(), stubRepo, filepath.Join(out, "x.bundle"), nil, nil)
	require.ErrorIs(t, err, ErrNoRefs)
	require.NotErrorIs(t, err, ErrRefused)
	assert.Empty(t, rec.commands)
	holds()
}

// TestBundleOfRefuses: a name outside the five namespaces or one
// checkRefName refuses, a name given twice, an object id or base that
// is not a full id, and a bundle path that is not absolute are refused
// before git starts.
func TestBundleOfRefuses(t *testing.T) {
	var rec recorder
	r := newRunner(t, Sandbox)
	rec.install(r)
	oid := strings.Repeat("a", 40)
	out, holds := outDir(t)
	file := filepath.Join(out, "x.bundle")
	for _, tc := range []struct {
		file  string
		pairs []Ref
		bases []string
	}{
		{file, []Ref{{Name: "refs/remotes/origin/main", OID: oid}}, nil},
		{file, []Ref{{Name: "HEAD", OID: oid}}, nil},
		{file, []Ref{{Name: "refs/heads/", OID: oid}}, nil},
		{file, []Ref{{Name: "refs/salvage/", OID: oid}}, nil},
		{file, []Ref{{Name: "refs/heads/a..b", OID: oid}}, nil},
		{file, []Ref{{Name: "refs/heads/a", OID: oid}, {Name: "refs/heads/a", OID: oid}}, nil},
		{file, []Ref{{Name: "refs/heads/a", OID: "abc"}}, nil},
		{file, []Ref{{Name: "refs/heads/a", OID: oid}}, []string{"HEAD"}},
		{"x.bundle", []Ref{{Name: "refs/heads/a", OID: oid}}, nil},
	} {
		err := r.BundleOf(t.Context(), stubRepo, tc.file, tc.pairs, tc.bases)
		require.ErrorIs(t, err, ErrRefused, "%v", tc)
	}
	assert.Empty(t, rec.commands)
	holds()
}

// TestBundleOfArgv pins the commands: the object format and the common
// git dir read in the user's repository, then, in the temporary bare
// repository, its init with no template, the refs in one transaction
// and the bundle, each with the flags of the mode; and no GIT_*
// variable but those of the mode and of the reads.
func TestBundleOfArgv(t *testing.T) {
	var commands []procgroup.Command
	var transaction []byte
	r := newRunner(t, Sandbox)
	common := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(common, "objects"), 0o700))
	r.start = func(_ context.Context, c procgroup.Command) error {
		commands = append(commands, c)
		switch {
		case slices.Contains(c.Argv, "--show-object-format"):
			_, err := c.Stdout.Write([]byte("sha1\n" + common + "\n"))
			return err
		case slices.Contains(c.Argv, "init"):
			// What git init --bare makes, so the checks of a repository pass.
			for _, d := range []string{"objects/info", "refs"} {
				if err := os.MkdirAll(filepath.Join(c.Dir, d), 0o700); err != nil {
					return err
				}
			}
			return os.WriteFile(filepath.Join(c.Dir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600)
		case slices.Contains(c.Argv, "update-ref"):
			var err error
			transaction, err = io.ReadAll(c.Stdin)
			if err != nil {
				return err
			}
			alternates, err := os.ReadFile(filepath.Join(c.Dir, "objects", "info", "alternates"))
			if err != nil {
				return err
			}
			if string(alternates) != filepath.Join(common, "objects")+"\n" {
				return errors.New("alternates " + string(alternates))
			}
		}
		return nil
	}
	oid := strings.Repeat("a", 40)
	base := strings.Repeat("b", 40)
	out, holds := outDir(t)
	file := filepath.Join(out, "x.bundle")
	require.NoError(t, r.BundleOf(t.Context(), stubRepo, file, []Ref{{Name: "refs/heads/a", OID: oid}, {Name: "refs/tags/t", OID: oid}}, []string{base}))
	holds()
	require.Len(t, commands, 4)
	readFlags := 1 + 2*(len(r.settings)+len(historySettings))
	flags := 1 + 2*len(r.settings)
	assert.Equal(t, []string{"rev-parse", "--show-object-format", "--path-format=absolute", "--git-common-dir"}, commands[0].Argv[readFlags:])
	assert.Equal(t, stubRepo, commands[0].Dir)
	assert.Equal(t, []string{"init", "--bare", "--quiet", "--template=", "--object-format=sha1", "--ref-format=reftable"}, commands[1].Argv[flags:])
	assert.Equal(t, []string{"update-ref", "--stdin", "-z"}, commands[2].Argv[flags:])
	assert.Equal(t, "create refs/heads/a\x00"+oid+"\x00create refs/tags/t\x00"+oid+"\x00", string(transaction), "every ref in one transaction")
	assert.Equal(t, []string{"bundle", "create", file, "refs/heads/a", "refs/tags/t", "^" + base}, commands[3].Argv[flags:])
	tmp := commands[1].Dir
	assert.Equal(t, out, filepath.Dir(tmp), "the temporary repository is under the bundle's directory")
	for i, c := range commands {
		assert.Equal(t, []string{"-c", "core.hooksPath=/dev/null"}, c.Argv[1:3], "command %d", i)
		if i > 0 {
			assert.Equal(t, tmp, c.Dir, "command %d runs in the temporary repository", i)
		}
		for _, kv := range c.Env {
			if strings.HasPrefix(kv, "GIT_") {
				assert.Contains(t, slices.Concat(r.env, statusEnv), kv, "command %d", i)
			}
		}
	}
}

// TestBundleOfKeepsRefNamesOutOfTheFileSystem: the temporary repository
// stores its refs in a reftable, so no ref name the caller gives
// becomes a file or directory name: when git bundle create runs, the
// repository's ref format is reftable and refs/heads holds no entry.
func TestBundleOfKeepsRefNamesOutOfTheFileSystem(t *testing.T) {
	dir, head := newRepo(t)
	r := newRunner(t, Sandbox)
	var format string
	var entries []os.DirEntry
	r.start = func(ctx context.Context, c procgroup.Command) error {
		if slices.Contains(c.Argv, "bundle") {
			format = plainGit(t, c.Dir, "rev-parse", "--show-ref-format")
			// A reftable repository keeps refs/heads a plain file, which
			// git writes so an older git does not read the directory.
			if info, err := os.Stat(filepath.Join(c.Dir, "refs", "heads")); err == nil && info.IsDir() {
				if entries, err = os.ReadDir(filepath.Join(c.Dir, "refs", "heads")); err != nil {
					return err
				}
			}
		}
		return procgroup.Run(ctx, c)
	}
	out, holds := outDir(t)
	require.NoError(t, r.BundleOf(t.Context(), dir, filepath.Join(out, "x.bundle"), []Ref{{Name: "refs/heads/feat/a.b", OID: head}}, nil))
	holds("x.bundle")
	assert.Equal(t, "reftable", format)
	assert.Empty(t, entries, "no ref is a file")
}
