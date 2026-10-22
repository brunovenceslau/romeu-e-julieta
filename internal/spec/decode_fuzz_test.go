// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"os"
	"reflect"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// stringSetters returns, for each string of v, a function that sets it:
// each string field of a struct, at any depth, each string of a list,
// and each key and each string value of a map, so every string field of
// a format is reached (I11).
func stringSetters(v reflect.Value) []func(string) {
	var out []func(string)
	var visit func(v reflect.Value, commit func())
	visit = func(v reflect.Value, commit func()) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				visit(v.Elem(), commit)
			}
		case reflect.String:
			if v.CanSet() {
				out = append(out, func(s string) { v.SetString(s); commit() })
			}
		case reflect.Struct:
			for _, field := range v.Fields() {
				visit(field, commit)
			}
		case reflect.Slice:
			for i := range v.Len() {
				visit(v.Index(i), commit)
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				out = append(out, func(s string) {
					val := v.MapIndex(k)
					v.SetMapIndex(k, reflect.Value{})
					v.SetMapIndex(reflect.ValueOf(s).Convert(k.Type()), val)
					commit()
				})
				// A map value is not addressable: set a copy, then put it
				// back.
				tmp := reflect.New(v.Type().Elem()).Elem()
				tmp.Set(v.MapIndex(k))
				visit(tmp, func() { v.SetMapIndex(k, tmp); commit() })
			}
		}
	}
	visit(v, func() {})
	return out
}

// roundTrip writes doc with the YAML marshaller and reads it back with
// the strict decoder of f, without the validators: each string field
// must come back as it was written (I11).
func roundTrip[T any](t *testing.T, f format, doc *T) {
	t.Helper()
	data, err := encode(doc)
	if err != nil {
		t.Fatalf("the marshaller refuses a document: %v", err)
	}
	var back T
	if err := decode(data, f, &back); err != nil {
		t.Fatalf("the strict decoder refuses what the marshaller wrote: %v\n%s", err, data)
	}
	require.Equal(t, *doc, back, "a string does not round-trip:\n%s", data)
}

// FuzzYAMLRoundTrip sets each string field of the example spec and the
// example host settings, one at a time, to the fuzzed string, writes the
// document with the YAML marshaller and reads it back with the strict
// decoder: the document is the same (05 5.2, I11: YAML only via
// marshallers, a fuzz round-trip of every spec string field). Its seeds
// are committed under testdata/fuzz/FuzzYAMLRoundTrip.
//
//romeu:invariant I11 test
func FuzzYAMLRoundTrip(f *testing.F) {
	projectData, err := os.ReadFile("testdata/valid-project.yaml")
	if err != nil {
		f.Fatal(err)
	}
	hostData, err := os.ReadFile("testdata/valid-host-settings.yaml")
	if err != nil {
		f.Fatal(err)
	}
	for _, s := range []string{"", "~", "null", "true", "1", "0x1", "1e3", "- a", "a: b", "#", "'", "\"", "\n", " a ", "<<", "*a", "&a", "!t", "%", "@", "`", "\u2028", "\ufeff", "a\x00b", "\t", "\n0", "\n", " a", "-"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			return
		}
		var p0 Project
		if err := decode(projectData, project, &p0); err != nil {
			t.Fatal(err)
		}
		for i := range stringSetters(reflect.ValueOf(&p0)) {
			var p Project
			if err := decode(projectData, project, &p); err != nil {
				t.Fatal(err)
			}
			stringSetters(reflect.ValueOf(&p))[i](s)
			p.Schema = project.Field()
			roundTrip(t, project, &p)
		}
		var h0 HostSettings
		if err := decode(hostData, hostSettings, &h0); err != nil {
			t.Fatal(err)
		}
		for i := range stringSetters(reflect.ValueOf(&h0)) {
			var h HostSettings
			if err := decode(hostData, hostSettings, &h); err != nil {
				t.Fatal(err)
			}
			stringSetters(reflect.ValueOf(&h))[i](s)
			h.Schema = hostSettings.Field()
			roundTrip(t, hostSettings, &h)
		}
	})
}

// FuzzDecodeProject is the fuzz target of the strict decoder of a
// project spec: it never panics, and a spec it accepts, written again by
// EncodeProject, is accepted and writes the same bytes again; the bytes
// are compared, since an empty list and a left-out one read alike.
// Its seeds are committed under testdata/fuzz/FuzzDecodeProject.
func FuzzDecodeProject(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := DecodeProject(data)
		if err != nil {
			return
		}
		again, err := EncodeProject(p)
		require.NoError(t, err, "an accepted spec does not encode")
		back, err := DecodeProject(again)
		require.NoError(t, err, "the encoding of an accepted spec is refused:\n%s", again)
		twice, err := EncodeProject(back)
		require.NoError(t, err)
		require.Equal(t, string(again), string(twice), "an accepted spec does not read the same after a round trip")
	})
}
