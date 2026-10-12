---
id: T075
title: "The remaining scenario functions and the host suite"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T074
after: []
ask_first: []
sitting: none
---

# T075 - The remaining scenario functions and the host suite

Re-plan (2026-10-12): deferred to M6; the CI journeys with the fake
cover J2, J3b, J6, J7, J10 and J11; the maintainer's own projects are
the smoke steps. The J5 function (from T059) and the J8 function with
its step 1 (from T053) join the functions this task adds, since those
tasks are M1 and M2 and the package starts in T041 (M3); the container
e2e of each asserts its outcome until then. Reopened by: the first
release candidate. The text
below is the one written before the re-plan; `needs` lists its earlier
dependencies with a folded id replaced by the id that absorbed it, and
`owns` is written when the task reopens.

- Implements: 10 10.3 (Shared scenarios), 10 10.1 (Host), 09 (J1, J9,
  J12), S7.
- Operator: no.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - `e2e/scenarios` gains the functions the command tasks did not add,
    each run in CI as far as the fake allows and holding at least one
    named assertion of its journey's outcome: the rest of J1 after its
    step 3 asserts the `doctor` output of step 4, the `approve
    --toolchain` record, the gate of step 6 and the `run` of step 7; J9
    asserts the gate 1 print, `sync --all`, and that only a project
    whose kit content or capabilities changed meets gate 2 and needs
    `recreate`; J12 runs J1 steps 3 to 6 under a fresh root, settings
    and state (09) and asserts the same outcomes there. J8, started in
    T053, gains its step 2: after `romeu sync`, a new non-upload catalog
    domain is applied live through the recorded argv (T047), and an
    upload-capable or unknown host meets gate 2.
  - A function exists for each `## J<n>` heading of 09, and a test
    compares the two lists. The test is the plan's own guard for 10 10.3
    (Shared scenarios), not a rule of the specification.
  - The session table of T019 is checked in its other direction here: a
    row that no end-to-end test declares fails, now that every test that
    calls the fake exists.
  - The five CI journeys of S7 pass, at the hybrid level where they
    reach julieta.
  - `e2e/host` compiles under the `host` tag and calls the same
    functions (10 10.3); it runs in block O6.
- Verify: `go test -tags e2e ./e2e/...`; `go vet -tags host
  ./e2e/host/...`.
