// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

// The states of the generation machine (01 1.6, Generation, REQ-0232).
// The five of them persist in projects/<name>.json.
const (
	GenerationOpen          State = "open"
	GenerationSalvaging     State = "salvaging"
	GenerationRemoving      State = "removing"
	GenerationClosedRemoved State = "closed-removed"
	GenerationClosedLost    State = "closed-lost"
)

// The events of the generation machine, one for each row of the table
// in 01 1.6.
const (
	// GenCreate: run finds the sandbox absent and is about to create it
	// (REQ-0088).
	GenCreate Event = "create"
	// GenCreateFailedAbsent: the sbx env run of the same command returns
	// non-zero and sbx ls --json then shows no sandbox (REQ-1212).
	GenCreateFailedAbsent Event = "create-failed-absent"
	// GenCreateFailedPresent: the same, and sbx ls --json shows the
	// sandbox (REQ-1213).
	GenCreateFailedPresent Event = "create-failed-present"
	// GenCreateTimeout: the sbx env run times out, whatever sbx ls shows
	// (REQ-0089).
	GenCreateTimeout Event = "create-timeout"
	// GenAdopt: adopt on a TTY, workspace path matches (REQ-0090).
	GenAdopt Event = "adopt"
	// GenSalvageStart: salvage, rm, recreate or retire starts the
	// sandbox half (REQ-0091).
	GenSalvageStart Event = "salvage-start"
	// GenSalvageRestart: the same command starts the sandbox half
	// again, on a generation a killed command left salvaging (REQ-1214).
	GenSalvageRestart Event = "salvage-restart"
	// GenSandboxHalfFailed: the sandbox half fails or its exec is
	// interrupted (REQ-0092).
	GenSandboxHalfFailed Event = "sandbox-half-failed"
	// GenSalvageFromHost: salvage --from-host, the sandbox absent
	// (REQ-0094, REQ-1215).
	GenSalvageFromHost Event = "salvage-from-host"
	// GenSalvageVerified: salvage verified, the command was salvage
	// (REQ-0095).
	GenSalvageVerified Event = "salvage-verified"
	// GenSalvageIncomplete: salvage incomplete, no --accept-loss
	// (REQ-1216).
	GenSalvageIncomplete Event = "salvage-incomplete"
	// GenSalvageThenRemove: salvage complete or --accept-loss, and the
	// command removes the sandbox (REQ-0096).
	GenSalvageThenRemove Event = "salvage-complete-remove"
	// GenRemoved: sbx env rm succeeds (REQ-0097).
	GenRemoved Event = "removed"
	// GenRemoveFailed: sbx env rm fails (REQ-0098).
	GenRemoveFailed Event = "remove-failed"
	// GenResumeRemoval: the sandbox is present on a removing generation
	// and a fresh fingerprint equals the newest one written with
	// removing (REQ-0099).
	GenResumeRemoval Event = "resume-removal"
	// GenFingerprintDiffers: the same, and the fingerprint differs
	// (REQ-1217).
	GenFingerprintDiffers Event = "fingerprint-differs"
	// GenSandboxAbsent: the sandbox is found absent (REQ-0100, REQ-0101,
	// REQ-1218); the row depends on the state it is found in.
	GenSandboxAbsent Event = "sandbox-absent"
)

