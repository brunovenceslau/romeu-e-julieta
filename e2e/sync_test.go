// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"bytes"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/e2e/internal/romeu"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j11"
	"github.com/brunovenceslau/romeu-e-julieta/e2e/scenarios/j2"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// TestSyncHarnessEmbedsJulieta: the romeu of romeu.NewEmbedded, the
// host-native build of tools/release, carries a julieta (REQ-0131, the
// e2e row of 10 10.2), which gate 1 reads: it prints no placeholder,
// and records a dev build's version and a sha256 that is not the
// placeholder's, the sha256 of no bytes.
func TestSyncHarnessEmbedsJulieta(t *testing.T) {
	t.Parallel()
	e := romeu.NewEmbedded(t, nil)
	e.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}})
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, e.SetSbxSession(session))

	res := approve(t, e)

	assert.NotContains(t, res.Stderr, "v0.0.0-placeholder")
	assert.NotContains(t, res.Stderr, "sha256 e3b0c44298fc")
	r := readToolchain(t, e)
	assert.True(t, strings.HasPrefix(r.Julieta.Version, "v0.0.0-dev+"), r.Julieta.Version)
	assert.Len(t, r.Julieta.SHA256, 64)
	assert.NotEqual(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", r.Julieta.SHA256)
}

// syncSetup is an Env, embedded or plain, with host settings and the
// fake sbx replaying the handshake of the recorded sbx 0.49.2, which
// answers sbx version.
func syncSetup(t *testing.T, embedded bool) *romeu.Env {
	t.Helper()
	e := romeu.New
	if embedded {
		e = romeu.NewEmbedded
	}
	env := e(t, nil)
	env.WriteSettings(t, all.HostSettings{Config: all.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"}})
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, env.SetSbxSession(session))
	return env
}

// TestSyncRefusesABuildWithoutJulieta: sync is linked into romeu, and a
// plain go build, which embeds no julieta (REQ-0130), exits 2 with
// julieta-not-embedded (RJ-312) at step 1, before it takes the lock,
// calls sbx or writes anything (04 4.2, REQ-0335).
func TestSyncRefusesABuildWithoutJulieta(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, false)

	res := e.Run(t, []string{"sync", "--all"})

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "julieta-not-embedded")
	assert.Empty(t, e.SbxCalls(t), "no sbx call")
	assert.NoDirExists(t, e.StateDir(), "no state is written")
	assert.NoDirExists(t, e.Root, "nothing is written under the root")
	res = e.Run(t, []string{"version"})
	assert.Equal(t, 0, res.Code, "version works on the same build: %s", res.Stderr)
	assert.Contains(t, res.Stdout, "protocol")
}

// TestSyncStopsAtGate1: the romeu of the embedded build passes the
// julieta check of step 1 and stops at gate 1 with
// toolchain-approval-required (RJ-302, exit 3) on a toolchain not
// acknowledged, having read sbx version alone, so a stale toolchain
// stops sync before it promotes (04 4.2, REQ-0335).
func TestSyncStopsAtGate1(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, true)

	res := e.Run(t, []string{"sync", "--all"})

	assert.Equal(t, 3, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "toolchain-approval-required")
	assert.NotContains(t, res.Stderr, "julieta-not-embedded")
	assert.Equal(t, [][]string{{"version"}}, e.SbxCalls(t))
	assert.NoDirExists(t, e.Root, "nothing is written under the root")
}

// TestSyncPassesGate1: once the toolchain is acknowledged, sync passes
// step 1 and stops at step 2 when no clone of the config repo exists,
// exit 2 naming romeu init, with nothing written under the root.
func TestSyncPassesGate1(t *testing.T) {
	t.Parallel()
	e := syncSetup(t, true)
	approve(t, e)

	res := e.Run(t, []string{"sync", "--all"})

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "config-clone-missing")
	assert.NoDirExists(t, e.Root, "nothing is written under the root")
}

// The project of the sync cases: foo, two repos of the origins, the
// primary on its branch main and the other on its origin's HEAD, the
// product kit julieta, and memory kept on the machine (08 8.1,
// REQ-1305).
const (
	// syncWorkload is the workload of foo, and its digest the index
	// digest the descriptor cache is looked up by.
	syncWorkload = "docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5"
	syncIndex    = "sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5"
	// syncManifest stands for the platform manifest the index names:
	// another digest, so a cache looked up by the index digest alone
	// would miss it.
	syncManifest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	fooSpec      = "schema: project/v1\nname: foo\nrepos:\n" +
		"  - url: https://github.com/owner/foo-api\n    primary: true\n    ref: main\n" +
		"  - url: https://github.com/owner/foo-web\n" +
		"workload: " + syncWorkload + "\nagent: sbx-kit-claude\nkits:\n  - product: julieta\nmemory: {remote: none}\n"
)

