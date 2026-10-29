// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore/write"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// The ids and the clock the tests give julieta (10 10.4).
var (
	clock      = time.Date(2026, 10, 29, 9, 30, 0, 0, time.UTC)
	generation = "01j9zc2q8x7w4t6y3r5e1v9k0p"
	salvageID  = "01jb000000000000000000000a"
)

// sandbox is a HOME and a manifest of one repo, api, the primary: a
// repository with a commit and a dirty working tree, and its memory dir.
type sandbox struct {
	home string
	m    manifest.Manifest
}

// newSandbox makes the sandbox, with the manifest's agent agent and a
// salvageRun when run is set, and sets HOME and the manifest's
// variable.
func newSandbox(t *testing.T, agent string, run bool) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir()}
	primary := filepath.Join(t.TempDir(), "api")
	mem := filepath.Join(t.TempDir(), "api")
	require.NoError(t, os.MkdirAll(mem, 0o700))
	git(t, filepath.Dir(primary), "init", "-q", primary)
	git(t, primary, "commit", "-q", "--allow-empty", "-m", "base")
	require.NoError(t, os.WriteFile(filepath.Join(primary, "loose"), []byte("loose\n"), 0o600))
	digest, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	s.m = manifest.Manifest{
		Agent: agent, Egress: []string{}, Generation: generation,
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: digest.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20), Salvage: manifest.Salvage{CapBytes: 1 << 20, ExcludeIgnored: []string{}},
		Repos: []manifest.Repo{{Base: []string{}, Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: primary, Memory: mem}},
	}
	if run {
		s.m.SalvageRun = &manifest.SalvageRun{ID: salvageID}
	}
	t.Setenv("HOME", s.home)
	v, err := manifest.EncodeEnv(s.m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
	return s
}

// dir returns the salvage dir of api.
func (s *sandbox) dir() string {
	return filepath.Join(s.m.Repos[0].Memory, "salvage", generation, salvageID)
}

// julieta runs julieta with args and returns the exit status, stdout and
// stderr.
func julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: func() time.Time { return clock }, ULIDs: canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 1<<12)))}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// git runs plain git to build fixtures, with no system or global config
// and a fixed identity, and returns its standard output, trimmed.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{"/usr/bin/git", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir, Stdout: &stdout, Stderr: &stderr,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		}})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// TestSalvageHelp is the help test of julieta salvage (12 12.3).
func TestSalvageHelp(t *testing.T) {
	code, stdout, stderr := julieta(t, "salvage", "--help")
	assert.Equal(t, 0, code)
	assert.Empty(t, stderr)
	assert.True(t, strings.HasPrefix(stdout, "Usage: julieta salvage [flags]\n"), stdout)
}

// TestSalvageFingerprint: --fingerprint prints the fingerprint object
// and nothing else, writes nothing, and needs no salvageRun; the
// --json document of the salvage that follows carries the same object.
func TestSalvageFingerprint(t *testing.T) {
	s := newSandbox(t, "no-profile", false)
	code, stdout, stderr := julieta(t, "salvage", "--fingerprint")
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr)
	var fp salvage.Fingerprint
	require.NoError(t, json.Unmarshal([]byte(stdout), &fp, json.RejectUnknownMembers(true)))
	require.Len(t, fp.Repos, 1)
	assert.Equal(t, "api", fp.Repos[0].Dir)
	entries, err := os.ReadDir(s.m.Repos[0].Memory)
	require.NoError(t, err)
	assert.Empty(t, entries, "--fingerprint writes nothing")

	code, _, stderr = julieta(t, "salvage", "--json")
	assert.Equal(t, 2, code, "no salvageRun")
	assert.Contains(t, stderr, "manifest-protocol")

	s = newSandbox(t, "no-profile", true)
	code, fpOut, stderr := julieta(t, "salvage", "--fingerprint")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(fpOut), &fp))
	code, stdout, stderr = julieta(t, "salvage", "--json")
	require.Equal(t, 0, code, stderr)
	var rep Report
	require.NoError(t, json.Unmarshal([]byte(stdout), &rep, json.RejectUnknownMembers(true)))
	assert.Equal(t, fp, rep.Fingerprint)
	assert.Equal(t, Schema, rep.Schema)
	assert.Equal(t, salvageID, rep.SalvageID)
	assert.Equal(t, []ReportManifest{{Complete: true, Path: filepath.Join(s.dir(), "manifest.json"), Repo: "api"}}, rep.Manifests)

	code, _, stderr = julieta(t, "salvage")
	assert.Equal(t, 1, code, "a second salvage with the same id cannot write")
	assert.Contains(t, stderr, "exists")
}

