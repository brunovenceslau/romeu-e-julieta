// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// newCommandFixture is a committed repository with the two assembled
// command sets, the files named in existing, and the env that runs
// against it.
func newCommandFixture(t *testing.T, existing ...string) fixture {
	t.Helper()
	r := gittest.New(t)
	r.Write("internal/cmd/romeu/all/doc.go", "package all\n")
	r.Write("internal/cmd/julieta/all/doc.go", "package all\n")
	for _, name := range existing {
		r.Write(name, "package x\n")
	}
	r.Commit("the command sets")
	f := fixture{repo: r, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	f.e = env{dir: r.Dir, gitEnv: r.Env, now: func() time.Time { return fixedDay }, stdout: f.stdout, stderr: f.stderr}
	return f
}

// TestNewCommandWritesTheScaffold is the acceptance case: the
// definition with its five side-effect fields unset, the help test and
// the import file, each printed, and the definition of a subcommand
// added to the package of its command with no second import file.
func TestNewCommandWritesTheScaffold(t *testing.T) {
	f := newCommandFixture(t)
	require.Equal(t, exitOK, run(f.e, []string{"command", "romeu", "sync"}), f.stderr.String())
	assert.Equal(t, "internal/cmd/romeu/sync/sync.go\ninternal/cmd/romeu/sync/sync_test.go\ninternal/cmd/romeu/all/import_sync.go\n", f.stdout.String())
	def := f.read(t, "internal/cmd/romeu/sync/sync.go")
	assert.True(t, strings.HasPrefix(def, header+"// Package sync holds the command romeu sync (04 4.2).\npackage sync\n"), def)
	assert.Contains(t, def, "c := cli.Define(cli.PreflightUnset, cli.LockUnset, cli.Appending{}, cli.SnapshotUnset, cli.IdempotencyUnset)\n")
	assert.Contains(t, def, "c.Binary, c.Name, c.Sub = \"romeu\", \"sync\", \"\"\n")
	assert.Contains(t, def, "RunE: func(c *cobra.Command, args []string) error {")
	assert.Contains(t, f.read(t, "internal/cmd/romeu/sync/sync_test.go"), "func TestSyncHelp(t *testing.T) {")
	assert.Equal(t, header+"package all\n\nimport _ \"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/sync\"\n", f.read(t, "internal/cmd/romeu/all/import_sync.go"))

	f = newCommandFixture(t, "internal/cmd/julieta/memory/add.go", "internal/cmd/julieta/all/import_memory.go")
	require.Equal(t, exitOK, run(f.e, []string{"command", "julieta", "memory", "list"}), f.stderr.String())
	assert.Equal(t, "internal/cmd/julieta/memory/list.go\ninternal/cmd/julieta/memory/list_test.go\n", f.stdout.String())
	def = f.read(t, "internal/cmd/julieta/memory/list.go")
	assert.True(t, strings.HasPrefix(def, header+"package memory\n"), "no second package comment:\n%s", def)
	assert.Contains(t, def, "c.Binary, c.Name, c.Sub = \"julieta\", \"memory\", \"list\"\n")
	assert.Contains(t, def, "Use:     \"list\",")
	assert.Contains(t, f.read(t, "internal/cmd/julieta/memory/list_test.go"), "[]string{\"memory\", \"list\", \"--help\"}")
}

// TestNewCommandRefuses checks the arguments it refuses, and that a
// refused run and one that meets a file it would overwrite leave the
// tree as it was.
func TestNewCommandRefuses(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"command", "romeu"}, "command takes a binary"},
		{[]string{"command", "romeu", "a", "b", "c"}, "command takes a binary"},
		{[]string{"command", "gh", "sync"}, `the binary "gh" is neither romeu nor julieta`},
		{[]string{"command", "romeu", "Sync"}, `the name "Sync" is not one lowercase word`},
		{[]string{"command", "romeu", "sync-all"}, `the name "sync-all" is not one lowercase word`},
		{[]string{"command", "romeu", "memory", "../x"}, `the name "../x" is not one lowercase word`},
		{[]string{"command", "romeu", "help"}, `the name "help" is a command every tree already has`},
		{[]string{"command", "romeu", "type"}, `the name "type" is a Go keyword`},
		{[]string{"command", "romeu", "all", "x"}, `the name "all" is the package of the assembled command set`},
		{[]string{"command", "romeu", "memory", "test"}, `the name "test" makes the go command treat a file`},
		{[]string{"command", "julieta", "linux"}, `the name "linux" makes the go command treat a file`},
		{[]string{"command", "julieta", "arm64"}, `the name "arm64" makes the go command treat a file`},
		{[]string{"command", "julieta", "status"}, "internal/cmd/julieta/status exists"},
		{[]string{"command", "julieta", "status", "one"}, "file exists"},
	}
	for _, tt := range tests {
		f := newCommandFixture(t, "internal/cmd/julieta/status/one.go")
		assert.Equal(t, exitError, run(f.e, tt.args), "%v", tt.args)
		assert.Contains(t, f.stderr.String(), tt.want, "%v", tt.args)
		assert.Empty(t, f.stdout.String(), "%v", tt.args)
		out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
			Class: procgroup.ToolStep,
			Argv:  []string{"git", "status", "--porcelain", "--untracked-files=all"},
			Dir:   f.repo.Dir,
			Env:   f.repo.Env,
		})
		require.NoError(t, err)
		assert.Empty(t, string(out), "%v leaves the tree as it was", tt.args)
	}
}

