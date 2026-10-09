// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// sha is a made-up commit id, 40 hex digits.
const sha = "0123456789abcdef0123456789abcdef01234567"

// goodWorkflow passes the grammar: every construct it allows, once.
const goodWorkflow = `name: ci
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
` + workflowEnv + `jobs:
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
        uses: actions/checkout@` + sha + `
        with:
          persist-credentials: false
      - uses: jdx/mise-action/sub/path@` + sha + `
        with:
          sha256: ${{ matrix.mise_sha256 }}
      - name: All checks
        run: go run ./tools/ci all
      - run: go run ./tools/release notes --tag "$GITHUB_REF_NAME" --out=notes.md
`

// workflowEnv is the env table every workflow holds: miseEnv, each
// value a YAML string.
const workflowEnv = `env:
  MISE_OVERRIDE_CONFIG_FILENAMES: mise.toml
  MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES: none
  MISE_ENV: ""
  MISE_AUTO_ENV: "false"
  MISE_GLOBAL_CONFIG_FILE: /dev/null/mise-global.toml
  MISE_ENV_FILE: ""
  GOENV: "off"
  GOTOOLCHAIN: local
`

// TestWorkflowsGrammar fails a fixture for each construct outside the
// grammar of 10 10.2, and passes the one that holds every construct
// inside it. Each fixture changes one line of goodWorkflow.
func TestWorkflowsGrammar(t *testing.T) {
	tests := []struct {
		name     string
		old, new string // the text of goodWorkflow to replace, and its replacement
		want     string // a part of the one finding expected; "" for none
	}{
		{"the good workflow", "", "", ""},
		{"an if on a step", "      - name: All checks\n", "      - name: All checks\n        if: always()\n", `"if" is outside the grammar of a run step`},
		{"an if on a job", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    if: true\n", `"if" is outside the grammar of a job`},
		{"a shell", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        shell: bash\n", `"shell" is outside the grammar of a run step`},
		{"continue-on-error", "      - name: All checks\n", "      - name: All checks\n        continue-on-error: true\n", `"continue-on-error"`},
		{"a container", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    container: alpine\n", `"container" is outside the grammar of a job`},
		{"a job that calls a workflow", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    uses: owner/repo/.github/workflows/x.yml@" + sha + "\n", `"uses" is outside the grammar of a job`},
		{"a key of the file", "permissions:\n  contents: read\nconcurrency", "defaults:\n  run:\n    shell: bash\npermissions:\n  contents: read\nconcurrency", `"defaults" is outside the grammar of a file`},
		{"no env at the top", workflowEnv, "", "env holds exactly the mise environment"},
		{"an env with another key: BASH_ENV", "  MISE_AUTO_ENV: \"false\"\n", "  MISE_AUTO_ENV: \"false\"\n  BASH_ENV: /tmp/x\n", "env holds exactly the mise environment"},
		{"an env with another value", "MISE_OVERRIDE_CONFIG_FILENAMES: mise.toml", "MISE_OVERRIDE_CONFIG_FILENAMES: mise.toml,mise.local.toml", "env holds exactly the mise environment"},
		{"an env without one variable", "  MISE_ENV: \"\"\n", "", "env holds exactly the mise environment"},
		{"an env with a null MISE_ENV", `MISE_ENV: ""`, "MISE_ENV:", "env holds exactly the mise environment"},
		{"an env with a boolean", `MISE_AUTO_ENV: "false"`, "MISE_AUTO_ENV: false", "env holds exactly the mise environment"},
		{"an env in another order", "  MISE_ENV: \"\"\n  MISE_AUTO_ENV: \"false\"\n", "  MISE_AUTO_ENV: \"false\"\n  MISE_ENV: \"\"\n", "env holds exactly the mise environment"},
		{"an env that is a list", workflowEnv, "env: [a]\n", "env holds exactly the mise environment"},
		{"an event outside the list", "  workflow_dispatch:\n", "  pull_request_target:\n", `the event "pull_request_target"`},
		{"an expression with an operator", "group: ci-${{ github.ref }}", "group: ci-${{ github.ref || 'main' }}", "one context path"},
		{"an expression with a function call", "group: ci-${{ github.ref }}", "group: ci-${{ format('{0}', github.ref) }}", "one context path"},
		{"an expression with a literal", "group: ci-${{ github.ref }}", "group: ci-${{ true }}", "one context path"},
		{"an expression with a number", "group: ci-${{ github.ref }}", "group: ci-${{ 1 }}", "one context path"},
		{"an expression with an index", "group: ci-${{ github.ref }}", "group: ci-${{ github['ref'] }}", "one context path"},
		{"an expression not closed", "group: ci-${{ github.ref }}", "group: ci-${{ github.ref", "not closed"},
		{"an expression in a run line", "run: go run ./tools/ci all\n", "run: go run ./tools/ci all ${{ github.ref }}\n", "run is one line"},
		{"a uses with a tag", "uses: actions/checkout@" + sha, "uses: actions/checkout@v6", "40-hex commit SHA"},
		{"a uses with a short SHA", "uses: actions/checkout@" + sha, "uses: actions/checkout@" + sha[:39], "40-hex commit SHA"},
		{"a uses with an upper-case SHA", "uses: actions/checkout@" + sha, "uses: actions/checkout@" + strings.ToUpper(sha), "40-hex commit SHA"},
		{"a local action", "uses: actions/checkout@" + sha, "uses: ./.github/actions/x", "40-hex commit SHA"},
		{"a local action with a SHA", "uses: actions/checkout@" + sha, "uses: ./.github/actions/x@" + sha, "40-hex commit SHA"},
		{"a parent directory with a SHA", "uses: actions/checkout@" + sha, "uses: ../x@" + sha, "40-hex commit SHA"},
		{"two parent directories with a SHA", "uses: actions/checkout@" + sha, "uses: ../..@" + sha, "40-hex commit SHA"},
		{"a dot path segment", "uses: actions/checkout@" + sha, "uses: actions/checkout/../x@" + sha, "40-hex commit SHA"},
		{"a docker action", "uses: actions/checkout@" + sha, "uses: docker://alpine@sha256:" + sha + sha[:24], "40-hex commit SHA"},
		{"an env on a uses step", "          persist-credentials: false\n", "          persist-credentials: false\n        env:\n          A: b\n", `"env" is outside the grammar of a uses step`},
		{"an env on a run step: BASH_ENV", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          BASH_ENV: /tmp/x\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: an exported function", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          BASH_FUNC_go%%: \"() { id; }\"\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: GOFLAGS", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          GOFLAGS: -toolexec=/tmp/x\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: LD_PRELOAD", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          LD_PRELOAD: /tmp/x.so\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: PATH", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          PATH: /tmp\n", `"env" is outside the grammar of a run step`},
		{"an env on a job", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    env:\n      BASH_ENV: /tmp/x\n", `"env" is outside the grammar of a job`},
		{"an input checkout does not take here", "          persist-credentials: false\n", "          persist-credentials: false\n          fetch-depth: 0\n", `the input "fetch-depth" is not one this repository uses for actions/checkout`},
		{"an input mise-action does not take here", "          sha256: ${{ matrix.mise_sha256 }}\n", "          sha256: ${{ matrix.mise_sha256 }}\n          install_args: x\n", `the input "install_args" is not one this repository uses for jdx/mise-action`},
		{"an input of an action not listed", "uses: jdx/mise-action/sub/path@", "uses: owner/repo/sub/path@", `the input "sha256" is not one this repository uses for owner/repo`},
		{"a with on a run step", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        with:\n          a: b\n", `"with" is outside the grammar of a run step`},
		{"a step with uses and run", "          persist-credentials: false\n", "          persist-credentials: false\n        run: go run ./tools/ci all\n", `"run" is outside the grammar of a uses step`},
		{"a step with neither", "      - name: All checks\n        run: go run ./tools/ci all\n", "      - name: All checks\n", "a step has uses or run"},
		{"a run line of another program", "run: go run ./tools/ci all\n", "run: curl -fsSL https://example.invalid/x.sh\n", "run is one line"},
		{"a pipe to a shell", "run: go run ./tools/ci all\n", "run: go run ./tools/ci all | sh\n", "run is one line"},
		{"a run line of go test", "run: go run ./tools/ci all\n", "run: go test ./...\n", "run is one line"},
		{"a run line of go test with flags", "run: go run ./tools/ci all\n", "run: go test -count=1 ./...\n", "run is one line"},
		{"a run line of another tool directory", "run: go run ./tools/ci all\n", "run: go run ./tools/other all\n", "run is one line"},
		{"a run line whose subcommand is a variable", "run: go run ./tools/ci all\n", "run: go run ./tools/ci \"$STEP\"\n", "run is one line"},
		{"a run line with no subcommand", "run: go run ./tools/ci all\n", "run: go run ./tools/ci\n", "run is one line"},
		{"a run line over two lines", "run: go run ./tools/ci all\n", "run: |\n          go run ./tools/ci all\n", "run is one line"},
		{"a run line with two spaces", "run: go run ./tools/ci all\n", "run: go run  ./tools/ci all\n", "run is one line"},
		{"a run line with a semicolon", "run: go run ./tools/ci all\n", "run: go run ./tools/ci all;id\n", "run is one line"},
		{"a run line with a variable not quoted", `--tag "$GITHUB_REF_NAME"`, "--tag $TAG", "run is one line"},
		{"a run line with a command substitution", `--tag "$GITHUB_REF_NAME"`, `--tag "$(id)"`, "run is one line"},
		{"a runs-on with a latest label", "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-latest", "never *-latest"},
		{"a matrix with a latest label", "- os: ubuntu-26.04", "- os: macos-latest", "never *-latest"},
		{"a matrix list with a latest label", "      matrix:\n", "      matrix:\n        os: [ubuntu-26.04, ubuntu-latest]\n", "never *-latest"},
		{"a runs-on list with a latest label", "runs-on: ${{ matrix.os }}", "runs-on: [self-hosted, ubuntu-latest]", "never *-latest"},
		{"a runs-on with a latest label and a size", "runs-on: ${{ matrix.os }}", "runs-on: macos-latest-xlarge", "never *-latest"},
		{"a runs-on from a variable", "runs-on: ${{ matrix.os }}", "runs-on: ${{ vars.RUNNER }}", "never *-latest"},
		{"a self-hosted runs-on", "runs-on: ${{ matrix.os }}", "runs-on: self-hosted", "never *-latest"},
		{"a self-hosted runs-on list", "runs-on: ${{ matrix.os }}", "runs-on: [self-hosted]", "never *-latest"},
		{"a runs-on with a pinned label", "runs-on: ${{ matrix.os }}", "runs-on: macos-26-intel", ""},
		{"a matrix key in capitals", "      matrix:\n", "      matrix:\n        OS: [self-hosted]\n", `the matrix key "OS" is outside the grammar`},
		{"a matrix key of another kind", "      matrix:\n", "      matrix:\n        node: [1]\n", `the matrix key "node" is outside the grammar`},
		{"an include key in mixed case", "            mise_sha256: aa\n", "            mise_sha256: aa\n            Os: self-hosted\n", `the include key "Os" is outside the grammar`},
		{"an include key of another kind", "            mise_sha256: aa\n", "            mise_sha256: aa\n            sum: bb\n", `the include key "sum" is outside the grammar`},
		{"a matrix that is an expression", "      matrix:\n        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "      matrix: ${{ github.event.inputs.m }}\n", "a matrix is a written mapping"},
		{"an include that is an expression", "        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "        include: ${{ github.event.inputs.m }}\n", "include is a written list"},
		{"a matrix without os", "          - os: ubuntu-26.04\n            mise_sha256: aa\n", "          - mise_sha256: aa\n", "the matrix names os"},
		{"a with that is a scalar", "        with:\n          persist-credentials: false\n", "        with: x\n", "with is a mapping"},
		{"a with that is a list", "        with:\n          persist-credentials: false\n", "        with: [ref]\n", "with is a mapping"},
		{"a uses with a path that starts with a dot", "uses: actions/checkout@" + sha, "uses: ./x@sha", "40-hex commit SHA"},
		{"a local action with a real SHA", "uses: actions/checkout@" + sha, "uses: ./x@" + sha, "40-hex commit SHA"},
		{"a uses that is a root path", "uses: actions/checkout@" + sha, "uses: /x@" + sha, "40-hex commit SHA"},
		{"a uses with a parent as repository", "uses: actions/checkout@" + sha, "uses: owner/..@" + sha, "40-hex commit SHA"},
		{"a uses with an empty path segment", "uses: actions/checkout@" + sha, "uses: actions/checkout/@" + sha, "40-hex commit SHA"},
		{"a uses with a 41-hex ref", "uses: actions/checkout@" + sha, "uses: actions/checkout@" + sha + "a", "40-hex commit SHA"},
		{"a comment after the SHA", "uses: actions/checkout@" + sha, "uses: actions/checkout@" + sha + " # v6", ""},
		{"a uses with a word after the SHA", "uses: actions/checkout@" + sha, "uses: \"actions/checkout@" + sha + " x\"", "40-hex commit SHA"},
		{"a runs-on with a larger macos image", "runs-on: ${{ matrix.os }}", "runs-on: macos-26-xlarge", "never *-latest"},
		{"a runs-on with a windows image", "runs-on: ${{ matrix.os }}", "runs-on: windows-2025", "never *-latest"},
		{"a runs-on with a prefix before the name", "runs-on: ${{ matrix.os }}", "runs-on: x-ubuntu-22.04", "never *-latest"},
		{"an anchor", "permissions:\n  contents: read\nconcurrency", "permissions: &p\n  contents: read\nconcurrency", "an anchor or an alias"},
		{"a merge key where keys are free", "  group: ci-${{ github.ref }}\n", "  group: ci-${{ github.ref }}\n  <<: {a: b}\n", `the tag "!!merge"`},
		{"a custom tag", "    timeout-minutes: 60\n", "    timeout-minutes: !custom 60\n", `the tag "!custom"`},
		{"a key written twice", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    timeout-minutes: 5\n", "written twice"},
		// Allowlists of 10 10.2 (operator decision 2026-10-08).
		{"a runs-on with a pinned label outside the four", "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-22.04", "never *-latest"},
		{"a matrix os outside the four", "- os: ubuntu-26.04", "- os: macos-14", "never *-latest"},
		{"a matrix os list outside the four", "      matrix:\n", "      matrix:\n        os: [ubuntu-26.04, ubuntu-24.04]\n", "never *-latest"},
		{"a runs-on that is a mapping", "runs-on: ${{ matrix.os }}", "runs-on: {group: big}", "never *-latest"},
		{"a with value read from the event", "sha256: ${{ matrix.mise_sha256 }}", "sha256: ${{ github.event.pull_request.title }}", "a literal"},
		{"a with value that mixes text and an expression", "sha256: ${{ matrix.mise_sha256 }}", "sha256: a${{ matrix.mise_sha256 }}", "a literal"},
		{"a matrix include value with an expression", "            mise_sha256: aa\n", "            mise_sha256: ${{ github.event.pull_request.title }}\n", "a matrix value is a literal scalar"},
		{"a matrix include value that is a list", "            mise_sha256: aa\n", "            mise_sha256: [aa]\n", "a matrix value is a literal scalar"},
		{"a with value that is a matrix key no entry gives", "sha256: ${{ matrix.mise_sha256 }}", "sha256: ${{ matrix.other }}", "a literal"},
		{"the token in an expression", "group: ci-${{ github.ref }}", "group: ci-${{ github.token }}", "the token"},
		{"the token in an expression, in capitals", "group: ci-${{ github.ref }}", "group: ci-${{ GitHub.Token }}", "the token"},
		{"a secret in an expression", "group: ci-${{ github.ref }}", "group: ci-${{ secrets.X }}", "secret"},
		{"a secret in an expression, in capitals", "group: ci-${{ github.ref }}", "group: ci-${{ SECRETS.GITHUB_TOKEN }}", "secret"},
		{"a write permission at the top", "permissions:\n  contents: read\nconcurrency", "permissions:\n  contents: write\nconcurrency", "never a write"},
		{"an id-token permission at the top", "permissions:\n  contents: read\nconcurrency", "permissions:\n  id-token: write\nconcurrency", "never a write"},
		{"a write-all permission at the top", "permissions:\n  contents: read\nconcurrency", "permissions: write-all\nconcurrency", "permissions is a mapping"},
		{"a write permission in a job", "    permissions:\n      contents: read\n", "    permissions:\n      contents: write\n", "never a write"},
		{"a none permission at the top", "permissions:\n  contents: read\nconcurrency", "permissions:\n  contents: none\nconcurrency", ""},
		{"a checkout without persist-credentials", "        with:\n          persist-credentials: false\n", "", "persist-credentials: false"},
		{"a checkout that keeps the credentials", "persist-credentials: false", "persist-credentials: true", "persist-credentials: false"},
		{"a checkout with persist-credentials in capitals", "persist-credentials: false", "persist-credentials: False", "persist-credentials: false"},
		{"a checkout in capitals without persist-credentials", "        uses: actions/checkout@" + sha + "\n        with:\n          persist-credentials: false\n", "        uses: Actions/CheckOut@" + sha + "\n", "persist-credentials: false"},
		{"a checkout in capitals with persist-credentials", "uses: actions/checkout@" + sha, "uses: Actions/CheckOut@" + sha, ""},
		{"a path segment that starts with an underscore", "uses: jdx/mise-action/sub/path@", "uses: jdx/mise-action/_sub/path@", ""},
		{"an owner that starts with an underscore", "uses: actions/checkout@" + sha, "uses: _x/checkout@" + sha, "40-hex commit SHA"},
		{"a repository that starts with an underscore", "uses: actions/checkout@" + sha, "uses: actions/_checkout@" + sha, "40-hex commit SHA"},
		{"a strategy key outside the grammar", "      fail-fast: false\n", "      fail-fast: false\n      other: 1\n", `the strategy key "other"`},
		{"a second strategy key outside the grammar", "      fail-fast: false\n", "      fail-fast: false\n      name: x\n", `the strategy key "name"`},
		{"a strategy with max-parallel", "      fail-fast: false\n", "      fail-fast: false\n      max-parallel: 2\n", ""},
		{"a uses path segment of a single underscore", "uses: actions/checkout@" + sha, "uses: actions/checkout/_x@" + sha, ""},
		{"a uses path segment that starts with two dots", "uses: actions/checkout@" + sha, "uses: actions/checkout/..x@" + sha, "40-hex commit SHA"},
		{"a job key written quoted", "    timeout-minutes: 60\n", "    \"if\": true\n", `"if" is outside the grammar of a job`},
		{"a quoted key that is allowed", "    timeout-minutes: 60\n", "    \"timeout-minutes\": 60\n", ""},
		{"a strategy in flow style", "    strategy:\n      fail-fast: false\n      matrix:\n        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "    strategy: {fail-fast: false, matrix: {include: [{os: ubuntu-26.04, mise_sha256: aa}]}}\n", ""},
		{"a strategy in flow style with a latest label", "    strategy:\n      fail-fast: false\n      matrix:\n        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "    strategy: {matrix: {include: [{os: ubuntu-latest, mise_sha256: aa}]}}\n", "never *-latest"},
		{"a step in flow style", "      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n", "      - {run: \"go run ./tools/ci all\", if: true}\n", `"if" is outside the grammar of a run step`},
		{"a key with a tag that is not a string", "    timeout-minutes: 60\n", "    !!int timeout-minutes: 60\n", "key is written as a plain string"},
		{"a key with the string tag", "    timeout-minutes: 60\n", "    !!str timeout-minutes: 60\n", ""},
		{"a strategy with a bad first key", "    strategy:\n      fail-fast: false\n", "    strategy:\n      other: 1\n      fail-fast: false\n", `the strategy key "other"`},
		{"steps in flow style", "    steps:\n      - name: Check out\n        uses: actions/checkout@" + sha + "\n        with:\n          persist-credentials: false\n      - uses: jdx/mise-action/sub/path@" + sha + "\n        with:\n          sha256: ${{ matrix.mise_sha256 }}\n      - name: All checks\n        run: go run ./tools/ci all\n      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n", "    steps: [{uses: actions/checkout@" + sha + ", with: {persist-credentials: false}}, {uses: jdx/mise-action/sub/path@" + sha + ", with: {sha256: \"${{ matrix.mise_sha256 }}\"}}, {run: \"go run ./tools/ci all\"}]\n", ""},
		{"steps in flow style with a key outside the grammar", "    steps:\n      - name: Check out\n        uses: actions/checkout@" + sha + "\n        with:\n          persist-credentials: false\n      - uses: jdx/mise-action/sub/path@" + sha + "\n        with:\n          sha256: ${{ matrix.mise_sha256 }}\n      - name: All checks\n        run: go run ./tools/ci all\n      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n", "    steps: [{uses: actions/checkout@" + sha + ", with: {persist-credentials: false}}, {uses: jdx/mise-action/sub/path@" + sha + ", with: {sha256: \"${{ matrix.mise_sha256 }}\"}}, {run: \"go run ./tools/ci all\", if: true}]\n", `"if" is outside the grammar of a run step`},
		{"an include entry key with a tag that is not a string", "          - os: ubuntu-26.04\n", "          - !!int os: ubuntu-26.04\n", "key is written as a plain string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := goodWorkflow
			if tt.old != "" {
				require.Contains(t, src, tt.old, "the fixture's text to replace")
				src = strings.Replace(src, tt.old, tt.new, 1)
			}
			got := workflowFindings(".github/workflows/x.yml", []byte(src))
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].msg, tt.want)
			assert.Equal(t, "workflows", got[0].rule)
			assert.True(t, strings.HasPrefix(got[0].where, ".github/workflows/x.yml:"), "the finding names the file and a line: %s", got[0].where)
		})
	}
}

// TestWorkflowsFile fails what the grammar refuses as a whole file: a
// name with another ending, text that is not one YAML mapping, and a
// ci.yml that a change to a pull request's text does not run again.
func TestWorkflowsFile(t *testing.T) {
	tests := []struct {
		name, path, src, want string
	}{
		{"a yaml ending", ".github/workflows/ci.yaml", goodWorkflow, "ends in .yml"},
		{"no ending", ".github/workflows/README", "text", "ends in .yml"},
		{"a file in a subdirectory", ".github/workflows/old/ci.json", "{}", "ends in .yml"},
		{"not YAML", ".github/workflows/x.yml", "a: [b\n", "not YAML"},
		{"two documents", ".github/workflows/x.yml", goodWorkflow + "---\nname: two\n", "one YAML document"},
		{"a list", ".github/workflows/x.yml", "- a\n", "a mapping"},
		{"empty", ".github/workflows/x.yml", "", "not YAML"},
		{"a ci.yml without edited", ".github/workflows/ci.yml", strings.Replace(goodWorkflow, ", edited]", "]", 1), "edited"},
		{"a ci.yml without pull_request", ".github/workflows/ci.yml", strings.Replace(goodWorkflow, "  pull_request:\n    types: [opened, synchronize, reopened, edited]\n", "", 1), "edited"},
		{"a ci.yml without on", ".github/workflows/ci.yml", "name: ci\njobs: {}\n" + workflowEnv, "edited"},
		{"a ci.yml whose on is one name", ".github/workflows/ci.yml", "on: pull_request\njobs: {}\n" + workflowEnv, "edited"},
		{"a ci.yml whose on is a list", ".github/workflows/ci.yml", "on: [pull_request, push]\njobs: {}\n" + workflowEnv, "edited"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workflowFindings(tt.path, []byte(tt.src))
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].msg, tt.want)
		})
	}
	t.Run("another file without edited", func(t *testing.T) {
		src := strings.Replace(goodWorkflow, ", edited]", "]", 1)
		assert.Empty(t, workflowFindings(".github/workflows/fuzz.yml", []byte(src)))
	})
	t.Run("a ci.yml with edited", func(t *testing.T) {
		assert.Empty(t, workflowFindings(".github/workflows/ci.yml", []byte(goodWorkflow)))
	})
}

// TestWorkflowsEvents reads the forms YAML gives "on": one name, a
// list of names, and a mapping.
func TestWorkflowsEvents(t *testing.T) {
	for _, on := range []string{"push", "[push, workflow_dispatch]", "{push: {branches: [main]}}"} {
		assert.Empty(t, workflowFindings(".github/workflows/x.yml", []byte("on: "+on+"\njobs: {}\n"+workflowEnv)), "on: %s", on)
	}
	for _, on := range []string{"issues", "[push, issues]", "{issues: {}}"} {
		got := workflowFindings(".github/workflows/x.yml", []byte("on: "+on+"\njobs: {}\n"+workflowEnv))
		require.Len(t, got, 1, "on: %s: %v", on, got)
		assert.Contains(t, got[0].msg, `the event "issues"`)
	}
}

// TestWorkflowsCommitted runs the check the way fast does, on the tree
// of a fixture repository: it reads the committed workflows, and only
// those under .github/workflows.
func TestWorkflowsCommitted(t *testing.T) {
	r := newTree(t)
	r.Write(".github/workflows/ci.yml", goodWorkflow)
	r.Write("docs/example.yml", "jobs: {x: {if: true}}\n")
	r.Commit("fixture")
	got, err := workflows(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	assert.Empty(t, got)

	r.Write(".github/workflows/ci.yml", strings.Replace(goodWorkflow, "${{ matrix.os }}\n    needs", "ubuntu-latest\n    needs", 1))
	got, err = workflows(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	assert.Empty(t, got, "a change that is not committed is in no push")
	r.Commit("a floating label")
	got, err = workflows(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, ".github/workflows/ci.yml:26", got[0].where)
}

// TestWorkflowsRepository checks the workflows of this repository, as
// they are in the working tree.
func TestWorkflowsRepository(t *testing.T) {
	root := moduleRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(workflowsDir)))
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workflowsDir), e.Name()))
		require.NoError(t, err)
		assert.Empty(t, workflowFindings(workflowsDir+e.Name(), src), "%s", e.Name())
	}
}

// TestWorkflowsCIFile reads the repository's ci.yml as data and holds
// it to what T002 asks of it: the four pinned runners, mise installed
// by a pinned version and a sha256 per platform before setup, and
// setup before all.
func TestWorkflowsCIFile(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(moduleRoot(t), filepath.FromSlash(workflowsDir), "ci.yml"))
	require.NoError(t, err)
	var wf struct {
		Jobs map[string]struct {
			RunsOn   string `yaml:"runs-on"`
			Strategy struct {
				Matrix struct {
					Include []map[string]string `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
				Uses string         `yaml:"uses"`
				Run  string         `yaml:"run"`
				With map[string]any `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(src, &wf))
	require.Len(t, wf.Jobs, 1)
	job, ok := wf.Jobs["all"]
	require.True(t, ok, "the job is named all")
	assert.Equal(t, matrixRunner, job.RunsOn)

	var labels []string
	for _, entry := range job.Strategy.Matrix.Include {
		labels = append(labels, entry["os"])
		assert.Regexp(t, `^[0-9a-f]{64}$`, entry["mise_sha256"], "the mise sha256 of %s", entry["os"])
	}
	assert.Equal(t, []string{"ubuntu-26.04", "ubuntu-26.04-arm", "macos-26", "macos-26-intel"}, labels)

	index := map[string]int{}
	for i, s := range job.Steps {
		switch {
		case strings.HasPrefix(s.Uses, "jdx/mise-action@"):
			index["mise"] = i
			assert.Equal(t, "2026.10.3", s.With["version"])
			assert.Equal(t, "${{ matrix.mise_sha256 }}", s.With["sha256"])
			assert.Equal(t, false, s.With["cache"])
			assert.Equal(t, false, s.With["env"])
		case s.Run == "go run ./tools/ci setup":
			index["setup"] = i
		case s.Run == "go run ./tools/ci all":
			index["all"] = i
		}
	}
	require.Len(t, index, 3, "the mise, setup and all steps: %v", index)
	assert.Less(t, index["mise"], index["setup"])
	assert.Less(t, index["setup"], index["all"])
}

// edits returns goodWorkflow with each pair of olds and news replaced.
func edits(t *testing.T, pairs ...string) string {
	t.Helper()
	src := goodWorkflow
	for i := 0; i+1 < len(pairs); i += 2 {
		require.Contains(t, src, pairs[i], "the fixture's text to replace")
		src = strings.Replace(src, pairs[i], pairs[i+1], 1)
	}
	return src
}

const (
	goodStrategy = "    strategy:\n      fail-fast: false\n      matrix:\n        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n"
	onRunner     = "runs-on: ${{ matrix.os }}"
)

// TestWorkflowsWithValues holds a with value to a literal when the job
// has no matrix that could give it one: no strategy, or a strategy with
// no matrix. A matrix reference there names nothing, and so does an
// expression of the event.
func TestWorkflowsWithValues(t *testing.T) {
	const sha256 = "sha256: ${{ matrix.mise_sha256 }}"
	noStrategy := []string{goodStrategy, "", onRunner, "runs-on: ubuntu-26.04"}
	noMatrix := []string{goodStrategy, "    strategy:\n      fail-fast: false\n", onRunner, "runs-on: ubuntu-26.04"}
	tests := []struct {
		name  string
		pairs []string
		want  string
	}{
		{"no strategy, an event value", append([]string{sha256, "sha256: ${{ github.head_ref }}"}, noStrategy...), "a literal"},
		{"no strategy, a matrix reference", noStrategy, "a literal"},
		{"no strategy, a literal", append([]string{sha256, "sha256: aa"}, noStrategy...), ""},
		{"a strategy without a matrix, an event value", append([]string{sha256, "sha256: ${{ github.head_ref }}"}, noMatrix...), "a literal"},
		{"a strategy without a matrix, a matrix reference", noMatrix, "a literal"},
		{"a strategy without a matrix, a literal", append([]string{sha256, "sha256: aa"}, noMatrix...), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workflowFindings(".github/workflows/x.yml", []byte(edits(t, tt.pairs...)))
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].msg, tt.want)
		})
	}
}

// TestWorkflowsWrongKinds fails a value of the wrong kind where the
// grammar asks for a mapping or a list, with one finding each.
func TestWorkflowsWrongKinds(t *testing.T) {
	tests := []struct {
		name  string
		pairs []string
		want  string
	}{
		{"jobs as a list", []string{goodWorkflow[strings.Index(goodWorkflow, "jobs:"):], "jobs: []\n"}, "jobs is a mapping"},
		{"steps as a word", []string{goodWorkflow[strings.Index(goodWorkflow, "    steps:"):], "    steps: foo\n"}, "steps is a list"},
		{"a strategy as a list", []string{goodStrategy, "    strategy: [a]\n", onRunner, "runs-on: ubuntu-26.04", "sha256: ${{ matrix.mise_sha256 }}", "sha256: aa"}, "strategy is a mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workflowFindings(".github/workflows/x.yml", []byte(edits(t, tt.pairs...)))
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].msg, tt.want)
		})
	}
}