// syncFixture is an embedded Env whose origins hold the config repo
// owner/cfg with the spec of foo and foo's two repos, after romeu init,
// gate 1, and the descriptor cache entry of foo's workload.
type syncFixture struct {
	e *romeu.Env
	o *romeu.Origins
}

// newSyncFixture returns the fixture.
func newSyncFixture(t *testing.T) syncFixture {
	t.Helper()
	o := romeu.NewOrigins(t)
	url := o.PushConfigRepo(t, "owner", "cfg", map[string]string{"projects/foo.yaml": fooSpec})
	o.Push(t, "owner/foo-api", "main", map[string]string{"README": "foo-api"})
	o.Push(t, "owner/foo-web", "main", map[string]string{"README": "foo-web"})
	e := romeu.NewEmbedded(t, o)
	res := e.Run(t, []string{"init", "--config-url", url})
	require.Equal(t, 0, res.Code, res.Stderr)
	e.AppendSettings(t, "gitHosts:\n  - host: github.com\nworkloadRepositories:\n  - docker.io/docker/sbx-kit-claude\n")
	session, err := filepath.Abs(filepath.Join("testdata", "sbx", "0.49.2", "handshake.jsonl"))
	require.NoError(t, err)
	require.NoError(t, e.SetSbxSession(session))
	approve(t, e)
	e.SeedDescriptor(t, syncIndex, syncManifest)
	return syncFixture{e: e, o: o}
}

// env returns the path p, slash-separated, below foo-env/.
func (f syncFixture) env(p string) string {
	return filepath.Join(f.e.Root, "foo-env", filepath.FromSlash(p))
}

// approveFoo runs romeu sync foo on a terminal answered y.
func (f syncFixture) approveFoo(t *testing.T) romeu.Result {
	t.Helper()
	res := f.e.RunTerminal(t, romeu.GatePrompt, "y\n", []string{"sync", "foo"})
	require.Equal(t, 0, res.Code, res.Stderr)
	return res
}

