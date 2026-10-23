// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
)

// WriteSettings writes h as the host settings of the Env, mode 0600,
// after it fills what h leaves empty: the schema, the Root of the Env
// and the two tools of Bin. It decodes what it wrote by the rules of
// 03 3.4, so a test never starts from settings romeu would refuse.
func (e *Env) WriteSettings(tb testing.TB, h all.HostSettings) {
	tb.Helper()
	if h.Schema == "" {
		h.Schema = "host-settings/v1"
	}
	if h.Root == "" {
		h.Root = e.Root
	}
	if h.Tools.Sbx == "" {
		h.Tools.Sbx = e.Sbx
	}
	if h.Tools.Git == "" {
		h.Tools.Git = e.Git
	}
	data, err := all.EncodeHostSettings(h)
	if err != nil {
		tb.Fatal(err)
	}
	if _, err := all.DecodeHostSettings(data); err != nil {
		tb.Fatalf("the settings the test wrote are invalid: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(e.Settings()), 0o700); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(e.Settings(), data, 0o600); err != nil {
		tb.Fatal(err)
	}
}

// ReadSettings returns the host settings of the Env as romeu reads them.
func (e *Env) ReadSettings(tb testing.TB) all.HostSettings {
	tb.Helper()
	data, err := os.ReadFile(e.Settings())
	if err != nil {
		tb.Fatal(err)
	}
	h, err := all.DecodeHostSettings(data)
	if err != nil {
		tb.Fatal(err)
	}
	return h
}
