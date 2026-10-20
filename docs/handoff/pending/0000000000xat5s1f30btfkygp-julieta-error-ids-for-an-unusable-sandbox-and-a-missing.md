# julieta error ids for an unusable sandbox and a missing manifest

What is at stake: small; the exit codes are right, only the error id
and its hint are wrong, so a user reads the wrong advice. A new id is
one row in 04 4.4 and its `errors.go` entry.

Decided on 2026-10-20, while building T051: no new error id lands with
T051, which forecasts no ask-first surface.

- `julieta setup` exits 1 when the sandbox is left unusable (the PATH
  link, a secondary clone). With no id of its own, the error maps to
  `internal` (`RJ-348`), whose hint says to open an issue: REQ-0786 calls
  an error without a `*cli.Error` a defect, which this is not. A row such
  as `sandbox-unusable`, exit 1, hint `decide: fix <step>, then run
  julieta setup`, fixes it; `TestSetupUnusable` then asserts the id, not
  only the code.
- A missing or unreadable manifest exits 2 with `manifest-protocol`
  (`RJ-342`), whose row (REQ-0440) covers only a protocol outside N and
  N-1. Its hint, `romeu run <name>`, is the right fix, so the reuse is
  tolerable; widening REQ-0440 or a `manifest-missing` row settles it.
  The tests pin RJ-342 for the missing case, so a split touches them.

Reopen: the next change to the error table of 04 4.4, or the first user
report of the misleading hint; as a spec and `contracts` pull request.
