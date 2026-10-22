// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import "slices"

// Placeholder is a type of value that varies from call to call. A
// session under e2e/testdata/sbx/<version>/ holds its Token in place of
// the value (10 10.3, Replay only), the fake sbx matches an argv
// against it, and a builder gives a value to its slot only when Check
// accepts it, since no string from outside becomes an argv element
// without a validator (04 4.1, Output, REQ-0271).
//
// The tokens are the placeholder types the sessions use; a recorder
// (T018) writes the same tokens, and a placeholder parses as the type it
// stands for, so a recorded listing still runs through the parsers.
type Placeholder struct {
	// Token is what a session holds, such as "<name>".
	Token string
	// What says what the placeholder stands for.
	What string
	// Check returns why a value is not of the type, or nil.
	Check func(string) error
}

// placeholders is the list of the placeholder types, in the order the
// sessions use them.
var placeholders = []Placeholder{}

// PlaceholderTokens returns the tokens of the placeholder types, which
// are the only angle-bracket words an argv in a session may hold.
func PlaceholderTokens() []string {
	out := make([]string, 0, len(placeholders))
	for _, p := range placeholders {
		out = append(out, p.Token)
	}
	return slices.Clone(out)
}