// TestCommandScaffoldBuilds puts the scaffold of a command and of a
// subcommand into this module through an overlay: each package and its
// help test build, and the test binary fails at init, where Register names the five
// side-effect fields to fill.
func TestCommandScaffoldBuilds(t *testing.T) {
	root := moduleRoot(t)
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	for _, args := range [][]string{{"romeu", "scaffoldfixture"}, {"julieta", "scaffoldfixture", "one"}} {
		f := newCommandFixture(t)
		require.Equal(t, exitOK, run(f.e, append([]string{"command"}, args...)), f.stderr.String())
		replace := map[string]string{}
		for rel := range strings.Lines(f.stdout.String()) {
			rel = strings.TrimSpace(rel)
			if strings.Contains(rel, "/all/") {
				continue
			}
			target := filepath.Join(root, filepath.FromSlash(rel))
			require.NoFileExists(t, target, "the overlay adds the file")
			replace[target] = filepath.Join(f.repo.Dir, filepath.FromSlash(rel))
		}
		dir := t.TempDir()
		overlay, err := json.Marshal(map[string]any{"Replace": replace})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "overlay.json"), overlay, 0o600))
		pkg := "./internal/cmd/" + args[0] + "/" + args[1]
		bin := filepath.Join(dir, "scaffold.test")
		// test runs the help test of the package from dir; the go command
		// builds it with vet off, since vet would change into the package
		// directory, which only the overlay holds.
		test := func() ([]byte, error) {
			out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
				Class: procgroup.ToolStep,
				Argv:  []string{goBin, "test", "-c", "-vet=off", "-overlay=" + filepath.Join(dir, "overlay.json"), "-o", bin, pkg},
				Dir:   root,
				Env:   buildEnv(),
			})
			require.NoError(t, err, "%v: the package and its help test build: %s", args, out)
			return procgroup.CombinedOutput(t.Context(), procgroup.Command{
				Class: procgroup.ToolStep,
				Argv:  []string{bin},
				Dir:   dir,
				Env:   []string{"PATH=" + os.Getenv("PATH")},
			})
		}
		out, err := test()
		require.Error(t, err, "%v: the unset fields fail at init", args)
		assert.Contains(t, string(out), "lacks the preflight field, the lock field, the appending field, the snapshot field, the idempotency field", "%v", args)

		// Filled, the definition registers and its help test passes.
		for target, source := range replace {
			if strings.HasSuffix(target, "_test.go") {
				continue
			}
			def, err := os.ReadFile(source)
			require.NoError(t, err)
			filled := strings.Replace(string(def), "cli.PreflightUnset, cli.LockUnset, cli.Appending{}, cli.SnapshotUnset, cli.IdempotencyUnset",
				"cli.PreflightNone, cli.LockNone, cli.AppendsNothing(), cli.SnapshotNone, cli.ReadsOnly", 1)
			require.NoError(t, os.WriteFile(source, []byte(filled), 0o600))
		}
		out, err = test()
		require.NoError(t, err, "%v: the help test of the filled scaffold passes: %s", args, out)
	}
}
