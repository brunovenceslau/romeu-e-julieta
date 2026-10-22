// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"errors"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// fixture returns the bytes of testdata/name.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

// valid returns the decoded testdata/valid.json.
func valid(t *testing.T) Manifest {
	t.Helper()
	m, err := Decode(fixture(t, "valid.json"))
	require.NoError(t, err)
	return m
}

// withMember returns the canonical valid.json with the member key set
// to the JSON text value, which the test writes by hand.
func withMember(t *testing.T, key, value string) []byte {
	t.Helper()
	data := fixture(t, "valid.json")
	start := bytes.Index(data, []byte(`"`+key+`":`))
	require.GreaterOrEqual(t, start, 0, key)
	start += len(key) + 3
	m := valid(t)
	enc, err := Encode(m)
	require.NoError(t, err)
	require.Equal(t, string(data), string(enc), "valid.json is canonical")
	end := start + valueLen(t, data[start:])
	return slices.Concat(data[:start], []byte(value), data[end:])
}

// valueLen returns the length of the scalar JSON value data starts with.
func valueLen(t *testing.T, data []byte) int {
	t.Helper()
	if data[0] == '"' {
		i := bytes.IndexByte(data[1:], '"')
		require.GreaterOrEqual(t, i, 0)
		return i + 2
	}
	i := bytes.IndexAny(data, ",}")
	require.GreaterOrEqual(t, i, 0)
	return i
}

// rowOf returns the row of the *cli.Error in err's chain.
func rowOf(t *testing.T, err error) cli.Row {
	t.Helper()
	var e *cli.Error
	require.ErrorAs(t, err, &e)
	return e.Row
}

func TestDecodeTheExampleOf036(t *testing.T) {
	m := valid(t)
	assert.Equal(t, "manifest/v1", m.Schema)
	assert.Equal(t, Protocol, m.Protocol)
	assert.Equal(t, "shop", m.Project)
	require.Len(t, m.Repos, 2)
	assert.True(t, m.Repos[0].Primary)
	assert.Equal(t, "shop-web", m.Repos[1].Dir)
	assert.Equal(t, int64(1073741824), m.Salvage.CapBytes)
	require.NotNil(t, m.SalvageRun)
	assert.Equal(t, "01j9zd0a1b2c3d4e5f6g7h8j9k", m.SalvageRun.ID)
	assert.Equal(t, []string{"dl.google.com", "github.com"}, m.Egress)
	assert.Contains(t, m.Run, "tabs")
}

// TestProtocolWindow holds REQ-0219 and REQ-0440 over N-2, N-1, N and
// N+1, for the protocol of this build and for a later one, so the N-1
// case is accepted even while N is the first protocol.
func TestProtocolWindow(t *testing.T) {
	for _, n := range []int{Protocol, Protocol + 1} {
		for _, c := range []struct {
			name   string
			p      int
			accept bool
		}{
			{"N-2", n - 2, false},
			{"N-1", n - 1, n-1 >= FirstProtocol},
			{"N", n, true},
			{"N+1", n + 1, false},
		} {
			t.Run("N="+strconv.Itoa(n)+"/"+c.name, func(t *testing.T) {
				assert.Equal(t, c.accept, acceptsProtocol(c.p, n))
			})
		}
	}
}

// TestDecodeRefusesAProtocolOutsideTheWindow decodes a document of each
// protocol around Protocol: outside the window it is manifest-protocol,
// exit 2, with the hint filled with the project.
func TestDecodeRefusesAProtocolOutsideTheWindow(t *testing.T) {
	for _, c := range []struct {
		name string
		p    int
	}{{"N-2", Protocol - 2}, {"N-1", Protocol - 1}, {"N", Protocol}, {"N+1", Protocol + 1}, {"zero", 0}} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode(withMember(t, "protocol", strconv.Itoa(c.p)))
			if acceptsProtocol(c.p, Protocol) {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, ManifestProtocol, rowOf(t, err))
			assert.Equal(t, 2, rowOf(t, err).Exit)
			var e *cli.Error
			require.ErrorAs(t, err, &e)
			assert.Contains(t, e.Hint(), "romeu run shop")
		})
	}
}

