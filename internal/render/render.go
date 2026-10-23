// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package render renders a project's candidate (01 1.6, 12 12.2): its
// sbxenv.yaml (03 3.3), its render.json (03 3.7) and the workspace files
// of $ROMEU_ROOT (02 2.3), each written through a marshaller and never a
// template (I11). types.go holds the layout of a candidate, the
// render.json type and the Materializer seam that kit materialization
// fills; the promotion commit copies a candidate to its live paths.
package render

import (
	"context"
	"fmt"
	"os"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// FileMode is the mode of each file render writes into a candidate:
// the directories above it are 0700 (REQ-0141), and nothing else on
// the host reads it but romeu and sbx, as the user.
const FileMode = 0o600

// Input is what a render reads: the validated spec, the host settings
// and HOME, which a secret argv runs with.
type Input struct {
	// Project is the validated project spec, its defaults set, so each
	// repo has its dir and exactly one is primary.
	Project spec.Project
	// Host is the host settings, read for the secret argv of each spec
	// secret, keyed <name>@<project>.
	Host spec.HostSettings
	// Home is the absolute HOME a secret argv runs with (REQ-0208).
	Home string
}

// Rendered is what Render wrote into the candidate.
type Rendered struct {
	// Env is the bytes of the candidate's sbxenv.yaml.
	Env []byte
	// Kits are the kits the Materializer wrote, in the spec's order.
	Kits []MaterializedKit
}

// Render writes the candidate c of in: the kits through m, then
// sbxenv.yaml, which names them. render.json is written after the gate
// has its digests (WriteRecord).
func Render(ctx context.Context, c Candidate, in Input, m Materializer) (Rendered, error) {
	kits, err := m.Materialize(ctx, c, in.Project)
	if err != nil {
		return Rendered{}, fmt.Errorf("render: materialize the kits: %w", err)
	}
	env, err := Env(EnvInput{Input: in, Kits: kits})
	if err != nil {
		return Rendered{}, err
	}
	if err := writeFile(c, CandidateEnvFile, env); err != nil {
		return Rendered{}, err
	}
	return Rendered{Env: env, Kits: kits}, nil
}

// writeFile writes data to name in the candidate dir, through an
// os.Root on it, so no name can leave it.
// It truncates in place, with no atomic rename: a candidate dir is new
// and no live file references it (01 1.6); a live path is written by
// the promotion commit, never here.
func writeFile(c Candidate, name string, data []byte) error {
	r, err := os.OpenRoot(c.Dir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = r.Close() }()
	if err := r.WriteFile(name, data, FileMode); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}
