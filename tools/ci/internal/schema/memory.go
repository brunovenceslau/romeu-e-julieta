// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The format memory-entry.v1 (03 3.9), the front matter of a memory
// entry, whose Go type lives in internal/memstore, and the --json
// document of the julieta memory commands (04 4.3); tools/ci reaches
// both through internal/cmd/julieta/all (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "memory-entry.v1", Type: reflect.TypeFor[all.MemoryEntry]()})
	schemagen.Register(schemagen.Schema{Name: "julieta-memory.v1", Type: reflect.TypeFor[all.MemoryReport]()})
}
