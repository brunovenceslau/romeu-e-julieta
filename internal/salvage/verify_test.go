// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// project is the project of the host half's tests.
const project = "shop"

// hostRunner returns a gitsafe runner in host mode, romeu's, with a
// HOME of its own and the git hosts given.
func hostRunner(t *testing.T, hosts ...gitsafe.GitHost) *gitsafe.Runner {
	t.Helper()
	r, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Host, Git: gitPath(t), Home: t.TempDir(), GitHosts: hosts})
	require.NoError(t, err)
	return r
}

// salvaged runs the sandbox half for s, with edit applied to its
// config, and returns the host clone to import into, a new empty
// repository.
func salvaged(t *testing.T, s sandbox, edit func(*salvage.Config)) string {
	t.Helper()
	c := s.config(t)
	if edit != nil {
		edit(&c)
	}
	_, err := salvage.Salvage(t.Context(), c)
	require.NoError(t, err)
	return repo(t)
}

// verifyConfig returns the config of the host half of the salvage of s
// into clone, cap 1 MiB.
func (s sandbox) verifyConfig(t *testing.T, clone string) salvage.VerifyConfig {
	t.Helper()
	return salvage.VerifyConfig{
		Git: hostRunner(t), Name: project, Generation: salvageGen, SalvageID: salvageID,
		Repos:    []salvage.VerifyRepo{{Dir: "api", Memory: s.memory, Clone: clone}},
		CapBytes: 1 << 20, Work: workDir(t),
	}
}

// editManifest rewrites the manifest of the salvage dir dir with edit
// applied, as EncodeManifest writes it.
func editManifest(t *testing.T, dir string, edit func(*salvage.Manifest)) {
	t.Helper()
	m := readManifest(t, dir)
	edit(&m)
	data, err := salvage.EncodeManifest(m)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, salvage.ManifestFile), data, 0o600))
}

// items returns the items as "reason item".
func items(v salvage.Verification) []string {
	var out []string
	for _, i := range v.Skipped {
		out = append(out, i.Reason+" "+i.Item)
	}
	return out
}

