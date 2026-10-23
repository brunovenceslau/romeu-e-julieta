// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// julietaBin builds julieta from this tree, with the go the tests run
// with, as the check builds it with the pinned one, and returns the
// path of the binary.
func julietaBin(t *testing.T) string {
	t.Helper()
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	bin := filepath.Join(t.TempDir(), "julieta")
	require.NoError(t, build(t.Context(), goBin, citest.ModuleRoot(t), bin))
	return bin
}

// lockOf is a lock that holds gh on both linux platforms and a
// version-only tool.
const lockOf = `lockfile_version = 3

[[tools.gh]]
version = "2.102.0"
backend = "aqua:cli/cli"
specifiers = ["2.102.0"]

[tools.gh."platforms.linux-arm64"]
checksum = "sha256:7862c86c72f43df3a2d93ddde6f473285b4e2af61b494849846827e513ef6484"

[tools.gh."platforms.linux-x64"]
checksum = "sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386"

[[tools."go:golang.org/x/tools/gopls"]]
version = "0.23.0"
backend = "go:golang.org/x/tools/gopls"
specifiers = ["0.23.0"]
`

// tomlOf names the two tools of lockOf.
const tomlOf = "[tools]\ngh = \"2.102.0\"\n\"go:golang.org/x/tools/gopls\" = \"0.23.0\"\n"

// tree makes a tree of the files given below a new directory and returns it.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	return root
}

// TestMiseDirs lists the top of the tree and each directory below
// examples/ that holds a mise.toml, sorted, and none that holds only a
// lock.
func TestMiseDirs(t *testing.T) {
	root := tree(t, map[string]string{
		"mise.toml": tomlOf, "examples/b/mise.toml": tomlOf, "examples/a/x/mise.toml": tomlOf,
		"examples/c/mise.lock": lockOf, "other/mise.toml": tomlOf,
	})
	dirs, err := miseDirs(root)
	require.NoError(t, err)
	assert.Equal(t, []string{".", "examples/a/x", "examples/b"}, dirs)

	dirs, err = miseDirs(tree(t, map[string]string{"README.md": "x"}))
	require.NoError(t, err)
	assert.Empty(t, dirs)
}

// TestJulietaLockCheck runs julieta lock --check of this tree on a
// directory: a fresh lock has no finding, a mise.toml with a tool the
// lock lacks has one, with no lock each tool is missing, an entry
// without a checksum is a finding too (REQ-0407 as decided on
// 2026-10-22), and a lock that does not parse is a finding, not an error
// of the check (REQ-0406).
func TestJulietaLockCheck(t *testing.T) {
	bin := julietaBin(t)
	tmp := t.TempDir()
	lockCheck := func(t *testing.T, files map[string]string) []check.Finding {
		t.Helper()
		root := tree(t, files)
		f, err := checkDir(t.Context(), bin, root, tmp, ".")
		require.NoError(t, err)
		return f
	}
	t.Run("fresh", func(t *testing.T) {
		assert.Empty(t, lockCheck(t, map[string]string{"mise.toml": tomlOf, "mise.lock": lockOf}))
	})
	t.Run("a tool the lock lacks", func(t *testing.T) {
		f := lockCheck(t, map[string]string{"mise.toml": tomlOf + "jq = \"1.7\"\n", "mise.lock": lockOf})
		require.Len(t, f, 1)
		assert.Equal(t, "mise.lock", f[0].Where)
		assert.Equal(t, rule, f[0].Rule)
		assert.Equal(t, "lock-tool-missing jq", f[0].Msg)
	})
	t.Run("a platform the lock lacks", func(t *testing.T) {
		f := lockCheck(t, map[string]string{"mise.toml": tomlOf, "mise.lock": strings.Replace(lockOf, `"platforms.linux-arm64"`, `"platforms.macos-arm64"`, 1)})
		require.Len(t, f, 1)
		assert.Equal(t, "lock-platform-missing gh linux-arm64", f[0].Msg)
	})
	t.Run("no lock", func(t *testing.T) {
		assert.Len(t, lockCheck(t, map[string]string{"mise.toml": tomlOf}), 2)
	})
	t.Run("a missing checksum is a finding", func(t *testing.T) {
		noSum := strings.Replace(lockOf, `checksum = "sha256:bb766f710eef8ede859c18578c72c327597cd4c8a85b06001b1f3843c6019386"`+"\n", "", 1)
		f := lockCheck(t, map[string]string{"mise.toml": tomlOf, "mise.lock": noSum})
		require.Len(t, f, 1)
		assert.Equal(t, "lock-no-checksum gh linux-x64", f[0].Msg)
	})
	t.Run("a lock that does not parse is a finding", func(t *testing.T) {
		f := lockCheck(t, map[string]string{"mise.toml": tomlOf, "mise.lock": "lockfile_version = 3\n" + lockOf})
		require.Len(t, f, 1)
		assert.True(t, strings.HasPrefix(f[0].Msg, "lock-unparsed mise.lock: "), f[0].Msg)
	})
	t.Run("a directory below the tree", func(t *testing.T) {
		root := tree(t, map[string]string{"examples/shop/mise.toml": tomlOf + "jq = \"1.7\"\n", "examples/shop/mise.lock": lockOf})
		f, err := checkDir(t.Context(), bin, root, tmp, "examples/shop")
		require.NoError(t, err)
		require.Len(t, f, 1)
		assert.Equal(t, "examples/shop/mise.lock", f[0].Where)
	})
}

