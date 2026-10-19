// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// tree returns valid.json decoded into maps and slices.
func tree(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, jsonv2.Unmarshal(fixture(t, "valid.json"), &doc))
	return doc
}

// canonicalOf returns the canonical JSON of doc: the keys of every
// object sorted, compact, no escape of <, > and &.
func canonicalOf(t *testing.T, doc any) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	require.NoError(t, enc.Encode(doc))
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// memberPaths returns the path of every member of the objects of doc
// that the format's own types declare, the run's content left out, as
// "repos.0.dir"; each is with its object and the member's key.
func memberPaths(prefix string, v any, out map[string]func(remove bool) any) {
	switch v := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(v)) {
			p := strings.TrimPrefix(prefix+"."+k, ".")
			obj, key := v, k
			out[p] = func(remove bool) any {
				old := obj[key]
				if remove {
					delete(obj, key)
				} else {
					obj[key] = nil
				}
				return old
			}
			if p != "run" {
				memberPaths(p, v[k], out)
			}
		}
	case []any:
		for i, e := range v {
			memberPaths(prefix+"."+strconv.Itoa(i), e, out)
		}
	}
}

// TestDecodeRefusesAMissingOrNullMember removes each member of the
// format's types from valid.json, at every level, and then sets each
// to null: Decode refuses both, except that salvageRun, present only on
// salvage calls, may be left out.
func TestDecodeRefusesAMissingOrNullMember(t *testing.T) {
	paths := map[string]func(bool) any{}
	memberPaths("", tree(t), paths)
	require.Contains(t, paths, "repos.1.base")
	require.Contains(t, paths, "salvage.capBytes")
	require.Contains(t, paths, "salvageRun.id")
	for _, remove := range []bool{true, false} {
		for _, p := range slices.Sorted(maps.Keys(paths)) {
			name := "null"
			if remove {
				name = "missing"
			}
			t.Run(name+"/"+p, func(t *testing.T) {
				doc := tree(t)
				fresh := map[string]func(bool) any{}
				memberPaths("", doc, fresh)
				fresh[p](remove)
				_, err := Decode(canonicalOf(t, doc))
				if remove && p == "salvageRun" {
					require.NoError(t, err)
					return
				}
				require.ErrorIs(t, err, ErrInvalid)
			})
		}
	}
}

// TestDecodeRefusesABadValue changes one value of valid.json at a time,
// in a form that stays canonical, so only the check of the value can
// refuse it.
func TestDecodeRefusesABadValue(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(m *Manifest)
	}{
		{"a generation that is not a ULID", func(m *Manifest) { m.Generation = "not-a-ulid" }},
		{"an uppercase generation", func(m *Manifest) { m.Generation = strings.ToUpper(m.Generation) }},
		{"an empty generation", func(m *Manifest) { m.Generation = "" }},
		{"a salvage id that is not a ULID", func(m *Manifest) { m.SalvageRun.ID = "x" }},
		{"a short spec commit", func(m *Manifest) { m.SpecCommit = m.SpecCommit[:39] }},
		{"an uppercase spec commit", func(m *Manifest) { m.SpecCommit = strings.ToUpper(m.SpecCommit) }},
		{"a spec commit of 64 digits", func(m *Manifest) { m.SpecCommit = strings.Repeat("a", 64) }},
		{"a base that is not a commit id", func(m *Manifest) { m.Repos[0].Base = []string{"main"} }},
		{"a run digest that is not hex", func(m *Manifest) { m.RunDigest = strings.Repeat("z", 64) }},
		{"an uppercase run digest", func(m *Manifest) { m.RunDigest = strings.ToUpper(m.RunDigest) }},
		{"a run digest of another run", func(m *Manifest) { m.RunDigest = strings.Repeat("a", 64) }},
		{"a run changed under its digest", func(m *Manifest) { m.Run["tabs"] = []any{} }},
		{"a negative cap", func(m *Manifest) { m.Salvage.CapBytes = -1 }},
		{"no primary", func(m *Manifest) { m.Repos[0].Primary = false }},
		{"two primaries", func(m *Manifest) { m.Repos[1].Primary = true }},
		{"no repository", func(m *Manifest) { m.Repos = []Repo{} }},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			m := valid(t)
			c.change(&m)
			data, err := Encode(m)
			require.NoError(t, err)
			_, err = Decode(data)
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
}

