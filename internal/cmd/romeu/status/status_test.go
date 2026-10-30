// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"bytes"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

const (
	gen1 = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	gen2 = "01j9zc3a8x7w4t6y3r5e1v9k0q"
	slv1 = "01j9zd0a1b2c3d4e5f6g7h8j9k"
)

// world is the host of the cases: shop with a closed generation and an
// open one that a complete salvage closed before, blog with one adopted
// generation, closed lost.
func world(t *testing.T) map[string]*state.Project {
	t.Helper()
	closed := generation(t, gen1, state.GenerationClosedRemoved, "2026-10-01T09:00:00Z")
	closed.Salvage = []state.Salvage{salvage(t, slv1, state.SalvageComplete, "2026-10-01T09:30:00Z")}
	open := generation(t, gen2, state.GenerationOpen, "2026-10-02T09:14:07Z")
	shop := state.NewProject()
	shop.Generations = []state.Generation{closed, open}
	adopted := generation(t, gen1, state.GenerationClosedLost, "2026-10-03T10:00:00Z")
	adopted.Adopted = true
	adopted.CreatedWith = [32]byte{}
	adopted.Salvage = []state.Salvage{salvage(t, slv1, state.SalvageLost, "2026-10-03T10:30:00Z")}
	blog := state.NewProject()
	blog.Generations = []state.Generation{adopted}
	return map[string]*state.Project{"shop": shop, "blog": blog}
}

func decode(t *testing.T, out string) Report {
	t.Helper()
	var rep Report
	require.NoError(t, json.Unmarshal([]byte(out), &rep, json.RejectUnknownMembers(true)))
	return rep
}

func TestRunReportsTheHostStateOfEveryProject(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	sbx := fakeSbx{list: []sbxdrv.Sandbox{{Name: "shop", Status: "running"}, {Name: "other", Status: "running", Foreign: true}}}
	out, errOut, err := output(t, dir, sbx, true)
	require.NoError(t, err)
	assert.Empty(t, errOut)
	rep := decode(t, out)
	require.Len(t, rep.Projects, 2)
	blog, shop := rep.Projects[0], rep.Projects[1]
	assert.Equal(t, "blog", blog.Name, "projects in name order")
	assert.Equal(t, SandboxAbsent, blog.Sandbox)
	require.Len(t, blog.Generations, 1)
	assert.True(t, blog.Generations[0].Adopted)
	assert.Equal(t, "closed-lost", blog.Generations[0].State)
	assert.Equal(t, "lost", blog.Generations[0].Salvage[0].Result)
	assert.Equal(t, "running", shop.Sandbox)
	require.Len(t, shop.Generations, 2)
	assert.Equal(t, gen2, shop.Generations[1].ID, "generations in record order, the newest last")
	assert.Equal(t, "open", shop.Generations[1].State)
	assert.Nil(t, shop.Generations[1].ClosedAt)
	assert.Equal(t, "complete", shop.Generations[0].Salvage[0].Result)
	assert.Equal(t, "refs/romeu/salvage/shop/"+slv1+"/", shop.Generations[0].Salvage[0].RefsPrefix)
	assert.Empty(t, shop.Errors)
	assert.Empty(t, shop.Findings)
}

func TestRunText(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	out, _, err := output(t, dir, fakeSbx{list: []sbxdrv.Sandbox{{Name: "shop", Status: "stopped"}}}, false)
	require.NoError(t, err)
	assert.Equal(t, `blog
  sandbox: absent
  generation `+gen1+`: closed-lost (adopted, not gated), created 2026-10-03T10:00:00Z, closed 2026-10-03T11:00:00Z
    salvage `+slv1+`: lost, 2026-10-03T10:30:00Z, refs/romeu/salvage/shop/`+slv1+`/
shop
  sandbox: stopped
  generation `+gen1+`: closed-removed, created 2026-10-01T09:00:00Z, closed 2026-10-01T10:00:00Z
    salvage `+slv1+`: complete, 2026-10-01T09:30:00Z, refs/romeu/salvage/shop/`+slv1+`/
  generation `+gen2+`: open, created 2026-10-02T09:14:07Z
`, out)
}

