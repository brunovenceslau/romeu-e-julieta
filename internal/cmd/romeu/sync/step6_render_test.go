// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/kits"
)

// The unit tests of step 6 render on the host tree of step 4's
// fixture, with fixtures in place of the registry (fetchWorkload) and
// of the product kits (productKits): the guarded client reads public
// HTTPS registries alone, and the real kits are the e2e's.

// The descriptors of the fixture kits: a mixin with nothing, the same
// with a capability the gate shows, one that declares the arg
// julietaBin, as julieta-claude does, one that allows a host at
// runtime, written with a port as the product kits write theirs, and
// one that allows a host at install alone.
const (
	plainKit     = "schemaVersion: \"3\"\nkind: mixin\ndisplayName: plain\ndescription: a fixture\n"
	lifecycleKit = plainKit + "capabilities:\n  - type: com.docker.sandbox/privileged@1\n"
	binKit       = plainKit + "args:\n  julietaBin:\n    default: \"\"\n    description: the path of julieta\n"
	networkKit   = plainKit + "capabilities:\n  - type: com.docker.sandbox/network-policy@2\n    config:\n      runtime: {allow: [\"kit.example.com:443\"]}\n"
	installKit   = plainKit + "capabilities:\n  - type: com.docker.sandbox/network-policy@2\n    config:\n      install: {allow: [\"install.example.com:443\"]}\n"
)

// The two workload references of the fixture: the pinned one of
// projectYAML, and another digest of the same repository.
const (
	workloadRef   = "docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5"
	workloadOther = "docker.io/docker/sbx-kit-claude@sha256:1111111111111111111111111111111111111111111111111111111111111111"
)

// renderFixture is a run whose steps 1 to 5 are done on the project
// shop of treeRun, with a config clone, the state store and its lock,
// and the fake registry, which counts its reads.
type renderFixture struct {
	*hostTree
	cfg string
	// reads are the workload refs the registry was asked for.
	reads []string
	// registry answers a read; nil reads as a manifest of its own
	// digest with no capability.
	registry func(ref string) (oci.Fetched, error)
}

// renderRun returns the fixture: the target shop has the workload of
// workloadRef and the product kit plain.
func renderRun(t *testing.T) *renderFixture {
	t.Helper()
	return renderRunOf(t, nil)
}

// renderRunOf is renderRun with setup, when not nil, run on the host
// tree before step 4, as addTarget is; every target has the workload of
// workloadRef and the product kit plain.
func renderRunOf(t *testing.T, setup func(h *hostTree)) *renderFixture {
	t.Helper()
	f := &renderFixture{hostTree: treeRun(t)}
	if setup != nil {
		setup(f.hostTree)
	}
	require.NoError(t, makeTree(t.Context(), f.r))
	tmp := t.TempDir()
	st, err := state.Open(filepath.Join(tmp, "state"))
	require.NoError(t, err)
	lk, err := st.Lock()
	require.NoError(t, err)
	t.Cleanup(func() { lk.Unlock(); _ = st.Close() })
	f.r.store, f.r.lock = st, lk
	f.r.bin, err = render.NewBin(runtime.GOARCH, []byte("julieta"))
	require.NoError(t, err)
	f.cfg = filepath.Join(f.root, "cfg-env", "cfg")
	require.NoError(t, os.MkdirAll(f.cfg, 0o700))
	gitIn(t, f.cfg, "init", "-q", "-b", "main")
	f.r.configClone, f.r.specCommit = f.cfg, commitFiles(t, f.cfg, map[string]string{"README": "cfg"})
	for i := range f.r.targets {
		p := &f.r.targets[i]
		p.Workload, p.Agent, p.Kits = workloadRef, "sbx-kit-claude", []spec.Kit{{Product: "plain"}}
	}

	oldFetch, oldKits := fetchWorkload, productKits
	fetchWorkload = func(_ context.Context, ref string, _ oci.Platform) (oci.Fetched, error) {
		f.reads = append(f.reads, ref)
		if f.registry != nil {
			return f.registry(ref)
		}
		w, err := spec.ParseWorkload(ref)
		if err != nil {
			return oci.Fetched{}, err
		}
		return oci.Fetched{Workload: w, ManifestDigest: w.Digest, Kit: oci.Kit{Capabilities: oci.Capabilities{Kind: "workload"}}}, nil
	}
	productKits = fstest.MapFS{
		"plain/plain.yaml":       {Data: []byte(plainKit), Mode: 0o444},
		"plain/files/run.sh":     {Data: []byte("#!/bin/sh\n"), Mode: 0o555},
		"withbin/withbin.yaml":   {Data: []byte(binKit), Mode: 0o444},
		"withcaps/withcaps.yaml": {Data: []byte(lifecycleKit), Mode: 0o444},
		"withnet/withnet.yaml":   {Data: []byte(networkKit), Mode: 0o444},
		"install/install.yaml":   {Data: []byte(installKit), Mode: 0o444},
	}
	t.Cleanup(func() { fetchWorkload, productKits = oldFetch, oldKits })
	return f
}

