// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// Seams are the values the package takes from outside, so a test fixes
// them (10 10.4, REQ-0940): the clock, and the source of the random
// bits of ULIDs. Every ULID the package writes comes from canon.NewULID
// over IDs.
type Seams struct {
	// Now returns the time; romeu passes time.Now.
	Now func() time.Time
	// IDs is the process's ULID source, over crypto/rand outside
	// tests.
	IDs *canon.ULIDSource
}

// now returns the time of a record's timestamp: UTC, whole seconds
// (REQ-0162).
func (s Seams) now() time.Time { return stamp(s.Now()) }

// newID returns a new ULID of the instant now.
func (s Seams) newID() (canon.ULID, error) {
	id, err := canon.NewULID(s.Now(), s.IDs)
	if err != nil {
		return canon.ULID{}, fmt.Errorf("state: %w", err)
	}
	return id, nil
}

// Input is what a command knows when it reports an event to the
// generation machine. Each event reads the fields its row needs and
// ignores the others.
type Input struct {
	// WorkspacePath, Repos and RecreateDigest are the record a create
	// or an adopt writes (REQ-0088, REQ-0090). RecreateDigest is the
	// zero value for an adopt, whose digest is unknown.
	WorkspacePath  string
	Repos          []GenerationRepo
	RecreateDigest canon.Sum[canon.RecreateKind]

	// SalvageID is the id of the salvage in progress, the one the event
	// that started it returned in Applied, for the events that end it
	// (REQ-0092, REQ-0095, REQ-1216, REQ-0096).
	SalvageID canon.ULID
	// ManifestSHA256, RefsPrefix, Reasons and AcceptedLoss fill the
	// salvage entry an event writes (REQ-0237).
	ManifestSHA256 SHA256
	RefsPrefix     string
	Reasons        []string
	AcceptedLoss   []string
	// Fingerprint is the fingerprint written with the state removing
	// (REQ-0236).
	Fingerprint canon.Sum[canon.FingerprintKind]
}

// Applied is what Apply did: the row, and the ids it made, which the
// command passes on.
type Applied struct {
	// Row is the transition table's row.
	Row Row
	// GenerationID is the id of the generation a create or an adopt
	// wrote.
	GenerationID canon.ULID
	// SalvageID is the new salvage id of the rows that name one
	// (REQ-0091, REQ-1214, REQ-1217, and the --from-host rows).
	SalvageID canon.ULID
}

// current returns the generation that is not closed, the last one, and
// its state; None and nil when the project has none.
func (p *Project) current() (*Generation, State) {
	if n := len(p.Generations); n > 0 {
		if g := &p.Generations[n-1]; slices.Contains(liveStates, g.State) {
			return g, g.State
		}
	}
	return nil, None
}

// Current returns the state of the project's generation that is not
// closed, or None.
func (p *Project) Current() State {
	_, st := p.current()
	return st
}

