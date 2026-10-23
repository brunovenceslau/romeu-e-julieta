// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
)

// sum is a well-formed checksum.
const sum = "sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386"

// lockWith returns a lock that holds the tool gh, backend aqua:cli/cli,
// on the platforms given with the checksum given for each.
func lockWith(checksum string, platforms ...string) string {
	var b strings.Builder
	b.WriteString("lockfile_version = 3\n\n[[tools.gh]]\nversion = \"2.102.0\"\nbackend = \"aqua:cli/cli\"\nspecifiers = [\"2.102.0\"]\n")
	for _, p := range platforms {
		b.WriteString("\n[tools.gh.\"platforms." + p + "\"]\n")
		if checksum != "" {
			b.WriteString("checksum = \"" + checksum + "\"\n")
		}
		b.WriteString("url = \"https://example.invalid/gh\"\n")
	}
	return b.String()
}

// ghToml names the tool gh.
const ghToml = "[tools]\ngh = \"2.102.0\"\n"

// check runs Check on the sources, parsed as a repo's files.
func check(t *testing.T, toml, lock, exceptions string) mise.Report {
	t.Helper()
	in := mise.Raw{Config: []byte(toml), HasConfig: true, Lock: []byte(lock), HasLock: lock != "", Exceptions: []byte(exceptions), HasExceptions: exceptions != ""}
	return in.Check()
}

