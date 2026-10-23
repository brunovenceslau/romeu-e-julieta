// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package egress_test

import (
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
)

// FuzzParseLock is the fuzz target of the mise.lock reader (10 10.1,
// Fuzz). Its seeds are committed under testdata/fuzz/FuzzParseLock: a
// lock of the pinned layout, a duplicate table, an unknown top-level
// key, and brackets and dotted keys past the limits. ParseLock never
// panics, reads what it accepts the same way twice, and returns no
// tool with an error.
func FuzzParseLock(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		lock, err := egress.ParseLock(data)
		if err != nil {
			if len(lock.Tools) != 0 {
				t.Fatalf("a lock refused with %v came with tools", err)
			}
			return
		}
		again, err := egress.ParseLock(data)
		if err != nil || len(again.Tools) != len(lock.Tools) {
			t.Fatalf("a lock read twice gave two results: %v", err)
		}
	})
}
