// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// TestCheckSum holds the sha256 check in Go (REQ-1328, REQ-1333): the
// line of the archive's name in checksums.txt must hold its sha256.
func TestCheckSum(t *testing.T) {
	t.Parallel()
	archive := []byte("the archive")
	name := "romeu_v1.0.0_darwin_arm64.tar.gz"
	other := sumLine([]byte("another"), "romeu_v1.0.0_darwin_amd64.tar.gz")
	require.NoError(t, checkSum(archive, name, []byte(other+sumLine(archive, name))))
	for _, tt := range []struct{ name, sums string }{
		{"another digest", other + sumLine([]byte("tampered"), name)},
		{"no line for the archive", other},
		{"a line that holds no sha256", "abc  " + name + "\n"},
		{"empty", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requireID(t, checkSum(archive, name, []byte(tt.sums)), VerifyFailed)
		})
	}
}

// TestVerificationArgv pins the attestation entry of the verification
// contract as ADR 0045, decision 1, has it (10 10.2, REQ-0910).
func TestVerificationArgv(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"/gh", "api", "repos/brunovenceslau/romeu-e-julieta/commits/v1.0.0-alpha.2", "--jq", ".sha"},
		tagCommitArgv("/gh", "v1.0.0-alpha.2"))
	assert.Equal(t, []string{"/gh", "attestation", "verify", "/tmp/a.tar.gz",
		"--repo", "brunovenceslau/romeu-e-julieta",
		"--signer-workflow", "brunovenceslau/romeu-e-julieta/.github/workflows/release.yml",
		"--source-digest", "0123",
		"--source-ref", "refs/heads/main",
		"--deny-self-hosted-runners",
	}, attestArgv("/gh", "/tmp/a.tar.gz", "0123"))
}

// TestAttest runs the attestation entry through the gh helper, which
// records its argv and its environment (REQ-1328, REQ-0296): the commit
// gh api prints is the source digest of gh attestation verify, the
// environment is exactly HOME, PATH=/usr/bin:/bin and LANG=C, and the
// archive is gone once attest returns. A gh that exits non-zero, or
// prints no commit id, is verify-failed with its output escaped and cut
// to 4 KiB in the details.
func TestAttest(t *testing.T) {
	t.Parallel()
	gh := ghBinary(t)
	name := "romeu_v1.0.0-alpha.2_darwin_arm64.tar.gz"
	commit := strings.Repeat("ab", 20)

	t.Run("verified", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, "gh-commit"), []byte(commit), 0o600))
		require.NoError(t, attest(t.Context(), gh, home, "v1.0.0-alpha.2", name, []byte("archive")))
		calls := ghCalls(t, home)
		require.Len(t, calls, 2)
		assert.Equal(t, tagCommitArgv(gh, "v1.0.0-alpha.2")[1:], calls[0].Argv)
		archive := calls[1].Argv[2]
		assert.Equal(t, attestArgv(gh, archive, commit)[1:], calls[1].Argv)
		assert.Equal(t, name, filepath.Base(archive))
		assert.NoFileExists(t, archive, "the archive is removed")
		for _, c := range calls {
			assert.Equal(t, []string{"HOME=" + home, "LANG=C", "PATH=/usr/bin:/bin"}, c.Env)
		}
	})
	for _, tt := range []struct {
		name  string
		files map[string]string
		calls int
	}{
		{"gh api fails", map[string]string{"gh-api-exit": "1", "gh-commit": "HTTP 404: Not Found"}, 1},
		{"gh api prints no commit id", map[string]string{"gh-commit": "\x1b[31mnot a commit"}, 1},
		{"the attestation is refused", map[string]string{"gh-commit": commit, "gh-attest-exit": "1",
			"gh-attest-output": "\x1b[31mverification failed\n" + strings.Repeat("y", 5000)}, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			home := t.TempDir()
			for f, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(home, f), []byte(content), 0o600))
			}
			err := attest(t.Context(), gh, home, "v1.0.0-alpha.2", name, []byte("archive"))
			requireID(t, err, VerifyFailed)
			assert.Len(t, ghCalls(t, home), tt.calls)
			var output string
			for _, d := range cli.Of(err).Details {
				if d.Key == "gh output" {
					output = d.Value
				}
			}
			assert.NotEmpty(t, output)
			assert.NotContains(t, output, "\x1b", "gh's output is escaped")
			assert.LessOrEqual(t, len(output), 4096+64, "gh's output is cut to 4 KiB before it is escaped")
		})
	}
}
