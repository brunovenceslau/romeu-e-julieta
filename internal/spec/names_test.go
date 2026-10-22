// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// TestValidDomain is the domain rule of 03 3.1 for the other sources of
// 07 7.5, the catalog and the kit-declared domains.
func TestValidDomain(t *testing.T) {
	require.NoError(t, spec.ValidDomain("registry.npmjs.org"))
	for _, d := range []string{"", "*.npmjs.org", "npmjs.org:443", "10.0.0.1", "NPMJS.org", "localhost"} {
		assert.Error(t, spec.ValidDomain(d), d)
	}
}

// TestParseWorkload reads a workload reference into the repository that
// workloadRepositories names and the digest internal/oci verifies.
func TestParseWorkload(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	w, err := spec.ParseWorkload("docker.io/docker/sbx-kit-claude@" + digest)
	require.NoError(t, err)
	assert.Equal(t, spec.Workload{Repository: "docker.io/docker/sbx-kit-claude", Digest: digest}, w)
	for _, ref := range []string{"docker.io/docker/sbx-kit-claude:latest", "docker/x@" + digest, "docker.io/x@sha256:abc"} {
		_, err := spec.ParseWorkload(ref)
		assert.Error(t, err, ref)
	}
}