// Apply reports an event to the generation machine of the project
// (REQ-0079): it looks the row up for the current state, and an event
// with no row is the error illegal-transition (RJ-350) with the record
// untouched. Otherwise it does the row's effect at the level of the
// record, which each effect case of the tests asserts (REQ-0080); the
// effects at the level of the sandbox and the exit code are the
// command's, named by the row. The caller writes the record by
// Store.UpdateProject, which refuses one in a form no reader would take.
func (p *Project) Apply(seams Seams, event Event, in Input) (Applied, error) {
	g, from := p.current()
	row, err := GenerationMachine.Transition(from, event)
	if err != nil {
		return Applied{}, err
	}
	out := Applied{Row: row}
	switch row.Effect {
	case EffectRecordGeneration, EffectRecordAdopted:
		id, err := seams.newID()
		if err != nil {
			return Applied{}, err
		}
		gen := Generation{
			ID: id, State: row.To, Adopted: row.Effect == EffectRecordAdopted, CreatedAt: seams.now(),
			WorkspacePath: in.WorkspacePath, Repos: slices.Clone(in.Repos),
		}
		if !gen.Adopted {
			gen.CreatedWith = in.RecreateDigest
		}
		p.Generations = append(p.Generations, gen)
		out.GenerationID = id
	case EffectRemoveGeneration:
		p.Generations = p.Generations[:len(p.Generations)-1]
	case EffectKeepRecordExit1, EffectKeepSalvageExit1, EffectResumeAtRm:
		// The record, and the salvage entry in it, stay as they are.
	case EffectNewSalvageID, EffectNewSalvageIDRefsKept, EffectSalvageAgain:
		if out.SalvageID, err = seams.newID(); err != nil {
			return Applied{}, err
		}
		g.State = row.To
	case EffectRecordIncomplete:
		if err = g.addSalvage(seams, in.SalvageID, SalvageIncomplete, []string{ReasonSandboxHalfFailed}, in); err == nil {
			g.State = row.To
		}
	case EffectImportLost:
		if out.SalvageID, err = seams.newID(); err != nil {
			return Applied{}, err
		}
		if err = g.addSalvage(seams, out.SalvageID, SalvageLost, []string{ReasonSandboxLost}, in); err == nil {
			g.State = row.To
		}
	case EffectRecordComplete:
		if err = g.addSalvage(seams, in.SalvageID, SalvageComplete, nil, in); err == nil {
			g.State = row.To
		}
	case EffectRecordIncompleteExit5:
		if err = g.addSalvage(seams, in.SalvageID, SalvageIncomplete, in.Reasons, in); err == nil {
			g.State = row.To
		}
	case EffectWriteRemoving:
		if err = g.writeRemoving(seams, in); err == nil {
			g.State = row.To
		}
	case EffectRemoveEgress, EffectKeepSalvageEgress:
		// The salvage record stays (REQ-0100); the egress rules are
		// gone with the sandbox (REQ-0097).
		g.close(seams, row.To)
		p.EgressApplied = nil
	case EffectPreserveLost:
		if err = g.preserveLost(seams, in); err == nil {
			g.close(seams, row.To)
		}
	default:
		return Applied{}, fmt.Errorf("state: the row %s has the effect %q, which Apply does not know", row.Req, row.Effect)
	}
	if err != nil {
		return Applied{}, err
	}
	return out, nil
}

// close sets the state of a generation to a closed one, at the time of
// the event.
func (g *Generation) close(seams Seams, to State) {
	g.State = to
	g.ClosedAt = seams.now()
}

// errNoSalvageID is the error of an event that ends a salvage and is
// given none: a defect of the caller.
var errNoSalvageID = errors.New("state: the event ends a salvage and Input.SalvageID is empty")

// addSalvage appends the salvage entry of one salvage: the id the
// salvage started with, the result and reasons the row names, and the
// manifest digest and refs prefix the caller read.
func (g *Generation) addSalvage(seams Seams, id canon.ULID, result string, reasons []string, in Input) error {
	if id == (canon.ULID{}) {
		return errNoSalvageID
	}
	g.Salvage = append(g.Salvage, Salvage{
		ID: id, CreatedAt: seams.now(), ManifestSHA256: in.ManifestSHA256, Result: result,
		Reasons: slices.Clone(reasons), RefsPrefix: in.RefsPrefix,
	})
	return nil
}

// writeRemoving is the effect of the row that moves a generation to
// removing (REQ-0096): the fingerprint goes into the salvage entry of
// the salvage that ended, in the same whole-file write as the state
// (REQ-0236). The entry is the newest one when it is the one of
// Input.SalvageID; otherwise this is the first write of the salvage's
// record, which the command made no earlier row write, and it is
// complete, or incomplete with the reasons the accepted loss names.
func (g *Generation) writeRemoving(seams Seams, in Input) error {
	if in.SalvageID == (canon.ULID{}) {
		return errNoSalvageID
	}
	if n := len(g.Salvage); n == 0 || g.Salvage[n-1].ID != in.SalvageID {
		result := SalvageComplete
		if len(in.Reasons) > 0 {
			result = SalvageIncomplete
		}
		if err := g.addSalvage(seams, in.SalvageID, result, in.Reasons, in); err != nil {
			return err
		}
	}
	e := &g.Salvage[len(g.Salvage)-1]
	e.Fingerprint = in.Fingerprint
	e.AcceptedLoss = slices.Clone(in.AcceptedLoss)
	return nil
}

// preserveLost keeps the lost record salvage --from-host wrote for
// this generation, which is its newest entry, and otherwise writes one
// (REQ-0101): the reasons the caller read, or sandbox-lost.
func (g *Generation) preserveLost(seams Seams, in Input) error {
	if n := len(g.Salvage); n > 0 && g.Salvage[n-1].Result == SalvageLost {
		return nil
	}
	id, err := seams.newID()
	if err != nil {
		return err
	}
	reasons := in.Reasons
	if len(reasons) == 0 {
		reasons = []string{ReasonSandboxLost}
	}
	return g.addSalvage(seams, id, SalvageLost, reasons, in)
}
