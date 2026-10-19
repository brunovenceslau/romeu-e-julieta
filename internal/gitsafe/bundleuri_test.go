// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoBundleURI: a fetch.bundleURI the repository's own config names
// is never contacted by Fetch, whose URL passed CheckURL (05 5.1). Plain
// git on the same repository does connect (the positive control).
func TestNoBundleURI(t *testing.T) {
	src, _ := newRepo(t)
	url := serveGit(t, bareCopy(t, src, "repo.git")) + "repo.git"
	refspec := "+refs/heads/*:refs/remotes/origin/*"

	hostile := func(t *testing.T) (dir string, conns *atomic.Int32) {
		t.Helper()
		conns = new(atomic.Int32)
		prefix := listen(t, func(_ context.Context, _ net.Conn) { conns.Add(1) })
		dir, _ = newRepo(t)
		plainGit(t, dir, "config", "fetch.bundleURI", "https://"+strings.TrimPrefix(prefix, "git://")+"b")
		return dir, conns
	}

	t.Run("control", func(t *testing.T) {
		dir, conns := hostile(t)
		_, _ = plainGitErr(t, dir, nil, "fetch", "-q", url, refspec)
		assert.Eventually(t, func() bool { return conns.Load() > 0 }, 5*time.Second, 10*time.Millisecond,
			"the config is live: plain git connects")
	})
	for _, mode := range []Mode{Host, Sandbox} {
		t.Run(mode.String(), func(t *testing.T) {
			dir, conns := hostile(t)
			dst := "refs/remotes/origin/"
			if mode == Host {
				dst = "refs/sandboxes/shop/"
			}
			require.NoError(t, newRunner(t, mode).Fetch(t.Context(), dir, url, []string{"+refs/heads/*:" + dst + "*"}))
			// git connects before the listener counts, so give a
			// connection time to show.
			assert.Never(t, func() bool { return conns.Load() > 0 }, 200*time.Millisecond, 10*time.Millisecond,
				"no connection to the bundle URI")
		})
	}
}