// TestVerify verifies and imports a whole salvage: every ref of the
// bundle becomes a create-only ref below the project's salvage prefix,
// each pointing at the bundle's object, the manifest's sha256 is that
// of its bytes, the ignored files make it secret-bearing, and a forged
// complete changes nothing, since completeness is recomputed from the
// items (REQ-0717).
func TestVerify(t *testing.T) {
	s := newSandbox(t)
	git(t, s.repo, "tag", "v1")
	put(t, s.repo, "stashed", "stash me\n")
	git(t, s.repo, "add", "stashed")
	git(t, s.repo, "stash", "-q")
	stash := git(t, s.repo, "rev-parse", "stash@{0}")
	put(t, s.repo, "kept", "two\n")
	put(t, s.repo, "a.log", "log\n")
	clone := salvaged(t, s, nil)

	v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
	require.NoError(t, err)
	prefix := "refs/romeu/salvage/shop/" + salvageGen + "/" + salvageID + "/"
	assert.Equal(t, prefix, v.RefsPrefix)
	assert.Empty(t, v.Skipped)
	assert.True(t, salvage.Complete(v.Skipped))
	assert.True(t, v.SecretBearing, "ignored.tar.gz was verified")
	require.Len(t, v.Repos, 1)
	data, err := os.ReadFile(filepath.Join(s.dir(), salvage.ManifestFile))
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	assert.Equal(t, hex.EncodeToString(sum[:]), v.Repos[0].ManifestSHA256)
	assert.Equal(t, salvageID, v.Repos[0].Manifest.SalvageID)

	sandboxPrefix := "salvage/" + salvageGen + "/" + salvageID + "/"
	want := map[string]string{
		"heads/main":                 s.head,
		"tags/v1":                    s.head,
		sandboxPrefix + "stash-0":    stash,
		sandboxPrefix + "worktree-0": git(t, s.repo, "rev-parse", salvage.RefPrefix(salvageGen, salvageID)+"worktree-0"),
	}
	got := map[string]string{}
	for _, h := range v.Repos[0].Imported {
		got[strings.TrimPrefix(h.Ref, prefix+"api/")] = h.OID
		assert.Equal(t, h.OID, git(t, clone, "rev-parse", h.Ref), "the ref %s is in the clone", h.Ref)
	}
	assert.Equal(t, want, got)
	assert.Equal(t, "two", git(t, clone, "show", prefix+"api/"+sandboxPrefix+"worktree-0:kept"), "the capture's objects are in the clone")
	assert.Empty(t, git(t, clone, "for-each-ref", "refs/heads", "refs/tags", "refs/remotes"), "nothing outside the salvage namespace (I5)")

	t.Run("a second import", func(t *testing.T) {
		// The refs are create-only (I6): the same salvage imported again
		// fails on the first ref, creates none and moves none.
		before := git(t, clone, "for-each-ref")
		again, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
		require.Error(t, err)
		assert.Equal(t, before, git(t, clone, "for-each-ref"))
		require.Len(t, again.Repos, 1)
		assert.Empty(t, again.Repos[0].Imported, "what Verify did is returned with the error")
	})

	t.Run("a forged complete", func(t *testing.T) {
		editManifest(t, s.dir(), func(m *salvage.Manifest) {
			m.Skipped = append(m.Skipped, salvage.Skipped{Item: "worktree-0", Reason: salvage.ReasonWorktreeChanging})
		})
		data, err := os.ReadFile(filepath.Join(s.dir(), salvage.ManifestFile))
		require.NoError(t, err)
		forged := bytes.Replace(data, []byte(`"complete":false`), []byte(`"complete":true`), 1)
		require.NotEqual(t, data, forged)
		require.NoError(t, os.WriteFile(filepath.Join(s.dir(), salvage.ManifestFile), forged, 0o600))
		v, err := salvage.Verify(t.Context(), s.verifyConfig(t, repo(t)))
		require.NoError(t, err)
		assert.Equal(t, []string{"worktree-changing worktree-0"}, items(v))
		assert.False(t, salvage.Complete(v.Skipped))
	})
}

// TestVerifyRepoMissing makes repo-missing a repo of the recorded set
// with no salvage dir, a salvage dir without its manifest, which an
// interrupted salvage leaves (REQ-0708), a manifest that does not
// decode, and one that names another generation or repo; the other
// repos are imported all the same.
func TestVerifyRepoMissing(t *testing.T) {
	s := newSandbox(t)
	salvaged(t, s, nil)
	other := func(t *testing.T) string {
		t.Helper()
		dir := filepath.Join(t.TempDir(), "web")
		require.NoError(t, os.Mkdir(dir, 0o700))
		return dir
	}
	tests := []struct {
		name  string
		setup func(t *testing.T, memory string)
	}{
		{"no salvage dir", func(*testing.T, string) {}},
		{"no manifest", func(t *testing.T, memory string) {
			t.Helper()
			require.NoError(t, os.MkdirAll(filepath.Join(memory, "salvage", salvageGen, salvageID), 0o700))
		}},
		{"a manifest that does not decode", func(t *testing.T, memory string) {
			t.Helper()
			put(t, memory, "salvage/"+salvageGen+"/"+salvageID+"/manifest.json", `{"schema":"salvage/v1"}`)
		}},
		{"another repo", func(t *testing.T, memory string) {
			t.Helper()
			m := validManifest()
			m.Repo, m.Bundles, m.Files, m.Refs = "api", nil, nil, nil
			put(t, memory, "salvage/"+salvageGen+"/"+salvageID+"/manifest.json", string(encode(t, m)))
		}},
		{"another generation", func(t *testing.T, memory string) {
			t.Helper()
			m := validManifest()
			m.Repo, m.Generation, m.Bundles, m.Files, m.Refs = "web", "01j9zc2q8x7w4t6y3r5e1v9k0q", nil, nil, nil
			put(t, memory, "salvage/"+salvageGen+"/"+salvageID+"/manifest.json", string(encode(t, m)))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := other(t)
			tt.setup(t, memory)
			c := s.verifyConfig(t, repo(t))
			c.Repos = append(c.Repos, salvage.VerifyRepo{Dir: "web", Memory: memory, Clone: repo(t)})
			v, err := salvage.Verify(t.Context(), c)
			require.NoError(t, err)
			require.Len(t, v.Skipped, 1)
			assert.Equal(t, salvage.Item{Repo: "web", Item: v.Skipped[0].Item, Reason: salvage.ReasonRepoMissing, Detail: v.Skipped[0].Detail}, v.Skipped[0])
			assert.Equal(t, []string{salvage.ReasonRepoMissing}, salvage.Reasons(v.Skipped))
			require.Len(t, v.Repos, 2)
			assert.NotEmpty(t, v.Repos[0].Imported, "the repo api is imported")
			assert.Empty(t, v.Repos[1].ManifestSHA256)
		})
	}
}

