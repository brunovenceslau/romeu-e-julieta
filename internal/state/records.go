// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
)

// SHA256 is a plain sha256, the one of a file's bytes, which is none of
// the kinds of canon (03 3.12, raw). Its text is 64 lowercase
// hexadecimal digits.
type SHA256 [32]byte

// String returns the digest as lowercase hexadecimal.
func (s SHA256) String() string { return hex.EncodeToString(s[:]) }

// MarshalText returns the digest as String writes it.
func (s SHA256) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText reads 64 lowercase hexadecimal digits, and leaves s as
// it was on an error.
func (s *SHA256) UnmarshalText(text []byte) error {
	if len(text) != hex.EncodedLen(len(s)) {
		return fmt.Errorf("a sha256 is %d hexadecimal digits, not %d bytes of text", hex.EncodedLen(len(s)), len(text))
	}
	var out SHA256
	if _, err := hex.Decode(out[:], text); err != nil {
		return fmt.Errorf("a sha256: %w", err)
	}
	if out.String() != string(text) {
		return fmt.Errorf("a sha256 is in lowercase")
	}
	*s = out
	return nil
}

// Catalog is the acknowledged catalog, decoded, so the next gate 1 can
// print a diff (REQ-0228). Its Go type lands with the package that owns
// the catalog (T-catalog); until then the record holds it as the JSON
// object it is, as manifest.Run holds the run layout.
type Catalog map[string]any

// Capabilities is the parsed, normalized capability set of one
// descriptor (REQ-0240); its Go type lands with internal/oci.
type Capabilities map[string]any

// Toolchain is toolchain.json, the record of gate 1 (03 3.8, REQ-0228):
// what the operator acknowledged on this machine. The format is
// state-toolchain/v1.
type Toolchain struct {
	// Schema is "state-toolchain/v1".
	Schema string `json:"schema"`
	// SbxVersion is the sbx version acknowledged.
	SbxVersion string `json:"sbxVersion"`
	// Romeu is the acknowledged romeu binary.
	Romeu Binary `json:"romeu"`
	// Julieta is the acknowledged julieta binary, the one romeu embeds.
	Julieta JulietaBinary `json:"julieta"`
	// CatalogDigest is the digest of the acknowledged catalog.
	CatalogDigest canon.Sum[canon.CatalogKind] `json:"catalogDigest"`
	// Catalog is the acknowledged catalog itself.
	Catalog Catalog `json:"catalog"`
	// AcknowledgedAt is when gate 1 was answered.
	AcknowledgedAt time.Time `json:"acknowledgedAt"`
}

// Binary is a binary of the toolchain record: its version and the
// sha256 of its bytes.
type Binary struct {
	// Version is the version stamp.
	Version string `json:"version"`
	// SHA256 is the digest of the file.
	SHA256 SHA256 `json:"sha256"`
}

// JulietaBinary is the julieta of the toolchain record: a Binary and the
// protocol number it speaks (REQ-0218).
type JulietaBinary struct {
	// Version is the version stamp.
	Version string `json:"version"`
	// Protocol is the romeu/julieta contract number.
	Protocol int `json:"protocol"`
	// SHA256 is the digest of the file.
	SHA256 SHA256 `json:"sha256"`
}

// Project is projects/<name>.json, the record of one project (03 3.8,
// REQ-0229): its approval, its generations, its applied egress. It hosts
// the generation machine, which Apply drives. The format is
// state-project/v1.
//
// A list that is empty is left out of the file, and so is an absent
// object or time, where the specification's example writes null or [];
// a reader takes either, and the generated schema says what this build
// writes.
type Project struct {
	// Schema is "state-project/v1".
	Schema string `json:"schema"`
	// Approval is the gate 2 approval, absent before the first.
	Approval *Approval `json:"approval,omitzero"`
	// Retiring is present while the retire steps of 01 1.6 run
	// (REQ-0230).
	Retiring *Retiring `json:"retiring,omitzero"`
	// Generations are every generation of the project, the newest last
	// (REQ-0231).
	Generations []Generation `json:"generations,omitempty"`
	// EgressApplied are the domains romeu applied to the sandbox.
	EgressApplied []string `json:"egressApplied,omitempty"`
	// LastRunAt is the time of the last run, the one documented state
	// timestamp (REQ-0277).
	LastRunAt time.Time `json:"lastRunAt,omitzero"`
}

