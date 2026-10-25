// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/kits"
)

// fakeGitSource implements render.GitSource for unit testing.
type fakeGitSource struct {
	fsckErr    error
	walkErr    error
	entries    []gitsafe.TreeEntry
	files      map[string][]byte
	commitFile func(ctx context.Context, dir, commit, name string, limit int) ([]byte, bool, error)
}

func (f *fakeGitSource) Fsck(context.Context, string, string) error {
	return f.fsckErr
}

func (f *fakeGitSource) WalkTree(context.Context, string, string) ([]gitsafe.TreeEntry, error) {
	if f.walkErr != nil {
		return nil, f.walkErr
	}
	return f.entries, nil
}

func (f *fakeGitSource) CommitFile(ctx context.Context, dir, commit, name string, limit int) ([]byte, bool, error) {
	if f.commitFile != nil {
		return f.commitFile(ctx, dir, commit, name, limit)
	}
	data, ok := f.files[name]
	if !ok {
		return nil, false, nil
	}
	return data, true, nil
}

const frontendHeader = "# syntax=docker/sandbox-kit:3.0.0-m.8@sha256:e6a397771e865625047cf84256a914c7974b3e749d858259b5bc9c653ddb6f0d\n"

// minimalKitDescriptor returns a minimal valid v3 kit descriptor with the given capabilities block.
func minimalKitDescriptor(name, capsBlock string) string {
	return frontendHeader +
		"schemaVersion: \"3\"\n" +
		"kind: mixin\n" +
		"displayName: " + name + "\n" +
		"description: " + name + " kit\n" +
		capsBlock
}

func TestMaterializeProductKitsFromEmbedSeam(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{
			{Product: "julieta"},
			{Product: "os-base"},
			{Product: "git-ssh-sign"},
		},
	}

	m := render.NewMaterializer(render.KitMaterializerConfig{
		Root: "/home/agent/dev",
	})

	mats, err := m.Materialize(t.Context(), c, p)
	require.NoError(t, err)
	require.Len(t, mats, 3)

	assert.Equal(t, "julieta", mats[0].ID)
	assert.Equal(t, "os-base", mats[1].ID)
	assert.Equal(t, "git-ssh-sign", mats[2].ID)

	for _, mat := range mats {
		kitDir := filepath.Join(c.Dir, "kits", mat.Dir())
		info, err := os.Stat(kitDir)
		require.NoError(t, err, "kit dir %s must exist in candidate", kitDir)
		assert.True(t, info.IsDir())

		descFile := filepath.Join(kitDir, mat.ID+".yaml")
		_, err = os.Stat(descFile)
		require.NoError(t, err, "descriptor %s must exist in candidate", descFile)
	}

	// Julieta has SHA256SUMS staged beside its descriptor.
	julietaSums := filepath.Join(c.Dir, "kits", mats[0].Dir(), "SHA256SUMS")
	_, err = os.Stat(julietaSums)
	require.NoError(t, err)
}

func TestMaterializeProductKitWithInjectedFS(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	fixtureFS := fstest.MapFS{
		"my-kit/my-kit.yaml": {Data: []byte(minimalKitDescriptor("my-kit", "args:\n  julietaBin:\n    description: path to julieta\n"))},
		"my-kit/setup.sh":    {Data: []byte("#!/bin/sh\necho ok\n"), Mode: 0o755},
		"my-kit/testdata/x":  {Data: []byte("ignore me")},
	}

	p := spec.Project{
		Name: "my-proj",
		Kits: []spec.Kit{
			{Product: "my-kit"},
		},
	}

	m := render.NewMaterializer(render.KitMaterializerConfig{
		ProductFS: fixtureFS,
		Root:      "/home/agent/dev",
	})

	mats, err := m.Materialize(t.Context(), c, p)
	require.NoError(t, err)
	require.Len(t, mats, 1)

	assert.Equal(t, "my-kit", mats[0].ID)
	assert.Equal(t, "/home/agent/dev/my-proj-env/.romeu/bin", mats[0].Rendered["julietaBin"])

	kitDir := filepath.Join(c.Dir, "kits", mats[0].Dir())
	setupPath := filepath.Join(kitDir, "setup.sh")
	setupInfo, err := os.Stat(setupPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), setupInfo.Mode().Perm())

	testdataPath := filepath.Join(kitDir, "testdata")
	_, err = os.Stat(testdataPath)
	assert.ErrorIs(t, err, os.ErrNotExist, "testdata/ must not be materialized")
}

