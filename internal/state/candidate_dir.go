// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// Candidate is one candidate of the candidate machine: an unapproved
// render under .romeu/candidates/<candidate-id>/ (01 1.6, REQ-0084). It
// lives within one command (REQ-0082); nothing about it is a record
// but the approval it leads to.
type Candidate struct {
	id    canon.ULID
	state State
	root  string
}

// RenderCandidate is the event "sync renders" (REQ-0085): it deletes
// the candidate dir a killed command left, since the next sync deletes
// it and renders again (REQ-0082) and at most one candidate exists
// (REQ-0084), then makes the dir of a new candidate, mode 0700, below
// the candidates directory root. render writes the candidate's files
// into Path.
func RenderCandidate(root string, seams Seams) (*Candidate, error) {
	row, err := CandidateMachine.Transition(None, CandidateRender)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("state: %w", err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			return nil, fmt.Errorf("state: %w", err)
		}
	}
	id, err := seams.newID()
	if err != nil {
		return nil, err
	}
	c := &Candidate{id: id, state: row.To, root: root}
	if err := os.MkdirAll(c.Path(), DirMode); err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	return c, nil
}

// ID returns the candidate's id, the name of its dir and the binding of
// an approval to it (I7).
func (c *Candidate) ID() canon.ULID { return c.id }

// State returns the candidate's state.
func (c *Candidate) State() State { return c.state }

// Path returns the candidate's dir.
func (c *Candidate) Path() string { return filepath.Join(c.root, c.id.String()) }

// Apply reports an event to the candidate machine (REQ-0079). An event
// with no row for the candidate's state is the error illegal-transition
// (RJ-350), and nothing changes. Otherwise it does the effect of the row
// at the level of the state: the approval is recorded into p, which
// the caller writes (REQ-1208), and the candidate dir is deleted
// (REQ-1209, REQ-1210, REQ-1211). The promotion commit of REQ-1207 is
// the ordered steps of 01 1.6, which the render package runs, and the
// exit and the printed digest of REQ-1210 are the command's. p and a are
// read for the event CandidateApprove alone.
func (c *Candidate) Apply(event Event, p *Project, a Approval) (Row, error) {
	row, err := CandidateMachine.Transition(c.state, event)
	if err != nil {
		return Row{}, err
	}
	switch row.Effect {
	case EffectRecordApproval:
		if p == nil || a.CandidateID != c.id {
			return Row{}, fmt.Errorf("state: an approval binds to the candidate %s, and needs a project record", c.id)
		}
		p.Approval = &a
	case EffectDeleteCandidateDir, EffectPrintDigestExit3:
		if err := os.RemoveAll(c.Path()); err != nil {
			return Row{}, fmt.Errorf("state: %w", err)
		}
	}
	c.state = row.To
	return row, nil
}
