// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import "time"

// Schema is the value of the schema field of the --json document of
// romeu status, the format romeu-status.v1 (REQ-0161).
const Schema = "romeu-status/v1"

// The sandbox states status reports beside the ones sbx prints
// (running, stopped).
const (
	// SandboxAbsent: sbx ls --json lists no sandbox of the project.
	SandboxAbsent = "absent"
	// SandboxUnknown: sbx ls --json failed, so nothing is known.
	SandboxUnknown = "unknown"
)

// The states of a project's record in host state.
const (
	// RecordPresent: the record reads.
	RecordPresent = "present"
	// RecordAbsent: no record, so the project was never synced.
	RecordAbsent = "absent"
	// RecordUnreadable: the record did not read; the block's errors say
	// why.
	RecordUnreadable = "unreadable"
)

// RecordStates are the values of a block's record, in the order above.
var RecordStates = []string{RecordPresent, RecordAbsent, RecordUnreadable}

// Report is the document of romeu status --json (04 4.2, REQ-0327): one
// block per project, in name order. Its schema, romeu-status.v1, is
// generated from this type (tools/ci/internal/schema), with RecordStates
// and ToolchainStates as the values its two closed members admit.
type Report struct {
	// Toolchain is the state of gate 1 on this machine (01 1.4); absent
	// when it was not read.
	Toolchain *Toolchain `json:"toolchain,omitzero"`
	// Warnings are the report's warnings, each by its slug of 04 4.1,
	// such as sbx-untested.
	Warnings []Finding `json:"warnings"`
	// Errors are the reads that failed outside any project's block
	// (REQ-0329): sbx version, the toolchain record.
	Errors []ReadError `json:"errors"`
	// Projects are the blocks of the report.
	Projects []Project `json:"projects"`
	// Schema is "romeu-status/v1".
	Schema string `json:"schema"`
}

// The states of the toolchain acknowledgement (gate 1, 01 1.4).
const (
	// ToolchainAcknowledged: the record holds the live toolchain.
	ToolchainAcknowledged = "acknowledged"
	// ToolchainStale: a member of the live toolchain differs from the
	// record; romeu approve --toolchain acknowledges it.
	ToolchainStale = "stale"
	// ToolchainMissing: no toolchain was ever acknowledged.
	ToolchainMissing = "missing"
	// ToolchainUnknown: a read the comparison needs failed.
	ToolchainUnknown = "unknown"
)

// ToolchainStates are the values of the toolchain's state, in the order
// above.
var ToolchainStates = []string{ToolchainAcknowledged, ToolchainStale, ToolchainMissing, ToolchainUnknown}

// Toolchain is the state of gate 1: status reports a mismatch and does
// not stop on it (REQ-0062).
type Toolchain struct {
	// State is acknowledged, stale, missing or unknown.
	State string `json:"state"`
	// Sbx is the live sbx version; absent when it was not read.
	Sbx string `json:"sbx,omitzero"`
	// Changes are the members whose live value differs from the record,
	// in the order of 03 3.8; every member when none was acknowledged,
	// and an empty list when the record holds the live toolchain.
	Changes []ToolchainChange `json:"changes"`
}

// ToolchainChange is one member of the toolchain whose live value is not
// the recorded one.
type ToolchainChange struct {
	// Member names it by its path in toolchain.json.
	Member string `json:"member"`
	// Recorded is the recorded value; "" when none was acknowledged.
	Recorded string `json:"recorded"`
	// Live is the live value.
	Live string `json:"live"`
}

// Project is the block of one project.
type Project struct {
	// Name is the project's name.
	Name string `json:"name"`
	// Record is the state of its record: present, absent or unreadable.
	Record string `json:"record"`
	// SyncedCommit is the config commit the live render read the spec
	// at; absent before the first promotion.
	SyncedCommit string `json:"syncedCommit,omitzero"`
	// Sandbox is the sandbox's state: what sbx ls --json says (running,
	// stopped), absent, or unknown when the listing failed.
	Sandbox string `json:"sandbox"`
	// Generations are the project's generations, the newest last
	// (01 1.6, REQ-0231).
	Generations []Generation `json:"generations"`
	// Findings are the conditions doctor fails on that status shows in
	// the block, each by its slug of 04 4.1.
	Findings []Finding `json:"findings"`
	// Errors are the reads of the block that failed (REQ-0329).
	Errors []ReadError `json:"errors"`
}

// Generation is one generation of the record (03 3.8).
type Generation struct {
	// ID is the generation's id.
	ID string `json:"id"`
	// State is its state in the generation machine (REQ-0232).
	State string `json:"state"`
	// Adopted says it was adopted, and so not gated (REQ-0233).
	Adopted bool `json:"adopted"`
	// CreatedAt is when it was created or adopted.
	CreatedAt time.Time `json:"createdAt"`
	// ClosedAt is when it closed; absent while it lives, the zero time
	// being left out.
	ClosedAt time.Time `json:"closedAt,omitzero"`
	// Salvage are its salvage records, the newest last.
	Salvage []Salvage `json:"salvage"`
}

// Salvage is one salvage record of a generation (03 3.8, REQ-0237).
type Salvage struct {
	// ID is the salvage's id.
	ID string `json:"id"`
	// CreatedAt is when the record was written.
	CreatedAt time.Time `json:"createdAt"`
	// Result is complete, incomplete or lost.
	Result string `json:"result"`
	// Reasons lists each skipped item and romeu's reasons.
	Reasons []string `json:"reasons"`
	// RefsPrefix is where the salvaged refs live in the clones.
	RefsPrefix string `json:"refsPrefix"`
}

// Finding is a condition of the block, by its slug of 04 4.1.
type Finding struct {
	// Slug names the condition, such as removal-pending.
	Slug string `json:"slug"`
	// Message says what was found and what to run.
	Message string `json:"message"`
}

// ReadError is a read of the block that failed: its error id and slug
// of 04 4.4, what was read, and the error's message (REQ-0329).
type ReadError struct {
	// ID is the error id, such as RJ-308.
	ID string `json:"id"`
	// Slug is the error's slug.
	Slug string `json:"slug"`
	// Read names what was read: the project record, sbx ls.
	Read string `json:"read"`
	// Message is the error's message.
	Message string `json:"message"`
}