// TestCheck holds the rules of lock --check (07 7.3, REQ-0619,
// REQ-0407 as amended on 2026-10-22): a tool without an entry, a
// lockable entry without both linux platforms, an entry without a
// checksum and one whose checksum is not sha256: and 64 lowercase
// hexadecimal digits are each a finding.
func TestCheck(t *testing.T) {
	both := []string{"linux-x64", "linux-arm64"}
	tests := []struct {
		name      string
		toml      string
		lock      string
		exception string
		want      []mise.Finding
	}{
		{"fresh", ghToml, lockWith(sum, both...), "", nil},
		{"a platform of the repo is not required", ghToml, lockWith(sum, append([]string{"macos-x64"}, both...)...), "", nil},
		{"a tool without an entry", ghToml + "jq = \"1\"\n", lockWith(sum, both...), "", []mise.Finding{{Slug: mise.ToolMissing, Tool: "jq"}}},
		{"a linux platform missing", ghToml, lockWith(sum, "linux-x64"), "", []mise.Finding{{Slug: mise.PlatformMissing, Tool: "gh", Platform: "linux-arm64"}}},
		{"both linux platforms missing", ghToml, lockWith(sum, "macos-x64"), "", []mise.Finding{
			{Slug: mise.PlatformMissing, Tool: "gh", Platform: "linux-x64"}, {Slug: mise.PlatformMissing, Tool: "gh", Platform: "linux-arm64"}}},
		{"no checksum", ghToml, lockWith("", both...), "", []mise.Finding{
			{Slug: mise.NoChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.NoChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"an md5 checksum", ghToml, lockWith("md5:zz", both...), "", []mise.Finding{
			{Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"an empty sha256", ghToml, lockWith("sha256:", both...), "", []mise.Finding{
			{Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"an upper case sha256", ghToml, lockWith("sha256:"+strings.ToUpper(sum[len("sha256:"):]), both...), "", []mise.Finding{
			{Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"a short sha256", ghToml, lockWith(sum[:len(sum)-1], both...), "", []mise.Finding{
			{Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"a long sha256", ghToml, lockWith(sum+"0", both...), "", []mise.Finding{
			{Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"an exception excuses the missing checksum of its tool", ghToml, lockWith("", both...), "gh the vendor publishes none\n", nil},
		{"an exception does not excuse a malformed one", ghToml, lockWith("md5:zz", both...), "gh the vendor publishes none\n", []mise.Finding{
			{Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.BadChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"an exception does not excuse a missing platform", ghToml, lockWith("", "linux-x64"), "gh the vendor publishes none\n", []mise.Finding{
			{Slug: mise.PlatformMissing, Tool: "gh", Platform: "linux-arm64"}}},
		{"an exception of another tool excuses nothing", ghToml, lockWith("", both...), "jq nothing\n", []mise.Finding{
			{Slug: mise.NoChecksum, Tool: "gh", Platform: "linux-arm64"}, {Slug: mise.NoChecksum, Tool: "gh", Platform: "linux-x64"}}},
		{"a version-only tool by its key needs no platform and no checksum", "[tools]\n\"go:example.invalid/x\" = \"1\"\n",
			"lockfile_version = 3\n[[tools.\"go:example.invalid/x\"]]\nversion = \"1\"\nbackend = \"go:example.invalid/x\"\n", "", nil},
		{"the backend a lock entry claims does not make its tool version-only", ghToml,
			"lockfile_version = 3\n[[tools.gh]]\nversion = \"2.102.0\"\nbackend = \"go:example.invalid/gh\"\n", "", []mise.Finding{
				{Slug: mise.PlatformMissing, Tool: "gh", Platform: "linux-x64"}, {Slug: mise.PlatformMissing, Tool: "gh", Platform: "linux-arm64"}}},
		{"a short name of a version-only core tool", "[tools]\nrust = \"1\"\n", "lockfile_version = 3\n[[tools.rust]]\nversion = \"1\"\nbackend = \"core:rust\"\n", "", nil},
		{"a lock that does not parse", ghToml, "lockfile_version = 3\nlockfile_version = 3\n", "", []mise.Finding{{Slug: mise.Unparsed, Tool: "mise.lock"}}},
		{"a mise.toml that does not parse", "[tools\n", lockWith(sum, both...), "", []mise.Finding{{Slug: mise.Unparsed, Tool: "mise.toml"}}},
		{"an exceptions file that does not parse", ghToml, lockWith(sum, both...), "gh\n", []mise.Finding{{Slug: mise.Unparsed, Tool: mise.ExceptionsFile}}},
		{"findings follow the order of mise.toml", "[tools]\nzz = \"1\"\naa = \"1\"\n", lockWith(sum, both...), "", []mise.Finding{
			{Slug: mise.ToolMissing, Tool: "zz"}, {Slug: mise.ToolMissing, Tool: "aa"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := check(t, tt.toml, tt.lock, tt.exception).Findings
			for i := range got {
				got[i].Detail = "" // the text of a parse error is the decoder's
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCheckNoLock: a mise.toml with no mise.lock misses each tool.
func TestCheckNoLock(t *testing.T) {
	rep := mise.Raw{Config: []byte(ghToml + "jq = \"1\"\n"), HasConfig: true}.Check()
	assert.Equal(t, []mise.Finding{{Slug: mise.ToolMissing, Tool: "gh"}, {Slug: mise.ToolMissing, Tool: "jq"}}, rep.Findings)
	assert.Empty(t, mise.Raw{}.Check().Findings, "no mise.toml, nothing to check")
}

// TestFindingString prints a finding as one line, with the decoder's
// text after a colon for a file that does not parse.
func TestFindingString(t *testing.T) {
	assert.Equal(t, "lock-tool-missing jq", mise.Finding{Slug: mise.ToolMissing, Tool: "jq"}.String())
	assert.Equal(t, "lock-no-checksum gh linux-x64", mise.Finding{Slug: mise.NoChecksum, Tool: "gh", Platform: "linux-x64"}.String())
	assert.Equal(t, "lock-unparsed mise.lock: boom", mise.Finding{Slug: mise.Unparsed, Tool: "mise.lock", Detail: "boom"}.String())
}

// TestVersionOnly names the backends the pinned mise exempts from the
// lock's URL check, by the key of mise.toml (07 7.3), and the short
// names of the core tools among them.
func TestVersionOnly(t *testing.T) {
	for _, k := range []string{"go:golang.org/x/tools/gopls", "npm:prettier", "cargo:ripgrep", "pipx:black", "pypi:black", "gem:rake", "asdf:foo", "ubi:cli/cli", "core:rust", "core:dotnet", "core:swift", "vfox:foo", "rust", "dotnet", "swift"} {
		assert.True(t, mise.VersionOnly(k), k)
	}
	for _, k := range []string{"go", "node", "core:go", "core:node", "aqua:cli/cli", "github:cli/cli", "http:x", "golangci-lint", "gh"} {
		assert.False(t, mise.VersionOnly(k), k)
	}
}

// TestVersionOnlyTools lists the version-only tools of a repo, by the
// key of mise.toml, once each, for the warning of install (REQ-0617).
func TestVersionOnlyTools(t *testing.T) {
	cfg, err := mise.ParseConfig([]byte("[tools]\ngh = \"1\"\n\"go:a/b\" = \"1\"\n\"npm:c\" = \"1\"\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"go:a/b", "npm:c"}, mise.VersionOnlyTools(cfg))
}

// TestParseExceptions reads the exceptions file: one tool and its
// reason on each line, comments and blank lines left out.
func TestParseExceptions(t *testing.T) {
	ex, err := mise.ParseExceptions([]byte("# why\n\ngh  the vendor publishes no checksum\n\"http:x\" is not a key\n"))
	require.NoError(t, err)
	assert.Equal(t, mise.Exceptions{"gh": "the vendor publishes no checksum", "\"http:x\"": "is not a key"}, ex)
	for _, bad := range []string{"gh\n", "gh \n", "gh a\ngh b\n", "g\x00h reason\n", strings.Repeat("x", mise.MaxFileSize+1)} {
		_, err := mise.ParseExceptions([]byte(bad))
		assert.Error(t, err, "%q", bad[:min(len(bad), 20)])
	}
}

// writeRepo writes the files of a repo into a temp dir and returns it.
func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(data), 0o600))
	}
	return dir
}

// TestCheckDir reads a repo's files from disk, the exceptions file
// among them.
func TestCheckDir(t *testing.T) {
	both := []string{"linux-x64", "linux-arm64"}
	t.Run("fresh", func(t *testing.T) {
		rep, raw, err := mise.CheckDir(writeRepo(t, map[string]string{"mise.toml": ghToml, "mise.lock": lockWith(sum, both...)}))
		require.NoError(t, err)
		assert.True(t, raw.HasConfig)
		assert.True(t, raw.HasLock)
		assert.Empty(t, rep.Findings)
	})
	t.Run("an exception of the repo", func(t *testing.T) {
		files := map[string]string{"mise.toml": ghToml, "mise.lock": lockWith("", both...)}
		rep, _, err := mise.CheckDir(writeRepo(t, files))
		require.NoError(t, err)
		assert.Len(t, rep.Findings, 2)
		files[mise.ExceptionsFile] = "gh no checksum upstream\n"
		rep, raw, err := mise.CheckDir(writeRepo(t, files))
		require.NoError(t, err)
		assert.True(t, raw.HasExceptions)
		assert.Empty(t, rep.Findings)
	})
	t.Run("no mise.toml", func(t *testing.T) {
		rep, raw, err := mise.CheckDir(writeRepo(t, nil))
		require.NoError(t, err)
		assert.False(t, raw.HasConfig)
		assert.Empty(t, rep.Findings)
	})
	t.Run("a lock that does not parse is a finding", func(t *testing.T) {
		rep, _, err := mise.CheckDir(writeRepo(t, map[string]string{"mise.toml": ghToml, "mise.lock": "lockfile_version = 3\nlockfile_version = 3\n"}))
		require.NoError(t, err)
		require.Len(t, rep.Findings, 1)
		assert.Equal(t, mise.Unparsed, rep.Findings[0].Slug)
	})
	t.Run("a lock that is a link is an error", func(t *testing.T) {
		dir := writeRepo(t, map[string]string{"mise.toml": ghToml, "real.lock": lockWith(sum, both...)})
		require.NoError(t, os.Symlink("real.lock", filepath.Join(dir, "mise.lock")))
		_, _, err := mise.CheckDir(dir)
		require.Error(t, err)
	})
}

// TestRawParse parses what install needs: both files, or the error
// that names the one that failed.
func TestRawParse(t *testing.T) {
	f, err := mise.Raw{Config: []byte(ghToml), HasConfig: true, Lock: []byte(lockWith(sum, "linux-x64", "linux-arm64")), HasLock: true}.Parse()
	require.NoError(t, err)
	assert.Equal(t, []string{"gh"}, f.Config.Tools)
	assert.Len(t, f.Lock.Tools["gh"], 1)
	_, err = mise.Raw{Config: []byte(ghToml), HasConfig: true, Lock: []byte("x ="), HasLock: true}.Parse()
	require.ErrorIs(t, err, mise.ErrUnparsed)
	assert.Contains(t, err.Error(), "mise.lock")
	_, err = mise.Raw{Config: []byte("[t"), HasConfig: true}.Parse()
	require.ErrorIs(t, err, mise.ErrUnparsed)
	assert.Contains(t, err.Error(), "mise.toml")
}

// TestRefuseIrregularLock: a mise.lock that is not a regular file is
// refused before mise writes it (the audit of 2026-10-22).
func TestRefuseIrregularLock(t *testing.T) {
	require.NoError(t, mise.CheckLockTarget(t.TempDir()), "no lock yet")
	dir := writeRepo(t, map[string]string{"mise.lock": "x"})
	require.NoError(t, mise.CheckLockTarget(dir))
	link := t.TempDir()
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(link, "mise.lock")))
	require.Error(t, mise.CheckLockTarget(link))
	dirLock := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dirLock, "mise.lock"), 0o755))
	require.Error(t, mise.CheckLockTarget(dirLock))
}
