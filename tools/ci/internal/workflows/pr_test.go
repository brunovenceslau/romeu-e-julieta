// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package workflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// goodPR passes the grammar: the pr job row of 10 10.2 as pr.yml holds
// it.
const goodPR = `name: pr
on:
  pull_request_target:
    branches: [main]
    types: [opened, synchronize, reopened, edited]
concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number }}
  cancel-in-progress: true
permissions:
  contents: read
` + citest.WorkflowEnv + `jobs:
  pr:
    name: pr
    runs-on: ubuntu-26.04-arm
    timeout-minutes: 10
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@` + citest.Sha + `
        with:
          persist-credentials: false
          fetch-depth: 0
      - uses: jdx/mise-action@` + citest.Sha + `
        with:
          version: 2026.10.3
          sha256: aa
          persist_github_token: false
      - run: go run ./tools/ci/internal/askfirst/cmd/pr "$GITHUB_EVENT_PATH"
`

const prPath = ".github/workflows/pr.yml"

// TestWorkflowsPRAdmitted passes the pr workflow, and the committed
// pr.yml is that shape (TestWorkflowsRepository reads it too).
func TestWorkflowsPRAdmitted(t *testing.T) {
	assert.Empty(t, workflowFindings(prPath, []byte(goodPR)))
	src, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(prPath)))
	require.NoError(t, err)
	assert.Empty(t, workflowFindings(prPath, src))
}

// TestWorkflowsPRRefused fails each change the pr job row refuses: one
// fixture per refusal of T181's acceptance (the event without
// branches: [main], a permission other than contents: read, a step
// naming a secret or github.token, a checkout of another ref, one of
// pull_request.base.sha, a checkout without persist-credentials: false,
// the file without edited), then the rest of the row.
func TestWorkflowsPRRefused(t *testing.T) {
	const checkoutWith = "          persist-credentials: false\n          fetch-depth: 0\n"
	tests := []struct {
		name     string
		old, new string
		want     string
	}{
		{"the event without branches", "    branches: [main]\n", "", "holds branches: [main]"},
		{"the event for another branch", "    branches: [main]\n", "    branches: [dev]\n", "holds branches: [main]"},
		{"the event for main and another branch", "    branches: [main]\n", "    branches: [main, dev]\n", "holds branches: [main]"},
		{"a permission other than contents: read, at the top", "permissions:\n  contents: read\nenv", "permissions:\n  contents: read\n  pull-requests: read\nenv", "the permissions of pr.yml is contents: read"},
		{"a write permission in the job", "    permissions:\n      contents: read\n", "    permissions:\n      contents: write\n", "the permissions of the job of pr.yml"},
		{"no permissions in the job", "    permissions:\n      contents: read\n", "", "the permissions of the job of pr.yml"},
		{"a step naming a secret", "          version: 2026.10.3\n", "          version: ${{ secrets.TOKEN }}\n", "a secret is read only"},
		{"a step naming github.token", "          version: 2026.10.3\n", "          version: ${{ github.token }}\n", "the token is read through no expression"},
		{"a checkout of the head", checkoutWith, checkoutWith + "          ref: ${{ github.event.pull_request.head.sha }}\n", "its checkout names no ref"},
		{"a checkout of pull_request.base.sha", checkoutWith, checkoutWith + "          ref: ${{ github.event.pull_request.base.sha }}\n", "never pull_request.base.sha"},
		{"a checkout of another repository", checkoutWith, checkoutWith + "          repository: someone/fork\n", "names no repository"},
		{"a checkout without persist-credentials: false", "          persist-credentials: false\n          fetch-depth: 0\n", "          fetch-depth: 0\n", "persist-credentials: false"},
		{"the file without edited", "    types: [opened, synchronize, reopened, edited]\n", "    types: [opened, synchronize, reopened]\n", "lists edited"},
		{"the file without types", "    types: [opened, synchronize, reopened, edited]\n", "", "lists edited"},
		{"a second event", "on:\n  pull_request_target:\n", "on:\n  push:\n  pull_request_target:\n", "runs on pull_request_target alone"},
		{"another event alone", "  pull_request_target:\n", "  pull_request:\n", "runs on pull_request_target alone"},
		{"another filter", "    branches: [main]\n", "    branches: [main]\n    paths: [docs/**]\n", "the filter paths"},
		{"a checkout of part of the history", "          fetch-depth: 0\n", "          fetch-depth: 1\n", "fetch-depth: 0"},
		{"a checkout of the default depth", "          fetch-depth: 0\n", "", "fetch-depth: 0"},
		{"a second job", "jobs:\n", "jobs:\n  other:\n    runs-on: ubuntu-26.04-arm\n    steps:\n      - run: go run ./tools/ci static\n", "holds one job"},
		{"the job under another id", "jobs:\n  pr:\n", "jobs:\n  judge:\n", "holds one job"},
		{"the job under another name", "    name: pr\n", "    name: judge\n", "is named pr"},
		{"a setup step, which builds all of tools/ci", "      - run: go run ./tools/ci/internal/askfirst/cmd/pr", "      - run: go run ./tools/ci setup\n      - run: go run ./tools/ci/internal/askfirst/cmd/pr", "the steps of the pr job"},
		{"the pr line with another argument", `cmd/pr "$GITHUB_EVENT_PATH"`, `cmd/pr "$HOME"`, "the steps of the pr job"},
		{"the steps in another order", "      - uses: jdx/mise-action@", "      - run: go run ./tools/ci/internal/askfirst/cmd/pr \"$GITHUB_EVENT_PATH\"\n      - uses: jdx/mise-action@", "the steps of the pr job"},
		{"a matrix", "    timeout-minutes: 10\n", "    timeout-minutes: 10\n    strategy:\n      matrix:\n        os: [ubuntu-26.04-arm]\n", "has no strategy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, goodPR, tt.old, "the fixture's text to replace")
			src := strings.Replace(goodPR, tt.old, tt.new, 1)
			wantAny(t, workflowFindings(prPath, []byte(src)), tt.want)
		})
	}
}

