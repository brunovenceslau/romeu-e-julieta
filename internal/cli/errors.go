// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

// The rows of the error table (04 4.4) that this package raises.
var (
	// CheckFindings is the error check-findings (RJ-328): a check ran
	// and found what it checks for, exit 6 (04 4.1, Exit codes;
	// REQ-0435): romeu doctor, julieta lock --check, julieta pin check,
	// julieta spec validate and julieta memory check raise it, and the
	// details list each finding by its slug. It is declared here, once,
	// since the commands of both binaries raise it.
	CheckFindings = Row{
		ID:          "RJ-328",
		Slug:        "check-findings",
		Exit:        6,
		Fix:         "decide: fix each finding the details name, then run the check again",
		RecoveredIn: "none: the findings name the paths to fix",
	}

	// SubprocessTimeout is the error subprocess-timeout (RJ-323): a
	// subprocess ran past the limit of its class (04 4.1, Subprocesses,
	// item 5; REQ-0300). It is declared here, once, since every package
	// that starts a subprocess raises it; internal/procgroup returns
	// *procgroup.TimeoutError with the same id and a caller wraps it in
	// this row.
	SubprocessTimeout = Row{
		ID:          "RJ-323",
		Slug:        "subprocess-timeout",
		Exit:        1,
		Fix:         "decide: the sandbox or the daemon may still be busy, so look at sbx ls --json and run the command again once it is idle",
		RecoveredIn: "none: the command runs again once the sandbox is idle",
	}

	// LockUnparsed is the error lock-unparsed (RJ-325): a mise.lock or a
	// mise.toml that does not parse with the pinned layout, an unknown
	// top-level key among them (07 7.5, step 1, REQ-0629). romeu sync
	// raises it for a lock it reads from a commit, julieta install for the
	// files of a repo; julieta lock --check reports the same reading as a
	// finding. The hint is filled with the repo's dir.
	LockUnparsed = Row{
		ID:          "RJ-325",
		Slug:        "lock-unparsed",
		Exit:        2,
		Fix:         "julieta lock --repo <dir>",
		RecoveredIn: "none: relocking writes a lock of the pinned layout",
	}

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
