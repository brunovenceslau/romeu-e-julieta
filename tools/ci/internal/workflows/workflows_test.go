// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package workflows

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/container"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// TestWorkflowsGrammar fails a fixture for each construct outside the
// grammar of 10 10.2, and passes the one that holds every construct
// inside it. Each fixture changes one line of citest.GoodWorkflow.
func TestWorkflowsGrammar(t *testing.T) {
	tests := []struct {
		name     string
		old, new string // the text of goodWorkflow to replace, and its replacement
		want     string // a part of the one finding expected; "" for none
	}{
		{"the good workflow", "", "", ""},
		{"an if on a step", "      - name: All checks\n", "      - name: All checks\n        if: always()\n", `"if" is outside the grammar of a run step`},
		{"a shell", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        shell: bash\n", `"shell" is outside the grammar of a run step`},
		{"continue-on-error", "      - name: All checks\n", "      - name: All checks\n        continue-on-error: true\n", `"continue-on-error"`},
		{"a container", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    container: alpine\n", `"container" is outside the grammar of a job`},
		{"a job that calls a workflow", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    uses: owner/repo/.github/workflows/x.yml@" + citest.Sha + "\n", `"uses" is outside the grammar of a job`},
		{"a key of the file", "permissions:\n  contents: read\nconcurrency", "defaults:\n  run:\n    shell: bash\npermissions:\n  contents: read\nconcurrency", `"defaults" is outside the grammar of a file`},
		{"no env at the top", citest.WorkflowEnv, "", "env holds exactly the mise environment"},
		{"an env with another key: BASH_ENV", "  MISE_AUTO_ENV: \"false\"\n", "  MISE_AUTO_ENV: \"false\"\n  BASH_ENV: /tmp/x\n", "env holds exactly the mise environment"},
		{"an env with another value", "MISE_OVERRIDE_CONFIG_FILENAMES: mise.toml", "MISE_OVERRIDE_CONFIG_FILENAMES: mise.toml,mise.local.toml", "env holds exactly the mise environment"},
		{"an env without one variable", "  MISE_ENV: \"\"\n", "", "env holds exactly the mise environment"},
		{"an env with a null MISE_ENV", `MISE_ENV: ""`, "MISE_ENV:", "env holds exactly the mise environment"},
		{"an env with a boolean", `MISE_AUTO_ENV: "false"`, "MISE_AUTO_ENV: false", "env holds exactly the mise environment"},
		{"an env in another order", "  MISE_ENV: \"\"\n  MISE_AUTO_ENV: \"false\"\n", "  MISE_AUTO_ENV: \"false\"\n  MISE_ENV: \"\"\n", "env holds exactly the mise environment"},
		{"an env that is a list", citest.WorkflowEnv, "env: [a]\n", "env holds exactly the mise environment"},
		{"an event outside the list", "  workflow_dispatch:\n", "  pull_request_target:\n", `the event "pull_request_target"`},
		{"an expression with an operator", "group: ci-${{ github.ref }}", "group: ci-${{ github.ref || 'main' }}", "one context path"},
		{"an expression with a function call", "group: ci-${{ github.ref }}", "group: ci-${{ format('{0}', github.ref) }}", "one context path"},
		{"an expression with a literal", "group: ci-${{ github.ref }}", "group: ci-${{ true }}", "one context path"},
		{"an expression with a number", "group: ci-${{ github.ref }}", "group: ci-${{ 1 }}", "one context path"},
		{"an expression with an index", "group: ci-${{ github.ref }}", "group: ci-${{ github['ref'] }}", "one context path"},
		{"an expression not closed", "group: ci-${{ github.ref }}", "group: ci-${{ github.ref", "not closed"},
		{"an expression in a run line", "run: go run ./tools/ci all\n", "run: go run ./tools/ci all ${{ github.ref }}\n", "run is one line"},
		{"a uses with a tag", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout@v6", "40-hex commit SHA"},
		{"a uses with a short SHA", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout@" + citest.Sha[:39], "40-hex commit SHA"},
		{"a uses with an upper-case SHA", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout@" + strings.ToUpper(citest.Sha), "40-hex commit SHA"},
		{"a local action", "uses: actions/checkout@" + citest.Sha, "uses: ./.github/actions/x", "40-hex commit SHA"},
		{"a local action with a SHA", "uses: actions/checkout@" + citest.Sha, "uses: ./.github/actions/x@" + citest.Sha, "40-hex commit SHA"},
		{"a parent directory with a SHA", "uses: actions/checkout@" + citest.Sha, "uses: ../x@" + citest.Sha, "40-hex commit SHA"},
		{"two parent directories with a SHA", "uses: actions/checkout@" + citest.Sha, "uses: ../..@" + citest.Sha, "40-hex commit SHA"},
		{"a dot path segment", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout/../x@" + citest.Sha, "40-hex commit SHA"},
		{"a docker action", "uses: actions/checkout@" + citest.Sha, "uses: docker://alpine@sha256:" + citest.Sha + citest.Sha[:24], "40-hex commit SHA"},
		{"an env on a uses step", "          persist-credentials: false\n", "          persist-credentials: false\n        env:\n          A: b\n", `"env" is outside the grammar of a uses step`},
		{"an env on a run step: BASH_ENV", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          BASH_ENV: /tmp/x\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: an exported function", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          BASH_FUNC_go%%: \"() { id; }\"\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: GOFLAGS", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          GOFLAGS: -toolexec=/tmp/x\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: LD_PRELOAD", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          LD_PRELOAD: /tmp/x.so\n", `"env" is outside the grammar of a run step`},
		{"an env on a run step: PATH", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          PATH: /tmp\n", `"env" is outside the grammar of a run step`},
		{"an env on a job", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    env:\n      BASH_ENV: /tmp/x\n", `"env" is outside the grammar of a job`},
		{"an input checkout does not take here", "          persist-credentials: false\n", "          persist-credentials: false\n          fetch-depth: 0\n", `the input "fetch-depth" is not one this repository uses for actions/checkout`},
		{"an input mise-action does not take here", "          sha256: ${{ matrix.mise_sha256 }}\n", "          sha256: ${{ matrix.mise_sha256 }}\n          install_args: x\n", `the input "install_args" is not one this repository uses for jdx/mise-action`},
		{"a mise step without persist_github_token", "          persist_github_token: false\n", "", "every jdx/mise-action step writes persist_github_token: false"},
		{"a mise step that persists the token", "persist_github_token: false", "persist_github_token: true", "every jdx/mise-action step writes persist_github_token: false"},
		{"a mise step that persists the token, quoted", "persist_github_token: false", `persist_github_token: "false"`, "every jdx/mise-action step writes persist_github_token: false"},
		{"a mise step whose with is not a mapping", "        with:\n          sha256: ${{ matrix.mise_sha256 }}\n          persist_github_token: false\n", "        with: x\n", "with is a mapping"},
		{"a mise step without with", "        with:\n          sha256: ${{ matrix.mise_sha256 }}\n          persist_github_token: false\n", "", "every jdx/mise-action step writes persist_github_token: false"},
		{"an input actions/cache does not take here", "          key: cache-aa\n", "          key: cache-aa\n          restore-keys: cache-\n", `the input "restore-keys" is not one this repository uses for actions/cache`},
		{"an input actions/cache does not take, in another case", "uses: actions/cache@", "uses: Actions/Cache@", ""},
		{"a concurrency key outside the grammar", "  cancel-in-progress: true\n", "  cancel-in-progress: true\n  queue: max\n", `the concurrency key "queue" is outside the grammar: group and cancel-in-progress`},
		{"a concurrency that is a name", "concurrency:\n  group: ci-${{ github.ref }}\n  cancel-in-progress: true\n", "concurrency: ci\n", "concurrency is a mapping of group and cancel-in-progress"},
		{"a concurrency that cancels by an expression", "cancel-in-progress: true", "cancel-in-progress: ${{ github.event.x }}", "cancel-in-progress is true, false or"},
		{"a concurrency that cancels for pull requests alone", "cancel-in-progress: true", "cancel-in-progress: ${{ github.event_name == 'pull_request' }}", ""},
		{"a concurrency that cancels for pushes alone", "cancel-in-progress: true", "cancel-in-progress: ${{ github.event_name == 'push' }}", "one context path"},
		{"the expression of pull requests in the group", "group: ci-${{ github.ref }}", "group: ci-${{ github.event_name == 'pull_request' }}", "one context path"},
		{"the expression of pull requests in a name", "      - name: All checks\n", "      - name: ${{ github.event_name == 'pull_request' }}\n", "one context path"},
		{"a cache keyed by the runner", "key: cache-aa", "key: cache-${{ runner.os }}-${{ runner.arch }}-aa", ""},
		{"a cache keyed by the ref", "key: cache-aa", "key: cache-${{ github.ref }}-aa", "this with input is a literal"},
		{"a cache path of the runner", "path: /tmp/cache", "path: /tmp/${{ runner.arch }}", "this with input is a literal"},
		{"the runner in another action's input", "sha256: ${{ matrix.mise_sha256 }}", "sha256: x-${{ runner.arch }}", "this with input is a literal"},
		{"the build of tools/ci", "run: go run ./tools/ci all\n", "run: go build -o \"$RUNNER_TEMP/ci\" ./tools/ci\n", ""},
		{"the built tools/ci", "run: go run ./tools/ci all\n", "run: '\"$RUNNER_TEMP/ci\" all'\n", ""},
		{"a build of another program", "run: go run ./tools/ci all\n", "run: go build -o \"$RUNNER_TEMP/ci\" ./tools/release\n", "run is one line"},
		{"a build to another path", "run: go run ./tools/ci all\n", "run: go build -o /tmp/ci ./tools/ci\n", "run is one line"},
		{"a build with a flag", "run: go run ./tools/ci all\n", "run: go build -toolexec=x -o \"$RUNNER_TEMP/ci\" ./tools/ci\n", "run is one line"},
		{"another built program", "run: go run ./tools/ci all\n", "run: '\"$RUNNER_TEMP/x\" all'\n", "run is one line"},
		{"the built tools/ci with no subcommand", "run: go run ./tools/ci all\n", "run: '\"$RUNNER_TEMP/ci\"'\n", "run is one line"},
		{"the built tools/ci piped", "run: go run ./tools/ci all\n", "run: '\"$RUNNER_TEMP/ci\" all | sh'\n", "run is one line"},
		{"a concurrency that does not cancel", "cancel-in-progress: true", "cancel-in-progress: false", ""},
		{"a concurrency without a group", "  group: ci-${{ github.ref }}\n", "", "concurrency names its group"},
		{"an input of an action not listed", "uses: jdx/mise-action/sub/path@" + citest.Sha + "\n        with:\n          sha256: ${{ matrix.mise_sha256 }}\n          persist_github_token: false\n", "uses: owner/repo/sub/path@" + citest.Sha + "\n        with:\n          sha256: ${{ matrix.mise_sha256 }}\n", `the input "sha256" is not one this repository uses for owner/repo`},
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
		{"a uses with a path that starts with a dot", "uses: actions/checkout@" + citest.Sha, "uses: ./x@sha", "40-hex commit SHA"},
		{"a local action with a real SHA", "uses: actions/checkout@" + citest.Sha, "uses: ./x@" + citest.Sha, "40-hex commit SHA"},
		{"a uses that is a root path", "uses: actions/checkout@" + citest.Sha, "uses: /x@" + citest.Sha, "40-hex commit SHA"},
		{"a uses with a parent as repository", "uses: actions/checkout@" + citest.Sha, "uses: owner/..@" + citest.Sha, "40-hex commit SHA"},
		{"a uses with an empty path segment", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout/@" + citest.Sha, "40-hex commit SHA"},
		{"a uses with a 41-hex ref", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout@" + citest.Sha + "a", "40-hex commit SHA"},
		{"a comment after the SHA", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout@" + citest.Sha + " # v6", ""},
		{"a uses with a word after the SHA", "uses: actions/checkout@" + citest.Sha, "uses: \"actions/checkout@" + citest.Sha + " x\"", "40-hex commit SHA"},
		{"a runs-on with a larger macos image", "runs-on: ${{ matrix.os }}", "runs-on: macos-26-xlarge", "never *-latest"},
		{"a runs-on with a windows image", "runs-on: ${{ matrix.os }}", "runs-on: windows-2025", "never *-latest"},
		{"a runs-on with a prefix before the name", "runs-on: ${{ matrix.os }}", "runs-on: x-ubuntu-22.04", "never *-latest"},
		{"an anchor", "permissions:\n  contents: read\nconcurrency", "permissions: &p\n  contents: read\nconcurrency", "an anchor or an alias"},
		{"a merge key where keys are free", "    branches: [main]\n", "    branches: [main]\n    <<: {a: b}\n", `the tag "!!merge"`},
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
		{"a checkout in capitals without persist-credentials", "        uses: actions/checkout@" + citest.Sha + "\n        with:\n          persist-credentials: false\n", "        uses: Actions/CheckOut@" + citest.Sha + "\n", "persist-credentials: false"},
		{"a checkout in capitals with persist-credentials", "uses: actions/checkout@" + citest.Sha, "uses: Actions/CheckOut@" + citest.Sha, ""},
		{"a path segment that starts with an underscore", "uses: jdx/mise-action/sub/path@", "uses: jdx/mise-action/_sub/path@", ""},
		{"an owner that starts with an underscore", "uses: actions/checkout@" + citest.Sha, "uses: _x/checkout@" + citest.Sha, "40-hex commit SHA"},
		{"a repository that starts with an underscore", "uses: actions/checkout@" + citest.Sha, "uses: actions/_checkout@" + citest.Sha, "40-hex commit SHA"},
		{"a strategy key outside the grammar", "      fail-fast: false\n", "      fail-fast: false\n      other: 1\n", `the strategy key "other"`},
		{"a second strategy key outside the grammar", "      fail-fast: false\n", "      fail-fast: false\n      name: x\n", `the strategy key "name"`},
		{"a strategy with max-parallel", "      fail-fast: false\n", "      fail-fast: false\n      max-parallel: 2\n", ""},
		{"a uses path segment of a single underscore", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout/_x@" + citest.Sha, ""},
		{"a uses path segment that starts with two dots", "uses: actions/checkout@" + citest.Sha, "uses: actions/checkout/..x@" + citest.Sha, "40-hex commit SHA"},
		{"a job key written quoted", "    timeout-minutes: 60\n", "    \"continue-on-error\": true\n", `"continue-on-error" is outside the grammar of a job`},
		{"a quoted key that is allowed", "    timeout-minutes: 60\n", "    \"timeout-minutes\": 60\n", ""},
		{"a strategy in flow style", "    strategy:\n      fail-fast: false\n      matrix:\n        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "    strategy: {fail-fast: false, matrix: {include: [{os: ubuntu-26.04, mise_sha256: aa}]}}\n", ""},
		{"a strategy in flow style with a latest label", "    strategy:\n      fail-fast: false\n      matrix:\n        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n", "    strategy: {matrix: {include: [{os: ubuntu-latest, mise_sha256: aa}]}}\n", "never *-latest"},
		{"a step in flow style", "      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n", "      - {run: \"go run ./tools/ci all\", if: true}\n", `"if" is outside the grammar of a run step`},
		{"a key with a tag that is not a string", "    timeout-minutes: 60\n", "    !!int timeout-minutes: 60\n", "key is written as a plain string"},
		{"a key with the string tag", "    timeout-minutes: 60\n", "    !!str timeout-minutes: 60\n", ""},
		{"a strategy with a bad first key", "    strategy:\n      fail-fast: false\n", "    strategy:\n      other: 1\n      fail-fast: false\n", `the strategy key "other"`},
		{"steps in flow style", "    steps:\n      - name: Check out\n        uses: actions/checkout@" + citest.Sha + "\n        with:\n          persist-credentials: false\n      - uses: jdx/mise-action/sub/path@" + citest.Sha + "\n        with:\n          sha256: ${{ matrix.mise_sha256 }}\n          persist_github_token: false\n      - uses: actions/cache@" + citest.Sha + "\n        with:\n          path: /tmp/cache\n          key: cache-aa\n      - name: All checks\n        run: go run ./tools/ci all\n      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n", "    steps: [{uses: actions/checkout@" + citest.Sha + ", with: {persist-credentials: false}}, {uses: jdx/mise-action/sub/path@" + citest.Sha + ", with: {sha256: \"${{ matrix.mise_sha256 }}\", persist_github_token: false}}, {run: \"go run ./tools/ci all\"}]\n", ""},
		{"steps in flow style with a key outside the grammar", "    steps:\n      - name: Check out\n        uses: actions/checkout@" + citest.Sha + "\n        with:\n          persist-credentials: false\n      - uses: jdx/mise-action/sub/path@" + citest.Sha + "\n        with:\n          sha256: ${{ matrix.mise_sha256 }}\n          persist_github_token: false\n      - uses: actions/cache@" + citest.Sha + "\n        with:\n          path: /tmp/cache\n          key: cache-aa\n      - name: All checks\n        run: go run ./tools/ci all\n      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n", "    steps: [{uses: actions/checkout@" + citest.Sha + ", with: {persist-credentials: false}}, {uses: jdx/mise-action/sub/path@" + citest.Sha + ", with: {sha256: \"${{ matrix.mise_sha256 }}\", persist_github_token: false}}, {run: \"go run ./tools/ci all\", if: true}]\n", `"if" is outside the grammar of a run step`},
		{"an include entry key with a tag that is not a string", "          - os: ubuntu-26.04\n", "          - !!int os: ubuntu-26.04\n", "key is written as a plain string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := citest.GoodWorkflow
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
			assert.Contains(t, got[0].Msg, tt.want)
			assert.Equal(t, "workflows", got[0].Rule)
			assert.True(t, strings.HasPrefix(got[0].Where, ".github/workflows/x.yml:"), "the finding names the file and a line: %s", got[0].Where)
		})
	}
}

// TestWorkflowsStepEnvRefusesEventText holds the env of a step, whose
// values reach tools/ci and tools/release as arguments, to the text of
// the event: refused in every file, on a plain run step and next to the
// token of a release step, while the group of the concurrency keeps the
// event expression it admits today.
func TestWorkflowsStepEnvRefusesEventText(t *testing.T) {
	const run = "        run: go run ./tools/ci all\n"
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/x64.yml", ".github/workflows/other.yml"} {
		for _, value := range []string{"${{ github.event.pull_request.title }}", "${{ github.event.head_commit.message }}", "${{ GitHub.Event.pull_request.head.ref }}"} {
			src := strings.Replace(citest.GoodWorkflow, run, run+"        env:\n          TITLE: "+value+"\n", 1)
			wantAny(t, workflowFindings(path, []byte(src)), `"env" is outside the grammar of a run step`)
		}
	}
	env := "        env:\n          GH_TOKEN: ${{ github.token }}\n          TITLE: ${{ github.event.pull_request.title }}\n"
	src := strings.Replace(goodRelease, "      - name: Notes\n", "      - run: go run ./tools/release notes\n"+env+"      - name: Notes\n", 1)
	wantAny(t, workflowFindings(releasePath, []byte(src)), "the env of a release step")
	grouped := strings.Replace(citest.GoodWorkflow, "group: ci-${{ github.ref }}", "group: ci-${{ github.event.head_commit.id }}", 1)
	assert.Empty(t, workflowFindings(".github/workflows/other.yml", []byte(grouped)), "the group keeps its event expression")
}

// TestWorkflowsFile fails what the grammar refuses as a whole file: a
// name with another ending, text that is not one YAML mapping, and a
// ci.yml that a change to a pull request's text does not run again.
func TestWorkflowsFile(t *testing.T) {
	tests := []struct {
		name, path, src, want string
	}{
		{"a yaml ending", ".github/workflows/ci.yaml", citest.GoodWorkflow, "ends in .yml"},
		{"no ending", ".github/workflows/README", "text", "ends in .yml"},
		{"a file in a subdirectory", ".github/workflows/old/ci.json", "{}", "ends in .yml"},
		{"not YAML", ".github/workflows/x.yml", "a: [b\n", "not YAML"},
		{"two documents", ".github/workflows/x.yml", citest.GoodWorkflow + "---\nname: two\n", "one YAML document"},
		{"a list", ".github/workflows/x.yml", "- a\n", "a mapping"},
		{"empty", ".github/workflows/x.yml", "", "not YAML"},
		{"a ci.yml without edited", ".github/workflows/ci.yml", strings.Replace(citest.GoodWorkflow, ", edited]", "]", 1), "edited"},
		{"a ci.yml without pull_request", ".github/workflows/ci.yml", strings.Replace(citest.GoodWorkflow, "  pull_request:\n    types: [opened, synchronize, reopened, edited]\n", "", 1), "edited"},
		{"a ci.yml without on", ".github/workflows/ci.yml", "name: ci\njobs: {}\n" + citest.WorkflowEnv, "edited"},
		{"a ci.yml whose on is one name", ".github/workflows/ci.yml", "on: pull_request\njobs: {}\n" + citest.WorkflowEnv, "edited"},
		{"a ci.yml whose on is a list", ".github/workflows/ci.yml", "on: [pull_request, push]\njobs: {}\n" + citest.WorkflowEnv, "edited"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withoutHosted(workflowFindings(tt.path, []byte(tt.src)))
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].Msg, tt.want)
		})
	}
	t.Run("another file without edited", func(t *testing.T) {
		src := strings.Replace(citest.GoodWorkflow, ", edited]", "]", 1)
		assert.Empty(t, workflowFindings(".github/workflows/fuzz.yml", []byte(src)))
	})
	t.Run("a ci.yml with edited", func(t *testing.T) {
		assert.Empty(t, withoutHosted(workflowFindings(".github/workflows/ci.yml", []byte(citest.GoodWorkflow))))
	})
}

// withoutHosted drops the findings of the rules that ci.yml and x64.yml
// alone are held to (hosted), for the tests that use the one-job fixture
// under the name ci.yml to reach a rule of the file.
func withoutHosted(findings []check.Finding) []check.Finding {
	var out []check.Finding
	for _, f := range findings {
		if !strings.Contains(f.Msg, "the jobs of this file") && !strings.Contains(f.Msg, "with its own subcommand") &&
			!strings.Contains(f.Msg, "the matrix of the job") && !strings.Contains(f.Msg, "aggregators") {
			out = append(out, f)
		}
	}
	return out
}

// TestWorkflowsEvents reads the forms YAML gives "on": one name, a
// list of names, and a mapping.
func TestWorkflowsEvents(t *testing.T) {
	for _, on := range []string{"push", "[push, workflow_dispatch]", "{push: {branches: [main]}}"} {
		assert.Empty(t, workflowFindings(".github/workflows/x.yml", []byte("on: "+on+"\njobs: {}\n"+citest.WorkflowEnv)), "on: %s", on)
	}
	for _, on := range []string{"issues", "[push, issues]", "{issues: {}}"} {
		got := workflowFindings(".github/workflows/x.yml", []byte("on: "+on+"\njobs: {}\n"+citest.WorkflowEnv))
		require.Len(t, got, 1, "on: %s: %v", on, got)
		assert.Contains(t, got[0].Msg, `the event "issues"`)
	}
}

// TestWorkflowsCommitted runs the check the way fast does, on the tree
// of a fixture repository: it reads the committed workflows, and only
// those under .github/workflows.
func TestWorkflowsCommitted(t *testing.T) {
	r := citest.NewTree(t)
	r.Write(".github/workflows/build.yml", citest.GoodWorkflow)
	r.Write("docs/example.yml", "jobs: {x: {if: true}}\n")
	r.Commit("fixture")
	got, err := workflows(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	assert.Empty(t, got)

	r.Write(".github/workflows/build.yml", strings.Replace(citest.GoodWorkflow, "${{ matrix.os }}\n    needs", "ubuntu-latest\n    needs", 1))
	got, err = workflows(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	assert.Empty(t, got, "a change that is not committed is in no push")
	r.Commit("a floating label")
	got, err = workflows(t.Context(), r.Repo, "HEAD")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, ".github/workflows/build.yml:29", got[0].Where)
}

// TestWorkflowsRepository checks the workflows of this repository, as
// they are in the working tree.
func TestWorkflowsRepository(t *testing.T) {
	root := citest.ModuleRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(Dir)))
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Dir), e.Name()))
		require.NoError(t, err)
		assert.Empty(t, workflowFindings(Dir+e.Name(), src), "%s", e.Name())
	}
}

// TestWorkflowsCIFile reads the repository's ci.yml and x64.yml as
// data and holds them to what T002 and T157 ask of them: the pinned
// runners, arm64 on every pull request and x64 on a schedule (10 10.2,
// Runners), the jobs static, race and e2e, each with mise installed by a
// pinned version and a sha256 per platform before setup, and setup before
// the subcommand of the job, and the two aggregators that every other job
// feeds. Past the header, the trigger and the matrix, the two files are
// the same, so the scheduled jobs are the jobs a pull request runs.
func TestWorkflowsCIFile(t *testing.T) {
	read := func(name string) []byte {
		src, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(Dir), name))
		require.NoError(t, err)
		return src
	}
	for name, want := range map[string]struct {
		events      []string
		runners     []string // the linux label, then the macOS one
		concurrency map[string]any
	}{
		// x64.yml runs no event a pull request waits for. ci.yml cancels
		// the run in progress of a pull request alone; a push has a
		// group of its own commit, so no push replaces another.
		"ci.yml": {[]string{"pull_request", "push"}, []string{"ubuntu-26.04-arm", "macos-26"},
			map[string]any{"group": "${{ github.workflow }}-${{ github.ref }}-${{ github.event.head_commit.id }}", "cancel-in-progress": "${{ github.event_name == 'pull_request' }}"}},
		// x64.yml cancels nothing, and its cron and a dispatch are two
		// groups.
		"x64.yml": {[]string{"schedule", "workflow_dispatch"}, []string{"ubuntu-26.04", "macos-26-intel"},
			map[string]any{"group": "${{ github.workflow }}-${{ github.event_name }}-${{ github.ref }}", "cancel-in-progress": false}},
	} {
		t.Run(name, func(t *testing.T) { checkCIJobs(t, read(name), want.events, want.runners, want.concurrency) })
	}
	assert.Equal(t, jobBody(t, read("ci.yml")), jobBody(t, read("x64.yml")), "the jobs of x64.yml are the jobs of ci.yml")
}

// jobBody returns the lines of a workflow from "permissions:" on, without the
// matrix entries, the aggregators' runners and the comments: what ci.yml
// and x64.yml share.
func jobBody(t *testing.T, src []byte) []string {
	t.Helper()
	_, body, found := strings.Cut(string(src), "\npermissions:\n")
	require.True(t, found, "a permissions table")
	var lines []string
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- os: ") || strings.HasPrefix(trimmed, "mise_sha256: ") ||
			strings.HasPrefix(trimmed, "name: all on ") || (strings.HasPrefix(trimmed, "runs-on: ") && !strings.Contains(trimmed, "matrix.os")) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// cacheKey returns the key of the cache of the container e2e, from the
// fixture mise.lock and the pins of this repository (container.CacheKey).
func cacheKey(t *testing.T) string {
	t.Helper()
	root := citest.ModuleRoot(t)
	lock, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(container.FixtureLock)))
	require.NoError(t, err)
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pins.Path)))
	require.NoError(t, err)
	p, err := pins.Parse(src)
	require.NoError(t, err)
	return container.CacheKey(lock, p)
}

