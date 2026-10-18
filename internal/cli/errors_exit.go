// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package cli

// ExitMeaning is one exit status of both binaries and what it means (04
// 4.1, Exit codes; Signals).
type ExitMeaning struct {
	Code    int
	Meaning string
}

// ExitCodes are the exit statuses of both binaries, in order: the one
// source of the exit-code section of every --help and of
// docs/reference/exit-codes.md (12 12.3).
var ExitCodes = []ExitMeaning{
	{0, "ok"},
	{1, "the operation failed"},
	{2, "usage or precondition"},
	{3, "approval required"},
	{4, "drift detected, or an interrupted promotion abandoned"},
	{5, "refused to protect data"},
	{6, "the check ran and found what it checks for"},
	{130, "ended by SIGINT"},
	{143, "ended by SIGTERM"},
}

// SignalPrecedence is the sentence of 04 4.1, Signals, item 2, that the
// exit-code sections print after the list.
const SignalPrecedence = "A signal wins over a failure the command had already found (1 to 6); a command that had finished its work keeps 0."
