// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckURL is the URL table of 05 5.1: HTTPS anywhere, git:// on
// 127.0.0.1 only, and nothing else.
func TestCheckURL(t *testing.T) {
	accepted := []string{
		"https://github.com/o/r.git",
		"https://github.com/o/r",
		"https://127.0.0.1:8443/o/r.git",
		"git://127.0.0.1:9418/shop",
		"git://127.0.0.1/shop",
	}
	for _, u := range accepted {
		require.NoError(t, CheckURL(u), u)
	}
	refused := []struct{ name, url string }{
		{"git on another host", "git://example.com/o/r"},
		{"git on localhost", "git://localhost:9418/o/r"},
		{"git on the ipv6 loopback", "git://[::1]:9418/o/r"},
		{"git on a lookalike", "git://127.0.0.1.example.com/o/r"},
		{"git with userinfo", "git://u@127.0.0.1/o/r"},
		{"file", "file:///srv/o/r.git"},
		{"file with a host", "file://localhost/srv/o/r.git"},
		{"absolute path", "/srv/o/r.git"},
		{"relative path", "../r.git"},
		{"dot", "."},
		{"empty", ""},
		{"plain http", "http://github.com/o/r.git"},
		{"ssh", "ssh://git@github.com/o/r.git"},
		{"scp-like", "git@github.com:o/r.git"},
		{"transport helper", "ext::sh -c touch% /tmp/x"},
		{"fd helper", "fd::3"},
		{"upper-case scheme", "HTTPS://github.com/o/r.git"},
		{"https without a host", "https:///o/r.git"},
		{"https with userinfo", "https://token@github.com/o/r.git"},
		{"option", "--upload-pack=touch /tmp/x"},
		{"control character", "https://github.com/o/r\n.git"},
		{"space", "https://github.com/o /r.git"},
		{"unparsable", "https://github.com/%zz"},
		{"opaque", "https:github.com/o/r"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, CheckURL(tt.url), ErrRefused)
		})
	}
}

// TestFetchRefusesURLWithCAFile: a caFile changes nothing about which
// URLs gitsafe refuses (05 5.1, REQ-0467).
func TestFetchRefusesURLWithCAFile(t *testing.T) {
	r, err := New(Config{
		Mode: Host, Git: "/usr/bin/git", Home: "/h",
		GitHosts: []GitHost{{Host: "localhost", CAFile: "/ca.pem"}, {Host: "example.com", CAFile: "/ca.pem"}},
	})
	require.NoError(t, err)
	var rec recorder
	rec.install(r)
	for _, u := range []string{"git://example.com/o/r", "file:///srv/r.git", "file://localhost/srv/r.git", "/srv/r.git", "r.git"} {
		err := r.Fetch(t.Context(), stubRepo, u, []string{"refs/heads/main:refs/romeu/origin/r/main"})
		require.ErrorIs(t, err, ErrRefused, u)
	}
	assert.Empty(t, rec.commands, "a refused URL runs nothing")
}

// TestInsteadOf covers the one key gitsafe does not neutralize (05
// 5.1): a local url.<base>.insteadOf that rewrites to a refused
// protocol, or to git://127.0.0.1 for a URL that was not, is refused;
// one to another HTTPS host is not.
func TestInsteadOf(t *testing.T) {
	tests := []struct {
		name    string
		base    string // the rewrite target
		from    string // what it rewrites
		url     string // the fetch URL
		refused bool
	}{
		{"to file", "file:///srv/", "https://github.com/", "https://github.com/o/r.git", true},
		{"to a plain path", "/srv/", "https://github.com/", "https://github.com/o/r.git", true},
		{"to ssh", "ssh://git@github.com/", "https://github.com/", "https://github.com/o/r.git", true},
		{"to git on another host", "git://evil.example/", "https://github.com/", "https://github.com/o/r.git", true},
		{"to the loopback daemon", "git://127.0.0.1:9418/", "https://github.com/", "https://github.com/o/r.git", true},
		{"to another https host", "https://mirror.example/", "https://github.com/", "https://github.com/o/r.git", false},
		{"loopback to loopback", "git://127.0.0.1:9419/", "git://127.0.0.1:9418/", "git://127.0.0.1:9418/r", false},
		{"no match", "file:///srv/", "https://other.example/", "https://github.com/o/r.git", false},
	}
	for _, mode := range []Mode{Host, Sandbox} {
		for _, tt := range tests {
			t.Run(mode.String()+"/"+tt.name, func(t *testing.T) {
				dir, _ := newRepo(t)
				plainGit(t, dir, "config", "url."+tt.base+".insteadOf", tt.from)
				r := newRunner(t, mode)
				got, err := r.resolveURL(t.Context(), dir, tt.url)
				if tt.refused {
					require.ErrorIs(t, err, ErrRefused)
					return
				}
				require.NoError(t, err)
				assert.NotEmpty(t, got)
			})
		}
	}
}

// TestInsteadOfStopsFetch: a refused rewrite stops the fetch before it
// starts.
func TestInsteadOfStopsFetch(t *testing.T) {
	dir, _ := newRepo(t)
	plainGit(t, dir, "config", "url.file:///srv/.insteadOf", "https://github.com/")
	err := newRunner(t, Host).Fetch(t.Context(), dir, "https://github.com/o/r.git", []string{"refs/heads/main:refs/romeu/origin/r/main"})
	require.ErrorIs(t, err, ErrRefused)
	assert.Contains(t, err.Error(), "file:///srv/o/r.git")
}
