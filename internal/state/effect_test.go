// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// effectKey is the key an Effect case registers (REQ-0080).
type effectKey struct {
	machine string
	from    State
	event   Event
}

// effectCases are the hand-written cases of REQ-0080, each registered
// under the (from, event) key of the row whose Effect it asserts at the
// level of the state (01 1.6).
var effectCases = map[effectKey]func(*effectCase){}

// effectCase is what an Effect case runs with: the test of its row, and
// the seams a test fixes (10 10.4).
type effectCase struct {
	T     *testing.T
	Seams Seams
}

// genCase is what the Effect case of a generation row runs with: the
// project in the row's from state, and the id of the salvage in progress
// when that state is salvaging.
type genCase struct {
	T  *testing.T
	S  Seams
	P  *Project
	ID canon.ULID
}

// effect registers the case of one row.
func effect(m Machine, from State, event Event, fn func(c *effectCase)) {
	k := effectKey{m.Name(), from, event}
	if _, dup := effectCases[k]; dup {
		panic("two effect cases for " + m.Name() + " " + string(from) + " " + string(event))
	}
	effectCases[k] = fn
}

// TestEffects runs the effect case of every row, and fails when the
// registered keys differ from the rows of the tables (REQ-0080).
func TestEffects(t *testing.T) {
	t.Run("every transition row has an Effect case", func(t *testing.T) {
		var want, got []effectKey
		for _, m := range []Machine{CandidateMachine, GenerationMachine} {
			for _, r := range m.Rows() {
				want = append(want, effectKey{m.Name(), r.From, r.Event})
			}
		}
		for k := range effectCases {
			got = append(got, k)
		}
		assert.ElementsMatch(t, want, got, "the registered (from, event) keys are the table's rows")
	})
	for _, m := range []Machine{CandidateMachine, GenerationMachine} {
		for _, r := range m.Rows() {
			fn, ok := effectCases[effectKey{m.Name(), r.From, r.Event}]
			if !ok {
				continue
			}
			t.Run(m.Name()+"/"+r.Req+" "+string(r.From)+" "+string(r.Event), func(t *testing.T) { fn(&effectCase{T: t, Seams: testSeams()}) })
		}
	}
}

