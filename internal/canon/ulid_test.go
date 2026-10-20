// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package canon

import (
	"bytes"
	"crypto/rand"
	"errors"
	"go/parser"
	"go/token"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oracle encodes the 48-bit millisecond and the 80 random bits as a
// ULID through math/big, apart from the code under test: a 130-bit
// number, written as 26 base-32 digits of the lowercase alphabet.
func oracle(ms uint64, random []byte) string {
	n := new(big.Int).SetUint64(ms)
	n.Lsh(n, 80)
	n.Or(n, new(big.Int).SetBytes(random))
	const alphabet = "0123456789abcdefghjkmnpqrstvwxyz"
	out := make([]byte, 26)
	thirtyTwo := big.NewInt(32)
	digit := new(big.Int)
	for i := len(out) - 1; i >= 0; i-- {
		n.DivMod(n, thirtyTwo, digit)
		out[i] = alphabet[digit.Int64()]
	}
	return string(out)
}

// entropy returns a reader of the given bytes, then of nothing.
func entropy(b ...byte) *bytes.Reader { return bytes.NewReader(b) }

// counting is ten bytes 0x00..0x09, the random part of a test.
var counting = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

func TestNewULID(t *testing.T) {
	t.Parallel()
	maxMS := int64(1)<<48 - 1
	ff := bytes.Repeat([]byte{0xff}, 10)
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		now      time.Time
		random   []byte
		expected string
	}{
		{"the epoch and zero entropy", time.UnixMilli(0), make([]byte, 10), strings.Repeat("0", 26)},
		{"one millisecond", time.UnixMilli(1), make([]byte, 10), "0000000001" + strings.Repeat("0", 16)},
		{"thirty-two milliseconds", time.UnixMilli(32), make([]byte, 10), "0000000010" + strings.Repeat("0", 16)},
		{"the last millisecond and full entropy", time.UnixMilli(maxMS), ff, "7" + strings.Repeat("z", 25)},
		{"a day in 2026", at, counting, oracle(uint64(at.UnixMilli()), counting)},
		{"a time in another zone", at.In(time.FixedZone("x", 3600)), counting, oracle(uint64(at.UnixMilli()), counting)},
		{"sub-millisecond time truncated", at.Add(999 * time.Microsecond), counting, oracle(uint64(at.UnixMilli()), counting)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, err := NewULID(tt.now, NewULIDSource(entropy(tt.random...)))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, id.String())
			assert.Len(t, id.String(), 26)
			assert.Equal(t, strings.ToLower(id.String()), id.String())
		})
	}
}

func TestNewULIDIsMonotonicWithinOneSource(t *testing.T) {
	t.Parallel()
	at := time.UnixMilli(1_700_000_000_000)
	high := bytes.Repeat([]byte{0x80}, 10)
	low := make([]byte, 10)
	src := NewULIDSource(chunks(high, low, low, low))

	first, err := NewULID(at, src)
	require.NoError(t, err)
	assert.Equal(t, oracle(uint64(at.UnixMilli()), high), first.String())

	// The same millisecond: the random part of the last id plus one,
	// though the entropy holds a smaller value.
	second, err := NewULID(at, src)
	require.NoError(t, err)
	next := slices.Clone(high)
	next[9]++
	assert.Equal(t, oracle(uint64(at.UnixMilli()), next), second.String())

	// A clock that went back: the last millisecond is kept.
	third, err := NewULID(at.Add(-time.Hour), src)
	require.NoError(t, err)
	next[9]++
	assert.Equal(t, oracle(uint64(at.UnixMilli()), next), third.String())

	// A later millisecond reads fresh entropy.
	later := at.Add(time.Millisecond)
	fourth, err := NewULID(later, src)
	require.NoError(t, err)
	assert.Equal(t, oracle(uint64(later.UnixMilli()), low), fourth.String())

	ids := []string{first.String(), second.String(), third.String(), fourth.String()}
	assert.True(t, slices.IsSorted(ids))
	assert.Len(t, slices.Compact(slices.Clone(ids)), len(ids))
}

// chunks returns a reader of the byte slices, one after the other.
func chunks(b ...[]byte) *bytes.Reader { return bytes.NewReader(bytes.Join(b, nil)) }

func TestNewULIDCarriesIntoHigherBytes(t *testing.T) {
	t.Parallel()
	at := time.UnixMilli(5)
	random := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff}
	src := NewULIDSource(entropy(random...))
	_, err := NewULID(at, src)
	require.NoError(t, err)
	id, err := NewULID(at, src)
	require.NoError(t, err)
	assert.Equal(t, oracle(5, []byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0}), id.String())
}

