// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package workflows

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// goodRelease passes the grammar: a test job, and the publish job of
// 10 10.2 with every step the row admits.
const goodRelease = `name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: read
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false
` + citest.WorkflowEnv + `jobs:
  all:
    runs-on: ${{ matrix.os }}
    timeout-minutes: 60
    strategy:
      fail-fast: false
      matrix:
        include:
          - os: ubuntu-26.04
            mise_sha256: aa
    steps:
      - uses: actions/checkout@` + citest.Sha + `
        with:
          persist-credentials: false
      - run: go run ./tools/ci setup
      - run: go run ./tools/ci all
  publish:
    runs-on: ubuntu-26.04
    needs: [all]
    timeout-minutes: 60
    permissions:
      contents: write
      id-token: write
      attestations: write
      actions: read
      pull-requests: read
      issues: read
    steps:
      - uses: actions/checkout@` + citest.Sha + `
        with:
          persist-credentials: false
      - uses: jdx/mise-action@` + citest.Sha + `
        with:
          version: 2026.10.3
          sha256: aa
          persist_github_token: false
      - name: Setup
        run: go run ./tools/ci setup
      - name: Build
        run: go run ./tools/release build
        env:
          GH_TOKEN: ${{ github.token }}
      - uses: actions/attest-build-provenance@` + citest.Sha + `
        with:
          subject-path: dist/*
      - name: Verify
        run: go run ./tools/release verify
        env:
          GH_TOKEN: ${{ github.token }}
      - name: Notes
        run: go run ./tools/release notes --tag "$GITHUB_REF_NAME" --out=notes.md
        env:
          GH_TOKEN: ${{ github.token }}
      - name: Publish
        run: go run ./tools/release publish --tag "$GITHUB_REF_NAME"
        env:
          GH_TOKEN: ${{ github.token }}
`

const releasePath = ".github/workflows/release.yml"

// TestWorkflowsReleaseAdmitted passes the release workflow that holds
// every admitted step of the publish job.
func TestWorkflowsReleaseAdmitted(t *testing.T) {
	assert.Empty(t, workflowFindings(releasePath, []byte(goodRelease)))
}

// TestWorkflowsReleaseStepEnv passes the env of the row on each of the
// four tools/release steps, and refuses it on every other step of the
// job.
func TestWorkflowsReleaseStepEnv(t *testing.T) {
	env := "        env:\n          GH_TOKEN: ${{ github.token }}\n"
	for _, sub := range []string{"build", "notes", "verify", "publish"} {
		t.Run("admitted "+sub, func(t *testing.T) {
			src := strings.Replace(goodRelease, "      - name: Notes\n", "      - run: go run ./tools/release "+sub+"\n"+env+"      - name: Notes\n", 1)
			assert.Empty(t, workflowFindings(releasePath, []byte(src)))
		})
	}
	for name, step := range map[string]string{
		"refused on tools/ci setup":            "      - run: go run ./tools/ci setup\n" + env,
		"refused on an unknown tools/release":  "      - run: go run ./tools/release other\n" + env,
		"refused on a checkout":                "      - uses: actions/checkout@" + citest.Sha + "\n        with:\n          persist-credentials: false\n" + env,
		"refused on the attest step":           "      - uses: actions/attest-build-provenance@" + citest.Sha + "\n        with:\n          subject-path: dist/*\n" + env,
		"refused on a step with a bad command": "      - run: go run ./tools/release build $(id)\n" + env,
	} {
		t.Run(name, func(t *testing.T) {
			src := strings.Replace(goodRelease, "      - name: Notes\n", step+"      - name: Notes\n", 1)
			wantAny(t, workflowFindings(releasePath, []byte(src)), "outside the grammar of a")
		})
	}
}