// again readies the fixture for one more step 6, as a later sync.
func (f *renderFixture) again() { f.r.candidates = nil }

// candidates returns the names under the candidates dir of shop.
func (f *renderFixture) candidates(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.in(render.CandidatesDir))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// outsideCandidates returns snapshot of the env dir without what is
// below .romeu/candidates/.
func (f *renderFixture) outsideCandidates(t *testing.T) map[string]string {
	t.Helper()
	all := snapshot(t, f.env)
	for p := range all {
		if strings.HasPrefix(p, f.in(render.CandidatesDir)+"/") {
			delete(all, p)
		}
	}
	return all
}

// TestRenderCandidatesRendersTheCandidate is step 6 of 04 4.2 on a
// first sync: one candidate dir under .romeu/candidates/, made 0700,
// holding sbxenv.yaml, the kits and a render.json whose digests are the
// gate's, and nothing written outside it but .romeu/candidates/ itself.
func TestRenderCandidatesRendersTheCandidate(t *testing.T) {
	f := renderRun(t)

	require.NoError(t, renderCandidates(t.Context(), f.r))

	require.Len(t, f.r.candidates, 1)
	c := f.r.candidates[0]
	assert.Equal(t, []string{c.dir.ID().String()}, f.candidates(t), "one candidate dir, named by its id")
	assert.Equal(t, fs.ModeDir|0o700, mode(t, c.dir.Path()))
	env, err := os.ReadFile(filepath.Join(c.dir.Path(), render.CandidateEnvFile))
	require.NoError(t, err)
	assert.Contains(t, string(env), "./memory/shop-api")
	data, err := os.ReadFile(filepath.Join(c.dir.Path(), render.CandidateRecord))
	require.NoError(t, err)
	rec, err := render.DecodeRecord(data)
	require.NoError(t, err)
	assert.Equal(t, c.record.WideningDigest, rec.WideningDigest)
	want, err := gate.Digest(c.set)
	require.NoError(t, err)
	assert.Equal(t, want, rec.WideningDigest, "the record holds the digest of the set")
	assert.Equal(t, plainSum(env), rec.Files[render.LiveEnvFile])
	require.Len(t, c.kits, 1)
	assert.Equal(t, c.kits[0].Digest.String(), rec.Files[c.kits[0].LivePath()])
	for p, d := range f.r.bin.Files() {
		assert.Equal(t, d, rec.Files[p], p)
	}
	assert.Equal(t, f.r.specCommit, rec.SpecSource.Commit)
	assert.Equal(t, "refs/romeu/origin/cfg/main", rec.SpecSource.Ref)
	assert.Equal(t, []string{"shop-api", "shop-web"}, []string{rec.Repos[0].Dir, rec.Repos[1].Dir})
	assert.Equal(t, f.r.trees[0].repos[0].egressCommit, rec.Repos[0].EgressCommit)
	assert.Equal(t, c.set, c.side.Set, "the candidate side of the diff holds the set")
	assert.Len(t, c.side.KitFiles["product:plain"], 2)
	assert.FileExists(t, filepath.Join(c.dir.Path(), render.CandidateKitsDir, c.kits[0].Dir(), "files", "run.sh"))
	assert.NoFileExists(t, f.in(render.LiveEnvFile), "step 6 writes no live file")
}

// TestRenderCandidatesReadsTheCachedDescriptor: a second sync with the
// same workload digest, a workload of one manifest, reads the
// descriptor from the cache record of host state and makes no registry
// read, so it passes with the registry unreachable (REQ-0571).
func TestRenderCandidatesReadsTheCachedDescriptor(t *testing.T) {
	f := renderRun(t)
	require.NoError(t, renderCandidates(t.Context(), f.r))
	require.Equal(t, []string{workloadRef}, f.reads, "the first sync reads the registry")
	f.registry = func(string) (oci.Fetched, error) {
		return oci.Fetched{}, oci.RegistryUnreachable.New("the registry docker.io did not answer")
	}
	f.again()

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.Equal(t, []string{workloadRef}, f.reads, "the second sync makes no registry read")
}

