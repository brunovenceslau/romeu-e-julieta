// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
)

// readTestdata returns a file of testdata.
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func TestDecodeProject(t *testing.T) {
	t.Run("the record of 03 3.8 round-trips byte for byte", func(t *testing.T) {
		want := readTestdata(t, "project.json")
		p, err := DecodeProject(want)
		require.NoError(t, err)
		assert.Equal(t, "open", string(p.Generations[0].State))
		assert.Equal(t, "shop-api", p.Generations[0].Repos[0].Dir)
		got, err := EncodeProject(p)
		require.NoError(t, err)
		assert.Equal(t, string(want), string(got))
	})
	t.Run("a member it does not know is refused, at any depth (ADR 0041, Consequences)", func(t *testing.T) {
		base := string(readTestdata(t, "project.json"))
		top := strings.Replace(base, `"lastRunAt"`, `"newMember": {"deep": [1, {"x": 2}]}, "lastRunAt"`, 1)
		deep := strings.Replace(base, `"primary": true`, `"primary": true, "added": "v"`, 1)
		for name, doc := range map[string]string{"top level": top, "nested": deep} {
			_, err := DecodeProject([]byte(doc))
			require.ErrorIs(t, err, ErrInvalid, name)
		}
	})
	t.Run("a newer version is format-newer, RJ-311", func(t *testing.T) {
		data := strings.Replace(string(readTestdata(t, "project.json")), "state-project/v1", "state-project/v2", 1)
		data = strings.Replace(data, `"lastRunAt"`, `"newMember": 1, "lastRunAt"`, 1)
		_, err := DecodeProject([]byte(data))
		require.Error(t, err)
		assert.Equal(t, "RJ-311", cli.Of(err).ID)
	})
	invalid := []struct {
		name string
		edit func(string) string
	}{
		{"a duplicate member", func(s string) string {
			return strings.Replace(s, `"adopted": false,`, `"adopted": false, "adopted": true,`, 1)
		}},
		{"the schema of another format", func(s string) string { return strings.Replace(s, "state-project/v1", "state-toolchain/v1", 1) }},
		{"no schema", func(s string) string { return strings.Replace(s, `"schema": "state-project/v1",`, "", 1) }},
		{"a ULID in uppercase", func(s string) string {
			return strings.Replace(s, "01j9zc2q8x7w4t6y3r5e1v9k0p", "01J9ZC2Q8X7W4T6Y3R5E1V9K0P", 1)
		}},
		{"a digest of another length", func(s string) string {
			return strings.Replace(s, "5648016dd24b762bb7cb530d055be770c0b364045b8c63bcb0eede13f025cbad", "43", 1)
		}},
		{"a widening set changed after its digest", func(s string) string {
			return strings.Replace(s, `"domain": "github.com"`, `"domain": "github.co"`, 1)
		}},
		{"an approval whose renderDigest is not its set's", func(s string) string {
			return strings.Replace(s, `"renderDigest": "5555555555555555555555555555555555555555555555555555555555555555",
    "specCommit"`, `"renderDigest": "6666666666666666666666666666666666666666666666666666666666666666",
    "specCommit"`, 1)
		}},
		{"a state that is no generation state", func(s string) string { return strings.Replace(s, `"state": "open"`, `"state": "none"`, 1) }},
		{"a generation with no id", func(s string) string { return strings.Replace(s, `"id": "01j9zc2q8x7w4t6y3r5e1v9k0p",`, "", 1) }},
		{"a time that is not a time", func(s string) string { return strings.Replace(s, "2026-10-02T09:14:07Z", "yesterday", 1) }},
		{"text that is not JSON", func(string) string { return "{" }},
		{"a widening set with an unknown member", func(s string) string {
			return strings.Replace(s, `"project": "shop",`, `"project": "shop", "devices": [],`, 1)
		}},
		{"a widening set with a digest that is no digest", func(s string) string {
			return strings.Replace(s, `"renderDigest": "5555555555555555555555555555555555555555555555555555555555555555"`, `"renderDigest": "55"`, 1)
		}},
		{"trailing data", func(s string) string { return s + "{}" }},
		{"a result no salvage has", func(s string) string { return strings.Replace(s, `"result": "complete"`, `"result": "fine"`, 1) }},
		{"a second open generation", func(s string) string {
			i := strings.Index(s, `    {`+"\n"+`      "id"`)
			j := strings.LastIndex(s, "    }\n  ],")
			one := s[i : j+len("    }")]
			other := strings.Replace(one, "01j9zc2q8x7w4t6y3r5e1v9k0p", "01j9zc2q8x7w4t6y3r5e1v9k0q", 1)
			return s[:j+len("    }")] + ",\n" + other + s[j+len("    }"):]
		}},
	}
	for _, tt := range invalid {
		t.Run(tt.name+" is refused", func(t *testing.T) {
			_, err := DecodeProject([]byte(tt.edit(string(readTestdata(t, "project.json")))))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalid)
		})
	}
	t.Run("the cap is exact: MaxRecordSize bytes read, one more is refused", func(t *testing.T) {
		doc := readTestdata(t, "project.json")
		at := append(slices.Clone(doc), bytes.Repeat([]byte(" "), MaxRecordSize-len(doc))...)
		require.Len(t, at, MaxRecordSize)
		_, err := DecodeProject(at)
		require.NoError(t, err)
		_, err = DecodeProject(append(at, ' '))
		require.ErrorIs(t, err, ErrInvalid)
		assert.Contains(t, err.Error(), "cap")
	})
	t.Run("a document over the cap is refused", func(t *testing.T) {
		_, err := DecodeProject([]byte(`{"schema": "state-project/v1", "x": "` + strings.Repeat("a", MaxRecordSize) + `"}`))
		assert.ErrorIs(t, err, ErrInvalid)
	})
}

