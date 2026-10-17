// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// newTaskFixture is a committed repository whose task directory holds
// the files named in existing, and the env that runs against it.
func newTaskFixture(t *testing.T, existing ...string) fixture {
	t.Helper()
	r := gittest.New(t)
	r.Write("docs/plan/tasks/.keep", "")
	for _, name := range existing {
		r.Write("docs/plan/tasks/"+name, "---\n---\n")
	}
	r.Commit("the tasks")
	f := fixture{repo: r, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	f.e = env{dir: r.Dir, gitEnv: r.Env, now: func() time.Time { return fixedDay }, stdout: f.stdout, stderr: f.stderr}
	return f
}

// TestNewTaskWritesTheFile is the acceptance case: the next free id, the
// filename from slug(title), the front matter of the plan with the
// fields to fill, and no other file of the tree changed.
func TestNewTaskWritesTheFile(t *testing.T) {
	f := newTaskFixture(t, "T001-first.md", "T002-second.md")
	code := run(f.e, []string{"task", "The `plan` check, and \"quotes\""})
	require.Equal(t, exitOK, code, f.stderr.String())
	assert.Equal(t, "docs/plan/tasks/T003-the-plan-check-and-quotes.md\n", f.stdout.String())
	want := `---
id: T003
title: "The ` + "`plan`" + ` check, and \"quotes\""
milestone: none
module: none
fate: new
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T003 - The ` + "`plan`" + ` check, and "quotes"

Goal:

- Implements: none:
- Operator: no
- Acceptance:
- Verify:
`
	assert.Equal(t, want, f.read(t, "docs/plan/tasks/T003-the-plan-check-and-quotes.md"))
	status := f.repo.Git("status", "--porcelain", "--untracked-files=all")
	assert.Equal(t, "?? docs/plan/tasks/T003-the-plan-check-and-quotes.md", status, "one file added, none edited")
}

// TestNewTaskNumbering covers the first task, a gap (the highest id plus
// one, never a hole filled), and names that are no task file.
func TestNewTaskNumbering(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		want     string
	}{
		{"an empty directory starts at T001", nil, "docs/plan/tasks/T001-t.md"},
		{"the next after the highest", []string{"T001-a.md", "T141-b.md"}, "docs/plan/tasks/T142-t.md"},
		{"a gap is not filled", []string{"T001-a.md", "T005-b.md"}, "docs/plan/tasks/T006-t.md"},
		{"a name without three digits is no task", []string{"T001-a.md", "T12-b.md", "T0123-c.md"}, "docs/plan/tasks/T002-t.md"},
		{"another extension is no task", []string{"T001-a.md", "T009-b.txt"}, "docs/plan/tasks/T002-t.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTaskFixture(t, tt.existing...)
			require.Equal(t, exitOK, run(f.e, []string{"task", "t"}), f.stderr.String())
			assert.Equal(t, tt.want+"\n", f.stdout.String())
		})
	}
}

// TestNewTaskRefuses covers what writes no file: a wrong command line, a
// title that cannot be a first line or has no slug, a missing task
// directory, an id with no room left, and a run whose re-scan finds its
// id taken by another file.
func TestNewTaskRefuses(t *testing.T) {
	t.Run("no title", func(t *testing.T) {
		f := newTaskFixture(t)
		assert.Equal(t, exitError, run(f.e, []string{"task"}))
		assert.Contains(t, f.stderr.String(), "task takes the title as one argument")
	})
	t.Run("two arguments", func(t *testing.T) {
		f := newTaskFixture(t)
		assert.Equal(t, exitError, run(f.e, []string{"task", "a", "b"}))
	})
	t.Run("a line break", func(t *testing.T) {
		f := newTaskFixture(t)
		assert.Equal(t, exitError, run(f.e, []string{"task", "a\nb"}))
		assert.Contains(t, f.stderr.String(), "does not show")
	})
	t.Run("no ASCII letter or digit", func(t *testing.T) {
		f := newTaskFixture(t)
		assert.Equal(t, exitError, run(f.e, []string{"task", "---"}))
		assert.Contains(t, f.stderr.String(), "filename would be empty")
	})
	t.Run("no task directory", func(t *testing.T) {
		r := gittest.New(t)
		r.Write("README.md", "x\n")
		r.Commit("no tasks")
		f := fixture{repo: r, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
		f.e = env{dir: r.Dir, gitEnv: r.Env, now: func() time.Time { return fixedDay }, stdout: f.stdout, stderr: f.stderr}
		assert.Equal(t, exitError, run(f.e, []string{"task", "t"}))
		assert.Contains(t, f.stderr.String(), "docs/plan/tasks", "the error names the directory")
		assert.NoDirExists(t, filepath.Join(r.Dir, "docs", "plan", "tasks"))
	})
	t.Run("T999 is the last id", func(t *testing.T) {
		f := newTaskFixture(t, "T999-last.md")
		assert.Equal(t, exitError, run(f.e, []string{"task", "t"}))
		assert.Contains(t, f.stderr.String(), "T999")
	})
	t.Run("a file that took the id at the same time", func(t *testing.T) {
		f := newTaskFixture(t, "T001-a.md")
		// Another file with the same id appears between the scan and the
		// write.
		f.e.afterScan = func() {
			require.NoError(t, os.WriteFile(filepath.Join(f.repo.Dir, "docs", "plan", "tasks", "T002-other.md"), []byte("x\n"), 0o600))
		}
		assert.Equal(t, exitError, run(f.e, []string{"task", "t"}))
		assert.Contains(t, f.stderr.String(), "another task took T002")
		assert.NoFileExists(t, filepath.Join(f.repo.Dir, "docs", "plan", "tasks", "T002-t.md"), "its own file is removed")
		assert.FileExists(t, filepath.Join(f.repo.Dir, "docs", "plan", "tasks", "T002-other.md"))
	})
}

// TestNewTaskTitleQuoting checks that the title line of the front matter
// is a YAML double-quoted string that holds the title as written, a
// backslash and a quote included.
func TestNewTaskTitleQuoting(t *testing.T) {
	f := newTaskFixture(t)
	require.Equal(t, exitOK, run(f.e, []string{"task", `a \ b "c"`}), f.stderr.String())
	body := f.read(t, strings.TrimSpace(f.stdout.String()))
	assert.Contains(t, body, "title: \"a \\\\ b \\\"c\\\"\"\n")
}

// TestNewTaskStaysInsideTheRepository checks that a task directory that
// is a link to a directory outside the repository is not written to.
func TestNewTaskStaysInsideTheRepository(t *testing.T) {
	f := newTaskFixture(t)
	outside := t.TempDir()
	require.NoError(t, os.RemoveAll(filepath.Join(f.repo.Dir, "docs", "plan", "tasks")))
	require.NoError(t, os.Symlink(outside, filepath.Join(f.repo.Dir, "docs", "plan", "tasks")))
	assert.Equal(t, exitError, run(f.e, []string{"task", "t"}))
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing was written outside")
}
