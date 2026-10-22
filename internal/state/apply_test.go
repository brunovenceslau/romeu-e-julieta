// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// testNow is the instant the tests' clock stands at.
var testNow = time.Date(2026, 10, 2, 9, 14, 7, 123_000_000, time.UTC)

// testSeams returns a clock fixed at testNow and a ULID source over
// entropy the test fixes, so every id is the same on every run
// (10 10.4, REQ-0940).
func testSeams() Seams {
	return Seams{
		Now: func() time.Time { return testNow },
		IDs: canon.NewULIDSource(bytes.NewReader(bytes.Repeat([]byte{0x2a}, 4096))),
	}
}

// testInput is the input of a create: the record of 03 3.8.
func testInput() Input {
	return Input{
		WorkspacePath: "/abs/root/shop-env/shop-api",
		Repos: []GenerationRepo{
			{Dir: "shop-api", URL: "https://github.com/example/shop-api", Primary: true},
			{Dir: "shop-web", URL: "https://github.com/example/shop-web"},
		},
		RecreateDigest: canon.Sum[canon.RecreateKind]{9},
		ManifestSHA256: SHA256{4},
		RefsPrefix:     "refs/romeu/salvage/shop/01j9zc2q/01j9zd0a/",
		Fingerprint:    canon.Sum[canon.FingerprintKind]{7},
	}
}

// apply reports an event and requires it to be legal.
func apply(t *testing.T, p *Project, s Seams, ev Event, in Input) Applied {
	t.Helper()
	got, err := p.Apply(s, ev, in)
	require.NoError(t, err, "event %s from %s", ev, p.Current())
	return got
}

// projectIn returns a project whose generation is in state st, reached by
// the rows of the table, and the id of the salvage in progress when st is
// salvaging.
func projectIn(t *testing.T, s Seams, st State) (*Project, canon.ULID) {
	t.Helper()
	p := NewProject()
	p.EgressApplied = []string{"github.com"}
	if st == None {
		return p, canon.ULID{}
	}
	apply(t, p, s, GenCreate, testInput())
	if st == GenerationOpen {
		return p, canon.ULID{}
	}
	started := apply(t, p, s, GenSalvageStart, testInput())
	in := testInput()
	in.SalvageID = started.SalvageID
	switch st {
	case GenerationSalvaging:
	case GenerationRemoving:
		apply(t, p, s, GenSalvageThenRemove, in)
	case GenerationClosedRemoved:
		apply(t, p, s, GenSalvageThenRemove, in)
		apply(t, p, s, GenRemoved, in)
	case GenerationClosedLost:
		apply(t, p, s, GenSandboxAbsent, in)
	default:
		t.Fatalf("no path to the state %q", st)
	}
	return p, started.SalvageID
}

// last returns the generation of a project that has one.
func last(t *testing.T, p *Project) *Generation {
	t.Helper()
	require.NotEmpty(t, p.Generations)
	return &p.Generations[len(p.Generations)-1]
}

func TestProject_Apply_illegal(t *testing.T) {
	t.Run("an event with no row changes nothing and is RJ-350, exit 2", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationOpen)
		before, err := EncodeProject(p)
		require.NoError(t, err)
		_, err = p.Apply(s, GenRemoved, testInput())
		require.Error(t, err)
		assert.Equal(t, "RJ-350", cli.Of(err).ID)
		assert.Equal(t, 2, cli.ExitCode(err))
		after, err := EncodeProject(p)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after))
	})
	t.Run("a closed generation is none: a second create opens another", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationClosedRemoved)
		assert.Equal(t, None, p.Current())
		apply(t, p, s, GenCreate, testInput())
		assert.Len(t, p.Generations, 2)
		assert.Equal(t, GenerationOpen, p.Current())
		_, err := EncodeProject(p)
		assert.NoError(t, err, "one closed generation and one open")
	})
	t.Run("an event that ends a salvage with no salvage id is refused and changes nothing", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationSalvaging)
		_, err := p.Apply(s, GenSalvageVerified, testInput())
		require.Error(t, err)
		assert.Equal(t, GenerationSalvaging, p.Current())
		assert.Empty(t, last(t, p).Salvage)
	})
}

