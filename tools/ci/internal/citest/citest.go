// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package citest holds the fixtures that the tests of the tools/ci
// checks share: a fixture repository, a fake mise, the workflow and the
// cover profile of the fixtures. It imports no check package, so the
// test of every check may import it.
package citest

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/git"
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

// ValidAskFirst is the smallest list that passes: one surface, which is
// the list itself.
const ValidAskFirst = `version: 1
owner: "@someone"
surfaces:
  - id: ask-first
    globs: [.github/ask-first.yaml]
    reason: this list
`

// IsolatedMiseEnv is an environment for the real mise built from
// nothing but PATH: its home, configuration, state and cache
// directories are new and empty, the system and global configuration
// files do not exist, and it stays off the network. So no
// configuration of the person who runs the tests (a global
// "paranoid = true", say) changes what a test measures. A test that
// needs the pinned tools below HOME uses IsolateMiseConfig instead.
func IsolatedMiseEnv(t *testing.T) []string {
	t.Helper()
	none := filepath.Join(t.TempDir(), "none.toml")
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"XDG_CONFIG_HOME=" + t.TempDir(),
		"MISE_CONFIG_DIR=" + t.TempDir(),
		"MISE_STATE_DIR=" + t.TempDir(),
		"MISE_CACHE_DIR=" + t.TempDir(),
		"MISE_SYSTEM_CONFIG_FILE=" + none,
		"MISE_GLOBAL_CONFIG_FILE=" + none,
		"MISE_OFFLINE=1",
	}
}

// FakeMiseVersion is the version the fake mise of a test reports, and
// AnswerVersion the lines of its script that report it, as "mise
// --version" prints it.
const (
	FakeMiseVersion = "2026.10.3"
	AnswerVersion   = "if [ \"$1\" = --version ]; then echo '" + FakeMiseVersion + " linux-x64 (2026-10-05)'; exit 0; fi\n"
)

// MiseWorkflow returns GoodWorkflow with its mise-action step pinning
// mise at version.
func MiseWorkflow(t *testing.T, version string) string {
	t.Helper()
	return Edits(t, "          sha256: ${{ matrix.mise_sha256 }}\n", "          version: "+version+"\n          sha256: ${{ matrix.mise_sha256 }}\n")
}

// PinnedLintTools returns the pinned tools as resolve finds them for
// this repository, run in a mise state directory of the test's own
// (IsolateMiseConfig), where the test first trusts the repository's
// mise configuration. mise keeps its trust records in the state
// directory (https://mise.jdx.dev/directories.html), so the tests do
// not depend on a "mise trust" of the person who runs them, and leave
// no trust behind. environ is the environment of a mise run; it and
// resolve come from the mise package, which citest does not import, as
// the tests of that package import citest.
func PinnedLintTools(t *testing.T, environ func([]string) []string, resolve func(context.Context, string) (check.Tools, error)) check.Tools {
	t.Helper()
	root := ModuleRoot(t)
	IsolateMiseConfig(t)
	out, err := procgroup.CombinedOutput(t.Context(), procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{"mise", "-C", root, "trust"},
		Dir:   root,
		Env:   environ(check.PassThroughEnv()),
	})
	require.NoError(t, err, "mise trust\n%s", out)
	tools, err := resolve(t.Context(), root)
	require.NoError(t, err)
	return tools
}

// Run runs one subcommand in the fixture r, with stdin (an empty input
// when nil), and returns its exit status, as the dispatcher of tools/ci
// computes it (check.Status), and what it printed to standard output
// and standard error, in the order it printed it.
func Run(t *testing.T, r *gittest.Repo, stdin io.Reader, run func(context.Context, check.Env, []string) (bool, error), args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: stdin, Stdout: &out, Stderr: &out}
	ok, err := run(t.Context(), e, args)
	return check.Status(&out, ok, err), out.String()
}

// PartialClone clones upstream with --filter=blob:none and no checkout,
// so the clone holds every commit and tree and no blob; a later fetch
// brings no blob either. It returns the clone. The fixture's git runs
// with GIT_NO_LAZY_FETCH=1 (git.Repo.Run), so a checkout could not
// fetch the blobs it needs.
func PartialClone(t *testing.T, upstream *gittest.Repo) git.Repo {
	t.Helper()
	upstream.Git("config", "uploadpack.allowFilter", "true")
	dir := filepath.Join(t.TempDir(), "clone")
	upstream.Git("clone", "--quiet", "--filter=blob:none", "--no-checkout", "file://"+upstream.Dir, dir)
	return git.Repo{Dir: dir, Env: upstream.Env}
}

// RangeOf reads the verdicts of pushedRange back as its findings, and
// whether names were matched: no verdict carries the "not checked"
// note. A verdict that could not judge is returned as its error.
func RangeOf(verdicts []check.Verdict, err error) (findings []check.Finding, checked bool, _ error) {
	checked = true
	for _, v := range verdicts {
		switch {
		case v.Err != nil:
			return nil, false, v.Err
		case v.Note != "":
			checked = false
		}
		findings = append(findings, v.Findings...)
	}
	return findings, checked, err
}
