---
id: T181
title: "the base-defined pr job runs tools/ci pr on every pull request"
milestone: M2
module: ci-bootstrap
fate: new
owns:
  - .github/workflows/pr.yml
  - .github/workflows/ci.yml
  - tools/ci/check_pr.go
  - tools/ci/internal/askfirst/**
  - tools/ci/internal/workflows/**
  - .github/ask-first.yaml
  - docs/spec/12-engineering.md
needs:
  - T180
after: []
ask_first:
  - approvals
  - ask-first
  - checks
  - dependencies
sitting: none
---

# T181 - the base-defined pr job runs tools/ci pr on every pull request

Goal: the pr job of 12 12.4 runs the `pr` of T180 on every pull
request into `main`, from a workflow defined in the default branch, so
an approval line the diff needs and the maintainer has not ticked
fails the pull request. It is the second half of the first slice of
T005 (that file's re-plan of 2026-10-12), decided on 2026-10-29 with
T180, scope this repository.

1. `.github/workflows/pr.yml`: `on: pull_request_target` with
   `branches: [main]` and the types `opened, synchronize, reopened,
   edited`, `permissions: contents: read`, no secret and no
   `github.token`, one checkout of `github.sha` with
   `persist-credentials: false`, and its mise step with
   `persist_github_token: false`. The `edited` rule of `tools/ci
   workflows` moves from `ci.yml` to this file.
2. The job reads the head only as git objects (REQ-1152), prints the
   author's text behind a fixed prefix (REQ-1153), and refuses a head
   that could impersonate the job (REQ-1154, REQ-1155).
3. Invariant (b) does not hold today: `tools/ci` and the askfirst
   package import packages outside the `approvals` globs (the `check`
   package, and the YAML library, a module dependency). This task
   decides where the code `pr` builds lives: the job builds a `main`
   whose imports stay under `approvals` globs, with
   `.github/ask-first.yaml` gaining a glob when one is needed, and
   `go run ./tools/ci pr` stays the local command over the same code.
   REQ-0539 lists the `approvals` globs in 05 5.3, which T167 owns: a
   new glob makes this task re-planned with `after: T167` before its
   pull request opens. The local name stays `tools/ci pr`, so 05 is not
   otherwise edited. REQ-0541, invariant (d) and the 05 5.4 row "no
   check reads the approval line or a checkpoint grant" stay in T005
   (decided on 2026-10-29).

- Implements: REQ-0540 (05 5.3), REQ-0813 (10 10.2), REQ-0815 (10 10.2),
  REQ-0816 (10 10.2), REQ-1148 (12 12.4), REQ-1149 (12 12.4), REQ-1150
  (12 12.4), REQ-1151 (12 12.4), REQ-1152 (12 12.4), REQ-1153 (12 12.4),
  REQ-1154 (12 12.4), REQ-1155 (12 12.4).
- Operator: decide how invariant (b) counts a module dependency such
  as the YAML library (its pin is `go.sum`, on `dependencies`), before
  the pull request is ready. Before `pr.yml` is wired, open a throwaway pull request and
  measure whether ticking a checklist line in the GitHub UI fires
  `pull_request_target` `edited` and runs the job again; the result
  goes under Evidence and decides whether the tick needs another event.
  After the merge, add the pr job to the default-branch ruleset's
  required checks (REQ-1140) and record under the next pull request's
  Evidence the `head_sha` of its check run and whether the ruleset
  blocks and then allows a merge, or that it cannot be required. The
  pull request lists the open pull requests in its body; after the
  merge the maintainer edits each one's body, or closes and reopens it,
  so `pr` runs on it.
- Acceptance:
  - `tools/ci workflows` gains the pr job row of 10 10.2 with one
    fixture per refusal: `pull_request_target` in a second file, the
    event without `branches: [main]`, a permission other than
    `contents: read`, a step naming a secret or `github.token`, a
    checkout of a ref other than `github.sha`, one of
    `pull_request.base.sha`, a checkout without `persist-credentials:
    false`, the file without `edited`.
  - `pr` fetches `head.sha` without credentials only in the pr job and
    only when the object is missing; a fixture with the object present
    makes no network read.
  - The hostile fixture repository (a submodule, a textconv driver, an
    external diff driver and hooks) runs against each git call `pr`
    makes on head objects; removing any one flag of 12 12.4 from its
    call turns it red.
  - "pr escapes a workflow command in a PR body", with a leading
    space, a leading U+00A0 and a `##[` case: removing the fixed prefix
    or the `##[` escape turns it red. The leading-space and `##[` cases
    are measured once on a throwaway workflow run, and 12 12.4 drops
    "as remembered of the runner's code and not measured".
  - A head job outside the pr job's file whose check name equals the pr
    job's fails; one whose `name` holds `${{`, with a literal expression
    and with a matrix value, fails; a head version of the pr job's file
    that names an event other than `pull_request_target` fails; removing
    each comparison turns its fixture red.
  - Invariants (a), (b) and (c) of T005's acceptance hold, each with the
    fixtures written there.
- Verify: `go test ./tools/ci/... -run '^TestPR'` and `go run ./tools/ci
  workflows`. `pull_request_target` runs the default branch's file, so
  the job first runs on the pull request after this one, which links
  that run under its Evidence.