// seedProject writes the file of project name in the store.
func seedProject(t *testing.T, s *Store, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(s.Dir(), "projects", name+".json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestStore_UpdateProject(t *testing.T) {
	t.Run("an absent record is a new one, written 0600", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.UpdateProject(held(t, s), "shop", func(p *Project) error {
			assert.Equal(t, "state-project/v1", p.Schema)
			assert.Empty(t, p.Generations)
			p.EgressApplied = []string{"github.com"}
			return nil
		}))
		path := filepath.Join(s.Dir(), "projects", "shop.json")
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		p, err := s.LoadProject("shop")
		require.NoError(t, err)
		assert.Equal(t, []string{"github.com"}, p.EgressApplied)
	})
	t.Run("LoadProject of an absent record is fs.ErrNotExist", func(t *testing.T) {
		_, err := openTemp(t).LoadProject("shop")
		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
	t.Run("a record that does not decode is never written over", func(t *testing.T) {
		for _, bad := range []string{"{", "", `{"schema": "state-project/v1", "generations": "x"}`, `{"schema": "state-project/v1", "schema": "state-project/v1"}`} {
			s := openTemp(t)
			path := seedProject(t, s, "shop", []byte(bad))
			called := false
			err := s.UpdateProject(held(t, s), "shop", func(*Project) error { called = true; return nil })
			require.Error(t, err, bad)
			assert.False(t, called, "fn runs on a record that decoded")
			e := cli.Of(err)
			assert.Equal(t, "RJ-334", e.ID)
			assert.Equal(t, 2, e.Exit)
			assert.Contains(t, e.Details, cli.Detail{Key: "file", Value: path})
			assert.Contains(t, e.Hint(), "romeu salvage --from-host shop", "the project's name is in the hint")
			after, rerr := os.ReadFile(path)
			require.NoError(t, rerr)
			assert.Equal(t, bad, string(after), "byte-identical")
			_, lerr := s.LoadProject("shop")
			assert.Equal(t, "RJ-334", cli.Of(lerr).ID, "a reader reports it too")
		}
	})
	t.Run("a record whose approval is another project's is never written over", func(t *testing.T) {
		s := openTemp(t)
		doc := readTestdata(t, "project.json")
		path := seedProject(t, s, "other", doc)
		_, err := s.LoadProject("other")
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
		called := false
		err = s.UpdateProject(held(t, s), "other", func(*Project) error { called = true; return nil })
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
		assert.False(t, called)
		after, rerr := os.ReadFile(path)
		require.NoError(t, rerr)
		assert.Equal(t, doc, after, "byte-identical")
	})
	t.Run("a fn that records the approval of another project writes nothing", func(t *testing.T) {
		s := openTemp(t)
		err := s.UpdateProject(held(t, s), "shop", func(p *Project) error {
			a := approvalFor("other", canon.ULID{1})
			p.Approval = &a
			return nil
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "the project check", "the message names the check")
		require.NotErrorIs(t, err, ErrInvalid, "a refused write is no unreadable record")
		_, isCLI := errors.AsType[*cli.Error](err)
		assert.False(t, isCLI, "RJ-334 is for the read path only")
		assert.NoFileExists(t, filepath.Join(s.Dir(), "projects", "shop.json"))
	})
	t.Run("a failing fn writes nothing", func(t *testing.T) {
		s := openTemp(t)
		boom := errors.New("boom")
		require.ErrorIs(t, s.UpdateProject(held(t, s), "shop", func(*Project) error { return boom }), boom)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "projects", "shop.json"))
	})
	t.Run("a fn that leaves an invalid record writes nothing", func(t *testing.T) {
		s := openTemp(t)
		err := s.UpdateProject(held(t, s), "shop", func(p *Project) error {
			p.Generations = append(p.Generations, Generation{State: "nonsense"})
			return nil
		})
		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "projects", "shop.json"))
	})
	t.Run("a name that is no project name is refused", func(t *testing.T) {
		s := openTemp(t)
		for _, name := range []string{"", "../x", "Shop", "a/b", ".hidden", "x", strings.Repeat("a", 41)} {
			err := s.UpdateProject(held(t, s), name, func(*Project) error { return nil })
			assert.ErrorIs(t, err, ErrInvalidName, name)
		}
	})
}

