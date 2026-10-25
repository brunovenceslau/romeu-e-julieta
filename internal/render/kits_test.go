// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/shquote"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/kits"
)

const (
	plainDescriptor = "schemaVersion: \"3\"\nkind: mixin\ndisplayName: plain\ndescription: a fixture\n"
	binDescriptor   = plainDescriptor + "args:\n  julietaBin:\n    default: \"\"\n    description: the path of julieta\n"
)

// fixtureKits is the fixture the seam is filled with: two product kits.
func fixtureKits() fstest.MapFS {
	return fstest.MapFS{
		"plain/plain.yaml":      {Data: []byte(plainDescriptor), Mode: 0o444},
		"plain/files/a.txt":     {Data: []byte("a"), Mode: 0o444},
		"plain/files/run.sh":    {Data: []byte("#!/bin/sh\n"), Mode: 0o555},
		"withbin/withbin.yaml":  {Data: []byte(binDescriptor), Mode: 0o444},
		"stray/ignored.txt":     {Data: []byte("x")},
		"withbin/notes/b.txt":   {Data: []byte("b"), Mode: 0o444},
		"withbin/notes/c/d.txt": {Data: []byte("d"), Mode: 0o444},
	}
}

func productSource(t *testing.T, fsys fs.FS, envDir string) render.KitMaterializer {
	t.Helper()
	return render.KitMaterializer{FS: fsys, EnvDir: envDir, GOARCH: "arm64"}
}

func productSpec(names ...string) spec.Project {
	var p spec.Project
	for _, n := range names {
		p.Kits = append(p.Kits, spec.Kit{Product: n})
	}
	return p
}

