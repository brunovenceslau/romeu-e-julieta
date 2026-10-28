# T111 sitting M1 in the romeu-e-julieta sandbox: reads, the hook fix and the fourth edit

Done on 2026-10-27, in the repository of the sandbox definitions, whose
runbook holds each Record line. Its pull request numbers are given as
"there".

- Steps 6 and 7 after the recreate, and the first run of step 8: 77
  store items, 77 entries, 0 failed (#296 and #297 there). 63 of the
  77 entries were closed as obsolete, never deleted, so the end run
  does not add them again.
- The acceptance reads that this sandbox can make (#298 there): a
  signed commit, a push of a throwaway branch, the snapshot (51472
  bytes, only the base d4290531 required), status, handoff, version and
  memory. Not yet read: the wrappers and the guard with the bin mount
  absent, and the absence of the store writer, after a recreate.
- Each read found that the repository `.githooks/<hook>` ran twice: the
  system hook of the git-hooks kit ran it, and then julieta's tracked
  stage ran it again, so a push ran `tools/ci fast` twice. The kit's
  dispatcher now exits 0 when `JULIETA_HOOKS_SYSTEM=1` is set (#299
  there); julieta is unchanged, as REQ-0408 already runs both hooks.
  It takes effect at the next recreate.
- Step 10, the fourth edit (#300 there): the romeu-e-julieta and verona
  definitions drop the store's writer kit and its two variables, and
  mount the store read-only. It takes effect at the next recreate.
