// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// memoryProject returns a valid project spec with memory set to m.
func memoryProject(m *spec.Memory) spec.Project {
	return spec.Project{Schema: "project/v1", Name: "shop", Agent: "a",
		Workload: "docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5",
		Repos:    []spec.Repo{{URL: "https://github.com/example/shop-api", Primary: true}},
		Memory:   m}
}

// TestCheckMemoryRemote refuses a spec without memory.remote with the
// error memory-remote-missing (RJ-357, exit 2) and admits a remote and
// none (REQ-1305).
func TestCheckMemoryRemote(t *testing.T) {
	for name, m := range map[string]*spec.Memory{"no member": nil, "an empty remote": {}} {
		t.Run(name, func(t *testing.T) {
			p := memoryProject(m)
			err := spec.CheckMemoryRemote(&p)
			e, ok := errors.AsType[*cli.Error](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, "RJ-357", e.ID)
			assert.Equal(t, "memory-remote-missing", e.Slug)
			assert.Equal(t, 2, cli.ExitCode(err))
		})
	}
	for _, remote := range []string{"none", "https://github.com/example/shop-memory"} {
		p := memoryProject(&spec.Memory{Remote: remote})
		require.NoError(t, spec.CheckMemoryRemote(&p), remote)
	}
}

// TestLocalOnly is true for memory: {remote: none} alone.
func TestLocalOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    *spec.Memory
		want bool
	}{
		{"no member", nil, false},
		{"none", &spec.Memory{Remote: "none"}, true},
		{"a URL", &spec.Memory{Remote: "https://github.com/example/m"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := memoryProject(tc.m)
			assert.Equal(t, tc.want, p.LocalOnly())
		})
	}
}

// TestValidateMemoryRemote holds the shape of memory.remote when it is
// present, and leaves a spec without the member valid: the general
// validation does not require it (CheckMemoryRemote does, in sync).
func TestValidateMemoryRemote(t *testing.T) {
	p := memoryProject(nil)
	require.NoError(t, p.Validate(), "a spec without memory stays valid")
	for _, ok := range []string{"none", "https://github.com/example/shop-memory", "https://git.corp.internal:8443/a/b.git", "git://127.0.0.1:9418/m"} {
		p := memoryProject(&spec.Memory{Remote: ok})
		assert.NoError(t, p.Validate(), ok)
	}
	for _, bad := range []string{"", "None", "file:///tmp/m", "/tmp/m", "ssh://git@github.com/a/b", "git@github.com:a/b",
		"git://example.com/m", "https://user:pw@github.com/a/b", "https://github.com/a b", "https://", "https:///a", "http://github.com/a/b"} {
		p := memoryProject(&spec.Memory{Remote: bad})
		assert.Contains(t, rulesOf(t, p.Validate()), "memory-remote", "%q", bad)
	}
}

// TestDecodeMemory reads the member from YAML and refuses an unknown
// key under it.
func TestDecodeMemory(t *testing.T) {
	p, err := spec.DecodeProject(valid(t, "ports:", "memory:\n  remote: none\nports:"))
	require.NoError(t, err)
	require.NotNil(t, p.Memory)
	assert.True(t, p.LocalOnly())
	_, err = spec.DecodeProject(valid(t, "ports:", "memory:\n  remote: none\n  push: always\nports:"))
	require.Error(t, err)
}