// TestWorkflowsPermissionsRequired wants a permissions mapping at the
// top of the file, or in every job.
func TestWorkflowsPermissionsRequired(t *testing.T) {
	const (
		top  = "permissions:\n  contents: read\nconcurrency"
		job  = "    permissions:\n      contents: read\n"
		bare = "  other:\n    runs-on: ubuntu-26.04\n    steps:\n      - run: go run ./tools/ci all\n"
		set  = "  other:\n    runs-on: ubuntu-26.04\n" + job + "    steps:\n      - run: go run ./tools/ci all\n"
	)
	tests := []struct {
		name  string
		pairs []string
		extra string // a second job, appended
		want  string
	}{
		{"at the top, in the job too", nil, "", ""},
		{"at the top only", []string{job, ""}, "", ""},
		{"at the top, and a job without", []string{job, ""}, bare, ""},
		{"in every job only", []string{top, "concurrency"}, "", ""},
		{"in both of two jobs", []string{top, "concurrency"}, set, ""},
		{"nowhere", []string{top, "concurrency", job, ""}, "", "permissions is set at the top of the file or in every job"},
		{"in one of two jobs", []string{top, "concurrency"}, bare, "permissions is set at the top of the file or in every job"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workflowFindings(".github/workflows/x.yml", []byte(edits(t, tt.pairs...)+tt.extra))
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].msg, tt.want)
		})
	}
}