func TestStore_UpdateToolchain(t *testing.T) {
	s := openTemp(t)
	at := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	require.NoError(t, s.UpdateToolchain(held(t, s), func(tc *Toolchain) error {
		tc.SbxVersion = "0.46.0"
		tc.Romeu = Binary{Version: "v1.0.0", SHA256: SHA256{1}}
		tc.Julieta = JulietaBinary{Version: "v1.0.0", Protocol: 1, SHA256: SHA256{2}}
		tc.CatalogDigest = canon.Sum[canon.CatalogKind]{3}
		tc.AcknowledgedAt = at
		tc.Catalog = Catalog{"tools": map[string]any{"go": []any{"proxy.golang.org"}}}
		return nil
	}))
	tc, err := s.LoadToolchain()
	require.NoError(t, err)
	assert.Equal(t, "state-toolchain/v1", tc.Schema)
	assert.Equal(t, "0.46.0", tc.SbxVersion)
	assert.True(t, at.Equal(tc.AcknowledgedAt))
	info, err := os.Stat(filepath.Join(s.Dir(), "toolchain.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	t.Run("a record that does not decode is never written over", func(t *testing.T) {
		path := filepath.Join(s.Dir(), "toolchain.json")
		require.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))
		err := s.UpdateToolchain(held(t, s), func(*Toolchain) error { return nil })
		require.Error(t, err)
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "garbage", string(after))
	})
}

func TestStore_Descriptors(t *testing.T) {
	const digest = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	d := Descriptor{Digest: digest, Capabilities: &oci.Capabilities{Ports: []oci.Port{{Container: "80"}}}}
	t.Run("a descriptor is cached by its digest and read back", func(t *testing.T) {
		s := openTemp(t)
		_, err := s.GetDescriptor(digest)
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.NoError(t, s.PutDescriptor(held(t, s), d))
		got, err := s.GetDescriptor(digest)
		require.NoError(t, err)
		assert.Equal(t, "state-descriptor/v1", got.Schema)
		assert.Equal(t, digest, got.Digest)
		info, err := os.Stat(filepath.Join(s.Dir(), "descriptors", strings.Repeat("5", 64)+".json"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
	t.Run("entries are immutable", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.PutDescriptor(held(t, s), d))
		require.NoError(t, s.PutDescriptor(held(t, s), d), "the same content again is a no-op")
		other := Descriptor{Digest: digest, Capabilities: &oci.Capabilities{Ports: []oci.Port{{Container: "81"}}}}
		require.ErrorIs(t, s.PutDescriptor(held(t, s), other), ErrImmutable)
		got, err := s.GetDescriptor(digest)
		require.NoError(t, err)
		assert.Equal(t, d.Capabilities, got.Capabilities)
	})
	t.Run("a bad digest is refused before it is a path", func(t *testing.T) {
		s := openTemp(t)
		for _, bad := range []string{"", "../x", "sha256:zz", "md5:" + strings.Repeat("a", 32), "sha256:" + strings.Repeat("A", 64)} {
			_, err := s.GetDescriptor(bad)
			require.ErrorIs(t, err, ErrInvalidDigest, bad)
			require.ErrorIs(t, s.PutDescriptor(held(t, s), Descriptor{Digest: bad}), ErrInvalidDigest, bad)
		}
	})
	t.Run("a descriptor that does not decode is never written over", func(t *testing.T) {
		s := openTemp(t)
		path := filepath.Join(s.Dir(), "descriptors", strings.Repeat("5", 64)+".json")
		require.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))
		err := s.PutDescriptor(held(t, s), d)
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
		after, rerr := os.ReadFile(path)
		require.NoError(t, rerr)
		assert.Equal(t, "garbage", string(after))
	})
}

