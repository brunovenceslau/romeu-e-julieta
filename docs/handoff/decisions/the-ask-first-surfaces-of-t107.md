# The ask-first surfaces of T107

Decided on 2026-10-17, for T107, answering items 8, 9 and 11 of the
"Decisions of the re-plan" of docs/plan.md and the `tools/mutants`
question T107 carries. It replaces the decision file
"re-plan-items-8-9-and-11-the-ask-first-surfaces", whose last
paragraph gave the plan's record to T107.

- Item 8, yes: `checks` narrows to the runner and the packages a
  security rule rests on. The item's list was written before T105 moved
  the checks below `tools/ci/internal/`; its intent holds, so the
  runner (`tools/ci/*.go`, `tools/ci/internal/check/**`) and the moved
  security code stay on it: `internal/mise`, `internal/setup`,
  `internal/pushedrange`, `tools/ci/git`, `tools/ci/goenv`,
  `tools/ci/names` and `tools/ci/pushed`, besides the item's
  `hygiene`, `generated`, `workflows`, `plan`, `askfirst`, `pr`,
  `kits`, `catalog`, `callsites`, the denylist, `.githooks/**`,
  `.golangci.yml` and `docs/acceptance.json`.
- `tools/mutants/**` joins `checks`.
- Item 9, yes, in the form of T107's acceptance: `contracts` is
  `internal/*/errors.go`, `internal/cmd/*/*/errors.go`,
  `internal/spec/versions.go` and `internal/manifest/versions.go`.
- Item 11, yes: `sbxdrv` (`internal/sbxdrv/**`, `e2e/fakesbx/**`) is a
  new surface, `skills/**` joins `kits`, `catalog` keeps
  `internal/catalog/**` (T122 wrote it), `ledger` stays while its row
  is deferred.
- `sbxdrv` holds the I4 and I25 guards, so it joins the surfaces of
  12 12.4 (REQ-1162) that keep an approval line on each pull request.
- T107 edits no `docs/plan.md`, which T108 owns: T108, the re-plan
  record, writes these answers there as "decided on 2026-10-17".