// TestWorkflowInputWithUnreadableMatrix checks that a matrix that cannot
// be read excuses only a ${{ matrix.<key> }} reference, whose finding is
// the matrix's own: any other expression in a with value is still found.
func TestWorkflowInputWithUnreadableMatrix(t *testing.T) {
	unreadable := strings.Replace(goodWorkflow,
		"        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n",
		"        include: ${{ github.event.inputs.m }}\n", 1)
	require.NotEqual(t, goodWorkflow, unreadable)
	findings := func(with string) string {
		src := strings.Replace(unreadable, "sha256: ${{ matrix.mise_sha256 }}", "sha256: "+with, 1)
		var msgs []string
		for _, f := range workflowFindings(".github/workflows/x.yml", []byte(src)) {
			msgs = append(msgs, f.msg)
		}
		return strings.Join(msgs, "\n")
	}
	assert.NotContains(t, findings("${{ matrix.mise_sha256 }}"), "a literal", "the reference is the matrix's finding")
	assert.Contains(t, findings("${{ github.event.pull_request.title }}"), "a literal")
	assert.Contains(t, findings("a${{ matrix.mise_sha256 }}"), "a literal")
}

// TestWorkflowInputFromMatrixListValue checks that an expression inside
// a list value of the matrix (matrixValues) makes a reference to that
// key a finding, whatever else is found about the list.
func TestWorkflowInputFromMatrixListValue(t *testing.T) {
	src := strings.Replace(goodWorkflow, "      matrix:\n", "      matrix:\n        os: [ubuntu-26.04, \"${{ github.event.pull_request.title }}\"]\n", 1)
	src = strings.Replace(src, "sha256: ${{ matrix.mise_sha256 }}", "sha256: ${{ matrix.os }}", 1)
	var msgs []string
	for _, f := range workflowFindings(".github/workflows/x.yml", []byte(src)) {
		msgs = append(msgs, f.msg)
	}
	assert.Contains(t, strings.Join(msgs, "\n"), "this with input is a literal")
}