// TestRenderCandidatesStopsAtACacheEntryItCannotRead: an entry of the
// descriptor cache of a newer format is format-newer (RJ-311, exit 2),
// and the sync stops there rather than read the registry, so an offline
// sync is not reported as registry-unreachable for a cache it could not
// read (REQ-0165).
func TestRenderCandidatesStopsAtACacheEntryItCannotRead(t *testing.T) {
	f := renderRun(t)
	require.NoError(t, renderCandidates(t.Context(), f.r))
	dir := filepath.Join(f.r.store.Dir(), "descriptors")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	path := filepath.Join(dir, entries[0].Name())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	newer := strings.Replace(string(data), "state-descriptor/v1", "state-descriptor/v2", 1)
	require.NotEqual(t, string(data), newer)
	require.NoError(t, os.WriteFile(path, []byte(newer), 0o600))
	f.again()

	err = renderCandidates(t.Context(), f.r)

	e := cli.Of(err)
	require.NotNil(t, e, "%v", err)
	assert.Equal(t, "RJ-311", e.ID)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Equal(t, []string{workloadRef}, f.reads, "no registry read")
}

// TestRenderCandidatesReadsAChangedDigestOnce: a changed workload
// digest reads the registry once and writes its cache entry, which the
// next sync reads.
func TestRenderCandidatesReadsAChangedDigestOnce(t *testing.T) {
	f := renderRun(t)
	require.NoError(t, renderCandidates(t.Context(), f.r))
	f.r.targets[0].Workload = workloadOther
	f.again()

	require.NoError(t, renderCandidates(t.Context(), f.r))
	f.again()
	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.Equal(t, []string{workloadRef, workloadOther}, f.reads)
	w, err := spec.ParseWorkload(workloadOther)
	require.NoError(t, err)
	d, err := f.r.store.GetDescriptor(w.Digest)
	require.NoError(t, err)
	assert.Equal(t, w.Digest, d.Workload)
	assert.Equal(t, "linux/"+runtime.GOARCH, d.Platform)
}

// TestRenderCandidatesStopsAtTheRegistry: on a cache miss, a registry
// that cannot be read stops the sync with its row, and a descriptor the
// grammar refuses is a sync error (REQ-0573).
func TestRenderCandidatesStopsAtTheRegistry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		id   string
	}{
		{"unreachable", oci.RegistryUnreachable.New("the registry docker.io did not answer"), "RJ-322"},
		{"descriptor refused", fmt.Errorf("oci: line 3: \x1b[31m: %w", oci.ErrInvalid), cli.Usage.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := renderRun(t)
			f.registry = func(string) (oci.Fetched, error) { return oci.Fetched{}, tt.err }

			err := renderCandidates(t.Context(), f.r)

			e := cli.Of(err)
			require.NotNil(t, e, "%v", err)
			assert.Equal(t, tt.id, e.ID)
			assert.NotContains(t, err.Error(), "\x1b", "the text of the registry is escaped")
			assert.NoDirExists(t, f.in(render.CandidatesDir), "nothing is rendered")
		})
	}
}

// TestRenderCandidatesDeletesALeftoverCandidate: a candidate dir left
// as if by a killed command, which no record names, is deleted before
// the render, and one dir remains under .romeu/candidates/ (REQ-0082,
// REQ-0084); what is not named by a candidate id is left alone.
func TestRenderCandidatesDeletesALeftoverCandidate(t *testing.T) {
	f := renderRun(t)
	left := f.in(render.CandidatesDir + "/01j9zc2q8x7w4t6y3r5e1v9k0p")
	require.NoError(t, os.MkdirAll(filepath.Join(left, "kits"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(left, render.CandidateEnvFile), []byte("left"), 0o600))

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.NoDirExists(t, left)
	assert.Equal(t, []string{f.r.candidates[0].dir.ID().String()}, f.candidates(t))
}

// TestRenderCandidatesRefusesALinkedRomeuDir: a .romeu of the env dir
// that is a link is refused before any candidate is made through it
// (I12).
func TestRenderCandidatesRefusesALinkedRomeuDir(t *testing.T) {
	f := renderRun(t)
	elsewhere := t.TempDir()
	require.NoError(t, os.Symlink(elsewhere, f.in(romeuDir)))

	err := renderCandidates(t.Context(), f.r)

	assert.Equal(t, 2, cli.ExitCode(err), "%v", err)
	assert.Contains(t, err.Error(), "tree-unsafe")
	entries, err := os.ReadDir(elsewhere)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing is made through the link")
}

// TestRenderCandidatesRefusesAnOversizedManifest: a spec whose encoded
// manifest is over 32 KiB is manifest-size (RJ-313, exit 2), by the
// size function of internal/manifest, before gate 2 and before any
// sbxenv.yaml is rendered.
func TestRenderCandidatesRefusesAnOversizedManifest(t *testing.T) {
	f := renderRun(t)
	var extra []string
	for i := range 1500 {
		extra = append(extra, fmt.Sprintf("d%04d.example.com", i))
	}
	f.r.targets[0].Egress = &spec.Egress{Extra: extra}

	err := renderCandidates(t.Context(), f.r)

	e := cli.Of(err)
	require.NotNil(t, e, "%v", err)
	assert.Equal(t, ManifestSize.ID, e.ID)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, e.Hint(), "romeu sync shop")
	assert.Empty(t, f.candidates(t), "the candidate is declined, and its dir deleted, with no sbxenv.yaml rendered")
}

