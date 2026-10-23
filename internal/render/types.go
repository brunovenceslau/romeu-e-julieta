// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"context"
	"path"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The layout of <name>-env/ that render writes (02 2.3), each path
// relative to that directory and slash-separated: the live files, which
// a promotion writes, and the directory of the candidates.
const (
	// LiveEnvFile is the live sbxenv.yaml, the commit point of a
	// promotion (01 1.6).
	LiveEnvFile = "sbxenv.yaml"
	// LiveRecord is the live render.json (03 3.7).
	LiveRecord = ".romeu/render.json"
	// LiveKitsDir holds the approved kit dirs, each
	// <kit>-<digest12>, content-addressed and immutable.
	LiveKitsDir = ".romeu/kits"
	// BinDir holds the julieta binary and SHA256SUMS, mounted
	// read-only into the sandbox at the same relative path (06 6.3).
	BinDir = ".romeu/bin"
	// CandidatesDir holds the one unapproved candidate, in a dir named
	// by its id (01 1.6, REQ-0084).
	CandidatesDir = ".romeu/candidates"
)

// The layout of one candidate dir, .romeu/candidates/<candidate-id>/
// (02 2.3): the rendered files under the names below, which the
// promotion commit copies to their live paths. Each is relative to the
// candidate dir.
const (
	// CandidateEnvFile is the candidate's sbxenv.yaml.
	CandidateEnvFile = "sbxenv.yaml"
	// CandidateRecord is the candidate's render.json.
	CandidateRecord = "render.json"
	// CandidateKitsDir holds the candidate's materialized kits, each
	// in a dir of the name MaterializedKit.Dir returns.
	CandidateKitsDir = "kits"
)

// Candidate is the dir of one candidate, as internal/state made it
// (state.Candidate.Path). render writes the candidate's files into it
// and nothing else; a live file never references it.
type Candidate struct {
	// Dir is the absolute path of the candidate dir.
	Dir string
}

// MaterializedKit is one kit of the spec as the Materializer wrote it
// into the candidate's kits dir: what sbxenv.yaml names, and what the
// widening set holds of it (01 1.4).
type MaterializedKit struct {
	// ID is the kit's id: the product or personal name of the spec.
	ID string
	// Digest is the kit-tree digest of the kit's files (03 3.12).
	Digest canon.Sum[canon.KitTreeKind]
	// Rendered are the args romeu renders into the kit, such as
	// julietaBin (06 6.4), beside the args of the spec; a spec arg of
	// the same name is an error of RenderEnv.
	Rendered map[string]string
	// Capabilities is the normalized capability set of the kit's
	// descriptor, which the gate shows.
	Capabilities oci.Capabilities
}

// Dir returns the name of the kit's dir, <kit>-<digest12>, under the
// candidate's kits dir and under LiveKitsDir (01 1.6).
func (k MaterializedKit) Dir() string { return k.ID + "-" + k.Digest.String()[:12] }

// LivePath returns the kit dir's path in <name>-env/, the key of
// render.json files.
func (k MaterializedKit) LivePath() string { return path.Join(LiveKitsDir, k.Dir()) }

// Materializer is the seam kit materialization fills (T038): it writes
// each kit of the validated spec p into the candidate's kits dir, the
// product kits from the embed seam and the personal kits from the
// config commit's tree, and returns them in the spec's order. The
// promotion commit (T039) copies those dirs to LiveKitsDir.
type Materializer interface {
	Materialize(ctx context.Context, c Candidate, p spec.Project) ([]MaterializedKit, error)
}

// RecordSchema is the schema field of render.json (03 3.7, REQ-0161).
const RecordSchema = "render/v1"

