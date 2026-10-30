// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e && (linux || darwin)

package e2e

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// signingAgent is an in-process ssh-agent on a unix socket, standing for
// the dedicated signing agent of 06 6.4.
type signingAgent struct {
	socket  string
	keyring agent.Agent
}

// newSigningAgent serves an agent that holds no key, from a directory
// with a short path, since a socket path holds at most 104 bytes on
// macOS.
func newSigningAgent(t *testing.T) *signingAgent {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "se"+rand.Text()[:8])
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	a := &signingAgent{socket: filepath.Join(dir, "s"), keyring: agent.NewKeyring()}
	ln, err := new(net.ListenConfig).Listen(t.Context(), "unix", a.socket)
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
				_ = agent.ServeAgent(a.keyring, conn)
			}()
		}
	}()
	return a
}

// newKey returns a new ed25519 key and its public line, as the kit's
// signingKey holds it: the type and the base64 body, no comment.
func newKey(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return priv, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
}

// add gives the agent the key priv.
func (a *signingAgent) add(t *testing.T, priv ed25519.PrivateKey) {
	t.Helper()
	require.NoError(t, a.keyring.Add(agent.AddedKey{PrivateKey: priv}))
}

// fooSigned is the spec of foo with the real product kit git-ssh-sign
// of the romeu build added, its signingKey key.
func fooSigned(key string) string {
	return strings.Replace(fooSpec, "  - product: julieta\n",
		"  - product: julieta\n  - product: git-ssh-sign\n    args: {signingKey: \""+key+"\"}\n", 1)
}

// setSocket points signing.agentSocket of the host settings at socket.
func (f syncFixture) setSocket(t *testing.T, socket string) {
	t.Helper()
	f.e.AppendSettings(t, "signing:\n  agentSocket: "+socket+"\n")
}

// doctorSession sets the fake sbx to the recorded reads doctor makes
// (the handshake's version, ls and policy listing, and the secret
// listing of secrets-settings), after sync's handshake.
func (f syncFixture) doctorSession(t *testing.T) {
	t.Helper()
	lines := sessionLines(t, "handshake.jsonl", []string{"version"}, []string{"ls", "--json"}, argvPolicyList)
	lines = append(lines, sessionLines(t, "secrets-settings.jsonl", argvSecretList)[1:]...)
	session := filepath.Join(filepath.Dir(f.e.Bin), "doctor-session.jsonl")
	require.NoError(t, os.WriteFile(session, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	require.NoError(t, f.e.SetSbxSession(session))
}

// TestSigningSyncRefusesTheKitWithoutItsKey is the E case of I34 on the
// romeu binary (06 6.4, REQ-0600): the first sync that renders the real
// kit git-ssh-sign exits 2 with signing-socket (RJ-204) unless
// signing.agentSocket is set, reachable and holds exactly one key, the
// kit's signingKey; nothing is promoted or approved, and no gate 2
// prompt is reached.
//
//romeu:invariant I34 test
func TestSigningSyncRefusesTheKitWithoutItsKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// keys are the keys the agent holds: "signing" for the kit's
		// key, "other" for another; nil sets no socket at all.
		keys   []string
		socket bool
		says   string
	}{
		{"no socket set", nil, false, "no agent socket is set"},
		{"a socket that answers nothing", nil, true, "unreachable"},
		{"no key", []string{}, true, "it holds 0"},
		{"two keys, the signing key among them", []string{"signing", "other"}, true, "it holds 2"},
		{"one other key", []string{"other"}, true, "not the signing key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newSyncFixture(t)
			signing, key := newKey(t)
			other, _ := newKey(t)
			f.o.Push(t, "owner/cfg", "main", map[string]string{"projects/foo.yaml": fooSigned(key)})
			if tc.socket {
				socket := filepath.Join(os.TempDir(), "gone"+rand.Text()[:8])
				if tc.keys != nil {
					a := newSigningAgent(t)
					for _, k := range tc.keys {
						a.add(t, map[string]ed25519.PrivateKey{"signing": signing, "other": other}[k])
					}
					socket = a.socket
				}
				f.setSocket(t, socket)
			}

			res := f.e.Run(t, []string{"sync", "foo"})

			assert.Equal(t, 2, res.Code, res.Stderr)
			assert.Contains(t, res.Stderr, "RJ-204")
			assert.Contains(t, res.Stderr, "signing-socket")
			assert.Contains(t, res.Stderr, "the project foo uses the kit git-ssh-sign")
			assert.Contains(t, res.Stderr, tc.says)
			assert.NotContains(t, res.Stderr, "gate 2", "the rule runs before gate 2")
			assert.NoFileExists(t, f.env("sbxenv.yaml"), "nothing is promoted")
			assert.NoFileExists(t, filepath.Join(f.e.StateDir(), "projects", "foo.json"), "nothing is approved")
		})
	}
}

// TestSigningKitAddedThroughGate2 is the life of the kit on the romeu
// binary: an approved foo that adds git-ssh-sign shows the kit's
// capability set in the gate 2 diff of the first sync that renders it,
// which passes the rule with the one key in the socket (06 6.4); doctor
// then passes signing-socket for foo, and once the agent holds a second
// key doctor fails it and sync refuses again with RJ-204, so doctor and
// sync run the same check (REQ-0384, REQ-0601).
//
//romeu:invariant I34 test
func TestSigningKitAddedThroughGate2(t *testing.T) {
	t.Parallel()
	f := newSyncFixture(t)
	f.approveFoo(t)
	a := newSigningAgent(t)
	signing, key := newKey(t)
	a.add(t, signing)
	f.setSocket(t, a.socket)
	f.o.Push(t, "owner/cfg", "main", map[string]string{"projects/foo.yaml": fooSigned(key)})

	res := f.approveFoo(t)

	assert.Contains(t, res.Stderr, "gate 2: widening of the project foo")
	assert.Contains(t, res.Stderr, "git-ssh-sign", "the diff names the kit")
	assert.Contains(t, res.Stderr, "ssh-agent", "the diff shows the kit's capability")
	assert.Contains(t, res.Stdout, "promoted  foo")
	rec, err := os.ReadFile(f.env(".romeu/render.json"))
	require.NoError(t, err)
	assert.Contains(t, string(rec), "git-ssh-sign", "the live render holds the kit")

	f.doctorSession(t)
	rep, out := runDoctor(t, f.e)
	got, _ := rep.result(t, "signing-socket")
	assert.Equal(t, "pass", got, out.Stdout+out.Stderr)

	other, _ := newKey(t)
	a.add(t, other)
	rep, out = runDoctor(t, f.e)
	got, _ = rep.result(t, "signing-socket")
	assert.Equal(t, "fail", got, out.Stdout+out.Stderr)
	assert.Equal(t, 6, out.Code, out.Stderr)

	res = f.e.Run(t, []string{"sync", "foo"})
	assert.Equal(t, 2, res.Code, res.Stderr)
	assert.Contains(t, res.Stderr, "RJ-204")
	assert.Contains(t, res.Stderr, "it holds 2")
}
