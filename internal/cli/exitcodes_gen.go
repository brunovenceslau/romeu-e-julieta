// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

// exitCodesHelp is the exit-code section of every --help. "go generate
// ./..." writes this file from ExitCodes (generate_errors.go); do not
// edit it by hand.
const exitCodesHelp = "Exit codes:\n" +
	"  0    ok\n" +
	"  1    the operation failed\n" +
	"  2    usage or precondition\n" +
	"  3    approval required\n" +
	"  4    drift detected, or an interrupted promotion abandoned\n" +
	"  5    refused to protect data\n" +
	"  6    the check ran and found what it checks for\n" +
	"  130  ended by SIGINT\n" +
	"  143  ended by SIGTERM\n" +
	"A signal wins over a failure the command had already found (1 to 6); a command that had finished its work keeps 0.\n"
