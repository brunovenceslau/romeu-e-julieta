// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package preflight

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/signing"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// testKey is an OpenSSH public key line of no real key: the check reads
// its type and blob only.
const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHk1a3Rlc3RrZXlmb3J0aGVmaXh0dXJlb25seTAwMDA="

// TestSigningOf: the rule is keyed by the product kit git-ssh-sign; a
// project without it, or with a personal kit of that name, has no
// signing check, and one with it reads the socket of the host settings
// and the kit's signingKey, each empty when unset.
func TestSigningOf(t *testing.T) {
	host := &spec.HostSettings{Signing: &spec.Signing{AgentSocket: "/run/sign.sock"}}
	withKey := spec.Kit{Product: SigningKit, Args: map[string]string{SigningKeyArg: testKey}}
	tests := []struct {
		name string
		host *spec.HostSettings
		kits []spec.Kit
		want *Signing
	}{
		{"no kit", host, nil, nil},
		{"another product kit", host, []spec.Kit{{Product: "julieta"}}, nil},
		{"a personal kit of the same name", host, []spec.Kit{{Personal: SigningKit, Args: map[string]string{SigningKeyArg: testKey}}}, nil},
		{"the kit", host, []spec.Kit{{Product: "julieta"}, withKey}, &Signing{Socket: "/run/sign.sock", Key: testKey}},
		{"the kit with no socket set", &spec.HostSettings{}, []spec.Kit{withKey}, &Signing{Key: testKey}},
		{"the kit with no host settings", nil, []spec.Kit{withKey}, &Signing{Key: testKey}},
		{"the kit with no signingKey", host, []spec.Kit{{Product: SigningKit}}, &Signing{Socket: "/run/sign.sock"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SigningOf(tt.host, tt.kits))
		})
	}
}

// TestPreflightStep5Signing shows the row signing-socket of step 5
// (REQ-0384, I34): a project that does not use git-ssh-sign passes, and
// one that does stops the step with RJ-204, exit 2, naming the reason of
// internal/signing, when the socket is unset or answers nothing. The
// cases of an agent that answers are in the tests of sync's rule and of
// doctor, since this package imports no net (REQ-0077).
func TestPreflightStep5Signing(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone.sock")
	tests := []struct {
		name    string
		signing *Signing
		want    error
	}{
		{"a project without the kit", nil, nil},
		{"no socket set", &Signing{Key: testKey}, signing.ErrNoSocket},
		{"a socket nothing listens on", &Signing{Socket: gone, Key: testKey}, signing.ErrUnreachable},
		{"no signingKey", &Signing{Socket: gone}, signing.ErrSigningKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := quietHost(t)
			in.Signing = tt.signing
			err := hostChecksOnly(t, in, nil)
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-204", err)
			assert.Equal(t, 2, cli.Of(err).Exit)
			require.ErrorIs(t, err, tt.want)
			assert.Contains(t, err.Error(), "the project shop uses the kit git-ssh-sign")
		})
	}
}

// TestPreflightSigningForDoctor: the check runs in HostFindings under
// its slug, the one doctor's row reports, and a protective command
// prints it and goes on (REQ-0078).
func TestPreflightSigningForDoctor(t *testing.T) {
	in := quietHost(t)
	in.Signing = &Signing{Key: testKey}
	d := (&trace{}).deps(t)

	findings, _ := HostFindings(t.Context(), d, in)
	require.Len(t, findings, 1)
	assert.Equal(t, "signing-socket", findings[0].Slug)
	assert.ErrorIs(t, findings[0].Err, signing.ErrNoSocket)
	assert.Contains(t, HostSlugs(), "signing-socket")

	in.Command = CmdSalvage
	res, err := Run(t.Context(), d, in)
	require.NoError(t, err)
	require.Len(t, res.Printed, 1)
	assert.Equal(t, 5, res.Printed[0].Step)
	testsupport.RequireID(t, "RJ-204", res.Printed[0].Err)
}
