// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveIsFixed(t *testing.T) {
	files := []File{{"b", 0o755, []byte("one")}, {"a", 0o644, []byte("two")}}
	first, err := Archive(files, commitTime)
	require.NoError(t, err)
	second, err := Archive(files, commitTime)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	headers, content := members(t, first)
	require.Len(t, headers, 2)
	assert.Equal(t, "b", headers[0].Name, "the order given, not a sorted one")
	assert.Equal(t, "a", headers[1].Name)
	assert.Equal(t, "two", string(content["a"]))
	assert.True(t, headers[0].ModTime.Equal(commitTime))
}

func TestChecksumsFormat(t *testing.T) {
	got := Checksums(map[string][]byte{"b.tar.gz": []byte("b"), "a.tar.gz": []byte("a")})
	want := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb  a.tar.gz\n" +
		"3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d  b.tar.gz\n"
	assert.Equal(t, want, string(got))
}
