// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package signing

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireString writes s as a string of the agent wire format.
func wireString(s []byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, uint32(len(s))), s...)
}

// keyBlob returns a public key blob of the type, with body as its key
// data.
func keyBlob(kind, body string) []byte {
	return append(wireString([]byte(kind)), wireString([]byte(body))...)
}

// keyLine returns the OpenSSH public key line of blob.
func keyLine(kind string, blob []byte) string {
	return kind + " " + base64.StdEncoding.EncodeToString(blob) + " comment"
}

// answer is the identities answer that lists the blobs.
func answer(blobs ...[]byte) []byte {
	msg := []byte{identitiesAnswer}
	msg = binary.BigEndian.AppendUint32(msg, uint32(len(blobs)))
	for _, b := range blobs {
		msg = append(msg, wireString(b)...)
		msg = append(msg, wireString([]byte("a key"))...)
	}
	return msg
}

// framed is msg with its length in front.
func framed(msg []byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, uint32(len(msg))), msg...)
}

// shortDir returns a new temporary directory with a short path, since a
// socket path holds at most 104 bytes on macOS.
func shortDir(t *testing.T) string {
	t.Helper()
	// Not os.MkdirTemp, which usetesting flags, and not t.TempDir(),
	// whose path carries the name of the test and is too long for the
	// 104-byte socket path this directory is for.
	dir := filepath.Join(os.TempDir(), "sig"+rand.Text()[:8])
	require.NoError(t, os.Mkdir(dir, 0o700))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// fakeAgent listens on a unix socket in a short temporary directory and
// answers each connection's first request with reply, after checking
// that it is the identity listing. It returns the socket's path.
func fakeAgent(t *testing.T, reply []byte) string {
	t.Helper()
	socket := filepath.Join(shortDir(t), "agent.sock")
	l, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				req := make([]byte, 5)
				if _, err := io.ReadFull(conn, req); err != nil || req[4] != requestIdentities {
					return
				}
				_, _ = conn.Write(reply)
			}()
		}
	}()
	return socket
}

// TestIdentities lists the keys of an agent with their types and
// comments.
func TestIdentities(t *testing.T) {
	a, b := keyBlob("ssh-ed25519", "one"), keyBlob("ecdsa-sha2-nistp256", "two")
	ids, err := Identities(t.Context(), fakeAgent(t, framed(answer(a, b))))
	require.NoError(t, err)
	assert.Equal(t, []Identity{{Blob: a, Comment: "a key"}, {Blob: b, Comment: "a key"}}, ids)
	assert.Equal(t, "ssh-ed25519", ids[0].Type())
	assert.Equal(t, "ecdsa-sha2-nistp256", ids[1].Type())
}

// TestCheck is the identity listing table of I34: no socket, a socket
// nothing listens on, no key, two keys, one other key, and the one key.
//
//romeu:invariant I34 test
func TestCheck(t *testing.T) {
	signing := keyBlob("ssh-ed25519", "signing")
	other := keyBlob("ssh-ed25519", "other")
	line := keyLine("ssh-ed25519", signing)
	gone := filepath.Join(shortDir(t), "gone.sock")
	tests := []struct {
		name   string
		socket string
		want   error
	}{
		{"no socket", "", ErrNoSocket},
		{"unreachable", gone, ErrUnreachable},
		{"no key", fakeAgent(t, framed(answer())), ErrKeyCount},
		{"two keys", fakeAgent(t, framed(answer(signing, other))), ErrKeyCount},
		{"one other key", fakeAgent(t, framed(answer(other))), ErrOtherKey},
		{"the one key", fakeAgent(t, framed(answer(signing))), nil},
	}
	for _, tt := range tests {
		err := Check(t.Context(), tt.socket, line)
		if tt.want == nil {
			require.NoError(t, err, tt.name)
			continue
		}
		require.ErrorIs(t, err, tt.want, tt.name)
	}
}

// TestCheckRefusesTheSigningKey: a signing key that is not an OpenSSH
// public key line is refused before the socket is read.
func TestCheckRefusesTheSigningKey(t *testing.T) {
	blob := keyBlob("ssh-ed25519", "k")
	for _, line := range []string{"", "ssh-ed25519", "ssh-ed25519 !!!", "ssh-rsa " + base64.StdEncoding.EncodeToString(blob), "ssh-ed25519 " + base64.StdEncoding.EncodeToString([]byte{0, 0})} {
		require.ErrorIs(t, Check(t.Context(), "", line), ErrSigningKey, "%q", line)
	}
	got, err := ParseSigningKey(keyLine("ssh-ed25519", blob))
	require.NoError(t, err)
	assert.Equal(t, blob, got)
}

