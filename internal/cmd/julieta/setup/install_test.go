// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockText is a mise.lock of the layout of the pinned mise: gh locked
// on both linux platforms and a version-only go: tool.
const lockText = `lockfile_version = 3

[[tools.gh]]
version = "2.102.0"
backend = "aqua:cli/cli"
specifiers = ["2.102.0"]

[tools.gh."platforms.linux-arm64"]
checksum = "sha256:7862c86c72f43df3a2d93ddde6f473285b4e2af61b494849846827e513ef6484"
url = "https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_linux_arm64.tar.gz"

[tools.gh."platforms.linux-x64"]
checksum = "sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386"
url = "https://github.com/cli/cli/releases/download/v2.102.0/gh_2.102.0_linux_amd64.tar.gz"

[[tools."go:golang.org/x/tools/gopls"]]
version = "0.23.0"
backend = "go:golang.org/x/tools/gopls"
specifiers = ["0.23.0"]
`

// miseFiles are the mise.toml and the mise.lock the install tests put
// in the primary repo.
func miseFiles(t *testing.T, dir string) {
	t.Helper()
	toml := "[tools]\ngh = \"2.102.0\"\n\"go:golang.org/x/tools/gopls\" = \"0.23.0\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.toml"), []byte(toml), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mise.lock"), []byte(lockText), 0o600))
}

// fakeMise puts a mise on PATH that runs script, and returns the file
// the script appends to.
func fakeMise(t *testing.T, script string) (calls string) {
	t.Helper()
	bin := t.TempDir()
	calls = filepath.Join(t.TempDir(), "calls")
	body := "#!/bin/sh\necho \"$@\" >> '" + calls + "'\n" + script + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "mise"), []byte(body), 0o700))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return calls
}

// count returns the number of calls of "mise install" the file holds,
// 0 when it is absent: the other calls of setup (trust, tasks, run) are
// not what these cases count.
func count(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	n := 0
	for line := range strings.Lines(string(b)) {
		if strings.TrimSpace(line) == "install" {
			n++
		}
	}
	return n
}

// TestSetupInstalls: setup runs mise install in a repo that has a
// mise.toml, skips it when the lock-set digest equals the last success
// and runs it again when a lock changes (04 4.3, REQ-0400, REQ-0618).
func TestSetupInstalls(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	calls := fakeMise(t, ":")

	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Errors)
	assert.Equal(t, 1, count(t, calls))
	assert.FileExists(t, filepath.Join(s.home, ".local", "state", "julieta", "install.json"))

	code, _, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, 1, count(t, calls), "the lock-set is unchanged: no mise")

	lock, err := os.ReadFile(filepath.Join(s.primary(), "mise.lock"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(s.primary(), "mise.lock"), append(lock, "\n# changed\n"...), 0o600))
	code, _, stderr = setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, 2, count(t, calls))
}

// TestSetupInstallNothing: repos with no mise.toml start no mise and
// record nothing, so the no-op of setup costs nothing here (S8).
func TestSetupInstallNothing(t *testing.T) {
	s := newSandbox(t)
	calls := fakeMise(t, ":")
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	assert.Empty(t, rep.Errors)
	assert.Equal(t, 0, count(t, calls))
	assert.NoFileExists(t, filepath.Join(s.home, ".local", "state", "julieta", "install.json"))
}

// TestSetupInstallFailureLeavesSandboxUsable: a failed install is
// reported and setup exits 0, since only the failures the exit rule of
// 04 4.3 names leave the sandbox unusable; nothing is recorded, so the
// next setup installs again.
func TestSetupInstallFailureLeavesSandboxUsable(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	calls := fakeMise(t, `[ "$1" = install ] || exit 0; echo boom >&2; exit 1`)
	code, rep, stderr := setupJSON(t)
	assert.Equal(t, 0, code, stderr)
	require.Len(t, rep.Errors, 1)
	assert.True(t, strings.HasPrefix(rep.Errors[0], "install: "), rep.Errors[0])
	assert.Contains(t, rep.Errors[0], "boom")
	setupJSON(t)
	assert.Equal(t, 2, count(t, calls))
}

// TestSetupInstallLockMissing: a mise.toml with no mise.lock is
// reported with its id and hint, and nothing is installed (RJ-340).
func TestSetupInstallLockMissing(t *testing.T) {
	s := newSandbox(t)
	require.NoError(t, os.WriteFile(filepath.Join(s.primary(), "mise.toml"), []byte("[tools]\ngh = \"2.102.0\"\n"), 0o600))
	calls := fakeMise(t, ":")
	code, rep, stderr := setupJSON(t)
	assert.Equal(t, 0, code, stderr)
	require.Len(t, rep.Errors, 1)
	assert.Contains(t, rep.Errors[0], "RJ-340 lock-missing")
	assert.Contains(t, rep.Errors[0], "fix: julieta lock --repo api")
	assert.Equal(t, 0, count(t, calls))
}

// TestSetupInstallVersionOnly: a version-only tool of a lock is the
// warning version-only-tool, recorded for status and printed once
// (04 4.1, Warnings; REQ-0617).
func TestSetupInstallVersionOnly(t *testing.T) {
	s := newSandbox(t)
	miseFiles(t, s.primary())
	fakeMise(t, ":")
	code, rep, stderr := setupJSON(t)
	require.Equal(t, 0, code, stderr)
	var got []string
	for _, w := range rep.Warnings {
		if w.Slug == "version-only-tool" {
			got = append(got, w.Message)
		}
	}
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "go:golang.org/x/tools/gopls")
	assert.Equal(t, 1, strings.Count(stderr, "warning version-only-tool: "), stderr)
}
