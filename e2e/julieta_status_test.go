// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusJSON is the --json document of julieta status, every field,
// read strictly.
type statusJSON struct {
	Egress       []string `json:"egress"`
	Errors       []string `json:"errors"`
	HookFailures []struct {
		At       string `json:"at"`
		ExitCode int    `json:"exitCode"`
		Hook     string `json:"hook"`
		Message  string `json:"message"`
		Path     string `json:"path"`
		Repo     string `json:"repo"`
		Stage    string `json:"stage"`
	} `json:"hookFailures"`
	Project string `json:"project"`
	Repos   []struct {
		Branch   string `json:"branch"`
		Branches []struct {
			Ahead        int    `json:"ahead"`
			Behind       int    `json:"behind"`
			Current      bool   `json:"current"`
			Head         string `json:"head"`
			Name         string `json:"name"`
			Upstream     string `json:"upstream"`
			UpstreamGone bool   `json:"upstreamGone"`
		} `json:"branches"`
		Dir               string   `json:"dir"`
		Dirty             []string `json:"dirty"`
		Errors            []string `json:"errors"`
		Head              string   `json:"head"`
		HooksPath         string   `json:"hooksPath"`
		Path              string   `json:"path"`
		SnapshotsDisabled bool     `json:"snapshotsDisabled"`
		Stashes           []struct {
			Head    string `json:"head"`
			Subject string `json:"subject"`
		} `json:"stashes"`
		Worktrees []struct {
			Bare     bool   `json:"bare"`
			Branch   string `json:"branch"`
			Detached bool   `json:"detached"`
			Head     string `json:"head"`
			Locked   bool   `json:"locked"`
			Path     string `json:"path"`
			Prunable bool   `json:"prunable"`
		} `json:"worktrees"`
	} `json:"repos"`
	Schema     string        `json:"schema"`
	SpecCommit string        `json:"specCommit"`
	Warnings   []warningJSON `json:"warnings"`
}

// TestJulietaStatus runs julieta status in the workload (04 4.3): with
// no manifest it exits 2 (REQ-0425); after a setup that fetched a
// primary whose origin moved on, it reads the cached manifest and
// reports each repo, its main still at its commit and one behind
// (REQ-0399), the spec commit and the egress set romeu applied (07 7.5),
// in --json and in text, with no warning; after a setup whose fetch
// failed, the warning fetch-failed goes to stderr.
func TestJulietaStatus(t *testing.T) {
	steps := scenario(t, `
step none env -u JULIETA_MANIFEST "$J" status --json
step old git -C `+primaryPath+` rev-parse HEAD
step setup env JULIETA_MANIFEST=`+manifestEnv(t, 1, originURL+"api.git")+` "$J" setup
echo untracked >`+primaryPath+`/new
step json env -u JULIETA_MANIFEST "$J" status --json
step text env -u JULIETA_MANIFEST "$J" status
step tip git -C /tmp/srv/api.git rev-parse main
step failed env JULIETA_MANIFEST=`+manifestEnv(t, 1, originURL+"gone.git")+` "$J" setup
step warned env -u JULIETA_MANIFEST "$J" status
`)
	r := step(t, steps, "none")
	assert.Equal(t, 2, r.code, r.stderr)
	assert.Empty(t, r.stdout)
	assert.Equal(t, "RJ-342", errorID(t, r))
	require.Equal(t, 0, step(t, steps, "setup").code, step(t, steps, "setup").stderr)
	tip := strings.TrimSpace(step(t, steps, "tip").stdout)
	old := strings.TrimSpace(step(t, steps, "old").stdout)
	assert.NotEqual(t, old, tip)

	r = step(t, steps, "json")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Empty(t, r.stderr)
	var rep statusJSON
	decodeStrict(t, r.stdout, &rep)
	assert.Equal(t, "julieta-status/v1", rep.Schema)
	assert.Equal(t, "shop", rep.Project)
	assert.Equal(t, strings.Repeat("ab", 20), rep.SpecCommit)
	assert.Equal(t, []string{"dl.google.com", "github.com"}, rep.Egress)
	assert.Empty(t, rep.Errors)
	assert.Empty(t, rep.Warnings)
	require.Len(t, rep.Repos, 2)
	api, web := rep.Repos[0], rep.Repos[1]
	assert.Empty(t, api.Errors)
	assert.Equal(t, primaryPath, api.Path)
	assert.Equal(t, "main", api.Branch)
	assert.Equal(t, old, api.Head)
	assert.Equal(t, []string{"untracked"}, api.Dirty)
	require.Len(t, api.Branches, 1)
	assert.Equal(t, "refs/remotes/origin/main", api.Branches[0].Upstream)
	assert.Zero(t, api.Branches[0].Ahead)
	assert.Equal(t, 1, api.Branches[0].Behind)
	require.Len(t, api.Worktrees, 1)
	assert.Equal(t, primaryPath, api.Worktrees[0].Path)
	assert.Empty(t, api.Stashes)
	assert.False(t, api.SnapshotsDisabled)
	assert.Empty(t, web.Errors)
	assert.Equal(t, secondaryDir, web.Path)
	assert.Equal(t, "main", web.Branch)
	assert.Empty(t, web.Dirty)

	r = step(t, steps, "text")
	require.Equal(t, 0, r.code, r.stderr)
	assert.True(t, strings.HasPrefix(r.stdout, "project  shop\nspec     "+strings.Repeat("ab", 20)+"\negress   dl.google.com github.com\nrepo api  "+primaryPath+"\n"), r.stdout)
	assert.Contains(t, r.stdout, "  dirty      untracked\n")
	assert.Empty(t, r.stderr)

	require.Equal(t, 0, step(t, steps, "failed").code, "a failed fetch is a warning: %s", step(t, steps, "failed").stderr)
	r = step(t, steps, "warned")
	require.Equal(t, 0, r.code, r.stderr)
	assert.True(t, strings.HasPrefix(r.stderr, "warning fetch-failed: api: "), "warnings go to stderr: %q", r.stderr)
}