// TestVerifySalvageIDMismatch fails with salvage-id-mismatch (RJ-339),
// not a skipped item, when the manifest names another salvage, and
// imports nothing.
func TestVerifySalvageIDMismatch(t *testing.T) {
	s := newSandbox(t)
	clone := salvaged(t, s, nil)
	editManifest(t, s.dir(), func(m *salvage.Manifest) { m.SalvageID = "01jb000000000000000000000b" })
	_, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
	assertRow(t, salvage.SalvageIDMismatch, err)
	assert.Empty(t, git(t, clone, "for-each-ref"))
}

// TestVerifyHostilePaths fails with salvage-path (RJ-307) on each
// hostile path of a payload: absolute, with "..", through a symbolic
// link that stays inside the salvage dir or leaves it, a link in place
// of the file, a FIFO, a directory, and a salvage dir that is itself a
// link; nothing is imported.
func TestVerifyHostilePaths(t *testing.T) {
	outside := t.TempDir()
	put(t, outside, "repo.bundle", "outside\n")
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		file  string
	}{
		{"absolute", nil, "/etc/passwd"},
		{"a dot-dot element", nil, "../repo.bundle"},
		{"a link inside the salvage dir", func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.Symlink(".", filepath.Join(dir, "in")))
		}, "in/repo.bundle"},
		{"a link outside the salvage dir", func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.Symlink(outside, filepath.Join(dir, "out")))
		}, "out/repo.bundle"},
		{"a link in place of the file", func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.Symlink("repo.bundle", filepath.Join(dir, "alias.bundle")))
		}, "alias.bundle"},
		{"a FIFO", func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "fifo.bundle"), 0o600))
		}, "fifo.bundle"},
		{"a directory", func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.Mkdir(filepath.Join(dir, "d.bundle"), 0o700))
		}, "d.bundle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSandbox(t)
			clone := salvaged(t, s, nil)
			if tt.setup != nil {
				tt.setup(t, s.dir())
			}
			editManifest(t, s.dir(), func(m *salvage.Manifest) { m.Bundles[0].File = tt.file })
			_, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
			assertRow(t, salvage.SalvagePath, err)
			assert.Empty(t, git(t, clone, "for-each-ref"))
		})
	}
	t.Run("a salvage dir that is a link", func(t *testing.T) {
		s := newSandbox(t)
		clone := salvaged(t, s, nil)
		gen := filepath.Join(s.memory, "salvage", salvageGen)
		require.NoError(t, os.Rename(gen, gen+"x"))
		require.NoError(t, os.Symlink(salvageGen+"x", gen))
		_, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
		assertRow(t, salvage.SalvagePath, err)
	})
	t.Run("a worktree label", func(t *testing.T) {
		s := newSandbox(t)
		clone := salvaged(t, s, nil)
		editManifest(t, s.dir(), func(m *salvage.Manifest) { m.Worktrees[0].Path = "../worktree-0" })
		_, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
		assertRow(t, salvage.SalvagePath, err)
	})
}

