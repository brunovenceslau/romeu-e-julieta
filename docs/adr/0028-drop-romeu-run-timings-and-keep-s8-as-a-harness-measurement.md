# 28. Drop romeu run timings and keep S8 as a harness measurement

Date: 2026-10-18

## Status

Proposed

## Context

Success criterion S8 had its evidence in a flag, `romeu run --timings`,
and the flag existed for the criterion. Its bounds of 2 s and 3 s had
no stated origin. Review round 8 raised this as item SC3 of [round 8](../reviews/round-8.md#decisions-for-the-operator).
The decision supersedes the CI measurement of romeu's half that finding
H4 of the test-engineer lens set in round 1, and the finding
R3-testability-14 of round 6; the julieta assertion of op-s8-limit in
the container end-to-end test stays either way.

### Alternatives considered

- **Keep the flag.** In both options the bounds gain the words "starting
  values". Not taken.
- **Drop the flag**, its format and the CI assertion, and keep S8 as a
  harness measurement. Taken.

## Decision

Decided on 2026-10-08: `--timings`, the `run-timings.v1` format and the
CI assertion are dropped. S8 stays as a harness measurement (B5). The
2 s and 3 s bounds are starting values, and the first B5 measurement
confirms or resets them.

## Consequences

- The criterion no longer has a flag to show its evidence; the harness
  measures it.
- The bounds are not binding until a measurement sets them.
- The julieta assertion of the container end-to-end test stays.
