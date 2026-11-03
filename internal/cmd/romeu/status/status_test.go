// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
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
	assert.Equal(t, RecordPresent, shop.Record)
	assert.Equal(t, "running", shop.Sandbox)
	require.Len(t, shop.Generations, 2)
	assert.Equal(t, gen2, shop.Generations[1].ID, "generations in record order, the newest last")
	assert.Equal(t, "open", shop.Generations[1].State)
	assert.True(t, shop.Generations[1].ClosedAt.IsZero())
	var raw struct {
		Projects []struct {
			Generations []map[string]any `json:"generations"`
		} `json:"projects"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &raw))
	assert.NotContains(t, raw.Projects[1].Generations[1], "closedAt", "an open generation has no closedAt")
	assert.Contains(t, raw.Projects[1].Generations[0], "closedAt")
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
	assert.Equal(t, RecordAbsent, rep.Projects[0].Record)
	assert.Empty(t, rep.Projects[0].Generations)
	assert.Empty(t, rep.Projects[0].Errors, "a project never synced has no record, which is no failed read")
	out, _, err = output(t, dir, fakeSbx{}, false, "shop")
	require.NoError(t, err)
	assert.Equal(t, "shop\n  record: absent, the project was never synced\n  sandbox: absent\n", out)
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
			assert.Contains(t, err.Error(), "2 read(s) failed", "the one listing of sbx counts once, though each block shows it")
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
			assert.Equal(t, RecordUnreadable, blog.Record)
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

func TestRunRefusesAName(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	out, _, err := output(t, dir, fakeSbx{}, false, "shop", "../x")
	require.Error(t, err)
	assert.Equal(t, cli.Usage.ID, cli.Of(err).ID)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, out, "a refused argument prints no report")
}

func TestRunGivesAReadWithNoRowItsOwnID(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	out, _, err := output(t, dir, fakeSbx{err: errors.New("boom")}, true, "shop")
	require.Error(t, err)
	assert.Equal(t, []ReadError{{ID: StatusIncomplete.ID, Slug: StatusIncomplete.Slug, Read: "sbx ls", Message: "boom"}}, decode(t, out).Projects[0].Errors)
}

func TestRunWritesNothing(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), map[string]string{"blog": "{"})
	require.NoError(t, os.Chmod(dir, 0o755))
	before := snapshot(t, dir)
	_, _, err := output(t, dir, fakeSbx{list: []sbxdrv.Sandbox{{Name: "shop", Status: "running"}}}, false)
	require.Error(t, err, "the record of blog does not decode")
	assert.Equal(t, before, snapshot(t, dir), "no file, mode or lock changes, the mode doctor reports included")
}

func TestRunKeepsAHostileMessageOnItsLine(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	sbx := fakeSbx{err: errors.New("boom\n  error RJ-999 forged (x): y\x1b[2J\xff")}
	t.Run("text", func(t *testing.T) {
		t.Parallel()
		out, _, err := output(t, dir, sbx, false, "shop")
		require.Error(t, err)
		assert.NotContains(t, out, "\n  error RJ-999", "the line feed is written as two characters")
		assert.NotContains(t, out, "\x1b", "the writer escapes the escape byte")
		assert.Contains(t, out, `boom\n  error RJ-999 forged`)
	})
	t.Run("json", func(t *testing.T) {
		t.Parallel()
		out, _, err := output(t, dir, sbx, true, "shop")
		assert.Equal(t, 1, cli.ExitCode(err), "a byte that is not UTF-8 loses no report")
		assert.Contains(t, decode(t, out).Projects[0].Errors[0].Message, "\uFFFD")
	})
}

func TestRunNoProjects(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, nil, nil)
	out, _, err := output(t, dir, fakeSbx{err: errors.New("not called")}, false)
	require.NoError(t, err, "with no project, sbx is not listed")
	assert.Equal(t, "no projects: run romeu sync\n", out)
}

func TestRunNamesWithNoHostState(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out), JSON: true}
	require.NoError(t, run(t.Context(), env, host{sbx: fakeSbx{}}, []string{"shop"}))
	rep := decode(t, out.String())
	require.Len(t, rep.Projects, 1)
	assert.Equal(t, RecordAbsent, rep.Projects[0].Record)
}

func TestRunStopsOnACancelledContext(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, world(t), nil)
	store, err := state.OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var out bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out)}
	require.ErrorIs(t, run(ctx, env, host{store: store, sbx: fakeSbx{}}, nil), context.Canceled)
}
