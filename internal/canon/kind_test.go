// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package canon

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kindNames returns the names of the kinds, in the order of the table.
// fmt prints a kind as its string, since no kind type has a String
// method.
func kindNames() []string {
	names := make([]string, 0, len(kinds))
	for _, k := range kinds {
		names = append(names, fmt.Sprint(k))
	}
	return names
}

// specKindRow is a row of the kinds table of 03 3.12: the kind is the
// code span of its first cell.
var specKindRow = regexp.MustCompile("^\\| `([a-z-]+)` \\|")

// specKinds reads the kinds of the table of 03 3.12, in its order.
func specKinds(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "docs", "spec", "03-formats.md"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	var kinds []string
	inSection := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "## ") {
			inSection = strings.HasPrefix(line, "## 3.12 ")
			continue
		}
		if m := specKindRow.FindStringSubmatch(line); inSection && m != nil {
			kinds = append(kinds, m[1])
		}
	}
	require.NoError(t, s.Err())
	return kinds
}

func TestKindsAreTheTableOfTheSpecification(t *testing.T) {
	t.Parallel()
	want := specKinds(t)
	require.Len(t, want, 12, "the kinds table of 03 3.12")
	assert.Contains(t, want, "memory-content")
	assert.Contains(t, want, "fingerprint")
	assert.Equal(t, want, kindNames())
}

// digestOfEmpty digests the empty object under the kind k.
func digestOfEmpty[K Kind](k K) ([32]byte, error) {
	sum, err := Digest(k, rawOf[K](`{}`))
	return [32]byte(sum), err
}

func TestEveryKindDigests(t *testing.T) {
	t.Parallel()
	digests := map[string]func() ([32]byte, error){
		"widening":        func() ([32]byte, error) { return digestOfEmpty(Widening) },
		"recreate":        func() ([32]byte, error) { return digestOfEmpty(Recreate) },
		"render":          func() ([32]byte, error) { return digestOfEmpty(Render) },
		"egress":          func() ([32]byte, error) { return digestOfEmpty(Egress) },
		"catalog":         func() ([32]byte, error) { return digestOfEmpty(Catalog) },
		"kit-tree":        func() ([32]byte, error) { return digestOfEmpty(KitTree) },
		"candidate-files": func() ([32]byte, error) { return digestOfEmpty(CandidateFiles) },
		"run":             func() ([32]byte, error) { return digestOfEmpty(Run) },
		"lock-set":        func() ([32]byte, error) { return digestOfEmpty(LockSet) },
		"secret-argv":     func() ([32]byte, error) { return digestOfEmpty(SecretArgv) },
		"memory-content":  func() ([32]byte, error) { return digestOfEmpty(MemoryContent) },
		"fingerprint":     func() ([32]byte, error) { return digestOfEmpty(Fingerprint) },
	}
	assert.ElementsMatch(t, kindNames(), slices.Collect(maps.Keys(digests)))
	for name, digest := range digests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := digest()
			require.NoError(t, err)
			assert.Equal(t, sha256.Sum256(preimage(name, `{}`)), got)
		})
	}
}

func TestEveryKindIsItsOwnType(t *testing.T) {
	t.Parallel()
	seen := map[reflect.Type]string{}
	for _, k := range kinds {
		typ := reflect.TypeOf(k)
		if other, ok := seen[typ]; ok {
			t.Errorf("kinds %v and %v share the type %v", other, k, typ)
		}
		seen[typ] = fmt.Sprint(k)
	}
	assert.Len(t, seen, len(kinds))
}
