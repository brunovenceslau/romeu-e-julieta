// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package render_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// fixtureGit runs git in dir, without gitsafe, to build a config repo,
// and returns its standard output, trimmed. stdin feeds the command.
func fixtureGit(t *testing.T, dir string, stdin []byte, args ...string) string {
	t.Helper()
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	err = procgroup.Run(t.Context(), procgroup.Command{
		Class: procgroup.JulietaGit,
		Argv:  append([]string{git, "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...),
		Dir:   dir,
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + t.TempDir(), "LANG=C",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
		},
		Stdin:  bytes.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// configRepo makes a config repo whose one commit holds the files, a
// name ending in ".sh" executable, and returns its dir and the commit.
func configRepo(t *testing.T, files map[string]string) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	fixtureGit(t, dir, nil, "init", "-q")
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
		mode := os.FileMode(0o600)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o700
		}
		require.NoError(t, os.WriteFile(p, []byte(content), mode))
	}
	fixtureGit(t, dir, nil, "add", "-A")
	fixtureGit(t, dir, nil, "commit", "-q", "-m", "config")
	return dir, fixtureGit(t, dir, nil, "rev-parse", "HEAD")
}

// treeEntry is one entry of a tree object as git stores it.
type treeEntry struct{ mode, name, oid string }

// rawTree stores a tree object of the entries with hash-object
// --literally, which checks nothing, so a tree no git command would
// write can be built. The entries are sorted as git sorts them, a tree
// by its name and a "/", so only the hostile entry is wrong.
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
	return fixtureGit(t, dir, b.Bytes(), "hash-object", "-w", "-t", "tree", "--literally", "--stdin")
}

// hostileRepo makes a config repo of its own, since fsck checks every
// object of a repository, whose commit holds the personal kit mine: its
// descriptor and the entries of extra.
func hostileRepo(t *testing.T, extra func(dir, blob string) []treeEntry) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	fixtureGit(t, dir, nil, "init", "-q")
	desc := fixtureGit(t, dir, []byte(plainDescriptor), "hash-object", "-w", "--stdin")
	blob := fixtureGit(t, dir, []byte("x\n"), "hash-object", "-w", "--stdin")
	kit := rawTree(t, dir, append(extra(dir, blob), treeEntry{"100644", "mine.yaml", desc})...)
	kitsTree := rawTree(t, dir, treeEntry{"40000", "mine", kit})
	root := rawTree(t, dir, treeEntry{"40000", "kits", kitsTree})
	return dir, fixtureGit(t, dir, nil, "commit-tree", root, "-m", "hostile")
}

// hostRunner returns a gitsafe runner of the host.
func hostRunner(t *testing.T) *gitsafe.Runner {
	t.Helper()
	git, err := procgroup.LookPath("git")
	require.NoError(t, err)
	r, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: git, Home: t.TempDir()})
	require.NoError(t, err)
	return r
}

// personalSource is the materializer over the fixture product kits and
// the config repo dir at commit.
func personalSource(t *testing.T, dir, commit string) render.KitMaterializer {
	t.Helper()
	m := productSource(t, fixtureKits(), "/r/shop-env")
	m.Git = hostRunner(t)
	m.ConfigDir = dir
	m.ConfigCommit = commit
	return m
}

// candidateIn returns a candidate dir alone in a parent dir, so a test
// can hold that nothing was written beside it.
func candidateIn(t *testing.T) (parent string, c render.Candidate) {
	t.Helper()
	parent = t.TempDir()
	c = render.Candidate{Dir: filepath.Join(parent, "cand")}
	require.NoError(t, os.Mkdir(c.Dir, 0o700))
	return parent, c
}

// assertOnlyCandidate holds that parent holds the candidate dir alone.
func assertOnlyCandidate(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "cand", entries[0].Name())
}

const lifecycleDescriptor = plainDescriptor + `capabilities:
  - type: com.docker.sandbox/lifecycle@1
    config:
      install: [{command: [sh, -c, "true"], user: agent}]
  - type: com.docker.sandbox/privileged@1
`

