// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memory_test

import (
	"bytes"
	"encoding/json/v2"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/memory"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/setup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// The clock and the version stamp the tests give julieta (10 10.4).
var (
	clock = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	stamp = cli.Version{Version: "v0.1.0-test", Commit: "abc1234", Date: "2026-10-20T00:00:00Z", Protocol: manifest.Protocol}
)

// sandbox is a HOME with a manifest of the project shop: the primary
// repo api and the secondary repo web, each with its memory dir, and
// the ULID source every julieta run of the test shares.
type sandbox struct {
	home  string
	m     manifest.Manifest
	ulids *canon.ULIDSource
}

// newSandbox makes a sandbox, sets HOME and JULIETA_MANIFEST for the
// test, and works in the primary repo.
func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	s := &sandbox{home: t.TempDir(), ulids: canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 1<<12)))}
	primary := filepath.Join(t.TempDir(), "api")
	web := filepath.Join(s.home, "src", "shop", "web")
	mem := t.TempDir()
	for _, p := range []string{primary, web, filepath.Join(mem, "api"), filepath.Join(mem, "web")} {
		require.NoError(t, os.MkdirAll(p, 0o700))
	}
	run, err := canon.Digest(canon.Run, manifest.Run{})
	require.NoError(t, err)
	s.m = manifest.Manifest{
		Agent: "sbx-kit-claude", Egress: []string{}, Generation: "01j9zc2q8x7w4t6y3r5e1v9k0p",
		Project: "shop", Protocol: manifest.Protocol, Run: manifest.Run{}, RunDigest: run.String(), Schema: manifest.Field,
		SpecCommit: strings.Repeat("ab", 20),
		Repos: []manifest.Repo{
			{Dir: "api", URL: "git://127.0.0.1:9418/api.git", Primary: true, Ref: "main", Path: primary, Memory: filepath.Join(mem, "api")},
			{Dir: "web", URL: "git://127.0.0.1:9418/web.git", Ref: "main", Path: web, Memory: filepath.Join(mem, "web")},
		},
	}
	t.Setenv("HOME", s.home)
	v, err := manifest.EncodeEnv(s.m)
	require.NoError(t, err)
	t.Setenv(setup.ManifestEnv, v)
	t.Chdir(primary)
	return s
}

// mem returns the memory dir of the repo i.
func (s *sandbox) mem(i int) string { return s.m.Repos[i].Memory }

// julieta runs julieta with args and returns the exit status, stdout and
// stderr.
func (s *sandbox) julieta(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr),
		Now: func() time.Time { return clock }, ULIDs: s.ulids, Version: stamp}
	code := cli.Main(t.Context(), args, env)
	return code, stdout.String(), stderr.String()
}

// report runs julieta with args and --json, which exits 0, and returns
// the report, read with the strict decoder.
func (s *sandbox) report(t *testing.T, args ...string) memory.Report {
	t.Helper()
	code, stdout, stderr := s.julieta(t, append(args, "--json")...)
	require.Equal(t, 0, code, stderr)
	rep, err := memory.Decode([]byte(stdout))
	require.NoError(t, err, stdout)
	return rep
}

// errorID returns the id of the --json error report on stderr.
func errorID(t *testing.T, stderr string) string {
	t.Helper()
	var e struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &e), stderr)
	return e.ID
}

// TestAddStampsAndPrints adds an entry in the repo of the working
// directory: julieta stamps the id and both times (REQ-0656), writes
// the file 0600 in a dir 0700 (REQ-0646), and prints the id, the path
// and the repo, in text and with --json (REQ-0280, REQ-0677).
func TestAddStampsAndPrints(t *testing.T) {
	s := newSandbox(t)
	rep := s.report(t, "memory", "add", "--title", "Flaky test", "--body", "On arm64.\n", "--tag", "ci", "--tag", "arm64")
	want, err := canon.NewULID(clock, canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{7}, 16))))
	require.NoError(t, err)
	path := filepath.Join(s.mem(0), "entries", want.String()+".md")
	assert.Equal(t, memory.Report{Schema: memory.Schema, Repo: "api", Wrote: true, Entries: []memory.Entry{{
		Body: "On arm64.\n", CreatedAt: clock, ID: want.String(), Path: path, Status: "open",
		Tags: []string{"arm64", "ci"}, Title: "Flaky test", UpdatedAt: clock,
	}}}, rep)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode())
	info, err = os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())

	code, stdout, stderr := s.julieta(t, "memory", "add", "--title", "Second")
	require.Equal(t, 0, code, stderr)
	assert.Regexp(t, `^added [0-9a-hjkmnp-tv-z]{26} `+s.mem(0)+`/entries/[0-9a-hjkmnp-tv-z]{26}\.md repo api\n$`, stdout)
}