// TestVerifyOverCap reads no payload past salvage.capBytes: each one
// that does not fit in what is left is the item over-cap and is not
// imported, bundles first (REQ-0253).
func TestVerifyOverCap(t *testing.T) {
	s := newSandbox(t)
	put(t, s.repo, "a.log", "log\n")
	clone := salvaged(t, s, nil)
	m := readManifest(t, s.dir())
	require.Len(t, m.Bundles, 1)
	require.Len(t, m.Files, 1)

	c := s.verifyConfig(t, clone)
	c.CapBytes = m.Bundles[0].Size
	v, err := salvage.Verify(t.Context(), c)
	require.NoError(t, err)
	assert.Equal(t, []string{"over-cap ignored.tar.gz"}, items(v), "the bundle fits, the archive does not")
	assert.NotEmpty(t, v.Repos[0].Imported)
	assert.True(t, v.SecretBearing, "the archive is stored whether or not it is verified")

	c = s.verifyConfig(t, repo(t))
	c.CapBytes = m.Bundles[0].Size - 1
	v, err = salvage.Verify(t.Context(), c)
	require.NoError(t, err)
	assert.Equal(t, []string{"over-cap repo.bundle"}, items(v), "the archive fits what the bundle left")
	assert.Empty(t, v.Repos[0].Imported)
	assert.Equal(t, []string{salvage.ReasonOverCap}, salvage.Reasons(v.Skipped))

	t.Run("the bundles of every repo first", func(t *testing.T) {
		// The archive of the first repo would take the room of the second
		// repo's bundle; the bundles are read first, so the archive is
		// the one over the cap.
		w := newSandbox(t)
		wm := filepath.Join(t.TempDir(), "web")
		require.NoError(t, os.Mkdir(wm, 0o700))
		salvaged(t, sandbox{repo: w.repo, memory: wm, home: w.home, head: w.head}, func(c *salvage.Config) { c.Repos[0].Dir = "web" })
		webBundle := readManifest(t, filepath.Join(wm, "salvage", salvageGen, salvageID)).Bundles[0].Size
		c := s.verifyConfig(t, repo(t))
		c.Repos = append(c.Repos, salvage.VerifyRepo{Dir: "web", Memory: wm, Clone: repo(t)})
		c.CapBytes = m.Bundles[0].Size + webBundle
		v, err := salvage.Verify(t.Context(), c)
		require.NoError(t, err)
		assert.Equal(t, []string{"over-cap ignored.tar.gz"}, items(v))
		require.Len(t, v.Repos, 2)
		assert.NotEmpty(t, v.Repos[1].Imported)
	})
}

// TestVerifyManifestRefs makes payload-mismatch a ref of the manifest
// that no bundle holds, and a ref it names twice.
func TestVerifyManifestRefs(t *testing.T) {
	tests := []struct {
		name string
		edit func(*salvage.Manifest)
		want []string
	}{
		{"a ref of a repository no bundle holds", func(m *salvage.Manifest) {
			m.Refs = append(m.Refs, salvage.Ref{Nested: "worktree-0/gone", Ref: "refs/heads/main", SHA: m.Refs[0].SHA})
		}, []string{"payload-mismatch refs/heads/main"}},
		{"a ref named twice", func(m *salvage.Manifest) {
			m.Refs = append(m.Refs, m.Refs[0])
		}, []string{"payload-mismatch refs/heads/main"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSandbox(t)
			clone := salvaged(t, s, nil)
			editManifest(t, s.dir(), func(m *salvage.Manifest) {
				require.Equal(t, "refs/heads/main", m.Refs[0].Ref)
				tt.edit(m)
			})
			v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
			require.NoError(t, err)
			assert.Equal(t, tt.want, items(v))
		})
	}
}

