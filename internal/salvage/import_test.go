// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package salvage_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

// replaceBundle writes a bundle of the refs of the sandbox repo in
// place of repo.bundle, and the manifest's size, sha256 and refs to
// match it, as a julieta that bundled those refs would have.
func replaceBundle(t *testing.T, s sandbox, refs ...string) {
	t.Helper()
	built := filepath.Join(t.TempDir(), "b.bundle")
	git(t, s.repo, append([]string{"bundle", "create", "-q", built}, refs...)...)
	data, err := os.ReadFile(built)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(s.dir(), "repo.bundle"), data, 0o600))
	sum := sha256.Sum256(data)
	var heads []salvage.Ref
	for line := range strings.Lines(git(t, s.repo, "bundle", "list-heads", built)) {
		oid, ref, _ := strings.Cut(strings.TrimSpace(line), " ")
		heads = append(heads, salvage.Ref{Ref: ref, SHA: oid})
	}
	editManifest(t, s.dir(), func(m *salvage.Manifest) {
		m.Bundles = []salvage.Bundle{{File: "repo.bundle", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))}}
		m.Refs = heads
	})
}

// TestImportNested imports a nested repository's bundle into the
// holder's clone, below nested-<k>/, and the repo's own below its dir.
func TestImportNested(t *testing.T) {
	s := newSandbox(t)
	nested := filepath.Join(s.repo, "vendor", "lib")
	require.NoError(t, os.MkdirAll(nested, 0o700))
	git(t, nested, "init", "-q")
	inner := commit(t, nested, "nested")
	clone := salvaged(t, s, nil)

	v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
	require.NoError(t, err)
	assert.Empty(t, v.Skipped)
	prefix := v.RefsPrefix + "api/"
	assert.Equal(t, inner, git(t, clone, "rev-parse", prefix+"nested-1/heads/main"))
	assert.Equal(t, s.head, git(t, clone, "rev-parse", prefix+"heads/main"))
}

// TestImportRefNames fails the bundle as fsck-failed, importing none of
// it, when a ref name the manifest or the bundle names is one the
// validator refuses, or a salvage ref of another generation; a salvage
// ref of an earlier salvage of this generation is imported.
func TestImportRefNames(t *testing.T) {
	otherGen := "refs/salvage/01j9zc2q8x7w4t6y3r5e1v9k0q/" + salvageID + "/worktree-0"
	earlier := "refs/salvage/" + salvageGen + "/01jb000000000000000000000b/worktree-0"
	tests := []struct {
		name  string
		setup func(t *testing.T, s sandbox)
	}{
		{"a name with .. in the manifest", func(t *testing.T, s sandbox) {
			editManifest(t, s.dir(), func(m *salvage.Manifest) { m.Refs[0].Ref = "refs/heads/a..b" })
		}},
		{"a name with a control character in the manifest", func(t *testing.T, s sandbox) {
			editManifest(t, s.dir(), func(m *salvage.Manifest) { m.Refs[0].Ref = "refs/heads/a\x1bb" })
		}},
		{"a ref outside the bundle namespaces", func(t *testing.T, s sandbox) {
			git(t, s.repo, "update-ref", "refs/remotes/origin/main", s.head)
			replaceBundle(t, s, "refs/heads/main", "refs/remotes/origin/main")
		}},
		{"a salvage ref of another generation", func(t *testing.T, s sandbox) {
			git(t, s.repo, "update-ref", otherGen, s.head)
			replaceBundle(t, s, "refs/heads/main", otherGen)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSandbox(t)
			clone := salvaged(t, s, nil)
			tt.setup(t, s)
			v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
			require.NoError(t, err)
			assert.Equal(t, []string{"fsck-failed repo.bundle"}, items(v))
			assert.Empty(t, git(t, clone, "for-each-ref"), "nothing of the bundle is imported")
		})
	}
	t.Run("an earlier salvage of this generation", func(t *testing.T) {
		s := newSandbox(t)
		git(t, s.repo, "update-ref", earlier, s.head)
		clone := salvaged(t, s, nil)
		v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
		require.NoError(t, err)
		assert.Empty(t, v.Skipped)
		assert.Equal(t, s.head, git(t, clone, "rev-parse", v.RefsPrefix+"api/"+strings.TrimPrefix(earlier, "refs/")))
	})
}

// TestImportFsckFailed makes fsck-failed a bundle whose head is a
// malformed object, which git bundle verify and unbundle let through
// (05 5.1): no ref of it is created.
func TestImportFsckFailed(t *testing.T) {
	s := newSandbox(t)
	tree := git(t, s.repo, "rev-parse", "HEAD^{tree}")
	bad := hashCommit(t, s.repo, "tree "+tree+"\nparent "+s.head+"\nauthor nobody\ncommitter nobody\n\nmalformed\n")
	git(t, s.repo, "branch", "bad", bad)
	clone := salvaged(t, s, nil)

	v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
	require.NoError(t, err)
	require.Equal(t, []string{"fsck-failed repo.bundle"}, items(v))
	assert.Contains(t, v.Skipped[0].Detail, "missingEmail", "git fsck --strict found it")
	assert.Empty(t, git(t, clone, "for-each-ref"))
	assert.False(t, salvage.Complete(v.Skipped))
}

// serveHTTPS serves the files below base over HTTPS on 127.0.0.1, which
// git reads as the dumb HTTP protocol, and returns the host and port of
// the server and the git host entry that trusts its certificate.
func serveHTTPS(t *testing.T, base string) (string, gitsafe.GitHost) {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.FileServer(http.Dir(base)))
	srv.Config.ErrorLog = slog.NewLogLogger(slog.DiscardHandler, slog.LevelError)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	var buf bytes.Buffer
	require.NoError(t, pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
	require.NoError(t, os.WriteFile(caFile, buf.Bytes(), 0o600))
	host := srv.Listener.Addr().String()
	return host, gitsafe.GitHost{Host: host, CAFile: caFile}
}

// TestImportPrerequisites verifies a bundle whose prerequisites, the
// repos[].base the host knows, the host clone lacks: the fetch of
// origin into refs/romeu/origin/<dir>/ brings them, and the bundle
// verifies and is imported (REQ-0715). Without an origin, the bundle
// is fsck-failed.
func TestImportPrerequisites(t *testing.T) {
	s := newSandbox(t)
	base := t.TempDir()
	git(t, base, "clone", "-q", "--bare", s.repo, "api.git")
	git(t, filepath.Join(base, "api.git"), "update-server-info")
	next := commit(t, s.repo, "unpushed")
	salvaged(t, s, func(c *salvage.Config) { c.Repos[0].Base = []string{s.head} })

	t.Run("an origin", func(t *testing.T) {
		host, gh := serveHTTPS(t, base)
		clone := repo(t)
		c := s.verifyConfig(t, clone)
		c.Git = hostRunner(t, gh)
		c.Repos[0].Origin = "https://" + host + "/api.git"
		v, err := salvage.Verify(t.Context(), c)
		require.NoError(t, err)
		assert.Empty(t, v.Skipped)
		assert.Equal(t, s.head, git(t, clone, "rev-parse", "refs/romeu/origin/api/main"), "the fetch of origin")
		assert.Equal(t, next, git(t, clone, "rev-parse", v.RefsPrefix+"api/heads/main"))
	})

	t.Run("no origin", func(t *testing.T) {
		clone := repo(t)
		v, err := salvage.Verify(t.Context(), s.verifyConfig(t, clone))
		require.NoError(t, err)
		assert.Equal(t, []string{"fsck-failed repo.bundle"}, items(v))
		assert.Empty(t, git(t, clone, "for-each-ref"))
	})
}
