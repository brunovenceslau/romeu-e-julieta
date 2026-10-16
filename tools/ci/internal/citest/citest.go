// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package citest holds the fixtures that the tests of the tools/ci
// checks share: a fixture repository, a fake mise, the workflow and the
// cover profile of the fixtures. It imports no check package, so the
// test of every check may import it.
package citest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git/gittest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/names"
	"github.com/stretchr/testify/require"
)

// FixtureModule is the module path of the fixture cover profiles.
const FixtureModule = "example.invalid/m"

// ProfileOf writes a cover profile with one block per entry: the
// package path relative to the module, the statements, and the count.
func ProfileOf(blocks ...string) []byte {
	var b strings.Builder
	b.WriteString("mode: atomic\n")
	for i, block := range blocks {
		pkg, counts, _ := strings.Cut(block, " ")
		b.WriteString(FixtureModule + "/" + pkg + "/f.go:" + strings.Repeat("1", i+1) + ".2,3.4 " + counts + "\n")
	}
	return []byte(b.String())
}

// MadeUp is the forbidden name of every fixture here: an invented word
// pair that names nothing.
const MadeUp = "zorvex quimby"

// FixtureProse is a made-up word list, so that no test file holds a
// word the repository really bans.
const FixtureProse = `rules:
  - id: shiny
    phrases: [glimmerous]
    fails: [a glimmerous tool]
    passes: [a plain tool]
`

// NewTree returns a fixture repository that passes hygiene: the hook,
// the two data files, a denylist with the made-up name, and one page.
// Nothing is committed yet. The hook is the one this repository tracks,
// which the hook test of the mise package holds to its lists.
func NewTree(t *testing.T) *gittest.Repo {
	t.Helper()
	hook, err := os.ReadFile(filepath.Join(ModuleRoot(t), ".githooks", "pre-push"))
	require.NoError(t, err)
	r := gittest.New(t)
	r.Write(".githooks/pre-push", string(hook))
	require.NoError(t, os.Chmod(filepath.Join(r.Dir, ".githooks", "pre-push"), 0o744))
	r.Write("tools/ci/prose.yaml", FixtureProse)
	r.Write(names.Path, string(Denylist(t, MadeUp)))
	r.Write("README.md", "# A fixture\n\nPlain text.\n")
	return r
}

// Denylist returns a denylist that holds the made-up names.
func Denylist(t *testing.T, madeUpNames ...string) []byte {
	t.Helper()
	l := &names.List{}
	for _, n := range madeUpNames {
		e, err := names.NewEntry(n)
		require.NoError(t, err)
		l.Add(e)
	}
	return l.Marshal()
}

// MustAbs returns the absolute path of p.
func MustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	require.NoError(t, err)
	return abs
}

// MustEval returns p with its symbolic links resolved.
func MustEval(t *testing.T, p string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return real
}

// ModuleRoot returns the root of this repository: the nearest directory
// above the test's own that holds go.mod.
func ModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "no go.mod above the test's directory")
		dir = parent
	}
}

// Sha is a made-up commit id, 40 hex digits.
const Sha = "0123456789abcdef0123456789abcdef01234567"

// GoodWorkflow passes the grammar: every construct it allows, once.
const GoodWorkflow = `name: ci
on:
  pull_request:
    types: [opened, synchronize, reopened, edited]
  push:
    branches: [main]
  schedule:
    - cron: "17 3 * * 1"
  workflow_dispatch:
permissions:
  contents: read
concurrency:
  group: ci-${{ github.ref }}
` + WorkflowEnv + `jobs:
  all:
    name: all on ${{ matrix.os }}
    runs-on: ${{ matrix.os }}
    needs: []
    permissions:
      contents: read
    timeout-minutes: 60
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: ubuntu-26.04
            mise_sha256: aa
    steps:
      - name: Check out
        uses: actions/checkout@` + Sha + `
        with:
          persist-credentials: false
      - uses: jdx/mise-action/sub/path@` + Sha + `
        with:
          sha256: ${{ matrix.mise_sha256 }}
      - name: All checks
        run: go run ./tools/ci all
      - run: go run ./tools/release notes --tag "$GITHUB_REF_NAME" --out=notes.md
`

