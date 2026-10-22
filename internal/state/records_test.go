// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
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
	t.Run("a member it does not know is skipped, at any depth (ADR 0041)", func(t *testing.T) {
		data := string(readTestdata(t, "project.json"))
		data = strings.Replace(data, `"lastRunAt"`, `"newMember": {"deep": [1, {"x": 2}]}, "lastRunAt"`, 1)
		data = strings.Replace(data, `"primary": true`, `"primary": true, "added": "v"`, 1)
		p, err := DecodeProject([]byte(data))
		require.NoError(t, err)
		assert.True(t, p.Generations[0].Repos[0].Primary)
		assert.Len(t, p.EgressApplied, 2)
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
			return strings.Replace(s, "1111111111111111111111111111111111111111111111111111111111111111", "11", 1)
		}},
		{"a state that is no generation state", func(s string) string { return strings.Replace(s, `"state": "open"`, `"state": "none"`, 1) }},
		{"a generation with no id", func(s string) string { return strings.Replace(s, `"id": "01j9zc2q8x7w4t6y3r5e1v9k0p",`, "", 1) }},
		{"a time that is not a time", func(s string) string { return strings.Replace(s, "2026-10-02T09:14:07Z", "yesterday", 1) }},
		{"text that is not JSON", func(string) string { return "{" }},
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
		require.NoError(t, s.UpdateProject("shop", func(p *Project) error {
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
			err := s.UpdateProject("shop", func(*Project) error { called = true; return nil })
			require.Error(t, err, bad)
			assert.False(t, called, "fn runs on a record that decoded")
			e := cli.Of(err)
			assert.Equal(t, "RJ-334", e.ID)
			assert.Equal(t, 2, e.Exit)
			assert.Contains(t, e.Details, cli.Detail{Key: "file", Value: path})
			after, rerr := os.ReadFile(path)
			require.NoError(t, rerr)
			assert.Equal(t, bad, string(after), "byte-identical")
			_, lerr := s.LoadProject("shop")
			assert.Equal(t, "RJ-334", cli.Of(lerr).ID, "a reader reports it too")
		}
	})
	t.Run("a failing fn writes nothing", func(t *testing.T) {
		s := openTemp(t)
		boom := errors.New("boom")
		require.ErrorIs(t, s.UpdateProject("shop", func(*Project) error { return boom }), boom)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "projects", "shop.json"))
	})
	t.Run("a fn that leaves an invalid record writes nothing", func(t *testing.T) {
		s := openTemp(t)
		err := s.UpdateProject("shop", func(p *Project) error {
			p.Generations = append(p.Generations, Generation{State: "nonsense"})
			return nil
		})
		require.Error(t, err)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "projects", "shop.json"))
	})
	t.Run("a name that is no project name is refused", func(t *testing.T) {
		s := openTemp(t)
		for _, name := range []string{"", "../x", "Shop", "a/b", ".hidden", "x", strings.Repeat("a", 41)} {
			err := s.UpdateProject(name, func(*Project) error { return nil })
			assert.ErrorIs(t, err, ErrInvalidName, name)
		}
	})
}

func TestStore_UpdateToolchain(t *testing.T) {
	s := openTemp(t)
	at := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	require.NoError(t, s.UpdateToolchain(func(tc *Toolchain) error {
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
		require.Error(t, s.UpdateToolchain(func(*Toolchain) error { return nil }))
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "garbage", string(after))
	})
}

func TestStore_Descriptors(t *testing.T) {
	const digest = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	d := Descriptor{Digest: digest, Capabilities: Capabilities{"network": []any{"example.com"}}}
	t.Run("a descriptor is cached by its digest and read back", func(t *testing.T) {
		s := openTemp(t)
		_, err := s.GetDescriptor(digest)
		require.ErrorIs(t, err, fs.ErrNotExist)
		require.NoError(t, s.PutDescriptor(d))
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
		require.NoError(t, s.PutDescriptor(d))
		require.NoError(t, s.PutDescriptor(d), "the same content again is a no-op")
		other := Descriptor{Digest: digest, Capabilities: Capabilities{"network": []any{"other.example"}}}
		require.ErrorIs(t, s.PutDescriptor(other), ErrImmutable)
		got, err := s.GetDescriptor(digest)
		require.NoError(t, err)
		assert.Equal(t, d.Capabilities, got.Capabilities)
	})
	t.Run("a bad digest is refused before it is a path", func(t *testing.T) {
		s := openTemp(t)
		for _, bad := range []string{"", "../x", "sha256:zz", "md5:" + strings.Repeat("a", 32), "sha256:" + strings.Repeat("A", 64)} {
			_, err := s.GetDescriptor(bad)
			require.ErrorIs(t, err, ErrInvalidDigest, bad)
			require.ErrorIs(t, s.PutDescriptor(Descriptor{Digest: bad}), ErrInvalidDigest, bad)
		}
	})
	t.Run("a descriptor that does not decode is never written over", func(t *testing.T) {
		s := openTemp(t)
		path := filepath.Join(s.Dir(), "descriptors", strings.Repeat("5", 64)+".json")
		require.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))
		err := s.PutDescriptor(d)
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
	t.Run("skips a member it does not know", func(t *testing.T) {
		_, err := DecodeToolchain([]byte(strings.Replace(string(data), `"sbxVersion"`, `"added": [1], "sbxVersion"`, 1)))
		require.NoError(t, err)
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
	_, err := DecodeDescriptor([]byte(`{"schema": "state-descriptor/v1", "digest": "x", "capabilities": {}}`))
	require.ErrorIs(t, err, ErrInvalid)
}

func TestStore_GetDescriptor_otherDigestThanItsName(t *testing.T) {
	s := openTemp(t)
	d := Descriptor{Digest: "sha256:" + strings.Repeat("6", 64)}
	require.NoError(t, s.PutDescriptor(d))
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
		require.Error(t, s.UpdateProject("shop", func(*Project) error { return nil }))
		require.NoError(t, os.Mkdir(filepath.Join(s.Dir(), "descriptors", strings.Repeat("8", 64)+".json"), 0o700))
		require.Error(t, s.PutDescriptor(Descriptor{Digest: "sha256:" + strings.Repeat("8", 64)}))
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
		require.ErrorIs(t, s.UpdateToolchain(func(*Toolchain) error { return boom }), boom)
		assert.NoFileExists(t, filepath.Join(s.Dir(), "toolchain.json"))
		require.Error(t, s.UpdateToolchain(func(*Toolchain) error { return nil }), "a record with members missing is not written")
		assert.NoFileExists(t, filepath.Join(s.Dir(), "toolchain.json"))
	})
}