// Candidate rows (01 1.6, Candidate).
func init() {
	approval := func(c *Candidate) Approval {
		a := approvalFor("shop", c.ID())
		a.ApprovedAt = testNow
		return a
	}
	rendered := func(t *testing.T, s Seams) (*Candidate, string) {
		t.Helper()
		root := filepath.Join(t.TempDir(), ".romeu", "candidates")
		c, err := RenderCandidate(root, s)
		require.NoError(t, err)
		return c, root
	}
	effect(CandidateMachine, None, CandidateRender, func(ec *effectCase) {
		t, s := ec.T, ec.Seams
		root := filepath.Join(t.TempDir(), ".romeu", "candidates")
		// A dir a killed command left is deleted: at most one candidate.
		require.NoError(t, os.MkdirAll(filepath.Join(root, "01j9z00000000000000000abcd"), 0o700))
		c, err := RenderCandidate(root, s)
		require.NoError(t, err)
		assert.Equal(t, CandidateRendered, c.State())
		assert.Equal(t, filepath.Join(root, c.ID().String()), c.Path())
		info, err := os.Stat(c.Path())
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
		entries, err := os.ReadDir(root)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "the candidate dir written, the leftover deleted")
	})
	effect(CandidateMachine, CandidateRendered, CandidateDigestEqual, func(ec *effectCase) {
		t, s := ec.T, ec.Seams
		c, _ := rendered(t, s)
		row, err := c.Apply(CandidateDigestEqual, nil, Approval{})
		require.NoError(t, err)
		assert.Equal(t, EffectPromote, row.Effect)
		assert.Equal(t, CandidatePromoted, c.State())
		assert.DirExists(t, c.Path(), "the promotion commit deletes it at its step 4, not the machine")
	})
	effect(CandidateMachine, CandidateRendered, CandidateApprove, func(ec *effectCase) {
		t, s := ec.T, ec.Seams
		c, _ := rendered(t, s)
		p := NewProject()
		_, err := c.Apply(CandidateApprove, p, approval(c))
		require.NoError(t, err)
		assert.Equal(t, CandidateApproved, c.State())
		require.NotNil(t, p.Approval, "the approval is recorded")
		assert.Equal(t, c.ID(), p.Approval.CandidateID)
		assert.DirExists(t, c.Path())
		// An approval binds to its candidate (I7).
		other, _ := rendered(t, s)
		foreign := approval(other)
		foreign.CandidateID = canon.ULID{1}
		_, err = other.Apply(CandidateApprove, NewProject(), foreign)
		require.Error(t, err)
		assert.Equal(t, CandidateRendered, other.State())
	})
	effect(CandidateMachine, CandidateRendered, CandidateDecline, func(ec *effectCase) {
		t, s := ec.T, ec.Seams
		c, _ := rendered(t, s)
		p := NewProject()
		_, err := c.Apply(CandidateDecline, p, Approval{})
		require.NoError(t, err)
		assert.Equal(t, CandidateDeclined, c.State())
		assert.NoDirExists(t, c.Path(), "the candidate dir is deleted")
		assert.Nil(t, p.Approval)
	})
	effect(CandidateMachine, CandidateRendered, CandidateDeclineNoTTY, func(ec *effectCase) {
		t, s := ec.T, ec.Seams
		c, root := rendered(t, s)
		row, err := c.Apply(CandidateDeclineNoTTY, nil, Approval{})
		require.NoError(t, err)
		assert.Equal(t, EffectPrintDigestExit3, row.Effect, "the digest and exit 3 are the command's")
		assert.Equal(t, CandidateDeclined, c.State())
		assert.NoDirExists(t, c.Path())
		assert.DirExists(t, root)
	})
	effect(CandidateMachine, CandidateApproved, CandidatePromotionDone, func(ec *effectCase) {
		t, s := ec.T, ec.Seams
		c, _ := rendered(t, s)
		_, err := c.Apply(CandidateApprove, NewProject(), approval(c))
		require.NoError(t, err)
		_, err = c.Apply(CandidatePromotionDone, nil, Approval{})
		require.NoError(t, err)
		assert.Equal(t, CandidatePromoted, c.State())
		assert.NoDirExists(t, c.Path(), "the candidate dir is deleted")
	})
}

