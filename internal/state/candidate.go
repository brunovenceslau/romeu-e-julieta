// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

// The states of the candidate machine (01 1.6, Candidate). They are
// transient within one command (REQ-0082): none of them is a record.
const (
	CandidateRendered State = "rendered"
	CandidateApproved State = "approved"
	CandidatePromoted State = "promoted"
	CandidateDeclined State = "declined"
)

// The events of the candidate machine.
const (
	// CandidateRender is "sync renders" (REQ-0085).
	CandidateRender Event = "render"
	// CandidateDigestEqual is "widening digest equals the approval"
	// (REQ-1207).
	CandidateDigestEqual Event = "digest-equals-approval"
	// CandidateApprove is "digest differs, TTY, y" (REQ-1208).
	CandidateApprove Event = "approve"
	// CandidateDecline is "digest differs, TTY, other" (REQ-1209).
	CandidateDecline Event = "decline"
	// CandidateDeclineNoTTY is "digest differs, no TTY" (REQ-1210).
	CandidateDeclineNoTTY Event = "decline-no-tty"
	// CandidatePromotionDone is "promotion commit completes" (REQ-1211).
	CandidatePromotionDone Event = "promotion-completes"
)

// The effects of the candidate machine.
const (
	EffectWriteCandidateDir  Effect = "candidate dir written under .romeu/candidates/<candidate-id>/"
	EffectPromote            Effect = "promotion commit"
	EffectRecordApproval     Effect = "approval recorded"
	EffectDeleteCandidateDir Effect = "candidate dir deleted"
	EffectPrintDigestExit3   Effect = "candidate dir deleted; the widening digest printed; exit 3"
)

// CandidateMachine is the candidate machine of 01 1.6 (REQ-0079, REQ-0084): an
// unapproved render under .romeu/candidates/, at most one per project.
// Its table is the source of the candidate diagram under
// docs/diagrams/.
var CandidateMachine = Machine{name: "candidate", about: "The candidate machine of " +
	"[01 1.6](../spec/01-system-model.md#candidate): an unapproved render under `.romeu/candidates/`, " +
	"at most one per project. Its states are transient within one command and are never persisted.", rows: []Row{
	{None, CandidateRender, CandidateRendered, EffectWriteCandidateDir, "REQ-0085"},
	{CandidateRendered, CandidateDigestEqual, CandidatePromoted, EffectPromote, "REQ-1207"},
	{CandidateRendered, CandidateApprove, CandidateApproved, EffectRecordApproval, "REQ-1208"},
	{CandidateRendered, CandidateDecline, CandidateDeclined, EffectDeleteCandidateDir, "REQ-1209"},
	{CandidateRendered, CandidateDeclineNoTTY, CandidateDeclined, EffectPrintDigestExit3, "REQ-1210"},
	{CandidateApproved, CandidatePromotionDone, CandidatePromoted, EffectDeleteCandidateDir, "REQ-1211"},
}}