// Record is render.json (03 3.7, schema render.v1): where the spec of
// the render came from, the commit each repo's egress was derived at,
// the digest of each file render wrote, the derived egress and the
// digests the gate compares. A candidate holds one; the live one sits
// at LiveRecord, and the drift check reads its Files (01 1.5).
type Record struct {
	// Schema is RecordSchema.
	Schema string `json:"schema"`
	// RomeuVersion is the version stamp of the romeu that rendered it.
	RomeuVersion string `json:"romeuVersion"`
	// Promoting is set inside a promotion commit alone (REQ-0226), and
	// absent otherwise.
	Promoting *Promoting `json:"promoting,omitzero"`
	// SpecSource is the config commit the spec was read at.
	SpecSource SpecSource `json:"specSource"`
	// Repos are the spec's repos, in its order.
	Repos []RecordRepo `json:"repos"`
	// Files maps the path of each file or kit dir render wrote,
	// relative to <name>-env/, to its digest: the plain sha256 of a
	// file (REQ-0256), the kit-tree digest of a kit dir.
	Files map[string]string `json:"files"`
	// Egress is the derived egress (07 7.5).
	Egress RecordEgress `json:"egress"`
	// WideningDigest is the digest of the widening set (01 1.4).
	WideningDigest canon.Sum[canon.WideningKind] `json:"wideningDigest"`
	// RecreateDigest is the digest of the fields that need a recreate.
	RecreateDigest canon.Sum[canon.RecreateKind] `json:"recreateDigest"`
	// CatalogDigest is the digest of the egress catalog (gate 1).
	CatalogDigest canon.Sum[canon.CatalogKind] `json:"catalogDigest"`
}

// Promoting is the promotion marker of render.json (REQ-0226): the
// candidate being promoted, the digest of its files, and the files map
// before the promotion, which recovery writes back (REQ-0227).
type Promoting struct {
	// CandidateID is the id of the candidate being promoted.
	CandidateID canon.ULID `json:"candidateId"`
	// FilesDigest is the digest of the candidate dir's files.
	FilesDigest canon.Sum[canon.CandidateFilesKind] `json:"filesDigest"`
	// PreviousFiles is the Files of the record before the promotion.
	PreviousFiles map[string]string `json:"previousFiles"`
}

// SpecSourceOrigin is the kind of a spec read from the config repo's
// origin, at a romeu-owned ref.
const SpecSourceOrigin = "origin"

// SpecSource is the commit of the config repo a render read the spec at.
type SpecSource struct {
	// Kind is where the commit came from: SpecSourceOrigin.
	Kind string `json:"kind"`
	// Ref is the romeu-owned ref the commit was read from.
	Ref string `json:"ref"`
	// Commit is the commit's object id.
	Commit string `json:"commit"`
}

// RecordRepo is one repo of render.json: the commit its egress was
// derived at (07 7.5) and the sha256 of its mise.lock there.
type RecordRepo struct {
	// Dir is the repo's dir in <name>-env/.
	Dir string `json:"dir"`
	// URL is the repo's url in the spec.
	URL string `json:"url"`
	// EgressRef is the romeu-owned ref egressCommit was resolved from.
	EgressRef string `json:"egressRef"`
	// EgressCommit is the commit the egress was derived at.
	EgressCommit string `json:"egressCommit"`
	// LockSha256 is the sha256 of mise.lock at EgressCommit, absent
	// when the repo has none.
	LockSha256 string `json:"lockSha256,omitzero"`
}

// RecordEgress is the derived egress of render.json: the domains allowed
// at once, those that wait for an approval, and the egress digest of
// their union (REQ-0633).
type RecordEgress struct {
	// Auto are the domains allowed at once, sorted.
	Auto []string `json:"auto"`
	// Gated are the domains that wait for an approval, sorted.
	Gated []GatedDomain `json:"gated"`
	// EffectiveDigest is canon.Digest(canon.Egress, the sorted union).
	EffectiveDigest canon.Sum[canon.EgressKind] `json:"effectiveDigest"`
}

// GatedDomain is one gated domain and why it is gated.
type GatedDomain struct {
	// Domain is the domain.
	Domain string `json:"domain"`
	// Reason is why the domain waits for an approval.
	Reason egress.Reason `json:"reason"`
}

// EgressOf returns the egress of render.json for the derived egress r,
// with its effective digest.
func EgressOf(r egress.Result) (RecordEgress, error) {
	d, err := r.Digest()
	if err != nil {
		return RecordEgress{}, err
	}
	out := RecordEgress{Auto: r.Auto, Gated: make([]GatedDomain, 0, len(r.Gated)), EffectiveDigest: d}
	for _, g := range r.Gated {
		out.Gated = append(out.Gated, GatedDomain{Domain: g.Domain, Reason: g.Reason})
	}
	return out, nil
}
