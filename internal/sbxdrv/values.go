// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package sbxdrv

import (
	"fmt"
	"strings"
	"unicode"
)

// noControl refuses a value with a control character (unicode.IsControl:
// C0, DEL and C1), a newline among them, which no argv element or
// environment value of romeu holds.
func noControl(v string) error {
	if i := strings.IndexFunc(v, unicode.IsControl); i >= 0 {
		return fmt.Errorf("holds a control character at byte %d", i)
	}
	return nil
}
