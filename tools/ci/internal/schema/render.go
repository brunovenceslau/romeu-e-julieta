// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The format render.v1 (03 3.7), render.json, whose Go type lives in
// internal/render; tools/ci reaches it through internal/cmd/romeu/all
// (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "render.v1", Type: reflect.TypeFor[all.RenderRecord]()})
}
