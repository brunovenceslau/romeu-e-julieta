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
// 3.13, REQ-0261) refuses an unknown member, a duplicate one, a missing
// one, another schema, a time that is not UTC, a ref outside the
// namespaces of the bundle or with a character git refuses, an object
// id of another form, trailing data and a document over the file cap.
func TestDecodeHeadsIsStrict(t *testing.T) {
	ok := `{"createdAt":"2026-10-21T09:30:00Z","disabled":false,"refs":{"refs/heads/main":"` + sha1A + `"},"schema":"snapshot/v1"}`
	_, err := salvage.DecodeHeads([]byte(ok))
	require.NoError(t, err)
	for name, doc := range map[string]string{
		"unknown member":   strings.Replace(ok, `"disabled"`, `"extra":1,"disabled"`, 1),
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

func TestDecodeHeadsOfANewerVersionIsFormatNewer(t *testing.T) {
	_, err := salvage.DecodeHeads([]byte(`{"schema":"snapshot/v2","what":"else"}`))
	require.Error(t, err)
	assert.Equal(t, manifest.FormatNewer.ID, cli.Of(err).ID)
}
