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
jobs:
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
            sum: aa
    steps:
      - name: Check out
        uses: actions/checkout@` + sha + `
        with:
          persist-credentials: false
      - uses: owner/repo/sub/path@` + sha + `
        with:
          sha256: ${{ matrix.sum }}
      - name: All checks
        run: go run ./tools/ci all
      - run: go run ./tools/release notes --tag "$TAG" --out=notes.md
        env:
          TAG: ${{ github.ref_name }}
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
		{"a key of the file", "permissions:\n  contents: read\nconcurrency", "env:\n  A: b\npermissions:\n  contents: read\nconcurrency", `"env" is outside the grammar of a file`},
		{"an event outside the list", "  workflow_dispatch:\n", "  pull_request_target:\n", `the event "pull_request_target"`},
		{"an expression with an operator", "${{ matrix.os }}\n    needs", "${{ matrix.os || 'ubuntu-26.04' }}\n    needs", "one context path"},
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
		{"a docker action", "uses: actions/checkout@" + sha, "uses: docker://alpine@sha256:" + sha + sha[:24], "40-hex commit SHA"},
		{"an env on a uses step", "          persist-credentials: false\n", "          persist-credentials: false\n        env:\n          A: b\n", `"env" is outside the grammar of a uses step`},
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
		{"a run line with a variable not quoted", `--tag "$TAG"`, "--tag $TAG", "run is one line"},
		{"a run line with a command substitution", `--tag "$TAG"`, `--tag "$(id)"`, "run is one line"},
		{"a runs-on with a latest label", "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-latest", "never *-latest"},
		{"a matrix with a latest label", "- os: ubuntu-26.04", "- os: macos-latest", "never *-latest"},
		{"a runs-on list with a latest label", "runs-on: ${{ matrix.os }}", "runs-on: [self-hosted, ubuntu-latest]", "never *-latest"},
		{"an anchor", "permissions:\n  contents: read\nconcurrency", "permissions: &p\n  contents: read\nconcurrency", "an anchor or an alias"},
		{"a merge key where keys are free", "          persist-credentials: false\n", "          persist-credentials: false\n          <<: {a: b}\n", `the tag "!!merge"`},
		{"a custom tag", "    timeout-minutes: 60\n", "    timeout-minutes: !custom 60\n", `the tag "!custom"`},
		{"a key written twice", "    timeout-minutes: 60\n", "    timeout-minutes: 60\n    timeout-minutes: 5\n", "written twice"},
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
		{"a ci.yml without on", ".github/workflows/ci.yml", "name: ci\njobs: {}\n", "edited"},
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
		assert.Empty(t, workflowFindings(".github/workflows/x.yml", []byte("on: "+on+"\njobs: {}\n")), "on: %s", on)
	}
	for _, on := range []string{"issues", "[push, issues]", "{issues: {}}"} {
		got := workflowFindings(".github/workflows/x.yml", []byte("on: "+on+"\njobs: {}\n"))
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
	assert.Equal(t, ".github/workflows/ci.yml:17", got[0].where)
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
