// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// LockSetEntry is one repo of the lock-set: its dir and the sha256 of
// its mise.lock, in lowercase hexadecimal.
type LockSetEntry struct {
	Dir    string `json:"dir"`
	Sha256 string `json:"sha256"`
}

// LockSet is the input of the digest of kind lock-set (03 3.12): the
// {dir, mise.lock sha256} of each repo that has a lock, which the skip
// rule of julieta install compares (REQ-0618).
type LockSet []LockSetEntry

// CanonKind declares the kind of a LockSet, lock-set.
func (LockSet) CanonKind() canon.LockSetKind { return canon.LockSet }

// NewLockSet returns the lock-set of the locks given, by dir, sorted by
// dir, so the digest does not depend on the order of the manifest.
func NewLockSet(locks map[string][]byte) LockSet {
	set := make(LockSet, 0, len(locks))
	for dir, data := range locks {
		sum := sha256.Sum256(data)
		set = append(set, LockSetEntry{Dir: dir, Sha256: hex.EncodeToString(sum[:])})
	}
	slices.SortFunc(set, func(a, b LockSetEntry) int { return strings.Compare(a.Dir, b.Dir) })
	return set
}

// Digest returns the lock-set digest of the set as lowercase
// hexadecimal.
func (s LockSet) Digest() (string, error) {
	sum, err := canon.Digest(canon.LockSet, s)
	if err != nil {
		return "", err
	}
	return sum.String(), nil
}
