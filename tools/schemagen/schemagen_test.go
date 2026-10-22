// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schemagen

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixture is the generator's fixture type: one field of each kind the
// generator expresses.
type Fixture struct {
	Schema   string            `json:"schema"`
	Name     string            `json:"name"`
	Count    int               `json:"count,omitempty"`
	Size     uint32            `json:"size,omitzero"`
	Ratio    float64           `json:"ratio,omitempty"`
	On       bool              `json:"on"`
	Tags     []string          `json:"tags,omitempty"`
	Pair     [2]int            `json:"pair,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
	Child    *FixtureChild     `json:"child,omitempty"`
	Raw      []byte            `json:"raw,omitempty"`
	Blob     FixtureBlob       `json:"blob,omitempty"`
	Hash     [4]byte           `json:"hash,omitzero"`
	Level    FixtureLevel      `json:"level,omitzero"`
	At       time.Time         `json:"createdAt"`
	Addr     netip.Addr        `json:"addr,omitzero"`
	Any      any               `json:"any,omitempty"`
	Untagged string
	Skipped  string `json:"-"`
	hidden   string
	FixtureEmbedded
	Promoted FixturePromoted `json:",embed"`
}

// FixtureBlob is a named []byte, a base64 string as []byte is.
type FixtureBlob []byte

// FixtureLevel decodes from text alone, so it is a string.
type FixtureLevel int

// UnmarshalText reads a level.
func (l *FixtureLevel) UnmarshalText(b []byte) error {
	*l = FixtureLevel(len(b))
	return nil
}

// FixtureChild is a nested object of Fixture.
type FixtureChild struct {
	Dir string `json:"dir"`
}

// FixtureEmbedded is embedded in Fixture without a JSON name, so its
// members are Fixture's.
type FixtureEmbedded struct {
	Embedded string `json:"embedded,omitempty"`
}

// FixturePromoted is promoted into Fixture by the embed option.
type FixturePromoted struct {
	Promoted string `json:"promoted,omitempty"`
}

// fixture is the format of the fixture type.
var fixture = Schema{Name: "fixture.v1", Type: reflect.TypeFor[Fixture]()}

// fixtureSchema is the document the generator writes for the fixture
// type, with the comment "header".
const fixtureSchema = `{
  "$comment": "header",
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "additionalProperties": false,
  "properties": {
    "Untagged": {
      "type": "string"
    },
    "addr": {
      "type": "string"
    },
    "any": {},
    "blob": {
      "contentEncoding": "base64",
      "type": "string"
    },
    "child": {
      "additionalProperties": false,
      "properties": {
        "dir": {
          "type": "string"
        }
      },
      "required": [
        "dir"
      ],
      "type": "object"
    },
    "count": {
      "type": "integer"
    },
    "createdAt": {
      "format": "date-time",
      "type": "string"
    },
    "embedded": {
      "type": "string"
    },
    "hash": {
      "contentEncoding": "base64",
      "type": "string"
    },
    "labels": {
      "additionalProperties": {
        "type": "string"
      },
      "type": "object"
    },
    "level": {
      "type": "string"
    },
    "name": {
      "type": "string"
    },
    "on": {
      "type": "boolean"
    },
    "pair": {
      "items": {
        "type": "integer"
      },
      "maxItems": 2,
      "minItems": 2,
      "type": "array"
    },
    "promoted": {
      "type": "string"
    },
    "ratio": {
      "type": "number"
    },
    "raw": {
      "contentEncoding": "base64",
      "type": "string"
    },
    "schema": {
      "const": "fixture/v1",
      "type": "string"
    },
    "size": {
      "minimum": 0,
      "type": "integer"
    },
    "tags": {
      "items": {
        "type": "string"
      },
      "type": "array"
    }
  },
  "required": [
    "Untagged",
    "createdAt",
    "name",
    "on",
    "schema"
  ],
  "title": "fixture.v1",
  "type": "object"
}
`

// TestGenerateTheFixture writes the schema of the fixture type: every
// kind, the JSON names, the required members, the constant schema
// field, and the sorted members, one per line.
func TestGenerateTheFixture(t *testing.T) {
	got, err := Generate(fixture, "header")
	require.NoError(t, err)
	assert.JSONEq(t, fixtureSchema, string(got))
	assert.Equal(t, strings.Split(fixtureSchema, "\n"), strings.Split(string(got), "\n"), "the layout: one member per line, sorted")
	again, err := Generate(fixture, "header")
	require.NoError(t, err)
	assert.Equal(t, string(got), string(again), "the same document on every run")
	assert.Equal(t, "schemas/fixture.v1.json", fixture.Path())
	assert.Equal(t, "fixture/v1", fixture.Field())
	assert.Equal(t, "github.com/brunovenceslau/romeu-e-julieta/tools/schemagen.Fixture", fixture.Source())

	noComment, err := Generate(fixture, "")
	require.NoError(t, err)
	assert.NotContains(t, string(noComment), "$comment")
}

// TestTheFixtureSchemaValidates compiles the generated document as JSON
// Schema 2020-12 and shows that it admits a document of the fixture
// type and refuses an unknown member, a missing required member, a
// wrong schema field and a wrong type.
func TestTheFixtureSchemaValidates(t *testing.T) {
	doc, err := Generate(fixture, "header")
	require.NoError(t, err)
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{})
	require.NoError(t, c.AddResource("https://schemas.invalid/fixture.v1.json", parsed))
	sch, err := c.Compile("https://schemas.invalid/fixture.v1.json")
	require.NoError(t, err)

	good, err := jsonv2.Marshal(Fixture{hidden: "not a member", Schema: "fixture/v1", Name: "a", Child: &FixtureChild{Dir: "d"}, Labels: map[string]string{"k": "v"}})
	require.NoError(t, err)
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(good))
	require.NoError(t, err)
	require.NoError(t, sch.Validate(inst), "%s", good)

	for name, bad := range map[string]string{
		"unknown member": strings.Replace(string(good), `"name":"a"`, `"name":"a","extra":1`, 1),
		"missing name":   strings.Replace(string(good), `"name":"a",`, ``, 1),
		"other schema":   strings.Replace(string(good), `"fixture/v1"`, `"fixture/v2"`, 1),
		"wrong type":     strings.Replace(string(good), `"on":false`, `"on":"no"`, 1),
	} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			require.NotEqual(t, string(good), bad, "the fixture changes the document")
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(bad))
			require.NoError(t, err)
			assert.Error(t, sch.Validate(inst))
		})
	}
}

// Types the generator refuses, each with the error that names it.
type (
	withChan      struct{ C chan int }
	withIntMap    struct{ M map[int]string }
	withMethods   struct{ S interface{ M() } }
	withMarshaler struct{ J jsontext.Value }
	withOption    struct {
		N int `json:"n,string"`
	}
	withDup struct {
		A string `json:"B"`
		B string
	}
	withEmbedOption struct {
		S string `json:",embed"`
	}
	withBadElem        struct{ L []chan int }
	withDuration       struct{ D time.Duration }
	withUnmarshaler    struct{ U fixtureUnmarshaler }
	recursiveMap       map[string]recursiveMap
	recursiveSlice     []recursiveSlice
	withRecursiveMap   struct{ M recursiveMap }
	withRecursiveSlice struct{ L recursiveSlice }
	noSchemaField      struct{ Name string }
	intSchemaField     struct {
		Schema int `json:"schema"`
	}
	withBadValue struct{ M map[string]chan int }
	withBadInner struct{ E withChan }
	withBadEmbed struct{ withChan }
)

// TestGenerateRefuses shows each type the generator cannot express
// failing with the field it reached.
func TestGenerateRefuses(t *testing.T) {
	for _, c := range []struct {
		name string
		typ  reflect.Type
		want string
	}{
		{"chan", reflect.TypeFor[withChan](), "withChan.C: the kind chan"},
		{"int map", reflect.TypeFor[withIntMap](), "withIntMap.M: a map whose key is not a string"},
		{"interface", reflect.TypeFor[withMethods](), "withMethods.S: an interface with methods"},
		{"marshaler", reflect.TypeFor[withMarshaler](), "withMarshaler.J: a type with its own JSON methods"},
		{"option", reflect.TypeFor[withOption](), `withOption.N: the json option "string"`},
		{"duplicate", reflect.TypeFor[withDup](), `withDup.B: the JSON name "B" is used twice`},
		{"embed option", reflect.TypeFor[withEmbedOption](), "withEmbedOption.S: embed on a field that is not a struct"},
		{"element", reflect.TypeFor[withBadElem](), "withBadElem.L[]: the kind chan"},
		{"map value", reflect.TypeFor[withBadValue](), "withBadValue.M[key]: the kind chan"},
		{"inner", reflect.TypeFor[withBadInner](), "withBadInner.E.C: the kind chan"},
		{"embedded", reflect.TypeFor[withBadEmbed](), "withBadEmbed.C: the kind chan"},
		{"duration", reflect.TypeFor[withDuration](), "withDuration.D: time.Duration, which encoding/json/v2 has no default form for"},
		{"unmarshaler", reflect.TypeFor[withUnmarshaler](), "withUnmarshaler.U: a type with its own JSON methods"},
		{"recursive map", reflect.TypeFor[withRecursiveMap](), "withRecursiveMap.M[key]: the recursive type schemagen.recursiveMap"},
		{"recursive slice", reflect.TypeFor[withRecursiveSlice](), "withRecursiveSlice.L[]: the recursive type schemagen.recursiveSlice"},
		{"no schema field", reflect.TypeFor[noSchemaField](), "noSchemaField has no schema string field (REQ-0161)"},
		{"schema field of another type", reflect.TypeFor[intSchemaField](), "intSchemaField has no schema string field (REQ-0161)"},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			_, err := Generate(Schema{Name: "x.v1", Type: c.typ}, "")
			require.Error(t, err)
			assert.ErrorContains(t, err, "x.v1: "+c.want)
		})
	}
}

// TestRegister shows the registry sorted by name, and each registration
// that is a programming error panicking.
func TestRegister(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })
	registry = nil
	Register(Schema{Name: "zeta.v1", Type: reflect.TypeFor[Fixture]()})
	Register(fixture)
	assert.Equal(t, []string{"fixture.v1", "zeta.v1"}, names(Registered()))
	assert.PanicsWithValue(t, `schemagen: "fixture.v1" is registered twice`, func() { Register(fixture) })
	assert.PanicsWithValue(t, `schemagen: "Fixture" is not a schema name of the form <name>.v<n>`, func() {
		Register(Schema{Name: "Fixture", Type: fixture.Type})
	})
	assert.PanicsWithValue(t, `schemagen: the type of "s.v1" is not a named struct`, func() {
		Register(Schema{Name: "s.v1", Type: reflect.TypeFor[string]()})
	})
	assert.PanicsWithValue(t, `schemagen: the type of "n.v1" is not a named struct`, func() { Register(Schema{Name: "n.v1"}) })
}

// names returns the names of schemas.
func names(schemas []Schema) []string {
	var out []string
	for _, s := range schemas {
		out = append(out, s.Name)
	}
	return out
}

// fixtureUnmarshaler has its own UnmarshalJSON, so its form is its code's.
type fixtureUnmarshaler struct{}

// UnmarshalJSON accepts anything.
func (*fixtureUnmarshaler) UnmarshalJSON([]byte) error { return nil }