// TestRenderCandidatesPassesAManifestUnderTheCap: the same spec with a
// few extras fits, so the rule refuses by size alone.
func TestRenderCandidatesPassesAManifestUnderTheCap(t *testing.T) {
	f := renderRun(t)
	f.r.targets[0].Egress = &spec.Egress{Extra: []string{"d0001.example.com"}}

	require.NoError(t, renderCandidates(t.Context(), f.r))
}

// TestRenderCandidatesRefusesASpecThatSetsJulietaBin: julietaBin is the
// kit arg romeu renders (06 6.4, REQ-0596): a spec that sets it is a
// sync error naming the key, exit 2, on a kit that declares the arg and
// on one that does not, and no sbxenv.yaml is rendered.
func TestRenderCandidatesRefusesASpecThatSetsJulietaBin(t *testing.T) {
	for _, kit := range []string{"withbin", "plain"} {
		t.Run(kit, func(t *testing.T) {
			f := renderRun(t)
			f.r.targets[0].Kits = []spec.Kit{{Product: kit, Args: map[string]string{render.JulietaBinArg: "/x"}}}

			err := renderCandidates(t.Context(), f.r)

			assert.Equal(t, 2, cli.ExitCode(err), "%v", err)
			assert.Contains(t, err.Error(), "julieta-bin-reserved")
			assert.Contains(t, err.Error(), render.JulietaBinArg)
			assert.Empty(t, f.candidates(t), "the candidate is declined before any sbxenv.yaml is rendered")
		})
	}
}

// TestRenderCandidatesFlipsGate2OnACapability: a kit whose descriptor
// adds a capability changes the widening digest (I28).
func TestRenderCandidatesFlipsGate2OnACapability(t *testing.T) {
	f := renderRun(t)
	require.NoError(t, renderCandidates(t.Context(), f.r))
	before := f.r.candidates[0].widening
	f.r.targets[0].Kits = []spec.Kit{{Product: "withcaps"}}
	f.again()

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.NotEqual(t, before, f.r.candidates[0].widening)
}

// TestRenderCandidatesFlipsGate2OnARef: changing a repo's ref to
// another branch of its origin changes the egress commit and the
// widening digest, so the change meets gate 2 (I9).
func TestRenderCandidatesFlipsGate2OnARef(t *testing.T) {
	f := renderRun(t)
	require.NoError(t, renderCandidates(t.Context(), f.r))
	before, commit := f.r.candidates[0].widening, f.r.trees[0].repos[0].egressCommit
	origin := f.origins[apiURL]
	gitIn(t, origin, "checkout", "-q", "-b", "release")
	commitFiles(t, origin, map[string]string{"README": "release"})
	gitIn(t, origin, "checkout", "-q", "main")
	f.r.targets[0].Repos[0].Ref = "release"
	f.r.trees = nil
	require.NoError(t, makeTree(t.Context(), f.r))
	f.again()

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.NotEqual(t, commit, f.r.trees[0].repos[0].egressCommit)
	assert.NotEqual(t, before, f.r.candidates[0].widening)
}

// miseFiles are a mise.toml and its mise.lock, whose one tool is
// downloaded from host, so the derived egress gates host as a lock
// host (07 7.5).
func miseFiles(host string) map[string]string {
	return map[string]string{
		"mise.toml": "[tools]\ngo = \"1.27.3\"\n",
		"mise.lock": "lockfile_version = 3\n\n[[tools.\"go\"]]\nversion = \"1.27.3\"\nbackend = \"core:go\"\n\n" +
			"[tools.\"go\".\"platforms.linux-x64\"]\nchecksum = \"sha256:" + strings.Repeat("a", 64) + "\"\nurl = \"https://" + host + "/go.tar.gz\"\n",
	}
}

