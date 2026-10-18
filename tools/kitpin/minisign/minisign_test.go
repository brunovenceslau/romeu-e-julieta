// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package minisign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/minisign"
)

// The fixtures are under tools/kitpin/testdata (its REUSE.toml says how
// each was made): mise's key and the files of its release 2026.10.3, as
// downloaded; the same checksum file with one byte changed; and
// signatures of mise's file made with a test key.
func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", path))
	require.NoError(t, err)
	return b
}

func key(t *testing.T, path string) minisign.PublicKey {
	t.Helper()
	k, err := minisign.ParsePublicKey(read(t, path))
	require.NoError(t, err)
	return k
}

const miseSums = "mise/2026.10.3/SHASUMS256.txt"

func TestVerifyGoodSignature(t *testing.T) {
	trusted, err := minisign.Verify(key(t, "mise/minisign.pub"), read(t, miseSums), read(t, miseSums+".minisig"))
	require.NoError(t, err)
	assert.Equal(t, "timestamp:1791196029\tfile:SHASUMS256.txt\thashed", trusted)
}

func TestVerifyGoodSignatureOfTheTestKey(t *testing.T) {
	_, err := minisign.Verify(key(t, "test/minisign.pub"), read(t, miseSums), read(t, "test/signed/SHASUMS256.txt.minisig"))
	require.NoError(t, err)
}

func TestKeyID(t *testing.T) {
	assert.Equal(t, "64113EDF160FDEC2", key(t, "mise/minisign.pub").KeyID())
}

// TestVerifyRefuses holds each signature the check refuses, with the
// part of the message that names why.
func TestVerifyRefuses(t *testing.T) {
	cases := []struct {
		name, key, message, sig, want string
	}{
		{"changed_byte", "mise/minisign.pub", "mise/changed/SHASUMS256.txt", "mise/changed/SHASUMS256.txt.minisig", "verification failed"},
		{"another_key", "mise/minisign.pub", miseSums, "test/signed/SHASUMS256.txt.minisig", "not with the key 64113EDF160FDEC2"},
		{"another_key_with_the_key_id", "mise/minisign.pub", miseSums, "test/forged/SHASUMS256.txt.minisig", "verification failed"},
		{"legacy_mode", "test/minisign.pub", miseSums, "test/legacy/SHASUMS256.txt.minisig", "legacy mode"},
		{"changed_trusted_comment", "test/minisign.pub", miseSums, "test/comment/SHASUMS256.txt.minisig", "trusted comment's verification failed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := minisign.Verify(key(t, c.key), read(t, c.message), read(t, c.sig))
			require.ErrorContains(t, err, c.want)
		})
	}
}

// TestVerifyRefusesMalformed holds signature files that are not in
// minisign's form.
func TestVerifyRefusesMalformed(t *testing.T) {
	good := string(read(t, miseSums+".minisig"))
	cases := []struct{ name, sig, want string }{
		{"empty", "", "lines"},
		{"three_lines", "untrusted comment: x\nAAAA\ntrusted comment: y\n", "lines"},
		{"no_untrusted_prefix", "comment: x\n" + good[len("untrusted comment: signature from minisign secret key\n"):], "untrusted comment"},
		{"not_base64", "untrusted comment: x\n!!!\ntrusted comment: y\nAAAA\n", "not base64"},
		{"short_signature", "untrusted comment: x\nRUQ=\ntrusted comment: y\nAAAA\n", "bytes where 74"},
	}
	k := key(t, "mise/minisign.pub")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := minisign.Verify(k, read(t, miseSums), []byte(c.sig))
			require.ErrorContains(t, err, c.want)
		})
	}
}

func TestParsePublicKeyRefuses(t *testing.T) {
	cases := []struct{ name, file, want string }{
		{"empty", "", "lines"},
		{"no_comment", "RWTC3g8W3z4RZK3V3qv7fa1QY4JEWyBtqIHW+85QlJpZc5yG+uNYNBSZ\n", "lines"},
		{"wrong_prefix", "comment: x\nRWTC3g8W3z4RZK3V3qv7fa1QY4JEWyBtqIHW+85QlJpZc5yG+uNYNBSZ\n", "untrusted comment"},
		{"short", "untrusted comment: x\nRWTC3g8W\n", "bytes where 42"},
		{"wrong_algorithm", "untrusted comment: x\nRUTC3g8W3z4RZK3V3qv7fa1QY4JEWyBtqIHW+85QlJpZc5yG+uNYNBSZ\n", "algorithm"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := minisign.ParsePublicKey([]byte(c.file))
			require.ErrorContains(t, err, c.want)
		})
	}
}

// sigLines returns the four lines of mise's signature file.
func sigLines(t *testing.T) []string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(string(read(t, miseSums+".minisig")), "\n"), "\n")
	require.Len(t, lines, 4)
	return lines
}

// TestVerifyCRLF: a signature file with CRLF line ends verifies, as
// minisign's trim drops the carriage return.
func TestVerifyCRLF(t *testing.T) {
	sig := strings.Join(sigLines(t), "\r\n") + "\r\n"
	_, err := minisign.Verify(key(t, "mise/minisign.pub"), read(t, miseSums), []byte(sig))
	require.NoError(t, err)
}

// TestVerifyRefusesAChangedLine holds signature files with one line of
// mise's changed, each refused before or by the signature check.
func TestVerifyRefusesAChangedLine(t *testing.T) {
	cases := []struct {
		name string
		line int
		edit func(string) string
		want string
	}{
		{"control_character_in_the_trusted_comment", 2, func(l string) string { return l + "\x01" }, "control character"},
		{"trusted_comment_without_the_space", 2, func(l string) string { return strings.Replace(l, "trusted comment: ", "trusted comment:", 1) }, "does not start with \"trusted comment: \""},
		{"non_canonical_base64_bits", 3, func(l string) string {
			require.True(t, strings.HasSuffix(l, "Dw=="), "fixture")
			return strings.TrimSuffix(l, "Dw==") + "Dx=="
		}, "not base64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines := sigLines(t)
			lines[c.line] = c.edit(lines[c.line])
			_, err := minisign.Verify(key(t, "mise/minisign.pub"), read(t, miseSums), []byte(strings.Join(lines, "\n")+"\n"))
			require.ErrorContains(t, err, c.want)
		})
	}
}
