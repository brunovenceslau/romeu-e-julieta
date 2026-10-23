// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
)

// FuzzParseLock is the fuzz target of the mise.lock parser (10 10.1,
// Fuzz). Its seeds are committed under testdata/fuzz/FuzzParseLock, the
// fixtures of testdata among them. ParseLock never panics, and a lock
// it accepts is read the same way twice and checks without a panic.
func FuzzParseLock(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		lock, err := mise.ParseLock(data)
		if err != nil {
			return
		}
		again, err := mise.ParseLock(data)
		if err != nil || len(again.Tools) != len(lock.Tools) {
			t.Fatalf("a lock read twice gave two results: %v", err)
		}
		_ = mise.Check(mise.Config{Tools: []string{"go"}}, lock, nil)
	})
}

// FuzzParseConfig is the fuzz target of the mise.toml reader (10 10.1,
// Fuzz; the audit of 2026-10-22). Its seeds are committed under
// testdata/fuzz/FuzzParseConfig: the repository's own mise.toml, a
// duplicate key, and arrays and inline tables nested past the cap.
// ParseConfig never panics, reads what it accepts the same way twice,
// and refuses nesting past MaxDepth.
func FuzzParseConfig(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		cfg, err := mise.ParseConfig(data)
		if err != nil {
			return
		}
		if mise.CheckShape(data) != nil {
			t.Fatalf("a mise.toml nested past the cap was read")
		}
		again, err := mise.ParseConfig(data)
		if err != nil || len(again.Tools) != len(cfg.Tools) {
			t.Fatalf("a mise.toml read twice gave two results: %v", err)
		}
	})
}
