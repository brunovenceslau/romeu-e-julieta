// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const digest64 = "c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5"

// TestWorkloadPinsTheReference is the acceptance case: the reference
// is written as given, no request is made, and the home written by
// hand stays.
func TestWorkloadPinsTheReference(t *testing.T) {
	h := newHarness(t, nil)
	h.repo.Write("kits/pins.yaml", `workload:
  ref: docker.io/docker/sbx-kit-claude@sha256:`+strings.Repeat("0", 64)+`
  home:
    path: /home/agent
    source: sitting M2
`)
	ref := "docker.io/docker/sbx-kit-claude@sha256:" + digest64
	require.Equal(t, exitOK, h.run(t, "workload", ref), h.stderr.String())
	assert.Equal(t, header()+`workload:
  ref: `+ref+`
  home:
    path: /home/agent
    source: sitting M2
`, h.read(t, "kits/pins.yaml"))
	assert.Equal(t, int32(0), h.requests.Load())
}

func TestWorkloadRefuses(t *testing.T) {
	cases := []struct{ name, ref string }{
		{"no_registry", "docker/sbx-kit-claude@sha256:" + digest64},
		{"a_tag", "docker.io/docker/sbx-kit-claude:2.1.296"},
		{"a_tag_and_a_digest", "docker.io/docker/sbx-kit-claude:2.1.296@sha256:" + digest64},
		{"short_digest", "docker.io/docker/sbx-kit-claude@sha256:" + digest64[:63]},
		{"upper_case_digest", "docker.io/docker/sbx-kit-claude@sha256:" + strings.ToUpper(digest64)},
		{"upper_case_repository", "docker.io/Docker/sbx-kit-claude@sha256:" + digest64},
		{"sha512", "docker.io/docker/sbx-kit-claude@sha512:" + digest64 + digest64},
		{"dot_dot", "docker.io/../x@sha256:" + digest64},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			assert.Equal(t, exitError, h.run(t, "workload", c.ref))
			assert.Contains(t, h.stderr.String(), "is not <registry>/<repository>@sha256:<64 hex>")
			assert.False(t, h.exists("kits/pins.yaml"))
		})
	}
}

func TestWorkloadAcceptsARegistryWithAPort(t *testing.T) {
	cases := []struct{ name, ref string }{
		{"localhost_with_port", "localhost:5000/workload@sha256:" + digest64},
		{"one_label_with_port", "registry:5000/team/workload@sha256:" + digest64},
		{"three_components", "ghcr.io/a/b/c@sha256:" + digest64},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			assert.Equal(t, exitOK, h.run(t, "workload", c.ref), h.stderr.String())
		})
	}
}
