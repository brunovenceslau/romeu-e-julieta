// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package hygiene

import "github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"

// The never-tracked rule of 10 10.2 (REQ-0883; REQ-0825 in the pushed
// range): a tracked or pushed path whose last element is on the
// neverTracked list of tools/ci/denylist.yaml, in any case
// (names.List.NeverTracked), such as a person's own working agreement.
const (
	// NeverTrackedRule names the rule in a finding.
	NeverTrackedRule = "never-tracked"
	// NeverTrackedMsg is the reason the rule gives; it repeats no path.
	NeverTrackedMsg = "the file belongs outside product repositories (the neverTracked list of " + names.Path + ")"
)