func TestNewULIDIsDeterministicForAFixedClockAndEntropy(t *testing.T) {
	t.Parallel()
	at := time.UnixMilli(1_700_000_000_000)
	run := func() []string {
		src := NewULIDSource(entropy(counting...))
		var ids []string
		for range 3 {
			id, err := NewULID(at, src)
			require.NoError(t, err)
			ids = append(ids, id.String())
		}
		return ids
	}
	first, second := run(), run()
	assert.Equal(t, first, second)
}

func TestNewULIDRefuses(t *testing.T) {
	t.Parallel()
	ff := bytes.Repeat([]byte{0xff}, 10)
	tests := []struct {
		name string
		now  time.Time
		src  *ULIDSource
		is   error
	}{
		{"a time before the epoch", time.UnixMilli(-1), NewULIDSource(entropy(counting...)), ErrULIDTime},
		{"a time past 48 bits", time.UnixMilli(1 << 48), NewULIDSource(entropy(counting...)), ErrULIDTime},
		{"the zero time", time.Time{}, NewULIDSource(entropy(counting...)), ErrULIDTime},
		{"a time whose milliseconds wrap to a valid one", time.Unix(1<<62, 0), NewULIDSource(entropy(counting...)), ErrULIDTime},
		{"no source", time.UnixMilli(0), nil, ErrULIDSource},
		{"a source without entropy", time.UnixMilli(0), NewULIDSource(nil), ErrULIDSource},
		{"the zero source", time.UnixMilli(0), &ULIDSource{}, ErrULIDSource},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewULID(tt.now, tt.src)
			require.ErrorIs(t, err, tt.is)
		})
	}

	t.Run("the last millisecond of 48 bits", func(t *testing.T) {
		t.Parallel()
		u, err := NewULID(time.UnixMilli(1<<48-1), NewULIDSource(entropy(counting...)))
		require.NoError(t, err)
		assert.Equal(t, uint64(1<<48-1), u.millis())
		assert.Equal(t, byte('7'), u.String()[0])
	})

	t.Run("short entropy", func(t *testing.T) {
		t.Parallel()
		_, err := NewULID(time.UnixMilli(0), NewULIDSource(entropy(1, 2, 3)))
		require.Error(t, err)
		assert.ErrorContains(t, err, "canon: ulid entropy")
	})

	t.Run("a failed read leaves the source as it was", func(t *testing.T) {
		t.Parallel()
		src := NewULIDSource(&failsOnce{rest: counting})
		_, err := NewULID(time.UnixMilli(9), src)
		require.Error(t, err)
		// The same millisecond reads fresh entropy: nothing was issued.
		id, err := NewULID(time.UnixMilli(9), src)
		require.NoError(t, err)
		assert.Equal(t, oracle(9, counting), id.String())
	})

	t.Run("the ids of one millisecond spent", func(t *testing.T) {
		t.Parallel()
		src := NewULIDSource(chunks(ff, counting))
		_, err := NewULID(time.UnixMilli(7), src)
		require.NoError(t, err)
		_, err = NewULID(time.UnixMilli(7), src)
		require.ErrorIs(t, err, ErrULIDTime)
		// The failure leaves the source as it was: a later millisecond works.
		id, err := NewULID(time.UnixMilli(8), src)
		require.NoError(t, err)
		assert.Equal(t, oracle(8, counting), id.String())
	})
}

// failsOnce is an entropy reader whose first read fails, and whose later
// reads give rest.
type failsOnce struct {
	failed bool
	rest   []byte
}

func (f *failsOnce) Read(p []byte) (int, error) {
	if !f.failed {
		f.failed = true
		return 0, errors.New("entropy: failed once")
	}
	n := copy(p, f.rest)
	f.rest = f.rest[n:]
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func TestNewULIDCarriesPastTheLowerByte(t *testing.T) {
	t.Parallel()
	// 00 ff*9 plus one is 01 00*9: the carry crosses nine bytes.
	random := append([]byte{0}, bytes.Repeat([]byte{0xff}, 9)...)
	src := NewULIDSource(entropy(random...))
	_, err := NewULID(time.UnixMilli(5), src)
	require.NoError(t, err)
	id, err := NewULID(time.UnixMilli(5), src)
	require.NoError(t, err)
	assert.Equal(t, oracle(5, append([]byte{1}, make([]byte, 9)...)), id.String())
}

// admittedImports are the imports outside the standard library that
// canon may hold: the ULID library of ADR 0038 (T145), by name.
var admittedImports = []string{"github.com/oklog/ulid/v2"}

func TestCanonUsesTheStandardLibraryAndTheAdmittedImports(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		checked++
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if slices.Contains(admittedImports, path) {
				continue
			}
			first, _, _ := strings.Cut(path, "/")
			assert.NotContains(t, first, ".", "%s imports %s", file, path)
		}
	}
	require.NotZero(t, checked)
	_, err = os.Stat("digest.go")
	require.NoError(t, err)
}

