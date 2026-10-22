// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// The forms of the values validate checks.
var (
	// commitForm is a commit id of 40 or 64 lowercase hexadecimal digits.
	commitForm = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	// ociDigest is the OCI digest of a descriptor.
	ociDigest = regexp.MustCompile(`^sha256:([0-9a-f]{64})$`)
)

// generationStates are the states a record may hold (REQ-0232).
var generationStates = []State{GenerationOpen, GenerationSalvaging, GenerationRemoving, GenerationClosedRemoved, GenerationClosedLost}

// liveStates are the states of a generation that is not closed.
var liveStates = []State{GenerationOpen, GenerationSalvaging, GenerationRemoving}

// validate checks what decoding does not: a member a reader needs is
// there, and each value has its form. A member that is missing decodes
// as its zero value, so the zero value is what each check refuses.
func (t *Toolchain) validate() error {
	var errs []error
	if t.SbxVersion == "" {
		errs = append(errs, errors.New("sbxVersion is missing"))
	}
	if t.Romeu.Version == "" || t.Romeu.SHA256 == (SHA256{}) {
		errs = append(errs, errors.New("romeu needs a version and a sha256"))
	}
	if t.Julieta.Version == "" || t.Julieta.SHA256 == (SHA256{}) || t.Julieta.Protocol < 1 {
		errs = append(errs, errors.New("julieta needs a version, a protocol of 1 or more and a sha256"))
	}
	if t.CatalogDigest == (canon.Sum[canon.CatalogKind]{}) {
		errs = append(errs, errors.New("catalogDigest is missing"))
	}
	if t.Catalog == nil {
		errs = append(errs, errors.New("catalog is missing"))
	}
	if t.AcknowledgedAt.IsZero() {
		errs = append(errs, errors.New("acknowledgedAt is missing"))
	}
	return errors.Join(errs...)
}

// validate checks a project record: each generation, and that at most
// one of them is not closed, since a project has one sandbox.
func (p *Project) validate() error {
	var errs []error
	if a := p.Approval; a != nil {
		if a.WideningDigest == (canon.Sum[canon.WideningKind]{}) || a.RenderDigest == (canon.Sum[canon.RenderKind]{}) ||
			a.CandidateID == (canon.ULID{}) || a.ApprovedAt.IsZero() || a.Set == nil || !commitForm.MatchString(a.SpecCommit) {
			errs = append(errs, errors.New("approval is not whole: it needs both digests, the set, a spec commit, a candidate id and a time"))
		}
	}
	if p.Retiring != nil && p.Retiring.Attic == "" {
		errs = append(errs, errors.New("retiring has no attic"))
	}
	live := 0
	var seen []canon.ULID
	for i := range p.Generations {
		g := &p.Generations[i]
		if err := g.validate(); err != nil {
			errs = append(errs, fmt.Errorf("generations[%d]: %w", i, err))
		}
		if slices.Contains(liveStates, g.State) {
			live++
		}
		if slices.Contains(seen, g.ID) {
			errs = append(errs, fmt.Errorf("generations[%d]: the id %s is the id of an earlier generation", i, g.ID))
		}
		seen = append(seen, g.ID)
	}
	if live > 1 {
		errs = append(errs, fmt.Errorf("%d generations are not closed, and a project has one sandbox", live))
	}
	return errors.Join(errs...)
}

// validate checks one generation.
func (g *Generation) validate() error {
	var errs []error
	if g.ID == (canon.ULID{}) {
		errs = append(errs, errors.New("id is missing"))
	}
	if !slices.Contains(generationStates, g.State) {
		errs = append(errs, fmt.Errorf("state %q is not a generation state", clip(string(g.State))))
	}
	if g.CreatedAt.IsZero() {
		errs = append(errs, errors.New("createdAt is missing"))
	}
	if g.WorkspacePath == "" {
		errs = append(errs, errors.New("workspacePath is missing"))
	}
	if len(g.Repos) == 0 {
		errs = append(errs, errors.New("repos is empty"))
	}
	closed := !slices.Contains(liveStates, g.State)
	if closed != !g.ClosedAt.IsZero() && slices.Contains(generationStates, g.State) {
		errs = append(errs, errors.New("closedAt is set exactly when the state is closed"))
	}
	for i := range g.Salvage {
		if err := g.Salvage[i].validate(); err != nil {
			errs = append(errs, fmt.Errorf("salvage[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// validate checks one salvage entry.
func (s *Salvage) validate() error {
	var errs []error
	if s.ID == (canon.ULID{}) {
		errs = append(errs, errors.New("id is missing"))
	}
	if s.CreatedAt.IsZero() {
		errs = append(errs, errors.New("createdAt is missing"))
	}
	if !slices.Contains([]string{SalvageComplete, SalvageIncomplete, SalvageLost}, s.Result) {
		errs = append(errs, fmt.Errorf("result %q is none of complete, incomplete and lost", clip(s.Result)))
	}
	if s.RefsPrefix == "" {
		errs = append(errs, errors.New("refsPrefix is missing"))
	}
	return errors.Join(errs...)
}

// validate checks a descriptor record.
func (d *Descriptor) validate() error {
	var errs []error
	if !ociDigest.MatchString(d.Digest) {
		errs = append(errs, fmt.Errorf("digest %q is not sha256:<64 lowercase hex>", clip(d.Digest)))
	}
	if d.Capabilities == nil {
		errs = append(errs, errors.New("capabilities is missing"))
	}
	return errors.Join(errs...)
}

// stamp returns t in UTC at whole seconds, the form of every timestamp
// romeu writes (REQ-0162).
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }
