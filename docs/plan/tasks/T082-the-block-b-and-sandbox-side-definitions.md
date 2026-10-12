---
id: T082
title: "The block B and sandbox-side definitions"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T075
  - T079
  - T081
after: []
ask_first: []
sitting: none
---

# T082 - The block B and sandbox-side definitions

Re-plan (2026-10-12): deferred to M6; release only. Reopened by: a
real-sbx behaviour the fake cannot model breaks a daily run; or the
first release candidate. The text below is the one written before the
re-plan; `needs` lists its earlier dependencies with a folded id
replaced by the id that absorbed it, and `owns` is written when the task
reopens.

- Implements: 11 11.2, 11 11.3, 10 10.3 (Redaction), 07 7.4 (B4), S3,
  S5, S7, S8.
- Operator: feeds blocks O5 and O6.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - A definition exists for each id of 11 11.2: B1, B2, B3, B4 and B5,
    each `kind: check` with the predicate its Step cell names, and C2,
    C3, C4, C5 and C6, each with its declared `kind`, `expect` and
    decision table.
  - C5 is `kind: observe` with keys `sessionEndBeforeStart` (bool),
    `sessionStartSources` (string list), `hookOutputReachesContext`
    (bool), `hookOutputCutBytes` (int or null), `nonZeroExitEffect`
    (string), `sessionEndTimeoutMs` (int or null); its decision table
    has one row per key, each naming its own `affects`.
  - B1 installs the candidate from its GitHub release and verifies
    checksums and attestation; each other block B result copies two
    observations from it, the candidate's tag and the commit that tag
    names. `--only <id>` runs one probe of a block, for the B1 run on
    v1.0.0 (11 11.2); the plan's addition.
  - With a `--version <tag>` flag, B1 is not run, and the two
    observations B1 would give are taken from the flag and marked
    `rehearsal`; tested against the helper binary: B1 is absent from the
    result set and both observations carry `rehearsal`.
  - B3 runs `go test -tags host -json ./e2e/host/...`, writes the output
    through the recorder's redaction, the positive scrubbing of T018
    included, to `docs/probes/B3-<host>-test.json`, records its sha256
    and has one observation per journey. The `Output` lines of `go test
    -json` carry free text from the real `sbx` and the real agent, and
    an account email or organisation name is not among the computed
    identifiers, so the artifact keeps `Output` for no passing test;
    that is the plan's addition.
  - B4's run without a GitHub token binds no `github` secret in the
    throwaway sandbox's env file, and its run with one binds the
    project's named secret (07 7.4); neither run changes a global
    secret, so nothing has to be restored afterwards.
  - B5's predicate is the S8 bound; the sandbox-side checks run through
    `sbx env exec` in the sandbox B2 started.
  - Each is tested against the helper binary. This task lands before the
    candidate tag because the harness is outside `docs/`: a definition
    added after the tag would need a new candidate (11 11.2).
- Verify: `go run ./e2e/probes --block B --out <dir>` against the helper
  binary in a test, and the same with `--version v1.0.0-rc.1`, the
  command line of block O5, which must parse and exit 0.
