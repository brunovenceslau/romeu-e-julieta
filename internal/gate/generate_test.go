// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

func TestGenerateFailsWhenTheClassificationFails(t *testing.T) {
	t.Parallel()
	// spec.Classify fails on a field that is neither tagged nor on the
	// committed list of ungated fields (I7), so the generator fails too.
	_, err := generate(func() ([]spec.FieldClass, error) { return nil, errors.New("a field on neither") })
	assert.ErrorContains(t, err, "a field on neither")
}

func TestGenerateFailsOnAGatedFieldWithNoMemberOfTheSet(t *testing.T) {
	t.Parallel()
	classes, err := spec.Classify()
	require.NoError(t, err)
	classes = append(classes, spec.FieldClass{Name: "devices", Gate: spec.GateWidening, Apply: spec.ApplyRecreate})
	_, err = generate(func() ([]spec.FieldClass, error) { return classes, nil })
	assert.ErrorContains(t, err, "devices")
}

func TestGenerateFailsOnAMemberOfTheSetForNoGatedField(t *testing.T) {
	t.Parallel()
	classes, err := spec.Classify()
	require.NoError(t, err)
	var without []spec.FieldClass
	for _, c := range classes {
		if c.Name != "ports" {
			without = append(without, c)
		}
	}
	_, err = generate(func() ([]spec.FieldClass, error) { return without, nil })
	assert.ErrorContains(t, err, "ports")
}

func TestGeneratedFilesAreTheCommittedOnes(t *testing.T) {
	t.Parallel()
	files, err := Generated()
	require.NoError(t, err)
	require.Len(t, files, 2)
	for _, f := range files {
		committed, err := os.ReadFile(filepath.Base(f.Path))
		require.NoError(t, err)
		assert.Equal(t, string(f.Content), string(committed), "%s: run go generate ./...", f.Path)
	}
}

func TestFieldListsFollowTheTags(t *testing.T) {
	t.Parallel()
	classes, err := spec.Classify()
	require.NoError(t, err)
	var widening, ungated, recreate []string
	for _, c := range classes {
		if c.Gate == spec.GateWidening {
			widening = append(widening, c.Name)
		} else {
			ungated = append(ungated, c.Name)
		}
		if c.Apply == spec.ApplyRecreate {
			recreate = append(recreate, c.Name)
		}
	}
	assert.ElementsMatch(t, widening, wideningFields)
	assert.ElementsMatch(t, ungated, ungatedFields)
	assert.ElementsMatch(t, recreate, recreateFields)
	assert.ElementsMatch(t, []string{"sandboxOptions", "run"}, ungatedFields, "01 1.4: today the list holds sandboxOptions and run")
}