func TestProductKitsMaterializeIntoTheCandidate(t *testing.T) {
	t.Parallel()
	cand := render.Candidate{Dir: t.TempDir()}
	src := productSource(t, fixtureKits(), "/r/shop-env")
	got, err := src.Materialize(t.Context(), cand, productSpec("withbin", "plain"))
	require.NoError(t, err)
	require.Len(t, got, 2, "in the spec's order")
	assert.Equal(t, "withbin", got[0].ID)
	assert.Equal(t, "plain", got[1].ID)

	plain := got[1]
	want, err := gate.KitTreeDigest([]gate.File{
		{Path: "plain.yaml", Mode: gate.ModeRegular, Data: []byte(plainDescriptor)},
		{Path: "files/a.txt", Mode: gate.ModeRegular, Data: []byte("a")},
		{Path: "files/run.sh", Mode: gate.ModeExecutable, Data: []byte("#!/bin/sh\n")},
	})
	require.NoError(t, err)
	assert.Equal(t, want, plain.Digest, "the kit-tree digest of 03 3.12")

	dir := filepath.Join(cand.Dir, render.CandidateKitsDir, plain.Dir())
	for name, c := range map[string]string{"plain.yaml": plainDescriptor, "files/a.txt": "a", "files/run.sh": "#!/bin/sh\n"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		assert.Equal(t, c, string(b), name)
	}
	fi, err := os.Stat(filepath.Join(dir, "files", "run.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
	fi, err = os.Stat(filepath.Join(dir, "files", "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	_, err = os.Stat(filepath.Join(cand.Dir, render.CandidateKitsDir, "stray"))
	require.ErrorIs(t, err, fs.ErrNotExist, "a dir the spec does not name is not written")

	k, err := oci.ParseDescriptor([]byte(plainDescriptor))
	require.NoError(t, err)
	assert.Equal(t, k.Capabilities, plain.Capabilities)
}

func TestProductKitsRenderJulietaBinWhereDeclared(t *testing.T) {
	t.Parallel()
	got, err := productSource(t, fixtureKits(), "/r/shop-env").
		Materialize(t.Context(), render.Candidate{Dir: t.TempDir()}, productSpec("withbin", "plain"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"julietaBin": "/r/shop-env/.romeu/bin/julieta-linux-arm64"}, got[0].Rendered)
	assert.Empty(t, got[1].Rendered, "a kit that does not declare the arg gets none")
}

func TestProductKitsRenderACleanPathBare(t *testing.T) {
	t.Parallel()
	got, err := productSource(t, kits.FS, "/r/shop-env").
		Materialize(t.Context(), render.Candidate{Dir: t.TempDir()}, productSpec("julieta-claude"))
	require.NoError(t, err)
	assert.Equal(t, "/r/shop-env/.romeu/bin/julieta-linux-arm64", got[0].Rendered["julietaBin"])
}

// TestProductKitsRefuseAPathTheArgPatternRefuses: shquote's safe set is
// wider than the pattern of the julieta-claude arg, and a space is
// quoted, so the check names the cause at sync and not at install.
func TestProductKitsRefuseAPathTheArgPatternRefuses(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"/r/my shop-env", "/r/shop+env", "/r/shop@env"} {
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			cand := render.Candidate{Dir: t.TempDir()}
			_, err := productSource(t, kits.FS, env).
				Materialize(t.Context(), cand, productSpec("julieta-claude"))
			require.Error(t, err)
			for _, want := range []string{`"julieta-claude"`, `"julietaBin"`, "does not match its pattern", `^(/[A-Za-z0-9._-]+)*$`, "shop"} {
				assert.Contains(t, err.Error(), want)
			}
			assertNothingWritten(t, cand)
		})
	}
}

func TestProductKitsRefuseANULInTheEnvDir(t *testing.T) {
	t.Parallel()
	cand := render.Candidate{Dir: t.TempDir()}
	_, err := productSource(t, fixtureKits(), "/r/a\x00b").
		Materialize(t.Context(), cand, productSpec("withbin"))
	require.ErrorIs(t, err, shquote.ErrNUL)
	assertNothingWritten(t, cand)
}

// assertNothingWritten holds that the candidate dir is still empty.
func assertNothingWritten(t *testing.T, c render.Candidate) {
	t.Helper()
	entries, err := os.ReadDir(c.Dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestProductKitsRefuseWhatTheyCannotWrite(t *testing.T) {
	t.Parallel()
	link := fixtureKits()
	link["plain/link"] = &fstest.MapFile{Data: []byte("/etc/passwd"), Mode: fs.ModeSymlink | 0o777}
	noDescriptor := fstest.MapFS{"plain/a.txt": {Data: []byte("a")}}
	badDescriptor := fstest.MapFS{"plain/plain.yaml": {Data: []byte("schemaVersion: \"3\"\nunknownKey: 1\n")}}
	for name, tc := range map[string]struct {
		fsys fs.FS
		p    spec.Project
		want string
	}{
		"absent kit":     {fixtureKits(), productSpec("nope"), "nope"},
		"not one name":   {fixtureKits(), productSpec("../plain"), "../plain"},
		"symlink":        {link, productSpec("plain"), "link"},
		"no descriptor":  {noDescriptor, productSpec("plain"), "plain.yaml"},
		"bad descriptor": {badDescriptor, productSpec("plain"), "plain.yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cand := render.Candidate{Dir: t.TempDir()}
			_, err := productSource(t, tc.fsys, "/r/shop-env").Materialize(t.Context(), cand, tc.p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestProductKitsRelativeEnvDirIsRefused(t *testing.T) {
	t.Parallel()
	cand := render.Candidate{Dir: t.TempDir()}
	_, err := productSource(t, fixtureKits(), "shop-env").
		Materialize(t.Context(), cand, productSpec("plain"))
	require.ErrorContains(t, err, `the env dir "shop-env" is not an absolute clean path`)
	assertNothingWritten(t, cand)
}

// TestProductKitsFromTheEmbed is the embed and materialization row of
// T077: every kit of kits.FS, the one embed.FS, materializes, and the
// kit that declares julietaBin gets it.
func TestProductKitsFromTheEmbed(t *testing.T) {
	t.Parallel()
	got, err := productSource(t, kits.FS, "/r/shop-env").
		Materialize(t.Context(), render.Candidate{Dir: t.TempDir()}, productSpec("julieta", "julieta-claude", "os-base", "git-ssh-sign"))
	require.NoError(t, err)
	require.Len(t, got, 4)
	for _, k := range got {
		assert.Equal(t, "mixin", k.Capabilities.Kind, k.ID)
		_, declares := k.Capabilities.Args["julietaBin"]
		if declares {
			assert.Equal(t, "/r/shop-env/.romeu/bin/julieta-linux-arm64", k.Rendered["julietaBin"], k.ID)
		} else {
			assert.Empty(t, k.Rendered, k.ID)
		}
	}
	assert.Contains(t, got[1].Capabilities.Args, "julietaBin")
}

// TestProductKitsFillTheRender runs Render with the product seam: the
// env file names each kit dir and carries the rendered arg.
func TestProductKitsFillTheRender(t *testing.T) {
	t.Parallel()
	in := shopInput(t).Input
	in.Project.Kits = []spec.Kit{{Product: "julieta-claude"}}
	cand := render.Candidate{Dir: t.TempDir()}
	r, err := render.Render(t.Context(), cand, in, productSource(t, kits.FS, "/r/shop-env"))
	require.NoError(t, err)
	assert.Contains(t, string(r.Env), "julieta-claude-"+r.Kits[0].Digest.String()[:12])
	assert.Contains(t, string(r.Env), "/r/shop-env/.romeu/bin/julieta-linux-arm64")
}
