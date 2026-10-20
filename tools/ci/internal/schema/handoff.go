// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The format handoff.v1 (03 3.10), the front matter of a handoff file,
// whose Go type lives in internal/handoff, and the --json document of
// julieta handoff write and list (04 4.3); tools/ci reaches both through
// internal/cmd/julieta/all (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "handoff.v1", Type: reflect.TypeFor[all.HandoffFront]()})
	schemagen.Register(schemagen.Schema{Name: "julieta-handoff.v1", Type: reflect.TypeFor[all.HandoffReport]()})
}
