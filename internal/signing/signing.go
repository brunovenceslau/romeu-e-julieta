// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package signing checks the dedicated host signing socket of
// git-ssh-sign (06 6.4, I34): signing.agentSocket must be set, reachable
// and hold exactly one key, the kit's signingKey. It speaks the
// identity listing of the ssh-agent protocol itself, with no dependency
// and no subprocess (REQ-0601). The package is dark until T080, whose
// sync and preflight turn its errors into the error signing-socket
// (RJ-204).
//
// The check holds at the moment it runs: an agent that is given a second
// key afterwards is not seen until the next check, which is why it runs
// in sync, in doctor and in the preflight of every P command of the
// project (06 6.4), and why the socket must be one the user owns.
package signing

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// The messages of the identity listing (draft-miller-ssh-agent, 4.4).
const (
	requestIdentities = 11 // SSH_AGENTC_REQUEST_IDENTITIES
	identitiesAnswer  = 12 // SSH_AGENT_IDENTITIES_ANSWER
	agentFailure      = 5  // SSH_AGENT_FAILURE
)

// The bounds of an answer, far above an agent that holds one key: the
// length of the message and the number of keys it lists.
const (
	maxMessage = 256 << 10
	maxKeys    = 1024
)

// timeout bounds the whole listing when ctx sets no earlier deadline:
// an agent answers at once, and one that does not is unreachable.
const timeout = 5 * time.Second

// The reasons Check refuses a socket, each wrapped with its detail.
var (
	// ErrNoSocket: signing.agentSocket is not set.
	ErrNoSocket = errors.New("signing: no agent socket is set")
	// ErrUnreachable: the socket does not answer the identity listing.
	ErrUnreachable = errors.New("signing: the agent socket is unreachable")
	// ErrKeyCount: the agent holds no key, or more than one.
	ErrKeyCount = errors.New("signing: the agent does not hold exactly one key")
	// ErrOtherKey: the one key of the agent is not the signing key.
	ErrOtherKey = errors.New("signing: the agent's key is not the signing key")
	// ErrSigningKey: the signing key is not an OpenSSH public key line.
	ErrSigningKey = errors.New("signing: the signing key is not an OpenSSH public key")
	// ErrSocket: the path is not an absolute path to a socket this
	// process's user owns.
	ErrSocket = errors.New("signing: the agent socket is not a socket of this user")
)

// Identity is one key an agent lists: the public key blob of the wire
// format, and its comment. Both are what the agent says: the comment is
// untrusted text, printed escaped if at all.
type Identity struct {
	Blob    []byte
	Comment string
}

// Type returns the key type the blob names, such as ssh-ed25519.
func (id Identity) Type() string {
	if len(id.Blob) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(id.Blob)
	if uint64(n) > uint64(len(id.Blob)-4) {
		return ""
	}
	return string(id.Blob[4 : 4+n])
}

