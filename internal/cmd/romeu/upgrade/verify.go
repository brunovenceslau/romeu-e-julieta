// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// checkSum checks the sha256 of archive, named name, against the line
// of checksums.txt that names it, in the format of sha256sum (the hex
// digest, two spaces and the name). No line for the name, or another
// digest, is verify-failed (REQ-1333).
func checkSum(archive []byte, name string, checksums []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(checksums))
	for sc.Scan() {
		digest, file, ok := strings.Cut(sc.Text(), "  ")
		if !ok || file != name {
			continue
		}
		want, err := hex.DecodeString(digest)
		if err != nil || len(want) != sha256.Size {
			return VerifyFailed.New("the line of " + name + " in " + ChecksumsName + " holds no sha256")
		}
		got := sha256.Sum256(archive)
		if subtle.ConstantTimeCompare(got[:], want) != 1 {
			return VerifyFailed.New("the sha256 of "+name+" differs from "+ChecksumsName,
				cli.Detail{Key: "want", Value: digest}, cli.Detail{Key: "got", Value: hex.EncodeToString(got[:])})
		}
		return nil
	}
	return VerifyFailed.New(ChecksumsName + " has no line for " + name)
}

// The argv of the attestation entry of the verification contract (10
// 10.2, REQ-0910), the two gh commands romeu upgrade runs besides gh
// --version (REQ-1320). They follow ADR 0045, decision 1, as merged:
// the signer workflow release.yml, the commit the tag names as the
// source digest, and the ref the workflow started from, refs/heads/main.
// A change of the contract is a change of these two functions alone.

// tagCommitArgv reads the commit the tag names from the repository on
// GitHub.
func tagCommitArgv(gh, tag string) []string {
	return []string{gh, "api", "repos/" + Repo + "/commits/" + tag, "--jq", ".sha"}
}

// attestArgv verifies the attestation of the archive at path against
// the repository, the signer workflow, the commit, the ref and hosted
// runners.
func attestArgv(gh, path, commit string) []string {
	return []string{gh, "attestation", "verify", path,
		"--repo", Repo,
		"--signer-workflow", Repo + "/.github/workflows/release.yml",
		"--source-digest", commit,
		"--source-ref", "refs/heads/main",
		"--deny-self-hosted-runners",
	}
}

// commitForm is the output of the first command: a full commit id.
var commitForm = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// ghOutputLimit is how much of gh's output the details of
// verify-failed hold (REQ-1333).
const ghOutputLimit = 4096

// attest runs the attestation entry through gh, the tools.gh judged at
// the floor, with HOME home, on the archive written to a directory of
// its own under the temporary directory, named to gh by its absolute
// path, which it removes after. A
// command that exits non-zero, or a commit that is not a full commit
// id, is verify-failed with gh's output, escaped and cut, in the
// details.
func attest(ctx context.Context, gh, home, tag, name string, archive []byte) error {
	dir, err := os.MkdirTemp("", "romeu-upgrade-")
	if err != nil {
		return cli.Internal.Wrap(err, "upgrade: a temporary directory for the archive")
	}
	defer removeArchiveDir(dir)
	// gh runs in home, so a relative TMPDIR would point it elsewhere.
	path, err := filepath.Abs(filepath.Join(dir, name))
	if err != nil {
		return cli.Internal.Wrap(err, "upgrade: the path of the archive")
	}
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		return cli.Internal.Wrap(err, "upgrade: write the archive")
	}
	stdout, both, err := runGh(ctx, tagCommitArgv(gh, tag), home)
	if err != nil {
		return ghFailed(err, "gh api did not read the commit of "+tag, both)
	}
	commit := strings.TrimSpace(string(stdout))
	if !commitForm.MatchString(commit) {
		return ghFailed(nil, "gh api printed no commit id for "+tag, both)
	}
	if _, both, err := runGh(ctx, attestArgv(gh, path, commit), home); err != nil {
		return ghFailed(err, "gh attestation verify refused "+name, both)
	}
	return nil
}

// ghFailed returns verify-failed for a gh command that failed with err,
// or printed what the caller refused, with its output in the details;
// a timeout and the end of the command's context stay what they are.
func ghFailed(err error, message string, out []byte) error {
	if _, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return cli.SubprocessTimeout.Wrap(err, message)
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	text := string(out)
	if len(text) > ghOutputLimit {
		text = text[:ghOutputLimit]
	}
	e := VerifyFailed.New(message, cli.Detail{Key: "gh output", Value: termsafe.Escape(text)})
	if err != nil {
		e.Details = append(e.Details, cli.Detail{Key: "status", Value: err.Error()})
	}
	return e
}

// scrubbedPath is the PATH of the scrubbed environment (REQ-0296).
const scrubbedPath = "PATH=/usr/bin:/bin"

// runGh runs one command of the verification contract through gh with
// the scrubbed environment of REQ-0296, under the class Gh, and returns
// its standard output, and its standard output and standard error
// together in the order it wrote them, each cut to ghOutputLimit. The
// call site is admitted by the table of I16 (REQ-1320).
func runGh(ctx context.Context, argv []string, home string) (stdout, both []byte, err error) {
	out := &capped{left: ghOutputLimit}
	all := &capped{left: ghOutputLimit}
	err = procgroup.Run(ctx, procgroup.Command{
		Class:  procgroup.Gh,
		Argv:   argv,
		Dir:    home,
		Env:    []string{"HOME=" + home, scrubbedPath, "LANG=C"},
		Stdout: teeTo{out, all},
		Stderr: all,
	})
	return out.b.Bytes(), all.b.Bytes(), err
}

// capped keeps the first left bytes it is given and drops the rest,
// reporting every write as whole, so a command never blocks on it. Its
// writes take a lock: procgroup copies a command's standard output and
// standard error in two goroutines, and runGh gives both the same
// capped.
type capped struct {
	mu   sync.Mutex
	b    bytes.Buffer
	left int
}

// Write keeps what fits.
func (c *capped) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n := min(len(p), c.left); n > 0 {
		c.b.Write(p[:n])
		c.left -= n
	}
	return len(p), nil
}

// teeTo writes to both of its writers, which never fail.
type teeTo [2]*capped

// Write writes p to both.
func (t teeTo) Write(p []byte) (int, error) {
	_, _ = t[0].Write(p)
	return t[1].Write(p)
}

// removeArchiveDir removes the directory attest wrote the archive into:
// a temporary directory this command made (I16).
func removeArchiveDir(dir string) { _ = os.RemoveAll(dir) }
