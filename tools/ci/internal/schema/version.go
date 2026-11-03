// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The --json document of romeu version (04 4.1, 04 4.2), whose Go type
// lives in its command package; tools/ci reaches it through
// internal/cmd/romeu/all (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "romeu-version.v1", Type: reflect.TypeFor[all.VersionReport]()})
}
