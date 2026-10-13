---
id: T102
title: "Acceptance"
milestone: M6
module: ci-release
fate: deferred
owns: []
needs:
  - T094
  - T095
  - T101
  - T093
after: []
ask_first:
  - checks
sitting: O7
---

# T102 - Acceptance

Re-plan (2026-10-12): deferred to M6; release only; T091's v1 catalog
table folds into its acceptance, and its rows are copied below. Reopened
by: the first release candidate. The text below is the one written
before the re-plan; `needs` lists its earlier dependencies with a folded
id replaced by the id that absorbed it, and `owns` is written when the
task reopens.

- Implements: REQ-0002 (index), REQ-0003 (index), REQ-0004 (index),
  REQ-0006 (index), REQ-0007 (index), REQ-0009 (index), REQ-0475 (05
  5.2), REQ-0871 (10 10.2), REQ-0872 (10 10.2), REQ-0873 (10 10.2),
  REQ-0923 (10 10.2), REQ-0944 (10 10.5), REQ-0945 (10 10.5), REQ-0948
  (10 10.5), REQ-1014 (11 11.2), REQ-1035 (11 11.4), REQ-1093 (12 12.2),
  REQ-1095 (12 12.2), REQ-1096 (12 12.2), REQ-1097 (12 12.2), REQ-1098
  (12 12.2), REQ-1099 (12 12.2), REQ-1100 (12 12.2).
- Operator: block O7 comes first.
- Ask-first, as forecast before the re-plan: `checks`
  (`docs/acceptance.json`).
- Acceptance:
  - `go run ./tools/ci links` ran once, and a dead link it finds is
    fixed or reopens the deferred row about a scheduled check.
  - `docs/acceptance.json` has one entry per success criterion, and the
    release and CI-run items name the release and the run of the v1.0.0
    commit.
  - S4: the reference config repo's `validate.yml` run on the release's
    julieta, where `julieta spec validate --catalog projects/*.yaml`
    exits 0, is the S4 ci-run item; a key or host it lists first gains
    an entry under `internal/catalog/egress/` with an explicit `upload`
    flag, and the reference locks' output, read and redacted first, is a
    fixture under `internal/catalog/testdata/`.
  - The result files of the B1 and B2 runs on v1.0.0, written by the
    harness through its redaction, are under the pull request's
    Evidence, and no raw run output is; `go run ./tools/ci probes
    --require-pass B1,B2 --hosts 1 --dir <that directory>` exited 0 on
    them in block O7. **[review]**
  - The six end-of-plan checks of 10 10.5 join `all` in this pull
    request, and the seventh (every step row of 10 10.2 has a registered
    check).
- Acceptance (from T091, folded):
  - The output block O4 handed over, read and redacted by the maintainer
    first, is committed as a fixture under `internal/catalog/testdata/`;
    for the keys and hosts it lists, the catalog gains entries, each
    domain with an explicit `upload` flag, and a test asserts that every
    listed key and host has one.
  - `tools/ci catalog` passes.
  - That `julieta spec validate --catalog` then exits 0 against the
    reference config repo is the maintainer's first step of block O5.
- Verify: `go run ./tools/ci acceptance` exits 0.
