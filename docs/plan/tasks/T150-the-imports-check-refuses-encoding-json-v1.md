---
id: T150
title: "The imports check refuses encoding/json v1"
milestone: M1
module: none
fate: new
owns:
  - tools/ci/internal/imports/imports.go
  - tools/ci/internal/imports/imports_test.go
needs:
  - T148
  - T149
after: []
ask_first:
  - checks
sitting: none
---

# T150 - The imports check refuses encoding/json v1

Goal: keep `encoding/json/v2` the default once T148 and T149 have moved
every package to it. The imports check of `tools/ci` (REQ-0839) gains a
rule: a package whose non-test or test files import `encoding/json`,
the v1 package, is a finding that cites REQ-0960. The allowlist is
empty, decided on 2026-10-20, and test files are covered too. The check
reads the import lists it already loads from `go list -json`, for every
target of `lint.Targets` and with the tags of the tagged suites, so a
file of the e2e or host suite is judged too.

- Implements: REQ-0160 (03 (opening)), REQ-0960 (10 10.6).
- Operator: no.
- Acceptance:
  - `go run ./tools/ci imports` passes on the tree after T148 and T149.
  - A test plants `encoding/json` in a package's imports, and in its
    test imports, and each is a finding that names the package and
    REQ-0960; `encoding/json/v2` and `encoding/json/jsontext` are no
    finding.
  - A positive control: with the rule removed, the planted cases pass,
    and the test fails.
- Verify: `go test ./tools/ci/internal/imports/ && go run ./tools/ci imports`.
