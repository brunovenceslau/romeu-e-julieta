// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
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
	assert.Equal(t, 4096, ghOutputLimit, "REQ-1333: gh's output cut to its first 4 KiB")
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
		// ys is how many y the output keeps of the 5000 gh printed: 4 KiB
		// less the 25 bytes of its first line.
		ys int
	}{
		{"gh api fails", map[string]string{"gh-api-exit": "1", "gh-commit": "HTTP 404: Not Found"}, 1, 0},
		{"gh api prints no commit id", map[string]string{"gh-commit": "\x1b[31mnot a commit"}, 1, 0},
		{"the attestation is refused", map[string]string{"gh-commit": commit, "gh-attest-exit": "1",
			"gh-attest-output": "\x1b[31mverification failed\n" + strings.Repeat("y", 5000)}, 2, 4096 - 25},
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
			if tt.ys > 0 {
				assert.True(t, strings.HasSuffix(output, "\n"+strings.Repeat("y", tt.ys)) || strings.HasSuffix(output, `\n`+strings.Repeat("y", tt.ys)),
					"gh's output is cut at exactly 4 KiB")
				assert.False(t, strings.HasSuffix(output, strings.Repeat("y", tt.ys+1)), "gh's output is cut at exactly 4 KiB")
			}
		})
	}
}

// TestRunGhBothStreams holds the lock on gh's output: a gh that writes
// to its standard output and its standard error at once has the two
// copy goroutines of procgroup write the shared capture together, which
// the race detector (go test -race) reports when the capture takes no
// lock. The two captures are cut to 4 KiB.
func TestRunGhBothStreams(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	script := "#!/bin/sh\n" +
		"i=0; while [ $i -lt 1000 ]; do echo out-$i; i=$((i+1)); done &\n" +
		"i=0; while [ $i -lt 1000 ]; do echo err-$i >&2; i=$((i+1)); done\n" +
		"wait\n"
	require.NoError(t, os.WriteFile(gh, []byte(script), 0o700))

	stdout, both, err := runGh(t.Context(), []string{gh}, dir)

	require.NoError(t, err)
	assert.Len(t, both, ghOutputLimit, "the capture of both streams is cut to 4 KiB")
	assert.Len(t, stdout, ghOutputLimit, "the standard output is cut to 4 KiB")
	assert.True(t, strings.HasPrefix(string(stdout), "out-0\n"), "the standard output starts with its first line")
	assert.NotContains(t, string(stdout), "err-", "the standard error is not in the standard output")
}

// TestCappedConcurrentWrites holds the lock of capped on every run, not
// only when a process happens to write both streams at once: two
// goroutines write the same capture a few hundred times each, which the
// race detector (go test -race) reports without the lock, and the bytes
// kept stay at the cap.
func TestCappedConcurrentWrites(t *testing.T) {
	t.Parallel()
	c := &capped{left: ghOutputLimit}
	chunk := []byte(strings.Repeat("x", 100))
	var wg gosync.WaitGroup
	for range 2 {
		wg.Go(func() {
			for range 300 {
				n, _ := c.Write(chunk)
				assert.Equal(t, len(chunk), n, "every write is reported whole")
			}
		})
	}
	wg.Wait()

	assert.Equal(t, ghOutputLimit, c.b.Len(), "the capture is cut at the cap")
	assert.Zero(t, c.left)
}

// TestAttestNamesTheArchiveByItsAbsolutePath: gh runs in HOME, so the
// archive is named to it by an absolute path even when TMPDIR is
// relative. It changes the directory and the environment of the
// process, so it does not run in parallel.
func TestAttestNamesTheArchiveByItsAbsolutePath(t *testing.T) {
	gh := ghBinary(t)
	home := t.TempDir()
	work, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(work, "tmp"), 0o700))
	t.Chdir(work)
	t.Setenv("TMPDIR", "tmp")

	require.NoError(t, attest(t.Context(), gh, home, "v1.0.0-alpha.2", "romeu_v1.0.0-alpha.2_darwin_arm64.tar.gz", []byte("archive")))

	calls := ghCalls(t, home)
	require.Len(t, calls, 2)
	archive := calls[1].Argv[2]
	assert.True(t, filepath.IsAbs(archive), archive)
	assert.Equal(t, filepath.Join(work, "tmp"), filepath.Dir(filepath.Dir(archive)))
}
