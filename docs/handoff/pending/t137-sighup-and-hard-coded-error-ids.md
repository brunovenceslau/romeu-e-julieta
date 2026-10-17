# T137: SIGHUP and hard-coded error ids

- SIGHUP leaves process groups running, because Setpgid shields them
  from the hangup. This is a spec decision and a candidate Deferred
  decisions row.
- T008 created `cli.Error` (#109) but left two packages on their own
  method sets: internal/procgroup's TimeoutError (RJ-323) and
  internal/gitsafe's FloorError (RJ-344). Until they carry a
  `*cli.Error` in their chain, `cli.Main` reports them as the error
  internal (RJ-348), exit 1, with the hint to open an issue. No command
  exists yet, so no user sees it. Their adoption edits
  internal/procgroup/errors.go and internal/gitsafe/errors.go, on the
  procgroup, gitsafe and contracts surfaces, which T008 does not own
  (rule 1 of docs/plan.md). An `As` method on each type that yields
  its `*cli.Error` keeps the types the tests assert.

Reopen: the operator's approval of the procgroup and gitsafe surfaces
for the adoption, or the first command that runs a subprocess or git;
the next spec pass on signals.