// TestVerifyPayloadMismatch makes payload-mismatch a payload whose size
// or sha256 differs from the manifest, and one that is absent; a
// mismatched bundle is not imported.
func TestVerifyPayloadMismatch(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(t *testing.T, dir string)
		item   string
		bundle bool
	}{
		{"a changed archive", func(t *testing.T, dir string) {
			t.Helper()
			f := filepath.Join(dir, salvage.IgnoredFile)
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			data[len(data)-1] ^= 1
			require.NoError(t, os.WriteFile(f, data, 0o600))
		}, "ignored.tar.gz", false},
		{"an absent archive", func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.Remove(filepath.Join(dir, salvage.IgnoredFile)))
		}, "ignored.tar.gz", false},
		{"a longer bundle", func(t *testing.T, dir string) {
			t.Helper()
			f, err := os.OpenFile(filepath.Join(dir, "repo.bundle"), os.O_WRONLY|os.O_APPEND, 0)
			require.NoError(t, err)
			_, err = f.WriteString("x")
			require.NoError(t, errors.Join(err, f.Close()))
		}, "repo.bundle", true},
		{"another size in the manifest", func(t *testing.T, dir string) {
			t.Helper()
			editManifest(t, dir, func(m *salvage.Manifest) { m.Bundles[0].Size-- })
		}, "repo.bundle", true},
		{"a bundle ref the manifest does not name", func(t *testing.T, dir string) {
			t.Helper()
			editManifest(t, dir, func(m *salvage.Manifest) { m.Refs = m.Refs[1:] })
		}, "repo.bundle", true},
		{"a bundle ref at another object", func(t *testing.T, dir string) {
			t.Helper()
			editManifest(t, dir, func(m *salvage.Manifest) { m.Refs[0].SHA = strings.Repeat("d", 40) })
		}, "repo.bundle", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSandbox(t)
			put(t, s.repo, "a.log", "log\n")
			put(t, s.repo, "kept", "dirty\n")
			clone := salvaged(t, s, nil)
			tt.edit(t, s.dir())
			v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
			require.NoError(t, err)
			assert.Equal(t, []string{"payload-mismatch " + tt.item}, items(v))
			assert.Equal(t, tt.bundle, len(v.Repos[0].Imported) == 0)
		})
	}
}

// TestVerifyRefuses refuses a config it cannot name refs with, before
// it reads anything.
func TestVerifyRefuses(t *testing.T) {
	s := newSandbox(t)
	tests := []struct {
		name string
		edit func(*salvage.VerifyConfig)
	}{
		{"no runner", func(c *salvage.VerifyConfig) { c.Git = nil }},
		{"a generation that is no ULID", func(c *salvage.VerifyConfig) { c.Generation = "g" }},
		{"a salvage id that is no ULID", func(c *salvage.VerifyConfig) { c.SalvageID = "../x" }},
		{"a project name with a slash", func(c *salvage.VerifyConfig) { c.Name = "a/b" }},
		{"a repo dir git refuses in a ref", func(c *salvage.VerifyConfig) { c.Repos[0].Dir = "x.lock" }},
		{"an empty repo dir", func(c *salvage.VerifyConfig) { c.Repos[0].Dir = "" }},
		{"no work dir", func(c *salvage.VerifyConfig) { c.Work = "" }},
		{"a relative work dir", func(c *salvage.VerifyConfig) { c.Work = "work" }},
		{"a work dir others can open", func(c *salvage.VerifyConfig) {
			c.Work = t.TempDir()
			require.NoError(t, os.Chmod(c.Work, 0o755))
		}},
		{"a work dir that is a link", func(c *salvage.VerifyConfig) {
			c.Work = filepath.Join(t.TempDir(), "link")
			require.NoError(t, os.Symlink(workDir(t), c.Work))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := s.verifyConfig(t, repo(t))
			tt.edit(&c)
			_, err := salvage.Verify(t.Context(), c)
			require.Error(t, err)
		})
	}
}

// workDir returns a new work dir of the host half, 0700.
func workDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o700))
	return dir
}
