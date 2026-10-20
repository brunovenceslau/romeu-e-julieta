// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package canon holds every digest the gates compare and the one ULID
// writer (03 3.12).
//
// A structured value is digested with Digest: sha256 over
// "romeu/<kind>/v1", a NUL byte and the canonical JSON of the value, the
// RFC 8785 form of encoding/json/jsontext. The kind is in the digest's
// type, Sum, so digests of two kinds never compare. Raw file bytes are
// not digested here: they use plain sha256, which sha256sum and shasum
// reproduce.
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
	"math"
	"strconv"
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

// ErrNumberRange is the error of a number that a MarshalJSON method
// writes beyond the float64 range, or so small that a float64 flushes
// it to zero: RFC 8785 would write the largest float64 or 0 for it, so
// two different numbers would share one digest.
var ErrNumberRange = errors.New("canon: a number out of the float64 range")

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
// The canonical JSON is CanonicalJSON's. v is marshaled by
// encoding/json/v2, under its rules: omitempty omits a member whose JSON
// is null, "", {} or [], and a time.Duration, which v2 has no form for,
// is refused. Digest fails on a value CanonicalJSON refuses and on a
// kind value other than its constant (ErrKind).
func Digest[K Kind, T Input[K]](kind K, v T) (Sum[K], error) {
	if !known(kind) {
		return Sum[K]{}, fmt.Errorf("%w: %q", ErrKind, clip(string(kind)))
	}
	data, err := CanonicalJSON(v)
	if err != nil {
		return Sum[K]{}, fmt.Errorf("canon: digest %s: %w", string(kind), err)
	}
	preimage := append([]byte("romeu/"+string(kind)+"/v1\x00"), data...)
	return Sum[K](sha256.Sum256(preimage)), nil
}

// CanonicalJSON returns the canonical JSON of v (03 3.12): what
// encoding/json/v2 writes for v, nil lists and maps as null, in the form
// of RFC 8785, the JSON Canonicalization Scheme: keys sorted by their
// UTF-16 code units, minimal string escapes with U+2028 and U+2029 as
// they are, no insignificant whitespace, and a number with a fraction or
// an exponent in its float64 ECMAScript form, so 1.0 and 1e0 are 1 and
// -0 is 0; an integer keeps its digits, which RFC 8785 would round
// beyond 2^53.
//
// It refuses a value encoding/json/v2 cannot write (NaN, an infinity, a
// channel, a time.Duration), an object with a duplicate key, which only
// a MarshalJSON method can write, a number a MarshalJSON method writes
// that is out of the float64 range or that float64 flushes to zero
// (ErrNumberRange), and a string or key that holds U+FFFD
// (ErrInvalidText): text that is not UTF-8 becomes U+FFFD on the way.
// Each of these would let two different values share one digest, and a
// gate pass the one it never saw.
func CanonicalJSON(v any) ([]byte, error) {
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
// (ErrInvalidText), and a number Canonicalize would clamp to the largest
// float64 or flush to zero (ErrNumberRange). It reads the writer's
// output before Canonicalize, so each number is checked in the text the
// writer gave it, and it lets a duplicate key through, so that the
// invalid text of two keys that become one is ErrInvalidText.
func checkText(data []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(data), jsontext.AllowDuplicateNames(true))
	for {
		if dec.PeekKind() == '0' {
			num, err := dec.ReadValue()
			if err != nil {
				return err
			}
			if err := checkNumber(string(num)); err != nil {
				return err
			}
			continue
		}
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

// checkNumber refuses the JSON number text that is out of the float64
// range, which Canonicalize would write as the largest float64, and the
// one whose mantissa holds a digit other than zero but that float64
// flushes to zero, which Canonicalize would write as 0 (ErrNumberRange).
// A number that rounds to a subnormal float64, or to another float64,
// is kept: that is the one form of RFC 8785.
func checkNumber(text string) error {
	f, err := strconv.ParseFloat(text, 64)
	switch {
	case errors.Is(err, strconv.ErrRange) && math.IsInf(f, 0):
		return fmt.Errorf("%w: %s is beyond the float64 range", ErrNumberRange, clip(text))
	case err != nil:
		return fmt.Errorf("canon: the number %s: %w", clip(text), err)
	}
	mantissa, _, _ := strings.Cut(strings.ToLower(text), "e")
	if f == 0 && strings.ContainsAny(mantissa, "123456789") {
		return fmt.Errorf("%w: %s is flushed to zero in a float64", ErrNumberRange, clip(text))
	}
	return nil
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
