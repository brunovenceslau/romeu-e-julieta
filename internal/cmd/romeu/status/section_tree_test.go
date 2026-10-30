// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// envFile is the sbxenv.yaml of the fixture tree.
const envFile = "schemaVersion: \"1\"\n"

// specCommit is the config commit the fixture render read.
var specCommit = strings.Repeat("c", 40)

// tree writes <root>/shop-env/ with a live sbxenv.yaml and a render.json
// that records it, whose recreate digest is recreate; edit changes the
// record before it is written.
func tree(t *testing.T, root string, recreate canon.Sum[canon.RecreateKind], edit func(*render.Record)) string {
	t.Helper()
	env := filepath.Join(root, "shop-env")
	require.NoError(t, os.MkdirAll(filepath.Join(env, ".romeu"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveEnvFile), []byte(envFile), 0o600))
	sum := sha256.Sum256([]byte(envFile))
	eg, err := render.EgressOf(egress.Result{})
	require.NoError(t, err)
	rec := render.Record{
		RomeuVersion:   "v1.0.0",
		SpecSource:     render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/cfg/main", Commit: specCommit},
		Repos:          []render.RecordRepo{{Dir: "shop-api", URL: "https://github.com/example/shop-api", EgressRef: "refs/romeu/origin/shop-api/main", EgressCommit: strings.Repeat("a", 40)}},
		Files:          map[string]string{render.LiveEnvFile: hex.EncodeToString(sum[:])},
		Egress:         eg,
		RecreateDigest: recreate,
	}
	if edit != nil {
		edit(&rec)
	}
	data, err := render.EncodeRecord(rec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveRecord), data, 0o600))
	return env
}

// treeOutput runs status for shop over the state dir and the tree root.
func treeOutput(t *testing.T, dir, root string, jsonOut bool) (string, error) {
	t.Helper()
	store, err := state.OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	var out bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out), JSON: jsonOut}
	sbx := fakeSbx{list: []sbxdrv.Sandbox{{Name: "shop", Status: "running"}}}
	err = run(t.Context(), env, host{store: store, root: root, sbx: sbx}, []string{"shop"})
	return out.String(), err
}

func TestRunReportsTheTree(t *testing.T) {
	t.Parallel()
	created := canon.Sum[canon.RecreateKind]{1}
	other := canon.Sum[canon.RecreateKind]{9}
	live := func(adopted bool) map[string]*state.Project {
		g := generation(t, gen2, state.GenerationOpen, "2026-10-02T09:14:07Z")
		if adopted {
			g.Adopted, g.CreatedWith = true, canon.Sum[canon.RecreateKind]{}
		}
		p := state.NewProject()
		p.Generations = []state.Generation{g}
		return map[string]*state.Project{"shop": p}
	}
	tests := []struct {
		name     string
		records  map[string]*state.Project
		setup    func(t *testing.T, root string)
		commit   string
		findings []Finding
	}{
		{
			name:     "a project never synced has no tree",
			records:  live(false),
			setup:    func(*testing.T, string) {},
			findings: []Finding{},
		},
		{
			name:    "a tree that matches its render",
			records: live(false),
			setup: func(t *testing.T, root string) {
				t.Helper()
				tree(t, root, created, nil)
			},
			commit:   specCommit,
			findings: []Finding{},
		},
		{
			name:    "a drifted live file",
			records: live(false),
			setup: func(t *testing.T, root string) {
				t.Helper()
				env := tree(t, root, created, nil)
				require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveEnvFile), []byte("edited\n"), 0o600))
			},
			commit: specCommit,
			findings: []Finding{{Slug: "drift-detected", Message: "the live files sbxenv.yaml differ from render.json; move the edit into the spec, " +
				"or run romeu sync --overwrite-drift shop, which loses it"}},
		},
		{
			name:    "an interrupted promotion, which no drift is read for",
			records: live(false),
			setup: func(t *testing.T, root string) {
				t.Helper()
				env := tree(t, root, created, func(r *render.Record) {
					r.Promoting = &render.Promoting{CandidateID: id(t, slv1), FilesDigest: canon.Sum[canon.CandidateFilesKind]{4}, PreviousFiles: map[string]string{}}
				})
				require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveEnvFile), []byte("edited\n"), 0o600))
			},
			commit:   specCommit,
			findings: []Finding{{Slug: "interrupted-promotion", Message: "the promotion of candidate " + slv1 + " was interrupted; the next romeu sync shop recovers it"}},
		},
		{
			name:    "a live render the open generation was not created with",
			records: live(false),
			setup: func(t *testing.T, root string) {
				t.Helper()
				tree(t, root, other, nil)
			},
			commit:   specCommit,
			findings: []Finding{{Slug: "recreate-needed", Message: "generation " + gen2 + " was created with another recreate digest than the live render's; run romeu recreate shop"}},
		},
		{
			name:    "an adopted generation is not compared",
			records: live(true),
			setup: func(t *testing.T, root string) {
				t.Helper()
				tree(t, root, other, nil)
			},
			commit:   specCommit,
			findings: []Finding{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := stateDir(t, tt.records, nil)
			root := t.TempDir()
			tt.setup(t, root)
			out, err := treeOutput(t, dir, root, true)
			require.NoError(t, err)
			p := decode(t, out).Projects[0]
			assert.Equal(t, tt.commit, p.SyncedCommit)
			assert.Equal(t, tt.findings, p.Findings)
		})
	}
}

func TestRunTreeText(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, map[string]*state.Project{"shop": state.NewProject()}, nil)
	root := t.TempDir()
	tree(t, root, canon.Sum[canon.RecreateKind]{1}, nil)
	out, err := treeOutput(t, dir, root, false)
	require.NoError(t, err)
	assert.Equal(t, "shop\n  sandbox: running\n  synced commit: "+specCommit+"\n", out)
}

func TestRunReportsAnUnreadableRender(t *testing.T) {
	t.Parallel()
	dir := stateDir(t, map[string]*state.Project{"shop": state.NewProject()}, nil)
	root := t.TempDir()
	env := filepath.Join(root, "shop-env", ".romeu")
	require.NoError(t, os.MkdirAll(env, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(env, "render.json"), []byte("{"), 0o600))
	out, err := treeOutput(t, dir, root, true)
	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	p := decode(t, out).Projects[0]
	require.Len(t, p.Errors, 1)
	assert.Equal(t, state.RecordUnreadable.ID, p.Errors[0].ID)
	assert.Equal(t, "render.json", p.Errors[0].Read)
}