// TestWorkflowsReleaseRefused fails each change to the publish job.
func TestWorkflowsReleaseRefused(t *testing.T) {
	tests := []struct {
		name     string
		old, new string
		want     string
	}{
		{"a second env key", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GH_TOKEN: ${{ github.token }}\n          GOPROXY: \"off\"\n      - uses: actions/attest", "the env of a release step"},
		{"GOPROXY alone", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GOPROXY: \"off\"\n      - uses: actions/attest", "the env of a release step"},
		{"an event value in env", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GH_TOKEN: ${{ github.event.pull_request.title }}\n      - uses: actions/attest", "the env of a release step"},
		{"another name for the token", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GITHUB_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "the env of a release step"},
		{"a secret as the token", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GH_TOKEN: ${{ secrets.X }}\n      - uses: actions/attest", "the env of a release step"},
		{"the token with an operator", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GH_TOKEN: ${{ github.token || 'x' }}\n      - uses: actions/attest", "the env of a release step"},
		{"a literal in the token", "          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "          GH_TOKEN: x${{ github.token }}\n      - uses: actions/attest", "the env of a release step"},
		{"an env that is a list", "        env:\n          GH_TOKEN: ${{ github.token }}\n      - uses: actions/attest", "        env: [GH_TOKEN]\n      - uses: actions/attest", "the env of a release step"},
		{"the job on another runner", "    runs-on: ubuntu-26.04\n    needs: [all]", "    runs-on: macos-26\n    needs: [all]", "runs on ubuntu-26.04"},
		{"the job on a latest runner", "    runs-on: ubuntu-26.04\n    needs: [all]", "    runs-on: ubuntu-latest\n    needs: [all]", "never *-latest"},
		{"the job with no needs", "    needs: [all]\n    timeout-minutes: 60\n    permissions:\n      contents: write", "    timeout-minutes: 60\n    permissions:\n      contents: write", "needs every other job"},
		{"the job that needs another job", "needs: [all]\n    timeout-minutes: 60\n    permissions:\n      contents: write", "needs: [all, publish]\n    timeout-minutes: 60\n    permissions:\n      contents: write", "needs every other job"},
		{"a write-all permission", "    permissions:\n      contents: write\n", "    permissions: write-all\n    xpermissions:\n      contents: write\n", "permissions is a mapping"},
		{"a write of another scope", "      actions: read\n", "      actions: write\n", "a permission of the publish job"},
		{"a write of packages", "      actions: read\n", "      packages: write\n", "a permission of the publish job"},
		{"a tools/ci all step", "      - name: Build\n", "      - run: go run ./tools/ci all\n      - name: Build\n", "no test"},
		{"a built tools/ci", "      - name: Build\n", "      - run: go build -o \"$RUNNER_TEMP/ci\" ./tools/ci\n      - name: Build\n", "no test"},
		{"a second setup", "      - name: Build\n", "      - run: go run ./tools/ci setup\n      - name: Build\n", "once"},
		{"no setup", "      - name: Setup\n        run: go run ./tools/ci setup\n", "", "once"},
		{"setup after a release step", "      - name: Setup\n        run: go run ./tools/ci setup\n      - name: Build\n        run: go run ./tools/release build\n        env:\n          GH_TOKEN: ${{ github.token }}\n", "      - name: Build\n        run: go run ./tools/release build\n        env:\n          GH_TOKEN: ${{ github.token }}\n      - name: Setup\n        run: go run ./tools/ci setup\n", "before any tools/release"},
		{"a step key outside the grammar", "      - name: Verify\n", "      - if: always()\n        name: Verify\n", `"if" is outside the grammar of a run step`},
		{"the token as an input", "          subject-path: dist/*\n", "          subject-path: dist/*\n          github-token: ${{ github.token }}\n", "not one this repository uses"},
		{"an attest input outside the list", "          subject-path: dist/*\n", "          subject-path: dist/*\n          push-to-registry: true\n", "not one this repository uses"},
		{"an attest input from the event", "subject-path: dist/*", "subject-path: ${{ github.event.head_commit.message }}", "a literal"},
		{"the attest action by a tag", "actions/attest-build-provenance@" + citest.Sha, "actions/attest-build-provenance@v4", "40-hex commit SHA"},
		{"a mise step that keeps the token", "          persist_github_token: false\n      - name: Setup", "          persist_github_token: true\n      - name: Setup", "persist_github_token: false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, goodRelease, tt.old, "the fixture's text to replace")
			src := strings.Replace(goodRelease, tt.old, tt.new, 1)
			wantAny(t, workflowFindings(releasePath, []byte(src)), tt.want)
		})
	}
}

// TestWorkflowsReleaseNoPublishJob fails a release.yml with no publish
// job, and one whose only job is not named so.
func TestWorkflowsReleaseNoPublishJob(t *testing.T) {
	src := strings.Replace(goodRelease, "  publish:\n", "  ship:\n", 1)
	wantAny(t, workflowFindings(releasePath, []byte(src)), "holds a job named publish")
}

// TestWorkflowsReleaseElsewhere proves the exemptions of the row stay in
// the publish job of release.yml: the same text is refused in any other
// file, and in any other job of release.yml.
func TestWorkflowsReleaseElsewhere(t *testing.T) {
	good := citest.GoodWorkflow
	const job = "    permissions:\n      contents: read\n"
	step := "      - run: go run ./tools/release notes --tag \"$GITHUB_REF_NAME\" --out=notes.md\n"
	tests := []struct {
		name     string
		old, new string
		want     string
	}{
		{"contents write in a job of ci.yml", job, "    permissions:\n      contents: write\n", "never a write"},
		{"id-token write in a job of ci.yml", job, "    permissions:\n      id-token: write\n", "never a write"},
		{"attestations write in a job of ci.yml", job, "    permissions:\n      attestations: write\n", "never a write"},
		{"the env of the row on a release step of ci.yml", step, step + "        env:\n          GH_TOKEN: ${{ github.token }}\n", `"env" is outside the grammar of a run step`},
		{"an env on a step of ci.yml that is not a release step", "        run: go run ./tools/ci all\n", "        run: go run ./tools/ci all\n        env:\n          GH_TOKEN: ${{ github.token }}\n", `"env" is outside the grammar of a run step`},
		{"an event value in the env of a step of ci.yml", step, step + "        env:\n          X: ${{ github.event.pull_request.title }}\n", `"env" is outside the grammar of a run step`},
		{"the token in a group of ci.yml", "group: ci-${{ github.ref }}", "group: ci-${{ github.token }}", "the token"},
		{"the attest action in ci.yml", step, "      - uses: actions/attest-build-provenance@" + citest.Sha + "\n        with:\n          subject-path: dist/*\n" + step, "publish job of release.yml only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Contains(t, good, tt.old, "the fixture's text to replace")
			src := strings.Replace(good, tt.old, tt.new, 1)
			wantAny(t, workflowFindings(".github/workflows/ci.yml", []byte(src)), tt.want)
		})
	}
	t.Run("the release.yml text in another file", func(t *testing.T) {
		got := workflowFindings(".github/workflows/other.yml", []byte(goodRelease))
		wantAny(t, got, "never a write")
		wantAny(t, got, `"env" is outside the grammar of a run step`)
		wantAny(t, got, "the token")
		wantAny(t, got, "publish job of release.yml only")
	})
	t.Run("the token in a test job of release.yml", func(t *testing.T) {
		src := strings.Replace(goodRelease, "      - run: go run ./tools/ci all\n", "      - run: go run ./tools/ci all\n        env:\n          GH_TOKEN: ${{ github.token }}\n", 1)
		wantAny(t, workflowFindings(releasePath, []byte(src)), `"env" is outside the grammar of a run step`)
	})
	t.Run("a write in a test job of release.yml", func(t *testing.T) {
		src := strings.Replace(goodRelease, "    runs-on: ${{ matrix.os }}\n", "    runs-on: ${{ matrix.os }}\n    permissions:\n      contents: write\n", 1)
		wantAny(t, workflowFindings(releasePath, []byte(src)), "never a write")
	})
	t.Run("the attest action in a test job of release.yml", func(t *testing.T) {
		src := strings.Replace(goodRelease, "      - run: go run ./tools/ci all\n", "      - run: go run ./tools/ci all\n      - uses: actions/attest-build-provenance@"+citest.Sha+"\n", 1)
		wantAny(t, workflowFindings(releasePath, []byte(src)), "publish job of release.yml only")
	})
}

// wantAny fails unless some finding names want.
func wantAny(t *testing.T, got []check.Finding, want string) {
	t.Helper()
	for _, f := range got {
		if strings.Contains(f.Msg, want) {
			return
		}
	}
	t.Errorf("no finding names %q: %v", want, got)
}