// TestJulietaStatusHookFailures lists a failing git hook in julieta
// status, in --json and in text, from the store the dispatcher records
// it in, until the same hook passes (08 8.2, REQ-0675).
func TestJulietaStatusHookFailures(t *testing.T) {
	steps := scenario(t, strings.Replace(hooksScript, "%s", manifestEnv(t, 1, originURL+"api.git"), 1)+`
touch /tmp/stop
step stopped git commit -q --allow-empty -m first
step failed "$J" status --json
step text "$J" status
rm /tmp/stop
step recovered git commit -q --allow-empty -m first
step cleared "$J" status --json
`)
	require.Equal(t, 0, step(t, steps, "setup").code, step(t, steps, "setup").stderr)
	require.NotEqual(t, 0, step(t, steps, "stopped").code, "git stops the commit on the hook's status")

	r := step(t, steps, "failed")
	require.Equal(t, 0, r.code, r.stderr)
	var rep statusJSON
	decodeStrict(t, r.stdout, &rep)
	require.Len(t, rep.HookFailures, 1)
	f := rep.HookFailures[0]
	assert.Equal(t, "pre-commit", f.Hook)
	assert.Equal(t, primaryPath, f.Repo)
	assert.Equal(t, "tracked", f.Stage)
	assert.Equal(t, 3, f.ExitCode)
	assert.Equal(t, primaryPath+"/.githooks/pre-commit", f.Path)

	r = step(t, steps, "text")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Contains(t, r.stdout, "hook failure  pre-commit  repo "+primaryPath+"  stage tracked  exit 3  at ")

	require.Equal(t, 0, step(t, steps, "recovered").code, step(t, steps, "recovered").stderr)
	r = step(t, steps, "cleared")
	require.Equal(t, 0, r.code, r.stderr)
	rep = statusJSON{}
	decodeStrict(t, r.stdout, &rep)
	assert.Empty(t, rep.HookFailures, "the passing run cleared it")
}
