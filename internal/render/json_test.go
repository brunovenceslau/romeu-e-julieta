// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// hexSum returns a digest of any kind whose 64 hex digits are all c.
func hexSum[K canon.Kind](t *testing.T, c byte) canon.Sum[K] {
	t.Helper()
	var s canon.Sum[K]
	require.NoError(t, s.UnmarshalText([]byte(strings.Repeat(string(c), 64))))
	return s
}

// shopRecord is the render.json of the shop project, the example of
// 03 3.7 with every digest filled in.
func shopRecord(t *testing.T) render.Record {
	t.Helper()
	eg, err := render.EgressOf(egress.Result{
		Auto:  []string{"dl.google.com", "nodejs.org"},
		Gated: []egress.Gate{{Domain: "github.com", Reason: egress.ReasonUpload}},
	})
	require.NoError(t, err)
	return render.Record{
		RomeuVersion: "v1.0.0",
		SpecSource:   render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/my-config/main", Commit: strings.Repeat("c", 40)},
		Repos: []render.RecordRepo{
			{Dir: "shop-api", URL: "https://github.com/example/shop-api", EgressRef: "refs/romeu/origin/shop-api/main", EgressCommit: strings.Repeat("a", 40), LockSha256: strings.Repeat("e", 64)},
			{Dir: "shop-web", URL: "https://github.com/example/shop-web", EgressRef: "refs/romeu/origin/shop-web/main", EgressCommit: strings.Repeat("b", 40)},
		},
		Files: map[string]string{
			"sbxenv.yaml":                             strings.Repeat("1", 64),
			".romeu/kits/julieta-111111111111":        strings.Repeat("1", 64),
			".romeu/bin/julieta-linux-arm64":          strings.Repeat("2", 64),
			".romeu/bin/SHA256SUMS":                   strings.Repeat("3", 64),
			".romeu/kits/my-home-555555555555":        strings.Repeat("5", 64),
			".romeu/kits/julieta-claude-222222222222": strings.Repeat("2", 64),
		},
		Egress:         eg,
		WideningDigest: hexSum[canon.WideningKind](t, 'd'),
		RecreateDigest: hexSum[canon.RecreateKind](t, 'f'),
		CatalogDigest:  hexSum[canon.CatalogKind](t, '9'),
	}
}

// The goldens of render.json (10 10.1, REQ-0732): a record at rest, and
// one inside a promotion commit (REQ-0226).
func TestEncodeRecordGolden(t *testing.T) {
	got, err := render.EncodeRecord(shopRecord(t))
	require.NoError(t, err)
	golden(t, "env/render.json", got)

	r := shopRecord(t)
	id, err := canon.ParseULID("01m5mre3vz48a3n74a11ataacn")
	require.NoError(t, err)
	r.Promoting = &render.Promoting{
		CandidateID:   id,
		FilesDigest:   hexSum[canon.CandidateFilesKind](t, '7'),
		PreviousFiles: map[string]string{"sbxenv.yaml": strings.Repeat("0", 64)},
	}
	got, err = render.EncodeRecord(r)
	require.NoError(t, err)
	golden(t, "env/render-promoting.json", got)
}

// What EncodeRecord writes DecodeRecord reads back as the same record,
// its schema set.
//
//romeu:invariant I11 test
func TestRecordRoundTrip(t *testing.T) {
	want := shopRecord(t)
	data, err := render.EncodeRecord(want)
	require.NoError(t, err)
	got, err := render.DecodeRecord(data)
	require.NoError(t, err)
	want.Schema = render.RecordSchema
	assert.Equal(t, want, got)
}

// The derived egress of render.json carries the egress digest of the
// sorted union (REQ-0633).
func TestEgressOf(t *testing.T) {
	r := egress.Result{Auto: []string{"b.example"}, Gated: []egress.Gate{{Domain: "a.example", Reason: egress.ReasonKit}}}
	got, err := render.EgressOf(r)
	require.NoError(t, err)
	want, err := r.Digest()
	require.NoError(t, err)
	assert.Equal(t, want, got.EffectiveDigest)
	assert.Equal(t, []render.GatedDomain{{Domain: "a.example", Reason: egress.ReasonKit}}, got.Gated)

	empty, err := render.EgressOf(egress.Result{})
	require.NoError(t, err)
	assert.NotNil(t, empty.Gated, "an empty gated list is written as [], never null")
}