// TestDecodeReadsOnlyCanonicalJSON spells valid.json in ways another
// JSON reader accepts: Decode refuses each, since Encode writes none.
func TestDecodeReadsOnlyCanonicalJSON(t *testing.T) {
	data := fixture(t, "valid.json")
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"a trailing newline", append(slices.Clone(data), '\n')},
		{"a leading space", append([]byte(" "), data...)},
		{"a space after a colon", bytes.Replace(data, []byte(`"agent":"`), []byte(`"agent": "`), 1)},
		{"members out of order", bytes.Replace(data, []byte(`{"agent":"sbx-kit-claude","egress":["dl.google.com","github.com"],`), []byte(`{"egress":["dl.google.com","github.com"],"agent":"sbx-kit-claude",`), 1)},
		{"an escaped letter", bytes.Replace(data, []byte(`"project":"shop"`), []byte(`"project":"\u0073hop"`), 1)},
		{"an escaped slash", bytes.Replace(data, []byte(`"node_modules/"`), []byte(`"node_modules\/"`), 1)},
		{"an exponent", bytes.Replace(data, []byte(`"capBytes":1073741824`), []byte(`"capBytes":1073741824e0`), 1)},
		{"a number in the run spelled another way", bytes.Replace(data, []byte(`"ratio":0.5`), []byte(`"ratio":5E-1`), 1)},
		{"an escaped U+FFFD", bytes.Replace(data, []byte(`"project":"shop"`), []byte(`"project":"sh\ufffdop"`), 1)},
		{"a literal U+FFFD", bytes.Replace(data, []byte(`"project":"shop"`), []byte("\"project\":\"sh\uFFFDop\""), 1)},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			require.NotEqual(t, string(data), string(c.data), "the case changes the fixture")
			_, err := Decode(c.data)
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
}

// withRun returns valid.json with run set to the JSON text run and its
// digest set to the one canon gives.
func withRun(t *testing.T, run string) []byte {
	t.Helper()
	doc := tree(t)
	var r Run
	require.NoError(t, jsonv2.Unmarshal([]byte(run), &r))
	sum, err := canon.Digest(canon.Run, r)
	require.NoError(t, err)
	doc["run"] = map[string]any(r)
	doc["runDigest"] = sum.String()
	out := canonicalOf(t, doc)
	enc := canonicalOf(t, map[string]any(r))
	return bytes.Replace(out, enc, []byte(run), 1)
}

// TestRunNumbersKeepTheirText holds that no two spellings of the run
// decode to one manifest: a number past what a float64 holds exactly,
// or spelled in a form Encode does not write, is refused, and the one
// canonical spelling decodes.
func TestRunNumbersKeepTheirText(t *testing.T) {
	for _, c := range []struct {
		run    string
		accept bool
	}{
		{`{"n":9007199254740992}`, true},
		{`{"n":9007199254740993}`, false},
		{`{"n":100}`, true},
		{`{"n":1E2}`, false},
		{`{"n":1e2}`, false},
		{`{"n":100.0}`, false},
		{`{"n":0.5}`, true},
		{`{"n":-0}`, true},
	} {
		t.Run(c.run, func(t *testing.T) {
			m, err := Decode(withRun(t, c.run))
			if !c.accept {
				require.ErrorIs(t, err, ErrInvalid)
				return
			}
			require.NoError(t, err)
			data, err := Encode(m)
			require.NoError(t, err)
			assert.Contains(t, string(data), `"run":`+c.run)
		})
	}
}

// FuzzRunIsCanonical builds a manifest around a run of any JSON shape:
// Encode writes the bytes canon digests, and Decode of them gives the
// manifest back.
func FuzzRunIsCanonical(f *testing.F) {
	for _, s := range []string{`{}`, `{"a":[1,2.5,null,true,{"b":"<&>"}]}`, `{"\u2028":"é","z":{"y":{"x":[]}}}`, `{"n":1e21,"m":-0.0001}`} {
		f.Add([]byte(s))
	}
	data, err := os.ReadFile("testdata/valid.json")
	require.NoError(f, err)
	base, err := Decode(data)
	require.NoError(f, err)
	f.Fuzz(func(t *testing.T, run []byte) {
		var r Run
		if jsonv2.Unmarshal(run, &r) != nil || r == nil {
			return
		}
		m := base
		m.Run = r
		sum, err := canon.Digest(canon.Run, r)
		if err != nil {
			require.ErrorIs(t, err, canon.ErrInvalidText)
			return
		}
		m.RunDigest = sum.String()
		data, err := Encode(m)
		require.NoError(t, err)
		whole, err := canon.Digest(canon.Run, asRun{m})
		require.NoError(t, err)
		assert.Equal(t, sha256.Sum256(append([]byte("romeu/run/v1\x00"), data...)), [32]byte(whole))
		back, err := Decode(data)
		require.NoError(t, err)
		assert.Equal(t, m, back)
	})
}
