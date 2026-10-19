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
	schemagen.Register(schemagen.Schema{Name: "manifest.v1", Type: reflect.TypeFor[manifest.Manifest]()})
}
