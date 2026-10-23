// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

var update = flag.Bool("update", false, "rewrite the golden files of testdata")

// golden compares got with testdata/name, or rewrites it with -update
// (10 10.4: a golden changes only with -update, locally).
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", filepath.FromSlash(name))
	if *update {
		require.NoError(t, os.WriteFile(p, got, 0o644))
	}
	want, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

// sum returns a kit-tree digest of 32 bytes of b, as a test fixture.
func sum(t *testing.T, b byte) canon.Sum[canon.KitTreeKind] {
	t.Helper()
	var s canon.Sum[canon.KitTreeKind]
	require.NoError(t, s.UnmarshalText([]byte(strings.Repeat(string("0123456789abcdef"[b%16]), 64))))
	return s
}

// project decodes and validates a project spec, as sync reads one.
func project(t *testing.T, yaml string) spec.Project {
	t.Helper()
	p, err := spec.DecodeProject([]byte(yaml))
	require.NoError(t, err)
	require.NoError(t, p.Validate())
	return p
}

// host decodes and validates host settings.
func host(t *testing.T, yaml string) spec.HostSettings {
	t.Helper()
	h, err := spec.DecodeHostSettings([]byte(yaml))
	require.NoError(t, err)
	require.NoError(t, h.Validate())
	return h
}

// shopSpec is the project of 03 3.2 with every kind of kit of 03 3.3.
const shopSpec = `schema: project/v1
name: shop
repos:
  - url: https://github.com/example/shop-api
    primary: true
    ref: main
  - url: https://github.com/example/shop-web
workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5
agent: sbx-kit-claude
kits:
  - product: julieta
  - product: julieta-claude
  - product: os-base
    args: {tz: Etc/UTC}
  - product: git-ssh-sign
    args: {signingKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyOnly"}
  - personal: my-home
sandboxOptions:
  cpus: 4
  memory: 8g
secrets: [github]
ports: []
`

// hostSettings is the example of 03 3.4 with its paths filled in.
const hostSettings = `schema: host-settings/v1
root: /home/agent/dev
config:
  project: my-config
  url: https://github.com/example/my-config
  ref: main
tools:
  sbx: /usr/local/bin/sbx
  git: /usr/bin/git
gitHosts:
  - host: github.com
workloadRepositories:
  - docker.io/docker/sbx-kit-claude
secrets:
  github@my-config:
    service: github
    argv: [/usr/bin/security, find-generic-password, -s, romeu/my-config/github, -w]
  github@shop:
    service: github
    argv: [/usr/bin/security, find-generic-password, -s, romeu/shop/github, -w]
  npm@shop:
    service: npm
    argv: [/opt/homebrew/bin/op, read, "op://dev/npm/token"]
`

// shopKits are the kits of shopSpec as the Materializer would write
// them: julieta-claude takes julietaBin, which romeu renders (06 6.4).
func shopKits(t *testing.T) []render.MaterializedKit {
	t.Helper()
	return []render.MaterializedKit{
		{ID: "julieta", Digest: sum(t, 1)},
		{ID: "julieta-claude", Digest: sum(t, 2), Rendered: map[string]string{"julietaBin": "/home/agent/dev/shop-env/.romeu/bin"}},
		{ID: "os-base", Digest: sum(t, 3)},
		{ID: "git-ssh-sign", Digest: sum(t, 4)},
		{ID: "my-home", Digest: sum(t, 5)},
	}
}

// fakeMaterializer returns its kits and writes nothing.
type fakeMaterializer struct {
	kits []render.MaterializedKit
	err  error
}

func (f fakeMaterializer) Materialize(context.Context, render.Candidate, spec.Project) ([]render.MaterializedKit, error) {
	return f.kits, f.err
}

// Render writes the candidate's sbxenv.yaml, which Env returns,
// into the candidate dir, and returns the kits.
func TestRender(t *testing.T) {
	c := render.Candidate{Dir: t.TempDir()}
	in := render.Input{Project: project(t, shopSpec), Host: host(t, hostSettings), Home: "/home/agent"}
	got, err := render.Render(t.Context(), c, in, fakeMaterializer{kits: shopKits(t)})
	require.NoError(t, err)
	written, err := os.ReadFile(filepath.Join(c.Dir, render.CandidateEnvFile))
	require.NoError(t, err)
	assert.Equal(t, got.Env, written)
	assert.Equal(t, shopKits(t), got.Kits)
	info, err := os.Stat(filepath.Join(c.Dir, render.CandidateEnvFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(render.FileMode), info.Mode().Perm())
}

// A materializer that fails stops the render before any file is
// written.
func TestRenderMaterializerFails(t *testing.T) {
	c := render.Candidate{Dir: t.TempDir()}
	in := render.Input{Project: project(t, shopSpec), Host: host(t, hostSettings), Home: "/home/agent"}
	boom := errors.New("boom")
	_, err := render.Render(t.Context(), c, in, fakeMaterializer{err: boom})
	require.ErrorIs(t, err, boom)
	_, err = os.Stat(filepath.Join(c.Dir, render.CandidateEnvFile))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// A render that Env refuses writes nothing, and a candidate dir
// that does not exist is an error.
func TestRenderErrors(t *testing.T) {
	in := render.Input{Project: project(t, shopSpec), Host: host(t, hostSettings), Home: "/home/agent"}
	_, err := render.Render(t.Context(), render.Candidate{Dir: t.TempDir()}, in, fakeMaterializer{})
	require.ErrorContains(t, err, "5 kits and 0 were materialized")
	_, err = render.Render(t.Context(), render.Candidate{Dir: filepath.Join(t.TempDir(), "absent")}, in, fakeMaterializer{kits: shopKits(t)})
	require.Error(t, err)
}

// The kit dir of a materialized kit is <kit>-<digest12>, below
// .romeu/kits in the live tree.
func TestMaterializedKitDir(t *testing.T) {
	k := render.MaterializedKit{ID: "julieta", Digest: sum(t, 10)}
	assert.Equal(t, "julieta-aaaaaaaaaaaa", k.Dir())
	assert.Equal(t, ".romeu/kits/julieta-aaaaaaaaaaaa", k.LivePath())
}

// readFile returns the bytes of name in the candidate dir, and holds
// its mode to FileMode.
func readFile(t *testing.T, c render.Candidate, name string) []byte {
	t.Helper()
	p := filepath.Join(c.Dir, filepath.FromSlash(name))
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(render.FileMode), info.Mode().Perm())
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	return data
}