// TestReport prints the findings and the summary and says whether the
// tree is clean: a stale lock is not ok (REQ-0845).
func TestReport(t *testing.T) {
	bin := julietaBin(t)
	tmp := t.TempDir()
	var out bytes.Buffer
	ok, err := report(t.Context(), &out, bin, tree(t, map[string]string{"mise.toml": tomlOf, "mise.lock": lockOf}), tmp, []string{"."})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "mise: ok\n", out.String())

	out.Reset()
	stale := tree(t, map[string]string{"mise.toml": tomlOf + "jq = \"1\"\n", "mise.lock": lockOf, "examples/a/mise.toml": tomlOf, "examples/a/mise.lock": lockOf})
	ok, err = report(t.Context(), &out, bin, stale, tmp, []string{".", "examples/a"})
	require.NoError(t, err)
	assert.False(t, ok, "a stale lock fails the check")
	assert.Contains(t, out.String(), "lock-tool-missing jq")
	assert.True(t, strings.HasSuffix(out.String(), "mise: 1 finding(s)\n"), out.String())
}

// fakeJulieta writes a program that runs script, standing for julieta.
func fakeJulieta(t *testing.T, script string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "julieta")
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o700))
	return p
}

// TestCheckDirExits: an exit of 6 with a finding is the finding, an exit
// of 6 with none, any other exit and a failure to start are errors of
// the check, never a pass.
func TestCheckDirExits(t *testing.T) {
	root := tree(t, map[string]string{"mise.toml": tomlOf})
	tmp := t.TempDir()
	f, err := checkDir(t.Context(), fakeJulieta(t, `echo "finding lock-tool-missing x"; exit 6`), root, tmp, ".")
	require.NoError(t, err)
	require.Len(t, f, 1)
	assert.Equal(t, "lock-tool-missing x", f[0].Msg)

	_, err = checkDir(t.Context(), fakeJulieta(t, `echo "no finding line here"; exit 6`), root, tmp, ".")
	require.Error(t, err, "an exit of 6 with no finding is not a pass")
	assert.Contains(t, err.Error(), "no finding")

	_, err = checkDir(t.Context(), fakeJulieta(t, `echo boom >&2; exit 2`), root, tmp, ".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")

	f, err = checkDir(t.Context(), fakeJulieta(t, `echo "finding lock-tool-missing x"; exit 0`), root, tmp, ".")
	require.NoError(t, err)
	assert.Len(t, f, 1, "a finding line is a finding whatever the exit")

	_, err = checkDir(t.Context(), filepath.Join(t.TempDir(), "absent"), root, tmp, ".")
	require.Error(t, err)
}

// TestRunOnThisRepository runs the subcommand on this repository, whose
// lock is fresh (REQ-0845), and takes no argument.
func TestRunOnThisRepository(t *testing.T) {
	var out bytes.Buffer
	ok, err := Run(t.Context(), check.Env{Dir: citest.ModuleRoot(t), Stdout: &out}, nil)
	require.NoError(t, err, out.String())
	assert.True(t, ok, out.String())
	assert.Equal(t, "mise: ok\n", out.String())

	ok, err = Run(t.Context(), check.Env{Stdout: &out}, []string{"x"})
	require.Error(t, err)
	assert.False(t, ok)
}
