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
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	root := fixtureModule(t)
	goBin, err := ResolveGo(t.Context())
	require.NoError(t, err)
	gate := &FuzzGate{API: server.URL, Repo: "owner/romeu-e-julieta", Token: "token",
		Client: server.Client(), AllowHost: strings.TrimPrefix(server.URL, "https://")}
	tag(t, root, "v1.0.0")

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

// TestBuildBehindSymlinks: a repository reached through a symlink and a
// TMPDIR that is one (macOS keeps /var/folders under /private/var) build
// the same, since the overlay key must be the real path the go command
// sees.
func TestBuildBehindSymlinks(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "tmp-link")
	require.NoError(t, os.Symlink(target, link))
	t.Setenv("TMPDIR", link)

	root := fixtureModule(t)
	rootLink := filepath.Join(t.TempDir(), "root-link")
	require.NoError(t, os.Symlink(root, rootLink))
	res, err := Build(t.Context(), Options{Root: rootLink, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), DryRun: true, GoBin: goBinOf(t)})
	require.NoError(t, err)
	assert.True(t, res.Embedded)
	data, err := os.ReadFile(res.Archives[0])
	require.NoError(t, err)
	_, content := members(t, data)
	assert.Contains(t, string(content["romeu"]), markerJulieta)
}

func goBinOf(t *testing.T) string {
	t.Helper()
	goBin, err := ResolveGo(t.Context())
	require.NoError(t, err)
	return goBin
}

// TestEmbeddingDoesNotPassSilently: with no package that consumes
// internal/embedded/julieta a dry run builds and says Embedded is false;
// with one, it says it is embedded.
func TestEmbeddingDoesNotPassSilently(t *testing.T) {
	bare := fixtureModuleWith(t, false)
	res, err := Build(t.Context(), Options{Root: bare, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), DryRun: true, GoBin: goBinOf(t)})
	require.NoError(t, err)
	assert.False(t, res.Embedded)

	root := fixtureModule(t)
	res, err = Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), DryRun: true, GoBin: goBinOf(t)})
	require.NoError(t, err)
	assert.True(t, res.Embedded)
}

// fuzzedGate answers success for the head of root, so the gate passes.
func fuzzedGate(t *testing.T, root string) *FuzzGate {
	t.Helper()
	head, err := git(t.Context(), root, "rev-parse", "HEAD")
	require.NoError(t, err)
	return gateFor(t, 200, runs("schedule", "main", strings.TrimSpace(string(head)), "success"), nil)
}

func TestReleaseBuildRefusesWhenNothingEmbedsJulieta(t *testing.T) {
	bare := fixtureModuleWith(t, false)
	tag(t, bare, "v1.0.0")
	out := filepath.Join(t.TempDir(), "out")
	_, err := Build(t.Context(), Options{Root: bare, Version: "v1.0.0", Out: out, GoBin: goBinOf(t), Gate: fuzzedGate(t, bare)})
	require.ErrorContains(t, err, "embeds no julieta")
	assert.NoDirExists(t, out)
}

func TestReleaseBuildWithTheTagAtHead(t *testing.T) {
	root := fixtureModule(t)
	tag(t, root, "v1.0.0")
	res, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), GoBin: goBinOf(t), Gate: fuzzedGate(t, root)})
	require.NoError(t, err)
	assert.True(t, res.Embedded)
	assert.Len(t, res.Archives, 2)
}

func TestReleaseBuildChecksTheTag(t *testing.T) {
	root := fixtureModule(t)
	gate := fuzzedGate(t, root)
	_, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), GoBin: goBinOf(t), Gate: gate})
	require.ErrorContains(t, err, "does not exist")

	tag(t, root, "v1.0.0")
	write(t, root, map[string]string{"later.txt": "x\n"})
	commit(t, root, "a later commit", commitTime)
	_, err = Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), GoBin: goBinOf(t), Gate: gate})
	require.ErrorContains(t, err, "not HEAD")
}

// TestDirtyTree: any change, an untracked file included, refuses a
// release build and a dry run without --allow-dirty; a dry run with it
// stamps the commit dirty.
func TestDirtyTree(t *testing.T) {
	root := fixtureModule(t)
	tag(t, root, "v1.0.0")
	write(t, root, map[string]string{"untracked.txt": "x\n"})
	out := func() string { return filepath.Join(t.TempDir(), "out") }

	_, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: out(), DryRun: true, GoBin: goBinOf(t)})
	require.ErrorContains(t, err, "the tree has changes")
	_, err = Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: out(), GoBin: goBinOf(t), AllowDirty: true, Gate: fuzzedGate(t, root)})
	require.ErrorContains(t, err, "the tree has changes")

	res, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: out(), DryRun: true, AllowDirty: true, GoBin: goBinOf(t)})
	require.NoError(t, err)
	assert.True(t, res.Dirty)
	data, err := os.ReadFile(res.Archives[0])
	require.NoError(t, err)
	_, content := members(t, data)
	head, err := git(t.Context(), root, "rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, string(content["romeu"]), strings.TrimSpace(string(head))+"-dirty")
}

// TestOutput: a non-empty --out is refused, an empty one is replaced, and
// a failed build leaves nothing, not even a staging directory.
func TestOutput(t *testing.T) {
	root := fixtureModule(t)
	parent := t.TempDir()

	full := filepath.Join(parent, "full")
	write(t, full, map[string]string{"keep.txt": "x\n"})
	_, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: full, DryRun: true, GoBin: goBinOf(t)})
	require.ErrorContains(t, err, "is not empty")
	assert.FileExists(t, filepath.Join(full, "keep.txt"))

	empty := filepath.Join(parent, "empty")
	require.NoError(t, os.Mkdir(empty, 0o755))
	res, err := Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: empty, DryRun: true, GoBin: goBinOf(t)})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(empty, ChecksumsName), res.Checksums)
	assert.FileExists(t, res.Checksums)

	write(t, root, map[string]string{"cmd/julieta/main.go": "package main\n\nfunc main() { broken }\n"})
	commit(t, root, "break julieta", commitTime)
	failed := filepath.Join(parent, "failed")
	_, err = Build(t.Context(), Options{Root: root, Version: "v1.0.0", Out: failed, DryRun: true, GoBin: goBinOf(t)})
	require.ErrorContains(t, err, "build julieta")
	assert.NoDirExists(t, failed)
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"full", "empty"}, names)
}

// TestNoticesCapALicenseFile: a license file over 1 MiB is refused.
func TestNoticesCapALicenseFile(t *testing.T) {
	root := fixtureModule(t)
	dep := readReplace(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(dep, "LICENSE"), bytes.Repeat([]byte("x"), maxLicense+1), 0o644))
	_, err := Notices(t.Context(), root, goBinOf(t), []Target{{"julieta", "linux", "amd64"}})
	require.ErrorContains(t, err, "is larger than")
}
