// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package claude_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/agent/claude"
)

var update = flag.Bool("update", false, "rewrite the payload line of kits/julieta-claude/julieta-claude.yaml")

// payloadLine is the line of the first install step of the
// julieta-claude kit that writes the kit's files, as a gzip tar in
// base64, before the step unpacks them into /usr/share/julieta-claude.
// The files ride in the step because sbx puts no file of a mixin's
// directory into a sandbox of a v2 agent (probe of 2026-10-27).
var payloadLine = regexp.MustCompile(`(?m)^(\s*printf '%s' ')([A-Za-z0-9+/=]*)(' >/usr/share/julieta-claude/kit\.tar\.gz\.b64)$`)

// TestKitPayloadIsTheKitFiles holds the payload of the julieta-claude
// kit to the files of its directory that the digest covers (KitFiles):
// the same names and bytes, and no other entry. With -update it writes
// the payload again from those files.
func TestKitPayloadIsTheKitFiles(t *testing.T) {
	descriptor := filepath.Join(kitDir, "julieta-claude.yaml")
	src, err := os.ReadFile(descriptor)
	require.NoError(t, err)
	m := payloadLine.FindAllSubmatchIndex(src, -1)
	require.Len(t, m, 1, "the kit writes its payload on one printf line")
	want, err := claude.KitFiles(os.DirFS(kitDir))
	require.NoError(t, err)
	if *update {
		b64 := base64.StdEncoding.EncodeToString(pack(t, want))
		out := slices.Concat(src[:m[0][4]], []byte(b64), src[m[0][5]:])
		require.NoError(t, os.WriteFile(descriptor, out, 0o644))
		src = out
		m = payloadLine.FindAllSubmatchIndex(src, -1)
	}
	raw, err := base64.StdEncoding.DecodeString(string(src[m[0][4]:m[0][5]]))
	require.NoError(t, err)
	assert.Equal(t, want, unpack(t, raw), "the payload of the kit differs from its files: go test ./internal/agent/claude -run TestKitPayloadIsTheKitFiles -update")
}

// pack returns a gzip tar of files, in name order, each a regular file
// of mode 0644 owned by root, with no time.
func pack(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	z, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	require.NoError(t, err)
	w := tar.NewWriter(z)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		require.NoError(t, w.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: n, Mode: 0o644, Size: int64(len(files[n])), Format: tar.FormatUSTAR}))
		_, err := w.Write(files[n])
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, z.Close())
	return buf.Bytes()
}

// unpack returns the regular files of a gzip tar by name; any other
// entry, or a name that is not local, fails the test.
func unpack(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	z, err := gzip.NewReader(bytes.NewReader(raw))
	require.NoError(t, err)
	r := tar.NewReader(z)
	files := map[string][]byte{}
	for {
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		require.Equal(t, byte(tar.TypeReg), h.Typeflag, h.Name)
		require.True(t, filepath.IsLocal(h.Name), h.Name)
		b, err := io.ReadAll(r)
		require.NoError(t, err)
		files[h.Name] = b
	}
	return files
}
