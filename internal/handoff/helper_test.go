// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// update rewrites the goldens (10 10.4); CI never passes it.
var update = flag.Bool("update", false, "rewrite the golden files of testdata")

// golden compares got with testdata/name, or rewrites it with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.WriteFile(p, []byte(got), 0o644))
	}
	want, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, string(want), got, "golden %s; rerun with -update to rewrite it and read the diff", name)
}

// base is the time the ULIDs and files of the tests count from.
var base = time.Date(2026, 10, 2, 18, 30, 0, 0, time.UTC)

// id returns the ULID of the instant base plus d, with fixed entropy.
func id(t *testing.T, d time.Duration) canon.ULID {
	t.Helper()
	u, err := canon.NewULID(base.Add(d), canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 16))))
	require.NoError(t, err)
	return u
}

// head returns a full object id made of the digit c.
func head(c byte) string { return string(bytes.Repeat([]byte{c}, 40)) }

// narrative is a body with the headings of a clear handoff.
const narrativeBody = "## Goal\nShip the cart.\n## State\nTests pass.\n## Decisions\nUse a map.\n## Next steps\nOpen the PR.\n## Open questions\nNone.\n"

// file returns a handoff file of kind k made at base plus d, with the
// repos given.
func file(k handoff.Kind, d time.Duration, repos ...handoff.RepoFacts) handoff.File {
	f := handoff.File{Front: handoff.Front{Schema: handoff.Field, Kind: k, CreatedAt: base.Add(d), Project: "shop",
		Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p", Agent: "sbx-kit-claude", Repos: repos}}
	if k.Narrative() {
		f.Body = narrativeBody
	}
	if k == handoff.Final {
		f.Body += "## What would be lost\nNothing.\n"
	}
	return f
}

// put writes f as the handoff file with the id of base plus d in the
// memory dir mem, which it makes.
func put(t *testing.T, mem string, d time.Duration, f handoff.File) canon.ULID {
	t.Helper()
	u := id(t, d)
	data, err := handoff.Encode(f)
	require.NoError(t, err)
	dir := filepath.Join(mem, "handoff")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, handoff.FileName(u, f.Kind)), data, 0o600))
	return u
}

// api is the facts of the repo api at the head c with n dirty files.
func api(c byte, dirty int) handoff.RepoFacts {
	return handoff.RepoFacts{Dir: "api", Branch: "feat/cart", Head: head(c), Upstream: "origin/feat/cart", Ahead: 2, Dirty: dirty}
}
