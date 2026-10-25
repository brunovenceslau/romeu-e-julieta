// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/manifest"
)

// handWritten is a manifest as the bridge's runbook writes it: indented,
// keys in any order and no runDigest.
const handWritten = `{
  "schema": "manifest/v1",
  "protocol": 1,
  "project": "romeu-e-julieta",
  "agent": "claude",
  "generation": "01j9zc2q8x7w4t6y3r5e1v9k0p",
  "specCommit": "0123456789abcdef0123456789abcdef01234567",
  "egress": [],
  "repos": [{
    "dir": "romeu-e-julieta", "primary": true, "ref": "main",
    "url": "https://github.com/brunovenceslau/romeu-e-julieta",
    "path": "/w/romeu-e-julieta", "memory": "/m/romeu-e-julieta",
    "base": ["89abcdef0123456789abcdef0123456789abcdef"]
  }],
  "run": {},
  "salvage": {"capBytes": 0, "excludeIgnored": []}
}`

func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := start(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "m.json")
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func TestEncodeRoundTrips(t *testing.T) {
	code, out, errs := run(t, "", "encode", write(t, handWritten))
	require.Equal(t, exitOK, code, errs)
	m, err := manifest.DecodeEnv(strings.TrimSuffix(out, "\n"))
	require.NoError(t, err)
	assert.Equal(t, "romeu-e-julieta", m.Project)
	sum, err := canon.Digest(canon.Run, m.Run)
	require.NoError(t, err)
	assert.Equal(t, sum.String(), m.RunDigest)
}

func TestEncodeReadsStdin(t *testing.T) {
	code, out, errs := run(t, handWritten, "encode", "-")
	require.Equal(t, exitOK, code, errs)
	assert.NotEmpty(t, strings.TrimSpace(out))
}

func TestEncodeRefuses(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"an unknown member", strings.Replace(handWritten, `"agent"`, `"agnet": "x", "agent"`, 1), "agnet"},
		{"a wrong runDigest", strings.Replace(handWritten, `"run": {}`, `"run": {}, "runDigest": "00"`, 1), "runDigest"},
		{"two primaries", strings.Replace(handWritten, `"repos": [{`, `"repos": [{"dir": "x", "primary": true, "ref": "main", "url": "u", "path": "p", "memory": "m", "base": []}, {`, 1), "primary"},
		{"a bad generation", strings.Replace(handWritten, "01j9zc2q8x7w4t6y3r5e1v9k0p", "nope", 1), "generation"},
		{"not JSON", "{", "manifest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := run(t, "", "encode", write(t, tc.body))
			assert.Equal(t, exitError, code)
			assert.Empty(t, out)
			assert.Contains(t, errs, tc.want)
		})
	}
}

func TestULIDPrintsANewGeneration(t *testing.T) {
	code, out, errs := run(t, "", "ulid")
	require.Equal(t, exitOK, code, errs)
	_, err := canon.ParseULID(strings.TrimSuffix(out, "\n"))
	assert.NoError(t, err)
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"encode"}, {"encode", "a", "b"}, {"ulid", "x"}, {"other"}} {
		code, _, errs := run(t, "", args...)
		assert.Equal(t, exitUsage, code, args)
		assert.Contains(t, errs, "usage")
	}
}
