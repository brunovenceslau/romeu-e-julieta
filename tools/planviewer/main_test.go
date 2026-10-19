// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plan writes a plan of two tasks, one merged and one ready, and returns
// its directory.
func plan(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"T001-a.md": "---\nid: T001\nfate: merged\n---\n",
		"T002-b.md": "---\nid: T002\nfate: kept\nneeds:\n  - T001\n---\n",
	}
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}
	return dir
}

func TestRun(t *testing.T) {
	dir := plan(t)
	out := filepath.Join(dir, "out.html")
	tmpl := filepath.Join("template.html")
	plan := filepath.Join(dir, "plan.md")
	require.NoError(t, os.WriteFile(plan, []byte("| M | Name | Ends when (x) | Delivers |\n|---|---|---|---|\n| M0 | Shape | done | shape |\n"), 0o644))

	cases := []struct {
		name       string
		args       []string
		code       int
		stdout     string
		stderrPart string
	}{
		{"writes the page", []string{"-dir", dir, "-template", tmpl, "-plan", plan, "-o", out}, exitOK, out + "\n", ""},
		{"ready ids", []string{"-dir", dir, "-ready"}, exitOK, "T002\n", ""},
		{"critical path ids", []string{"-dir", dir, "-critical-path"}, exitOK, "T002\n", ""},
		{"both lists", []string{"-ready", "-critical-path"}, exitUsage, "", "exclude each other"},
		{"missing plan", []string{"-dir", dir, "-template", tmpl, "-plan", filepath.Join(dir, "none"), "-o", out}, exitError, "", "no such file"},
		{"help", []string{"-h"}, exitOK, "", "Usage of planviewer"},
		{"unknown flag", []string{"-nope"}, exitUsage, "", "flag provided but not defined"},
		{"extra argument", []string{"extra"}, exitUsage, "", "takes no argument"},
		{"empty directory", []string{"-dir", t.TempDir(), "-ready"}, exitError, "", "no task file"},
		{"missing template", []string{"-dir", dir, "-template", filepath.Join(dir, "none"), "-o", out}, exitError, "", "no such file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.Equal(t, c.code, run(c.args, &stdout, &stderr))
			assert.Equal(t, c.stdout, stdout.String())
			assert.Contains(t, stderr.String(), c.stderrPart)
		})
	}
}