// Identities asks the agent at socket for the keys it holds. socket is
// the absolute path of a socket this process's user owns, not a symbolic
// link, so a socket another user planted, or one in Linux's abstract
// namespace, is ErrSocket. Any failure to connect, a refusal and an
// answer that is not a well formed listing are ErrUnreachable, wrapped
// with what went wrong and, when ctx ended, with its error.
func Identities(ctx context.Context, socket string) (ids []Identity, err error) {
	if socket == "" {
		return nil, ErrNoSocket
	}
	if err := checkSocket(socket); err != nil {
		return nil, err
	}
	parent := ctx
	defer func() {
		if err == nil {
			return
		}
		cause := parent.Err()
		if d, ok := parent.Deadline(); ok && cause == nil && errors.Is(err, os.ErrDeadlineExceeded) && !time.Now().Before(d) {
			// The socket's deadline is the caller's and may fire before
			// the caller's own timer does.
			cause = context.DeadlineExceeded
		}
		if cause != nil && !errors.Is(err, cause) {
			err = fmt.Errorf("%w: %w", err, cause)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
		}
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	if _, err := conn.Write([]byte{0, 0, 0, 1, requestIdentities}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	msg, err := readMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	ids, err = parseIdentities(msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	return ids, nil
}

// checkSocket returns ErrSocket for a path that is not absolute, names
// Linux's abstract namespace (a leading @ or NUL), is a symbolic link or
// anything but a socket, or belongs to another user. A missing path is
// left to the dial, which reports it as unreachable.
func checkSocket(socket string) error {
	if !filepath.IsAbs(socket) || strings.ContainsRune(socket, 0) {
		return fmt.Errorf("%w: %q is not an absolute path", ErrSocket, socket)
	}
	info, err := os.Lstat(socket)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSocket, err)
	}
	if info.Mode().Type() != fs.ModeSocket {
		return fmt.Errorf("%w: %q is a %s, not a socket", ErrSocket, socket, info.Mode().Type())
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%w: %q belongs to another user", ErrSocket, socket)
	}
	return nil
}

// readMessage reads one length-prefixed message of the agent protocol.
func readMessage(r io.Reader) ([]byte, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(head[:])
	if n == 0 || n > maxMessage {
		return nil, fmt.Errorf("an answer of %d bytes", n)
	}
	msg := make([]byte, n)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// parseIdentities reads an identities answer: its type, the number of
// keys, then a key blob and a comment for each, with nothing after.
func parseIdentities(msg []byte) ([]Identity, error) {
	switch msg[0] {
	case identitiesAnswer:
	case agentFailure:
		return nil, errors.New("the agent refused the listing")
	default:
		return nil, fmt.Errorf("an answer of type %d", msg[0])
	}
	rest := msg[1:]
	if len(rest) < 4 {
		return nil, errors.New("an answer with no key count")
	}
	n := binary.BigEndian.Uint32(rest)
	rest = rest[4:]
	if n > maxKeys {
		return nil, fmt.Errorf("an answer of %d keys", n)
	}
	ids := make([]Identity, 0, n)
	for range n {
		blob, after, err := readString(rest)
		if err != nil {
			return nil, err
		}
		comment, after, err := readString(after)
		if err != nil {
			return nil, err
		}
		ids = append(ids, Identity{Blob: blob, Comment: string(comment)})
		rest = after
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("%d bytes after the last key", len(rest))
	}
	return ids, nil
}

// readString reads one string of the wire format: a length, then that
// many bytes.
func readString(b []byte) (s, rest []byte, err error) {
	if len(b) < 4 {
		return nil, nil, errors.New("a string with no length")
	}
	n := binary.BigEndian.Uint32(b)
	if uint64(n) > uint64(len(b)-4) {
		return nil, nil, fmt.Errorf("a string of %d bytes in %d", n, len(b)-4)
	}
	return b[4 : 4+n], b[4+n:], nil
}

// clip returns at most n bytes of s, which the agent chose.
func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// ParseSigningKey returns the key blob of an OpenSSH public key line,
// "<type> <base64> [comment]", as the kit's signingKey arg holds it
// (03 3.2), after checking that the blob names the line's type.
func ParseSigningKey(line string) ([]byte, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, fmt.Errorf("%w: %q has no key", ErrSigningKey, line)
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSigningKey, err)
	}
	if got := (Identity{Blob: blob}).Type(); got != fields[0] {
		return nil, fmt.Errorf("%w: the line says %q and its key %q", ErrSigningKey, fields[0], got)
	}
	return blob, nil
}

// Check returns nil when the agent at socket holds exactly one key and
// it is signingKey, an OpenSSH public key line; otherwise ErrNoSocket,
// ErrSocket, ErrUnreachable, ErrKeyCount, ErrOtherKey or ErrSigningKey,
// wrapped with the detail (I34). The comments of the keys play no part.
func Check(ctx context.Context, socket, signingKey string) error {
	want, err := ParseSigningKey(signingKey)
	if err != nil {
		return err
	}
	ids, err := Identities(ctx, socket)
	if err != nil {
		return err
	}
	if len(ids) != 1 {
		return fmt.Errorf("%w: it holds %d", ErrKeyCount, len(ids))
	}
	if !bytes.Equal(ids[0].Blob, want) {
		return fmt.Errorf("%w: it holds a %q key", ErrOtherKey, clip(ids[0].Type(), 64))
	}
	return nil
}