func TestSHA256_UnmarshalText(t *testing.T) {
	var s SHA256
	require.NoError(t, s.UnmarshalText([]byte(strings.Repeat("ab", 32))))
	assert.Equal(t, strings.Repeat("ab", 32), s.String())
	for _, bad := range []string{"", "ab", strings.Repeat("zz", 32), strings.Repeat("AB", 32)} {
		before := s
		require.Error(t, s.UnmarshalText([]byte(bad)), bad)
		assert.Equal(t, before, s, "left as it was")
	}
}

func TestEncode_refusesWhatNoReaderWouldTake(t *testing.T) {
	t.Run("a toolchain record with members missing", func(t *testing.T) {
		_, err := EncodeToolchain(&Toolchain{Schema: ToolchainField})
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrInvalid, "a defect of the code, not a record that is unreadable")
		for _, member := range []string{"sbxVersion", "romeu", "julieta", "catalogDigest", "catalog", "acknowledgedAt"} {
			assert.Contains(t, err.Error(), member)
		}
	})
	t.Run("a descriptor with a bad digest and no capabilities", func(t *testing.T) {
		_, err := EncodeDescriptor(&Descriptor{Schema: DescriptorField})
		require.Error(t, err)
	})
	t.Run("a value JSON cannot hold", func(t *testing.T) {
		tc := validToolchain()
		tc.Catalog = Catalog{"x": make(chan int)}
		_, err := EncodeToolchain(tc)
		require.Error(t, err)
	})
	t.Run("an approval that is not whole", func(t *testing.T) {
		p := NewProject()
		p.Approval = &Approval{}
		_, err := EncodeProject(p)
		require.Error(t, err)
	})
	t.Run("an approval whose widening set names no project", func(t *testing.T) {
		p := NewProject()
		a := approvalFor("", canon.ULID{1})
		p.Approval = &a
		_, err := EncodeProject(p)
		require.Error(t, err)
	})
	t.Run("an approval whose digest is not its set's", func(t *testing.T) {
		p := NewProject()
		a := approvalFor("shop", canon.ULID{1})
		a.WideningDigest = canon.Sum[canon.WideningKind]{1}
		p.Approval = &a
		_, err := EncodeProject(p)
		require.Error(t, err)
		require.ErrorContains(t, err, "the widening set check", "the message names the check")
		require.NotErrorIs(t, err, ErrInvalid, "a refused write is no unreadable record")
	})
	t.Run("a retiring with no attic", func(t *testing.T) {
		p := NewProject()
		p.Retiring = &Retiring{}
		_, err := EncodeProject(p)
		require.Error(t, err)
	})
	t.Run("a generation of the same id twice", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationClosedRemoved)
		g := p.Generations[0]
		p.Generations = append(p.Generations, g)
		_, err := EncodeProject(p)
		require.Error(t, err)
	})
	t.Run("a closed generation with no closedAt, and an open one with it", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationClosedRemoved)
		p.Generations[0].ClosedAt = time.Time{}
		_, err := EncodeProject(p)
		require.Error(t, err)
		q, _ := projectIn(t, s, GenerationOpen)
		q.Generations[0].ClosedAt = testNow
		_, err = EncodeProject(q)
		require.Error(t, err)
	})
	t.Run("a generation with no workspace path or repos, a salvage entry with no result", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationOpen)
		p.Generations[0].WorkspacePath = ""
		p.Generations[0].Repos = nil
		p.Generations[0].Salvage = []Salvage{{}}
		_, err := EncodeProject(p)
		require.Error(t, err)
		for _, want := range []string{"workspacePath", "repos", "salvage[0]", "refsPrefix", "result"} {
			assert.Contains(t, err.Error(), want)
		}
	})
}