// TestAddDuplicate prints the existing entry's id for content whose
// memory-content digest equals an open entry's, and writes nothing (04
// 4.1, Idempotency), so a rerun after a timeout adds nothing.
func TestAddDuplicate(t *testing.T) {
	s := newSandbox(t)
	args := []string{"memory", "add", "--title", "t", "--body", "b", "--tag", "x"}
	first := s.report(t, args...)
	again := s.report(t, args...)
	assert.False(t, again.Wrote)
	assert.Equal(t, first.Entries, again.Entries)
	entries, err := os.ReadDir(filepath.Join(s.mem(0), "entries"))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	code, stdout, _ := s.julieta(t, args...)
	require.Equal(t, 0, code)
	assert.Equal(t, "exists "+first.Entries[0].ID+" "+first.Entries[0].Path+" repo api (an open entry holds the same content; nothing written)\n", stdout)
}

// TestCallersCannotStamp refuses a supplied id or timestamp: no flag
// takes one, so it is the error usage (RJ-349), exit 2, and nothing is
// written (REQ-0241, REQ-0656).
func TestCallersCannotStamp(t *testing.T) {
	s := newSandbox(t)
	for _, extra := range [][]string{
		{"--id", "01j9z3k8m6t2q4w5e7r9y1v3k5"}, {"--createdAt", "2026-10-02T09:20:00Z"},
		{"--updatedAt", "2026-10-02T09:20:00Z"}, {"--created-at", "x"},
	} {
		code, _, stderr := s.julieta(t, append([]string{"memory", "add", "--title", "t", "--json"}, extra...)...)
		assert.Equal(t, 2, code, stderr)
		assert.Equal(t, "RJ-349", errorID(t, stderr))
	}
	code, _, stderr := s.julieta(t, "memory", "edit", "01j9z3k8m6t2q4w5e7r9y1v3k5", "--updatedAt", "x", "--json")
	assert.Equal(t, 2, code, stderr)
	_, err := os.Stat(filepath.Join(s.mem(0), "entries"))
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// TestListShowEdit filters with list --query, shows an entry, and
// closes one with edit --status done (04 4.3, REQ-0411); an edit that
// changes nothing writes nothing.
func TestListShowEdit(t *testing.T) {
	s := newSandbox(t)
	a := s.report(t, "memory", "add", "--title", "Flaky test on arm64", "--tag", "ci").Entries[0]
	b := s.report(t, "memory", "add", "--title", "Signing waits", "--body", "Ask the operator.\n", "--tag", "lesson").Entries[0]

	assert.Equal(t, []memory.Entry{a, b}, s.report(t, "memory", "list").Entries)
	assert.Equal(t, []memory.Entry{a}, s.report(t, "memory", "list", "--query", "ARM64").Entries)
	assert.Equal(t, []memory.Entry{b}, s.report(t, "memory", "list", "--query", "operator").Entries)
	assert.Empty(t, s.report(t, "memory", "list", "--query", "nothing").Entries)

	code, stdout, stderr := s.julieta(t, "memory", "list")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, a.ID+"  open  Flaky test on arm64  [ci]\n"+b.ID+"  open  Signing waits  [lesson]\n", stdout)

	assert.Equal(t, []memory.Entry{b}, s.report(t, "memory", "show", b.ID).Entries)
	code, stdout, stderr = s.julieta(t, "memory", "show", b.ID)
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "id       "+b.ID+"\ntitle    Signing waits\nstatus   open\ntags     lesson\ncreated  2026-10-20T12:00:00Z\n"+
		"updated  2026-10-20T12:00:00Z\npath     "+b.Path+"\nrepo     api\n\nAsk the operator.\n", stdout)

	done := s.report(t, "memory", "edit", a.ID, "--status", "done")
	assert.True(t, done.Wrote)
	assert.Equal(t, "done", done.Entries[0].Status)
	assert.Equal(t, []memory.Entry{b}, s.report(t, "memory", "list", "--status", "open").Entries)
	assert.Equal(t, []memory.Entry{done.Entries[0]}, s.report(t, "memory", "list", "--status", "done").Entries)
	assert.False(t, s.report(t, "memory", "edit", a.ID, "--status", "done").Wrote)

	code, stdout, stderr = s.julieta(t, "memory", "edit", b.ID, "--title", "Signing waits on the agent", "--tag", "lesson", "--tag", "signing", "--body", "x")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "edited "+b.ID+" "+b.Path+" repo api\n", stdout)
	code, stdout, stderr = s.julieta(t, "memory", "edit", b.ID, "--body", "x")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "unchanged "+b.ID+" "+b.Path+" repo api\n", stdout)
	got := s.report(t, "memory", "show", b.ID).Entries[0]
	assert.Equal(t, []string{"lesson", "signing"}, got.Tags)
}

