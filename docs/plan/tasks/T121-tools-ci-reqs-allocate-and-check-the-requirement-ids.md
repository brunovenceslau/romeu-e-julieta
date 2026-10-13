---
id: T121
title: "`tools/ci reqs`: allocate and check the requirement ids"
milestone: M0
module: ci-release
fate: new
owns:
  - tools/ci/reqs.go
  - tools/ci/reqs_test.go
  - tools/ci/main.go
  - tools/ci/fast.go
  - tools/ci/lintconfig_test.go
needs: []
after: []
ask_first:
  - checks
sitting: none
---

# T121 - `tools/ci reqs`: allocate and check the requirement ids

Goal: The check and the allocator of 12 12.11, in the idiom `tools/ci`
has today (a subcommand in `main.go`'s switch; T105 moves it to
`tools/ci/internal/reqs/` and `check_reqs.go` like every other check).
`go run ./tools/ci reqs allocate` replaces each `REQ-new` placeholder of
the working tree, in document order (`docs/spec.md`, then
`docs/spec/00-*.md` to `13-*.md`, top to bottom, left to right), with
the next number after the highest live or retired id, writes nothing
while an id or a retired row is a finding, and is idempotent on a tree
without placeholders. `go run ./tools/ci reqs`, in `fast` and `all`,
judges the tree at HEAD and fails a placeholder left, an id not matching
`REQ-[0-9]{4}`, a duplicate, an id both live and retired (the table of
12 12.11), a number below the highest absent from both, an id or a
placeholder placed against the placement rule of 12 12.11 (before a
list marker, inside a fenced block, inside another code span, in a
table header row), and an id inside an excluded page or section
(`docs/spec/13-*.md`, `docs/spec/09-*.md` from its first `## J` heading,
`docs/spec.md` from `## Open questions` to the end,
`docs/spec/00-*.md`'s `## 0.5` section, `docs/adr/`). The coverage half
over `docs/plan/tasks/*.md` lands here too, in the same two files: every
Implements entry a live id whose reading aid names the id's section,
every live id cited by a task of fate `kept`, `new`, `deferred` or
`merged`, the deferred ones reported apart, and no id cited by more
tasks than the cap of 12 12.11; its fixtures are task files written in
the same pass. The `main.go` and `fast.go` edits are the one-line seams
of rule 14; `lintconfig_test.go` gains the `romeu:invariant` directive
in its allowlist, so T007's tags need no M0 task before them.

Re-plan (2026-10-12, stage B): New. The tool was written with the pass
that placed the ids, so the specification and its check land together.

- Implements: REQ-0024 (index), REQ-0829 (10 10.2), REQ-1101 (12 12.2),
  REQ-1118 (12 12.3), REQ-1165 (12 12.5), REQ-1199 (12 12.11), REQ-1200
  (12 12.11), REQ-1201 (12 12.11), REQ-1202 (12 12.11).
- Operator: no
- Acceptance:
  - One test case per failure of the `reqs` row of 10 10.2, each titled
    by the failure: "a placeholder left in a page", "a token after the
    prefix that is neither four digits nor new", "an id that appears
    twice", "an id both live and retired", "a number below the highest
    that is neither live nor retired", "an id in chapter 13", "an id in
    a journey of chapter 09", "an id in the open questions of the
    index", "an id in section 0.5 of chapter 00", "an id in a decision
    record", "an id before a list marker", "an id inside a fenced
    block", "an id in a table header", "an id inside a code span", "a
    placeholder inside a retired row" and "a retired row that does not
    parse"; "a bare mention of the prefix is no finding", "an id after
    a list marker is well placed" and "a code span that wraps a line
    hides no id after it" pass; "a page that is a submodule entry is an
    error"; the baseline tree passes.
  - The coverage half has its cases: "an Implements entry whose id is
    retired or unknown", "an Implements entry with more digits than an
    id", "a reading aid that names another section", "a live id that
    no task cites", "an id cited by more tasks than the cap", "a shared
    id passes the cap", "a folded task is not read", "a fate outside
    the plan's set", "the coverage half is silent without task files",
    "a deferred-only id is reported and passes", "the deferred-only ids
    are listed eight to a line", "an Implements line that wraps is read
    whole", "a wrapped entry's finding names its line", "an untracked
    task file does not satisfy the coverage half" and "a task file
    without a fate is left to the plan check"; the tree passes.
  - `allocate` on a fixture with four placeholders across three files
    numbers them in document order from the highest live or retired id
    plus one, writes nothing else, and a second run changes no byte
    ("allocate numbers in document order from the highest live or
    retired id", "allocate is idempotent"); a placeholder in an excluded
    section, a misplaced placeholder, a retired row that does not parse
    and a number past `REQ-9999` each refuse with nothing written, the
    pages numbered before the refusal included; "a page with CR LF line
    ends keeps them".
  - `TestDisallowed` admits a `romeu:invariant` tag and refuses a
    misspelt one ("an invariant tag", "a misspelt invariant tag").
  - Two branches from one base that each allocate take the same numbers;
    the rebase-and-rerun rule of 12 12.3 covers it, and the check fails
    the duplicate ("the duplicate check catches a double allocation").
  - The retired table is read from 12 12.11 by its header row, and an id
    that is live and in the table fails.
  - `fast` and `all` judge `reqs` with `hygiene` and `workflows`, from
    the one read of the commit's tree, and print under the verdict the
    count of the ids that rest on deferred tasks alone.
- Verify: `go test ./tools/ci/... -run '^TestReqs'`; `go run ./tools/ci
  reqs` exits 0 on the tree.
