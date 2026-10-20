// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package canon holds every digest the gates compare and the one ULID
// writer (03 3.12).
//
// A structured value is digested with Digest: sha256 over
// "romeu/<kind>/v1", a NUL byte and the canonical JSON of the value, the
// RFC 8785 form of encoding/json/jsontext. The kind is in the digest's
// type, Sum, so digests of two kinds never compare. Raw file bytes are not digested here: they use plain sha256,
// which sha256sum and shasum reproduce.
//
// Every ULID the product writes comes from NewULID, in lowercase
// Crockford base32, and ParseULID refuses any other spelling.
package canon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ErrKind is the error of a kind value that is not the one constant of
// its kind type, such as MemoryContentKind("fingerprint").
var ErrKind = errors.New("canon: not a kind of the table")

// ErrInvalidText is the error of a string of a value, or a key, that
// holds U+FFFD: Digest writes each byte that is not UTF-8, and a
// MarshalJSON method's lone surrogate, as U+FFFD, so two strings that
// differ in those bytes would share one digest.
var ErrInvalidText = errors.New("canon: a string holds U+FFFD, the mark of text that is not UTF-8")

// ErrInvalidDigest is the error of a digest text that is not 64
// lowercase hexadecimal digits.
var ErrInvalidDigest = errors.New("canon: invalid digest")

// Sum is a digest of the kind K. Its text is 64 lowercase hexadecimal
// digits.
type Sum[K Kind] [sha256.Size]byte

// String returns the digest as lowercase hexadecimal.
func (s Sum[K]) String() string { return hex.EncodeToString(s[:]) }

// MarshalText returns the digest as lowercase hexadecimal.
func (s Sum[K]) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText reads a digest written by MarshalText. It refuses
// uppercase digits, so that one digest has one text, and leaves s as it
// was on an error.
func (s *Sum[K]) UnmarshalText(text []byte) error {
	if len(text) != hex.EncodedLen(sha256.Size) || bytes.ContainsFunc(text, notLowerHex) {
		return fmt.Errorf("%w: %q", ErrInvalidDigest, clip(string(text)))
	}
	var sum Sum[K]
	if _, err := hex.Decode(sum[:], text); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDigest, err)
	}
	*s = sum
	return nil
}

// notLowerHex reports whether r is not a lowercase hexadecimal digit.
func notLowerHex(r rune) bool {
	return (r < '0' || r > '9') && (r < 'a' || r > 'f')
}

// Digest returns the digest of v as the kind kind (03 3.12): sha256 over
// "romeu/<kind>/v1", a NUL byte and the canonical JSON of v. The type of
// v declares its one kind (Input), so the kind argument names at the
// call site what the type already fixes.
//
// The canonical JSON is what encoding/json/v2 writes for v, nil lists
// and maps as null, canonicalized by RFC 8785 (the JSON Canonicalization
// Scheme): keys sorted by their UTF-16 code units, numbers in the
// ECMAScript form, so 1 and 1.0 have one digest, minimal string escapes
// and no insignificant whitespace; an integer beyond 2^53 keeps its
// digits, which RFC 8785 would round. Digest fails on a value
// encoding/json/v2 cannot write, on an object with a duplicate key,
// which only a MarshalJSON method can write, on a kind value other than
// its constant (ErrKind), and on a string or key that holds U+FFFD
// (ErrInvalidText): text that is not UTF-8 becomes U+FFFD on the way,
// and a digest that two different strings share would let a gate pass
// the one it never saw.
func Digest[K Kind, T Input[K]](kind K, v T) (Sum[K], error) {
	if !known(kind) {
		return Sum[K]{}, fmt.Errorf("%w: %q", ErrKind, clip(string(kind)))
	}
	data, err := canonicalJSON(v)
	if err != nil {
		return Sum[K]{}, fmt.Errorf("canon: digest %s: %w", string(kind), err)
	}
	preimage := append([]byte("romeu/"+string(kind)+"/v1\x00"), data...)
	return Sum[K](sha256.Sum256(preimage)), nil
}

// canonicalJSON returns the canonical JSON of v, as Digest describes
// it. The writer turns text that is not UTF-8, and a lone surrogate of a
// MarshalJSON method, into U+FFFD and lets a duplicate key through, so
// that checkText refuses the first as ErrInvalidText before Canonicalize
// refuses the second.
func canonicalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v, json.Deterministic(true),
		json.FormatNilSliceAsNull(true), json.FormatNilMapAsNull(true),
		jsontext.AllowInvalidUTF8(true), jsontext.AllowDuplicateNames(true))
	if err != nil {
		return nil, err
	}
	if err := checkText(data); err != nil {
		return nil, err
	}
	out := jsontext.Value(data)
	if err := out.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		return nil, err
	}
	return out, nil
}

// checkText refuses a string or key of the JSON data that holds U+FFFD
// (ErrInvalidText).
func checkText(data []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(data), jsontext.AllowDuplicateNames(true))
	for {
		tok, err := dec.ReadToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if tok.Kind() == '"' {
			if s := tok.String(); strings.ContainsRune(s, utf8.RuneError) {
				return fmt.Errorf("%w: %q", ErrInvalidText, clip(s))
			}
		}
	}
}

// clipLen is the most bytes of an input that an error message quotes.
const clipLen = 64

// clip returns s cut to clipLen bytes, so that an error about a long
// input stays short. It may cut a UTF-8 sequence; %q escapes the rest.
func clip(s string) string {
	if len(s) <= clipLen {
		return s
	}
	return s[:clipLen] + "..."
}
