// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// TestSizedManifestIsTheManifestOfRun: the manifest the size rule
// measures holds what romeu run passes, each member sync cannot know
// in its longest form: the primary at its host clone, a secondary under
// the workload user's home (REQ-0221), the branch of each egress ref,
// one base per repo, every gated domain as approved, and the digest of
// the run layout.
func TestSizedManifestIsTheManifestOfRun(t *testing.T) {
	h := treeRun(t)
	require.NoError(t, makeTree(t.Context(), h.r))
	p := h.r.targets[0]
	p.Agent = "sbx-kit-claude"
	eg := egress.Result{Auto: []string{"a.example"}, Gated: []egress.Gate{{Domain: "b.example", Reason: egress.ReasonUpload}}}

	m, err := sizedManifest(p, h.r.trees[0], eg, "ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12")

	require.NoError(t, err)
	require.Len(t, m.Repos, 2)
	api, web := m.Repos[0], m.Repos[1]
	assert.Equal(t, manifest.Repo{Base: []string{h.r.trees[0].repos[0].egressCommit}, Dir: "shop-api", Memory: h.in("memory/shop-api"),
		Path: h.in("shop-api"), Primary: true, Ref: "main", URL: apiURL}, api)
	assert.Equal(t, "/home/agent/src/shop/shop-web", web.Path)
	assert.Equal(t, "trunk", web.Ref, "the origin's HEAD, as step 4 resolved it")
	assert.Equal(t, []string{"a.example", "b.example"}, m.Egress)
	// The encoded manifest decodes, so its size is the size of a
	// manifest julieta reads.
	value, err := manifest.EncodeEnv(m)
	require.NoError(t, err)
	_, err = manifest.DecodeEnv(value)
	require.NoError(t, err)
}
