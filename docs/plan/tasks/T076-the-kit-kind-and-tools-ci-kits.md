---
id: T076
title: "The `kit` kind and `tools/ci kits`"
milestone: M2
module: kits
fate: kept
owns:
  - tools/new/kit*.go
  - tools/ci/internal/kits/**
  - tools/ci/check_kits.go
needs:
  - T012
  - T105
  - T106
  - T113
after: []
ask_first:
  - checks
sitting: none
---

# T076 - The `kit` kind and `tools/ci kits`

Goal: The `kit` kind of `tools/new` and `tools/ci kits`: the scaffold,
the five-line and checksum rules, the pin equality rule, the descriptor
name from sitting M2.

Re-plan (2026-10-12): Kept. The `kit` kind registers by file (rule 4
of `docs/plan.md`, T106). Read differently until M6, in the first row
below: "the file name A11 recorded" is the descriptor file name read by
hand at sitting M2 (T113), kept as this task's fixture with
`source: sitting M2`; A11 re-measures it in M6.

- Implements: 12 12.3 (the `kit` kind), 10 10.2 (the `kits` step).
- Operator: no.
- Acceptance:
  - `go run ./tools/new kit <name>` writes `kits/<name>/` with a
    descriptor on the pinned frontend, under the file name A11 recorded.
  - `tools/ci kits` fails a second frontend pin, an install step over
    five lines, a download without a checksum line after it, and a
    download line with a pipe or a command substitution.
  - It also fails a kit whose frontend, mise or herdr pin differs from
    the pin file.
- Verify: `go test ./tools/ci/... -run '^TestKits'`, which runs the
  fixture kits; `go run ./tools/ci kits`.