// approvalFor returns a whole approval of the project whose digests are
// those of its widening set.
func approvalFor(project string, candidate canon.ULID) Approval {
	set := gate.Set{Project: project, RenderDigest: canon.Sum[canon.RenderKind]{2}}
	sum, err := gate.Digest(set)
	if err != nil {
		panic(err)
	}
	return Approval{
		WideningDigest: sum, Set: set, RenderDigest: set.RenderDigest,
		SpecCommit: "0123456789abcdef0123456789abcdef01234567", CandidateID: candidate,
		ApprovedAt: time.Date(2026, 10, 2, 9, 10, 0, 0, time.UTC),
	}
}

// validToolchain returns a toolchain record whole enough to write.
func validToolchain() *Toolchain {
	return &Toolchain{
		Schema: ToolchainField, SbxVersion: "0.46.0",
		Romeu:         Binary{Version: "v1.0.0", SHA256: SHA256{1}},
		Julieta:       JulietaBinary{Version: "v1.0.0", Protocol: 1, SHA256: SHA256{2}},
		CatalogDigest: canon.Sum[canon.CatalogKind]{3}, Catalog: Catalog{},
		AcknowledgedAt: testNow,
	}
}

// TestApprovalSetIsWrittenInItsNormalForm: the set of an approval is
// recorded in the normal form of gate.Normal, its sorted lists sorted,
// whatever order the caller gave.
func TestApprovalSetIsWrittenInItsNormalForm(t *testing.T) {
	p := NewProject()
	a := approvalFor("shop", canon.ULID{1})
	a.Set.Ports = []string{"9090:9090/tcp", "8080:8080/tcp"}
	sum, err := gate.Digest(a.Set)
	require.NoError(t, err)
	a.WideningDigest = sum
	p.Approval = &a
	data, err := EncodeProject(p)
	require.NoError(t, err)
	got, err := DecodeProject(data)
	require.NoError(t, err)
	assert.Equal(t, []string{"8080:8080/tcp", "9090:9090/tcp"}, got.Approval.Set.Ports)
}

func TestDecodeToolchain(t *testing.T) {
	data, err := EncodeToolchain(validToolchain())
	require.NoError(t, err)
	t.Run("round-trips", func(t *testing.T) {
		got, err := DecodeToolchain(data)
		require.NoError(t, err)
		again, err := EncodeToolchain(got)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(again))
	})
	t.Run("refuses a member it does not know", func(t *testing.T) {
		_, err := DecodeToolchain([]byte(strings.Replace(string(data), `"sbxVersion"`, `"added": [1], "sbxVersion"`, 1)))
		require.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("a document with members missing is unreadable", func(t *testing.T) {
		_, err := DecodeToolchain([]byte(`{"schema": "state-toolchain/v1"}`))
		require.ErrorIs(t, err, ErrInvalid)
	})
	t.Run("an older schema is refused, since no earlier version is read", func(t *testing.T) {
		_, err := DecodeToolchain([]byte(`{"schema": "state-toolchain/v0"}`))
		require.ErrorIs(t, err, ErrInvalid)
	})
}

func TestDecodeDescriptor(t *testing.T) {
	_, err0 := DecodeDescriptor([]byte(`{"schema": "state-descriptor/v1", "digest": "sha256:` + strings.Repeat("5", 64) + `", "capabilities": {}, "added": 1}`))
	require.ErrorIs(t, err0, ErrInvalid, "an unknown member")
	_, err := DecodeDescriptor([]byte(`{"schema": "state-descriptor/v1", "digest": "x", "capabilities": {}}`))
	require.ErrorIs(t, err, ErrInvalid)
}

func TestDecodeDescriptor_capabilitiesAreRequired(t *testing.T) {
	head := `{"schema": "state-descriptor/v1", "digest": "sha256:` + strings.Repeat("5", 64) + `"`
	for name, doc := range map[string]string{
		"absent": head + `}`,
		"null":   head + `, "capabilities": null}`,
	} {
		t.Run(name+" is not an empty set", func(t *testing.T) {
			_, err := DecodeDescriptor([]byte(doc))

			require.ErrorIs(t, err, ErrInvalid)
			assert.Contains(t, err.Error(), "capabilities is missing")
		})
	}
	t.Run("an empty set is a set", func(t *testing.T) {
		d, err := DecodeDescriptor([]byte(head + `, "capabilities": {"kind": "workload"}}`))

		require.NoError(t, err)
		require.NotNil(t, d.Capabilities)
	})
	t.Run("a record that would not read is not written", func(t *testing.T) {
		s := openTemp(t)

		err := s.PutDescriptor(held(t, s), Descriptor{Digest: "sha256:" + strings.Repeat("4", 64)})

		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "descriptors", strings.Repeat("4", 64)+".json"))
	})
}