// candidates returns the names under foo's candidates dir.
func (f syncFixture) candidates(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.env(".romeu/candidates"))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// workspaceFolders returns the folders of the workspace file name.
func (f syncFixture) workspaceFolders(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.e.Root, name))
	require.NoError(t, err)
	var doc struct {
		Folders []struct {
			Path string `json:"path"`
		} `json:"folders"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	out := []string{}
	for _, d := range doc.Folders {
		out = append(out, d.Path)
	}
	return out
}

// TestSyncJ2Step3 runs step 3 of J2 as the scenario function reads it
// from commands.yaml: sync clones the repos from origins served over
// TLS, makes foo-env/ and its memory dirs, renders and reaches gate 2,
// which, with no terminal in the world of a scenario, ends with exit 3
// and nothing promoted (04 4.2; 01 1.6, REQ-1210).
func TestSyncJ2Step3(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	tbl, err := scenarios.Load("j2")
	require.NoError(t, err)

	s := j2.J2(tbl, j2.Vars{Name: "foo", Dirs: []string{"foo-api", "foo-web"}})

	require.NoError(t, s.Run(f.e.World(t)))
	assert.Empty(t, tbl.Unread())
	assert.Equal(t, [][]string{{"version"}, {"version"}}, f.e.SbxCalls(t), "gate 1 of approve and of sync read sbx version alone")
}

// TestSyncApprovesOnYAndPromotes: the pty case "sync approves on y and
// promotes" (I7): on a terminal the gate 2 diff and its prompt go to
// stderr, never stdout, and y promotes: the live sbxenv.yaml, render.json
// and the julieta of .romeu/bin, the approval recorded, no candidate
// dir left, and the workspace files listing foo's clones (REQ-0048).
func TestSyncApprovesOnYAndPromotes(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)

	res := f.approveFoo(t)

	assert.Contains(t, res.Stderr, "gate 2: widening of the project foo")
	assert.NotContains(t, res.Stdout, romeu.GatePrompt, "the prompt goes to stderr")
	assert.Contains(t, res.Stdout, "promoted  foo")
	for _, p := range []string{"sbxenv.yaml", ".romeu/render.json", ".romeu/bin/julieta-linux-" + runtime.GOARCH} {
		assert.FileExists(t, f.env(p))
	}
	assert.Empty(t, f.candidates(t))
	live, err := os.ReadFile(f.env("sbxenv.yaml"))
	require.NoError(t, err)
	assert.FileExists(t, f.env(".metadata_never_index"))
	assert.NotContains(t, string(live), ".metadata_never_index", "the marker lies outside every mount (REQ-0138)")
	assert.NotRegexp(t, `(?m)(workspace: \.|- \.)/?$`, string(live), "foo-env/ itself is no mount")
	assert.FileExists(t, filepath.Join(f.e.StateDir(), "projects", "foo.json"))
	assert.Equal(t, []string{"foo-env/foo-api", "foo-env/foo-web"}, f.workspaceFolders(t, "dev.code-workspace"))
	assert.Empty(t, f.workspaceFolders(t, "review.code-workspace"))
}

// TestSyncDeclinesOnNAndDeletesTheCandidate: the pty case "sync
// declines on N and deletes the candidate" (I7): N exits 1 with
// candidate-declined naming foo, no candidate dir remains, and nothing
// is promoted or recorded.
func TestSyncDeclinesOnNAndDeletesTheCandidate(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)

	res := f.e.RunTerminal(t, romeu.GatePrompt, "N\n", []string{"sync", "foo"})

	assert.Equal(t, 1, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "candidate-declined")
	assert.Contains(t, res.Stderr, "romeu sync foo")
	assert.Empty(t, f.candidates(t))
	assert.NoFileExists(t, f.env("sbxenv.yaml"))
	assert.NoFileExists(t, filepath.Join(f.e.StateDir(), "projects", "foo.json"))
	assert.NoFileExists(t, filepath.Join(f.e.Root, "dev.code-workspace"), "a run that promotes nothing writes no workspace file")
}

// TestSyncJ11Step5 runs step 5 of J11 as the scenario function reads it:
// after an approval, a spec change that widens egress is pushed, and a
// sync without a terminal exits 3 with nothing kept (01 1.6,
// REQ-1210); the rerun on a terminal, answered y, promotes it: its
// render.json records the new domain (live egress is step 9, T047).
func TestSyncJ11Step5(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	live, err := os.ReadFile(f.env("sbxenv.yaml"))
	require.NoError(t, err)
	f.o.Push(t, "owner/cfg", "main", map[string]string{"projects/foo.yaml": fooSpec + "egress:\n  extra: [api.example.com]\n"})
	tbl, err := scenarios.Load("j11")
	require.NoError(t, err)

	s := j11.J11(tbl, j11.Vars{Name: "foo", Live: live})

	require.NoError(t, s.Run(f.e.World(t)))
	assert.Empty(t, tbl.Unread())
	rec, err := os.ReadFile(f.env(".romeu/render.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(rec), "api.example.com", "nothing of the change is live")
	res := f.approveFoo(t)
	assert.Contains(t, res.Stderr, "api.example.com", "the diff names the new domain")
	rec, err = os.ReadFile(f.env(".romeu/render.json"))
	require.NoError(t, err)
	assert.Contains(t, string(rec), "api.example.com", "the approved egress is promoted")
}

// The personal kit mine of the hostile and capability cases: its
// descriptor, the same with a capability a personal kit may hold, and
// foo's spec with the kit added.
const (
	mineKit     = "schemaVersion: \"3\"\nkind: mixin\ndisplayName: mine\ndescription: a personal kit\n"
	mineWithCap = mineKit + "capabilities:\n  - type: com.docker.sandbox/privileged@1\n"
)

// fooWithMine is fooSpec with the personal kit mine after julieta.
var fooWithMine = strings.Replace(fooSpec, "  - product: julieta\n", "  - product: julieta\n  - personal: mine\n", 1)

// liveTree returns the snapshot of foo-env/ without what is below
// .romeu/candidates/.
func (f syncFixture) liveTree(t *testing.T) map[string]romeu.Node {
	t.Helper()
	nodes := romeu.Snapshot(t, f.env(""))
	for p := range nodes {
		if strings.HasPrefix(p, ".romeu/candidates/") {
			delete(nodes, p)
		}
	}
	return nodes
}

// hostileEntry is one entry of a tree object writeTree stores.
type hostileEntry struct{ mode, name, oid string }

// writeTree stores in the repository dir a tree object of the entries,
// sorted as git sorts them, with hash-object --literally, which checks
// nothing, so a tree no other git command would write can be built.
func writeTree(t *testing.T, o *romeu.Origins, dir string, entries []hostileEntry) string {
	t.Helper()
	key := func(e hostileEntry) string {
		if e.mode == "40000" {
			return e.name + "/"
		}
		return e.name
	}
	slices.SortFunc(entries, func(a, b hostileEntry) int { return strings.Compare(key(a), key(b)) })
	var b bytes.Buffer
	for _, e := range entries {
		raw, err := hex.DecodeString(e.oid)
		require.NoError(t, err)
		b.WriteString(e.mode + " " + e.name)
		b.WriteByte(0)
		b.Write(raw)
	}
	p := filepath.Join(t.TempDir(), "tree")
	require.NoError(t, os.WriteFile(p, b.Bytes(), 0o600))
	return strings.TrimSpace(o.Git(t, dir, "hash-object", "-w", "-t", "tree", "--literally", p))
}

// readTree returns the entries of the tree rev of the repository dir.
func readTree(t *testing.T, o *romeu.Origins, dir, rev string) []hostileEntry {
	t.Helper()
	var out []hostileEntry
	for rec := range strings.SplitSeq(strings.TrimSuffix(o.Git(t, dir, "ls-tree", "-z", rev), "\x00"), "\x00") {
		meta, name, ok := strings.Cut(rec, "\t")
		require.True(t, ok, rec)
		f := strings.Fields(meta)
		require.Len(t, f, 3, rec)
		// ls-tree prints a tree's mode 040000; a tree object stores
		// 40000, and fsck refuses the zero-padded form.
		out = append(out, hostileEntry{strings.TrimPrefix(f[0], "0"), name, f[2]})
	}
	return out
}

// hostileConfig is a work clone of the config repo owner/cfg whose
// commit main adds the personal kit mine to foo's spec.
type hostileConfig struct {
	o    *romeu.Origins
	work string
	// base is that commit, and blob a blob of it.
	base, blob string
}

// newHostileConfig clones owner/cfg, commits fooWithMine and the
// descriptor of mine on main, and pushes nothing.
func newHostileConfig(t *testing.T, o *romeu.Origins) hostileConfig {
	t.Helper()
	h := hostileConfig{o: o, work: filepath.Join(t.TempDir(), "cfg")}
	o.Git(t, filepath.Dir(h.work), "clone", "-q", o.Bare("owner/cfg"), h.work)
	for p, data := range map[string]string{"projects/foo.yaml": fooWithMine, "kits/mine/mine.yaml": mineKit} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(h.work, p)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(h.work, p), []byte(data), 0o600))
	}
	o.Git(t, h.work, "add", "-A")
	o.Git(t, h.work, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "mine")
	h.base = strings.TrimSpace(o.Git(t, h.work, "rev-parse", "HEAD"))
	h.blob = strings.TrimSpace(o.Git(t, h.work, "rev-parse", "HEAD:kits/mine/mine.yaml"))
	return h
}

// commit returns a child of base whose kit mine also holds extra.
func (h hostileConfig) commit(t *testing.T, extra ...hostileEntry) string {
	t.Helper()
	mine := writeTree(t, h.o, h.work, append(readTree(t, h.o, h.work, h.base+":kits/mine"), extra...))
	kits := writeTree(t, h.o, h.work, []hostileEntry{{"40000", "mine", mine}})
	var root []hostileEntry
	for _, e := range readTree(t, h.o, h.work, h.base) {
		if e.name == "kits" {
			e.oid = kits
		}
		root = append(root, e)
	}
	tree := writeTree(t, h.o, h.work, root)
	return strings.TrimSpace(h.o.Git(t, h.work, "commit-tree", tree, "-p", h.base, "-m", "hostile"))
}

// TestSyncHostileKit is the E case of I27 for each hostile entry: a
// config commit whose personal kit mine holds the entry, pushed to a
// branch of the origin and named with --from, makes sync fail before a
// file is written outside .romeu/candidates/, with no candidate dir
// left. The walk of step 6 refuses a symbolic link, a gitlink and the
// two collisions with kit-refused, and git fsck --strict at step 3 a
// stored mode 100664, each exit 2. The fetch of step 2 already refuses
// a .GIT name, a .git with a U+200C inserted after the dot and a ".."
// name, since gitsafe fetches with fsck on; today that refusal is the
// error internal (RJ-348), exit 1, and its own id and exit 2 are T188's
// (decided on 2026-10-30), so those cases assert the refusal and not
// its exit. The cases share one fixture: each refusal leaves it as the
// approval did, and each case deletes its branch from the origin,
// since the next one fetches every branch of it.
func TestSyncHostileKit(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	h := newHostileConfig(t, f.o)
	gitlink := func() string {
		sub := writeTree(t, f.o, h.work, []hostileEntry{{"100644", "a", h.blob}})
		return strings.TrimSpace(f.o.Git(t, h.work, "commit-tree", sub, "-m", "sub"))
	}
	tests := []struct {
		name  string
		extra []hostileEntry
		// code is the exit, 0 for a refusal whose exit is T188's.
		code int
		want []string
	}{
		{"symbolic link", []hostileEntry{{"120000", "link", h.blob}}, 2, []string{"kit-refused", "symbolic link"}},
		{"gitlink", []hostileEntry{{"160000", "module", gitlink()}}, 2, []string{"kit-refused", "gitlink"}},
		{"case collision", []hostileEntry{{"100644", "README", h.blob}, {"100644", "readme", h.blob}}, 2, []string{"kit-refused", "are one path"}},
		{"normalization collision", []hostileEntry{{"100644", "caf\u00e9", h.blob}, {"100644", "cafe\u0301", h.blob}}, 2, []string{"kit-refused", "are one path"}},
		{".GIT", []hostileEntry{{"100644", ".GIT", h.blob}}, 0, []string{"fsck error"}},
		{".git with an HFS-ignorable character", []hostileEntry{{"100644", ".\u200cgit", h.blob}}, 0, []string{"fsck error"}},
		{"dot dot", []hostileEntry{{"100644", "..", h.blob}}, 0, []string{"fsck error"}},
		{"mode 100664", []hostileEntry{{"100664", "a", h.blob}}, 2, []string{"spec-commit-unsafe", "badFilemode"}},
	}
	before := f.liveTree(t)
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sha := h.commit(t, tt.extra...)
			branch := fmt.Sprintf("refs/heads/hostile-%d", i)
			f.o.Git(t, h.work, "push", "-q", "origin", sha+":"+branch)
			// The next case fetches every branch of the origin, and an
			// object fsck refuses would stop its fetch, so the branch
			// goes even when the case fails; the clone keeps its copy
			// under refs/romeu/origin/, which no later case reads.
			defer f.o.Git(t, h.work, "push", "-q", "origin", ":"+branch)

			res := f.e.Run(t, []string{"sync", "--from", sha, "foo"})

			if tt.code != 0 {
				assert.Equal(t, tt.code, res.Code, res.Stderr)
			} else {
				assert.NotEqual(t, 0, res.Code, res.Stderr)
			}
			for _, w := range tt.want {
				assert.Contains(t, res.Stderr, w)
			}
			assert.Equal(t, before, f.liveTree(t), "nothing is written outside .romeu/candidates/")
			assert.Empty(t, f.candidates(t))
		})
	}
}

// TestSyncHostileKitDeliveredByABundle is the bundle case of I27: a
// config commit whose personal kit holds a symbolic link, which git
// fsck --strict passes, reaches the config repo's clone by a bundle
// fetched into refs/heads/, outside refs/romeu/origin/. sync --from
// <sha> refuses it at step 2 with from-unreachable; once the commit is
// pushed to a branch of the origin, step 2 takes it and the walk of
// step 6 refuses the kit with kit-refused. Either way sync exits 2,
// leaves no candidate dir, and nothing of foo-env/ outside
// .romeu/candidates/ changes from what the approval left.
func TestSyncHostileKitDeliveredByABundle(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	work := filepath.Join(t.TempDir(), "cfg")
	f.o.Git(t, filepath.Dir(work), "clone", "-q", f.o.Bare("owner/cfg"), work)
	for p, data := range map[string]string{"projects/foo.yaml": fooWithMine, "kits/mine/mine.yaml": mineKit} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(work, p)), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(work, p), []byte(data), 0o600))
	}
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(work, "kits", "mine", "link")))
	f.o.Git(t, work, "add", "-A")
	f.o.Git(t, work, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "hostile")
	sha := strings.TrimSpace(f.o.Git(t, work, "rev-parse", "HEAD"))
	bundle := filepath.Join(t.TempDir(), "hostile.bundle")
	f.o.Git(t, work, "bundle", "create", "-q", bundle, "main")
	clone := filepath.Join(f.e.Root, "cfg-env", "cfg")
	f.o.Git(t, clone, "fetch", "-q", bundle, "main:refs/heads/delivered")
	before := f.liveTree(t)

	res := f.e.Run(t, []string{"sync", "--from", sha, "foo"})

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "from-unreachable")
	assert.Contains(t, res.Stderr, "is not reachable from", "the clone holds the commit")
	assert.Equal(t, before, f.liveTree(t))
	assert.Empty(t, f.candidates(t))

	f.o.Git(t, work, "push", "-q", "origin", "HEAD:refs/heads/hostile")
	res = f.e.Run(t, []string{"sync", "--from", sha, "foo"})

	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "kit-refused")
	assert.Contains(t, res.Stderr, "symbolic link")
	assert.Equal(t, before, f.liveTree(t), "nothing is written outside .romeu/candidates/")
	assert.Empty(t, f.candidates(t))
}

// TestSyncFlipsGate2OnAKitCapability is the E case of I28: once foo
// with its personal kit mine is approved, a commit that adds a
// capability to the kit's descriptor makes a sync without a terminal
// stop at gate 2, exit 3 with project-approval-required.
func TestSyncFlipsGate2OnAKitCapability(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.o.Push(t, "owner/cfg", "main", map[string]string{"projects/foo.yaml": fooWithMine, "kits/mine/mine.yaml": mineKit})
	f.approveFoo(t)
	f.o.Push(t, "owner/cfg", "main", map[string]string{"kits/mine/mine.yaml": mineWithCap})

	res := f.e.Run(t, []string{"sync", "foo"})

	assert.Equal(t, 3, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "project-approval-required")
	assert.Empty(t, f.candidates(t))
}

// idempotencyRows is the idempotency meta-test of 10 10.1 (REQ-0277): a
// converging command, run a second time on unchanged inputs, leaves the
// content and mode of every file of the root and of host state, every
// ref of a clone among them, as they were, except lastRunAt. Each task
// that adds a converging or an appending command adds its row here,
// and T073 checks that no command lacks one.
var idempotencyRows = []struct {
	// command is the command line of the row.
	command string
	// first readies the fixture and runs the command once.
	first func(t *testing.T) (*romeu.Env, []string)
}{
	{"sync", func(t *testing.T) (*romeu.Env, []string) {
		t.Helper()
		f := newSyncFixture(t)
		f.approveFoo(t)
		return f.e, []string{"sync", "foo"}
	}},
}

// TestSyncIdempotency runs each row of idempotencyRows: the second run,
// without a terminal, exits 0 and changes nothing but lastRunAt. For
// sync, the second run reads its multi-arch workload's descriptor from
// the cache by the index digest and the platform, with no registry read:
// the cache keeps its one entry (REQ-0571).
func TestSyncIdempotency(t *testing.T) {
	t.Parallel()
	for _, row := range idempotencyRows {
		t.Run(row.command, func(t *testing.T) {
			t.Parallel()
			e, args := row.first(t)
			root, state := romeu.Snapshot(t, e.Root), romeu.Snapshot(t, e.StateDir())

			res := e.Run(t, args)

			require.Equal(t, 0, res.Code, res.Stderr)
			assert.Equal(t, root, romeu.Snapshot(t, e.Root), "the root is as it was")
			after := romeu.Snapshot(t, e.StateDir())
			for p := range after {
				if strings.HasPrefix(p, "projects/") && strings.HasSuffix(p, ".json") {
					after[p] = withoutLastRunAt(t, after[p])
					state[p] = withoutLastRunAt(t, state[p])
				}
			}
			assert.Equal(t, state, after, "host state is as it was, but lastRunAt")
			entries, err := os.ReadDir(filepath.Join(e.StateDir(), "descriptors"))
			require.NoError(t, err)
			assert.Len(t, entries, 1, "no registry read wrote another entry")
		})
	}
}

// withoutLastRunAt returns the node of a project record with its
// lastRunAt member removed.
func withoutLastRunAt(t *testing.T, n romeu.Node) romeu.Node {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(n.Data), &doc))
	delete(doc, "lastRunAt")
	data, err := json.Marshal(doc, json.Deterministic(true))
	require.NoError(t, err)
	n.Data = string(data)
	return n
}
