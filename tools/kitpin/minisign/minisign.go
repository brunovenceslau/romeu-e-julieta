// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package minisign verifies a minisign signature against a public key,
// as "minisign -V" does with its default settings (07 7.4, Q31).
//
// The formats are those of minisign 0.12 (src/minisign.c and
// src/minisign.h of github.com/jedisct1/minisign):
//
//   - a public key file is two lines: "untrusted comment: <text>", then
//     the base64 of the two bytes "Ed", the 8-byte key id and the
//     32-byte Ed25519 public key;
//   - a signature file is four lines: "untrusted comment: <text>", the
//     base64 of the algorithm, the 8-byte key id and the 64-byte
//     signature, "trusted comment: <text>", and the base64 of the
//     64-byte global signature.
//
// The signature is Ed25519 over the BLAKE2b-512 hash of the file, the
// prehashed mode, whose algorithm bytes are "ED". The global signature
// is Ed25519 over the signature followed by the trusted comment, so
// the trusted comment cannot be changed either. The legacy mode "Ed",
// Ed25519 over the file itself, is refused, as minisign refuses it
// without -l. Nothing here skips a step: every call checks the key id,
// the signature and the global signature, in that order.
package minisign

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/blake2b"
)

const (
	untrustedPrefix = "untrusted comment: "
	trustedPrefix   = "trusted comment: "
	keyIDSize       = 8
	// algKey is the algorithm of a public key; algHashed of a prehashed
	// signature and algLegacy of a legacy one.
	algKey    = "Ed"
	algHashed = "ED"
	algLegacy = "Ed"
)

// PublicKey is a minisign public key.
type PublicKey struct {
	ID  [keyIDSize]byte
	Key ed25519.PublicKey
}

// KeyID is the key id as minisign prints it: the 8 bytes read as a
// little-endian number, in 16 upper-case hex digits.
func (k PublicKey) KeyID() string {
	return keyIDString(k.ID)
}

func keyIDString(id [keyIDSize]byte) string {
	return fmt.Sprintf("%016X", binary.LittleEndian.Uint64(id[:]))
}

// ParsePublicKey reads a public key file.
func ParsePublicKey(file []byte) (PublicKey, error) {
	lines, err := splitLines(file, 2)
	if err != nil {
		return PublicKey{}, fmt.Errorf("public key: %w", err)
	}
	if !strings.HasPrefix(lines[0], untrustedPrefix) {
		return PublicKey{}, fmt.Errorf("public key: the first line does not start with %q", untrustedPrefix)
	}
	raw, err := decode(lines[1], 2+keyIDSize+ed25519.PublicKeySize)
	if err != nil {
		return PublicKey{}, fmt.Errorf("public key: %w", err)
	}
	if string(raw[:2]) != algKey {
		return PublicKey{}, fmt.Errorf("public key: the algorithm is %q, not %q", raw[:2], algKey)
	}
	var k PublicKey
	copy(k.ID[:], raw[2:2+keyIDSize])
	k.Key = ed25519.PublicKey(bytes.Clone(raw[2+keyIDSize:]))
	return k, nil
}

// Verify checks that sig, the bytes of a signature file, signs message
// with key, and returns the trusted comment.
func Verify(key PublicKey, message, sig []byte) (string, error) {
	lines, err := splitLines(sig, 4)
	if err != nil {
		return "", fmt.Errorf("signature: %w", err)
	}
	if !strings.HasPrefix(lines[0], untrustedPrefix) {
		return "", fmt.Errorf("signature: the first line does not start with %q", untrustedPrefix)
	}
	raw, err := decode(lines[1], 2+keyIDSize+ed25519.SignatureSize)
	if err != nil {
		return "", fmt.Errorf("signature: %w", err)
	}
	switch alg := string(raw[:2]); alg {
	case algHashed:
	case algLegacy:
		return "", errors.New("signature: the legacy mode \"Ed\" signs the file without hashing it; minisign refuses it without -l, and so does this check")
	default:
		return "", fmt.Errorf("signature: the algorithm %q is not %q", alg, algHashed)
	}
	trusted, ok := strings.CutPrefix(lines[2], trustedPrefix)
	if !ok {
		return "", fmt.Errorf("signature: the third line does not start with %q", trustedPrefix)
	}
	if !printable(trusted) {
		return "", errors.New("signature: the trusted comment holds a control character or is not UTF-8")
	}
	global, err := decode(lines[3], ed25519.SignatureSize)
	if err != nil {
		return "", fmt.Errorf("signature: the global signature: %w", err)
	}
	var id [keyIDSize]byte
	copy(id[:], raw[2:2+keyIDSize])
	if id != key.ID {
		return "", fmt.Errorf("signature: made with the key %s, not with the key %s", keyIDString(id), key.KeyID())
	}
	signature := raw[2+keyIDSize:]
	hash := blake2b.Sum512(message)
	if !ed25519.Verify(key.Key, hash[:], signature) {
		return "", errors.New("signature: verification failed: the file is not the one the key signed")
	}
	if !ed25519.Verify(key.Key, append(bytes.Clone(signature), trusted...), global) {
		return "", errors.New("signature: the trusted comment's verification failed")
	}
	return trusted, nil
}

// printable reports whether s is UTF-8 with no control character but
// the tab, minisign's is_printable.
func printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r != '\t' && unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// splitLines returns the n lines of file, which ends in one newline or
// none. A carriage return before a newline is dropped, as minisign's
// trim drops it.
func splitLines(file []byte, n int) ([]string, error) {
	text := strings.TrimSuffix(string(file), "\n")
	lines := strings.Split(text, "\n")
	if len(lines) != n {
		return nil, fmt.Errorf("the file has %d lines, not %d", len(lines), n)
	}
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines, nil
}

// decode returns the bytes of the strict base64 s, which must be size
// long.
func decode(s string, size int) ([]byte, error) {
	raw, err := base64.StdEncoding.Strict().DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("not base64: %w", err)
	}
	if len(raw) != size {
		return nil, fmt.Errorf("%d bytes where %d are expected", len(raw), size)
	}
	return raw, nil
}