func TestDescriptor_keyedByThePlatformManifest(t *testing.T) {
	manifest := "sha256:" + strings.Repeat("a", 64)
	index := "sha256:" + strings.Repeat("b", 64)
	otherIndex := "sha256:" + strings.Repeat("c", 64)
	caps := &oci.Capabilities{Ports: []oci.Port{{Container: "80"}}}
	d := Descriptor{Digest: manifest, Workload: index, Platform: "linux/amd64", Capabilities: caps}

	t.Run("the workload and the platform are stored with the manifest digest", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.PutDescriptor(held(t, s), d))

		got, err := s.GetDescriptor(manifest)

		require.NoError(t, err)
		assert.Equal(t, index, got.Workload)
		assert.Equal(t, "linux/amd64", got.Platform)
	})
	t.Run("another index that lists the same manifest keeps the first", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.PutDescriptor(held(t, s), d))
		again := d
		again.Workload = otherIndex

		require.NoError(t, s.PutDescriptor(held(t, s), again))

		got, err := s.GetDescriptor(manifest)
		require.NoError(t, err)
		assert.Equal(t, index, got.Workload)
	})
	t.Run("another platform for the same manifest is other content", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, s.PutDescriptor(held(t, s), d))
		other := d
		other.Platform = "linux/arm64"

		require.ErrorIs(t, s.PutDescriptor(held(t, s), other), ErrImmutable)
	})
	t.Run("the pair is checked on a read", func(t *testing.T) {
		for name, mutate := range map[string]func(*Descriptor){
			"a workload without a platform":  func(d *Descriptor) { d.Platform = "" },
			"a platform without a workload":  func(d *Descriptor) { d.Workload = "" },
			"a workload that is no digest":   func(d *Descriptor) { d.Workload = "latest" },
			"a platform that is no platform": func(d *Descriptor) { d.Platform = "amd64" },
		} {
			bad := d
			mutate(&bad)

			_, err := EncodeDescriptor(&bad)

			require.Error(t, err, name)
		}
	})
	t.Run("a local kit has neither", func(t *testing.T) {
		local := Descriptor{Digest: manifest, Capabilities: caps}

		data, err := EncodeDescriptor(&local)

		require.NoError(t, err)
		assert.NotContains(t, string(data), "workload")
	})
}

