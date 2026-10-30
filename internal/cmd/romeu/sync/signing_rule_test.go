// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/signing"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// signingAgent serves an in-process ssh-agent holding n new ed25519
// keys, from a directory with a short path (a socket path holds at most
// 104 bytes on macOS), and returns its socket and the public line of
// each key, as the kit's signingKey holds one.
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
	dir := filepath.Join(os.TempDir(), "sr"+rand.Text()[:8])
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

// signingTarget is a target named name whose kits are julieta and, when
// key is not empty, git-ssh-sign with that signingKey.
func signingTarget(name, key string) spec.Project {
	p := spec.Project{Name: name, Kits: []spec.Kit{{Product: "julieta"}}}
	if key != "" {
		p.Kits = append(p.Kits, spec.Kit{Product: "git-ssh-sign", Args: map[string]string{"signingKey": key}})
	}
	return p
}

// signingRun is a run whose host settings name socket, empty for none,
// and whose targets are targets.
func signingRun(socket string, targets ...spec.Project) *run {
	r := newRun(&cli.Env{}, flags{})
	if socket != "" {
		r.settings.Signing = &spec.Signing{AgentSocket: socket}
	}
	r.targets = targets
	return r
}

// TestSigningRuleIsRegistered: the rule is in the registry of step 6.
func TestSigningRuleIsRegistered(t *testing.T) {
	assert.True(t, slices.ContainsFunc(rules, func(ru rule) bool { return ru.name == "signing-socket" }))
}

// TestSigningRule is the identity listing table of I34 as sync meets it
// (REQ-0600): a target with git-ssh-sign renders only when the socket
// holds exactly one key and it is the kit's signingKey; otherwise exit
// 2 with signing-socket (RJ-204), naming the project. A target without
// the kit is not checked, whatever the socket holds.
//
//romeu:invariant I34 test
func TestSigningRule(t *testing.T) {
	one, oneKeys := signingAgent(t, 1)
	two, twoKeys := signingAgent(t, 2)
	empty, _ := signingAgent(t, 0)
	_, other := signingAgent(t, 1)
	tests := []struct {
		name    string
		socket  string
		targets []spec.Project
		want    error
	}{
		{"a target without the kit and no socket", "", []spec.Project{signingTarget("shop", "")}, nil},
		{"a target without the kit and a socket of two keys", two, []spec.Project{signingTarget("shop", "")}, nil},
		{"the one key", one, []spec.Project{signingTarget("shop", oneKeys[0])}, nil},
		{"no socket set", "", []spec.Project{signingTarget("shop", oneKeys[0])}, signing.ErrNoSocket},
		{"a socket that answers nothing", filepath.Join(t.TempDir(), "gone"), []spec.Project{signingTarget("shop", oneKeys[0])}, signing.ErrUnreachable},
		{"no key", empty, []spec.Project{signingTarget("shop", oneKeys[0])}, signing.ErrKeyCount},
		{"two keys, the signing key among them", two, []spec.Project{signingTarget("shop", twoKeys[0])}, signing.ErrKeyCount},
		{"one other key", one, []spec.Project{signingTarget("shop", other[0])}, signing.ErrOtherKey},
		{"the second target fails after the first passes", one, []spec.Project{signingTarget("api", oneKeys[0]), signingTarget("shop", other[0])}, signing.ErrOtherKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSigningSocket(t.Context(), signingRun(tt.socket, tt.targets...))
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			testsupport.RequireID(t, "RJ-204", err)
			assert.Equal(t, "signing-socket", cli.Of(err).Slug)
			assert.Equal(t, 2, cli.Of(err).Exit)
			assert.ErrorIs(t, err, tt.want)
			assert.Contains(t, err.Error(), "the project shop uses the kit git-ssh-sign")
		})
	}
}
