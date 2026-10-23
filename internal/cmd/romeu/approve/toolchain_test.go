// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/sbxdrv"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// fixtureCatalog builds a catalog of a base file and one family.
func fixtureCatalog(t *testing.T, family string) catalog.Catalog {
	t.Helper()
	base := "schema: catalog/v1\nmeta: [mise.example]\nbase: [git.example]\n" +
		"domains:\n  mise.example: {upload: false}\n  git.example: {upload: true}\n"
	c, err := catalog.Build([]catalog.Source{{Name: catalog.BaseFile, Data: []byte(base)}, {Name: "f.yaml", Data: []byte(family)}})
	require.NoError(t, err)
	return c
}

// sampleFamily is a family with a tool that needs two domains and one
// that needs none.
const sampleFamily = "schema: catalog/v1\ndomains:\n  dl.example: {upload: false}\n  up.example: {upload: true}\n" +
	"tools:\n  core:go: [dl.example, up.example]\n  aqua:jqlang/jq: []\n"

func TestDecodeCatalog(t *testing.T) {
	t.Parallel()
	got := DecodeCatalog(fixtureCatalog(t, sampleFamily))
	assert.Equal(t, Catalog{
		Meta: []string{"mise.example"},
		Base: []string{"git.example"},
		Domains: map[string]Domain{
			"mise.example": {Upload: false}, "git.example": {Upload: true},
			"dl.example": {Upload: false}, "up.example": {Upload: true},
		},
		Tools: map[string][]string{"core:go": {"dl.example", "up.example"}, "aqua:jqlang/jq": {}},
	}, got)

	data, err := canon.CanonicalJSON(got)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"aqua:jqlang/jq":[]`, "a tool that needs no domain is [], never null")
}

// TestDecodeCatalogOfTheEmbeddedOne: the embedded catalog decodes and
// digests, and its record reads back to the same decoded form.
func TestDecodeCatalogOfTheEmbeddedOne(t *testing.T) {
	t.Parallel()
	c, err := catalog.Embedded()
	require.NoError(t, err)
	d := DecodeCatalog(c)
	_, err = canon.Digest(canon.Catalog, d)
	require.NoError(t, err)
	m, err := recordCatalog(d)
	require.NoError(t, err)
	back, err := recordedCatalog(m)
	require.NoError(t, err)
	assert.Equal(t, d, back)
}

// TestCatalogDigestFlips: the digest of the decoded catalog changes with
// a domain of a tool and with an upload flag.
func TestCatalogDigestFlips(t *testing.T) {
	t.Parallel()
	digest := func(family string) canon.Sum[canon.CatalogKind] {
		s, err := canon.Digest(canon.Catalog, DecodeCatalog(fixtureCatalog(t, family)))
		require.NoError(t, err)
		return s
	}
	base := digest(sampleFamily)
	assert.Equal(t, base, digest(sampleFamily), "one catalog, one digest")
	assert.NotEqual(t, base, digest("schema: catalog/v1\ndomains:\n  dl.example: {upload: false}\n  up.example: {upload: true}\n"+
		"tools:\n  core:go: [dl.example]\n  aqua:jqlang/jq: [up.example]\n"), "a domain moved to another tool")
	assert.NotEqual(t, base, digest("schema: catalog/v1\ndomains:\n  dl.example: {upload: true}\n  up.example: {upload: true}\n"+
		"tools:\n  core:go: [dl.example, up.example]\n  aqua:jqlang/jq: []\n"), "an upload flag")
}

// sampleLive is a live toolchain.
func sampleLive(t *testing.T) Live {
	t.Helper()
	d := DecodeCatalog(fixtureCatalog(t, sampleFamily))
	sum, err := canon.Digest(canon.Catalog, d)
	require.NoError(t, err)
	return Live{
		Sbx:           "0.49.2",
		Romeu:         state.Binary{Version: "v1.0.0", SHA256: state.SHA256{1}},
		Julieta:       state.JulietaBinary{Version: "v1.0.0", Protocol: 1, SHA256: state.SHA256{2}},
		Catalog:       d,
		CatalogDigest: sum,
	}
}

// recordOf returns the record gate 1 writes for live.
func recordOf(t *testing.T, live Live) *state.Toolchain {
	t.Helper()
	rec := &state.Toolchain{Schema: state.ToolchainField}
	require.NoError(t, acknowledge(rec, live, time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)))
	return rec
}

// TestChanges is the gate 1 flip table (I7, 05 5.2 I7 unit cases; 01
// 1.4, REQ-0062): each member flips the gate alone, and the record of a
// toolchain matches it.
//
//romeu:invariant I7 test
func TestChanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*Live)
		member string
	}{
		{"the sbx version", func(l *Live) { l.Sbx = "0.50.0" }, "sbxVersion"},
		{"romeu's version", func(l *Live) { l.Romeu.Version = "v1.1.0" }, "romeu.version"},
		{"romeu's sha256", func(l *Live) { l.Romeu.SHA256 = state.SHA256{9} }, "romeu.sha256"},
		{"the embedded julieta's sha256", func(l *Live) { l.Julieta.SHA256 = state.SHA256{9} }, "julieta.sha256"},
		{"the julieta version", func(l *Live) { l.Julieta.Version = "v1.1.0" }, "julieta.version"},
		{"the julieta protocol", func(l *Live) { l.Julieta.Protocol = 2 }, "julieta.protocol"},
		{"the catalog digest", func(l *Live) { l.CatalogDigest = canon.Sum[canon.CatalogKind]{9} }, "catalogDigest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			live := sampleLive(t)
			rec := recordOf(t, live)
			require.Empty(t, Changes(rec, live), "the record of a toolchain holds it")
			tt.change(&live)
			got := Changes(rec, live)
			require.Len(t, got, 1)
			assert.Equal(t, tt.member, got[0].Member)
			assert.NotEqual(t, got[0].Recorded, got[0].Live)
		})
	}
	t.Run("no record changes every member", func(t *testing.T) {
		t.Parallel()
		got := Changes(nil, sampleLive(t))
		assert.Len(t, got, 7)
		for _, c := range got {
			assert.Empty(t, c.Recorded, c.Member)
		}
	})
}

// openStore opens a state store in a temporary directory.
func openStore(t *testing.T) *state.Store {
	t.Helper()
	st, err := state.Open(filepath.Join(t.TempDir(), "state"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// write writes rec as the record of gate 1 in st.
func write(t *testing.T, st *state.Store, rec *state.Toolchain) {
	t.Helper()
	lk, err := st.Lock()
	require.NoError(t, err)
	defer lk.Unlock()
	require.NoError(t, st.UpdateToolchain(lk, func(t *state.Toolchain) error { *t = *rec; return nil }))
}

// TestCheck is the check the preflight and sync call: nil on the
// recorded toolchain, toolchain-approval-required (RJ-302, exit 3) with
// no record or on any change, naming the change, and record-unreadable
// (RJ-334) on a record that does not decode.
//
//romeu:invariant I7 test
func TestCheck(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)

	t.Run("no record", func(t *testing.T) {
		t.Parallel()
		err := Check(openStore(t), live)
		testsupport.RequireID(t, "RJ-302", err)
		assert.Equal(t, 3, cli.Of(err).Exit)
	})
	t.Run("the recorded toolchain", func(t *testing.T) {
		t.Parallel()
		st := openStore(t)
		write(t, st, recordOf(t, live))
		assert.NoError(t, Check(st, live))
	})
	t.Run("another sbx", func(t *testing.T) {
		t.Parallel()
		st := openStore(t)
		write(t, st, recordOf(t, live))
		other := live
		other.Sbx = "0.50.0"
		err := Check(st, other)
		testsupport.RequireID(t, "RJ-302", err)
		assert.Contains(t, err.Error(), "sbxVersion")
		assert.Contains(t, cli.Of(err).Details, cli.Detail{Key: "sbxVersion", Value: "0.49.2 -> 0.50.0"})
	})
	t.Run("a record that does not decode", func(t *testing.T) {
		t.Parallel()
		st := openStore(t)
		require.NoError(t, os.WriteFile(filepath.Join(st.Dir(), "toolchain.json"), []byte("{"), 0o600))
		testsupport.RequireID(t, "RJ-334", Check(st, live))
	})
}

// TestVersion: a stamped build records its stamp; a build without one
// is a dev build, v0.0.0-dev+<commit> (01 1.4, REQ-0065), its commit
// from the stamp or the VCS information of the binary.
func TestVersion(t *testing.T) {
	withBuildInfo := func(rev string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: rev}}}, true
		}
	}
	tests := []struct {
		name string
		v    cli.Version
		info func() (*debug.BuildInfo, bool)
		want string
	}{
		{"a release", cli.Version{Version: "v1.2.3", Commit: "abc"}, withBuildInfo("def"), "v1.2.3"},
		{"a dev build with a stamped commit", cli.Version{Version: "devel", Commit: "abc"}, withBuildInfo("def"), "v0.0.0-dev+abc"},
		{"a dev build with the VCS commit", cli.Version{Version: "devel", Commit: "unknown"}, withBuildInfo("def"), "v0.0.0-dev+def"},
		{"a dev build with no commit", cli.Version{Version: "devel", Commit: "unknown"}, func() (*debug.BuildInfo, bool) { return nil, false }, "v0.0.0-dev+unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := buildInfo
			buildInfo = tt.info
			t.Cleanup(func() { buildInfo = old })
			assert.Equal(t, tt.want, Version(tt.v))
		})
	}
}

// TestMeasure reads the binary's sha256 and the embedded catalog.
func TestMeasure(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "romeu")
	require.NoError(t, os.WriteFile(bin, []byte("a build"), 0o600))
	old := romeuSHA256
	romeuSHA256 = func() (state.SHA256, error) { return fileSHA256(func() (string, error) { return bin, nil }) }
	t.Cleanup(func() { romeuSHA256 = old })

	live, err := Measure(sbxdrv.Release{Major: 0, Minor: 49, Patch: 2}, cli.Version{Version: "v1.0.0", Protocol: 1})
	require.NoError(t, err)
	assert.Equal(t, "0.49.2", live.Sbx)
	assert.Equal(t, state.Binary{Version: "v1.0.0", SHA256: sha256.Sum256([]byte("a build"))}, live.Romeu)
	assert.Equal(t, state.JulietaBinary{Version: "v0.0.0-placeholder", Protocol: 1, SHA256: sha256.Sum256(nil)}, live.Julieta,
		"a build embeds the placeholder until the embed package lands")
	want, err := canon.Digest(canon.Catalog, live.Catalog)
	require.NoError(t, err)
	assert.Equal(t, want, live.CatalogDigest)

	romeuSHA256 = func() (state.SHA256, error) {
		return fileSHA256(func() (string, error) { return "", errors.New("no path") })
	}
	_, err = Measure(sbxdrv.Release{}, cli.Version{})
	testsupport.RequireID(t, "RJ-348", err)
}

// TestRomeuSHA256IsTheRunningImage: on Linux the sha256 is read from
// /proc/self/exe, the image that runs, not from the path os.Executable
// names, which a replaced file would change; elsewhere from
// os.Executable. The hash is read once per process.
func TestRomeuSHA256IsTheRunningImage(t *testing.T) {
	t.Parallel()
	path, err := selfPath()
	require.NoError(t, err)
	if runtime.GOOS == "linux" {
		assert.Equal(t, "/proc/self/exe", path)
	} else {
		want, err := os.Executable()
		require.NoError(t, err)
		assert.Equal(t, want, path)
	}
	exe, err := os.Executable()
	require.NoError(t, err)
	data, err := os.ReadFile(exe)
	require.NoError(t, err)
	got, err := romeuSHA256()
	require.NoError(t, err)
	assert.Equal(t, state.SHA256(sha256.Sum256(data)), got)
	again, err := romeuSHA256()
	require.NoError(t, err)
	assert.Equal(t, got, again)
}

// releaseEmbedPath is build.EmbedPath of tools/release
// (tools/release/internal/build/build.go), the file a release build maps
// the real julieta into; a test of internal/ cannot import tools/, so
// TestPlaceholderJulietaUntilEmbedded holds this copy to the source.
const releaseEmbedPath = "internal/embedded/julieta"

// moduleRoot is the repository, from this package's directory.
const moduleRoot = "../../../.."

// julietaEmbedders returns the Go files of fsys that embed the file at
// embedPath: a //go:embed directive in a file of its directory, or one
// with a pattern that, read from the file's directory as go:embed reads
// it, names the file or a directory above it, or is a glob that matches
// either. A pattern cannot climb with "..", so a file elsewhere whose
// pattern only shares the base name (kits/embed.go embeds the kit
// directory kits/julieta) does not embed it.
func julietaEmbedders(t *testing.T, fsys fs.FS, embedPath string) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		for line := range strings.Lines(string(data)) {
			args, ok := strings.CutPrefix(strings.TrimSpace(line), "//go:embed")
			if !ok || args != "" && args[0] != ' ' && args[0] != '\t' {
				continue
			}
			if path.Dir(p) == path.Dir(embedPath) || slices.ContainsFunc(embedPatterns(args), func(pattern string) bool {
				return embedsPath(path.Dir(p), pattern, embedPath)
			}) {
				out = append(out, p)
				break
			}
		}
		return nil
	})
	require.NoError(t, err)
	return out
}

// embedPatterns splits the arguments of a //go:embed directive as
// go/build reads them: words separated by spaces or tabs, each bare or a
// Go string literal, double-quoted or raw. A literal that does not parse
// is kept as it is; the compiler refuses it anyway.
func embedPatterns(args string) []string {
	var out []string
	for {
		args = strings.TrimLeft(args, " \t")
		if args == "" {
			return out
		}
		var word string
		if q := args[0]; q == '"' || q == '`' {
			end := 1
			for end < len(args) && args[end] != q {
				if q == '"' && args[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(args) {
				return append(out, args)
			}
			word, args = args[:end+1], args[end+1:]
			if unquoted, err := strconv.Unquote(word); err == nil {
				word = unquoted
			}
		} else {
			end := strings.IndexAny(args, " \t")
			if end < 0 {
				end = len(args)
			}
			word, args = args[:end], args[end:]
		}
		out = append(out, word)
	}
}

