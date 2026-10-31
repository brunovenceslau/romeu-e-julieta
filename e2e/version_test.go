// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"encoding/json/v2"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
)

// versionsDoc is the versions object of romeu version --json.
type versionsDoc struct {
	Versions struct {
		Sbx      string `json:"sbx"`
		Git      string `json:"git"`
		MacOS    string `json:"macos"`
		Projects []struct {
			Name     string `json:"name"`
			Workload string `json:"workload"`
			Kits     []struct {
				Kit    string `json:"kit"`
				Digest string `json:"digest"`
			} `json:"kits"`
			Errors []any `json:"errors"`
		} `json:"projects"`
		Errors []struct {
			ID   string `json:"id"`
			Read string `json:"read"`
		} `json:"errors"`
	} `json:"versions"`
}

// TestVersionCarriesTheVersions: after gate 1 and a sync of foo, romeu
// version --json carries the versions object: the sbx and git of the
// host, and foo's workload digest from its approval and the kit pin of
// its live render, so a report can paste one document; it reads them
// without the lock and writes nothing (04 4.1, REQ-0286).
func TestVersionCarriesTheVersions(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	stateBefore, rootBefore := romeu.Snapshot(t, f.e.StateDir()), romeu.Snapshot(t, f.e.Root)
	res := f.e.Run(t, []string{"version", "--json"})
	require.Equal(t, 0, res.Code, res.Stderr)
	var doc versionsDoc
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &doc), res.Stdout)
	v := doc.Versions
	assert.Equal(t, "0.49.2", v.Sbx)
	assert.Regexp(t, `^[0-9]+\.[0-9]+\.[0-9]+(\.rc)?$`, v.Git)
	if runtime.GOOS == "darwin" {
		assert.Regexp(t, `^[0-9]+\.[0-9]+(\.[0-9]+)?$`, v.MacOS)
	} else {
		assert.Empty(t, v.MacOS, "the linux build is for the tests alone")
	}
	assert.Empty(t, v.Errors)
	require.Len(t, v.Projects, 1)
	p := v.Projects[0]
	assert.Equal(t, "foo", p.Name)
	assert.Equal(t, syncIndex, p.Workload)
	require.Len(t, p.Kits, 1)
	assert.Equal(t, "julieta", p.Kits[0].Kit)
	assert.Regexp(t, `^[0-9a-f]{64}$`, p.Kits[0].Digest)
	assert.Empty(t, p.Errors)
	assert.Equal(t, stateBefore, romeu.Snapshot(t, f.e.StateDir()), "version writes no state, takes no lock")
	assert.Equal(t, rootBefore, romeu.Snapshot(t, f.e.Root), "version writes nothing in the tree")
}

// TestVersionWithoutSettings: on a machine with no host settings, romeu
// version --json still exits 0 and lists the settings as a failed read
// of its versions object (04 4.2: version exits 0).
func TestVersionWithoutSettings(t *testing.T) {
	t.Parallel()
	e := romeu.New(t, nil)
	res := e.Run(t, []string{"version", "--json"})
	require.Equal(t, 0, res.Code, res.Stderr)
	var doc versionsDoc
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &doc), res.Stdout)
	require.Len(t, doc.Versions.Errors, 1)
	assert.Equal(t, "RJ-349", doc.Versions.Errors[0].ID)
	assert.Equal(t, "host settings", doc.Versions.Errors[0].Read)
	assert.Empty(t, doc.Versions.Sbx)
	assert.Empty(t, doc.Versions.Projects)
}
