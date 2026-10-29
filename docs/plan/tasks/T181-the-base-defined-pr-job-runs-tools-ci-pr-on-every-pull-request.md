---
id: T181
title: "the base-defined pr job runs tools/ci pr on every pull request"
milestone: M2
module: ci-bootstrap
fate: merged
owns:
  - .github/workflows/pr.yml
  - .github/workflows/ci.yml
  - tools/ci/check_pr.go
  - tools/ci/internal/askfirst/**
  - tools/ci/internal/workflows/**
  - .github/ask-first.yaml
  - docs/spec/12-engineering.md
  - docs/spec/05-security.md
needs:
  - T180
after:
  - T167
  - T182
ask_first:
  - approvals
  - ask-first
  - checks
  - dependencies
  - spec
sitting: none
merged: 2026-10-29
pr: 335
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
   package import packages outside the `approvals` globs. Measured on
   2026-10-29 with `go list -deps`, the path of `pr` reaches, outside
   the standard library, `tools/ci/git` and `internal/procgroup` (both
   import only the standard library), `tools/ci/internal/check` (which
   brings `renameio`, and which the `pr` path can drop) and the YAML
   library, a module dependency. Decided by the operator on
   2026-10-29, scope this repository:
   - the job builds a `main` under `tools/ci/internal/askfirst/**`
     that does not import `check`, and `go run ./tools/ci pr` stays
     the local command over the same code;
   - the `approvals` surface gains `tools/ci/git/**` and
     `internal/procgroup/**`, so the one process starter stays the one
     `pr` uses, and a change to either needs the `approvals` line,
     which no grant covers;
   - a module dependency counts by its pin: `go.sum` joins the
     `approvals` globs, so a change to a dependency's code needs the
     `approvals` line too. `go.mod` stays off it: its `go` line stays
     a grantable input under REQ-0541, which T005 keeps, and a local
     `replace` in it is caught by the import test of invariant (b).
   These change REQ-0539 in 05 5.3, which T167 and T182 also own, hence
   `after: T167, T182`. That pull request also amends the sentence "Each path on
   `approvals` is also on `checks` or `dependencies`", which
   `internal/procgroup/**` breaks, and updates REQ-1150 in 12 12.4 with
   the command line of the step. The local name stays `tools/ci pr`.
   REQ-0541, invariant (d) and the 05 5.4 row "no check reads the
   approval line or a checkpoint grant" stay in T005 (decided on
   2026-10-29).

- Implements: REQ-0540 (05 5.3), REQ-0813 (10 10.2), REQ-0815 (10 10.2),
  REQ-0816 (10 10.2), REQ-1148 (12 12.4), REQ-1149 (12 12.4), REQ-1150
  (12 12.4), REQ-1151 (12 12.4), REQ-1152 (12 12.4), REQ-1153 (12 12.4),
  REQ-1154 (12 12.4), REQ-1155 (12 12.4), REQ-1314 (12 12.4), the
  stack guard of pr, landed after this task on 2026-10-29 with the pr
  job running on every pull request, whatever its base.
- Operator: `pr.yml` is wired with `edited`, and whether ticking a
  checklist line in the GitHub UI fires `pull_request_target` `edited`
  and runs the job again is measured on the first pull request after
  this one, since the event runs only a file already on the default
  branch (decided on 2026-10-29, instead of a probe workflow merged
  first); the result goes under that pull request's Evidence, and a
  tick that fires nothing reopens the job's events. After the merge, add the pr job to the default-branch ruleset's
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