func TestProject_Apply_timeoutThenRun(t *testing.T) {
	t.Run("a create that times out keeps the record, and the next run goes on", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationOpen)
		id := last(t, p).ID
		apply(t, p, s, GenCreateTimeout, testInput())
		require.Len(t, p.Generations, 1, "the record is kept (REQ-0089)")
		assert.Equal(t, id, last(t, p).ID)
		assert.Equal(t, GenerationOpen, p.Current())
		// The next run finds the sandbox present: it goes on, and the
		// machine has no event, since nothing about the record changes.
		_, err := EncodeProject(p)
		assert.NoError(t, err)
	})
	t.Run("a create that times out keeps the record, and the next run closes it as lost", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationOpen)
		apply(t, p, s, GenCreateTimeout, testInput())
		in := testInput()
		in.Reasons = []string{"dirty-at-last-facts:2"}
		apply(t, p, s, GenSandboxAbsent, in)
		g := last(t, p)
		assert.Equal(t, GenerationClosedLost, g.State)
		require.Len(t, g.Salvage, 1)
		assert.Equal(t, SalvageLost, g.Salvage[0].Result)
		assert.Equal(t, []string{"dirty-at-last-facts:2"}, g.Salvage[0].Reasons)
		// A run that closes a generation as lost does not create in the
		// same command (REQ-0104): that is the command's, and the next
		// create finds none.
		assert.Equal(t, None, p.Current())
	})
}

func TestProject_Apply_keepsTheLostRecordFromHost(t *testing.T) {
	for _, from := range []State{GenerationOpen, GenerationSalvaging} {
		t.Run(string(from), func(t *testing.T) {
			s := testSeams()
			p, id := projectIn(t, s, from)
			in := testInput()
			in.SalvageID = id
			// salvage --from-host wrote a lost record for this generation.
			wrote := apply(t, p, s, GenSalvageFromHost, in)
			apply(t, p, s, GenSandboxAbsent, in)
			g := last(t, p)
			assert.Equal(t, GenerationClosedLost, g.State)
			require.Len(t, g.Salvage, 1, "the lost record that --from-host wrote is kept, and none is added (REQ-0101)")
			assert.Equal(t, wrote.SalvageID, g.Salvage[0].ID)
		})
	}
}

func TestProject_Apply_errors(t *testing.T) {
	noEntropy := Seams{Now: func() time.Time { return testNow }}
	t.Run("a source with no entropy fails a create and changes nothing", func(t *testing.T) {
		p := NewProject()
		_, err := p.Apply(noEntropy, GenCreate, testInput())
		require.ErrorIs(t, err, canon.ErrULIDSource)
		assert.Empty(t, p.Generations)
	})
	t.Run("a source with no entropy fails the events that make a salvage id", func(t *testing.T) {
		s := testSeams()
		for _, ev := range []Event{GenSalvageStart, GenSalvageFromHost} {
			p, _ := projectIn(t, s, GenerationOpen)
			_, err := p.Apply(noEntropy, ev, testInput())
			require.ErrorIs(t, err, canon.ErrULIDSource, ev)
			assert.Equal(t, GenerationOpen, p.Current(), "the state is as it was")
			assert.Empty(t, last(t, p).Salvage)
		}
		p, _ := projectIn(t, s, GenerationOpen)
		_, err := p.Apply(noEntropy, GenSandboxAbsent, testInput())
		require.ErrorIs(t, err, canon.ErrULIDSource)
		assert.Equal(t, GenerationOpen, p.Current())
	})
	t.Run("the removing write needs the id of the salvage that ended", func(t *testing.T) {
		s := testSeams()
		p, _ := projectIn(t, s, GenerationSalvaging)
		_, err := p.Apply(s, GenSalvageThenRemove, testInput())
		require.ErrorIs(t, err, errNoSalvageID)
		assert.Equal(t, GenerationSalvaging, p.Current())
	})
	t.Run("the removing write goes into an entry an earlier row wrote", func(t *testing.T) {
		s := testSeams()
		p, id := projectIn(t, s, GenerationSalvaging)
		in := testInput()
		in.SalvageID = id
		// A salvage that ended incomplete wrote its entry, and the rerun
		// of the command salvages again under a new id; the removing
		// write names the new salvage, which has no entry yet.
		apply(t, p, s, GenSalvageVerified, in)
		again := apply(t, p, s, GenSalvageStart, in)
		in.SalvageID = again.SalvageID
		apply(t, p, s, GenSalvageThenRemove, in)
		g := last(t, p)
		require.Len(t, g.Salvage, 2)
		assert.Equal(t, again.SalvageID, g.Salvage[1].ID)
		assert.Equal(t, in.Fingerprint, g.Salvage[1].Fingerprint)
		assert.Equal(t, canon.Sum[canon.FingerprintKind]{}, g.Salvage[0].Fingerprint, "the earlier entry keeps none")
	})
}
