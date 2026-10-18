// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"fmt"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// pinWorkload rewrites the workload reference from its argument and
// reads no registry (06 6.2); the home written by hand stays.
func pinWorkload(p *pins.Pins, ref string) (string, error) {
	if !pins.WorkloadRef.MatchString(ref) {
		return "", fmt.Errorf("%q is not <registry>/<repository>@sha256:<64 hex> (03 3.1)", ref)
	}
	w := pins.Workload{Ref: ref}
	if p.Workload != nil {
		w.Home = p.Workload.Home
	}
	p.Workload = &w
	return "workload: " + ref, nil
}
