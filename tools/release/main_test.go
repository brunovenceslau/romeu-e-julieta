// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/release/internal/build"
)

// chdirRoot runs the rest of the test in the root of the repository.
func chdirRoot(t *testing.T) {
	t.Helper()
	t.Chdir("../..")
}

// TestRunDryRun builds the repository itself through run: the archives
// and checksums.txt are named on stdout, and a tree with changes needs
// --allow-dirty.
func TestRunDryRun(t *testing.T) {
	chdirRoot(t)
	out := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	args := []string{"build", "--version", "v1.0.0", "--dry-run", "--allow-dirty", "--out", out}
	require.Equal(t, exitOK, run(t.Context(), args, &stdout, &stderr), stderr.String())
	assert.Contains(t, stdout.String(), filepath.Join(out, "romeu_v1.0.0_darwin_arm64.tar.gz"))
	assert.Contains(t, stdout.String(), filepath.Join(out, "checksums.txt"))
	assert.NotContains(t, stdout.String(), "note: the archives carry no julieta")
	assert.FileExists(t, filepath.Join(out, "checksums.txt"))
	assert.Empty(t, stderr.String())
}

// TestRunHost builds the repository's romeu for this platform through
// run: the binary is named on stdout and runs here, and a second run to
// the same path refuses.
func TestRunHost(t *testing.T) {
	chdirRoot(t)
	out := filepath.Join(t.TempDir(), "romeu")
	var stdout, stderr bytes.Buffer
	require.Equal(t, exitOK, run(t.Context(), []string{"build", "--host", "--out", out}, &stdout, &stderr), stderr.String())
	assert.Equal(t, out+"\n", stdout.String())
	assert.Empty(t, stderr.String())
	printed, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{Class: procgroup.ToolStep, Argv: []string{out, "version"}})
	require.NoError(t, err, "%s", printed)
	assert.Contains(t, string(printed), "protocol")

	stdout.Reset()
	stderr.Reset()
	assert.Equal(t, exitError, run(t.Context(), []string{"build", "--host", "--out", out}, &stdout, &stderr))
	assert.Contains(t, stderr.String(), "exists")
}

func TestRunRefusals(t *testing.T) {
	chdirRoot(t)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"another_version", []string{"build", "--version", "v2.0.0", "--dry-run", "--allow-dirty"}, "is not a build of VERSION"},
		{"release_build_without_the_gate_inputs", []string{"build", "--version", "v1.0.0"}, "release build:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.Equal(t, exitError, run(t.Context(), tt.args, &stdout, &stderr))
			assert.Empty(t, stdout.String())
			assert.Contains(t, stderr.String(), tt.want)
		})
	}
}

// TestStart runs the same command under the signal rule.
func TestStart(t *testing.T) {
	var stdout, stderr bytes.Buffer
	assert.Equal(t, exitUsage, start(nil, &stdout, &stderr))
	assert.Contains(t, stderr.String(), "usage:")
}

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no_command", nil},
		{"another_command", []string{"publish"}},
		{"no_version", []string{"build", "--dry-run"}},
		{"extra_argument", []string{"build", "--version", "v1.0.0", "extra"}},
		{"unknown_flag", []string{"build", "--version", "v1.0.0", "--now"}},
		{"host_without_out", []string{"build", "--host"}},
		{"host_with_a_version", []string{"build", "--host", "--out", "x", "--version", "v1.0.0"}},
		{"host_dry_run", []string{"build", "--host", "--out", "x", "--dry-run"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.Equal(t, exitUsage, run(t.Context(), tt.args, &stdout, &stderr))
			assert.Empty(t, stdout.String())
			assert.Contains(t, stderr.String(), "usage:")
		})
	}
}

// TestNotes: a build that carries no julieta says so, and a dirty one
// says its stamp is dirty; a clean build that embeds julieta says
// nothing.
func TestNotes(t *testing.T) {
	t.Parallel()
	var none bytes.Buffer
	notes(&none, build.Result{Embedded: true})
	assert.Empty(t, none.String())

	var both bytes.Buffer
	notes(&both, build.Result{Dirty: true})
	assert.Contains(t, both.String(), "note: the tree has changes")
	assert.Contains(t, both.String(), "note: the archives carry no julieta: no package of romeu consumes internal/embedded/julieta")
}
