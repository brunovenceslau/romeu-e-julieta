// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package memstore_test

import (
	"bytes"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// FuzzDecode is the fuzz target of the memory entry decoder (10 10.1,
// Fuzz). Its seeds are committed under testdata/fuzz/FuzzDecode, the
// fixtures of testdata among them. Decode never panics, and an entry it
// accepts encodes, and its encoding decodes and encodes to the same
// bytes again.
func FuzzDecode(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := memstore.Decode(data)
		if err != nil {
			return
		}
		enc, err := memstore.Encode(e)
		if err != nil {
			t.Fatalf("an entry Decode accepts does not encode: %v", err)
		}
		again, err := memstore.Decode(enc)
		if err != nil {
			t.Fatalf("the encoding of an accepted entry does not decode: %v", err)
		}
		if !bytes.Equal(enc, mustEncode(t, again)) {
			t.Fatalf("encode and decode do not agree")
		}
	})
}

// mustEncode returns the file of e.
func mustEncode(t *testing.T, e memstore.Entry) []byte {
	t.Helper()
	enc, err := memstore.Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	return enc
}
