---
id: T087
title: "`tools/ci links` and `fuzz`"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T085
after: []
ask_first:
  - checks
  - dependencies
sitting: none
---

# T087 - `tools/ci links` and `fuzz`

Re-plan (2026-10-12): deferred to M6; release and docs. Reopened by: the
first adopter other than the maintainer; or the maintainer's
environments all on the new tools. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: 10 10.2 (the subcommands outside `all`), 10 10.1 (Fuzz),
  ADR 0001 rule 5.
- Operator: no.
- Ask-first, as forecast before the re-plan: `checks`, `dependencies`
  (`fuzz.yml`).
- Acceptance:
  - `links` fetches each external URL the docs cite, fails a fixture
    page with one dead URL, and is called by no workflow.
  - `fuzz` runs each fuzz target for a fixed time, and the scheduled
    `fuzz.yml` calls it and then `tools/ci mutate`; the file passes
    `tools/ci workflows`. A test lists every `Fuzz*` function of the
    module and fails when `fuzz` would miss one, so a run with zero
    targets cannot pass.
- Verify: `go test ./tools/ci/... -run '^Test(Links|Fuzz)'`; `go run
  ./tools/ci links`; `go run ./tools/ci fuzz`.