// TestRenderCandidatesKeepsTheEgressOfThePinnedCommit: the egress is
// read from the commit of refs/romeu/origin/<dir>/<ref> alone (I9). A
// lock change left uncommitted in the clone's work tree, a commit on
// another branch of the origin and a commit on another branch of the
// clone each leave the egress commit, the derived egress and the
// widening digest as they were; the same lock committed on the ref's
// branch of the origin changes all three, so the cases do not pass for
// want of a lock that reaches the digest.
func TestRenderCandidatesKeepsTheEgressOfThePinnedCommit(t *testing.T) {
	tests := []struct {
		name string
		// change makes the change in the clone or the origin of
		// shop-api.
		change func(t *testing.T, clone, origin string)
		moves  bool
	}{
		{"an uncommitted lock change", func(t *testing.T, clone, _ string) {
			t.Helper()
			for p, data := range miseFiles("changed.example.com") {
				require.NoError(t, os.WriteFile(filepath.Join(clone, p), []byte(data), 0o600))
			}
		}, false},
		{"a commit on another branch of the origin", func(t *testing.T, _, origin string) {
			t.Helper()
			gitIn(t, origin, "checkout", "-q", "-b", "release")
			commitFiles(t, origin, miseFiles("changed.example.com"))
			gitIn(t, origin, "checkout", "-q", "main")
		}, false},
		{"a commit on another branch of the clone", func(t *testing.T, clone, _ string) {
			t.Helper()
			gitIn(t, clone, "checkout", "-q", "-b", "mine")
			commitFiles(t, clone, miseFiles("changed.example.com"))
		}, false},
		{"control: a commit on the ref's branch of the origin", func(t *testing.T, _, origin string) {
			t.Helper()
			commitFiles(t, origin, miseFiles("changed.example.com"))
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := renderRunOf(t, func(h *hostTree) {
				commitFiles(t, h.origins[apiURL], miseFiles("pinned.example.com"))
			})
			require.NoError(t, renderCandidates(t.Context(), f.r))
			commit := f.r.trees[0].repos[0].egressCommit
			c := f.r.candidates[0]
			require.Contains(t, c.egress.Effective(), "pinned.example.com", "the pinned lock reaches the egress")
			tt.change(t, f.in("shop-api"), f.origins[apiURL])
			f.r.trees = nil
			require.NoError(t, makeTree(t.Context(), f.r))
			f.again()

			require.NoError(t, renderCandidates(t.Context(), f.r))

			got := f.r.candidates[0]
			if tt.moves {
				assert.NotEqual(t, commit, f.r.trees[0].repos[0].egressCommit)
				assert.Contains(t, got.egress.Effective(), "changed.example.com")
				assert.NotEqual(t, c.widening, got.widening)
				return
			}
			assert.Equal(t, commit, f.r.trees[0].repos[0].egressCommit)
			assert.Equal(t, c.egress, got.egress)
			assert.Equal(t, c.widening, got.widening)
		})
	}
}

// TestRenderCandidatesFlipsGate2OnAPersonalKitCapability: a personal
// kit read from the config commit's tree, and the same kit at a later
// commit whose descriptor adds a capability a personal kit may hold:
// the widening digest changes (I28).
func TestRenderCandidatesFlipsGate2OnAPersonalKitCapability(t *testing.T) {
	f := renderRun(t)
	f.r.specCommit = commitFiles(t, f.cfg, map[string]string{"kits/mine/mine.yaml": plainKit})
	f.r.targets[0].Kits = []spec.Kit{{Personal: "mine"}}
	require.NoError(t, renderCandidates(t.Context(), f.r))
	before := f.r.candidates[0].widening
	f.r.specCommit = commitFiles(t, f.cfg, map[string]string{"kits/mine/mine.yaml": lifecycleKit})
	f.again()

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.NotEqual(t, before, f.r.candidates[0].widening)
	assert.Contains(t, f.r.candidates[0].side.KitFiles, "personal:mine")
}

// treeEntry is one entry of a tree object rawTree writes.
type treeEntry struct{ mode, name, oid string }

// rawTree stores in the repository dir a tree object of the entries,
// sorted as git sorts them, with hash-object --literally, which checks
// nothing, so a tree no other git command would write can be built.
func rawTree(t *testing.T, dir string, entries ...treeEntry) string {
	t.Helper()
	key := func(e treeEntry) string {
		if e.mode == "40000" {
			return e.name + "/"
		}
		return e.name
	}
	slices.SortFunc(entries, func(a, b treeEntry) int { return strings.Compare(key(a), key(b)) })
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
	return gitIn(t, dir, "hash-object", "-w", "-t", "tree", "--literally", p)
}

// hostileCommit commits, in the config clone cfg, a tree that holds
// the personal kit mine alone: its descriptor and the entries extra
// makes from a blob of cfg.
func hostileCommit(t *testing.T, cfg string, extra func(blob string) []treeEntry) string {
	t.Helper()
	write := func(data string) string {
		p := filepath.Join(t.TempDir(), "blob")
		require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
		return gitIn(t, cfg, "hash-object", "-w", p)
	}
	blob := write("x\n")
	kit := rawTree(t, cfg, append(extra(blob), treeEntry{"100644", "mine.yaml", write(plainKit)})...)
	root := rawTree(t, cfg, treeEntry{"40000", "kits", rawTree(t, cfg, treeEntry{"40000", "mine", kit})})
	return gitIn(t, cfg, "commit-tree", root, "-m", "hostile")
}