func TestStore_FindDescriptor(t *testing.T) {
	manifest := "sha256:" + strings.Repeat("a", 64)
	index := "sha256:" + strings.Repeat("b", 64)
	otherIndex := "sha256:" + strings.Repeat("c", 64)
	local := "sha256:" + strings.Repeat("d", 64)
	caps := &oci.Capabilities{Ports: []oci.Port{{Container: "80"}}}
	seed := func(t *testing.T) *Store {
		t.Helper()
		s := openTemp(t)
		lk := held(t, s)
		require.NoError(t, s.PutDescriptor(lk, Descriptor{Digest: manifest, Workload: index, Platform: "linux/arm64", Capabilities: caps}))
		require.NoError(t, s.PutDescriptor(lk, Descriptor{Digest: local, Capabilities: caps}))
		return s
	}

	for _, tc := range []struct {
		name               string
		workload, platform string
		found              bool
	}{
		{"the pair of the entry", index, "linux/arm64", true},
		{"another platform", index, "linux/amd64", false},
		{"another index", otherIndex, "linux/arm64", false},
		{"an empty workload", "", "linux/arm64", false},
		{"an empty pair, which a local kit's entry has", "", "", false},
		{"the manifest digest is no workload", manifest, "linux/arm64", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := seed(t).FindDescriptor(tc.workload, tc.platform)

			if !tc.found {
				require.ErrorIs(t, err, fs.ErrNotExist)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, manifest, got.Digest)
			assert.Equal(t, caps, got.Capabilities)
		})
	}

	t.Run("an entry that does not decode is skipped and the match is returned", func(t *testing.T) {
		s := seed(t)
		dir := filepath.Join(s.Dir(), "descriptors")
		// Names sort before and after the match.
		for _, c := range []string{"0", "f"} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, strings.Repeat(c, 64)+".json"), []byte("{not json"), 0o600))
		}
		// An entry that holds another digest than its name says.
		data, err := os.ReadFile(filepath.Join(dir, strings.Repeat("a", 64)+".json"))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, strings.Repeat("1", 64)+".json"), data, 0o600))

		got, err := s.FindDescriptor(index, "linux/arm64")

		require.NoError(t, err)
		assert.Equal(t, manifest, got.Digest)
		_, err = s.GetDescriptor("sha256:" + strings.Repeat("0", 64))
		require.Error(t, err)
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
	})
	t.Run("an empty store has no entry", func(t *testing.T) {
		_, err := openTemp(t).FindDescriptor(index, "linux/arm64")

		require.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestStore_GetDescriptor_otherDigestThanItsName(t *testing.T) {
	s := openTemp(t)
	d := Descriptor{Digest: "sha256:" + strings.Repeat("6", 64), Capabilities: &oci.Capabilities{}}
	require.NoError(t, s.PutDescriptor(held(t, s), d))
	// A file of the name of one digest that holds another is damage.
	data, err := os.ReadFile(filepath.Join(s.Dir(), "descriptors", strings.Repeat("6", 64)+".json"))
	require.NoError(t, err)
	named := strings.Repeat("7", 64)
	require.NoError(t, os.WriteFile(filepath.Join(s.Dir(), "descriptors", named+".json"), data, 0o600))
	_, err = s.GetDescriptor("sha256:" + named)
	require.Error(t, err)
	assert.Equal(t, "RJ-334", cli.Of(err).ID)
}

func TestStore_readErrors(t *testing.T) {
	t.Run("a record that is a directory is an error that is not an absent record", func(t *testing.T) {
		s := openTemp(t)
		require.NoError(t, os.Mkdir(filepath.Join(s.Dir(), "projects", "shop.json"), 0o700))
		_, err := s.LoadProject("shop")
		require.Error(t, err)
		require.NotErrorIs(t, err, fs.ErrNotExist)
		require.Error(t, s.UpdateProject(held(t, s), "shop", func(*Project) error { return nil }))
		require.NoError(t, os.Mkdir(filepath.Join(s.Dir(), "descriptors", strings.Repeat("8", 64)+".json"), 0o700))
		require.Error(t, s.PutDescriptor(held(t, s), Descriptor{Digest: "sha256:" + strings.Repeat("8", 64), Capabilities: &oci.Capabilities{}}))
	})
	t.Run("a name that is no project name is refused by the reader too", func(t *testing.T) {
		_, err := openTemp(t).LoadProject("../x")
		require.ErrorIs(t, err, ErrInvalidName)
	})
	t.Run("a failing fn of the toolchain, and an absent toolchain", func(t *testing.T) {
		s := openTemp(t)
		_, err := s.LoadToolchain()
		require.ErrorIs(t, err, fs.ErrNotExist)
		boom := errors.New("boom")
		require.ErrorIs(t, s.UpdateToolchain(held(t, s), func(*Toolchain) error { return boom }), boom)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "toolchain.json"))
		require.Error(t, s.UpdateToolchain(held(t, s), func(*Toolchain) error { return nil }), "a record with members missing is not written")
		assert.NoFileExists(t, filepath.Join(s.Dir(), "toolchain.json"))
	})
}

