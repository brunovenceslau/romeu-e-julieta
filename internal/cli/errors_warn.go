// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

import "fmt"

// Warn prints the warning slug with its message to w, the CLI writer of
// stderr: "warning <slug>: <message>", on one line. A warning changes no exit code,
// and its slug is an id-less row of the list of 04 4.1, Warnings; a
// test reads every call and fails a slug that is not a literal of that
// list.
func Warn(w *Writer, slug, message string) {
	_, _ = fmt.Fprintf(w, "warning %s: %s\n", slug, oneLine(message))
}