// TestIdentitiesRefusesBadAnswers: a refusal, another message type, an
// answer over the bound, a key count over the bound, a truncated key,
// bytes after the last key, and an agent that closes or never answers
// are all ErrUnreachable.
func TestIdentitiesRefusesBadAnswers(t *testing.T) {
	big := binary.BigEndian.AppendUint32(nil, maxMessage+1)
	manyKeys := binary.BigEndian.AppendUint32([]byte{identitiesAnswer}, maxKeys+1)
	truncated := append(binary.BigEndian.AppendUint32([]byte{identitiesAnswer}, 1), 0, 0, 0, 9, 'x')
	trailing := append(answer(keyBlob("ssh-ed25519", "k")), 0)
	for name, reply := range map[string][]byte{
		"failure":       framed([]byte{agentFailure}),
		"another type":  framed([]byte{99}),
		"empty":         framed(nil),
		"too long":      big,
		"no count":      framed([]byte{identitiesAnswer, 0}),
		"too many keys": framed(manyKeys),
		"truncated key": framed(truncated),
		"no comment":    framed(append(binary.BigEndian.AppendUint32([]byte{identitiesAnswer}, 1), wireString([]byte("k"))...)),
		"trailing":      framed(trailing),
		"closed":        nil,
	} {
		_, err := Identities(t.Context(), fakeAgent(t, reply))
		require.ErrorIs(t, err, ErrUnreachable, name)
	}
	assert.Empty(t, Identity{Blob: []byte{0, 0, 0, 9}}.Type(), "a blob shorter than its type")
	assert.Empty(t, Identity{}.Type())
}

// TestIdentitiesStopsWithTheContext: an agent that accepts and never
// answers is ErrUnreachable once the context ends, by its deadline or by
// a cancel with no deadline, well before the timeout of the listing, and
// the error holds the context's.
func TestIdentitiesStopsWithTheContext(t *testing.T) {
	socket := filepath.Join(shortDir(t), "silent.sock")
	l, err := net.Listen("unix", socket)
	require.NoError(t, err)
	conns := make(chan net.Conn, 8)
	t.Cleanup(func() {
		_ = l.Close()
		for {
			select {
			case c := <-conns:
				_ = c.Close()
			default:
				return
			}
		}
	})
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conns <- conn
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = Identities(ctx, socket)
	require.ErrorIs(t, err, ErrUnreachable)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), timeout/2, "the deadline of ctx ends the read")

	ctx, cancel = context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)
	start = time.Now()
	_, err = Identities(ctx, socket)
	require.ErrorIs(t, err, ErrUnreachable)
	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), timeout/2, "a cancel with no deadline ends the read")
}

// TestIdentitiesRefusesTheSocketPath: a relative path, the abstract
// namespace, a regular file and a symbolic link to a good socket are
// ErrSocket, and none is dialed.
func TestIdentitiesRefusesTheSocketPath(t *testing.T) {
	good := fakeAgent(t, framed(answer(keyBlob("ssh-ed25519", "k"))))
	dir := shortDir(t)
	file := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(good, link))
	for _, socket := range []string{"agent.sock", "@agent", "\x00agent", file, link} {
		_, err := Identities(t.Context(), socket)
		require.ErrorIs(t, err, ErrSocket, "%q", socket)
	}
	_, err := Identities(t.Context(), good)
	require.NoError(t, err, "the socket itself")
}

// TestCheckEscapesTheAgentKeyType: the type of another key, which the
// agent chose, reaches the error quoted and clipped.
func TestCheckEscapesTheAgentKeyType(t *testing.T) {
	signing := keyBlob("ssh-ed25519", "signing")
	hostile := keyBlob("\x1b[2J"+strings.Repeat("x", 100), "k")
	err := Check(t.Context(), fakeAgent(t, framed(answer(hostile))), keyLine("ssh-ed25519", signing))
	require.ErrorIs(t, err, ErrOtherKey)
	assert.NotContains(t, err.Error(), "\x1b", "no raw escape")
	assert.Contains(t, err.Error(), `"\x1b[2J`)
	assert.Less(t, len(err.Error()), 200)
}