func TestDecodeRefusesANewerSchemaVersion(t *testing.T) {
	// A newer version may hold members this one does not name, and a
	// document that is not canonical: the version is read first, so it is
	// format-newer and not an invalid document.
	data := withMember(t, "schema", `"manifest/v2"`)
	data = bytes.Replace(data, []byte(`{"agent"`), []byte(`{"added":1,"agent"`), 1)
	_, err := Decode(data)
	require.Error(t, err)
	assert.Equal(t, FormatNewer, rowOf(t, err))
}

// TestDecodeIgnoresAnUnknownMember reads a manifest to which a newer
// romeu added a member at every level the format names, as the manifest
// without it (03 3.6, REQ-0163): same protocol, a member julieta may
// safely ignore.
func TestDecodeIgnoresAnUnknownMember(t *testing.T) {
	want := valid(t)
	doc := tree(t)
	added := func() map[string]any { return map[string]any{"a": []any{1.0}, "b": "x"} }
	doc["added"] = added()
	doc["salvage"].(map[string]any)["added"] = added()
	doc["salvageRun"].(map[string]any)["added"] = added()
	for _, r := range doc["repos"].([]any) {
		r.(map[string]any)["added"] = added()
	}
	data := canonicalOf(t, doc)
	require.Contains(t, string(data), `"added"`)
	got, err := Decode(data)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// The fixture of one member at the top, and the env value of it.
	got, err = Decode(fixture(t, "valid-unknown-member.json"))
	require.NoError(t, err)
	assert.Equal(t, want, got)
	got, err = DecodeEnv(base64.RawURLEncoding.EncodeToString(data))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// TestDecodeHostileUnknownMembers holds what a member this build skips
// may not do, whoever wrote it (03 3.6, REQ-0163): change what a known
// member says (a name of another case is another name, in either
// order), pass a duplicate or a nest too deep unseen, loosen the
// canonical form of the members it does not skip, or put its own text
// in an error.
func TestDecodeHostileUnknownMembers(t *testing.T) {
	want := valid(t)
	doc := strings.TrimSpace(string(fixture(t, "valid.json")))
	after := func(members string) []byte { return []byte(strings.TrimSuffix(doc, "}") + "," + members + "}") }
	before := func(members string) []byte { return []byte("{" + members + "," + strings.TrimPrefix(doc, "{")) }
	for name, members := range map[string]string{
		"a case variant of a known name":     `"Schema":"x","SCHEMA":"y"`,
		"a case variant of a value":          `"Project":"evil","AGENT":"evil","Egress":["evil.example"]`,
		"an escape in a name and a value":    `"\u001b[31mred":"\u0007\u001b[0m"`,
		"an unknown member holding an array": `"x":[{"a":[[],{}]},null,1.5e3]`,
	} {
		for pos, build := range map[string]func(string) []byte{"after": after, "before": before} {
			got, err := Decode(build(members))
			require.NoError(t, err, name+" "+pos)
			assert.Equal(t, want, got, name+" "+pos)
		}
	}
	// A case variant inside a repository is skipped and the real value wins.
	nested := bytes.Replace([]byte(doc), []byte(`{"base":["89abc`), []byte(`{"Dir":"evil","URL":"https://evil.example","base":["89abc`), 1)
	got, err := Decode(nested)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	refused := map[string][]byte{
		"a duplicate in an unknown member":        after(`"x":{"a":1,"a":2}`),
		"a nest of 10000 arrays":                  after(`"x":` + strings.Repeat("[", 10000) + strings.Repeat("]", 10000)),
		"a duplicate known name beside an escape": after(`"\u001b[31mred":1,"schema":"manifest/v1"`),
		"an unknown member over the size cap":     after(`"x":"` + strings.Repeat("a", MaxJSONSize) + `"`),
		"a duplicate unknown name":                after(`"x":1,"x":2`),
		"a known member missing":                  bytes.Replace(after(`"x":1`), []byte(`"agent":"sbx-kit-claude",`), nil, 1),
		"a known member null":                     bytes.Replace(after(`"x":1`), []byte(`"agent":"sbx-kit-claude"`), []byte(`"agent":null`), 1),
		"a known member spelled with a space":     bytes.Replace(after(`"x":1`), []byte(`"agent":"`), []byte(`"agent": "`), 1),
		"a known member spelled with an escape":   bytes.Replace(after(`"x":1`), []byte(`"agent":`), []byte(`"\u0061gent":`), 1),
		"a space between members":                 bytes.Replace(after(`"x":1`), []byte(`"egress"`), []byte(` "egress"`), 1),
		"a space before an unknown member":        bytes.Replace(after(`"x":1`), []byte(`,"x"`), []byte(`, "x"`), 1),
		"a known member only as a case variant":   bytes.Replace(after(`"x":1`), []byte(`"agent":`), []byte(`"Agent":`), 1),
		"invalid UTF-8 in an unknown member":      after("\"x\":\"\xff\""),
		"U+FFFD in an unknown name":               after("\"x\ufffd\":1"),
	}
	for name, data := range refused {
		_, err := Decode(data)
		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), "\x1b", name)
		assert.NotContains(t, err.Error(), "red", name)
	}
	// The name of a duplicate is in the error, as jsontext quotes it, with
	// the escape written out and no raw control character.
	_, err = Decode(after(`"\u001b[31mred":1,"\u001b[31mred":2`))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "\x1b")
}

