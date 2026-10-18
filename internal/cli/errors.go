// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

// The rows of the error table (04 4.4) that this package raises.
var (
	// Internal is the error internal (RJ-348): an error chain that holds
	// no *Error, a defect of the code (10 10.1, Error ids).
	Internal = Row{
		ID:          "RJ-348",
		Slug:        "internal",
		Exit:        1,
		Fix:         "decide: open an issue with the details",
		RecoveredIn: "none: a defect of the code, fixed in a release",
	}

	// Usage is the error usage (RJ-349): a command, subcommand, flag or
	// argument the definitions do not have (04 4.1, Parsing). Its hint
	// names the help of the binary, or of the command when one was named.
	Usage = Row{
		ID:          "RJ-349",
		Slug:        "usage",
		Exit:        2,
		Fix:         "<binary> <command> --help",
		RecoveredIn: "none: the help the hint names lists the commands and flags",
	}
)
