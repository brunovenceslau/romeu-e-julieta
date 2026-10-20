# The spec catches up with the hardening of T127, T143 and T144

What is at stake: small; the code is stricter than the specification
says, so nothing is admitted that should not be.

The spec text landed on 2026-10-20 (REQ-0808, REQ-0798, REQ-0463 and
I3 of 05 5.1; the step order of REQ-0901 went to T088's Acceptance).
One question is left open:

- Since T144, a cached download of the container e2e that fails its pin
  is removed and fetched once, and the removal writes no log line, so a
  corrupted or tampered cache entry leaves no trace (REQ-0798). Decide
  whether it should write one, without adding output the infrastructure
  classifier of `tools/ci/internal/container` counts as a test's own.

Decided on 2026-10-20: the removal writes a job annotation, a channel
the classifier does not read.

Reopen: when that task is planned, or when a cache entry is found
corrupted.
