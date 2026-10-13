---
id: T007
title: "`termsafe` and the CLI writer"
milestone: M1
module: termsafe
fate: kept
owns:
  - internal/termsafe/**
  - internal/cli/writer.go
needs: []
after:
  - T105
ask_first:
  - termsafe
sitting: none
---

# T007 - `termsafe` and the CLI writer

Earlier title: `termsafe`, the CLI writer and the invariant tooling.

Goal: The escape table of I29 with its fuzz seeds, and the CLI writer of
`internal/cli/writer.go`.

Re-plan (2026-10-12): Kept, narrowed. The `invariant` kind of
`tools/new`, `tools/ci invariants` and `mutate` leave this task for T133
(deferred, see Decisions this plan defers in `docs/plan.md`), which
carries the three acceptance rows written here before the re-plan; the
guard and test tags are written as comments so the check finds them when
it lands. `after: T105`: the `romeu:invariant` directive joins the
allowlist of the `generate` package of `tools/ci` in T105's lint split,
before this task's first tag.

- Implements: REQ-0011 (index), REQ-0017 (index), REQ-0020 (index),
  REQ-0050 (01 1.2), REQ-0053 (01 1.3), REQ-0056 (01 1.3), REQ-0141 (02
  2.3), REQ-0145 (02 2.3), REQ-0269 (04 4.1), REQ-0472 (05 5.2),
  REQ-0473 (05 5.2), REQ-0474 (05 5.2), REQ-0477 (05 5.2), REQ-0480 (05
  5.2), REQ-0518 (05 5.2), REQ-0536 (05 5.3), REQ-0752 (10 10.1),
  REQ-0753 (10 10.1), REQ-0754 (10 10.1), REQ-0755 (10 10.1), REQ-0756
  (10 10.1), REQ-0757 (10 10.1), REQ-0758 (10 10.1), REQ-0759 (10 10.1),
  REQ-0760 (10 10.1), REQ-0761 (10 10.1), REQ-0762 (10 10.1), REQ-0763
  (10 10.1), REQ-0764 (10 10.1), REQ-0765 (10 10.1), REQ-0766 (10 10.1),
  REQ-0767 (10 10.1), REQ-0768 (10 10.1), REQ-0769 (10 10.1), REQ-0770
  (10 10.1), REQ-0771 (10 10.1), REQ-0772 (10 10.1), REQ-0773 (10 10.1),
  REQ-0774 (10 10.1), REQ-0793 (10 10.1), REQ-0962 (10 10.6), REQ-1111
  (12 12.3), REQ-1121 (12 12.3).
- Operator: no.
- Acceptance:
  - A table test has one row per escaped class of I29 (control, bidi,
    zero-width, tag characters, U+2028 and U+2029); a fuzz target exists
    with committed seeds (10 10.1).
  - The CLI writer (`internal/cli/writer.go`) escapes by default (04
    4.1, Output).
  - Every tag line is exactly `//romeu:invariant I<n> guard` or
    `//romeu:invariant I<n> test`, and `TestOnlyAllowedDirectives`
    passes with the tags present.
- Verify: `go test ./internal/termsafe/...`; `go test
  ./internal/cli/...`.