// The records EncodeRecord refuses, each by the field it names.
func TestEncodeRecordErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(r *render.Record)
		want   string
	}{
		{"another spec source kind", func(r *render.Record) { r.SpecSource.Kind = "local" }, `the spec source kind "local"`},
		{"a short spec commit", func(r *render.Record) { r.SpecSource.Commit = "abc" }, "is not an object id"},
		{"an uppercase egress commit", func(r *render.Record) { r.Repos[0].EgressCommit = strings.Repeat("A", 40) }, "the egress commit"},
		{"a repo dir of two elements", func(r *render.Record) { r.Repos[0].Dir = "a/b" }, "is not one path element"},
		{"a lock digest out of form", func(r *render.Record) { r.Repos[0].LockSha256 = "xyz" }, "the mise.lock digest"},
		{"an absolute file path", func(r *render.Record) { r.Files["/etc/passwd"] = strings.Repeat("0", 64) }, `"/etc/passwd" is not a clean relative path`},
		{"a file path that leaves <name>-env/", func(r *render.Record) { r.Files["../x"] = strings.Repeat("0", 64) }, `"../x" is not a clean relative path`},
		{"an unclean file path", func(r *render.Record) { r.Files["a//b"] = strings.Repeat("0", 64) }, `"a//b" is not a clean relative path`},
		{"a control character in a path", func(r *render.Record) { r.Files["a\nb"] = strings.Repeat("0", 64) }, `"a\nb" is not a clean relative path`},
		{"the dir itself", func(r *render.Record) { r.Files["."] = strings.Repeat("0", 64) }, `"." is not a clean relative path`},
		{"a file digest out of form", func(r *render.Record) { r.Files["sbxenv.yaml"] = "0" }, `the digest of "sbxenv.yaml"`},
		{"a previous file out of form", func(r *render.Record) {
			r.Promoting = &render.Promoting{PreviousFiles: map[string]string{"x": "y"}}
		}, `previousFiles: the digest of "x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := shopRecord(t)
			tt.change(&r)
			_, err := render.EncodeRecord(r)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// The documents DecodeRecord refuses.
func TestDecodeRecordErrors(t *testing.T) {
	good, err := render.EncodeRecord(shopRecord(t))
	require.NoError(t, err)
	tests := []struct {
		name string
		data string
	}{
		{"not JSON", "{"},
		{"another schema", strings.Replace(string(good), `"render/v1"`, `"render/v2"`, 1)},
		{"an unknown member", strings.Replace(string(good), `"romeuVersion"`, `"x-note": 1, "romeuVersion"`, 1)},
		{"a duplicate member", strings.Replace(string(good), `"romeuVersion"`, `"schema": "render/v1", "romeuVersion"`, 1)},
		{"trailing data", string(good) + "{}"},
		{"a file path out of form", strings.Replace(string(good), `"sbxenv.yaml"`, `"../sbxenv.yaml"`, 1)},
		{"past the size cap", string(good) + strings.Repeat(" ", render.MaxRecordSize)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := render.DecodeRecord([]byte(tt.data))
			require.ErrorIs(t, err, render.ErrRecordInvalid)
		})
	}
}

// WriteRecord writes render.json into the candidate dir, 0600.
func TestWriteRecord(t *testing.T) {
	c := render.Candidate{Dir: t.TempDir()}
	require.NoError(t, render.WriteRecord(c, shopRecord(t)))
	data := readFile(t, c, render.CandidateRecord)
	want, err := render.EncodeRecord(shopRecord(t))
	require.NoError(t, err)
	assert.Equal(t, want, data)

	r := shopRecord(t)
	r.Files["/x"] = strings.Repeat("0", 64)
	require.Error(t, render.WriteRecord(render.Candidate{Dir: t.TempDir()}, r))
}