func TestMaterializePersonalKitSuccess(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	kitDesc := minimalKitDescriptor("custom-tool", "capabilities:\n  - type: com.docker.sandbox/lifecycle@1\n    config:\n      install: [{command: \"echo 1\"}]\n")
	git := &fakeGitSource{
		entries: []gitsafe.TreeEntry{
			{Path: "kits/custom-tool/custom-tool.yaml", Mode: gate.ModeRegular, OID: "1111111111111111111111111111111111111111"},
			{Path: "kits/custom-tool/tool.sh", Mode: gate.ModeExecutable, OID: "2222222222222222222222222222222222222222"},
		},
		files: map[string][]byte{
			"kits/custom-tool/custom-tool.yaml": []byte(kitDesc),
			"kits/custom-tool/tool.sh":          []byte("#!/bin/sh\necho tool\n"),
		},
	}

	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{
			{Personal: "custom-tool"},
		},
	}

	m := render.NewMaterializer(render.KitMaterializerConfig{
		Git:          git,
		ConfigDir:    "/var/git/config",
		ConfigCommit: "3333333333333333333333333333333333333333",
		Root:         "/home/agent/dev",
	})

	mats, err := m.Materialize(t.Context(), c, p)
	require.NoError(t, err)
	require.Len(t, mats, 1)

	assert.Equal(t, "custom-tool", mats[0].ID)
	require.NotNil(t, mats[0].Capabilities.Lifecycle)
	assert.Len(t, mats[0].Capabilities.Lifecycle.Install, 1)

	kitDir := filepath.Join(c.Dir, "kits", mats[0].Dir())
	descPath := filepath.Join(kitDir, "custom-tool.yaml")
	_, err = os.Stat(descPath)
	require.NoError(t, err)

	toolPath := filepath.Join(kitDir, "tool.sh")
	toolInfo, err := os.Stat(toolPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), toolInfo.Mode().Perm())
}

func TestMaterializePersonalKitRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		capsBlock string
		errSubstr string
	}{
		{
			name:      "network capability refused",
			capsBlock: "capabilities:\n  - type: com.docker.sandbox/network-policy@1\n    config:\n      runtime: {allow: [evil.example]}\n",
			errSubstr: "declares network capability, which is refused (REQ-0574)",
		},
		{
			name:      "volume capability refused",
			capsBlock: "capabilities:\n  - type: com.docker.sandbox/volume@1\n    config:\n      path: /host-data\n",
			errSubstr: "declares volume/mount capability, which is refused (REQ-0574)",
		},
		{
			name:      "host-mount capability refused",
			capsBlock: "capabilities:\n  - type: com.docker.sandbox/host-mount@1\n    config:\n      path: /shared\n",
			errSubstr: "declares volume/mount capability, which is refused (REQ-0574)",
		},
		{
			name:      "credential capability refused",
			capsBlock: "capabilities:\n  - type: com.docker.sandbox/credential@1\n    config:\n      service: github\n      phase: runtime\n      apiKey: {name: TOKEN, proxyManaged: true}\n",
			errSubstr: "declares credential capability, which is refused (REQ-0574)",
		},
		{
			name:      "ssh-agent capability refused",
			capsBlock: "capabilities:\n  - type: com.docker.sandbox/ssh-agent@1\n    config:\n      unrestricted: false\n      sign: [git]\n",
			errSubstr: "declares ssh-agent capability, which is refused (REQ-0574)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := render.Candidate{Dir: t.TempDir()}
			desc := minimalKitDescriptor("bad-kit", tt.capsBlock)
			git := &fakeGitSource{
				entries: []gitsafe.TreeEntry{
					{Path: "kits/bad-kit/bad-kit.yaml", Mode: gate.ModeRegular, OID: "1111111111111111111111111111111111111111"},
				},
				files: map[string][]byte{
					"kits/bad-kit/bad-kit.yaml": []byte(desc),
				},
			}

			p := spec.Project{
				Name: "shop",
				Kits: []spec.Kit{
					{Personal: "bad-kit"},
				},
			}

			m := render.NewMaterializer(render.KitMaterializerConfig{
				Git:          git,
				ConfigDir:    "/var/git/config",
				ConfigCommit: "3333333333333333333333333333333333333333",
				Root:         "/home/agent/dev",
			})

			_, err := m.Materialize(t.Context(), c, p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)

			// Asserts nothing was written.
			entries, err := os.ReadDir(c.Dir)
			require.NoError(t, err)
			assert.Empty(t, entries)
		})
	}
}

