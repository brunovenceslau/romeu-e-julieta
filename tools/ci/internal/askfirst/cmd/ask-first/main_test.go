// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
)

// list is the ask-first list of the fixture: the list itself and the
// decisions surface.
const list = `version: 1
owner: "@someone"
surfaces:
  - id: ask-first
    globs: [.github/ask-first.yaml]
    reason: this list
  - id: decisions
    globs: [docs/adr/**]
    reason: records
`

// TestRun runs the command of the ask-first job as the job does, in the
// checkout, on a saved event: it passes, fails on a finding, and fails
// to judge on a wrong command line or a payload it cannot read, each
// with its exit status.
func TestRun(t *testing.T) {
	r := gittest.New(t)
	r.Write(".github/ask-first.yaml", list)
	base := r.Commit("base")
	r.Git("checkout", "--quiet", "-b", "topic")
	r.Write("docs/adr/0001-new.md", "new\n")
	head := r.Commit("a record")
	r.Git("checkout", "--quiet", "main")
	t.Chdir(r.Dir)
	// The command clears GIT_CONFIG_GLOBAL with the rest of git's
	// configuration variables, and git then reads the fixture's HOME.
	for _, kv := range r.Env {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
	payload := func(body string) string {
		p := filepath.Join(t.TempDir(), "event.json")
		src := `{"pull_request":{"title":"t","body":` + body + `,"head":{"ref":"topic","sha":"` + head + `"},"base":{"sha":"` + base + `","ref":"main","repo":{"default_branch":"main"}}}}`
		require.NoError(t, os.WriteFile(p, []byte(src), 0o600))
		return p
	}
	cases := []struct {
		name   string
		args   []string
		status int
		out    string
	}{
		{"a pull request that passes", []string{payload(`"## Why\n\nA reason.\n\n## What changed\n\n- a file\n\n## Evidence\n\nok\n\n## Lessons\n\nnone\n\n> [!IMPORTANT]\n> - [x] Approval: decisions - add a record\n"`)}, 0, "ask-first: ok\n"},
		{"a pull request with a finding", []string{payload(`null`)}, 1, "has no approval line for it"},
		{"no argument", nil, 2, ""},
		{"two arguments", []string{"a", "b"}, 2, ""},
		{"a payload that is not there", []string{filepath.Join(t.TempDir(), "none.json")}, 2, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			assert.Equal(t, c.status, run(t.Context(), c.args, &stdout, &stderr), "%s%s", stdout.String(), stderr.String())
			assert.Contains(t, stdout.String(), c.out)
			if c.status == 2 {
				assert.NotEmpty(t, stderr.String())
			}
		})
	}
}
