// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The format manifest.v1 (03 3.6), whose Go type lives in
// internal/manifest; tools/ci imports it directly (REQ-0971).
func init() {
	s := schemagen.Schema{Name: "manifest.v1", Type: reflect.TypeFor[manifest.Manifest]()}
	schemagen.Register(s)
	RegisterPage(Page{
		Format: s, Path: "docs/reference/manifest.md", Title: "julieta manifest",
		Reader: "a contributor who changes what romeu passes to julieta, or who reads what julieta acts on",
		Source: "internal/manifest (the Go types)",
		Intro: "The format `manifest.v1`, which romeu passes to julieta on every `sbx env exec` and julieta\n" +
			"caches ([03 3.6](../spec/03-formats.md#36-julieta-manifest-schema-manifestv1)). The rules\n" +
			"that are design decisions, the protocol window and the must-understand rule among them,\n" +
			"are in 03 3.6.",
	})
}
