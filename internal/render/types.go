// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"context"
	"path"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
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
