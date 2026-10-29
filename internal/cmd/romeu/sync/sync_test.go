// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// TestACommandLineOfTheWrongShapeWritesNothing: no name and no --all,
// both together, and a --from that is neither origin nor a full object
// id are usage (exit 2) before step 1, so no lock is taken, no recovery
// runs and nothing exists in the state dir or the root after.
func TestACommandLineOfTheWrongShapeWritesNothing(t *testing.T) {
	tests := []struct {
		name string
		args []string
		slug string
	}{
		{name: "no name and no --all", args: nil, slug: "targets"},
		{name: "--all and a name", args: []string{"--all", "shop"}, slug: "targets"},
		{name: "an abbreviated --from", args: []string{"--from", strings.Repeat("a", 12), "shop"}, slug: "from-invalid"},
		{name: "a --from that is a branch name", args: []string{"--from", "main", "shop"}, slug: "from-invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHost(t)
			var stderr bytes.Buffer
			env := &cli.Env{Binary: "romeu", Stdout: cli.NewWriter(&bytes.Buffer{}), Stderr: cli.NewWriter(&stderr),
				Version: version, SettingsPath: h.settings}

			code := cli.Main(t.Context(), append([]string{"sync"}, tt.args...), env)

			assert.Equal(t, 2, code)
			assert.Contains(t, stderr.String(), tt.slug+": ")
			assert.NoDirExists(t, h.state, "sync wrote into the state dir")
			entries, err := os.ReadDir(h.root)
			require.NoError(t, err)
			assert.Empty(t, entries, "the root holds what sync wrote")
		})
	}
}
