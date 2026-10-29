// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassify classifies every top-level field of Project but schema,
// with the widening fields of 03 3.2 and the three ungated ones (I7).
func TestClassify(t *testing.T) {
	got, err := Classify()
	require.NoError(t, err)
	gates, applies := map[string]string{}, map[string]string{}
	for _, c := range got {
		gates[c.Name], applies[c.Name] = c.Gate, c.Apply
	}
	assert.Equal(t, map[string]string{
		"name": "widening", "repos": "widening", "workload": "widening", "agent": "widening", "kits": "widening",
		"sandboxOptions": "", "secrets": "widening", "memory": "", "egress": "widening", "ports": "widening", "salvage": "widening", "run": "",
	}, gates)
	assert.Equal(t, map[string]string{
		"name": "recreate", "repos": "recreate", "workload": "recreate", "agent": "recreate", "kits": "recreate",
		"sandboxOptions": "recreate", "secrets": "recreate", "memory": "live", "egress": "live", "ports": "recreate", "salvage": "live", "run": "live",
	}, applies)
}

// TestClassifyFailsOnAFieldOnNeither fails the classification of a type
// with a field that carries no gate tag and is not on the list, and of a
// list that names a tagged field, so a forgotten tag is a generator
// failure, not a silent strip (I7).
//
//romeu:invariant I7 test
func TestClassifyFailsOnAFieldOnNeither(t *testing.T) {
	type spec struct {
		Schema    string `json:"schema"`
		Gated     string `json:"gated" gate:"widening" apply:"recreate"`
		Forgotten string `json:"forgotten" apply:"live"`
		Listed    string `json:"listed" apply:"live"`
	}
	typ := reflect.TypeFor[spec]()
	_, err := classify(typ, []string{"listed"})
	require.ErrorContains(t, err, "forgotten")
	_, err = classify(typ, []string{"listed", "forgotten", "gated"})
	require.Error(t, err)
	_, err = classify(typ, []string{"listed", "forgotten"})
	require.NoError(t, err)

	type badTags struct {
		Odd string `json:"odd" gate:"maybe" apply:"live"`
	}
	_, err = classify(reflect.TypeFor[badTags](), nil)
	require.ErrorContains(t, err, "gate tag")
	type noApply struct {
		Bare string `json:"bare" gate:"widening"`
	}
	_, err = classify(reflect.TypeFor[noApply](), nil)
	require.ErrorContains(t, err, "apply tag")
}

// TestClassifyFailsOnAMemberOfAnUngatedField fails the classification of
// a type whose ungated field gains a member the committed list does not
// name, at any depth (I7).
func TestClassifyFailsOnAMemberOfAnUngatedField(t *testing.T) {
	type inner struct {
		C string `json:"c"`
	}
	type opts struct {
		A string  `json:"a"`
		B []inner `json:"b"`
	}
	type spec struct {
		Schema string `json:"schema"`
		Opts   opts   `json:"opts" apply:"recreate"`
	}
	typ := reflect.TypeFor[spec]()
	_, err := classify(typ, []string{"opts", "opts.a", "opts.b"})
	require.ErrorContains(t, err, "opts.b[].c")
	_, err = classify(typ, []string{"opts", "opts.a", "opts.b", "opts.b[].c"})
	require.NoError(t, err)
}

// TestTagNamesAgree holds the yaml and json names of every field of the
// formats' types equal, so the schema and the decoder read one member.
func TestTagNamesAgree(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var visit func(t2 reflect.Type)
	visit = func(t2 reflect.Type) {
		for t2.Kind() == reflect.Pointer || t2.Kind() == reflect.Slice || t2.Kind() == reflect.Map {
			t2 = t2.Elem()
		}
		if t2.Kind() != reflect.Struct || seen[t2] {
			return
		}
		seen[t2] = true
		for f := range t2.Fields() {
			yname, yopts, _ := strings.Cut(f.Tag.Get("yaml"), ",")
			jname, jopts, _ := strings.Cut(f.Tag.Get("json"), ",")
			assert.Equal(t, jname, yname, "%s.%s", t2.Name(), f.Name)
			assert.Equal(t, jopts == "omitzero", yopts == "omitempty", "%s.%s: optional in both", t2.Name(), f.Name)
			visit(f.Type)
		}
	}
	visit(reflect.TypeFor[Project]())
	visit(reflect.TypeFor[HostSettings]())
	assert.Len(t, seen, 17)
}
