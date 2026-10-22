// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package salvage_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
	"github.com/brunovenceslau/romeu-e-julieta/internal/salvage"
)

var (
	sha1A = strings.Repeat("a", 40)
	sha1B = strings.Repeat("b", 40)
	at    = time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC)
)

func TestHeadsRoundTrip(t *testing.T) {
	h := salvage.Heads{CreatedAt: at, Disabled: true, Refs: map[string]string{
		"refs/heads/main": sha1A, "refs/salvage-reflog/main/1": sha1B,
	}}
	data, err := salvage.EncodeHeads(h)
	require.NoError(t, err)
	assert.Equal(t, `{"createdAt":"2026-10-21T09:30:00Z","disabled":true,"refs":{"refs/heads/main":"`+sha1A+
		`","refs/salvage-reflog/main/1":"`+sha1B+`"},"schema":"snapshot/v1"}`+"\n", string(data))
	got, err := salvage.DecodeHeads(data)
	require.NoError(t, err)
	h.Schema = salvage.HeadsField
	assert.Equal(t, h, got)
}

func TestEncodeHeadsWritesAnEmptyRefsObject(t *testing.T) {
	data, err := salvage.EncodeHeads(salvage.Heads{CreatedAt: at})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"refs":{}`)
	_, err = salvage.DecodeHeads(data)
	require.NoError(t, err)
}

func TestEncodeHeadsRefusesWhatDecodeRefuses(t *testing.T) {
	_, err := salvage.EncodeHeads(salvage.Heads{CreatedAt: at, Refs: map[string]string{"refs/remotes/origin/main": sha1A}})
	require.ErrorIs(t, err, salvage.ErrInvalid)
}

// TestDecodeHeadsIsStrict: romeu's reader of snapshot/heads.json (03
// 3.13, REQ-0261) refuses a duplicate member, a missing one, another
// schema, a time that is not UTC, a ref outside the namespaces of the
// bundle or with a character git refuses, an object id of another
// form, trailing data and a document over the file cap.
func TestDecodeHeadsIsStrict(t *testing.T) {
	ok := `{"createdAt":"2026-10-21T09:30:00Z","disabled":false,"refs":{"refs/heads/main":"` + sha1A + `"},"schema":"snapshot/v1"}`
	_, err := salvage.DecodeHeads([]byte(ok))
	require.NoError(t, err)
	for name, doc := range map[string]string{
		"duplicate member": strings.Replace(ok, `"disabled":false`, `"disabled":false,"disabled":true`, 1),
		"missing refs":     strings.Replace(ok, `"refs":{"refs/heads/main":"`+sha1A+`"},`, ``, 1),
		"missing disabled": strings.Replace(ok, `"disabled":false,`, ``, 1),
		"missing created":  strings.Replace(ok, `"createdAt":"2026-10-21T09:30:00Z",`, ``, 1),
		"null refs":        strings.Replace(ok, `{"refs/heads/main":"`+sha1A+`"}`, `null`, 1),
		"older schema":     strings.Replace(ok, `snapshot/v1`, `snapshot/v0`, 1),
		"other schema":     strings.Replace(ok, `snapshot/v1`, `manifest/v1`, 1),
		"not utc":          strings.Replace(ok, `09:30:00Z`, `09:30:00+01:00`, 1),
		"zero time":        strings.Replace(ok, `2026-10-21T09:30:00Z`, `0001-01-01T00:00:00Z`, 1),
		"remote ref":       strings.Replace(ok, `refs/heads/main`, `refs/remotes/origin/main`, 1),
		"bare namespace":   strings.Replace(ok, `refs/heads/main`, `refs/heads/`, 1),
		"dotdot":           strings.Replace(ok, `refs/heads/main`, `refs/heads/a..b`, 1),
		"control":          strings.Replace(ok, `refs/heads/main`, `refs/heads/a\u001bb`, 1),
		"format char":      strings.Replace(ok, `refs/heads/main`, `refs/heads/a\u202eb`, 1),
		"short id":         strings.Replace(ok, sha1A, "abc", 1),
		"upper id":         strings.Replace(ok, sha1A, strings.Repeat("A", 40), 1),
		"trailing data":    ok + "{}",
		"not an object":    `[]`,
		"over the cap":     strings.Replace(ok, `"refs/heads/main"`, `"refs/heads/`+strings.Repeat("x", 1<<20)+`"`, 1),
	} {
		_, err := salvage.DecodeHeads([]byte(doc))
		require.Error(t, err, name)
		require.ErrorIs(t, err, salvage.ErrInvalid, name)
	}
	sha256 := strings.Replace(ok, sha1A, strings.Repeat("c", 64), 1)
	_, err = salvage.DecodeHeads([]byte(sha256))
	require.NoError(t, err, "a SHA-256 repository's object ids")
}

// TestDecodeHeadsIgnoresAnUnknownMember: a member a newer julieta adds to
// snapshot/heads.json is skipped, whatever its value (03, opening,
// REQ-0163).
func TestDecodeHeadsIgnoresAnUnknownMember(t *testing.T) {
	ok := `{"createdAt":"2026-10-21T09:30:00Z","disabled":false,"refs":{"refs/heads/main":"` + sha1A + `"},"schema":"snapshot/v1"}`
	want, err := salvage.DecodeHeads([]byte(ok))
	require.NoError(t, err)
	for name, extra := range map[string]string{"a number": `1`, "an object": `{"a":[1,{"b":null}]}`, "a string": `"x"`} {
		got, err := salvage.DecodeHeads([]byte(strings.Replace(ok, `"disabled"`, `"extra":`+extra+`,"disabled"`, 1)))
		require.NoError(t, err, name)
		assert.Equal(t, want, got, name)
	}
}

// TestDecodeHeadsHostileUnknownMembers holds what a member this build
// skips may not do, whoever wrote it (03, opening, REQ-0163): change what
// a known member says (a name of another case is another name, in either
// order), pass a duplicate, a nest too deep or a document over the file
// cap unseen, or put its own text in an error.
func TestDecodeHeadsHostileUnknownMembers(t *testing.T) {
	doc := `{"createdAt":"2026-10-21T09:30:00Z","disabled":false,"refs":{"refs/heads/main":"` + sha1A + `"},"schema":"snapshot/v1"}`
	want, err := salvage.DecodeHeads([]byte(doc))
	require.NoError(t, err)
	after := func(members string) []byte { return []byte(strings.TrimSuffix(doc, "}") + "," + members + "}") }
	before := func(members string) []byte { return []byte("{" + members + "," + strings.TrimPrefix(doc, "{")) }
	for name, members := range map[string]string{
		"a case variant of a known name":  `"Schema":"snapshot/v9","Disabled":true,"REFS":{}`,
		"an escape in a name and a value": `"\u001b[31mred":"\u0007\u001b[0m"`,
	} {
		for pos, build := range map[string]func(string) []byte{"after": after, "before": before} {
			got, err := salvage.DecodeHeads(build(members))
			require.NoError(t, err, name+" "+pos)
			assert.Equal(t, want, got, name+" "+pos)
		}
	}
	for name, members := range map[string]string{
		"a duplicate in an unknown member":        `"x":{"a":1,"a":2}`,
		"a nest of 20000 arrays":                  `"x":` + strings.Repeat("[", 20000) + strings.Repeat("]", 20000),
		"a duplicate known name beside an escape": `"\u001b[31mred":1,"disabled":true`,
		"the excess over the file cap":            `"x":"` + strings.Repeat("y", memstore.MaxFileSize) + `"`,
	} {
		_, err := salvage.DecodeHeads(after(members))
		require.ErrorIs(t, err, salvage.ErrInvalid, name)
		assert.NotContains(t, err.Error(), "\x1b", name)
		assert.NotContains(t, err.Error(), "red", name)
	}
	// The name of a duplicate is in the error, as jsontext quotes it, with
	// the escape written out and no raw control character.
	_, err = salvage.DecodeHeads(after(`"\u001b[31mred":1,"\u001b[31mred":2`))
	require.ErrorIs(t, err, salvage.ErrInvalid)
	assert.NotContains(t, err.Error(), "\x1b")
}

func TestDecodeHeadsOfANewerVersionIsFormatNewer(t *testing.T) {
	_, err := salvage.DecodeHeads([]byte(`{"schema":"snapshot/v2","what":"else"}`))
	require.Error(t, err)
	assert.Equal(t, manifest.FormatNewer.ID, cli.Of(err).ID)
}
