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
after: []
ask_first:
  - termsafe
sitting: none
---

# T007 - `termsafe` and the CLI writer

Earlier title: `termsafe`, the CLI writer and the invariant tooling.

Goal: The escape table of I29 with its fuzz seeds, and the CLI writer of
`internal/cli/writer.go`.

Re-plan (2026-10-12): Kept, narrowed. The `invariant` kind of
`tools/new`, `tools/ci invariants` and `mutate` leave this task
(deferred, see Decisions this plan defers in `docs/plan.md`); the guard
and test tags are written as comments so the check finds them when it
lands. The acceptance rows below that name that tooling are deferred
with it; the rest stand.

- Implements: 05 5.2 (I29, the tagging rules), 12 12.2 (the `termsafe`
  row), 12 12.3 (the `invariant` kind; the `mutate` trigger), 10 10.2
  (`invariants`, `mutate`), S6.
- Operator: no.
- Acceptance:
  - A table test has one row per escaped class of I29 (control, bidi,
    zero-width, tag characters, U+2028 and U+2029); a fuzz target exists
    with committed seeds (10 10.1).
  - The CLI writer (`internal/cli/writer.go`) escapes by default (04
    4.1, Output).
- Deferred with the tooling, and asserted when it lands (M6):
  - `go run ./tools/new invariant` scaffolds a tagged guard stub and a
    failing tagged test.
  - `tools/ci invariants` fails a guard tag without a test tag, the
    reverse, and a tagged id that is no row of 05 5.2.
  - `tools/ci mutate` builds the `mutate_I29` stub and a tagged test
    fails; it reads `.github/ask-first.yaml` to decide which pull
    requests it runs on (05 5.3).
- Verify: `go test ./internal/termsafe/...`; `go test
  ./internal/cli/...`.