func TestRunNamesOneProject(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	out, _, err := output(t, dir, fakeSbx{}, true, "shop")
	require.NoError(t, err)
	rep := decode(t, out)
	require.Len(t, rep.Projects, 1)
	assert.Equal(t, "shop", rep.Projects[0].Name)
}

func TestRunProjectWithoutRecord(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, nil, nil)
	out, _, err := output(t, dir, fakeSbx{}, true, "shop")
	require.NoError(t, err)
	rep := decode(t, out)
	require.Len(t, rep.Projects, 1)
	assert.Equal(t, SandboxAbsent, rep.Projects[0].Sandbox)
	assert.Empty(t, rep.Projects[0].Generations)
	assert.Empty(t, rep.Projects[0].Errors, "a project never synced has no record, which is no failed read")
}

func TestRunReportsRemovalPending(t *testing.T) {
	t.Parallel()
	removing := generation(t, gen2, state.GenerationRemoving, "2026-10-02T09:14:07Z")
	s := salvage(t, slv1, state.SalvageComplete, "2026-10-02T10:00:00Z")
	s.Fingerprint = [32]byte{3}
	removing.Salvage = []state.Salvage{s}
	p := state.NewProject()
	p.Generations = []state.Generation{removing}
	dir := stateDir(t, map[string]*state.Project{"shop": p}, nil)
	tests := []struct {
		name string
		list []sbxdrv.Sandbox
		want []Finding
	}{
		{"its sandbox present", []sbxdrv.Sandbox{{Name: "shop", Status: "stopped"}}, []Finding{{Slug: "removal-pending", Message: "generation " + gen2 + " is removing and its sandbox is present; run romeu rm shop"}}},
		{"its sandbox gone", nil, []Finding{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, _, err := output(t, dir, fakeSbx{list: tt.list}, true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, decode(t, out).Projects[0].Findings)
		})
	}
}

func TestRunGoesOnPastAFailedRead(t *testing.T) {
	t.Parallel()
	records := world(t)
	delete(records, "blog")
	dir := stateDir(t, records, map[string]string{"blog": `{"schema": "state-project/v1", "generations": 7}`})
	sbx := fakeSbx{err: sbxdrv.SbxUnknown.New("sbx ls --json failed, so it is not known whether a sandbox exists")}
	tests := []struct {
		name    string
		jsonOut bool
	}{{"json", true}, {"text", false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, _, err := output(t, dir, sbx, tt.jsonOut)
			require.Error(t, err)
			assert.Equal(t, StatusIncomplete.ID, cli.Of(err).ID)
			assert.Equal(t, 1, cli.ExitCode(err), "a failed read exits 1, never 3")
			if !tt.jsonOut {
				assert.Contains(t, out, "  error RJ-334 record-unreadable (project record): ")
				assert.Contains(t, out, "  error RJ-308 sbx-unknown (sbx ls): sbx ls --json failed")
				assert.Contains(t, out, "  generation "+gen2+": open", "the record that reads is still reported")
				return
			}
			rep := decode(t, out)
			require.Len(t, rep.Projects, 2)
			blog, shop := rep.Projects[0], rep.Projects[1]
			require.Len(t, blog.Errors, 2)
			assert.Equal(t, ReadError{ID: "RJ-334", Slug: "record-unreadable", Read: "project record", Message: blog.Errors[0].Message}, blog.Errors[0])
			assert.Equal(t, "sbx ls", blog.Errors[1].Read)
			assert.Equal(t, SandboxUnknown, shop.Sandbox)
			require.Len(t, shop.Errors, 1)
			assert.Equal(t, "RJ-308", shop.Errors[0].ID)
			assert.Len(t, shop.Generations, 2)
		})
	}
}

func TestRunNoHostState(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out)}
	require.NoError(t, run(t.Context(), env, host{sbx: fakeSbx{}}, nil))
	assert.Equal(t, "no host state: run romeu init, then romeu sync\n", out.String())
}
