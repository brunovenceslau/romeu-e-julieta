// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package canon

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// ErrInvalidULID is the error of a text that is not a ULID in lowercase
// Crockford base32: 26 characters of that alphabet, the first at most 7.
var ErrInvalidULID = errors.New("canon: invalid ulid")

// ErrULIDTime is the error of an instant no ULID can hold: before the
// Unix epoch, past 48 bits of milliseconds, or a millisecond whose ids
// one source has all written.
var ErrULIDTime = errors.New("canon: no ulid for this time")

// ErrULIDSource is the error of a ULID asked of no source, or of a
// source without entropy.
var ErrULIDSource = errors.New("canon: ulid source without entropy")

// The layout of a ULID: 48 bits of Unix milliseconds, then 80 random
// bits, as 26 characters of Crockford base32. github.com/oklog/ulid/v2
// encodes, decodes and stamps them (ADR 0038); canon keeps the text
// lowercase, because two names that differ only in case collide on
// APFS, so every ULID on the host disk is lowercase (03 3.12).
const (
	ulidLen   = 26
	timeLen   = 6 // bytes
	maxMillis = 1<<48 - 1
)

// ULID is a 128-bit id that sorts by the time it was written. Its text
// is 26 characters of lowercase Crockford base32.
type ULID [16]byte

// String returns the id as 26 characters of lowercase Crockford base32.
func (u ULID) String() string { return strings.ToLower(ulid.ULID(u).String()) }

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
	if i := strings.IndexFunc(s, func(r rune) bool { return 'A' <= r && r <= 'Z' }); i >= 0 {
		return ULID{}, fmt.Errorf("%w: %q has the uppercase %q at %d", ErrInvalidULID, s, s[i], i)
	}
	id, err := ulid.ParseStrict(s)
	if err != nil {
		return ULID{}, fmt.Errorf("%w: %q: %w", ErrInvalidULID, s, err)
	}
	return ULID(id), nil
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
	id, err := ulid.New(uint64(ms), io.LimitReader(src.entropy, 16-timeLen))
	if err != nil {
		return ULID{}, fmt.Errorf("canon: ulid entropy: %w", err)
	}
	src.last, src.isIssued = ULID(id), true
	return ULID(id), nil
}

// millis returns the millisecond of the id.
func (u ULID) millis() uint64 { return ulid.ULID(u).Time() }

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
