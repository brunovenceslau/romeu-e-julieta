// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package gitsafe_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// remoteCorpus are values of memory.remote on both sides of the rule of
// internal/spec, and the seeds of FuzzMemoryRemoteParity.
var remoteCorpus = []string{
	"https://github.com/example/shop-memory", "https://github.com/example/shop-memory.git",
	"https://git.corp.internal:8443/a/b", "https://h", "https://h/", "git://127.0.0.1/m", "git://127.0.0.1:9418/m",
	"git://localhost/m", "git://example.com/m", "http://github.com/a/b", "file:///tmp/m", "/tmp/m", "m",
	"ssh://git@github.com/a/b", "git@github.com:a/b", "https://u@github.com/a/b", "https://u:p@github.com/a/b",
	"https://github.com/a b", "https://github.com/a\tb", "https://github.com/a\x7fb", "https://", "https:///a",
	"https:github.com/a", "HTTPS://github.com/a", "https://[::1]/a", "https://%zz/a", "", "none!",
}

// FuzzMemoryRemoteParity holds the rule memory-remote of internal/spec
// to CheckURL, the check of the URL Push and Fetch take, on one corpus:
// a URL one admits, the other admits; none is the opt-out, which is no
// URL.
func FuzzMemoryRemoteParity(f *testing.F) {
	for _, v := range remoteCorpus {
		f.Add(v)
	}
	base := spec.Project{Schema: "project/v1", Name: "shop", Agent: "a",
		Workload: "docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5",
		Repos:    []spec.Repo{{URL: "https://github.com/example/shop-api", Primary: true}}}
	require.NoError(f, base.Validate(), "the spec without memory is valid")
	f.Fuzz(func(t *testing.T, v string) {
		if v == spec.MemoryNone {
			return
		}
		p := base
		p.Memory = &spec.Memory{Remote: v}
		s := p.Validate() == nil
		g := gitsafe.CheckURL(v) == nil
		require.Equal(t, s, g, "%q: spec admits %v, gitsafe %v", v, s, g)
	})
}
