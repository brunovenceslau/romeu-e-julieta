// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
)

// statusDoc is the part of romeu status --json these cases read.
type statusDoc struct {
	Toolchain *struct {
		State string `json:"state"`
		Sbx   string `json:"sbx"`
	} `json:"toolchain"`
	Warnings []struct {
		Slug string `json:"slug"`
	} `json:"warnings"`
	Projects []struct {
		Name         string `json:"name"`
		Record       string `json:"record"`
		SyncedCommit string `json:"syncedCommit"`
		Sandbox      string `json:"sandbox"`
		Findings     []struct {
			Slug string `json:"slug"`
		} `json:"findings"`
		Errors []struct {
			ID   string `json:"id"`
			Read string `json:"read"`
		} `json:"errors"`
	} `json:"projects"`
}

// statusOf decodes the stdout of romeu status --json.
func statusOf(t *testing.T, out string) statusDoc {
	t.Helper()
	var doc statusDoc
	require.NoError(t, json.Unmarshal([]byte(out), &doc), out)
	return doc
}

// TestStatusAfterSync: after gate 1 and a sync of foo, status reports
// the toolchain acknowledged, foo's record, its synced commit and no
// finding, exit 0, and changes nothing on disk (04 4.2, REQ-0327,
// REQ-0328).
func TestStatusAfterSync(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	stateBefore, rootBefore := romeu.Snapshot(t, f.e.StateDir()), romeu.Snapshot(t, f.e.Root)
	res := f.e.Run(t, []string{"status", "--json", "foo"})
	require.Equal(t, 0, res.Code, res.Stderr)
	doc := statusOf(t, res.Stdout)
	require.NotNil(t, doc.Toolchain)
	assert.Equal(t, "acknowledged", doc.Toolchain.State)
	assert.Equal(t, "0.49.2", doc.Toolchain.Sbx)
	assert.Empty(t, doc.Warnings, "0.49.2 is the tested window")
	require.Len(t, doc.Projects, 1)
	p := doc.Projects[0]
	assert.Equal(t, "foo", p.Name)
	assert.Equal(t, "present", p.Record)
	assert.Regexp(t, `^[0-9a-f]{40}$`, p.SyncedCommit)
	assert.Empty(t, p.Findings)
	assert.Empty(t, p.Errors)
	assert.Equal(t, stateBefore, romeu.Snapshot(t, f.e.StateDir()), "status writes no state, takes no lock")
	assert.Equal(t, rootBefore, romeu.Snapshot(t, f.e.Root), "status writes nothing in the tree")
}

// TestStatusReportsDrift: an edited live sbxenv.yaml is drift-detected
// in foo's block, and status still exits 0: it reports, it does not
// stop (REQ-0385).
func TestStatusReportsDrift(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	require.NoError(t, os.WriteFile(f.env("sbxenv.yaml"), []byte("edited\n"), 0o600))
	res := f.e.Run(t, []string{"status", "foo"})
	require.Equal(t, 0, res.Code, res.Stdout+res.Stderr)
	assert.Contains(t, res.Stdout, "  drift-detected: the live files sbxenv.yaml differ from render.json")
}

// TestStatusGoesOnPastAnUnreadableRecord: a project record that does not
// decode is record-unreadable in its block, the report is printed, and
// the command exits 1 with status-incomplete, never 3 (REQ-0329).
func TestStatusGoesOnPastAnUnreadableRecord(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.e.StateDir(), "projects", "foo.json"), []byte("{"), 0o600))
	res := f.e.Run(t, []string{"status"})
	require.Equal(t, 1, res.Code, res.Stderr)
	assert.Contains(t, res.Stdout, "toolchain: acknowledged\n")
	assert.Contains(t, res.Stdout, "  error RJ-334 record-unreadable (project record): ")
	assert.Contains(t, res.Stderr, "error RJ-359 status-incomplete: ")
}

// TestStatusOfAPlainBuild: a romeu of plain go build, which embeds no
// julieta, runs status as the release build does and never stops with
// julieta-not-embedded (T048, from T043; 04 4.2).
func TestStatusOfAPlainBuild(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, false)
	res := e.Run(t, []string{"status"})
	require.Equal(t, 0, res.Code, res.Stderr)
	assert.Equal(t, "no host state: run romeu init, then romeu sync\n", res.Stdout)
	assert.NotContains(t, res.Stderr, "julieta-not-embedded")
}

// TestStatusRefusesItsArguments: two names, or a name that is no project
// name, are usage, exit 2, with no report.
func TestStatusRefusesItsArguments(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, false)
	for _, args := range [][]string{{"status", "a", "b"}, {"status", "Shop"}} {
		res := e.Run(t, args)
		assert.Equal(t, 2, res.Code, res.Stderr)
		assert.Empty(t, res.Stdout)
		assert.Contains(t, res.Stderr, "error RJ-349 usage: ")
	}
}