// TestWorkflowsMatrixRunnerNeedsOS fails runs-on: ${{ matrix.os }} when
// no matrix gives the job an os: no strategy at all, a strategy with no
// matrix, an include that is empty, and an include whose entry is not a
// mapping (the case "an include entry that is a word"), which also has
// the finding of the entry.
func TestWorkflowsMatrixRunnerNeedsOS(t *testing.T) {
	const needsOS = "so the matrix names os"
	tests := []struct {
		name  string
		pairs []string
		want  []string
	}{
		{"no strategy", []string{goodStrategy, ""}, []string{needsOS}},
		{"a strategy with no matrix", []string{goodStrategy, "    strategy:\n      fail-fast: false\n"}, []string{needsOS}},
		{"an empty include", []string{"include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "include: []\n"}, []string{needsOS}},
		{"an include entry that is a word", []string{"include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "include: [x]\n"}, []string{"an include entry is a mapping", needsOS}},
		{"an os key beside an empty include", []string{"include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "os: [ubuntu-26.04]\n        include: []\n"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A literal for the input keeps the matrix reference out of the findings.
			pairs := append([]string{"sha256: ${{ matrix.mise_sha256 }}", "sha256: aa"}, tt.pairs...)
			var msgs []string
			for _, f := range workflowFindings(".github/workflows/x.yml", []byte(edits(t, pairs...))) {
				msgs = append(msgs, f.msg)
			}
			require.Len(t, msgs, len(tt.want), "%v", msgs)
			for i, want := range tt.want {
				assert.Contains(t, msgs[i], want)
			}
		})
	}
}
