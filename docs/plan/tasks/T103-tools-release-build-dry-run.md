---
id: T103
title: "`tools/release build --dry-run`"
milestone: M3
module: ci-release
fate: kept
owns:
  - tools/release/**
needs:
  - T009
  - T012
after: []
ask_first:
  - release
sitting: none
---

# T103 - `tools/release build --dry-run`

Goal: `tools/release build --dry-run`: the embedding, so a source build
refuses `run`, `sync` and promotion.

Re-plan (2026-10-12): Kept. The second row below is corrected for the
new order: the kits land in M2, before the first slot of sitting M3.

- Implements: 10 10.2 ("Release and bootstrap"), 02 2.1 (Embedding), 12
  12.2 (the smoke steps).
- Operator: no.
- Acceptance:
  - `tools/release build --version v1.0.0 --dry-run` builds the two
    julieta binaries, embeds them with the kits and the catalog present
    at that task, builds romeu for darwin and writes the archives and
    `checksums.txt`.
  - A romeu from that build runs `sync` past the `julieta-not-embedded`
    refusal of a source build (02 2.1), so the first slot of sitting M3
    (T116), which is smoke step 1 of 12 12.2 as amendment 7 in
    `docs/plan.md` reads it, can run. The product kits exist by then
    (M2), and the build embeds the ones present at that task, so the
    slot's project spec names its product kits as 03 3.2 asks.
  - The fuzz-run fixtures of step 1 of the release steps of 10 10.2
    are tests of `tools/release`, by their titles there, and one more,
    "workflow_dispatch fuzz run refuses": a successful `fuzz.yml` run
    whose event is `workflow_dispatch` does not count, and the build
    refuses when no `schedule` run qualifies.
- Verify: `go test ./tools/release/...`; `go run ./tools/release build
  --version v1.0.0 --dry-run`.