// TestRenderCandidatesRefusesAHostilePersonalKit: a personal kit whose
// tree holds each hostile entry of I27, or a network capability
// (I28), fails the sync, exit 2 with kit-refused, before a file is
// written outside .romeu/candidates/, the product kit before it in the
// spec included. git fsck --strict refuses a .GIT name, a .git with a
// U+200C inserted after the dot, a ".." name and a stored mode 100664,
// and the walk the rest, which fsck passes. The case table of the walk
// itself is render's and gitsafe's (T024, T038); this one runs them
// through step 6. It runs in the race job on every runner of ci.yml,
// macos-26 among them.
func TestRenderCandidatesRefusesAHostilePersonalKit(t *testing.T) {
	entry := func(mode, name string) func(t *testing.T, cfg string) string {
		return func(t *testing.T, cfg string) string {
			t.Helper()
			return hostileCommit(t, cfg, func(blob string) []treeEntry { return []treeEntry{{mode, name, blob}} })
		}
	}
	tests := []struct {
		name string
		// commit returns the config commit that holds the kit mine.
		commit func(t *testing.T, cfg string) string
		want   string
	}{
		{"symbolic link", func(t *testing.T, cfg string) string {
			t.Helper()
			require.NoError(t, os.MkdirAll(filepath.Join(cfg, "kits", "mine"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(cfg, "kits", "mine", "mine.yaml"), []byte(plainKit), 0o600))
			require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(cfg, "kits", "mine", "link")))
			return commitFiles(t, cfg, nil)
		}, "symbolic link"},
		{"gitlink", func(t *testing.T, cfg string) string {
			t.Helper()
			return hostileCommit(t, cfg, func(blob string) []treeEntry {
				sub := gitIn(t, cfg, "commit-tree", rawTree(t, cfg, treeEntry{"100644", "a", blob}), "-m", "sub")
				return []treeEntry{{"160000", "module", sub}}
			})
		}, "gitlink"},
		{"case collision", func(t *testing.T, cfg string) string {
			t.Helper()
			return hostileCommit(t, cfg, func(blob string) []treeEntry {
				return []treeEntry{{"100644", "README", blob}, {"100644", "readme", blob}}
			})
		}, "are one path"},
		{"normalization collision", func(t *testing.T, cfg string) string {
			t.Helper()
			return hostileCommit(t, cfg, func(blob string) []treeEntry {
				return []treeEntry{{"100644", "caf\u00e9", blob}, {"100644", "cafe\u0301", blob}}
			})
		}, "are one path"},
		{".GIT", entry("100644", ".GIT"), "fsck"},
		{".git with an HFS-ignorable character", entry("100644", ".\u200cgit"), "fsck"},
		{"dot dot", entry("100644", ".."), "fsck"},
		{"mode 100664", entry("100664", "a"), "fsck"},
		{"network capability", func(t *testing.T, cfg string) string {
			t.Helper()
			return commitFiles(t, cfg, map[string]string{"kits/mine/mine.yaml": plainKit +
				"capabilities:\n  - type: com.docker.sandbox/network-policy@2\n    config:\n      runtime: {allow: [a.example]}\n"})
		}, "network"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := renderRun(t)
			f.r.specCommit = tt.commit(t, f.cfg)
			f.r.targets[0].Kits = []spec.Kit{{Product: "plain"}, {Personal: "mine"}}
			before := f.outsideCandidates(t)

			err := renderCandidates(t.Context(), f.r)

			assert.Equal(t, 2, cli.ExitCode(err), "%v", err)
			assert.Contains(t, err.Error(), "kit-refused")
			assert.Contains(t, err.Error(), tt.want)
			after := f.outsideCandidates(t)
			delete(after, f.in(romeuDir))
			delete(after, f.in(render.CandidatesDir))
			assert.Equal(t, before, after, "nothing is written outside .romeu/candidates/")
			assert.Empty(t, f.candidates(t), "the candidate is declined")
		})
	}
}

// TestRenderCandidatesGatesAKitDeclaredDomain: a domain the workload's
// descriptor declares, which the catalog does not allow, is gated as
// kit-declared in the widening set and in render.json (I10).
func TestRenderCandidatesGatesAKitDeclaredDomain(t *testing.T) {
	f := renderRun(t)
	f.registry = func(ref string) (oci.Fetched, error) {
		w, err := spec.ParseWorkload(ref)
		if err != nil {
			return oci.Fetched{}, err
		}
		caps := oci.Capabilities{Kind: "workload", Network: &oci.Network{Runtime: oci.Rules{Allow: []oci.NetEntry{{Hosts: []string{"kit.example.com"}}}}}}
		return oci.Fetched{Workload: w, ManifestDigest: w.Digest, Kit: oci.Kit{Capabilities: caps}}, nil
	}

	require.NoError(t, renderCandidates(t.Context(), f.r))

	c := f.r.candidates[0]
	assert.Contains(t, c.set.EgressGated, gate.GatedDomain{Domain: "kit.example.com", Reason: gate.ReasonKitDeclared})
	assert.Contains(t, c.record.Egress.Gated, render.GatedDomain{Domain: "kit.example.com", Reason: "kit"})
}

