---
id: T144
title: "The follow-ups of the container e2e"
milestone: M1
module: julieta-core
fate: new
owns:
  - e2e/internal/container/**
  - tools/ci/internal/container/**
  - tools/ci/internal/workflows/workflows.go
  - tools/ci/internal/workflows/workflows_test.go
needs:
  - T013
after:
  - T127
ask_first:
  - checks
sitting: none
---

# T144 - The follow-ups of the container e2e

Goal: the three code gaps the review of T013 (#186) left in the
container e2e and the workflows grammar.

Re-plan (2026-10-20): New. A change to merged work is a task. T013 is
merged, and T127, which also owns `workflows.go`, comes first. The 60
minute job timeout was re-measured on 2026-10-20 with the image pull
and its cache: the slowest job of the last five runs on `main` took
about 15 minutes, so it stays.

- Implements: none: hardening of REQ-0798 and REQ-0808, which T013 and
  T127 implement.
- Operator: no.
- Acceptance:
  - A cached download whose sha256 does not match its pin is deleted and
    fetched once; the fetched file must match the pin, or the run is the
    infrastructure error it is today. A test plants a corrupt entry and
    shows the run recover.
  - The grammar refuses a `github.event.*` expression in a step `env`
    value, in every file; `concurrency.group` keeps the forms it admits
    today. One fixture refuses it in `ci.yml`, and the existing fixtures
    pass unchanged.
  - A failed test is an infrastructure error only when its only failure
    is the infrastructure error it recorded: a test that fails an
    assertion and then records one reads as a test failure. A fixture
    shows each case.
- Verify: `go test ./tools/ci/internal/container/... ./tools/ci/internal/workflows/... ./e2e/internal/container/... -v`.