// The effects of the generation machine.
const (
	EffectRecordGeneration      Effect = "generation record written: id, recreate digest, workspace path, repo set"
	EffectRemoveGeneration      Effect = "the record just written removed; exit 1"
	EffectKeepRecordExit1       Effect = "the record kept; exit 1"
	EffectRecordAdopted         Effect = "generation record written with adopted: true and an unknown recreate digest"
	EffectNewSalvageID          Effect = "new salvage id"
	EffectNewSalvageIDRefsKept  Effect = "new salvage id; the refs the earlier salvage created stay"
	EffectRecordIncomplete      Effect = "salvage record result: incomplete, reasons: [sandbox-half-failed]"
	EffectImportLost            Effect = "refs and snapshot bundles imported under a new salvage id; salvage record result: lost, reasons: [sandbox-lost]"
	EffectRecordComplete        Effect = "salvage record result: complete"
	EffectRecordIncompleteExit5 Effect = "salvage record result: incomplete with reasons; exit 5"
	EffectWriteRemoving         Effect = "state written, with the salvage entry's fingerprint, before sbx env rm"
	EffectRemoveEgress          Effect = "egress rules removed"
	EffectKeepSalvageExit1      Effect = "the existing salvage record kept; exit 1"
	EffectResumeAtRm            Effect = "the existing salvage record kept; the command resumes at sbx env rm"
	EffectSalvageAgain          Effect = "new salvage id; the salvaging rows decide how that salvage ends"
	EffectKeepSalvageEgress     Effect = "the existing salvage record kept; egress rules removed"
	EffectPreserveLost          Effect = "keeps a lost record salvage --from-host wrote for this generation, else writes one: result lost"
)

// GenerationMachine is the generation machine of 01 1.6 (REQ-0079): one
// sandbox lifetime. Its table is the source of the generation diagram
// under docs/diagrams/.
var GenerationMachine = Machine{name: "generation", about: "The generation machine of " +
	"[01 1.6](../spec/01-system-model.md#generation): one sandbox lifetime. Its five states persist " +
	"in `projects/<name>.json`.", rows: []Row{
	{None, GenCreate, GenerationOpen, EffectRecordGeneration, "REQ-0088"},
	{GenerationOpen, GenCreateFailedAbsent, None, EffectRemoveGeneration, "REQ-1212"},
	{GenerationOpen, GenCreateFailedPresent, GenerationOpen, EffectKeepRecordExit1, "REQ-1213"},
	{GenerationOpen, GenCreateTimeout, GenerationOpen, EffectKeepRecordExit1, "REQ-0089"},
	{None, GenAdopt, GenerationOpen, EffectRecordAdopted, "REQ-0090"},
	{GenerationOpen, GenSalvageStart, GenerationSalvaging, EffectNewSalvageID, "REQ-0091"},
	{GenerationSalvaging, GenSalvageRestart, GenerationSalvaging, EffectNewSalvageIDRefsKept, "REQ-1214"},
	{GenerationSalvaging, GenSandboxHalfFailed, GenerationOpen, EffectRecordIncomplete, "REQ-0092"},
	{GenerationOpen, GenSalvageFromHost, GenerationOpen, EffectImportLost, "REQ-0094"},
	{GenerationSalvaging, GenSalvageFromHost, GenerationSalvaging, EffectImportLost, "REQ-1215"},
	{GenerationSalvaging, GenSalvageVerified, GenerationOpen, EffectRecordComplete, "REQ-0095"},
	{GenerationSalvaging, GenSalvageIncomplete, GenerationOpen, EffectRecordIncompleteExit5, "REQ-1216"},
	{GenerationSalvaging, GenSalvageThenRemove, GenerationRemoving, EffectWriteRemoving, "REQ-0096"},
	{GenerationRemoving, GenRemoved, GenerationClosedRemoved, EffectRemoveEgress, "REQ-0097"},
	{GenerationRemoving, GenRemoveFailed, GenerationRemoving, EffectKeepSalvageExit1, "REQ-0098"},
	{GenerationRemoving, GenResumeRemoval, GenerationRemoving, EffectResumeAtRm, "REQ-0099"},
	{GenerationRemoving, GenFingerprintDiffers, GenerationSalvaging, EffectSalvageAgain, "REQ-1217"},
	{GenerationRemoving, GenSandboxAbsent, GenerationClosedRemoved, EffectKeepSalvageEgress, "REQ-0100"},
	{GenerationOpen, GenSandboxAbsent, GenerationClosedLost, EffectPreserveLost, "REQ-0101"},
	{GenerationSalvaging, GenSandboxAbsent, GenerationClosedLost, EffectPreserveLost, "REQ-1218"},
}}