// TestWorkflowsPRElsewhere proves the exemptions of the row stay in the
// pr job of pr.yml: pull_request_target in a second file, the pr line
// and the fetch-depth input in any other file are refused.
func TestWorkflowsPRElsewhere(t *testing.T) {
	good := citest.GoodWorkflow
	tests := []struct {
		name, old, new, want string
	}{
		{"pull_request_target in a second file", "on:\n", "on:\n  pull_request_target:\n    branches: [main]\n", "is the event of pr.yml alone"},
		{"the pr line in another file", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci/internal/askfirst/cmd/pr \"$GITHUB_EVENT_PATH\"\n", "run is one line"},
		{"fetch-depth in another file", "          persist-credentials: false\n", "          persist-credentials: false\n          fetch-depth: 0\n", `the input "fetch-depth" is not one`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, good, tt.old, "the fixture's text to replace")
			src := strings.Replace(good, tt.old, tt.new, 1)
			wantAny(t, workflowFindings(".github/workflows/build.yml", []byte(src)), tt.want)
		})
	}
	t.Run("pull_request_target as one name", func(t *testing.T) {
		wantAny(t, workflowFindings(".github/workflows/x.yml", []byte("on: pull_request_target\njobs: {}\n"+citest.WorkflowEnv)), "is the event of pr.yml alone")
	})
	t.Run("the pr line in another job of pr.yml", func(t *testing.T) {
		src := strings.Replace(goodPR, "jobs:\n", "jobs:\n  other:\n    runs-on: ubuntu-26.04-arm\n    steps:\n      - run: go run ./tools/ci/internal/askfirst/cmd/pr \"$GITHUB_EVENT_PATH\"\n", 1)
		wantAny(t, workflowFindings(prPath, []byte(src)), "run is one line")
	})
	t.Run("a pr.yml with no job", func(t *testing.T) {
		head, _, _ := strings.Cut(goodPR, "jobs:\n")
		wantAny(t, workflowFindings(prPath, []byte(head+"jobs: {}\n")), "holds one job")
	})
}
