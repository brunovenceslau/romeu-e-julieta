// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package salvage

import (
	"encoding/json/v2"
	"slices"
	"strconv"
)

// ManifestFile is the salvage manifest of one repo, the last file of
// its salvage dir (08 8.5 step 6, REQ-0708): a salvage dir without it
// is one an interrupted salvage left, which verification ignores and
// keeps.
const ManifestFile = "manifest.json"

// The payload files of a salvage dir besides the bundles (08 8.5 steps
// 4 and 5).
const (
	// IgnoredFile is the archive of the ignored files of the repo.
	IgnoredFile = "ignored.tar.gz"
	// AgentDir is the directory of the agent's state, in the salvage
	// dir of the primary repo: one archive per agent profile,
	// <profile>.tar.gz, since the state's file names are the agent's
	// and the layout of a memory dir admits no dotfile (08 8.1).
	AgentDir = "agent"
)

// ManifestVersion is the version of salvage.v1 this build writes.
const ManifestVersion = 1

// ManifestField is the value of the schema member of a salvage
// manifest, "salvage/v<ManifestVersion>" (03, opening, REQ-0161).
var ManifestField = "salvage/v" + strconv.Itoa(ManifestVersion)

// The skipped reasons of the sandbox half (03 3.11, REQ-0246); the host
// half adds its own.
const (
	ReasonExcludedBySpec      = "excluded-by-spec"
	ReasonOverCap             = "over-cap"
	ReasonTranscriptExcluded  = "transcript-excluded"
	ReasonUnreadable          = "unreadable"
	ReasonDiskFull            = "disk-full"
	ReasonWorktreeChanging    = "worktree-changing"
	ReasonInProgressOperation = "in-progress-operation"
	ReasonAgentNotMatched     = "agent-not-matched"
	ReasonAgentPathMissing    = "agent-path-missing"
)

// acceptedReasons are the reasons that leave a salvage complete
// (REQ-0717): chosen by the spec or by a missing flag.
var acceptedReasons = []string{ReasonExcludedBySpec, ReasonTranscriptExcluded}

// Manifest is the salvage manifest of one repo, the format salvage.v1
// (03 3.11): what the sandbox half wrote into the repo's salvage dir
// memory/<dir>/salvage/<generation>/<salvage-id>/, for the host half to
// verify. Every path in it is relative to the salvage dir, clean and
// has no ".." element (REQ-0252). The fields are in the order of their
// JSON names.
type Manifest struct {
	// AgentStop is what --stop-agents did; absent without it, and in
	// every manifest but the primary repo's.
	AgentStop *AgentStop `json:"agentStop,omitzero"`
	// Bundles are the bundles of the repo and of each nested
	// repository found in its worktrees.
	Bundles []Bundle `json:"bundles"`
	// Complete is advisory: nothing was skipped but for the reasons of
	// the spec or a missing flag. romeu recomputes it (REQ-0717).
	Complete bool `json:"complete"`
	// Files are the payload files besides the bundles.
	Files []File `json:"files"`
	// Generation is the sandbox generation's ULID.
	Generation string `json:"generation"`
	// Refs are the refs of the bundles, each with its object.
	Refs []Ref `json:"refs"`
	// Repo is the repo's dir in the project.
	Repo string `json:"repo"`
	// SalvageID is the id romeu passed in the manifest's salvageRun.
	SalvageID string `json:"salvageId"`
	// Schema is ManifestField.
	Schema string `json:"schema"`
	// Skipped are the items the salvage left out, each with its reason.
	Skipped []Skipped `json:"skipped"`
	// Worktrees are the worktrees the salvage read: the repo's, then
	// those of each nested repository.
	Worktrees []Worktree `json:"worktrees"`
}

// AgentStop is what --stop-agents did (08 8.5 step 1).
type AgentStop struct {
	// Killed is the number of processes that needed SIGKILL.
	Killed int `json:"killed"`
	// Matched is the number of processes signalled.
	Matched int `json:"matched"`
}

// Bundle is one bundle file of the salvage dir.
type Bundle struct {
	// File is its path in the salvage dir.
	File string `json:"file"`
	// Nested is the path of the nested repository it holds, relative
	// to the root of the worktree that holds it; empty for the repo.
	Nested string `json:"nested"`
	// SHA256 is the plain sha256 of its bytes, lowercase hex.
	SHA256 string `json:"sha256"`
	// Size is its size in bytes.
	Size int64 `json:"size"`
}

// File is one payload file of the salvage dir.
type File struct {
	// Path is its path in the salvage dir.
	Path string `json:"path"`
	// SHA256 is the plain sha256 of its bytes, lowercase hex.
	SHA256 string `json:"sha256"`
	// Size is its size in bytes.
	Size int64 `json:"size"`
}

// Ref is one ref of a bundle.
type Ref struct {
	// Nested is the nested repository whose bundle holds it, as
	// Bundle.Nested; empty for the repo's.
	Nested string `json:"nested"`
	// Ref is the full ref name.
	Ref string `json:"ref"`
	// SHA is the object it names, a full object id.
	SHA string `json:"sha"`
}

// Skipped is one item a salvage left out.
type Skipped struct {
	// Detail is what failed, such as git's error, escaped; empty when
	// the reason says it all.
	Detail string `json:"detail"`
	// Item names what was left out: a path of a worktree below its
	// label, an agent path below $HOME, or the agent.
	Item string `json:"item"`
	// Reason is one of the skipped reasons of 03 3.11.
	Reason string `json:"reason"`
}

// Worktree is one worktree the salvage read.
type Worktree struct {
	// Branch is the full ref name its HEAD points at; empty when
	// detached.
	Branch string `json:"branch"`
	// Capture is the salvage ref of the commit that records its
	// changes; empty when it holds none or was skipped.
	Capture string `json:"capture"`
	// Detached says that its HEAD names no branch.
	Detached bool `json:"detached"`
	// Dirty says that it held tracked changes or untracked files.
	Dirty bool `json:"dirty"`
	// Head is the commit its HEAD names; empty on an unborn branch.
	Head string `json:"head"`
	// Location is its absolute path in the sandbox, for the reader;
	// not a path of the salvage dir.
	Location string `json:"location"`
	// Nested is the nested repository it belongs to, as Bundle.Nested.
	Nested string `json:"nested"`
	// Path is its label, worktree-<n>: the main worktree is 0 and the
	// linked ones follow in the order of their absolute paths, so a
	// second read numbers them alike; a nested repository's worktree
	// is <label>/<nested>, below the worktree that holds it. It is
	// relative and clean (REQ-0252).
	Path string `json:"path"`
}

// complete reports whether skipped holds only accepted reasons.
func complete(skipped []Skipped) bool {
	for _, s := range skipped {
		if !slices.Contains(acceptedReasons, s.Reason) {
			return false
		}
	}
	return true
}

// EncodeManifest returns the bytes of m, with its schema set: JSON with
// its members sorted and a final line feed. Nil lists are written as
// empty ones.
func EncodeManifest(m Manifest) ([]byte, error) {
	m.Schema = ManifestField
	m.Complete = complete(m.Skipped)
	m.Bundles = orEmpty(m.Bundles)
	m.Files = orEmpty(m.Files)
	m.Refs = orEmpty(m.Refs)
	m.Skipped = orEmpty(m.Skipped)
	m.Worktrees = orEmpty(m.Worktrees)
	data, err := json.Marshal(m, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// orEmpty returns s, or an empty slice for nil.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