// TestNoRemoveNoSearch has no memory rm and no memory search: each is
// the error usage (04 4.3).
func TestNoRemoveNoSearch(t *testing.T) {
	s := newSandbox(t)
	for _, sub := range []string{"rm", "search", "delete"} {
		code, _, stderr := s.julieta(t, "memory", sub)
		assert.Equal(t, 2, code, stderr)
	}
}

// TestRepoOfTheWorkingDirectory files an entry under the repo whose
// path holds the working directory, or the one --repo names; a
// working directory in no repo and an unknown --repo fail (08 8.3,
// REQ-0677).
func TestRepoOfTheWorkingDirectory(t *testing.T) {
	s := newSandbox(t)
	sub := filepath.Join(s.m.Repos[1].Path, "pkg")
	require.NoError(t, os.Mkdir(sub, 0o700))
	t.Chdir(sub)
	rep := s.report(t, "memory", "add", "--title", "in web")
	assert.Equal(t, "web", rep.Repo)
	assert.True(t, strings.HasPrefix(rep.Entries[0].Path, s.mem(1)+"/entries/"), rep.Entries[0].Path)
	assert.Equal(t, "api", s.report(t, "memory", "add", "--title", "in api", "--repo", "api").Repo)

	code, _, _ := s.julieta(t, "memory", "list", "--repo", "nope")
	assert.Equal(t, 1, code)
	t.Chdir(t.TempDir())
	code, _, stderr := s.julieta(t, "memory", "list")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "in no repo of the manifest")
}

// TestRepoThroughALink names the repo of a working directory reached
// through a symbolic link, as a TMPDIR under /var is on macOS.
func TestRepoThroughALink(t *testing.T) {
	s := newSandbox(t)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(s.m.Repos[0].Path, link))
	t.Chdir(link)
	assert.Equal(t, "api", s.report(t, "memory", "list").Repo)
}

// TestWithoutManifest exits 2 with manifest-protocol (RJ-342) when no
// manifest is set or cached (REQ-0425).
func TestWithoutManifest(t *testing.T) {
	s := newSandbox(t)
	t.Setenv(setup.ManifestEnv, "")
	for _, args := range [][]string{{"memory", "add", "--title", "t"}, {"memory", "list"}, {"memory", "check"}} {
		code, _, stderr := s.julieta(t, append(args, "--json")...)
		if args[1] == "check" {
			code, _, stderr = s.julieta(t, args...)
			assert.Equal(t, 2, code, stderr)
			continue
		}
		assert.Equal(t, 2, code, stderr)
		assert.Equal(t, "RJ-342", errorID(t, stderr))
	}
}