func TestEncode_timestampsAreUTCWholeSeconds(t *testing.T) {
	// 2026-10-02T06:14:07.987654321-03:00 is 09:14:07Z and a fraction
	// (REQ-0162): every timestamp a record holds is written in UTC at
	// whole seconds, whoever set it.
	zone := time.FixedZone("minus3", -3*3600)
	at := time.Date(2026, 10, 2, 6, 14, 7, 987654321, zone)
	const want = "2026-10-02T09:14:07Z"
	t.Run("toolchain", func(t *testing.T) {
		tc := validToolchain()
		tc.AcknowledgedAt = at
		data, err := EncodeToolchain(tc)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"acknowledgedAt": "`+want+`"`)
	})
	t.Run("project", func(t *testing.T) {
		s := testSeams()
		p, id := projectIn(t, s, GenerationClosedRemoved)
		a := approvalFor("shop", id)
		a.ApprovedAt = at
		p.Approval = &a
		p.LastRunAt = at
		p.Generations[0].CreatedAt = at
		p.Generations[0].ClosedAt = at
		p.Generations[0].Salvage[0].CreatedAt = at
		data, err := EncodeProject(p)
		require.NoError(t, err)
		assert.Equal(t, 5, strings.Count(string(data), want), "approvedAt, lastRunAt, createdAt, closedAt and the salvage createdAt")
		assert.NotContains(t, string(data), "987654321")
		assert.NotContains(t, string(data), "-03:00")
	})
	t.Run("a clock in another zone with a fraction stamps the same way", func(t *testing.T) {
		s := Seams{Now: func() time.Time { return at }, IDs: testSeams().IDs}
		p := NewProject()
		_, err := p.Apply(s, GenCreate, testInput())
		require.NoError(t, err)
		g := p.Generations[0]
		assert.True(t, g.CreatedAt.Equal(time.Date(2026, 10, 2, 9, 14, 7, 0, time.UTC)), "whole seconds")
		assert.Equal(t, time.UTC, g.CreatedAt.Location())
	})
}

func TestStore_nonRegularRecord(t *testing.T) {
	s := openTemp(t)
	fifo := filepath.Join(s.Dir(), "projects", "shop.json")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	done := make(chan error, 1)
	go func() {
		_, err := s.LoadProject("shop")
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Equal(t, "RJ-334", cli.Of(err).ID, "a record that is no regular file is unreadable, and does not hang the reader")
	case <-time.After(10 * time.Second):
		t.Fatal("reading a FIFO as a record hangs")
	}
}

func TestStore_PutDescriptor_comparesValues(t *testing.T) {
	s := openTemp(t)
	d := Descriptor{Digest: "sha256:" + strings.Repeat("9", 64), Capabilities: &oci.Capabilities{Ports: []oci.Port{{Container: "1"}, {Container: "2"}}}}
	require.NoError(t, s.PutDescriptor(held(t, s), d))
	require.NoError(t, s.PutDescriptor(held(t, s), d), "the same content again writes nothing")
	changed := Descriptor{Digest: d.Digest, Capabilities: &oci.Capabilities{Ports: []oci.Port{{Container: "1"}, {Container: "3"}}}}
	require.ErrorIs(t, s.PutDescriptor(held(t, s), changed), ErrImmutable)
}

func TestStore_Descriptors_holdTheNormalizedSetOfTheSittingKit(t *testing.T) {
	empty := ""
	tests := []struct {
		name string
		edit func(c *oci.Capabilities)
	}{
		{"a credential file with a structure", func(c *oci.Capabilities) {
			c.Credentials = []oci.Credential{{
				Service: "svc", Phases: []string{"runtime"},
				OAuth: &oci.OAuth{CredentialFile: &oci.CredentialFile{Path: "/a.json", Structure: map[string]any{"a": 1.0, "b": map[string]any{"c": "d"}}}},
			}}
		}},
		// A member present but empty is not a member absent: each reads
		// back as it was written.
		{"an empty default and pattern", func(c *oci.Capabilities) {
			c.Args = map[string]oci.Arg{"host": {Default: &empty, Pattern: &empty}}
		}},
		{"an empty oauth", func(c *oci.Capabilities) {
			c.Credentials = []oci.Credential{{Service: "svc", Phases: []string{"runtime"}, OAuth: &oci.OAuth{}}}
		}},
		{"an empty structure", func(c *oci.Capabilities) {
			c.Credentials = []oci.Credential{{
				Service: "svc", Phases: []string{"runtime"},
				OAuth: &oci.OAuth{CredentialFile: &oci.CredentialFile{Path: "/a.json", Structure: map[string]any{}}},
			}}
		}},
		{"an empty network", func(c *oci.Capabilities) { c.Network = &oci.Network{} }},
		{"an empty resources", func(c *oci.Capabilities) { c.Resources = &oci.Resources{} }},
	}
	data, err := os.ReadFile("../../kits/git-ssh-sign/git-ssh-sign.yaml")
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kit, err := oci.ParseDescriptor(data)
			require.NoError(t, err)
			tt.edit(&kit.Capabilities)
			d := Descriptor{Digest: "sha256:" + strings.Repeat("7", 64), Capabilities: &kit.Capabilities}
			s := openTemp(t)

			require.NoError(t, s.PutDescriptor(held(t, s), d))
			require.NoError(t, s.PutDescriptor(held(t, s), d), "the same set again writes nothing")
			got, err := s.GetDescriptor(d.Digest)

			require.NoError(t, err)
			assert.Equal(t, d.Capabilities, got.Capabilities, "the set reads back as it was written")
		})
	}
}