// TestUnknownMemberKeepsTheProtocolWindow holds that a member this
// build skips does not widen the window of REQ-0219: N-2 and N+1 are
// still manifest-protocol (RJ-342, exit 2), and N and N-1 are read.
func TestUnknownMemberKeepsTheProtocolWindow(t *testing.T) {
	for _, c := range []struct {
		name string
		p    int
	}{{"N-2", Protocol - 2}, {"N+1", Protocol + 1}, {"zero", 0}} {
		t.Run(c.name, func(t *testing.T) {
			data := bytes.Replace(withMember(t, "protocol", strconv.Itoa(c.p)), []byte(`"agent"`), []byte(`"added":{"a":1},"agent"`), 1)
			_, err := Decode(data)
			require.Error(t, err)
			assert.Equal(t, ManifestProtocol, rowOf(t, err))
			assert.Equal(t, 2, rowOf(t, err).Exit)
		})
	}
	data := bytes.Replace(fixture(t, "valid.json"), []byte(`"agent"`), []byte(`"added":{"a":1},"agent"`), 1)
	_, err := Decode(data)
	require.NoError(t, err, "N")
}

func TestDecodeRefuses(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"a duplicate member", fixture(t, "invalid-duplicate-member.json")},
		{"a duplicate nested member", bytes.Replace(fixture(t, "valid.json"), []byte(`{"id":"01j9`), []byte(`{"id":"x","id":"01j9`), 1)},
		{"a duplicate member in the run", bytes.Replace(fixture(t, "valid.json"), []byte(`"label":"dev"`), []byte(`"label":"dev","label":"x"`), 1)},
		{"a member of another case", bytes.Replace(fixture(t, "valid.json"), []byte(`"agent":`), []byte(`"Agent":`), 1)},
		{"another format", withMember(t, "schema", `"project/v1"`)},
		{"an older version", withMember(t, "schema", `"manifest/v0"`)},
		{"no version", withMember(t, "schema", `"manifest"`)},
		{"a version with a leading zero", withMember(t, "schema", `"manifest/v01"`)},
		{"a schema that is not a string", withMember(t, "schema", `1`)},
		{"no schema", bytes.Replace(fixture(t, "valid.json"), []byte(`"schema":"manifest/v1",`), nil, 1)},
		{"a fractional protocol", withMember(t, "protocol", `1.5`)},
		{"a protocol as a string", withMember(t, "protocol", `"1"`)},
		{"trailing data", append(fixture(t, "valid.json"), []byte(` {}`)...)},
		{"two documents", append(fixture(t, "valid.json"), fixture(t, "valid.json")...)},
		{"invalid UTF-8", bytes.Replace(fixture(t, "valid.json"), []byte(`"shop"`), []byte("\"sh\xffop\""), 1)},
		{"not an object", []byte(`[]`)},
		{"null", []byte(`null`)},
		{"empty", nil},
		{"over the size cap", append(fixture(t, "valid.json"), bytes.Repeat([]byte(" "), MaxJSONSize)...)},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			_, err := Decode(c.data)
			require.Error(t, err)
			if e, ok := errors.AsType[*cli.Error](err); ok {
				assert.NotEqual(t, ManifestProtocol, e.Row, "a document that does not decode is not a protocol error")
			}
		})
	}
}