func TestHostileTreeStopsBeforeWriting(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	walkErr := errors.New("gitsafe: hostile tree: case collision")
	git := &fakeGitSource{
		walkErr: walkErr,
	}

	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{
			{Personal: "my-kit"},
		},
	}

	m := render.NewMaterializer(render.KitMaterializerConfig{
		Git:          git,
		ConfigDir:    "/var/git/config",
		ConfigCommit: "3333333333333333333333333333333333333333",
		Root:         "/home/agent/dev",
	})

	_, err := m.Materialize(t.Context(), c, p)
	require.ErrorIs(t, err, walkErr)

	// Invariant I27: no file written to candidate when tree is rejected.
	entries, err := os.ReadDir(c.Dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFsckFailureStopsBeforeWriting(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	fsckErr := errors.New("git fsck: corrupt object")
	git := &fakeGitSource{
		fsckErr: fsckErr,
	}

	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{
			{Personal: "my-kit"},
		},
	}

	m := render.NewMaterializer(render.KitMaterializerConfig{
		Git:          git,
		ConfigDir:    "/var/git/config",
		ConfigCommit: "3333333333333333333333333333333333333333",
		Root:         "/home/agent/dev",
	})

	_, err := m.Materialize(t.Context(), c, p)
	require.ErrorIs(t, err, fsckErr)

	entries, err := os.ReadDir(c.Dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestConformanceFixturesParseToRecordedCapabilities(t *testing.T) {
	t.Parallel()
	testdataDir := filepath.Join("testdata", "kits")
	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	foundCount := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		foundCount++
		p := filepath.Join(testdataDir, e.Name())
		data, err := os.ReadFile(p)
		require.NoError(t, err, "reading fixture %s", e.Name())

		// Invariant I28: every fixture is marked source: sitting M2.
		assert.Contains(t, string(data), "source: sitting M2", "%s must have source: sitting M2", e.Name())

		kit, err := oci.ParseDescriptor(data)
		require.NoError(t, err, "parsing fixture %s", e.Name())
		assert.Equal(t, "mixin", kit.Capabilities.Kind, "%s kind must be mixin", e.Name())
	}
	assert.GreaterOrEqual(t, foundCount, 18, "all 18 capability types must have conformance fixtures")
}

func TestMaterializeAssertsT077Row(t *testing.T) {
	t.Parallel()
	// T077 row: "romeu embeds kits/* through kits/embed.go, and sync materializes them"
	// Assert kits.FS contains every product kit and materializer produces valid digests.
	c := render.Candidate{Dir: t.TempDir()}
	top, err := fs.ReadDir(kits.FS, ".")
	require.NoError(t, err)

	var p spec.Project
	p.Name = "embed-test"
	for _, entry := range top {
		if entry.IsDir() {
			p.Kits = append(p.Kits, spec.Kit{Product: entry.Name()})
		}
	}
	require.NotEmpty(t, p.Kits)

	m := render.NewMaterializer(render.KitMaterializerConfig{
		Root: "/home/agent/dev",
	})

	mats, err := m.Materialize(t.Context(), c, p)
	require.NoError(t, err)
	require.Len(t, mats, len(p.Kits))

	for i, mat := range mats {
		assert.Equal(t, p.Kits[i].Product, mat.ID)
		assert.NotEqual(t, canon.Sum[canon.KitTreeKind]{}, mat.Digest, "digest of %s must not be zero", mat.ID)
		assert.DirExists(t, filepath.Join(c.Dir, "kits", mat.Dir()))
	}
}

// ---------------------------------------------------------------------------
// Additional coverage tests
// ---------------------------------------------------------------------------

func TestMaterializeEmptyCandidateDirReturnsError(t *testing.T) {
	t.Parallel()
	m := render.NewMaterializer(render.KitMaterializerConfig{Root: "/r"})
	p := spec.Project{Name: "p", Kits: []spec.Kit{{Product: "julieta"}}}
	_, err := m.Materialize(t.Context(), render.Candidate{Dir: ""}, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "candidate directory is empty")
}

func TestMaterializePersonalKitMissingGitConfig(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	// Personal kit but no Git/ConfigDir/ConfigCommit provided.
	m := render.NewMaterializer(render.KitMaterializerConfig{Root: "/r"})
	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{{Personal: "my-kit"}},
	}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "personal kits require Git, ConfigDir and ConfigCommit")
}

func TestMaterializeKitMissingBothProductAndPersonal(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	m := render.NewMaterializer(render.KitMaterializerConfig{Root: "/r"})
	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{{}}, // neither product nor personal
	}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must specify exactly one of product or personal")
}

func TestMaterializeProductKitWalkError(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	// FS that has no kit named "nonexistent" — WalkDir returns an error.
	m := render.NewMaterializer(render.KitMaterializerConfig{
		ProductFS: fstest.MapFS{},
		Root:      "/r",
	})
	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{{Product: "nonexistent"}},
	}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
}

func TestMaterializeProductKitBadDescriptor(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	fixtureFS := fstest.MapFS{
		"bad-kit/bad-kit.yaml": {Data: []byte("not: valid: kit: descriptor!!!")},
	}
	m := render.NewMaterializer(render.KitMaterializerConfig{
		ProductFS: fixtureFS,
		Root:      "/r",
	})
	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{{Product: "bad-kit"}},
	}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
}

