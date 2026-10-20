// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/julieta/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The --json documents of julieta setup and julieta status (04 4.3),
// whose Go types live in their command packages; tools/ci reaches them
// through internal/cmd/julieta/all (REQ-0971).
func init() {
	schemagen.Register(schemagen.Schema{Name: "julieta-setup.v1", Type: reflect.TypeFor[all.SetupReport]()})
	schemagen.Register(schemagen.Schema{Name: "julieta-status.v1", Type: reflect.TypeFor[all.StatusReport]()})
}