// TestFieldsAreDeclaredInKeyOrder holds that the fields of every type of
// the format are declared sorted by JSON name, the order in which
// canonical JSON (03 3.12) writes them, so the type reads as the
// document does.
func TestFieldsAreDeclaredInKeyOrder(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[Manifest](), reflect.TypeFor[Repo](), reflect.TypeFor[Salvage](), reflect.TypeFor[SalvageRun](),
	} {
		var names []string
		for f := range typ.Fields() {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			require.NotEmpty(t, name, "%s.%s has a JSON name", typ.Name(), f.Name)
			names = append(names, name)
		}
		assert.True(t, slices.IsSorted(names), "%s: %v", typ.Name(), names)
	}
}

// asRun is a manifest under the kind run, for a test only: canon.Digest
// writes no canonical JSON out, so the test compares digests, which are
// equal only when Encode writes the bytes canon digests.
type asRun struct{ Manifest }

func (asRun) CanonKind() canon.RunKind { return canon.Run }

func TestEncodeWritesTheCanonicalJSONOfCanon(t *testing.T) {
	hostile := valid(t)
	hostile.Project = "<a & b>\u2028\u2029\t\"\\\x01é😀"
	hostile.Run = map[string]any{"z": 1.0, "a": map[string]any{"é": []any{"<", nil, true, 0.5}, "A": "b"}, "\u00ff": "", "\U0001F600": 1e21}
	hostile.SalvageRun = nil
	for name, m := range map[string]Manifest{"the example": valid(t), "hostile text": hostile} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			data, err := Encode(m)
			require.NoError(t, err)
			sum, err := canon.Digest(canon.Run, asRun{m})
			require.NoError(t, err)
			assert.Equal(t, sha256.Sum256(append([]byte("romeu/run/v1\x00"), data...)), [32]byte(sum))
		})
	}
}

func TestEncodeRefusesTextThatIsNotUTF8(t *testing.T) {
	for _, s := range []string{"a\xffb", "a\uFFFDb"} {
		m := valid(t)
		m.Agent = s
		_, err := Encode(m)
		require.ErrorIs(t, err, canon.ErrInvalidText)
		m = valid(t)
		m.Run = map[string]any{s: 1}
		_, err = Encode(m)
		require.ErrorIs(t, err, canon.ErrInvalidText)
	}
}

// TestEncodeRefusesANumberOutOfRange: a run value whose own JSON is a
// number beyond the float64 range, or one a float64 flushes to zero, is
// refused as canon refuses it, not written as the largest float64 or 0.
func TestEncodeRefusesANumberOutOfRange(t *testing.T) {
	for _, n := range []string{"1e400", "-1e400", "1e-400"} {
		m := valid(t)
		m.Run = map[string]any{"n": jsontext.Value(n)}
		_, err := Encode(m)
		require.ErrorIs(t, err, canon.ErrNumberRange, n)
	}
}

func TestEncodeWritesAnEmptyListForANilOne(t *testing.T) {
	m := valid(t)
	m.Egress, m.Repos[0].Base, m.Salvage.ExcludeIgnored, m.Run, m.SalvageRun = nil, nil, nil, nil, nil
	data, err := Encode(m)
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, `"egress":[]`)
	assert.Contains(t, s, `"base":[]`)
	assert.Contains(t, s, `"excludeIgnored":[]`)
	assert.Contains(t, s, `"run":{}`)
	assert.NotContains(t, s, "salvageRun")
	assert.NotContains(t, s, "null")
	assert.Nil(t, m.Egress, "Encode leaves its argument as it was")
}

