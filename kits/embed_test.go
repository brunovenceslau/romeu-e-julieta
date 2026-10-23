// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package kits

import (
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
)

// hasDescriptor reports whether dir, in fsys, holds a .yaml descriptor
// directly.
func hasDescriptor(t *testing.T, fsys fs.FS, dir string) bool {
	t.Helper()
	entries, err := fs.ReadDir(fsys, dir)
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() && path.Ext(e.Name()) == ".yaml" {
			return true
		}
	}
	return false
}

// filesBelow returns every file below dir in fsys, the whole tree but
// the testdata/ directories, which are a kit's tests and not its content.
func filesBelow(t *testing.T, fsys fs.FS, dir string) []string {
	t.Helper()
	var files []string
	require.NoError(t, fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "testdata":
			return fs.SkipDir
		case !d.IsDir():
			files = append(files, p)
		}
		return nil
	}))
	return files
}

// TestFSHoldsEveryProductKit holds the embed both ways: each kit
// directory of kits/ (a directory with a descriptor) is in FS with its
// files but its testdata/, and FS holds nothing else, so neither
// pins.yaml nor the material of the mise bootstrap (kits/mise has no
// descriptor) nor a kit's tests is embedded.
func TestFSHoldsEveryProductKit(t *testing.T) {
	disk := os.DirFS(".")
	top, err := fs.ReadDir(disk, ".")
	require.NoError(t, err)
	var onDisk []string
	for _, e := range top {
		if e.IsDir() && hasDescriptor(t, disk, e.Name()) {
			onDisk = append(onDisk, e.Name())
		}
	}
	require.Subset(t, onDisk, []string{"julieta", "os-base", "git-ssh-sign"})

	var embedded []string
	entries, err := fs.ReadDir(FS, ".")
	require.NoError(t, err)
	for _, e := range entries {
		require.True(t, e.IsDir(), "FS holds the file %s, and only kit directories", e.Name())
		embedded = append(embedded, e.Name())
		assert.True(t, hasDescriptor(t, FS, e.Name()), "%s is embedded with no descriptor", e.Name())
	}
	assert.ElementsMatch(t, onDisk, embedded, "kits/ and FS name the same kits")

	for _, name := range onDisk {
		assert.ElementsMatch(t, filesBelow(t, disk, name), filesBelow(t, FS, name), "the files of %s", name)
	}
}

// TestEveryKitParses reads each embedded descriptor with the product's
// one descriptor grammar (internal/oci, REQ-0570), the gate a rendered
// kit passes, so a kit of this directory that the gate would refuse
// fails here first.
func TestEveryKitParses(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		src, err := fs.ReadFile(FS, path.Join(e.Name(), e.Name()+".yaml"))
		require.NoError(t, err, "the descriptor of %s is <name>/<name>.yaml", e.Name())
		kit, err := oci.ParseDescriptor(src)
		require.NoError(t, err, e.Name())
		assert.Equal(t, e.Name(), kit.DisplayName)
		assert.Equal(t, "mixin", kit.Capabilities.Kind, "%s: a product kit is a mixin (06 6.4)", e.Name())
	}
}

// pinned is the part of kits/pins.yaml this file reads: herdr and mise,
// a version and a sha256 for each architecture. tools/kitpin/pins is
// not imported, since a package outside tools/ imports none of tools/.
type pinned struct {
	Herdr, Mise struct {
		Version string
		SHA256  map[string]string
	}
}

// TestJulietaSumsEqualThePins holds the sums file the install steps of
// the julieta kit check against to the pin file: one line per tool and
// architecture, named as the step names its download, with the pinned
// sha256. The steps read the version from args that tools/ci kits
// already holds to the pin file; this closes the hashes.
func TestJulietaSumsEqualThePins(t *testing.T) {
	src, err := os.ReadFile("pins.yaml")
	require.NoError(t, err)
	var p pinned
	require.NoError(t, yaml.Unmarshal(src, &p))

	sums, err := fs.ReadFile(FS, "julieta/SHA256SUMS")
	require.NoError(t, err)
	got := map[string]string{} // file -> sha256
	for line := range strings.SplitSeq(strings.TrimSpace(string(sums)), "\n") {
		sum, file, ok := strings.Cut(line, "  ")
		require.True(t, ok, "the line %q is not '<sha256>  <file>'", line)
		got[file] = sum
	}
	want := map[string]string{
		"mise-x86_64":   p.Mise.SHA256["linux-amd64"],
		"mise-aarch64":  p.Mise.SHA256["linux-arm64"],
		"herdr-x86_64":  p.Herdr.SHA256["linux-amd64"],
		"herdr-aarch64": p.Herdr.SHA256["linux-arm64"],
	}
	for file, sum := range want {
		require.Len(t, sum, 64, "the pin file has no sha256 for %s", file)
	}
	assert.Equal(t, want, got)
}
