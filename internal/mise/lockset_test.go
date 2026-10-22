// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
)

// TestLockSetDigest: the digest holds the dir and the sha256 of each
// lock, and nothing of the order they were given in (03 3.12).
func TestLockSetDigest(t *testing.T) {
	digest := func(locks map[string][]byte) string {
		d, err := mise.NewLockSet(locks).Digest()
		require.NoError(t, err)
		return d
	}
	a := digest(map[string][]byte{"api": []byte("x"), "web": []byte("y")})
	assert.Regexp(t, `^[0-9a-f]{64}$`, a)
	assert.Equal(t, a, digest(map[string][]byte{"web": []byte("y"), "api": []byte("x")}))
	assert.NotEqual(t, a, digest(map[string][]byte{"api": []byte("x2"), "web": []byte("y")}), "a lock changed")
	assert.NotEqual(t, a, digest(map[string][]byte{"api": []byte("x")}), "a repo left")
	assert.NotEqual(t, a, digest(map[string][]byte{"api": []byte("y"), "web": []byte("x")}), "the locks swapped dirs")
	assert.Equal(t, "web", mise.NewLockSet(map[string][]byte{"web": nil, "api": nil})[1].Dir)
}
