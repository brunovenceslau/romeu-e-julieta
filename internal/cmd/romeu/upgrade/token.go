// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// The finish token binds romeu upgrade --finish to a swap that just
// happened (REQ-1330, T196): right before its exec, the swap step writes
// it in the romeu state directory, and the finish reads it, deletes it
// and refuses unless it names the running version it was given, the
// sha256 of its own executable and an expiry not yet passed. A finish
// that no swap started finds none and writes nothing. The file name and
// the line format are part of the interface every later release keeps,
// as the two argv forms are, since the old binary writes it and the new
// one reads it:
//
//	romeu-upgrade-finish/1
//	version <the version of the romeu that ran the swap>
//	sha256 <the hex sha256 of the new binary>
//	expires <the expiry, RFC 3339 in UTC>
//
// each line ended by a line feed, and nothing else.
const (
	// tokenName is the name of the token in the state directory.
	tokenName = "upgrade-finish"
	// tokenHeader is the first line, which names the format.
	tokenHeader = "romeu-upgrade-finish/1"
	// tokenLife is how long after the swap the token holds.
	tokenLife = 60 * time.Second
	// tokenLimit bounds what the finish reads of the file: a token is
	// under 200 bytes.
	tokenLimit = 4096
)

// finishToken is what the token holds.
type finishToken struct {
	// version is the version of the romeu that ran the swap, which the
	// finish takes as --finish.
	version string
	// sum is the sha256 of the new binary's bytes.
	sum [sha256.Size]byte
	// expires is when the token stops holding.
	expires time.Time
}

// encode returns the token in its line format.
func (t finishToken) encode() []byte {
	return fmt.Appendf(nil, "%s\nversion %s\nsha256 %s\nexpires %s\n",
		tokenHeader, t.version, hex.EncodeToString(t.sum[:]), t.expires.UTC().Format(time.RFC3339))
}

// errTokenFormat is the error of a token that is not of the line format.
var errTokenFormat = errors.New("the token is not of the format " + tokenHeader)

// parseToken reads a token in its line format: the four lines, in their
// order, and nothing else.
func parseToken(data []byte) (finishToken, error) {
	lines := strings.Split(string(data), "\n")
	if len(lines) != 5 || lines[4] != "" || lines[0] != tokenHeader {
		return finishToken{}, errTokenFormat
	}
	version, ok1 := strings.CutPrefix(lines[1], "version ")
	sum, ok2 := strings.CutPrefix(lines[2], "sha256 ")
	expires, ok3 := strings.CutPrefix(lines[3], "expires ")
	if !ok1 || !ok2 || !ok3 || !runningVersion(version) {
		return finishToken{}, errTokenFormat
	}
	var t finishToken
	t.version = version
	raw, err := hex.DecodeString(sum)
	if err != nil || len(raw) != sha256.Size || hex.EncodeToString(raw) != sum {
		return finishToken{}, errTokenFormat
	}
	copy(t.sum[:], raw)
	if t.expires, err = time.Parse(time.RFC3339, expires); err != nil {
		return finishToken{}, errTokenFormat
	}
	return t, nil
}

// writeToken writes t into the state directory dir, mode 0600, made
// with O_EXCL and never through a link: a token a swap whose exec failed
// left is removed first.
func writeToken(dir string, t finishToken) (err error) {
	path := filepath.Join(dir, tokenName)
	if err := removeToken(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = removeToken(path)
		}
	}()
	if _, err := f.Write(t.encode()); err != nil {
		return err
	}
	return f.Sync()
}

// consumeToken reads the token of the state directory dir and deletes
// it, so a token serves one finish. It refuses a token that is absent, a
// link or no regular file, not owned by the user or of another mode than
// 0600, over tokenLimit, or not of the line format; whatever stood at
// its path is removed but an absent one.
func consumeToken(dir string) (finishToken, error) {
	path := filepath.Join(dir, tokenName)
	// O_NONBLOCK keeps a FIFO planted at the path from blocking the open.
	f, openErr := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(openErr, fs.ErrNotExist) {
		return finishToken{}, errors.New("no swap left a token")
	}
	rmErr := removeToken(path)
	if openErr != nil {
		return finishToken{}, fmt.Errorf("the token does not open: %w", openErr)
	}
	defer func() { _ = f.Close() }()
	if rmErr != nil {
		return finishToken{}, fmt.Errorf("the token is not deleted: %w", rmErr)
	}
	info, err := f.Stat()
	if err != nil {
		return finishToken{}, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode() != 0o600 || !ok || int(st.Uid) != os.Getuid() {
		return finishToken{}, fmt.Errorf("the token is not a regular file of this user, mode 0600 (mode %s)", info.Mode())
	}
	data, err := io.ReadAll(io.LimitReader(f, tokenLimit+1))
	if err != nil {
		return finishToken{}, err
	}
	if len(data) > tokenLimit {
		return finishToken{}, fmt.Errorf("the token passes %d bytes", tokenLimit)
	}
	return parseToken(data)
}

// removeToken removes the token at path: a file this command or the
// swap of the romeu before it made, which serves one finish (I16).
func removeToken(path string) error { return os.Remove(path) }

// fileSum returns the sha256 of the file at path, read through no link
// at its last element.
func fileSum(path string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return sum, err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, err
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