// TestRenderCandidatesGatesADomainAProductKitDeclares: a domain a
// product kit's descriptor declares for the runtime, with a port, which
// the catalog does not allow, is gated as kit-declared in the widening
// set, its port stripped by egress.Split (I10, REQ-0069).
func TestRenderCandidatesGatesADomainAProductKitDeclares(t *testing.T) {
	f := renderRun(t)
	f.r.targets[0].Kits = []spec.Kit{{Product: "withnet"}}

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.Contains(t, f.r.candidates[0].set.EgressGated, gate.GatedDomain{Domain: "kit.example.com", Reason: gate.ReasonKitDeclared})
}

// TestRenderCandidatesLeavesAnInstallHostOutOfTheEgress: a host a kit
// allows at install alone is not in the derived egress, neither allowed
// nor gated, and gate 2 shows it with the kit's capability set
// (REQ-1315, 07 7.5).
func TestRenderCandidatesLeavesAnInstallHostOutOfTheEgress(t *testing.T) {
	f := renderRun(t)
	f.r.targets[0].Kits = []spec.Kit{{Product: "install"}}

	require.NoError(t, renderCandidates(t.Context(), f.r))

	c := f.r.candidates[0]
	assert.NotContains(t, c.egress.Effective(), "install.example.com", "the install host is not in the derived set")
	assert.NotContains(t, c.record.Egress.Auto, "install.example.com")
	for _, g := range c.set.EgressGated {
		assert.NotEqual(t, "install.example.com", g.Domain)
	}
	require.Len(t, c.kits, 1)
	require.NotNil(t, c.kits[0].Capabilities.Network)
	assert.Equal(t, []string{"install.example.com:443"}, c.kits[0].Capabilities.Network.Install.Allow[0].Hosts, "the capability set gate 2 shows holds it")
}

// TestRenderCandidatesRefusesAHostThatIsNoDomain: a declared host that
// egress.Split refuses, in either phase, stops the sync as usage
// (descriptor-invalid, exit 2) naming the kit and the host, and the
// candidate is declined.
func TestRenderCandidatesRefusesAHostThatIsNoDomain(t *testing.T) {
	for _, phase := range []string{"install", "runtime"} {
		t.Run(phase, func(t *testing.T) {
			f := renderRun(t)
			f.registry = func(ref string) (oci.Fetched, error) {
				w, err := spec.ParseWorkload(ref)
				if err != nil {
					return oci.Fetched{}, err
				}
				rules := oci.Rules{Allow: []oci.NetEntry{{Hosts: []string{"kit.example.com:0"}}}}
				network := &oci.Network{}
				if phase == "install" {
					network.Install = rules
				} else {
					network.Runtime = rules
				}
				return oci.Fetched{Workload: w, ManifestDigest: w.Digest, Kit: oci.Kit{Capabilities: oci.Capabilities{Kind: "workload", Network: network}}}, nil
			}

			err := renderCandidates(t.Context(), f.r)

			assert.Equal(t, 2, cli.ExitCode(err), "%v", err)
			assert.Contains(t, err.Error(), "descriptor-invalid")
			assert.Contains(t, err.Error(), "kit.example.com:0")
			assert.Contains(t, err.Error(), workloadRef)
			assert.Empty(t, f.candidates(t), "the candidate is declined")
		})
	}
}

// TestRenderCandidatesRendersEveryProductKit: every product kit of the
// embedded copy (06 6.4) renders through step 6, its declared network
// hosts split by egress.Split before the derivation (REQ-0069).
func TestRenderCandidatesRendersEveryProductKit(t *testing.T) {
	f := renderRun(t)
	productKits = kits.FS
	entries, err := fs.ReadDir(kits.FS, ".")
	require.NoError(t, err)
	var all []spec.Kit
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		k := spec.Kit{Product: e.Name()}
		if e.Name() == "git-ssh-sign" {
			k.Args = map[string]string{"signingKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHk1a3Rlc3RrZXlmb3J0aGVmaXh0dXJlb25seTAwMDA="}
		}
		all = append(all, k)
	}
	require.NotEmpty(t, all)
	f.r.targets[0].Kits = all

	require.NoError(t, renderCandidates(t.Context(), f.r))

	require.Len(t, f.r.candidates[0].kits, len(all))
}

