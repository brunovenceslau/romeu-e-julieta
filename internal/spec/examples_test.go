// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// TestTheExampleConfigRepoValidates reads examples/projects, the
// adopter's starter (REQ-0119), as romeu sync step 3 does: every spec
// decodes and validates, and with host settings that name the example
// config repo the rules romeu alone checks pass too.
func TestTheExampleConfigRepoValidates(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "projects")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var files []spec.File
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		files = append(files, spec.File{Path: "projects/" + e.Name(), Data: data})
	}
	require.NotEmpty(t, files)
	specs, err := spec.LoadSet(files, nil)
	require.NoError(t, err)
	assert.Len(t, specs, len(files))

	h, err := spec.DecodeHostSettings(fixture(t, "valid-host-settings.yaml"))
	require.NoError(t, err)
	_, err = spec.LoadSet(files, &h)
	require.NoError(t, err)
}
