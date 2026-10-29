// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The format salvage.v1 (03 3.11), the salvage manifest of one repo,
// whose Go type lives in internal/salvage, and the --json document of
// julieta salvage (04 4.3); tools/ci reaches both through
// internal/cmd/julieta/all (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "salvage.v1", Type: reflect.TypeFor[all.SalvageManifest]()})
	schemagen.Register(schemagen.Schema{Name: "julieta-salvage.v1", Type: reflect.TypeFor[all.SalvageReport]()})
}