// TestReadersRefuse fails list and show on a memory dir the readers
// refuse: an entry of a newer version is format-newer (RJ-311, exit 2);
// a symlink in entries/ fails without being followed; an id that is no
// ULID, ".." among them, and a status other than open and done are the
// error usage (RJ-349, exit 2).
func TestReadersRefuse(t *testing.T) {
	s := newSandbox(t)
	entries := filepath.Join(s.mem(0), "entries")
	require.NoError(t, os.Mkdir(entries, 0o700))
	newer := "---\nschema: memory-entry/v2\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(entries, "01j9z3k8m6t2q4w5e7r9y1v3k5.md"), []byte(newer), 0o600))
	code, _, stderr := s.julieta(t, "memory", "list", "--json")
	assert.Equal(t, 2, code, stderr)
	assert.Equal(t, "RJ-311", errorID(t, stderr))
	code, _, stderr = s.julieta(t, "memory", "add", "--title", "t", "--json")
	assert.Equal(t, 2, code, stderr)
	assert.Equal(t, "RJ-311", errorID(t, stderr))

	require.NoError(t, os.Remove(filepath.Join(entries, "01j9z3k8m6t2q4w5e7r9y1v3k5.md")))
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(entries, "01j9z3k8m6t2q4w5e7r9y1v3k5.md")))
	for _, args := range [][]string{{"memory", "list"}, {"memory", "show", "01j9z3k8m6t2q4w5e7r9y1v3k5"}} {
		code, stdout, _ := s.julieta(t, args...)
		assert.Equal(t, 1, code, args)
		assert.NotContains(t, stdout, "root:")
	}
	for _, args := range [][]string{{"memory", "show", ".."}, {"memory", "list", "--status", "closed"}} {
		code, _, stderr := s.julieta(t, append(args, "--json")...)
		assert.Equal(t, 2, code, args)
		assert.Equal(t, "RJ-349", errorID(t, stderr), args)
	}
}

// TestBadValuesAreUsage refuses a flag or argument value the format
// does not admit as the error usage (RJ-349, exit 2; REQ-0273), with
// the help of the command as its hint, and writes nothing.
func TestBadValuesAreUsage(t *testing.T) {
	s := newSandbox(t)
	for _, args := range [][]string{
		{"memory", "add"},
		{"memory", "add", "--title", ""},
		{"memory", "add", "--title", "a\nb"},
		{"memory", "add", "--title", "t", "--status", "closed"},
		{"memory", "add", "--title", "t", "--tag", "Bad Tag"},
		{"memory", "add", "--title", "t", "--body", "\xff"},
		{"memory", "edit", "01j9z3k8m6t2q4w5e7r9y1v3k5", "--status", "closed"},
		{"memory", "edit", "01j9z3k8m6t2q4w5e7r9y1v3k5", "--tag", ""},
		{"memory", "edit", "../x", "--status", "done"},
	} {
		code, _, stderr := s.julieta(t, append(args, "--json")...)
		assert.Equal(t, 2, code, "%q: %s", args, stderr)
		assert.Equal(t, "RJ-349", errorID(t, stderr), args)
	}
	code, _, stderr := s.julieta(t, "memory", "add", "--title", "t", "--status", "closed")
	require.Equal(t, 2, code)
	assert.Contains(t, stderr, "fix: julieta memory add --help\n")
	_, err := os.Stat(filepath.Join(s.mem(0), "entries"))
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// TestCheckEscapesNames prints a name an agent planted, holding a line
// feed and an escape sequence, escaped: it cannot forge a line of the
// output or reach the terminal raw (I29).
func TestCheckEscapesNames(t *testing.T) {
	s := newSandbox(t)
	mem := s.mem(0)
	require.NoError(t, os.WriteFile(filepath.Join(mem, "x\nmemory web ok\x1b[2J"), nil, 0o600))
	code, stdout, stderr := s.julieta(t, "memory", "check")
	assert.Equal(t, 6, code, stderr)
	assert.Equal(t, "memory api "+mem+" 1 findings\nfinding top-level "+mem+"/x\\nmemory web ok\\u001b[2J\nmemory web "+s.mem(1)+" ok\n", stdout)
	assert.NotContains(t, stderr, "\x1b")
}

// TestCheck passes on valid memory dirs and notes a leftover temporary
// file on stderr, which refuses nothing (REQ-0654); it fails, exit 1,
// on a dir that is not mounted (REQ-0412).
func TestCheck(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, os.Mkdir(filepath.Join(s.mem(0), "entries"), 0o700))
	temp := filepath.Join(s.mem(0), "entries", ".tmp-01j9z3k8m6t2q4w5e7r9y1v3k5")
	require.NoError(t, os.WriteFile(temp, nil, 0o600))
	code, stdout, stderr := s.julieta(t, "memory", "check")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "memory api "+s.mem(0)+" ok\nmemory web "+s.mem(1)+" ok\n", stdout)
	assert.Equal(t, "note temp-file "+temp+": a leftover julieta temporary file, never read; julieta setup removes it\n", stderr)

	require.NoError(t, os.RemoveAll(s.mem(1)))
	code, stdout, _ = s.julieta(t, "memory", "check")
	assert.Equal(t, 1, code)
	assert.Contains(t, stdout, "memory web "+s.mem(1)+" error: memstore: the memory dir is not mounted")
}

