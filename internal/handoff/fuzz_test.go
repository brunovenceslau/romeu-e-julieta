// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff_test

import (
	"bytes"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/handoff"
)

// FuzzDecode is the fuzz target of the handoff file decoder (10 10.1,
// Fuzz). Its seeds are committed under testdata/fuzz/FuzzDecode, the
// fixtures of testdata among them. Decode never panics, and a file it
// accepts encodes, and its encoding decodes and encodes to the same
// bytes again.
func FuzzDecode(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		h, err := handoff.Decode(data)
		if err != nil {
			return
		}
		enc, err := handoff.Encode(h)
		if err != nil {
			t.Fatalf("a file Decode accepts does not encode: %v", err)
		}
		again, err := handoff.Decode(enc)
		if err != nil {
			t.Fatalf("the encoding of an accepted file does not decode: %v", err)
		}
		enc2, err := handoff.Encode(again)
		if err != nil || !bytes.Equal(enc, enc2) {
			t.Fatalf("encode and decode do not agree: %v", err)
		}
	})
}