// TestPersonalKitsMaterializeFromTheConfigCommit: a personal kit is read
// from the commit's kits/<kit>/, never from the work tree, digested and
// written into the candidate beside a product kit, in the spec's order;
// another kit of the config repo is not written, and nothing is written
// outside the candidate.
func TestPersonalKitsMaterializeFromTheConfigCommit(t *testing.T) {
	t.Parallel()
	dir, commit := configRepo(t, map[string]string{
		"projects/shop.yaml":    "name: shop\n",
		"kits/mine/mine.yaml":   binDescriptor,
		"kits/mine/files/a.txt": "a",
		"kits/mine/bin/run.sh":  "#!/bin/sh\n",
		"kits/other/other.yaml": plainDescriptor,
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kits", "mine", "files", "a.txt"), []byte("work tree"), 0o600))
	parent, cand := candidateIn(t)
	p := spec.Project{Kits: []spec.Kit{{Product: "plain"}, {Personal: "mine"}}}
	got, err := personalSource(t, dir, commit).Materialize(t.Context(), cand, p)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "plain", got[0].ID)
	mine := got[1]
	assert.Equal(t, "mine", mine.ID)

	want, err := gate.KitTreeDigest([]gate.File{
		{Path: "mine.yaml", Mode: gate.ModeRegular, Data: []byte(binDescriptor)},
		{Path: "files/a.txt", Mode: gate.ModeRegular, Data: []byte("a")},
		{Path: "bin/run.sh", Mode: gate.ModeExecutable, Data: []byte("#!/bin/sh\n")},
	})
	require.NoError(t, err)
	assert.Equal(t, want, mine.Digest)
	assert.Equal(t, map[string]string{"julietaBin": "/r/shop-env/.romeu/bin/julieta-linux-arm64"}, mine.Rendered)
	k, err := oci.ParseDescriptor([]byte(binDescriptor))
	require.NoError(t, err)
	assert.Equal(t, k.Capabilities, mine.Capabilities)

	kitDir := filepath.Join(cand.Dir, render.CandidateKitsDir, mine.Dir())
	b, err := os.ReadFile(filepath.Join(kitDir, "files", "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "a", string(b), "the commit's bytes, not the work tree's")
	fi, err := os.Stat(filepath.Join(kitDir, "bin", "run.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
	entries, err := os.ReadDir(filepath.Join(cand.Dir, render.CandidateKitsDir))
	require.NoError(t, err)
	assert.Len(t, entries, 2, "the kit other is not written")
	assertOnlyCandidate(t, parent)
}

// TestPersonalKitsRefuseTheFiveCapabilities: a personal kit that
// declares a network, mount, volume, credential or ssh-agent capability
// is a sync error naming the kit and the type, and nothing is written.
func TestPersonalKitsRefuseTheFiveCapabilities(t *testing.T) {
	t.Parallel()
	caps := map[string]string{
		"network":    "  - type: com.docker.sandbox/network-policy@2\n    config:\n      runtime: {allow: [a.example]}\n",
		"mount":      "  - type: com.docker.sandbox/host-mount@1\n    config: {path: /m}\n",
		"volume":     "  - type: com.docker.sandbox/volume@1\n    config: {path: /data}\n",
		"credential": "  - type: com.docker.sandbox/credential@1\n    config: {service: svc, phase: runtime, apiKey: {name: KEY}}\n",
		"ssh-agent":  "  - type: com.docker.sandbox/ssh-agent@1\n    config: {phase: runtime, unrestricted: false, sign: [git]}\n",
	}
	files := map[string]string{}
	for typ, c := range caps {
		files["kits/"+typ+"/"+typ+".yaml"] = plainDescriptor + "capabilities:\n" + c
	}
	dir, commit := configRepo(t, files)
	m := personalSource(t, dir, commit)
	for typ := range caps {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			parent, cand := candidateIn(t)
			p := spec.Project{Kits: []spec.Kit{{Product: "plain"}, {Personal: typ}}}
			_, err := m.Materialize(t.Context(), cand, p)
			require.ErrorIs(t, err, render.ErrPersonalCapability)
			assert.Contains(t, err.Error(), `"`+typ+`"`)
			assert.Contains(t, err.Error(), "a "+typ+" capability")
			assertNothingWritten(t, cand)
			assertOnlyCandidate(t, parent)
		})
	}
}

// TestPersonalKitsKeepTheOtherCapabilities: an install step and a
// privilege are not refused; they stay in the capability set the gate
// shows.
func TestPersonalKitsKeepTheOtherCapabilities(t *testing.T) {
	t.Parallel()
	dir, commit := configRepo(t, map[string]string{"kits/life/life.yaml": lifecycleDescriptor})
	got, err := personalSource(t, dir, commit).
		Materialize(t.Context(), render.Candidate{Dir: t.TempDir()}, spec.Project{Kits: []spec.Kit{{Personal: "life"}}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	k, err := oci.ParseDescriptor([]byte(lifecycleDescriptor))
	require.NoError(t, err)
	assert.Equal(t, k.Capabilities, got[0].Capabilities)
	require.NotNil(t, got[0].Capabilities.Lifecycle)
	assert.NotEmpty(t, got[0].Capabilities.Flags)
}

// TestPersonalKitsRefuseHostileTrees: a config commit with a hostile
// entry of I27 in the personal kit fails before any file is written,
// the product kit before it in the spec included. fsck refuses some
// (a .GIT name, a stored mode 100664, a ".." name) and the walk the rest
// (a symlink, a gitlink, a case collision, a Unicode normalization
// collision), which fsck passes; the walk's table is T024's.
//
//romeu:invariant I27 test
func TestPersonalKitsRefuseHostileTrees(t *testing.T) {
	t.Parallel()
	file := func(name string) func(string, string) []treeEntry {
		return func(_, blob string) []treeEntry { return []treeEntry{{"100644", name, blob}} }
	}
	tests := []struct {
		name  string
		extra func(dir, blob string) []treeEntry
		walk  bool // refused by WalkTree with gitsafe.ErrRefused
		want  string
	}{
		{"symlink", func(_, blob string) []treeEntry { return []treeEntry{{"120000", "link", blob}} }, true, "is a symbolic link"},
		{"gitlink", func(dir, blob string) []treeEntry {
			c := fixtureGit(t, dir, nil, "commit-tree", rawTree(t, dir, treeEntry{"100644", "a", blob}), "-m", "sub")
			return []treeEntry{{"160000", "module", c}}
		}, true, "is a gitlink"},
		{"case collision", func(_, blob string) []treeEntry {
			return []treeEntry{{"100644", "README", blob}, {"100644", "readme", blob}}
		}, true, "are one path"},
		{"normalization collision", func(_, blob string) []treeEntry {
			return []treeEntry{{"100644", "café", blob}, {"100644", "café", blob}}
		}, true, "are one path"},
		{".GIT", file(".GIT"), false, "fsck"},
		{"dot dot", file(".."), false, "fsck"},
		{"mode 100664", func(_, blob string) []treeEntry { return []treeEntry{{"100664", "a", blob}} }, false, "fsck"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, commit := hostileRepo(t, tt.extra)
			parent, cand := candidateIn(t)
			p := spec.Project{Kits: []spec.Kit{{Product: "plain"}, {Personal: "mine"}}}
			got, err := personalSource(t, dir, commit).Materialize(t.Context(), cand, p)
			require.Error(t, err)
			assert.Nil(t, got)
			if tt.walk {
				require.ErrorIs(t, err, gitsafe.ErrRefused)
			}
			assert.Contains(t, err.Error(), tt.want)
			assertNothingWritten(t, cand)
			assertOnlyCandidate(t, parent)
		})
	}
}

// TestPersonalKitsRefuseWhatTheyCannotRead: a kit the commit does not
// hold, a kit without its descriptor, a name that is not one element,
// and a personal kit with no config repo set are each an error naming
// the cause, and nothing is written.
func TestPersonalKitsRefuseWhatTheyCannotRead(t *testing.T) {
	t.Parallel()
	dir, commit := configRepo(t, map[string]string{
		"kits/nodesc/a.txt":   "a",
		"kits/mine/mine.yaml": plainDescriptor,
	})
	for name, tc := range map[string]struct {
		m    render.KitMaterializer
		kit  string
		want string
	}{
		"absent kit":     {personalSource(t, dir, commit), "nope", `the personal kit "nope" is not in the config commit`},
		"no descriptor":  {personalSource(t, dir, commit), "nodesc", "nodesc.yaml"},
		"not one name":   {personalSource(t, dir, commit), "../mine", "../mine"},
		"no config repo": {productSource(t, fixtureKits(), "/r/shop-env"), "mine", "no config commit"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cand := render.Candidate{Dir: t.TempDir()}
			_, err := tc.m.Materialize(t.Context(), cand, spec.Project{Kits: []spec.Kit{{Product: "plain"}, {Personal: tc.kit}}})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assertNothingWritten(t, cand)
		})
	}
}
