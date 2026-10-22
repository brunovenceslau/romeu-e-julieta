# Shard the macOS race run and cache GOCACHE, deferred

What is at stake: small. The macOS race, about 9 minutes on 2026-10-20
and 9m43 in the green run of T157 (#234, merged 2026-10-22), is the
gate's floor.

Scope: this repository.

Deferred on 2026-10-20:

- Shard the macOS race over two or three jobs by package, with a final
  job that merges the cover profiles before `tools/ci coverage`.
- Restore GOCACHE between runs. Probably the largest single gain, since
  the race builds every package from nothing, but the repository treats
  a poisoned GOCACHE as a threat (`tools/ci/internal/mise/misefiles.go:160`),
  so it is a security decision for the operator; measure first how much
  of the race step is compilation.

Reopen: when the parallel jobs land and the macOS race is still the
slowest job, or when the operator asks for the GOCACHE measurement.