// Approval is the gate 2 approval of a project (REQ-0229, 01 1.4).
type Approval struct {
	// WideningDigest is the digest of the widening set approved.
	WideningDigest canon.Sum[canon.WideningKind] `json:"wideningDigest"`
	// Set is the full widening set (03 3.8, REQ-0229). Its digest is
	// computed again on every read and every write of the record and
	// must equal WideningDigest, the one stored; gate.Build refuses a
	// set that would not read back as itself, so the two agree.
	Set gate.Set `json:"set"`
	// RenderDigest is the digest of the normalized render approved.
	RenderDigest canon.Sum[canon.RenderKind] `json:"renderDigest"`
	// SpecCommit is the config repo commit of the spec approved.
	SpecCommit string `json:"specCommit"`
	// CandidateID is the candidate the approval binds to (I7).
	CandidateID canon.ULID `json:"candidateId"`
	// ApprovedAt is when the approval was given.
	ApprovedAt time.Time `json:"approvedAt"`
}

// Retiring marks a retire in progress (REQ-0230, REQ-0108).
type Retiring struct {
	// Attic is the UTC timestamp of the attic directory.
	Attic string `json:"attic"`
}

// The results of a salvage entry (REQ-0237).
const (
	SalvageComplete   = "complete"
	SalvageIncomplete = "incomplete"
	SalvageLost       = "lost"
)

// The reasons romeu itself writes in a salvage entry (REQ-0237).
const (
	ReasonSandboxHalfFailed = "sandbox-half-failed"
	ReasonSandboxLost       = "sandbox-lost"
)

// Generation is one sandbox lifetime (01 1.6, REQ-0231).
type Generation struct {
	// ID is the generation's ULID.
	ID canon.ULID `json:"id"`
	// State is one of the five states of the generation machine
	// (REQ-0232).
	State State `json:"state"`
	// SalvageID is the id of the salvage in progress, written with the
	// state salvaging (01 1.6, "the state salvaging with the new salvage
	// id") and absent in every other state.
	SalvageID canon.ULID `json:"salvageId,omitzero"`
	// Adopted says the generation was adopted, not created (REQ-0090).
	Adopted bool `json:"adopted"`
	// CreatedAt is when the record was written.
	CreatedAt time.Time `json:"createdAt"`
	// CreatedWith is the recreate digest at create, absent for an
	// adopted generation (REQ-0233).
	CreatedWith canon.Sum[canon.RecreateKind] `json:"createdWith,omitzero"`
	// WorkspacePath is the sandbox's workspace path on the host.
	WorkspacePath string `json:"workspacePath"`
	// Repos is the recorded repo set, which a salvage uses instead of
	// the current spec (REQ-0106).
	Repos []GenerationRepo `json:"repos"`
	// ClosedAt is when the generation closed, absent while it lives.
	ClosedAt time.Time `json:"closedAt,omitzero"`
	// Salvage holds one entry per salvage, the newest last (REQ-0235).
	Salvage []Salvage `json:"salvage,omitempty"`
}

// GenerationRepo is one repo of a generation's recorded set.
type GenerationRepo struct {
	// Dir is the repo's directory name.
	Dir string `json:"dir"`
	// URL is the repo's origin.
	URL string `json:"url"`
	// Primary says whether this is the primary repo.
	Primary bool `json:"primary"`
}

// Salvage is the record of one salvage (03 3.8).
type Salvage struct {
	// ID is the salvage's ULID.
	ID canon.ULID `json:"id"`
	// CreatedAt is when the entry was written.
	CreatedAt time.Time `json:"createdAt"`
	// ManifestSHA256 is the plain sha256 of the salvage manifest.json
	// (REQ-0234).
	ManifestSHA256 SHA256 `json:"manifestSha256"`
	// Result is complete, incomplete or lost (REQ-0237).
	Result string `json:"result"`
	// Reasons lists each skipped item and the reasons of REQ-0237.
	Reasons []string `json:"reasons,omitempty"`
	// AcceptedLoss lists the reasons --accept-loss named.
	AcceptedLoss []string `json:"acceptedLoss,omitempty"`
	// RefsPrefix is the ref namespace the salvage's refs live under.
	RefsPrefix string `json:"refsPrefix"`
	// Fingerprint is written with the state removing (REQ-0236), and
	// absent otherwise.
	Fingerprint canon.Sum[canon.FingerprintKind] `json:"fingerprint,omitzero"`
}

// Descriptor is descriptors/<digest>.json, the parsed, normalized
// capability set of one workload or kit descriptor, keyed by its OCI
// digest (REQ-0240, REQ-0571). Entries are immutable. The format is
// state-descriptor/v1.
type Descriptor struct {
	// Schema is "state-descriptor/v1".
	Schema string `json:"schema"`
	// Digest is the OCI digest of the descriptor, sha256:<64 hex>.
	Digest string `json:"digest"`
	// Capabilities is the normalized capability set.
	Capabilities Capabilities `json:"capabilities"`
}
