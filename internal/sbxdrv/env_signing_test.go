// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// TestSigningEnvReachesTheCallsOfItsProjectOnly: an sbx call of a
// project that uses git-ssh-sign carries SSH_AUTH_SOCK, the signing
// socket of the host settings, and a call of another project, of a
// personal kit of that name, of no project, or with no socket set
// carries none, even when this process has one (04 4.1, Subprocesses,
// item 1; I34, REQ-0520).
func TestSigningEnvReachesTheCallsOfItsProjectOnly(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/host/agent.sock")
	sbx := buildEnvHelper(t)
	host := &spec.HostSettings{Signing: &spec.Signing{AgentSocket: "/run/sign.sock"}}
	with := &spec.Project{Name: "shop", Kits: []spec.Kit{{Product: "julieta"}, {Product: "git-ssh-sign"}}}
	without := &spec.Project{Name: "blog", Kits: []spec.Kit{{Product: "julieta"}}}
	personal := &spec.Project{Name: "mine", Kits: []spec.Kit{{Personal: "git-ssh-sign"}}}
	const base = "arg:version\nenv:HOME=/home/test\nenv:LANG=C\nenv:PATH=/usr/bin:/bin\n"
	tests := []struct {
		name    string
		host    *spec.HostSettings
		project *spec.Project
		want    string
	}{
		{"the project that uses the kit", host, with, base + "env:SSH_AUTH_SOCK=/run/sign.sock\n"},
		{"a project without the kit", host, without, base},
		{"a personal kit of the same name", host, personal, base},
		{"a call of no project", host, nil, base},
		{"no socket set", &spec.HostSettings{}, with, base},
		{"no host settings", nil, with, base},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runner(sbx)
			r.Host = tt.host
			res, err := r.Run(context.Background(), Version(), Opts{Project: tt.project})
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(res.Stdout))
		})
	}
}
