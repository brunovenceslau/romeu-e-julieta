# Re-plan items 8, 9 and 11: the ask-first surfaces

Decided on 2026-10-17, for T107 and the "Decisions of the re-plan" of
docs/plan.md, each as its recommendation proposed:

- Item 8, yes, in T107: the `checks` surface narrows to the runner and
  the security-relevant packages (`tools/ci/*.go`,
  `tools/ci/denylist.yaml`, `.githooks/**`, `.golangci.yml`,
  `tools/ci/internal/{hygiene,names,pr,generated,workflows,plan,askfirst,kits,catalog,callsites}/**`,
  `docs/acceptance.json`).
- Item 9, yes, in T107, in the form of T107's acceptance, which is
  newer than the item's text: the `contracts` globs become
  `internal/*/errors.go`, `internal/cmd/*/*/errors.go`,
  `internal/spec/versions.go` and `internal/manifest/versions.go`. The
  item's own text named `internal/cli/exit.go` and
  `internal/spec/versions.go`; the per-package error ids stay under
  approval because they are a contract.
- Item 11, yes, in T107: add `sbxdrv`, put `skills/**` under `kits`
  (R8-05-3), `catalog` is `internal/catalog/**` (T122 already wrote
  that glob), and `ledger` stays while its row is deferred.

T107 records each answer in docs/plan.md as "decided on 2026-10-17".
Item 10 is still decided at sitting M0.

Reopen: T107's pull request, which applies them.