// Generation rows (01 1.6, Generation).
func init() {
	gen := func(from State, ev Event, fn func(c genCase)) {
		effect(GenerationMachine, from, ev, func(ec *effectCase) {
			p, id := projectIn(ec.T, ec.Seams, from)
			fn(genCase{T: ec.T, S: ec.Seams, P: p, ID: id})
		})
	}
	in := func(id canon.ULID) Input {
		i := testInput()
		i.SalvageID = id
		return i
	}

	gen(None, GenCreate, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		got := apply(t, p, s, GenCreate, testInput())
		g := last(t, p)
		assert.Equal(t, got.GenerationID, g.ID)
		assert.Equal(t, GenerationOpen, g.State)
		assert.False(t, g.Adopted)
		assert.Equal(t, testInput().RecreateDigest, g.CreatedWith, "the recreate digest")
		assert.Equal(t, "/abs/root/shop-env/shop-api", g.WorkspacePath)
		assert.Equal(t, testInput().Repos, g.Repos, "the repo set")
		assert.Equal(t, "2026-10-02T09:14:07Z", g.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), "UTC, whole seconds")
		assert.True(t, g.ClosedAt.IsZero())
	})
	gen(GenerationOpen, GenCreateFailedAbsent, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		apply(t, p, s, GenCreateFailedAbsent, testInput())
		assert.Empty(t, p.Generations, "the record just written is removed")
		assert.Equal(t, None, p.Current())
	})
	gen(GenerationOpen, GenCreateFailedPresent, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		before := *last(t, p)
		apply(t, p, s, GenCreateFailedPresent, testInput())
		require.Len(t, p.Generations, 1)
		assert.Equal(t, before, *last(t, p), "the record is kept")
	})
	gen(GenerationOpen, GenCreateTimeout, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		before := *last(t, p)
		apply(t, p, s, GenCreateTimeout, testInput())
		require.Len(t, p.Generations, 1)
		assert.Equal(t, before, *last(t, p), "the record is kept")
	})
	gen(None, GenAdopt, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		in := testInput()
		in.RecreateDigest = canon.Sum[canon.RecreateKind]{} // unknown
		apply(t, p, s, GenAdopt, in)
		g := last(t, p)
		assert.Equal(t, GenerationOpen, g.State)
		assert.True(t, g.Adopted)
		assert.Equal(t, canon.Sum[canon.RecreateKind]{}, g.CreatedWith, "the recreate digest is unknown")
		data, err := EncodeProject(p)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "createdWith")
	})
	gen(GenerationOpen, GenSalvageStart, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		got := apply(t, p, s, GenSalvageStart, testInput())
		assert.Equal(t, GenerationSalvaging, last(t, p).State)
		assert.NotEqual(t, canon.ULID{}, got.SalvageID, "a new salvage id")
		assert.NotEqual(t, last(t, p).ID, got.SalvageID)
	})
	gen(GenerationSalvaging, GenSalvageRestart, func(c genCase) {
		t, s, p, first := c.T, c.S, c.P, c.ID
		got := apply(t, p, s, GenSalvageRestart, testInput())
		assert.Equal(t, GenerationSalvaging, last(t, p).State)
		assert.NotEqual(t, first, got.SalvageID, "a new salvage id")
		assert.NotEqual(t, canon.ULID{}, got.SalvageID)
		assert.Empty(t, last(t, p).Salvage, "no record is written")
	})
	gen(GenerationSalvaging, GenSandboxHalfFailed, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		apply(t, p, s, GenSandboxHalfFailed, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationOpen, g.State)
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, id, g.Salvage[0].ID)
		assert.Equal(t, SalvageIncomplete, g.Salvage[0].Result)
		assert.Equal(t, []string{"sandbox-half-failed"}, g.Salvage[0].Reasons)
	})
	gen(GenerationOpen, GenSalvageFromHost, func(c genCase) {
		t, s, p := c.T, c.S, c.P
		got := apply(t, p, s, GenSalvageFromHost, testInput())
		g := last(t, p)
		assert.Equal(t, GenerationOpen, g.State)
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, got.SalvageID, g.Salvage[0].ID, "under a new salvage id")
		assert.Equal(t, SalvageLost, g.Salvage[0].Result)
		assert.Equal(t, []string{"sandbox-lost"}, g.Salvage[0].Reasons)
		assert.Equal(t, testInput().RefsPrefix, g.Salvage[0].RefsPrefix)
	})
	gen(GenerationSalvaging, GenSalvageFromHost, func(c genCase) {
		t, s, p, started := c.T, c.S, c.P, c.ID
		got := apply(t, p, s, GenSalvageFromHost, testInput())
		g := last(t, p)
		assert.Equal(t, GenerationSalvaging, g.State, "as the open row, the state stays")
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, got.SalvageID, g.Salvage[0].ID)
		assert.NotEqual(t, started, got.SalvageID)
		assert.Equal(t, SalvageLost, g.Salvage[0].Result)
	})
	gen(GenerationSalvaging, GenSalvageVerified, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		apply(t, p, s, GenSalvageVerified, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationOpen, g.State)
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, id, g.Salvage[0].ID)
		assert.Equal(t, SalvageComplete, g.Salvage[0].Result)
		assert.Empty(t, g.Salvage[0].Reasons)
	})
	gen(GenerationSalvaging, GenSalvageIncomplete, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		i := in(id)
		i.Reasons = []string{"over-cap", "unreadable"}
		apply(t, p, s, GenSalvageIncomplete, i)
		g := last(t, p)
		assert.Equal(t, GenerationOpen, g.State)
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, SalvageIncomplete, g.Salvage[0].Result)
		assert.Equal(t, []string{"over-cap", "unreadable"}, g.Salvage[0].Reasons)
	})
	gen(GenerationSalvaging, GenSalvageThenRemove, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		apply(t, p, s, GenSalvageThenRemove, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationRemoving, g.State)
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, id, g.Salvage[0].ID)
		assert.Equal(t, testInput().Fingerprint, g.Salvage[0].Fingerprint, "the fingerprint is written with the state")
		assert.Equal(t, SalvageComplete, g.Salvage[0].Result)

		// With --accept-loss the entry is incomplete and names what was
		// accepted; an entry an earlier row wrote is the one that gets the
		// fingerprint.
		p2, id2 := projectIn(t, s, GenerationSalvaging)
		i := in(id2)
		i.Reasons, i.AcceptedLoss = []string{"over-cap"}, []string{"over-cap"}
		apply(t, p2, s, GenSalvageThenRemove, i)
		e := last(t, p2).Salvage[0]
		assert.Equal(t, SalvageIncomplete, e.Result)
		assert.Equal(t, []string{"over-cap"}, e.AcceptedLoss)
		data, err := EncodeProject(p2)
		require.NoError(t, err)
		_, err = DecodeProject(data)
		require.NoError(t, err)
	})
	gen(GenerationRemoving, GenRemoved, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		require.NotEmpty(t, p.EgressApplied)
		apply(t, p, s, GenRemoved, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationClosedRemoved, g.State)
		assert.False(t, g.ClosedAt.IsZero())
		assert.Empty(t, p.EgressApplied, "egress rules removed")
	})
	gen(GenerationRemoving, GenRemoveFailed, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		before := *last(t, p)
		apply(t, p, s, GenRemoveFailed, in(id))
		assert.Equal(t, before, *last(t, p), "the existing salvage record is kept")
		assert.Equal(t, GenerationRemoving, p.Current())
		assert.NotEmpty(t, p.EgressApplied)
	})
	gen(GenerationRemoving, GenResumeRemoval, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		before := *last(t, p)
		apply(t, p, s, GenResumeRemoval, in(id))
		assert.Equal(t, before, *last(t, p), "the existing salvage record is kept")
		assert.Equal(t, GenerationRemoving, p.Current())
	})
	gen(GenerationRemoving, GenFingerprintDiffers, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		got := apply(t, p, s, GenFingerprintDiffers, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationSalvaging, g.State)
		assert.NotEqual(t, id, got.SalvageID, "a new salvage id")
		require.Len(t, g.Salvage, 1, "the earlier entry, with its fingerprint, stays as the newest written with removing")
		assert.Equal(t, testInput().Fingerprint, g.Salvage[0].Fingerprint)
	})
	gen(GenerationRemoving, GenSandboxAbsent, func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		apply(t, p, s, GenSandboxAbsent, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationClosedRemoved, g.State)
		require.Len(t, g.Salvage, 1, "the existing salvage record is kept")
		assert.Equal(t, SalvageComplete, g.Salvage[0].Result)
		assert.Empty(t, p.EgressApplied, "egress rules removed")
	})
	preserve := func(c genCase) {
		t, s, p, id := c.T, c.S, c.P, c.ID
		apply(t, p, s, GenSandboxAbsent, in(id))
		g := last(t, p)
		assert.Equal(t, GenerationClosedLost, g.State)
		assert.False(t, g.ClosedAt.IsZero())
		require.Len(t, g.Salvage, 1, "a lost record is written")
		assert.Equal(t, SalvageLost, g.Salvage[0].Result)
		assert.Equal(t, []string{"sandbox-lost"}, g.Salvage[0].Reasons)
		assert.Equal(t, testInput().RefsPrefix, g.Salvage[0].RefsPrefix)
	}
	gen(GenerationOpen, GenSandboxAbsent, preserve)
	gen(GenerationSalvaging, GenSandboxAbsent, preserve)
}
