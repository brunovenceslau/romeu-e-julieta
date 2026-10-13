// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package canon holds every digest the gates compare and the one ULID
// writer (03 3.12).
//
// A structured value is digested with Digest: sha256 over
// "romeu/<kind>/v1", a NUL byte and the canonical JSON of the value. The
// kind is in the digest's type, Sum, so digests of two kinds never
// compare. Raw file bytes are not digested here: they use plain sha256,
// which sha256sum and shasum reproduce.
//
// Every ULID the product writes comes from NewULID, in lowercase
// Crockford base32, and ParseULID refuses any other spelling.
package canon

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
)

// ErrKind is the error of a kind value that is not the one constant of
// its kind type, such as MemoryContentKind("fingerprint").
var ErrKind = errors.New("canon: not a kind of the table")

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
// The canonical JSON is what encoding/json writes for v, with the keys
// of every object sorted by their UTF-8 bytes, no insignificant
// whitespace, and no escape of <, > and &. Digest fails on a value
// encoding/json cannot write, on an object with a duplicate key, which
// only a MarshalJSON method can write, and on a kind value other than
// its constant (ErrKind). A string with invalid UTF-8 is written by
// encoding/json with each invalid byte as U+FFFD, so two strings that
// differ only in invalid bytes have one digest; a MarshalJSON method
// that writes a number writes its text, so 1 and 1.0 have two.
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
// it. encoding/json writes v and checks the output of any MarshalJSON
// method; the copy that follows sorts the keys. Numbers are copied as
// encoding/json wrote them, so an int64 above 2^53 keeps every digit.
func canonicalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out bytes.Buffer
	if err := copyValue(&out, dec); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// member is one key of an object and its value, already canonical.
type member struct {
	key   string
	value []byte
}

// copyValue writes the next value of dec to out in canonical form.
func copyValue(out *bytes.Buffer, dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch tok := tok.(type) {
	case json.Delim:
		if tok == '{' {
			return copyObject(out, dec)
		}
		return copyArray(out, dec)
	case string:
		return writeString(out, tok)
	case json.Number:
		out.WriteString(tok.String())
	case bool:
		out.WriteString(strconv.FormatBool(tok))
	default: // nil, the only other token of a value
		out.WriteString("null")
	}
	return nil
}

// copyObject writes the members of the object whose '{' dec has just
// read, sorted by key, and reads its '}'.
func copyObject(out *bytes.Buffer, dec *json.Decoder) error {
	var members []member
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := tok.(string) // the decoder reads an object key as a string, or fails
		var value bytes.Buffer
		if err := copyValue(&value, dec); err != nil {
			return err
		}
		members = append(members, member{key: key, value: value.Bytes()})
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	slices.SortStableFunc(members, func(a, b member) int { return cmp.Compare(a.key, b.key) })
	out.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			if m.key == members[i-1].key {
				return fmt.Errorf("duplicate key %q", m.key)
			}
			out.WriteByte(',')
		}
		if err := writeString(out, m.key); err != nil {
			return err
		}
		out.WriteByte(':')
		out.Write(m.value)
	}
	out.WriteByte('}')
	return nil
}

// copyArray writes the elements of the array whose '[' dec has just
// read, in their order, and reads its ']'.
func copyArray(out *bytes.Buffer, dec *json.Decoder) error {
	out.WriteByte('[')
	for i := 0; dec.More(); i++ {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := copyValue(out, dec); err != nil {
			return err
		}
	}
	out.WriteByte(']')
	_, err := dec.Token()
	return err
}

// writeString writes s as a JSON string, escaping what JSON requires
// and leaving <, > and & as they are.
func writeString(out *bytes.Buffer, s string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	out.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
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
