// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/schema"
	"github.com/brunovenceslau/romeu-e-julieta/tools/schemagen"
)

// newSchemaTree returns citest.NewTree with the schema of every format
// registered with tools/schemagen written under schemas/, so the schema
// check of all passes on the fixture as it does on the repository.
func newSchemaTree(t *testing.T) *gittest.Repo {
	t.Helper()
	r := citest.NewTree(t)
	outs, err := schema.Outputs(schemagen.Registered())
	require.NoError(t, err)
	for _, o := range outs {
		r.Write(o.Path, string(o.Content))
	}
	return r
}
