// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package hygiene

import "github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"

// NameFix is the fix line of a forbidden-name hit (10 10.2, REQ-0863),
// addressed to whoever pushed: an agent, a contributor or the
// maintainer.
const NameFix = "reword this line; after one refused rewrite, or if it is your own identity, hand the location to the maintainer, who re-authors the commit"

// NameFinding returns the finding of a forbidden-name hit at where: the
// rule "name", what holds the name, and NameFix. It repeats no text of
// the hit.
func NameFinding(where, what string) check.Finding {
	return check.Finding{Where: where, Rule: "name", Msg: what + "; " + NameFix}
}
