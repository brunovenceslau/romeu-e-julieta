# The allowlist pass-throughs and tools/ci on a shared machine

Two items left open by the allowlist of the pre-push hook and of every
mise run (#34), found open on 2026-10-17:

- The `HOME`, `PATH` and Go cache pass-throughs of the allowlist let a
  poisoned module or build cache reach the run.
- Several agents on one machine run `tools/ci` at the same time, so any
  check that takes a machine-wide resource must be safe under that. The
  lint lock was fixed in #39 (`--allow-parallel-runners`, `-j 1`); the
  reproduction of the parallel-run flake is item 12 of T136.

Reopen: the next change to `hookAllow` or to the mise package of
`tools/ci`, or T136 reopens.
