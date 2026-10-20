// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package signing checks the dedicated host signing socket of
// git-ssh-sign (06 6.4, I34): signing.agentSocket must be set, reachable
// and hold exactly one key, the kit's signingKey. It reads the identity
// listing of the ssh-agent protocol through golang.org/x/crypto/ssh/agent,
// with no subprocess (REQ-0601, T147). The package is dark until T080, whose
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

	"golang.org/x/crypto/ssh/agent"
)

// maxRead bounds the bytes read from the agent, far above an agent that
// holds one key and below the library's own bound of 16 MiB.
const maxRead = 256 << 10

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
// answer the library does not read as a listing are ErrUnreachable,
// wrapped with what went wrong and, when ctx ended, with its error.
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
		if d, ok := parent.Deadline(); ok && cause == nil && !time.Now().Before(d) {
			// The socket's deadline is the caller's and may fire before
			// the caller's own timer does. The library keeps only the
			// text of a read error, so the deadline is read from the
			// clock, not from the error.
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
	// The client gets the connection without its Close, so it runs the
	// one call in place and starts no reader goroutine (agent.NewClient).
	rw := &countingConn{Reader: io.LimitReader(conn, maxRead), Writer: conn}
	keys, err := agent.NewClient(rw).List()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	// The library reads the whole answer and parses only the keys its
	// count names, so an answer that counts fewer keys than it holds
	// would pass I34 with a second key unseen. The answer must be
	// exactly its length, its type, its count and the keys listed.
	want := int64(4 + 1 + 4)
	ids = make([]Identity, 0, len(keys))
	for _, k := range keys {
		want += int64(4 + len(k.Blob) + 4 + len(k.Comment))
		ids = append(ids, Identity{Blob: k.Blob, Comment: k.Comment})
	}
	if rw.read != want {
		return nil, fmt.Errorf("%w: %d bytes after the last key", ErrUnreachable, rw.read-want)
	}
	return ids, nil
}

// countingConn counts the bytes read through it. It has no Close, which
// keeps agent.NewClient on its serial path.
type countingConn struct {
	io.Reader
	io.Writer
	read int64
}

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.read += int64(n)
	return n, err
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