// WorkflowEnv is the env table every workflow holds: miseEnv, each
// value a YAML string.
const WorkflowEnv = `env:
  MISE_OVERRIDE_CONFIG_FILENAMES: mise.toml
  MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES: none
  MISE_ENV: ""
  MISE_AUTO_ENV: "false"
  MISE_GLOBAL_CONFIG_FILE: /dev/null/mise-global.toml
  MISE_SYSTEM_CONFIG_FILE: /dev/null/mise-system.toml
  GOENV: "off"
  GOTOOLCHAIN: local
`

// Edits returns goodWorkflow with each pair of olds and news replaced.
// Edits returns GoodWorkflow with each pair of old and new text
// replaced, once each, and fails when an old text is not there.
func Edits(t *testing.T, pairs ...string) string {
	t.Helper()
	src := GoodWorkflow
	for i := 0; i+1 < len(pairs); i += 2 {
		require.Contains(t, src, pairs[i], "the fixture's text to replace")
		src = strings.Replace(src, pairs[i], pairs[i+1], 1)
	}
	return src
}

// Passing and Failing are steps that pass and fail, for the tests that
// replace the real steps of a run.
var (
	Passing = check.Step{Name: "passes", Argv: []string{"go", "version"}}
	Failing = check.Step{Name: "fails", Argv: []string{"go", "no-such-command"}}
)

// IsolateMiseConfig gives the real mise that a test starts through
// check.PassThroughEnv a state directory of the test's own, new and empty, so
// the trust records of the person who runs the tests do not change what
// the test measures. HOME and the data directory variables stay: the
// pinned tools are installed where mise puts them. No global or system
// configuration reaches mise (miseEnv names none). A test that builds
// the whole environment of mise itself uses isolatedMiseEnv instead.
func IsolateMiseConfig(t *testing.T) {
	t.Helper()
	t.Setenv("MISE_STATE_DIR", t.TempDir())
}

// FakeMise is a home directory with the mise installs directory of
// miseInstalls, and a directory on PATH that holds a mise of its own,
// a script, in place of the real one. HOME and PATH point at them, and
// MISE_DATA_DIR and XDG_DATA_HOME are cleared.
type FakeMise struct {
	Home, Bin string
}

// NewFakeMise creates the fake mise, and points HOME and PATH at it.
func NewFakeMise(t *testing.T) FakeMise {
	t.Helper()
	f := FakeMise{Home: t.TempDir(), Bin: t.TempDir()}
	require.NoError(t, os.MkdirAll(f.Installs(), 0o755))
	t.Setenv("HOME", f.Home)
	t.Setenv("PATH", f.Bin)
	t.Setenv("MISE_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	return f
}

// Installs returns the mise installs directory of the fake home.
func (f FakeMise) Installs() string {
	return filepath.Join(f.Home, ".local", "share", "mise", "installs")
}

// Tool creates an executable file below the installs directory.
func (f FakeMise) Tool(t *testing.T, rel ...string) string {
	t.Helper()
	return Executable(t, filepath.Join(append([]string{f.Installs()}, rel...)...))
}

// Executable creates an empty shell script at path.
func Executable(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700))
	return path
}

// Script writes the fake mise: it runs prelude, then prints linter for
// "mise -C <dir> which golangci-lint" and goCmd for "... which go".
func (f FakeMise) Script(t *testing.T, prelude, linter, goCmd string) {
	t.Helper()
	script := "#!/bin/sh\n" + prelude + "case \"$4\" in\ngolangci-lint) echo '" + linter + "' ;;\ngo) echo '" + goCmd + "' ;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte(script), 0o700))
}

// LockFixture is the mise.lock of the fakes: the versions the fake
// installs below carry.
const LockFixture = `[[tools.go]]
version = "1.27.0"

[[tools."go:golang.org/x/vuln/cmd/govulncheck"]]
version = "1.8.0"

[[tools.golangci-lint]]
version = "2.14.0"
`

// LockedRoot returns a directory that holds a mise.lock with lock.
func LockedRoot(t *testing.T, lock string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "mise.lock"), []byte(lock), 0o600))
	return root
}
