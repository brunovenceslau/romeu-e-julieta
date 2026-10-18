// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package hygiene

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
)

// TestNeverTrackedAtAnyDepthAndCase: a fixture name on the neverTracked
// list of the denylist fails hygiene at the root, below a directory and
// in another case, and passes once the list no longer holds it, so the
// rule reads the list and no constant.
func TestNeverTrackedAtAnyDepthAndCase(t *testing.T) {
	l, err := names.Parse(citest.Denylist(t, citest.MadeUp))
	require.NoError(t, err)
	require.NoError(t, l.AddNeverTracked("NOTES.local"))
	for _, p := range []string{"NOTES.local", "a/b/notes.LOCAL"} {
		r := citest.NewTree(t)
		r.Write(names.Path, string(l.Marshal()))
		r.Write(p, "text\n")
		r.Commit("fixture")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitFail, code, p)
		assert.Contains(t, out, p+": "+NeverTrackedRule+": "+NeverTrackedMsg, p)
	}

	// The default fixture denylist lists PROLOGUE.md and not this name.
	r := citest.NewTree(t)
	r.Write("NOTES.local", "text\n")
	r.Commit("fixture")
	code, out := citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitOK, code, "a name the list does not hold: %s", out)
}

// TestNeverTrackedListIsKept: a denylist that keeps its entries but lists
// no never-tracked name fails hygiene, so dropping the key cannot turn
// the rule off.
func TestNeverTrackedListIsKept(t *testing.T) {
	l := &names.List{}
	e, err := names.NewEntry(citest.MadeUp)
	require.NoError(t, err)
	l.Add(e)
	r := citest.NewTree(t)
	r.Write(names.Path, string(l.Marshal()))
	r.Commit("fixture")
	code, out := citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitFail, code)
	assert.Contains(t, out, names.Path+": denylist: the denylist lists no neverTracked name at HEAD")
}
