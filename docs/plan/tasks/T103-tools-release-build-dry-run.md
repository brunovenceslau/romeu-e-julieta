---
id: T103
title: "`tools/release build --dry-run`"
milestone: M3
module: ci-release
fate: merged
owns:
  - tools/release/**
needs:
  - T009
  - T012
after: []
ask_first:
  - release
sitting: none
merged: 2026-10-19
pr: 183
---

# T103 - `tools/release build --dry-run`

Merged on 2026-10-19 in pull request #183. The text below is the one
written at the re-plan.

Goal: `tools/release build --dry-run`: the embedding, so a source build
refuses `run`, `sync` and promotion.

Re-plan (2026-10-12): Kept. The second row below is corrected for the
new order: the kits land in M2, before the first slot of sitting M3.

Acceptance that cannot hold when this task merges (recorded on
2026-10-19): the build is written and tested against fixture modules,
but no package of the repository consumes what it embeds, and
`tools/release build` says so plainly (a `--dry-run` prints a line that
names the missing consumer; a build without `--dry-run` refuses). Each
item below is deferred, with the task whose merge reopens it:

- The julieta binary in romeu: the build maps
  `internal/embedded/julieta` with a `go build` overlay, and nothing
  reads that file. Reopened by T043 (the `sync` refusal) and T066 (the
  `run` refusal), whose embed package consumes the path or moves the
  `EmbedPath` constant.
- "A romeu from that build runs `sync` past the `julieta-not-embedded`
  refusal" (smoke step 1 of 12 12.2, T116): reopened by T043, which
  adds `sync` and the refusal.
- The product kits embedded: `kits/embed.go` does not exist. Reopened by
  T077.
- The catalog embedded: `internal/catalog` does not exist. Reopened by
  T031.

- Implements: REQ-0006 (index), REQ-0011 (index), REQ-0037 (00 0.4),
  REQ-0040 (01 1.1), REQ-0128 (02 2.1), REQ-0129 (02 2.1), REQ-0130 (02
  2.1), REQ-0131 (02 2.1), REQ-0432 (04 4.4), REQ-0837 (10 10.2),
  REQ-0902 (10 10.2), REQ-0903 (10 10.2), REQ-0904 (10 10.2), REQ-1038
  (12 12.1), REQ-1039 (12 12.1), REQ-1079 (12 12.2), REQ-1091 (12 12.2),
  REQ-1092 (12 12.2).
- Operator: no.
- Acceptance:
  - `tools/release build --version v1.0.0 --dry-run` builds the two
    julieta binaries, embeds them with the kits and the catalog present
    at that task, builds romeu for darwin and writes the archives and
    `checksums.txt`.
  - `tools/release build`, `--dry-run` included, writes `COPYING` and a
    third-party notices file generated from each binary's module graph
    into every archive; a test reads both back.
  - The build uses `-trimpath`, `CGO_ENABLED=0`, a fixed build id and
    the commit time as build date, and writes archives with fixed
    modification times, owners and order, in the steps' environment
    built from nothing (10 10.2, the build contract); the fixture
    "dry-run build makes no network read" holds.
  - A romeu from that build runs `sync` past the `julieta-not-embedded`
    refusal of a source build (02 2.1), so the first slot of sitting M3
    (T116), smoke step 1 of 12 12.2, can run. The product kits exist by
    then (M2), and the build embeds the ones present at that task, so
    the slot's project spec names its product kits as 03 3.2 asks.
  - The fuzz-run fixtures of step 1 of the release steps of 10 10.2 are
    tests of `tools/release`, by their titles there, and one more,
    "workflow_dispatch fuzz run refuses": a successful `fuzz.yml` run
    whose event is `workflow_dispatch` does not count, and the build
    refuses when no `schedule` run qualifies (round 18, ship gate round
    1, F10).
- Verify: `go test ./tools/release/...`; `go run ./tools/release build
  --version v1.0.0 --dry-run`.

Re-plan (2026-10-21): changed by T156. The row of REQ-0837 states its own reason for `-count=1` (the suite builds julieta through `go build`, a subprocess the test cache does not record), and the step takes its argv from `race.Argv`.
