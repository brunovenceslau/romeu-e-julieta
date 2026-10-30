// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/preflight"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// signingAgent serves an in-process ssh-agent holding n new ed25519
// keys, from a directory with a short path (a socket path holds at most
// 104 bytes on macOS), and returns its socket and each key's public
// line.
func signingAgent(t *testing.T, n int) (socket string, keys []string) {
	t.Helper()
	keyring := agent.NewKeyring()
	for range n {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		require.NoError(t, keyring.Add(agent.AddedKey{PrivateKey: priv}))
		signer, err := ssh.NewSignerFromKey(priv)
		require.NoError(t, err)
		keys = append(keys, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))))
	}
	dir := filepath.Join(os.TempDir(), "ds"+rand.Text()[:8])
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket = filepath.Join(dir, "s")
	ln, err := new(net.ListenConfig).Listen(t.Context(), "unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_ = agent.ServeAgent(keyring, conn)
			}()
		}
	}()
	return socket, keys
}

// approved returns the record of shop with an approval whose widening
// set holds kits.
func approved(t *testing.T, kits ...gate.Kit) *state.Project {
	t.Helper()
	p := project(t, state.GenerationOpen)
	set := gate.Set{Project: "shop", Kits: kits, RenderDigest: canon.Sum[canon.RenderKind]{2}}
	sum, err := gate.Digest(set)
	require.NoError(t, err)
	id, err := canon.ParseULID("01j9zd0a1b2c3d4e5f6g7h8j9k")
	require.NoError(t, err)
	p.Approval = &state.Approval{
		WideningDigest: sum, Set: set, RenderDigest: set.RenderDigest,
		SpecCommit: "0123456789abcdef0123456789abcdef01234567", CandidateID: id,
		ApprovedAt: time.Date(2026, 10, 2, 9, 10, 0, 0, time.UTC),
	}
	return p
}

// signKit is the product kit git-ssh-sign with the signingKey key.
func signKit(key string) gate.Kit {
	return gate.Kit{ID: "git-ssh-sign", Source: gate.SourceProduct, Args: map[string]string{"signingKey": key}}
}

// TestDoctorSigningSocket is the row signing-socket (REQ-0384, I34): a
// project whose approval holds git-ssh-sign passes when the socket of
// the host settings holds exactly its signingKey, and fails, naming the
// project, the reason and J1 step 5, with exit 6, otherwise. A project
// without the kit, or with a personal kit of that name, is not checked.
//
//romeu:invariant I34 test
func TestDoctorSigningSocket(t *testing.T) {
	t.Parallel()
	one, oneKeys := signingAgent(t, 1)
	two, twoKeys := signingAgent(t, 2)
	_, other := signingAgent(t, 1)
	tests := []struct {
		name   string
		record *state.Project
		socket string
		want   string
		says   []string
	}{
		{"no record", nil, "", resultPass, []string{"no approved project uses git-ssh-sign"}},
		{"an approval without the kit", approved(t, gate.Kit{ID: "julieta", Source: gate.SourceProduct}), "", resultPass, []string{"no approved project"}},
		{"a personal kit of the same name", approved(t, gate.Kit{ID: "git-ssh-sign", Source: gate.SourcePersonal}), "", resultPass, []string{"no approved project"}},
		{"the one key", approved(t, signKit(oneKeys[0])), one, resultPass, []string{"one signing key of shop"}},
		{"no socket set", approved(t, signKit(oneKeys[0])), "", resultFail, []string{"the project shop", "no agent socket is set", "J1 step 5"}},
		{"two keys", approved(t, signKit(twoKeys[0])), two, resultFail, []string{"the project shop", "exactly one key", "J1 step 5"}},
		{"one other key", approved(t, signKit(other[0])), one, resultFail, []string{"the project shop", "not the signing key"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			if tt.record != nil {
				f.writeProject(t, "shop", tt.record)
			}
			if tt.socket != "" {
				f.settings.Signing = &spec.Signing{AgentSocket: tt.socket}
			}

			rep, out, err := f.output(t)

			c := check(t, rep, slugSigningSocket)
			assert.Equal(t, tt.want, c.Result, out)
			for _, s := range tt.says {
				assert.Contains(t, c.Message, s)
			}
			if tt.want == resultFail {
				assert.Equal(t, 6, exitOf(err), out)
			}
		})
	}
}

// TestDoctorSigningSocketIsStep5s: the row is the check step 5 runs, by
// the same slug, so a step 5 that runs none is an error, and a finding
// step 5 reports under the slug is the row's result, never dropped.
func TestDoctorSigningSocketIsStep5s(t *testing.T) {
	t.Parallel()
	assert.Contains(t, preflight.HostSlugs(), slugSigningSocket)

	f := newFixture(t)
	f.preSlugs = func() []string {
		return slices.DeleteFunc(preflight.HostSlugs(), func(s string) bool { return s == slugSigningSocket })
	}
	rep, out, err := f.output(t)
	assert.Equal(t, 1, exitOf(err), out)
	assert.Equal(t, resultError, check(t, rep, slugSigningSocket).Result)

	f = newFixture(t)
	f.hostFindings = func(ctx context.Context, d preflight.Deps, in preflight.Input) ([]preflight.Finding, []string) {
		findings, unchecked := preflight.HostFindings(ctx, d, in)
		err := preflight.SigningSocket.Wrap(errors.New("the socket answers no key"), "the project shop uses the kit git-ssh-sign")
		return append(findings, preflight.Finding{Slug: slugSigningSocket, Err: err}), unchecked
	}
	rep, out, err = f.output(t)
	assert.Equal(t, 6, exitOf(err), out)
	c := check(t, rep, slugSigningSocket)
	assert.Equal(t, resultFail, c.Result)
	assert.Contains(t, c.Message, "the socket answers no key")
}
