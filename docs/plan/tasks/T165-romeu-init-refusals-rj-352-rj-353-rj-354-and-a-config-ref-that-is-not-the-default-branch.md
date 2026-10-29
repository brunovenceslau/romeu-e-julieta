---
id: T165
title: "`romeu init` refusals: RJ-352, RJ-353, RJ-354 and a config ref that is not the default branch"
milestone: M3
module: romeu-cli
fate: merged
owns:
  - internal/cmd/romeu/init/**
  - internal/cmd/romeu/all/import_init.go
  - e2e/init_test.go
  - docs/reference/errors.md
needs:
  - T041
after:
  - T164
ask_first:
  - contracts
sitting: none
merged: 2026-10-29
pr: 326
---

# T165 - `romeu init` refusals: RJ-352, RJ-353, RJ-354 and a config ref that is not the default branch

Goal: `romeu init` raises the error ids RJ-352 `root-unsafe`, RJ-353
`settings-exist` and RJ-354 `settings-unwritable` that 04 4.4 lists, and
refuses a `--config-ref` that is not the remote's default branch.
Decided on 2026-10-23, scope this repository.

T041 (#259) refuses with the three slugs as plain rule findings, exit 2
and no id of its own (`refuse` in `internal/cmd/romeu/init/init.go`,
REQ-1258). The three rows of 04 4.4 were added on 2026-10-23 without code
because `docs/reference/errors.md` is generated from the `errors.go` of
the package that raises an id, and `errors.go` is on the `contracts`
surface. T164 creates `internal/cmd/romeu/init/errors.go` first, and
this task adds the three rows and their hints to it, switches `refuse`
to them, and regenerates the reference page.

The second part is a rename. #259 already refuses a `--config-ref`
that is not the remote's default branch, and a remote with no default
branch, but both findings carry the slug `config-ref-not-default`
(`internal/cmd/romeu/init/init.go`, about lines 413 and 416). The
decision of 2026-10-23 is two slugs: `config-ref-not-default` for a ref
other than the default branch and `config-no-default-branch` for a
remote with no default branch (04 4.2). This task renames the second
finding and updates the e2e (`e2e/init_test.go`, about lines 246 and
266). Both stay plain rule findings with no id of their own (REQ-1258).

- Implements: REQ-1275 (04 4.2), REQ-1278 (04 4.4), REQ-1279 (04 4.4),
  REQ-1280 (04 4.4).
- Operator: no.
- Acceptance:
  - `errors.go` declares RJ-352, RJ-353 and RJ-354 with the slugs, exit
    2 and hints of 04 4.4, and `go generate ./...` leaves
    `docs/reference/errors.md` listing them.
  - The e2e cases that assert the three slugs (`e2e/init_test.go`) also
    assert the ids.
  - A remote with no default branch exits 2 with
    `config-no-default-branch`, and a ref that is not the default
    branch still exits 2 with `config-ref-not-default`; the e2e asserts
    each slug.
- Verify: `go test -tags e2e ./e2e/... -run '^TestInit' -v`; the
  reviewer reads the `TestInit*` functions this task adds in the `-v`
  output (10 10.8).