// TestRoundTrip decodes each canonical fixture and encodes it again:
// the bytes are the same, so Encode is the fixed point of a decode.
func TestRoundTrip(t *testing.T) {
	for _, name := range []string{"valid.json", "size-32k.json"} {
		t.Run(name, func(t *testing.T) {
			data := fixture(t, name)
			m, err := Decode(data)
			require.NoError(t, err)
			got, err := Encode(m)
			require.NoError(t, err)
			assert.Equal(t, string(data), string(got))
			env, err := EncodeEnv(m)
			require.NoError(t, err)
			back, err := DecodeEnv(env)
			require.NoError(t, err)
			assert.Equal(t, m, back)
		})
	}
}

// TestTheFixtureOfI30 is the 32 KiB manifest of I30: its env value is
// the cap exactly, which EncodedSize reports and which DecodeEnv still
// reads; one byte more of JSON passes the cap, which EncodeEnv refuses
// to write and DecodeEnv to read.
func TestTheFixtureOfI30(t *testing.T) {
	m, err := Decode(fixture(t, "size-32k.json"))
	require.NoError(t, err)
	size, err := EncodedSize(m)
	require.NoError(t, err)
	assert.Equal(t, MaxEncodedSize, size)
	assert.Equal(t, 32*1024, MaxEncodedSize)
	env, err := EncodeEnv(m)
	require.NoError(t, err)
	assert.Len(t, env, size)

	m.Agent += "x"
	size, err = EncodedSize(m)
	require.NoError(t, err)
	assert.Greater(t, size, MaxEncodedSize)
	_, err = EncodeEnv(m)
	require.ErrorIs(t, err, ErrTooLarge)
	data, err := Encode(m)
	require.NoError(t, err)
	_, err = DecodeEnv(base64.RawURLEncoding.EncodeToString(data))
	require.ErrorIs(t, err, ErrInvalid)
}

func TestDecodeEnvRefuses(t *testing.T) {
	data := fixture(t, "valid.json")
	for _, c := range []struct {
		name  string
		value string
	}{
		{"padding", base64.URLEncoding.EncodeToString(data[:len(data)-1])},
		{"the standard alphabet", strings.ReplaceAll(strings.ReplaceAll(base64.RawURLEncoding.EncodeToString(data), "-", "+"), "_", "/") + "+/"},
		{"a space", " " + base64.RawURLEncoding.EncodeToString(data)},
		{"a newline", base64.RawURLEncoding.EncodeToString(data) + "\n"},
		{"non-zero trailing bits", base64.RawURLEncoding.EncodeToString(data)[:len(base64.RawURLEncoding.EncodeToString(data))-1] + "_"},
		{"empty", ""},
		{"a valid value over the cap", base64.RawURLEncoding.EncodeToString(append(data, bytes.Repeat([]byte(" "), MaxJSONSize)...))},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			_, err := DecodeEnv(c.value)
			require.Error(t, err)
		})
	}
}

// FuzzDecode feeds the decoder hostile bytes: it never panics, and a
// document it accepts encodes to canonical JSON that decodes to the
// same manifest and encodes to the same bytes again.
func FuzzDecode(f *testing.F) {
	for _, name := range []string{"valid.json", "valid-unknown-member.json", "invalid-duplicate-member.json"} {
		data, err := os.ReadFile("testdata/" + name)
		require.NoError(f, err)
		f.Add(data)
	}
	f.Add([]byte(`{"schema":"manifest/v1","protocol":1,"run":{"a":[1e400]}}`))
	f.Add([]byte(`{"schema":"manifest/v9","protocol":1}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := Decode(data)
		if err != nil {
			return
		}
		enc, err := Encode(m)
		if err != nil {
			require.ErrorIs(t, err, canon.ErrInvalidText)
			return
		}
		back, err := Decode(enc)
		require.NoError(t, err)
		again, err := Encode(back)
		require.NoError(t, err)
		assert.Equal(t, string(enc), string(again))
	})
}
