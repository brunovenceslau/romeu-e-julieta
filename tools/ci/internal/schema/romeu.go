// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package schema

import (
	"reflect"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cmd/romeu/all"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// The --json document of romeu doctor (04 4.2), whose Go type lives in
// its command package; tools/ci reaches it through
// internal/cmd/romeu/all (REQ-0971). The schema admits the four results
// of a check alone (REQ-0331).
func init() {
	results := make([]any, 0, len(all.DoctorResults))
	for _, r := range all.DoctorResults {
		results = append(results, r)
	}
	schemagen.Register(schemagen.Schema{Name: "romeu-doctor.v1", Type: reflect.TypeFor[all.DoctorReport](),
		Keywords: []schemagen.Keyword{{Path: "checks[].result", Values: map[string]any{"enum": results}}}})
}
