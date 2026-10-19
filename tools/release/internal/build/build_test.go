// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildFixture builds the fixture module as a dry run into a new
// directory and returns the root and the result.
func buildFixture(t *testing.T, root string) Result {
	t.Helper()
	goBin, err := ResolveGo(t.Context())
	require.NoError(t, err)
	res, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: t.TempDir(), DryRun: true, GoBin: goBin})
	require.NoError(t, err)
	return res
}

// TestDryRunBuild: the two archives and checksums.txt are written; each
// archive holds romeu, COPYING and the notices in that order, with the
// fixed times and owners; the romeu binary is a darwin Mach-O that
// carries the julieta binary of the same arch and the version stamp.
func TestDryRunBuild(t *testing.T) {
	root := fixtureModule(t)
	res := buildFixture(t, root)

	require.Len(t, res.Archives, 2)
	assert.Equal(t, "romeu_v1.0.0_darwin_amd64.tar.gz", filepath.Base(res.Archives[0]))
	assert.Equal(t, "romeu_v1.0.0_darwin_arm64.tar.gz", filepath.Base(res.Archives[1]))

	sums, err := os.ReadFile(res.Checksums)
	require.NoError(t, err)
	archives := map[string][]byte{}
	for _, path := range res.Archives {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		archives[filepath.Base(path)] = data
	}
	assert.Equal(t, string(Checksums(archives)), string(sums))

	head, err := git(t.Context(), root, "rev-parse", "HEAD")
	require.NoError(t, err)
	for name, data := range archives {
		t.Run(name, func(t *testing.T) {
			headers, content := members(t, data)
			require.Len(t, headers, 3)
			var names []string
			for _, h := range headers {
				names = append(names, h.Name)
				assert.True(t, h.ModTime.Equal(commitTime), "%s mtime %s", h.Name, h.ModTime)
				assert.Zero(t, h.Uid)
				assert.Zero(t, h.Gid)
				assert.Empty(t, h.Uname)
				assert.Empty(t, h.Gname)
			}
			assert.Equal(t, []string{"romeu", "COPYING", NoticesName}, names)
			assert.Equal(t, int64(0o755), headers[0].Mode)

			assert.Equal(t, "The fixture's own license.\n", string(content["COPYING"]))
			notices := string(content[NoticesName])
			assert.Contains(t, notices, "example.com/dep v0.0.0")
			assert.Contains(t, notices, "The dep license: use it freely.")
			assert.Contains(t, notices, "linked by: julieta, romeu")

			romeu := content["romeu"]
			assert.True(t, bytes.HasPrefix(romeu, []byte{0xcf, 0xfa, 0xed, 0xfe}), "a 64-bit Mach-O")
			assert.Contains(t, string(romeu), markerJulieta, "romeu embeds julieta")
			assert.Contains(t, string(romeu), "v1.0.0")
			assert.Contains(t, string(romeu), strings.TrimSpace(string(head)), "the commit stamp")
			assert.Contains(t, string(romeu), "2026-03-04T05:06:07Z", "the commit time is the build date")
		})
	}
}

// TestBuildIsReproducible: two builds of one commit write the same
// archives, so the contract of REQ-0905 holds as far as this tool
// controls it (nothing compares them in the release itself).
func TestBuildIsReproducible(t *testing.T) {
	root := fixtureModule(t)
	first, second := buildFixture(t, root), buildFixture(t, root)
	for i := range first.Archives {
		a, err := os.ReadFile(first.Archives[i])
		require.NoError(t, err)
		b, err := os.ReadFile(second.Archives[i])
		require.NoError(t, err)
		assert.True(t, bytes.Equal(a, b), first.Archives[i])
	}
}

// TestBuildRefusesAnotherVersion: the tag must be a build of VERSION.
func TestBuildRefusesAnotherVersion(t *testing.T) {
	root := fixtureModule(t)
	_, err := Build(t.Context(), Options{Root: root, Version: "v2.0.0", Out: t.TempDir(), DryRun: true, GoBin: "go"})
	require.ErrorContains(t, err, "is not a build of VERSION 1.0.0")
}

// TestDryRunBuildMakesNoNetworkRead is the fixture "dry-run build makes
// no network read": a server counts every request, the gate points at
// it, and a dry run sends none, while the same build without --dry-run
// sends one.
func TestDryRunBuildMakesNoNetworkRead(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	root := fixtureModule(t)
	goBin, err := ResolveGo(t.Context())
	require.NoError(t, err)
	gate := &FuzzGate{API: server.URL, Repo: "owner/romeu-e-julieta", Token: "token"}

	_, err = Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: t.TempDir(), DryRun: true, GoBin: goBin, Gate: gate})
	require.NoError(t, err)
	assert.Zero(t, requests.Load(), "a dry run reads no network")

	_, err = Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: t.TempDir(), GoBin: goBin, Gate: gate})
	require.ErrorContains(t, err, "the fuzz run")
	assert.Equal(t, int32(1), requests.Load(), "the same build without --dry-run asks the API")
}

// TestReleaseBuildNeedsTheGate: a build that is not a dry run and has no
// gate refuses before it builds anything.
func TestReleaseBuildNeedsTheGate(t *testing.T) {
	root := fixtureModule(t)
	out := t.TempDir()
	_, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: out, GoBin: "go"})
	require.ErrorContains(t, err, "needs the fuzz gate")
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestGoEnvIsBuiltFromNothing: a variable of the caller that changes
// what is built has no effect, and the network is off.
func TestGoEnvIsBuiltFromNothing(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=evil")
	t.Setenv("GOPROXY", "https://proxy.example.com")
	t.Setenv("GOOS", "plan9")
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOEXPERIMENT", "boringcrypto")
	t.Setenv("PATH", "/usr/bin")
	env := goEnv("/pinned/bin/go")
	assert.Contains(t, env, "PATH=/pinned/bin:/usr/bin")
	assert.Contains(t, env, "GOPROXY=off")
	assert.Contains(t, env, "GOFLAGS=-mod=readonly")
	assert.Contains(t, env, "GOTOOLCHAIN=local")
	assert.Contains(t, env, "GOENV=off")
	assert.Contains(t, env, "GOWORK=off")
	for _, kv := range env {
		assert.False(t, strings.HasPrefix(kv, "GOOS=") || strings.HasPrefix(kv, "CGO_ENABLED=") || strings.HasPrefix(kv, "GOEXPERIMENT="), kv)
		assert.NotContains(t, kv, "evil")
		assert.NotContains(t, kv, "proxy.example.com")
	}
}

// TestNoticesRefuseAModuleWithoutALicense: an archive could not carry
// the notice of such a module, so the build refuses.
func TestNoticesRefuseAModuleWithoutALicense(t *testing.T) {
	root := fixtureModule(t)
	dep := readReplace(t, root)
	require.NoError(t, os.Remove(filepath.Join(dep, "LICENSE")))
	goBin, err := ResolveGo(t.Context())
	require.NoError(t, err)
	_, err = Notices(t.Context(), root, goBin, []Target{{"julieta", "linux", "amd64"}})
	require.ErrorContains(t, err, "example.com/dep")
	require.ErrorContains(t, err, "has no license file")
}

// readReplace returns the directory the fixture's replace directive names.
func readReplace(t *testing.T, root string) string {
	t.Helper()
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	_, dir, ok := strings.Cut(string(mod), "=> ")
	require.True(t, ok)
	return strings.TrimSpace(dir)
}
