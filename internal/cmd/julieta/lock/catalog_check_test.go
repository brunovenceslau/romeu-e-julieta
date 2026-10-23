// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lock_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownToml is the mise.toml of testdata/unknown-tool.lock.
const unknownToml = "[tools]\n\"aqua:acme/tool\" = \"1.1.0\"\n\"npm:left-pad\" = \"1.3.0\"\n"

// unknownLock is testdata/unknown-tool.lock, read before a test changes
// the working directory.
var unknownLock = func() []byte {
	b, err := os.ReadFile(filepath.Join("testdata", "unknown-tool.lock"))
	if err != nil {
		panic(err)
	}
	return b
}()

// TestCheckWarnsUnknownTool: a backend:tool the embedded catalog lacks is
// the warning lock-unknown-tool (07 7.3, REQ-0620), once for each tool
// whatever the number of its versions, version-only tools included. It
// names the two fixes of 07 7.5, step 2, and changes neither the exit
// code nor stdout.
func TestCheckWarnsUnknownTool(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(unknownToml))
	write(t, s.api, "mise.lock", unknownLock)
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 0, code, stderr)
	assert.Equal(t, "lock api ok\n", stdout)
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	require.Len(t, lines, 2, stderr)
	assert.True(t, strings.HasPrefix(lines[0], "warning lock-unknown-tool: aqua:acme/tool "), lines[0])
	assert.True(t, strings.HasPrefix(lines[1], "warning lock-unknown-tool: npm:left-pad "), lines[1])
	for _, line := range lines {
		assert.Contains(t, line, "api", "the repo")
		assert.Contains(t, line, "pull request that adds it to the egress catalog")
		assert.Contains(t, line, "egress.tools")
	}
}

// TestCheckKnownToolsNoWarning: the lock of this repository holds only
// tools of the embedded catalog and warns of none.
func TestCheckKnownToolsNoWarning(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(miseToml))
	write(t, s.api, "mise.lock", fixture(t))
	code, _, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 0, code, stderr)
	assert.NotContains(t, stderr, "lock-unknown-tool")
}

// TestCheckWarnsBesideFindings: the warning is printed when the check
// also finds what fails it, and the exit is the finding's, 6.
func TestCheckWarnsBesideFindings(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(unknownToml+"jq = \"1.7.1\"\n"))
	write(t, s.api, "mise.lock", unknownLock)
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code, stderr)
	assert.Contains(t, stdout, "finding lock-tool-missing jq\n")
	assert.Contains(t, stderr, "warning lock-unknown-tool: aqua:acme/tool ")
	assert.Contains(t, stderr, "error RJ-328 check-findings: ")
}

// TestCheckNoWarningForAFileThatDoesNotParse: a lock that does not parse is
// the finding lock-unparsed, and the catalog has nothing to look up in it.
func TestCheckNoWarningForAFileThatDoesNotParse(t *testing.T) {
	s := newSandbox(t)
	write(t, s.api, "mise.toml", []byte(unknownToml))
	write(t, s.api, "mise.lock", []byte("lockfile_version = 3\nsurprise = 1\n"))
	code, stdout, stderr := julieta(t, "lock", "--check")
	assert.Equal(t, 6, code, stderr)
	assert.Contains(t, stdout, "finding lock-unparsed mise.lock: ")
	assert.NotContains(t, stderr, "lock-unknown-tool")
}
