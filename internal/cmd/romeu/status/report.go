// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package status

import "time"

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

// Report is the document of romeu status --json (04 4.2, REQ-0327): one
// block per project, in name order. Its schema, romeu-status.v1, is
// generated once the report is whole (T048, the last slice).
type Report struct {
	// Projects are the blocks of the report.
	Projects []Project `json:"projects"`
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
	// ClosedAt is when it closed; absent while it lives.
	ClosedAt *time.Time `json:"closedAt,omitzero"`
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
