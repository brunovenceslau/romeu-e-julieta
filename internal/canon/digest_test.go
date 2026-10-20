// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package canon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entry is a value of the kind memory-content, as a test writes one:
// its struct fields are declared out of key order, so that the
// canonical form has to sort them.
type entry struct {
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Body   string   `json:"body"`
	Tags   []string `json:"tags"`
}

func (entry) CanonKind() MemoryContentKind { return MemoryContent }

// fingerprint is a value of the kind fingerprint with the same JSON
// shape as entry, so that only the kind tells their digests apart.
type fingerprint entry

func (fingerprint) CanonKind() FingerprintKind { return Fingerprint }

// rawOf is a value of the kind K whose JSON a test writes by hand.
// Digest reads the kind from its argument, not from CanonKind, so the
// zero K stands for the kind here.
type rawOf[K Kind] jsontext.Value

func (rawOf[K]) CanonKind() K {
	var k K
	return k
}

func (r rawOf[K]) MarshalJSON() ([]byte, error) { return r, nil }

// raw is a memory-content value whose JSON a test writes by hand.
type raw = rawOf[MemoryContentKind]

// anyValue is a memory-content value of any shape.
type anyValue struct{ v any }

func (anyValue) CanonKind() MemoryContentKind { return MemoryContent }

// MarshalJSONTo writes the value with the encoder Digest passes, under
// Digest's own options.
func (a anyValue) MarshalJSONTo(enc *jsontext.Encoder) error { return json.MarshalEncode(enc, a.v) }

// preimage is what Digest hashes, written out by hand from 03 3.12.
func preimage(kind, canonical string) []byte {
	return []byte("romeu/" + kind + "/v1\x00" + canonical)
}

func TestDigestHashesTheDomainNulAndTheCanonicalJSON(t *testing.T) {
	t.Parallel()
	v := entry{Title: "t", Status: "open", Body: "b", Tags: []string{"x", "y"}}
	got, err := Digest(MemoryContent, v)
	require.NoError(t, err)
	want := sha256.Sum256(preimage("memory-content",
		`{"body":"b","status":"open","tags":["x","y"],"title":"t"}`))
	assert.Equal(t, want, [32]byte(got))
}

func TestDigestOfTwoKindsOverOneValueDiffers(t *testing.T) {
	t.Parallel()
	v := entry{Title: "t", Status: "open", Body: "b", Tags: []string{}}
	m, err := Digest(MemoryContent, v)
	require.NoError(t, err)
	f, err := Digest(Fingerprint, fingerprint(v))
	require.NoError(t, err)
	assert.NotEqual(t, [32]byte(m), [32]byte(f))
	assert.Equal(t, sha256.Sum256(preimage("fingerprint",
		`{"body":"b","status":"open","tags":[],"title":"t"}`)), [32]byte(f))
}

func TestCanonicalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{"keys sorted at every depth", map[string]any{"b": 1, "a": map[string]any{"d": true, "c": nil}},
			`{"a":{"c":null,"d":true},"b":1}`},
		{"struct fields sorted by json name", entry{Title: "z", Body: "a"},
			`{"body":"a","status":"","tags":null,"title":"z"}`},
		{"keys sorted bytewise", map[string]int{"\u00e9": 1, "z": 2, "Z": 3, "": 4},
			"{\"\":4,\"Z\":3,\"z\":2,\"\u00e9\":1}"},
		{"html characters kept as they are", "<a & b>", `"<a & b>"`},
		{"non-ascii kept as utf-8", "a\u00e7\u00e3o \u2603 \U0001F600", "\"a\u00e7\u00e3o \u2603 \U0001F600\""},
		{"control characters escaped", "a\nb\u0001", `"a\nb\u0001"`},
		{"int64 above 2^53 kept exact", int64(math.MaxInt64), `9223372036854775807`},
		{"uint64 kept exact", uint64(math.MaxUint64), `18446744073709551615`},
		{"float in its ecmascript form", 0.1, `0.1`},
		{"array order kept", []any{3, "a", false, []int{}}, `[3,"a",false,[]]`},
		{"keys escaped as strings are", map[string]int{"<k&>": 1}, `{"<k&>":1}`},
		{"control keys escaped, line separators kept", map[string]int{"\t": 1, "\u2028": 2, "\x7f": 3},
			"{\"\\t\":1,\"\x7f\":3,\"\u2028\":2}"},
		{"keys sorted by utf-16, not utf-8", map[string]int{"\U0001F600": 2, "\uff5e": 1},
			"{\"\U0001F600\":2,\"\uff5e\":1}"},
		{"keys sorted as in rfc 8785 3.2.3", raw(`{"\u20ac":"Euro Sign","\r":"Carriage Return","\ufb33":"Hebrew Letter Dalet With Dagesh","1":"One","\ud83d\ude00":"Emoji: Grinning Face","\u0080":"Control","\u00f6":"Latin Small Letter O With Diaeresis"}`),
			"{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\",\"\u00f6\":\"Latin Small Letter O With Diaeresis\",\"\u20ac\":\"Euro Sign\",\"\U0001F600\":\"Emoji: Grinning Face\",\"\ufb33\":\"Hebrew Letter Dalet With Dagesh\"}"},
		{"strings escaped as in rfc 8785 3.2.2.2", raw(`"\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/"`),
			"\"\u20ac$\\u000f\\nA'B\\\"\\\\\\\\\\\"/\""},
		{"objects inside arrays sorted", raw(`[{"b":1,"a":2}]`), `[{"a":2,"b":1}]`},
		{"large and small floats in their ecmascript form", []float64{1e21, 1e-7}, `[1e+21,1e-7]`},
		{"numbers of MarshalJSON in their one form", raw(`[1,1.0,1e0,-0,4.50,2e-3,1E30,0.000001,1e-7]`), `[1,1,1,0,4.5,0.002,1e+30,0.000001,1e-7]`},
		{"integer of MarshalJSON above 2^53 kept exact", raw(`[9007199254740993,-12345678901234567890]`), `[9007199254740993,-12345678901234567890]`},
		{"float above 2^53 in its ecmascript form", float64(1 << 60), `1152921504606847000`},
		{"empty object", map[string]int{}, `{}`},
		{"null", nil, `null`},
		{"bytes as base64", []byte{0, 1, 2}, `"AAEC"`},
		{"insignificant whitespace removed", raw(" {\n\t\"b\" : [ 1 , 2 ] ,\r\n \"a\" : \"x y\" } "),
			`{"a":"x y","b":[1,2]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalJSON(tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}

func TestCanonicalJSONSameContentSameDigest(t *testing.T) {
	t.Parallel()
	a, err := Digest(MemoryContent, raw(`{"title":"t","body":"b"}`))
	require.NoError(t, err)
	b, err := Digest(MemoryContent, raw(`{ "body": "b", "title": "t" }`))
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestDigestRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value Input[MemoryContentKind]
		msg   string
	}{
		{"a value encoding/json/v2 cannot write", anyValue{math.NaN()}, "unsupported value"},
		{"a channel", anyValue{make(chan int)}, "chan int"},
		{"a duplicate key", raw(`{"a":1,"a":2}`), `duplicate object member name "a"`},
		{"a MarshalJSON that writes invalid JSON", raw(`{"a":`), "unexpected EOF"},
		{"a nested duplicate key", raw(`{"x":[{"k":1,"k":1}]}`), `duplicate object member name "k"`},
		{"invalid utf-8 in a string", anyValue{"a\xff"}, "U+FFFD"},
		{"invalid utf-8 in a key", anyValue{map[string]int{"k\xfe": 1}}, "U+FFFD"},
		{"a lone surrogate from MarshalJSON", raw(`["\ud800"]`), "U+FFFD"},
		{"a U+FFFD the value holds", anyValue{"a\ufffd"}, "U+FFFD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Digest(MemoryContent, tt.value)
			require.ErrorContains(t, err, "canon: digest memory-content")
			require.ErrorContains(t, err, tt.msg)
		})
	}
}

func TestDigestRefusesAKindOutsideTheTable(t *testing.T) {
	t.Parallel()
	_, err := Digest(MemoryContentKind("fingerprint"), entry{})
	require.ErrorIs(t, err, ErrKind)
	_, err = Digest(MemoryContentKind(""), entry{})
	require.ErrorIs(t, err, ErrKind)
}

func TestSumText(t *testing.T) {
	t.Parallel()
	sum, err := Digest(MemoryContent, raw(`{}`))
	require.NoError(t, err)
	want := sha256.Sum256(preimage("memory-content", `{}`))
	assert.Equal(t, hex.EncodeToString(want[:]), sum.String())
	text, err := sum.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, sum.String(), string(text))

	var back Sum[MemoryContentKind]
	require.NoError(t, back.UnmarshalText(text))
	assert.Equal(t, sum, back)

	tests := []struct {
		name string
		text string
	}{
		{"uppercase digits", strings.ToUpper(sum.String())},
		{"too short", sum.String()[:63]},
		{"too long", sum.String() + "0"},
		{"one byte short", sum.String()[:62]},
		{"one byte long", sum.String() + "00"},
		{"not hexadecimal", "g" + sum.String()[1:]},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := back
			require.ErrorIs(t, got.UnmarshalText([]byte(tt.text)), ErrInvalidDigest)
			assert.Equal(t, back, got, "a refused text leaves the digest as it was")
		})
	}
}

func TestSumInJSON(t *testing.T) {
	t.Parallel()
	type record struct {
		Fingerprint Sum[FingerprintKind] `json:"fingerprint"`
	}
	sum, err := Digest(Fingerprint, rawOf[FingerprintKind](`{"a":1}`))
	require.NoError(t, err)
	data, err := json.Marshal(record{Fingerprint: sum})
	require.NoError(t, err)
	assert.JSONEq(t, `{"fingerprint":"`+sum.String()+`"}`, string(data))
	var back record
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, sum, back.Fingerprint)
}

func TestDigestErrorsQuoteAShortInput(t *testing.T) {
	t.Parallel()
	var sum Sum[MemoryContentKind]
	err := sum.UnmarshalText(bytes.Repeat([]byte("a"), 1<<20))
	require.ErrorIs(t, err, ErrInvalidDigest)
	assert.Less(t, len(err.Error()), 200)
	_, err = Digest(MemoryContentKind(strings.Repeat("k", 1<<20)), entry{})
	require.ErrorIs(t, err, ErrKind)
	assert.Less(t, len(err.Error()), 200)
}

func TestDigestOfTwoInvalidStringsIsNoDigest(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"a\xff", "a\xfe"} {
		_, err := Digest(MemoryContent, anyValue{s})
		require.ErrorIs(t, err, ErrInvalidText, "%q", s)
	}
}
