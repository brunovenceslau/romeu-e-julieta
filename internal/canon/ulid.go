// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package canon

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// ErrInvalidULID is the error of a text that is not a ULID in lowercase
// Crockford base32: 26 characters of the alphabet crockford, the first
// at most 7.
var ErrInvalidULID = errors.New("canon: invalid ulid")

// ErrULIDTime is the error of an instant no ULID can hold: before the
// Unix epoch, past 48 bits of milliseconds, or a millisecond whose ids
// one source has all written.
var ErrULIDTime = errors.New("canon: no ulid for this time")

// ErrULIDSource is the error of a ULID asked of no source, or of a
// source without entropy.
var ErrULIDSource = errors.New("canon: ulid source without entropy")

// crockford is the lowercase Crockford base32 alphabet. Two names that
// differ only in case collide on APFS, so every ULID on the host disk is
// lowercase (03 3.12).
const crockford = "0123456789abcdefghjkmnpqrstvwxyz"

// The layout of a ULID: 48 bits of Unix milliseconds, then 80 random
// bits, as 26 characters of 5 bits, the first of which holds 3.
const (
	ulidLen    = 26
	timeLen    = 6 // bytes
	maxMillis  = 1<<48 - 1
	maxFirstCh = 7 // the value of the first character
)

// decoding maps a byte of the alphabet to its value, and every other
// byte, uppercase letters among them, to invalidCh.
var decoding = func() [256]byte {
	var d [256]byte
	for i := range d {
		d[i] = invalidCh
	}
	for i := range len(crockford) {
		d[crockford[i]] = byte(i)
	}
	return d
}()

// invalidCh marks a byte outside the alphabet in decoding.
const invalidCh = 0xff

// ULID is a 128-bit id that sorts by the time it was written. Its text
// is 26 characters of lowercase Crockford base32.
type ULID [16]byte

// String returns the id as 26 characters of lowercase Crockford base32.
func (u ULID) String() string {
	hi := binary.BigEndian.Uint64(u[:8])
	lo := binary.BigEndian.Uint64(u[8:])
	var out [ulidLen]byte
	for i := ulidLen - 1; i >= 0; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | hi<<59
		hi >>= 5
	}
	return string(out[:])
}

// MarshalText returns the id as String writes it.
func (u ULID) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

// UnmarshalText reads an id as ParseULID does, and leaves u as it was on
// an error.
func (u *ULID) UnmarshalText(text []byte) error {
	id, err := ParseULID(string(text))
	if err != nil {
		return err
	}
	*u = id
	return nil
}

// ParseULID reads a ULID written in lowercase Crockford base32. It
// refuses uppercase, which a decoder of the standard alphabet accepts,
// so that one id has one name on a case-insensitive disk (03 3.12).
func ParseULID(s string) (ULID, error) {
	if len(s) != ulidLen {
		return ULID{}, fmt.Errorf("%w: %q has %d bytes, not %d", ErrInvalidULID, clip(s), len(s), ulidLen)
	}
	var hi, lo uint64
	for i := range ulidLen {
		v := decoding[s[i]]
		if v == invalidCh {
			return ULID{}, fmt.Errorf("%w: %q has %q at %d", ErrInvalidULID, clip(s), s[i], i)
		}
		hi = hi<<5 | lo>>59
		lo = lo<<5 | uint64(v)
	}
	if decoding[s[0]] > maxFirstCh {
		return ULID{}, fmt.Errorf("%w: %q is past 128 bits", ErrInvalidULID, clip(s))
	}
	var u ULID
	binary.BigEndian.PutUint64(u[:8], hi)
	binary.BigEndian.PutUint64(u[8:], lo)
	return u, nil
}

// ULIDSource is the injected source of the random bits of ULIDs, and the
// memory that keeps the ids it writes in order. A process holds one, so
// its ULIDs are monotonic within the process; a test holds its own, over
// entropy it fixes, so its ids do not depend on what other tests ran.
// It is safe for use by several goroutines.
type ULIDSource struct {
	mu       sync.Mutex
	entropy  io.Reader
	last     ULID // the last id written, when isIssued
	isIssued bool
}

// NewULIDSource returns a source that reads the random bits of a new
// millisecond from entropy, crypto/rand.Reader outside tests.
func NewULIDSource(entropy io.Reader) *ULIDSource {
	return &ULIDSource{entropy: entropy}
}

// NewULID writes the ULID of the instant now from src. It is the one
// writer of every ULID the product writes. The ids of one source are
// strictly increasing: in a millisecond already used, or one before it
// (a clock that went back), the id is the last one plus one, at the
// last one's millisecond. A new millisecond reads 80 fresh bits.
func NewULID(now time.Time, src *ULIDSource) (ULID, error) {
	if src == nil || src.entropy == nil {
		return ULID{}, ErrULIDSource
	}
	// The range is checked on the time, since UnixMilli wraps for a time
	// far from the epoch and would turn it into a valid millisecond.
	if now.Before(time.UnixMilli(0)) || now.After(time.UnixMilli(maxMillis)) {
		return ULID{}, fmt.Errorf("%w: %s is outside 48 bits of milliseconds", ErrULIDTime, now)
	}
	ms := now.UnixMilli()
	src.mu.Lock()
	defer src.mu.Unlock()
	if src.isIssued && uint64(ms) <= src.last.millis() {
		next, ok := src.last.next()
		if !ok {
			return ULID{}, fmt.Errorf("%w: the ids of millisecond %d are spent", ErrULIDTime, src.last.millis())
		}
		src.last = next
		return next, nil
	}
	var u ULID
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(ms))
	copy(u[:timeLen], ts[8-timeLen:])
	if _, err := io.ReadFull(src.entropy, u[timeLen:]); err != nil {
		return ULID{}, fmt.Errorf("canon: ulid entropy: %w", err)
	}
	src.last, src.isIssued = u, true
	return u, nil
}

// millis returns the millisecond of the id.
func (u ULID) millis() uint64 {
	var ts [8]byte
	copy(ts[8-timeLen:], u[:timeLen])
	return binary.BigEndian.Uint64(ts[:])
}

// next returns the id with the same millisecond and the random part
// plus one, and false when the random part is already the greatest.
func (u ULID) next() (ULID, bool) {
	for i := len(u) - 1; i >= timeLen; i-- {
		u[i]++
		if u[i] != 0 {
			return u, true
		}
	}
	return ULID{}, false
}
