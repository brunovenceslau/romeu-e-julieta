---
id: T194
title: "tools/ci ask-first requires the Why heading of the pull request body"
milestone: M3
module: ci-bootstrap
fate: new
owns:
  - tools/ci/internal/askfirst/approval/sections*.go
needs: []
after: []
ask_first:
  - approvals
  - checks
sitting: none
---

# T194 - tools/ci ask-first requires the Why heading of the pull request body

Goal: the section check of REQ-1131, in the form amended on 2026-10-31:
the pull request body holds `## Why`, `## What changed` and
`## Evidence`, each with text under it, and `## Lessons` with text or
`none`.

Added on 2026-11-01 (ADR 0044, decided on 2026-10-31, scope this
repository): a slice of T005 (`tools/ci pr`, deferred to M6) brought
forward, since the changelog of T192 reads the Why of an entry from the
merge commit's body by that heading, and pull requests today write
"Why:" in prose (#361, measured on 2026-10-31: 13 of the last 40 merge
commits carry the heading). T005 keeps the rest of its list (the
Middleware line, the goldens under Evidence, the `Fixes-release:`
trailer). The pull request template already carries the four headings.

- Implements: REQ-1131 (12 12.4).
- Operator: no.
- Acceptance:
  - The ask-first job (`cmd/ask-first`, on the event payload) and the
    local form (`--title <title> --body <file>`) both fail a body that
    lacks one of the four level-2 headings, writes "Why:" in prose
    instead, or has an empty section, naming the heading and 12 12.4;
    a body with the four headings and text passes; `## Lessons` with
    `none` passes; a heading of another level or with other text after
    it does not count; a fixture per case.
  - The merge commit of this pull request carries `## Why`, so the
    first changelog section renders it (T192's golden names this pull
    request's form as its example).
- Verify: `go test ./tools/ci/internal/askfirst/approval/ -run
  '^TestSections' -v`, which lists the fixture cases by name; `go run
  ./tools/ci ask-first --title <this title> --body <this body>` exits 0.
