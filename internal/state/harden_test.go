// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// TestProject_liveGenerationIsTheLast holds the rule current() relies
// on: a generation that is not closed is the last element, so a record
// with a live one earlier cannot make romeu create a second sandbox.
func TestProject_liveGenerationIsTheLast(t *testing.T) {
	s := testSeams()
	live, _ := projectIn(t, s, GenerationRemoving)
	closed, _ := projectIn(t, s, GenerationClosedLost)
	bad := NewProject()
	bad.Generations = []Generation{live.Generations[0], closed.Generations[0]}
	bad.Generations[1].ID = canon.ULID{9}

	t.Run("a writer refuses it", func(t *testing.T) {
		_, err := EncodeProject(bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "last")
	})
	t.Run("a reader takes it as unreadable, RJ-334, and the file stays", func(t *testing.T) {
		st := openTemp(t)
		good := NewProject()
		good.Generations = []Generation{closed.Generations[0], live.Generations[0]}
		good.Generations[0].ID = canon.ULID{9}
		data, err := EncodeProject(good)
		require.NoError(t, err, "the closed generation first, the live one last, is whole")
		path := seedProject(t, st, "shop", data)
		_, err = st.LoadProject("shop")
		require.NoError(t, err)
		// The same two generations, the other way round.
		bad.Generations[1].CreatedAt = bad.Generations[1].CreatedAt.Add(1)
		raw, err := encodeRecord(bad)
		require.NoError(t, err)
		require.NoError(t, writeFile(st.projects, "shop.json", raw))
		_, err = st.LoadProject("shop")
		require.ErrorIs(t, err, ErrInvalid)
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
		err = st.UpdateProject("shop", func(*Project) error { return nil })
		assert.Equal(t, "RJ-334", cli.Of(err).ID)
		after, rerr := os.ReadFile(path)
		require.NoError(t, rerr)
		assert.Equal(t, string(raw), string(after), "byte-identical")
	})
	t.Run("Apply fails closed on a record that has one in memory", func(t *testing.T) {
		before := len(bad.Generations)
		_, err := bad.Apply(s, GenCreate, testInput())
		require.Error(t, err)
		assert.Len(t, bad.Generations, before, "no second live generation is created")
	})
}

func TestProject_salvageRules(t *testing.T) {
	s := testSeams()
	t.Run("the id of the salvage in progress is recorded, and cleared when it ends", func(t *testing.T) {
		p, id := projectIn(t, s, GenerationSalvaging)
		assert.Equal(t, id, last(t, p).SalvageID, "state salvaging with the new salvage id (01 1.6)")
		data, err := EncodeProject(p)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"salvageId": "`+id.String()+`"`)
		in := testInput()
		in.SalvageID = id
		apply(t, p, s, GenSalvageVerified, in)
		assert.Equal(t, canon.ULID{}, last(t, p).SalvageID)
	})
	t.Run("an event that ends a salvage names the salvage in progress", func(t *testing.T) {
		for _, ev := range []Event{GenSandboxHalfFailed, GenSalvageVerified, GenSalvageIncomplete, GenSalvageThenRemove} {
			p, _ := projectIn(t, s, GenerationSalvaging)
			in := testInput()
			in.SalvageID = canon.ULID{5} // another salvage
			_, err := p.Apply(s, ev, in)
			require.Error(t, err, ev)
			assert.Equal(t, GenerationSalvaging, p.Current(), ev)
			assert.Empty(t, last(t, p).Salvage, ev)
		}
	})
	t.Run("a removing write needs a fingerprint", func(t *testing.T) {
		p, id := projectIn(t, s, GenerationSalvaging)
		in := testInput()
		in.SalvageID, in.Fingerprint = id, canon.Sum[canon.FingerprintKind]{}
		_, err := p.Apply(s, GenSalvageThenRemove, in)
		require.Error(t, err)
		assert.Equal(t, GenerationSalvaging, p.Current())
	})
	invalid := map[string]func(*Generation){
		"salvaging with no salvage id":           func(g *Generation) { g.State, g.SalvageID = GenerationSalvaging, canon.ULID{} },
		"a salvage id outside salvaging":         func(g *Generation) { g.SalvageID = canon.ULID{3} },
		"a complete salvage with no manifest":    func(g *Generation) { g.Salvage[0].ManifestSHA256 = SHA256{} },
		"an incomplete salvage with no manifest": func(g *Generation) { g.Salvage[0].Result = SalvageIncomplete; g.Salvage[0].ManifestSHA256 = SHA256{} },
		"removing with no fingerprint": func(g *Generation) {
			g.State, g.ClosedAt, g.Salvage[0].Fingerprint = GenerationRemoving, zeroTime, canon.Sum[canon.FingerprintKind]{}
		},
		"two salvage entries of one id":           func(g *Generation) { g.Salvage = append(g.Salvage, g.Salvage[0]) },
		"a salvage in progress that has an entry": func(g *Generation) { g.State, g.ClosedAt, g.SalvageID = GenerationSalvaging, zeroTime, g.Salvage[0].ID },
	}
	for name, edit := range invalid {
		t.Run(name+" is refused", func(t *testing.T) {
			p, id := projectIn(t, s, GenerationClosedRemoved)
			_ = id
			edit(last(t, p))
			_, err := EncodeProject(p)
			require.Error(t, err)
		})
	}
	t.Run("a lost salvage may have no manifest", func(t *testing.T) {
		p, _ := projectIn(t, s, GenerationOpen)
		in := testInput()
		in.ManifestSHA256 = SHA256{}
		apply(t, p, s, GenSalvageFromHost, in)
		_, err := EncodeProject(p)
		require.NoError(t, err)
	})
}

// zeroTime is the zero value of a timestamp that a record leaves out.
var zeroTime time.Time