// checkCIJobs holds a workflow to the events it runs on, its three jobs
// to the runners they run on (static and e2e on the Linux label alone,
// race on both), to the mise, setup, build and subcommand steps, and its
// two aggregators to the names of the required checks and to the jobs
// they need.
func checkCIJobs(t *testing.T, src []byte, events, runners []string, concurrency map[string]any) {
	t.Helper()
	type job struct {
		Name     string   `yaml:"name"`
		RunsOn   string   `yaml:"runs-on"`
		Needs    []string `yaml:"needs"`
		If       string   `yaml:"if"`
		Timeout  int      `yaml:"timeout-minutes"`
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
	}
	var wf struct {
		On          map[string]yaml.Node `yaml:"on"`
		Concurrency map[string]any       `yaml:"concurrency"`
		Jobs        map[string]job       `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(src, &wf))
	assert.Equal(t, concurrency, wf.Concurrency, "the concurrency of the file")
	got := slices.Sorted(maps.Keys(wf.On))
	assert.Equal(t, slices.Sorted(slices.Values(events)), got, "the events")
	linux, mac := runners[0], runners[1]
	assert.ElementsMatch(t, []string{"static", "race", "e2e", "all_linux", "all_macos"}, slices.Collect(maps.Keys(wf.Jobs)), "the jobs")

	labelsOf := map[string][]string{"static": {linux}, "race": {linux, mac}, "e2e": {linux}}
	for id, wantLabels := range labelsOf {
		j := wf.Jobs[id]
		assert.Equal(t, id+" on ${{ matrix.os }}", j.Name)
		assert.Equal(t, matrixRunner, j.RunsOn)
		assert.Positive(t, j.Timeout, "the job %s has a timeout", id)
		var labels []string
		for _, entry := range j.Strategy.Matrix.Include {
			labels = append(labels, entry["os"])
			assert.Regexp(t, `^[0-9a-f]{64}$`, entry["mise_sha256"], "the mise sha256 of %s in %s", entry["os"], id)
		}
		assert.Equal(t, wantLabels, labels, "the runners of %s", id)

		index := map[string]int{}
		for i, s := range j.Steps {
			switch {
			case strings.HasPrefix(s.Uses, "jdx/mise-action@"):
				index["mise"] = i
				assert.Equal(t, "2026.10.3", s.With["version"])
				assert.Equal(t, "${{ matrix.mise_sha256 }}", s.With["sha256"])
				assert.Equal(t, false, s.With["cache"])
				assert.Equal(t, false, s.With["env"])
				assert.Equal(t, false, s.With["persist_github_token"])
			case strings.HasPrefix(s.Uses, "actions/cache@"):
				index["cache"] = i
				assert.Equal(t, map[string]any{"path": container.CacheDir, "key": cacheKey(t)}, s.With, "the cache of the container e2e")
			case s.Run == "go run ./tools/ci setup":
				index["setup"] = i
			case s.Run == BuildLine:
				index["build"] = i
			case s.Run == BuiltCI+" "+id:
				index["run"] = i
			}
		}
		if id == "e2e" {
			require.Len(t, index, 5, "the mise, cache, setup, build and subcommand steps of %s: %v", id, index)
			assert.Less(t, index["mise"], index["cache"])
			assert.Less(t, index["cache"], index["setup"])
		} else {
			require.Len(t, index, 4, "the mise, setup, build and subcommand steps of %s, and no cache of the container e2e: %v", id, index)
		}
		assert.Less(t, index["mise"], index["setup"])
		assert.Less(t, index["setup"], index["build"])
		assert.Less(t, index["build"], index["run"], "%s runs the built tools/ci, so its exit status reaches the job", id)
	}

	aggregators := map[string]string{"all_linux": "all on " + linux, "all_macos": "all on " + mac}
	for id, name := range aggregators {
		j := wf.Jobs[id]
		assert.Equal(t, name, j.Name, "the aggregator %s is a required check by its name", id)
		assert.Equal(t, linux, j.RunsOn, "the aggregator %s runs on the Linux label of its file", id)
		assert.ElementsMatch(t, []string{"static", "race", "e2e"}, j.Needs, "%s needs every other job", id)
		assert.Equal(t, AggregatorIf, j.If)
		assert.Equal(t, 5, j.Timeout, "an aggregator has a short timeout")
		assert.Empty(t, j.Strategy.Matrix.Include, "an aggregator is not a matrix")
		require.Len(t, j.Steps, 1)
		assert.Equal(t, AggregatorStep, j.Steps[0].Run)
	}
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
			got := workflowFindings(".github/workflows/x.yml", []byte(citest.Edits(t, tt.pairs...)))
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].Msg, tt.want)
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
		{"jobs as a list", []string{citest.GoodWorkflow[strings.Index(citest.GoodWorkflow, "jobs:"):], "jobs: []\n"}, "jobs is a mapping"},
		{"steps as a word", []string{citest.GoodWorkflow[strings.Index(citest.GoodWorkflow, "    steps:"):], "    steps: foo\n"}, "steps is a list"},
		{"a strategy as a list", []string{goodStrategy, "    strategy: [a]\n", onRunner, "runs-on: ubuntu-26.04", "sha256: ${{ matrix.mise_sha256 }}", "sha256: aa"}, "strategy is a mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workflowFindings(".github/workflows/x.yml", []byte(citest.Edits(t, tt.pairs...)))
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].Msg, tt.want)
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
			got := workflowFindings(".github/workflows/x.yml", []byte(citest.Edits(t, tt.pairs...)+tt.extra))
			if tt.want == "" {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].Msg, tt.want)
		})
	}
}

// TestWorkflowInputWithUnreadableMatrix checks that a matrix that cannot
// be read excuses only a ${{ matrix.<key> }} reference, whose finding is
// the matrix's own: any other expression in a with value is still found.
func TestWorkflowInputWithUnreadableMatrix(t *testing.T) {
	unreadable := strings.Replace(citest.GoodWorkflow,
		"        include:\n          - os: ubuntu-26.04\n            mise_sha256: aa\n",
		"        include: ${{ github.event.inputs.m }}\n", 1)
	require.NotEqual(t, citest.GoodWorkflow, unreadable)
	findings := func(with string) string {
		src := strings.Replace(unreadable, "sha256: ${{ matrix.mise_sha256 }}", "sha256: "+with, 1)
		var msgs []string
		for _, f := range workflowFindings(".github/workflows/x.yml", []byte(src)) {
			msgs = append(msgs, f.Msg)
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
	src := strings.Replace(citest.GoodWorkflow, "      matrix:\n", "      matrix:\n        os: [ubuntu-26.04, \"${{ github.event.pull_request.title }}\"]\n", 1)
	src = strings.Replace(src, "sha256: ${{ matrix.mise_sha256 }}", "sha256: ${{ matrix.os }}", 1)
	var msgs []string
	for _, f := range workflowFindings(".github/workflows/x.yml", []byte(src)) {
		msgs = append(msgs, f.Msg)
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
			for _, f := range workflowFindings(".github/workflows/x.yml", []byte(citest.Edits(t, pairs...))) {
				msgs = append(msgs, f.Msg)
			}
			require.Len(t, msgs, len(tt.want), "%v", msgs)
			for i, want := range tt.want {
				assert.Contains(t, msgs[i], want)
			}
		})
	}
}

// TestWorkflowsCommand runs "tools/ci workflows" on a fixture
// repository: exit 0 with a workflow inside the grammar, 1 outside it.
func TestWorkflowsCommand(t *testing.T) {
	r := citest.NewTree(t)
	r.Write(".github/workflows/build.yml", citest.GoodWorkflow)
	r.Commit("fixture")
	code, out := citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitOK, code, out)
	assert.Equal(t, "workflows: ok\n", out)

	r.Write(".github/workflows/extra.yaml", citest.GoodWorkflow)
	r.Commit("a second ending")
	code, out = citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitFail, code, out)
	assert.Equal(t, ".github/workflows/extra.yaml: workflows: a file here ends in .yml\nworkflows: 1 finding(s)\n", out)
}

// aggregators are the two jobs that end the fixture, in the shape the
// grammar admits (AggregatorIf): they need the one job of GoodWorkflow,
// run on a literal label, and have one step that fails.
const aggregators = `  aggregate_linux:
    name: all on ubuntu-26.04-arm
    runs-on: ubuntu-26.04-arm
    needs: [all]
    if: ` + AggregatorIf + `
    timeout-minutes: 5
    steps:
      - run: exit 1
  aggregate_macos:
    name: all on macos-26
    runs-on: ubuntu-26.04-arm
    needs:
      - all
    if: ` + AggregatorIf + `
    timeout-minutes: 5
    steps:
      - name: Fail
        run: exit 1
`

// TestWorkflowsAggregator holds the aggregator job of 10 10.2: the fixture
// with two of them passes, and each fixture that changes one part of one
// of them fails with the finding for that part alone, so each rule has a
// positive control.
func TestWorkflowsAggregator(t *testing.T) {
	good := citest.GoodWorkflow + aggregators
	assert.Empty(t, workflowFindings(".github/workflows/x.yml", []byte(good)), "the fixture with two aggregators")
	assert.Equal(t, "${{ always() && (contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled') || contains(needs.*.result, 'skipped')) }}", AggregatorIf)

	tests := []struct {
		name     string
		old, new string // the text of the fixture to replace, once, and its replacement
		want     string
		// several is set for a fixture that breaks two rules at once, each
		// with a finding of its own: the one named is among them.
		several bool
	}{
		{"an if that differs by one character: result", "'failure'", "'failures'", "holds this if, byte for byte", false},
		{"an if that differs by one character: && for ||", "&& (contains(needs.*.result, 'failure')", "|| (contains(needs.*.result, 'failure')", "holds this if, byte for byte", false},
		{"an if without always()", "${{ always() && (", "${{ (", "holds this if, byte for byte", false},
		{"an if that is a plain expression", "if: " + AggregatorIf + "\n    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "if: ${{ failure() }}\n    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "holds this if, byte for byte", false},
		{"an if that is a boolean", "if: " + AggregatorIf + "\n    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "if: true\n    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "holds this if, byte for byte", false},
		{"an if with a trailing space, quoted", "if: " + AggregatorIf + "\n    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "if: \"" + AggregatorIf + " \"\n    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "holds this if, byte for byte", false},
		{"an aggregator with no timeout", "    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "    steps:\n      - run: exit 1\n  aggregate_macos", "has timeout-minutes: 5", false},
		{"an aggregator with a longer timeout", "    timeout-minutes: 5\n    steps:\n      - run: exit 1\n  aggregate_macos", "    timeout-minutes: 360\n    steps:\n      - run: exit 1\n  aggregate_macos", "has timeout-minutes: 5", false},
		{"an aggregator whose needs omits a job", "needs: [all]", "needs: []", `missing "all"`, false},
		{"an aggregator with no needs", "    needs: [all]\n", "", `missing "all"`, false},
		{"an aggregator whose needs names a job that is not there", "needs: [all]", "needs: [all, other]", `unknown "other"`, false},
		{"an aggregator that needs the other aggregator", "needs: [all]", "needs: [all, aggregate_macos]", `unknown "aggregate_macos"`, false},
		{"an aggregator with a matrix", "    runs-on: ubuntu-26.04-arm\n    needs: [all]\n", "    runs-on: ubuntu-26.04-arm\n    needs: [all]\n    strategy:\n      matrix:\n        include:\n          - os: ubuntu-26.04-arm\n            mise_sha256: aa\n", "has no strategy", false},
		{"an aggregator whose name holds an expression", "name: all on ubuntu-26.04-arm", "name: all on ${{ matrix.os }}", "has a literal name", false},
		{"an aggregator whose runs-on is the matrix", "    runs-on: ubuntu-26.04-arm\n    needs: [all]\n", "    runs-on: ${{ matrix.os }}\n    needs: [all]\n", "runs on one literal runner label", true},
		{"an aggregator with no runs-on", "    runs-on: ubuntu-26.04-arm\n    needs: [all]\n", "    needs: [all]\n", "runs on one literal runner label", false},
		{"an aggregator on a floating label", "    runs-on: ubuntu-26.04-arm\n    needs: [all]\n", "    runs-on: ubuntu-latest\n    needs: [all]\n", "never *-latest", false},
		{"an aggregator with another command", "      - run: exit 1\n  aggregate_macos", "      - run: go run ./tools/ci all\n  aggregate_macos", "has one step, run: exit 1", false},
		{"an aggregator with exit 0", "      - run: exit 1\n  aggregate_macos", "      - run: exit 0\n  aggregate_macos", "run is one line", true},
		{"an aggregator with a second step", "      - run: exit 1\n  aggregate_macos", "      - run: exit 1\n      - run: exit 1\n  aggregate_macos", "has one step, run: exit 1", true},
		{"an aggregator with a uses step", "      - run: exit 1\n  aggregate_macos", "      - uses: actions/checkout@" + citest.Sha + "\n        with:\n          persist-credentials: false\n  aggregate_macos", "has one step, run: exit 1", false},
		{"an aggregator with no steps", "    steps:\n      - run: exit 1\n  aggregate_macos", "  aggregate_macos", "has one step, run: exit 1", false},
		{"an aggregator with exit 1 and a condition on the step", "      - run: exit 1\n  aggregate_macos", "      - run: exit 1\n        if: always()\n  aggregate_macos", `"if" is outside the grammar of a run step`, false},
		{"an aggregator with a shell on the step", "      - run: exit 1\n  aggregate_macos", "      - run: exit 1\n        shell: bash\n  aggregate_macos", `"shell" is outside the grammar of a run step`, false},
		{"an aggregator with an env", "    needs: [all]\n", "    needs: [all]\n    env:\n      A: b\n", `"env" is outside the grammar of a job`, false},
		{"an aggregator with continue-on-error", "    needs: [all]\n", "    needs: [all]\n    continue-on-error: true\n", `"continue-on-error" is outside the grammar of a job`, false},
		{"an if written twice", "    needs: [all]\n", "    needs: [all]\n    if: " + AggregatorIf + "\n", "written twice", true},
		{"exit 1 in a job with no if", "        run: go run ./tools/ci all\n", "        run: exit 1\n", "run is one line", false},
		{"exit 1 as the step of a job with no if, beside others", "      - name: All checks\n        run: go run ./tools/ci all\n", "      - name: All checks\n        run: go run ./tools/ci all\n      - run: exit 1\n", "run is one line", false},
		{"exit 1 in a job whose if is not an aggregator's but that has more steps", "      - run: exit 1\n  aggregate_macos", "      - run: exit 1\n      - run: exit 1\n  aggregate_macos", "run is one line", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := good
			require.Contains(t, src, tt.old, "the fixture's text to replace")
			src = strings.Replace(src, tt.old, tt.new, 1)
			got := workflowFindings(".github/workflows/x.yml", []byte(src))
			if tt.several {
				wantAny(t, got, tt.want)
				return
			}
			require.Len(t, got, 1, "%v", got)
			assert.Contains(t, got[0].Msg, tt.want)
			assert.Equal(t, "workflows", got[0].Rule)
		})
	}
}

// TestWorkflowsIfOnAnotherJob holds that an if is the aggregator's alone:
// on a job that builds, with a matrix and many steps, even the exact text
// fails, for what such a job is not, and so does any other text.
func TestWorkflowsIfOnAnotherJob(t *testing.T) {
	for _, value := range []string{"true", "always()", "${{ github.event_name == 'push' }}", AggregatorIf} {
		src := strings.Replace(citest.GoodWorkflow, "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    if: "+value+"\n", 1)
		got := workflowFindings(".github/workflows/x.yml", []byte(src))
		wantAny(t, got, "has a literal name")
		wantAny(t, got, "has no strategy")
		wantAny(t, got, "has one step, run: exit 1")
		if value != AggregatorIf {
			wantAny(t, got, "holds this if, byte for byte")
		}
	}
	src := strings.Replace(citest.GoodWorkflow, "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    \"if\": true\n", 1)
	wantAny(t, workflowFindings(".github/workflows/x.yml", []byte(src)), "an aggregator job (the only job with an if)")
	// A step's if stays outside the grammar, and so does an if expression
	// elsewhere than on a job.
	src = strings.Replace(citest.GoodWorkflow, "      - name: All checks\n", "      - name: All checks\n        if: ${{ always() }}\n", 1)
	wantAny(t, workflowFindings(".github/workflows/x.yml", []byte(src)), `"if" is outside the grammar of a run step`)
	src = strings.Replace(citest.GoodWorkflow, "group: ci-${{ github.ref }}", "group: ci-"+AggregatorIf, 1)
	wantAny(t, workflowFindings(".github/workflows/x.yml", []byte(src)), "one context path")
}

// TestWorkflowsHostedFilesKeepTheirJobs holds ci.yml and x64.yml to the
// jobs of 10 10.2, whatever the rest of the grammar admits: besides the
// aggregators the jobs are exactly check.Jobs, each runs its own
// subcommand of the built tools/ci, the race job's matrix holds both the
// Linux and the macOS label of the file and the other two the Linux
// label alone, and there are exactly two aggregators, named for those
// labels. Each fixture changes the real file in one way, so it is a
// positive control for one rule; a job removed from the file and from
// both needs lists passes every other rule of the grammar.
func TestWorkflowsHostedFilesKeepTheirJobs(t *testing.T) {
	files := map[string]struct{ linux, mac string }{
		"ci.yml":  {"ubuntu-26.04-arm", "macos-26"},
		"x64.yml": {"ubuntu-26.04", "macos-26-intel"},
	}
	cut := func(t *testing.T, src, from, to string) string {
		t.Helper()
		i, j := strings.Index(src, from), strings.Index(src, to)
		require.True(t, i >= 0 && j > i, "%q to %q", from, to)
		return src[:i] + src[j:]
	}
	for name, f := range files {
		raw, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(Dir), name))
		require.NoError(t, err)
		good := string(raw)
		assert.Empty(t, workflowFindings(Dir+name, raw), "%s as it is", name)
		macEntry := regexp.MustCompile(`(?m)^          - os: ` + regexp.QuoteMeta(f.mac) + `\n            mise_sha256: [0-9a-f]{64}\n`)
		linuxEntry := regexp.MustCompile(`(?m)^          - os: ` + regexp.QuoteMeta(f.linux) + `\n            mise_sha256: ([0-9a-f]{64})\n`)
		const run = `run: '"$RUNNER_TEMP/ci" `
		tests := []struct {
			name   string
			mutate func(t *testing.T) string
			want   string
		}{
			{"the race job deleted and removed from both needs", func(t *testing.T) string {
				return strings.ReplaceAll(cut(t, good, "  # The one test run", "  # The container e2e"), "needs: [static, race, e2e]", "needs: [static, e2e]")
			}, "the jobs of this file are static, race, e2e besides the aggregators, missing race"},
			{"the static job renamed", func(*testing.T) string {
				return strings.Replace(strings.ReplaceAll(good, "needs: [static, race, e2e]", "needs: [lint, race, e2e]"), "\n  static:\n", "\n  lint:\n", 1)
			}, `missing static, unknown lint`},
			{"a fourth job", func(*testing.T) string {
				return strings.ReplaceAll(good, "needs: [static, race, e2e]", "needs: [static, race, e2e, extra]") +
					"\n  extra:\n    name: extra\n    runs-on: " + f.linux + "\n    timeout-minutes: 5\n    steps:\n      - run: go run ./tools/ci hygiene\n"
			}, `unknown extra`},
			{"the race job running the static subcommand", func(*testing.T) string {
				return strings.Replace(good, run+`race'`, run+`static'`, 1)
			}, "the job race runs the built tools/ci with its own subcommand"},
			{"the static job building tools/ci and running nothing", func(*testing.T) string {
				return strings.Replace(good, "        "+run+`static'`, "        run: "+BuildLine, 1)
			}, "the job static runs the built tools/ci with its own subcommand"},
			{"the e2e job running all", func(*testing.T) string {
				return strings.Replace(good, run+`e2e'`, run+`all'`, 1)
			}, "the job e2e runs the built tools/ci with its own subcommand"},
			{"the macOS aggregator removed", func(t *testing.T) string {
				return good[:strings.Index(good, "  all_macos:\n")]
			}, "exactly two aggregators"},
			{"a third aggregator", func(*testing.T) string {
				i := strings.Index(good, "  all_linux:\n")
				block := good[i:strings.Index(good, "  all_macos:\n")]
				return good + "\n" + strings.Replace(strings.Replace(block, "all_linux", "all_third", 1), "all on "+f.linux, "all on third", 1)
			}, "exactly two aggregators"},
			{"an aggregator named for the other file's label", func(*testing.T) string {
				return strings.Replace(good, "name: all on "+f.linux+"\n", "name: all on ubuntu-latest\n", 1)
			}, `the aggregators are named "all on ` + f.linux + `" and "all on ` + f.mac + `"`},
			{"the race matrix without the macOS label", func(*testing.T) string {
				require.Len(t, macEntry.FindAllString(good, -1), 1)
				return macEntry.ReplaceAllString(good, "")
			}, "the matrix of the job race holds " + f.linux + " and " + f.mac},
			{"the e2e matrix with the macOS label", func(*testing.T) string {
				i := strings.Index(good, "  e2e:\n")
				tail := linuxEntry.ReplaceAllString(good[i:], "${0}          - os: "+f.mac+"\n            mise_sha256: "+strings.Repeat("a", 64)+"\n")
				return good[:i] + tail
			}, "the matrix of the job e2e holds " + f.linux + " alone"},
		}
		for _, tt := range tests {
			t.Run(name+": "+tt.name, func(t *testing.T) {
				wantAny(t, workflowFindings(Dir+name, []byte(tt.mutate(t))), tt.want)
			})
		}
		t.Run(name+": the race job without its macOS leg is a valid file for any other file", func(t *testing.T) {
			// The same text in a file of another name is outside these rules.
			assert.Empty(t, workflowFindings(Dir+"other.yml", []byte(strings.ReplaceAll(macEntry.ReplaceAllString(good, ""), "name: all on "+f.linux, "name: all on third"))))
		})
	}
}
