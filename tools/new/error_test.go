// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// errorSpec is a page with the table of 04 4.4: the highest id of the
// fixture is RJ-120, a retired row, which keeps its number.
const errorSpec = "# 4\n\n## 4.4 Error ids\n\n| Id | Slug | Exit | Raised by |\n|---|---|---|---|\n" +
	"| `RJ-101` | `locked` | 2 | romeu |\n| `RJ-120` | `gone` | 1 | retired |\n\n## 4.5 Next\n"

// newErrorFixture is a committed repository with the table of 04 4.4,
// the errors.go of package cli, one of package gitsafe that declares
// RJ-105, a package with no errors.go and one whose errors.go does not
// import cli.
func newErrorFixture(t *testing.T) fixture {
	t.Helper()
	r := gittest.New(t)
	r.Write(errorTable, errorSpec)
	r.Write("internal/cli/errors.go", "package cli\n\nvar Usage = Row{ID: \"RJ-110\", Slug: \"usage\", Exit: 2, Fix: \"x\", RecoveredIn: \"none: x\"}\n")
	r.Write("internal/gitsafe/errors.go", "package gitsafe\n\nimport \"github.com/brunovenceslau/romeu-e-julieta/internal/cli\"\n\nvar (\n\tRefBad = cli.Row{ID: \"RJ-105\", Slug: \"ref-bad\", Exit: 2, Fix: \"x\", RecoveredIn: \"none: x\"}\n)\n")
	r.Write("internal/cmd/romeu/sync/sync.go", "package sync\n")
	r.Write("internal/procgroup/errors.go", "package procgroup\n\nconst timeoutID = \"RJ-102\"\n")
	r.Commit("the table")
	f := fixture{repo: r, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	f.e = env{dir: r.Dir, gitEnv: r.Env, now: func() time.Time { return fixedDay }, stdout: f.stdout, stderr: f.stderr}
	return f
}

// TestNewErrorDeclaresTheNextID is the acceptance case: the id after the
// highest of the declared rows and of 04 4.4, a retired one included,
// appended to an errors.go or written in a new one, as a Row the table
// reads back with its exit code, hint and "recovered in" cell to fill.
func TestNewErrorDeclaresTheNextID(t *testing.T) {
	f := newErrorFixture(t)
	require.Equal(t, exitOK, run(f.e, []string{"error", "gitsafe", "ref-missing"}), f.stderr.String())
	assert.Equal(t, "internal/gitsafe/errors.go RJ-121 ref-missing\n", f.stdout.String())
	assert.Contains(t, f.read(t, "internal/gitsafe/errors.go"), ")\n\n// RefMissing is the error ref-missing (RJ-121). fill: when the package returns it.\nvar RefMissing = cli.Row{\n\tID:          \"RJ-121\",\n\tSlug:        \"ref-missing\",\n\tExit:        0, // fill: the exit code of 04 4.1\n")

	f.stdout.Reset()
	require.Equal(t, exitOK, run(f.e, []string{"error", "cmd/romeu/sync", "sync-failed"}), f.stderr.String())
	assert.Equal(t, "internal/cmd/romeu/sync/errors.go RJ-122 sync-failed\n", f.stdout.String())
	assert.Contains(t, f.read(t, "internal/cmd/romeu/sync/errors.go"), header+"package sync\n\nimport \"github.com/brunovenceslau/romeu-e-julieta/internal/cli\"\n\n// SyncFailed is the error")

	f.stdout.Reset()
	require.Equal(t, exitOK, run(f.e, []string{"error", "cli", "usage-two"}), f.stderr.String())
	assert.Contains(t, f.read(t, "internal/cli/errors.go"), "var UsageTwo = Row{\n")

	root, err := os.OpenRoot(f.repo.Dir)
	require.NoError(t, err)
	defer func() { _ = root.Close() }()
	table, err := cli.ReadTable(root.FS())
	require.NoError(t, err)
	var ids []string
	for _, d := range table {
		ids = append(ids, d.ID+" "+d.Slug)
	}
	assert.Equal(t, []string{"RJ-105 ref-bad", "RJ-110 usage", "RJ-121 ref-missing", "RJ-122 sync-failed", "RJ-123 usage-two"}, ids, "the table reads each new row")
}

// TestNewErrorSameIDAtOnce: a run that declares the id this run took
// while it wrote makes this run fail, naming the id.
func TestNewErrorSameIDAtOnce(t *testing.T) {
	f := newErrorFixture(t)
	f.e.afterScan = func() {
		require.NoError(t, os.MkdirAll(f.repo.Dir+"/internal/other", 0o755))
		require.NoError(t, os.WriteFile(f.repo.Dir+"/internal/other/errors.go", []byte("package other\n\nimport \"github.com/brunovenceslau/romeu-e-julieta/internal/cli\"\n\nvar X = cli.Row{ID: \"RJ-121\", Slug: \"other\", Exit: 1}\n"), 0o600))
	}
	assert.Equal(t, exitError, run(f.e, []string{"error", "gitsafe", "ref-missing"}))
	assert.Contains(t, f.stderr.String(), "another run declared RJ-121 at the same time")
	assert.Empty(t, f.stdout.String())
}

// TestNewErrorRefuses checks the arguments it refuses, each with the
// tree left as it was.
func TestNewErrorRefuses(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"error", "gitsafe"}, "error takes a package"},
		{[]string{"error", "../x", "a"}, `the package "../x" is not a path below internal/`},
		{[]string{"error", "Gitsafe", "a"}, `the package "Gitsafe" is not a path below internal/`},
		{[]string{"error", "gitsafe", "Bad"}, `the slug "Bad" is not lowercase words`},
		{[]string{"error", "gitsafe", "a--b"}, `the slug "a--b" is not lowercase words`},
		{[]string{"error", "gitsafe", "locked"}, `the slug "locked" is the error RJ-101 already`},
		{[]string{"error", "gitsafe", "ref-bad"}, `the slug "ref-bad" is the error RJ-105 already`},
		{[]string{"error", "nonesuch", "a"}, "internal/nonesuch is no package directory"},
		{[]string{"error", "cmd/romeu/type", "a"}, `the package "cmd/romeu/type" holds the Go keyword "type"`},
		{[]string{"error", "procgroup", "a"}, "internal/procgroup/errors.go does not import"},
	}
	for _, tt := range tests {
		f := newErrorFixture(t)
		before := f.read(t, "internal/gitsafe/errors.go")
		assert.Equal(t, exitError, run(f.e, tt.args), "%v", tt.args)
		assert.Contains(t, f.stderr.String(), tt.want, "%v", tt.args)
		assert.Empty(t, f.stdout.String(), "%v", tt.args)
		assert.Equal(t, before, f.read(t, "internal/gitsafe/errors.go"), "%v", tt.args)
	}
}
