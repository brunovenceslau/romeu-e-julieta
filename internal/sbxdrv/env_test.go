// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// signingFixture is a contribution as T080's env_signing.go will write
// it: the socket of the signing agent for a project that uses the kit.
func signingFixture() contribution {
	return contribution{name: "signing", vars: func(in EnvInput) []string {
		if in.Host == nil || in.Host.Signing == nil || in.Project == nil {
			return nil
		}
		for _, k := range in.Project.Kits {
			if k.Product == "git-ssh-sign" {
				return []string{"SSH_AUTH_SOCK=" + in.Host.Signing.AgentSocket}
			}
		}
		return nil
	}}
}

// TestEnvironmentBase is the scrubbed environment of 04 4.1: PATH, HOME
// and LANG, and TERM for an interactive call only.
func TestEnvironmentBase(t *testing.T) {
	captured, err := environment("/home/x", "xterm", EnvInput{Call: Call{Mode: Captured}}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"PATH=/usr/bin:/bin", "HOME=/home/x", "LANG=C"}, captured)

	interactive, err := environment("/home/x", "xterm", EnvInput{Call: Call{Mode: Interactive}}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"PATH=/usr/bin:/bin", "HOME=/home/x", "LANG=C", "TERM=xterm"}, interactive)

	noTerm, err := environment("/home/x", "", EnvInput{Call: Call{Mode: Interactive}}, nil)
	require.NoError(t, err)
	assert.Len(t, noTerm, 3, "no TERM is set when the caller has none")
}

// TestEnvironmentContributionSeam is the seam of T080: a contribution
// adds its variable for the project it is for and for no other.
func TestEnvironmentContributionSeam(t *testing.T) {
	host := &spec.HostSettings{Signing: &spec.Signing{AgentSocket: "/run/agent.sock"}}
	with := &spec.Project{Kits: []spec.Kit{{Product: "git-ssh-sign"}}}
	without := &spec.Project{Kits: []spec.Kit{{Product: "other"}}}

	env, err := environment("/h", "", EnvInput{Call: Call{Mode: Captured}, Host: host, Project: with}, []contribution{signingFixture()})
	require.NoError(t, err)
	assert.Equal(t, []string{"PATH=/usr/bin:/bin", "HOME=/h", "LANG=C", "SSH_AUTH_SOCK=/run/agent.sock"}, env)

	env, err = environment("/h", "", EnvInput{Call: Call{Mode: Captured}, Host: host, Project: without}, []contribution{signingFixture()})
	require.NoError(t, err)
	assert.NotContains(t, env, "SSH_AUTH_SOCK=/run/agent.sock")

	env, err = environment("/h", "", EnvInput{Call: Call{Mode: Captured}}, []contribution{signingFixture()})
	require.NoError(t, err)
	assert.Len(t, env, 3, "a call with no project gets no contribution")
}

// TestEnvironmentContributionOrder sorts the contributions by name.
func TestEnvironmentContributionOrder(t *testing.T) {
	regs := []contribution{
		{name: "b", vars: func(EnvInput) []string { return []string{"B_VAR=2"} }},
		{name: "a", vars: func(EnvInput) []string { return []string{"A_VAR=1"} }},
	}
	env, err := environment("/h", "", EnvInput{Call: Call{Mode: Captured}}, regs)
	require.NoError(t, err)
	assert.Equal(t, []string{"A_VAR=1", "B_VAR=2"}, env[3:])
}

// TestEnvironmentRefusesDefects refuses a contribution that overrides
// the base, collides with another or is not NAME=VALUE.
func TestEnvironmentRefusesDefects(t *testing.T) {
	give := func(name string, entries ...string) contribution {
		return contribution{name: name, vars: func(EnvInput) []string { return entries }}
	}
	tests := []struct {
		name string
		regs []contribution
	}{
		{"PATH", []contribution{give("x", "PATH=/evil")}},
		{"HOME", []contribution{give("x", "HOME=/evil")}},
		{"LANG", []contribution{give("x", "LANG=de")}},
		{"TERM", []contribution{give("x", "TERM=vt100")}},
		{"a collision", []contribution{give("x", "A_VAR=1"), give("y", "A_VAR=2")}},
		{"a lower case name", []contribution{give("x", "a_var=1")}},
		{"no equals sign", []contribution{give("x", "A_VAR")}},
		{"a newline in a value", []contribution{give("x", "A_VAR=1\nB_VAR=2")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := environment("/h", "", EnvInput{Call: Call{Mode: Captured}}, tt.regs)
			testsupport.RequireID(t, "RJ-348", err)
		})
	}
}

// TestRegister adds to the registry that every call reads.
func TestRegister(t *testing.T) {
	saved := slices.Clone(contributions)
	t.Cleanup(func() { contributions = saved })
	register("x", func(EnvInput) []string { return []string{"X_VAR=1"} })
	require.Len(t, contributions, len(saved)+1)
	assert.Equal(t, "x", contributions[len(contributions)-1].name)
}