// TestCheckFindings fails on a memory dir that breaks the layout
// allowlist and names each finding (REQ-0412, REQ-0651): a symlink, a
// FIFO, a dotfile, a .git directory and a top-level name outside the
// four, as the error check-findings (RJ-328), exit 6 (REQ-0435), whose
// details name each path with its slug.
func TestCheckFindings(t *testing.T) {
	s := newSandbox(t)
	mem := s.mem(0)
	for _, d := range []string{"entries", "snapshot/.git", "notes"} {
		require.NoError(t, os.MkdirAll(filepath.Join(mem, d), 0o700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(mem, "entries", ".hidden"), nil, 0o600))
	require.NoError(t, os.Symlink("/etc", filepath.Join(mem, "handoff")))
	require.NoError(t, syscall.Mkfifo(filepath.Join(mem, "entries", "fifo"), 0o600))
	code, stdout, stderr := s.julieta(t, "memory", "check")
	assert.Equal(t, 6, code, stderr)
	assert.True(t, strings.HasPrefix(stderr, "error RJ-328 check-findings: "), stderr)
	assert.Contains(t, stderr, "\nfix: decide: fix each finding the details name, then run the check again\n")
	assert.Contains(t, stderr, "\n"+mem+"/handoff: symlink\n")
	assert.Equal(t, "memory api "+mem+" 5 findings\n"+
		"finding dotfile "+mem+"/entries/.hidden\n"+
		"finding fifo "+mem+"/entries/fifo\n"+
		"finding symlink "+mem+"/handoff\n"+
		"finding top-level "+mem+"/notes\n"+
		"finding git "+mem+"/snapshot/.git\n"+
		"memory web "+s.mem(1)+" ok\n", stdout)
	assert.Contains(t, stderr, "dotfile, fifo, symlink, top-level, git")
}

// TestDecodeIgnoresAnUnknownMember reads a document to which a newer
// julieta added a member, at every level, as the document without it
// (03, opening, REQ-0163).
func TestDecodeIgnoresAnUnknownMember(t *testing.T) {
	valid, err := os.ReadFile(filepath.Join("testdata", "valid.json"))
	require.NoError(t, err)
	want, err := memory.Decode(valid)
	require.NoError(t, err)
	// Every object of the document gains a member, as a newer julieta
	// that added one would write it.
	newer := strings.ReplaceAll(strings.ReplaceAll(string(valid), "{", `{"added":{"a":[1]},`), ",}", "}")
	got, err := memory.Decode([]byte(newer))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestDecodeIsStrict refuses a duplicate member, by a fixture, and
// another schema.
func TestDecodeIsStrict(t *testing.T) {
	for name, valid := range map[string]bool{"valid.json": true, "invalid-duplicate-member.json": false} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)
		_, err = memory.Decode(data)
		if valid {
			require.NoError(t, err, name)
		} else {
			require.Error(t, err, name)
		}
	}
	_, err := memory.Decode([]byte(`{"entries":[],"repo":"api","schema":"julieta-memory/v2","wrote":false}`))
	require.Error(t, err)
}

// TestEntryFileIsTheFormat writes a file the memstore decoder reads as
// the entry the command printed.
func TestEntryFileIsTheFormat(t *testing.T) {
	s := newSandbox(t)
	e := s.report(t, "memory", "add", "--title", "t", "--body", "b\n").Entries[0]
	data, err := os.ReadFile(e.Path)
	require.NoError(t, err)
	got, err := memstore.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, e.ID, got.ID.String())
	assert.True(t, strings.HasPrefix(string(data), "---\nschema: memory-entry/v1\nid: "+e.ID+"\n"), string(data))
}