// TestReadKitsRefusesAKitChangedAfterItsDigest: a kit file that changes
// between the materialization and the read-back is not the kit the
// digest names, and the gate never sees it: the read fails closed.
func TestReadKitsRefusesAKitChangedAfterItsDigest(t *testing.T) {
	f := renderRun(t)
	dir := t.TempDir()
	m := render.KitMaterializer{FS: productKits, EnvDir: f.env, GOARCH: runtime.GOARCH}
	kits, err := m.Materialize(t.Context(), render.Candidate{Dir: dir}, spec.Project{Kits: []spec.Kit{{Product: "plain"}}})
	require.NoError(t, err)
	_, err = readKits(dir, kits)
	require.NoError(t, err, "the kit as written reads back")
	require.NoError(t, os.WriteFile(filepath.Join(dir, render.CandidateKitsDir, kits[0].Dir(), "plain.yaml"), []byte(lifecycleKit), 0o600))

	_, err = readKits(dir, kits)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not the kit materialized")
}

// TestRenderCandidatesExitsOneOnAnUnknownKitAndDeclinesIt: a spec kit
// that is in neither the product kits nor the config commit (ghost)
// stops the sync and leaves no candidate dir: the candidate is
// declined. It exits 1, an error of no row: render exports no sentinel
// that tells an unknown kit apart from a failure on romeu's own paths,
// so it is not kit-refused (exit 2) until it does.
func TestRenderCandidatesExitsOneOnAnUnknownKitAndDeclinesIt(t *testing.T) {
	for _, kit := range []spec.Kit{{Personal: "ghost"}, {Product: "ghost"}} {
		t.Run(kit.Product+kit.Personal+" "+map[bool]string{true: "personal", false: "product"}[kit.Personal != ""], func(t *testing.T) {
			f := renderRun(t)
			f.r.targets[0].Kits = []spec.Kit{kit}

			err := renderCandidates(t.Context(), f.r)

			require.Error(t, err)
			assert.Equal(t, 1, cli.ExitCode(err), "%v", err)
			assert.Contains(t, err.Error(), "ghost")
			assert.Empty(t, f.candidates(t), "the candidate is declined")
		})
	}
}

// TestRenderCandidatesDeclinesEveryCandidateOnAnError: with two
// targets, an unknown kit of the second declines the candidate of the
// first too, so no candidate dir remains under either env dir.
func TestRenderCandidatesDeclinesEveryCandidateOnAnError(t *testing.T) {
	f := renderRun(t)
	f.addTarget(t, "cart", "cart-api")
	cart := &f.r.targets[1]
	cart.Workload, cart.Agent, cart.Kits = workloadRef, "sbx-kit-claude", []spec.Kit{{Personal: "ghost"}}
	f.r.trees = nil
	require.NoError(t, makeTree(t.Context(), f.r))

	err := renderCandidates(t.Context(), f.r)

	require.Error(t, err)
	require.Len(t, f.r.candidates, 1, "the first target was resolved")
	assert.NoDirExists(t, f.r.candidates[0].dir.Path())
	assert.Empty(t, f.candidates(t))
	entries, err := os.ReadDir(filepath.Join(f.root, "cart-env", filepath.FromSlash(render.CandidatesDir)))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestRenderCandidatesReadsAnIndexWorkloadFromTheCache: the entry of a
// multi-arch workload is written under the digest of the platform
// manifest (REQ-0240), and the next sync finds it by the reference's
// index digest and the platform (Store.FindDescriptor), so it reads no
// registry, with the registry unreachable.
func TestRenderCandidatesReadsAnIndexWorkloadFromTheCache(t *testing.T) {
	f := renderRun(t)
	const platformManifest = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	f.registry = func(ref string) (oci.Fetched, error) {
		w, err := spec.ParseWorkload(ref)
		if err != nil {
			return oci.Fetched{}, err
		}
		return oci.Fetched{Workload: w, ManifestDigest: platformManifest, Kit: oci.Kit{Capabilities: oci.Capabilities{Kind: "workload"}}}, nil
	}
	require.NoError(t, renderCandidates(t.Context(), f.r))
	f.registry = func(string) (oci.Fetched, error) {
		return oci.Fetched{}, oci.RegistryUnreachable.New("the registry docker.io did not answer")
	}
	f.again()

	require.NoError(t, renderCandidates(t.Context(), f.r))

	assert.Equal(t, []string{workloadRef}, f.reads, "the second sync reads no registry")
	d, err := f.r.store.GetDescriptor(platformManifest)
	require.NoError(t, err, "the entry is under the platform manifest's digest")
	w, err := spec.ParseWorkload(workloadRef)
	require.NoError(t, err)
	assert.Equal(t, w.Digest, d.Workload)
	_, err = f.r.store.GetDescriptor(w.Digest)
	require.ErrorIs(t, err, fs.ErrNotExist, "nothing is under the index digest")
}
