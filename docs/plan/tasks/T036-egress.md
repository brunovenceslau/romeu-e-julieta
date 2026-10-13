---
id: T036
title: "`egress`"
milestone: M3
module: egress
fate: kept
owns:
  - internal/egress/**
  - internal/cmd/julieta/lock/catalog_check.go
needs:
  - T031
  - T024
after:
  - T053
ask_first: []
sitting: none
---

# T036 - `egress`

Goal: `egress`: the split, `egress.extra`, `egress.tools`, kit-declared
domains, upload flags, derivation from locks with the TOML parser; the
catalog lookup of `julieta lock --check`.

Re-plan (2026-10-12): Kept. The TOML parser enters `go.mod` with its
first importer, the mise driver (T053), which this task comes after
(rule 7 of `docs/plan.md`); T053 lands `lock --check` without the
catalog lookup, and this task adds it in
`internal/cmd/julieta/lock/catalog_check.go`, admitted by that `after`
edge, so M2 does not wait for lane A.

- Implements: REQ-0044 (01 1.2), REQ-0057 (01 1.3), REQ-0069 (01 1.4),
  REQ-0169 (03 (opening)), REQ-0188 (03 3.1), REQ-0214 (03 3.5),
  REQ-0341 (04 4.2), REQ-0437 (04 4.4), REQ-0438 (04 4.4), REQ-0439 (04
  4.4), REQ-0440 (04 4.4), REQ-0441 (04 4.4), REQ-0442 (04 4.4),
  REQ-0443 (04 4.4), REQ-0444 (04 4.4), REQ-0445 (04 4.4), REQ-0446 (04
  4.4), REQ-0492 (05 5.2), REQ-0493 (05 5.2), REQ-0569 (06 6.1),
  REQ-0610 (07 7.1), REQ-0613 (07 7.3), REQ-0615 (07 7.3), REQ-0617 (07
  7.3), REQ-0618 (07 7.3), REQ-0619 (07 7.3), REQ-0620 (07 7.3),
  REQ-0621 (07 7.3), REQ-0628 (07 7.5), REQ-0629 (07 7.5), REQ-0630 (07
  7.5), REQ-0631 (07 7.5), REQ-0632 (07 7.5), REQ-0633 (07 7.5),
  REQ-0752 (10 10.1), REQ-0753 (10 10.1), REQ-0754 (10 10.1), REQ-0755
  (10 10.1), REQ-0756 (10 10.1), REQ-0757 (10 10.1), REQ-0758 (10 10.1),
  REQ-0759 (10 10.1), REQ-0760 (10 10.1), REQ-0761 (10 10.1), REQ-0762
  (10 10.1), REQ-0763 (10 10.1), REQ-0764 (10 10.1), REQ-0765 (10 10.1),
  REQ-0766 (10 10.1), REQ-0767 (10 10.1), REQ-0768 (10 10.1), REQ-0769
  (10 10.1), REQ-0770 (10 10.1), REQ-0771 (10 10.1), REQ-0772 (10 10.1),
  REQ-0773 (10 10.1), REQ-0774 (10 10.1).
- Operator: no.
- Acceptance:
  - The eight steps of 07 7.5 have a table test: a `mise.toml` without
    `mise.lock` is `lock-missing` (`RJ-340`); an unknown `backend:tool`
    without a spec entry is exit 2 naming the tool; a lock host the
    catalog did not produce is gated.
  - A lock that does not parse, or has an unknown top-level key, is
    `lock-unparsed` (`RJ-325`) naming the repo and the fix `relock with
    julieta lock`, with no partial result; one fixture of the pinned
    mise layout backs the parser test. A `mise.lock` with a duplicate
    key or table, an unknown top-level key or over the cap is `RJ-325`.
  - A lock host that fails the egress domain rule of 03 3.1 is
    `lock-host-invalid` (`RJ-326`) naming the entry.
  - A lock over 1 MiB is refused by the shared file cap.
  - A domain with `upload: true` or with no `domains` entry is gated,
    for catalog, base and kit-declared domains (I10).
  - The lock is read with `git cat-file` at `egressCommit`, and no mise
    process runs (07 7.1; I9).
  - `julieta lock --check` warns `lock-unknown-tool` for a
    `backend:tool` the embedded catalog lacks, naming the two fixes of
    07 7.5 step 2, with a fixture each way.
- Verify: `go test ./internal/egress/...`.