func TestMaterializeProductKitNoDescriptorYaml(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	// Kit has only a shell script — no .yaml descriptor.
	fixtureFS := fstest.MapFS{
		"no-desc-kit/setup.sh": {Data: []byte("#!/bin/sh\n"), Mode: 0o755},
	}
	m := render.NewMaterializer(render.KitMaterializerConfig{
		ProductFS: fixtureFS,
		Root:      "/r",
	})
	p := spec.Project{
		Name: "shop",
		Kits: []spec.Kit{{Product: "no-desc-kit"}},
	}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no descriptor")
}

func TestFindDescriptorFallbackToAnyYaml(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	// Kit is named "my-kit" but descriptor file is "alt-name.yaml", not "my-kit.yaml".
	fixtureFS := fstest.MapFS{
		"my-kit/alt-name.yaml": {Data: []byte(minimalKitDescriptor("my-kit", ""))},
	}
	m := render.NewMaterializer(render.KitMaterializerConfig{
		ProductFS: fixtureFS,
		Root:      "/r",
	})
	p := spec.Project{
		Name: "proj",
		Kits: []spec.Kit{{Product: "my-kit"}},
	}
	mats, err := m.Materialize(t.Context(), c, p)
	require.NoError(t, err)
	require.Len(t, mats, 1)
	assert.Equal(t, "my-kit", mats[0].ID)
}

func TestMaterializePersonalKitCommitFileError(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	boom := errors.New("git: object corrupt")
	git := &fakeGitSource{
		entries: []gitsafe.TreeEntry{
			{Path: "kits/my-kit/my-kit.yaml", Mode: gate.ModeRegular, OID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"},
		},
		commitFile: func(_ context.Context, _, _, _ string, _ int) ([]byte, bool, error) {
			return nil, false, boom
		},
	}
	m := render.NewMaterializer(render.KitMaterializerConfig{
		Git:          git,
		ConfigDir:    "/cfg",
		ConfigCommit: "aaaa",
		Root:         "/r",
	})
	p := spec.Project{Name: "s", Kits: []spec.Kit{{Personal: "my-kit"}}}
	_, err := m.Materialize(t.Context(), c, p)
	require.ErrorIs(t, err, boom)
}

func TestMaterializePersonalKitFileNotFound(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	git := &fakeGitSource{
		entries: []gitsafe.TreeEntry{
			{Path: "kits/my-kit/my-kit.yaml", Mode: gate.ModeRegular, OID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"},
		},
		commitFile: func(_ context.Context, _, _, _ string, _ int) ([]byte, bool, error) {
			return nil, false, nil // found=false, no error
		},
	}
	m := render.NewMaterializer(render.KitMaterializerConfig{
		Git:          git,
		ConfigDir:    "/cfg",
		ConfigCommit: "aaaa",
		Root:         "/r",
	})
	p := spec.Project{Name: "s", Kits: []spec.Kit{{Personal: "my-kit"}}}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in commit")
}

func TestMaterializePersonalKitNoFiles(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	// Walk entries exist but none match the kit's prefix.
	git := &fakeGitSource{
		entries: []gitsafe.TreeEntry{
			{Path: "kits/other-kit/other-kit.yaml", Mode: gate.ModeRegular, OID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1"},
		},
	}
	m := render.NewMaterializer(render.KitMaterializerConfig{
		Git:          git,
		ConfigDir:    "/cfg",
		ConfigCommit: "aaaa",
		Root:         "/r",
	})
	p := spec.Project{Name: "s", Kits: []spec.Kit{{Personal: "my-kit"}}}
	_, err := m.Materialize(t.Context(), c, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no files found")
}

func TestQuoteJulietaBinSpecialCharacters(t *testing.T) {
	t.Parallel()
	c := render.Candidate{Dir: t.TempDir()}
	fixtureFS := fstest.MapFS{
		"my-kit/my-kit.yaml": {Data: []byte(minimalKitDescriptor("my-kit", "args:\n  julietaBin:\n    description: path\n"))},
	}
	p := spec.Project{
		Name: "space proj",
		Kits: []spec.Kit{
			{Product: "my-kit"},
		},
	}

	m := render.NewMaterializer(render.KitMaterializerConfig{
		ProductFS: fixtureFS,
		Root:      "/home/agent/my path",
	})

	mats, err := m.Materialize(t.Context(), c, p)
	require.NoError(t, err)
	require.Len(t, mats, 1)

	// Since path has spaces, it must be quoted through shquote.
	want := "'/home/agent/my path/space proj-env/.romeu/bin'"
	assert.Equal(t, want, mats[0].Rendered["julietaBin"])
}