func TestParseULIDErrorQuotesAShortInput(t *testing.T) {
	t.Parallel()
	_, err := ParseULID(strings.Repeat("a", 1<<20))
	require.ErrorIs(t, err, ErrInvalidULID)
	assert.Less(t, len(err.Error()), 200)
}

func TestNewULIDFromManyGoroutines(t *testing.T) {
	t.Parallel()
	src := NewULIDSource(rand.Reader)
	at := time.UnixMilli(1_700_000_000_000)
	const n = 64
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			id, err := NewULID(at, src)
			ids[i], errs[i] = id.String(), err
		})
	}
	wg.Wait()
	require.NoError(t, errors.Join(errs...))
	slices.Sort(ids)
	assert.Len(t, slices.Compact(slices.Clone(ids)), n)
	// One millisecond, one source: the ids are consecutive.
	for i := 1; i < n; i++ {
		prev, err := ParseULID(ids[i-1])
		require.NoError(t, err)
		next, ok := prev.next()
		require.True(t, ok)
		assert.Equal(t, next.String(), ids[i])
	}
}

func TestParseULID(t *testing.T) {
	t.Parallel()
	valid := oracle(1_700_000_000_000, counting)
	tests := []struct {
		name  string
		input string
		ok    bool
	}{
		{"a lowercase id", valid, true},
		{"the zero id", strings.Repeat("0", 26), true},
		{"the greatest id", "7" + strings.Repeat("z", 25), true},
		{"an uppercase id", strings.ToUpper(valid), false},
		{"one uppercase letter", strings.Repeat("0", 25) + "Z", false},
		{"too short", valid[:25], false},
		{"too long", valid + "0", false},
		{"empty", "", false},
		{"the letter i", "i" + valid[1:], false},
		{"the letter l", valid[:25] + "l", false},
		{"the letter o", valid[:25] + "o", false},
		{"the letter u", valid[:25] + "u", false},
		{"a hyphen", valid[:25] + "-", false},
		{"past 128 bits", "8" + strings.Repeat("0", 25), false},
		{"a multibyte character", "\u00e9" + valid[2:], false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, err := ParseULID(tt.input)
			if !tt.ok {
				require.ErrorIs(t, err, ErrInvalidULID)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.input, id.String())
		})
	}
}

func TestULIDText(t *testing.T) {
	t.Parallel()
	id, err := NewULID(time.UnixMilli(1_700_000_000_000), NewULIDSource(entropy(counting...)))
	require.NoError(t, err)
	text, err := id.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, id.String(), string(text))
	var back ULID
	require.NoError(t, back.UnmarshalText(text))
	assert.Equal(t, id, back)
	err = back.UnmarshalText(bytes.ToUpper(text))
	require.ErrorIs(t, err, ErrInvalidULID)
	assert.Equal(t, id, back, "a refused text leaves the id as it was")
}

func FuzzParseULID(f *testing.F) {
	f.Add(oracle(1_700_000_000_000, counting))
	f.Add(strings.Repeat("0", 26))
	f.Add("7" + strings.Repeat("z", 25))
	f.Add("8" + strings.Repeat("0", 25))
	f.Add(strings.ToUpper(oracle(1, counting)))
	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseULID(s)
		if err != nil {
			if !errors.Is(err, ErrInvalidULID) {
				t.Fatalf("ParseULID(%q): %v is not ErrInvalidULID", s, err)
			}
			return
		}
		if id.String() != s {
			t.Fatalf("ParseULID(%q).String() = %q", s, id.String())
		}
	})
}

// TestULIDTextMatchesTheLibrary holds the text and the parse of canon
// equal to the library's for a fixed set of ids, so the wrapper cannot
// drift from it: String is the library's text lowercased, and ParseULID
// reads every lowercase text as the library does.
func TestULIDTextMatchesTheLibrary(t *testing.T) {
	t.Parallel()
	src := NewULIDSource(rand.Reader)
	ids := []ULID{{}, ULID(bytes.Repeat([]byte{0xff}, 16))}
	for i := range 64 {
		id, err := NewULID(time.UnixMilli(1_700_000_000_000+int64(i)*7919), src)
		require.NoError(t, err)
		ids = append(ids, id)
	}
	for _, id := range ids {
		lib := ulid.ULID(id)
		require.Equal(t, strings.ToLower(lib.String()), id.String())
		parsed, err := ParseULID(id.String())
		require.NoError(t, err)
		want, err := ulid.ParseStrict(id.String())
		require.NoError(t, err)
		require.Equal(t, ULID(want), parsed)
	}
}