// TestSalvageUsage: --fingerprint with a flag of the salvage is a usage
// error.
func TestSalvageUsage(t *testing.T) {
	newSandbox(t, "no-profile", true)
	for _, flag := range []string{"--stop-agents", "--include-transcripts"} {
		code, _, stderr := julieta(t, "salvage", "--fingerprint", flag)
		assert.Equal(t, 2, code, flag)
		assert.Contains(t, stderr, "--fingerprint")
	}
}

// TestSalvageWaitsOnTheStoreLocks: after --stop-agents, salvage waits
// on a store lock another holds, says so on stderr once the wait passes
// salvage.LockReport, as a JSON line with --json, and goes on once it
// is released.
func TestSalvageWaitsOnTheStoreLocks(t *testing.T) {
	old := salvage.LockReport
	salvage.LockReport = 20 * time.Millisecond
	t.Cleanup(func() { salvage.LockReport = old })
	for _, asJSON := range []bool{false, true} {
		t.Run(fmt.Sprint("json=", asJSON), func(t *testing.T) {
			s := newSandbox(t, "no-profile", true)
			lock := write.LockPath(s.home, sharedDir)
			unlock, err := write.Lock(lock, nil)
			require.NoError(t, err)
			released := make(chan struct{})
			go func() {
				time.Sleep(300 * time.Millisecond)
				unlock()
				close(released)
			}()
			args := []string{"salvage", "--stop-agents"}
			if asJSON {
				args = append(args, "--json")
			}
			code, _, stderr := julieta(t, args...)
			<-released
			require.Equal(t, 0, code, stderr)
			if asJSON {
				assert.Equal(t, `{"path":"`+lock+`","progress":"lock-wait"}`+"\n", stderr)
			} else {
				assert.Equal(t, "waiting for the store lock "+lock+"\n", stderr)
			}
			assert.FileExists(t, filepath.Join(s.dir(), "manifest.json"))
		})
	}
}

// TestSalvageText: the text output names each manifest, each skipped
// item, and the payload's secret-bearing line.
func TestSalvageText(t *testing.T) {
	s := newSandbox(t, "no-profile", true)
	primary := s.m.Repos[0].Path
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".gitignore"), []byte("*.log\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(primary, "a.log"), []byte("log\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(primary, ".git", "MERGE_HEAD"), []byte(strings.Repeat("a", 40)+"\n"), 0o600))
	code, stdout, stderr := julieta(t, "salvage")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "wrote "+filepath.Join(s.dir(), "manifest.json")+" repo api salvage "+salvageID+"\n"+
		"skipped in-progress-operation worktree-0: a merge is in progress\n"+secretLine+"\n", stdout)
}

// TestTranscript: a .jsonl file below the projects dir is a transcript,
// and nothing else is.
func TestTranscript(t *testing.T) {
	assert.True(t, transcript(".claude/projects/p/s.jsonl"))
	assert.False(t, transcript(".claude/projects/p/memory/MEMORY.md"))
	assert.False(t, transcript(".claude/todos/s.jsonl"))
	assert.Nil(t, profileOf("no-profile"))
	p := profileOf("claude")
	require.NotNil(t, p)
	assert.Equal(t, "claude", p.Name)
	assert.NotEmpty(t, p.Paths)
}

// TestSalvageBadManifest: a repo the setup checks refuse, or a memory
// dir that is not absolute, stops the command before it writes.
func TestSalvageBadManifest(t *testing.T) {
	for name, change := range map[string]func(*manifest.Repo){
		"dir":    func(r *manifest.Repo) { r.Dir = "memory" },
		"memory": func(r *manifest.Repo) { r.Memory = "relative" },
	} {
		t.Run(name, func(t *testing.T) {
			s := newSandbox(t, "no-profile", true)
			change(&s.m.Repos[0])
			v, err := manifest.EncodeEnv(s.m)
			require.NoError(t, err)
			t.Setenv(setup.ManifestEnv, v)
			code, _, _ := julieta(t, "salvage")
			assert.NotEqual(t, 0, code)
		})
	}
}
