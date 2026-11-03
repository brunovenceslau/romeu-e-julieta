// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/approve"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// TestReportIsItsSchema: the --json document of a report with every
// member filled (a stale toolchain, sbx-untested, generations open and
// closed with their salvage, a finding and a failed read in a block),
// and of one whose reads of the machine failed, validates against
// schemas/romeu-status.v1.json, the generated schema of the format,
// and names that format.
func TestReportIsItsSchema(t *testing.T) {
	t.Parallel()
	sch, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schemas", "romeu-status.v1.json"))
	require.NoError(t, err)
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(sch))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("romeu-status.v1.json", schemaDoc))
	compiled, err := c.Compile("romeu-status.v1.json")
	require.NoError(t, err)

	measure := func(v sbxdrv.Release) (approve.Live, error) { return liveToolchain(v), nil }
	tests := []struct {
		name string
		sbx  fakeSbx
		want []string
	}{
		{
			name: "every member",
			sbx:  fakeSbx{version: sbxdrv.Release{Minor: 50, Patch: 1}, list: []sbxdrv.Sandbox{{Name: "shop", Status: "running"}}},
			want: []string{`"state":"stale"`, `"slug":"sbx-untested"`, `"closedAt":`, `"slug":"drift-detected"`, `"record":"unreadable"`, `"id":"RJ-334"`},
		},
		{
			name: "the reads of the machine failed",
			sbx:  fakeSbx{versionErr: errors.New("sbx version is not read")},
			want: []string{`"state":"unknown"`, `"read":"sbx version"`, `"sandbox":"unknown"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := stateDir(t, world(t), map[string]string{"bad": "{"})
			acknowledge(t, dir, "0.49.2")
			root := t.TempDir()
			env := tree(t, root, canon.Sum[canon.RecreateKind]{1}, nil)
			require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveEnvFile), []byte("edited\n"), 0o600))
			store, err := state.OpenRead(dir)
			require.NoError(t, err)
			defer func() { require.NoError(t, store.Close()) }()
			var out bytes.Buffer
			e := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&out), JSON: true}
			_ = run(t.Context(), e, host{store: store, root: root, sbx: tt.sbx, measure: measure}, nil)

			doc := strings.TrimSuffix(out.String(), "\n")
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
			require.NoError(t, err, doc)
			require.NoError(t, compiled.Validate(inst), doc)
			assert.Contains(t, doc, `"schema":"romeu-status/v1"`)
			for _, w := range tt.want {
				assert.Contains(t, doc, w, "the document holds the member it is meant to")
			}
		})
	}
}

// TestReportStatesAreClosed: a record or toolchain state the schema does
// not admit fails its validation, so the enums are held.
func TestReportStatesAreClosed(t *testing.T) {
	t.Parallel()
	sch, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "schemas", "romeu-status.v1.json"))
	require.NoError(t, err)
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(sch))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	require.NoError(t, c.AddResource("romeu-status.v1.json", schemaDoc))
	compiled, err := c.Compile("romeu-status.v1.json")
	require.NoError(t, err)
	for _, doc := range []string{
		`{"schema":"romeu-status/v1","warnings":[],"errors":[],"projects":[{"name":"shop","record":"lost","sandbox":"absent","generations":[],"findings":[],"errors":[]}]}`,
		`{"schema":"romeu-status/v1","toolchain":{"state":"fine","changes":[]},"warnings":[],"errors":[],"projects":[]}`,
	} {
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
		require.NoError(t, err)
		assert.Error(t, compiled.Validate(inst), doc)
	}
}
