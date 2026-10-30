// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// fakeSbx answers sbx ls --json with a list or an error.
type fakeSbx struct {
	list []sbxdrv.Sandbox
	err  error
}

func (f fakeSbx) List(context.Context, sbxdrv.Opts) ([]sbxdrv.Sandbox, error) { return f.list, f.err }

// id parses a ULID of a fixture.
func id(t *testing.T, s string) canon.ULID {
	t.Helper()
	u, err := canon.ParseULID(s)
	require.NoError(t, err)
	return u
}

// at is a fixed time of a fixture.
func at(s string) time.Time {
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return v
}

// generation returns a generation of the project in state st, created
// at created, with one repo; a closed state gets its closedAt.
func generation(t *testing.T, ulid string, st state.State, created string) state.Generation {
	t.Helper()
	g := state.Generation{
		ID:            id(t, ulid),
		State:         st,
		CreatedAt:     at(created),
		CreatedWith:   canon.Sum[canon.RecreateKind]{1},
		WorkspacePath: "/root/shop-env/shop-api",
		Repos:         []state.GenerationRepo{{Dir: "shop-api", URL: "https://github.com/example/shop-api", Primary: true}},
	}
	if st == state.GenerationClosedRemoved || st == state.GenerationClosedLost {
		g.ClosedAt = at(created).Add(time.Hour)
	}
	return g
}

// salvage returns a salvage entry with the given result.
func salvage(t *testing.T, ulid, result, created string) state.Salvage {
	t.Helper()
	s := state.Salvage{
		ID:         id(t, ulid),
		CreatedAt:  at(created),
		Result:     result,
		RefsPrefix: "refs/romeu/salvage/shop/" + ulid + "/",
	}
	if result != state.SalvageLost {
		s.ManifestSHA256 = state.SHA256{2}
	}
	return s
}

// stateDir makes a state directory as romeu makes it and writes each
// record into projects/; a nil record writes raw instead, the bytes of
// a record that does not decode.
func stateDir(t *testing.T, records map[string]*state.Project, raw map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "romeu")
	s, err := state.Open(dir)
	require.NoError(t, err)
	require.NoError(t, s.Close())
	for name, p := range records {
		data, err := state.EncodeProject(p)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projects", name+".json"), data, state.FileMode))
	}
	for name, data := range raw {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projects", name+".json"), []byte(data), state.FileMode))
	}
	return dir
}

// output runs status over the state directory dir and the sbx f and
// returns stdout, stderr and the error status returned.
func output(t *testing.T, dir string, f fakeSbx, jsonOut bool, names ...string) (string, string, error) {
	t.Helper()
	store, err := state.OpenRead(dir)
	require.NoError(t, err)
	defer func() { require.NoError(t, store.Close()) }()
	var out, errOut bytes.Buffer
	env := &cli.Env{Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&errOut), JSON: jsonOut}
	err = run(t.Context(), env, host{store: store, sbx: f}, names)
	return out.String(), errOut.String(), err
}