// embedsPath reports whether the go:embed pattern of a file in dir takes
// the file at target: the pattern, ignoring any all: prefix and read
// from dir, names target or a directory above it, or is a glob that
// matches either (go:embed takes a matched directory whole).
func embedsPath(dir, pattern, target string) bool {
	joined := path.Join(dir, strings.TrimPrefix(pattern, "all:"))
	for name := target; name != "." && name != "/"; name = path.Dir(name) {
		if ok, err := path.Match(joined, name); name == joined || err == nil && ok {
			return true
		}
	}
	return false
}

// TestPlaceholderJulietaUntilEmbedded is the guard of the placeholder:
// once a package embeds the julieta tools/release maps in (T043, T066),
// embeddedJulieta must read it, or gate 1 would record the placeholder
// for a real julieta and show "julieta unchanged" for a changed one.
// The test fails until embeddedJulieta stops returning the placeholder.
//
//romeu:invariant I7 test
func TestPlaceholderJulietaUntilEmbedded(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join(moduleRoot, "tools", "release", "internal", "build", "build.go"))
	require.NoError(t, err)
	require.Contains(t, string(src), `const EmbedPath = "`+releaseEmbedPath+`"`, "build.EmbedPath moved: update releaseEmbedPath")

	fixture := fstest.MapFS{
		"internal/embedded/embedded.go": {Data: []byte("package embedded\n\nimport _ \"embed\"\n\n//go:embed julieta\nvar Julieta []byte\n")},
		"internal/catalog/catalog.go":   {Data: []byte("package catalog\n\n//go:embed egress/*.yaml\nvar files embed.FS\n")},
		"internal/above.go":             {Data: []byte("package internal\n\n//go:embed embedded/julieta\nvar J []byte\n")},
		"internal/tree.go":              {Data: []byte("package internal\n\n//go:embed all:embedded\nvar T embed.FS\n")},
		"internal/glob.go":              {Data: []byte("package internal\n\n//go:embed embedded/j*\nvar G embed.FS\n")},
		"internal/allfile.go":           {Data: []byte("package internal\n\n//go:embed all:embedded/julieta\nvar A []byte\n")},
		"internal/parent.go":            {Data: []byte("package internal\n\n//go:embed emb*\nvar P embed.FS\n")},
		"internal/quoted.go":            {Data: []byte("package internal\n\n//go:embed \"embedded/julieta\"\nvar Q []byte\n")},
		"internal/raw.go":               {Data: []byte("package internal\n\n//go:embed `embedded/julieta`\nvar R []byte\n")},
		"internal/tab.go":               {Data: []byte("package internal\n\n//go:embed\tembedded/julieta\nvar B []byte\n")},
		"internal/many.go":              {Data: []byte("package internal\n\n//go:embed README.md\n//go:embed x.txt embedded/julieta\nvar M embed.FS\n")},
		"internal/near.go":              {Data: []byte("package internal\n\n//go:embed embedded/juli embedded/julieta2 \"embedded/julieta \"\n// go:embed embedded/julieta\n//go:embeddedjulieta\nvar N embed.FS\n")},
		"root.go":                       {Data: []byte("package root\n\n//go:embed internal\nvar I embed.FS\n")},
		"kits/embed.go":                 {Data: []byte("package kits\n\n//go:embed julieta os-base\nvar FS embed.FS\n")},
	}
	require.Equal(t, []string{
		"internal/above.go", "internal/allfile.go", "internal/embedded/embedded.go", "internal/glob.go", "internal/many.go",
		"internal/parent.go", "internal/quoted.go", "internal/raw.go", "internal/tab.go", "internal/tree.go", "root.go",
	}, julietaEmbedders(t, fixture, releaseEmbedPath), "the guard finds every embedder, and not the kit directory of the same name or a name that only begins like it")

	if embeddedJulieta(cli.Version{Protocol: 1}).SHA256 == placeholderSHA256 {
		assert.Empty(t, julietaEmbedders(t, os.DirFS(moduleRoot), releaseEmbedPath),
			"a package embeds the julieta of tools/release now, and embeddedJulieta still returns the placeholder: make it read the embedded julieta")
	}
}
