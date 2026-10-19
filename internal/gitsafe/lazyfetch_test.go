// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lazyCommits are the commits of the lazy fixture, the same in every
// repository that builds it: head names blob a and
// moved has a tree that names blobs a and b.
type lazyCommits struct {
	head, moved string
	a, b, tree  string
}

// lazyRepo is a repository whose own config names a promisor remote at
// a loopback listener, and whose commits name blobs that are not in the
// object store. It returns the repository, its commits and the count of
// connections the listener has seen.
func lazyRepo(t *testing.T) (dir string, c lazyCommits, conns *atomic.Int32) {
	t.Helper()
	conns = new(atomic.Int32)
	prefix := listen(t, func(_ context.Context, _ net.Conn) { conns.Add(1) })
	dir, base := newRepo(t)
	c = lazyObjects(t, dir, base)
	for _, blob := range []string{c.a, c.b, c.tree} {
		require.NoError(t, os.Remove(filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])))
	}
	plainGit(t, dir, "update-ref", "refs/heads/main", c.head)
	plainGit(t, dir, "config", "core.repositoryformatversion", "1")
	plainGit(t, dir, "config", "extensions.partialclone", "origin")
	plainGit(t, dir, "config", "remote.origin.promisor", "true")
	plainGit(t, dir, "config", "remote.origin.url", prefix+"x")
	return dir, c, conns
}

// lazyObjects stores the blobs, trees and commits of the fixture on top
// of base. Fixture identities and dates are fixed, so the ids are the
// same in every repository that holds base.
func lazyObjects(t *testing.T, dir, base string) lazyCommits {
	t.Helper()
	var c lazyCommits
	c.a = writeBlob(t, dir, "a\n")
	c.b = writeBlob(t, dir, "b\n")
	one := writeTree(t, dir, rawEntry{"100644", "a", c.a})
	two := writeTree(t, dir, rawEntry{"100644", "a", c.a}, rawEntry{"100644", "b", c.b})
	c.tree = two
	c.head = plainGit(t, dir, "commit-tree", one, "-p", base, "-m", "head")
	c.moved = plainGit(t, dir, "commit-tree", two, "-p", c.head, "-m", "moved")
	return c
}

// lazyCall is what a verb of TestNoLazyFetch runs with.
type lazyCall struct {
	t   *testing.T
	r   *Runner
	dir string
	c   lazyCommits
}

// TestNoLazyFetch: a promisor remote the repository's own config names
// is never contacted when a verb meets a missing object (05 5.1); git
// would fetch the object from it, past CheckURL and the allowlist. Plain
// git on the same repository does connect (the positive control).
func TestNoLazyFetch(t *testing.T) {
	t.Run("control", func(t *testing.T) {
		dir, _, conns := lazyRepo(t)
		_, err := plainGitErr(t, dir, nil, "bundle", "create", filepath.Join(t.TempDir(), "x.bundle"), "refs/heads/main")
		require.Error(t, err)
		assert.Eventually(t, func() bool { return conns.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
			"the config is live: plain git connects")
	})

	// Only the verbs that read a missing object in this fixture are
	// here; the others (Fsck, BundleVerify, Unbundle, CreateRef) never
	// do, and TestRunnerEnvironmentAndFlags pins the environment every
	// verb runs with. FastForward is sandbox only: host mode refuses it.
	verbs := []struct {
		name  string
		modes []Mode
		run   func(a lazyCall) error
	}{
		{"WalkTree", []Mode{Host, Sandbox}, func(a lazyCall) error {
			_, err := a.r.WalkTree(a.t.Context(), a.dir, a.c.moved)
			return err
		}},
		{"BundleCreate", []Mode{Host, Sandbox}, func(a lazyCall) error {
			return a.r.BundleCreate(a.t.Context(), a.dir, filepath.Join(a.t.TempDir(), "x.bundle"), []string{"refs/heads/main"}, nil)
		}},
		{"FastForward", []Mode{Sandbox}, func(a lazyCall) error {
			return a.r.FastForward(a.t.Context(), a.dir, a.c.moved)
		}},
	}
	for _, v := range verbs {
		for _, mode := range v.modes {
			t.Run(v.name+"/"+mode.String(), func(t *testing.T) {
				dir, c, conns := lazyRepo(t)
				plainGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
				_ = v.run(lazyCall{t, newRunner(t, mode), dir, c})
				// git connects before the listener counts, so give a
				// connection time to show.
				assert.Never(t, func() bool { return conns.Load() > 0 }, 200*time.Millisecond, 10*time.Millisecond,
					"no connection to the promisor remote")
			})
		}
	}
}
