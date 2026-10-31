// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// project returns a record of the project shop with one generation in
// the state st.
func project(t *testing.T, st state.State) *state.Project {
	t.Helper()
	id, err := canon.ParseULID("01j9zc2q8x7w4t6y3r5e1v9k0p")
	require.NoError(t, err)
	g := state.Generation{
		ID:            id,
		State:         st,
		CreatedAt:     time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
		CreatedWith:   canon.Sum[canon.RecreateKind]{1},
		WorkspacePath: "/root/shop-env/shop-api",
		Repos:         []state.GenerationRepo{{Dir: "shop-api", URL: "https://github.com/example/shop-api", Primary: true}},
	}
	if st == state.GenerationClosedRemoved || st == state.GenerationClosedLost {
		g.ClosedAt = g.CreatedAt.Add(time.Hour)
	}
	if st == state.GenerationRemoving {
		// A removing generation's newest salvage carries its fingerprint
		// (REQ-0236).
		sid, err := canon.ParseULID("01j9zd0a1b2c3d4e5f6g7h8j9k")
		require.NoError(t, err)
		g.Salvage = []state.Salvage{{
			ID: sid, CreatedAt: g.CreatedAt.Add(time.Minute), ManifestSHA256: state.SHA256{2},
			Result: state.SalvageComplete, RefsPrefix: "refs/romeu/salvage/shop/01j9zd0a1b2c3d4e5f6g7h8j9k/",
			Fingerprint: canon.Sum[canon.FingerprintKind]{3},
		}}
	}
	p := state.NewProject()
	p.Generations = []state.Generation{g}
	return p
}

// shopSandbox is the sandbox of the project shop as sbx lists it.
var shopSandbox = sbxdrv.Sandbox{Name: "shop", Status: "running", Workspaces: []string{"/root/shop-env/shop-api"}}

// TestDoctorGenerationSandbox: an open generation with no sandbox, or a
// sandbox of a project's name whose record has no generation in open,
// salvaging or removing, fails generation-sandbox (REQ-0389).
func TestDoctorGenerationSandbox(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		st        state.State
		sandboxes []sbxdrv.Sandbox
		want      string
	}{
		{"an open generation with its sandbox", state.GenerationOpen, []sbxdrv.Sandbox{shopSandbox}, resultPass},
		{"an open generation with no sandbox", state.GenerationOpen, nil, resultFail},
		{"a sandbox and only a closed generation", state.GenerationClosedRemoved, []sbxdrv.Sandbox{shopSandbox}, resultFail},
		{"a sandbox of another name is none of romeu's", state.GenerationClosedRemoved, []sbxdrv.Sandbox{{Name: "x", Foreign: true}}, resultPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			f.writeProject(t, "shop", project(t, tc.st))
			f.sbx.sandboxes = tc.sandboxes

			rep, out, _ := f.output(t)

			assert.Equal(t, tc.want, check(t, rep, "generation-sandbox").Result, out)
		})
	}
}
