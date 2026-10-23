---
id: T165
title: "`romeu init` refusals: RJ-352, RJ-353, RJ-354 and a config ref that is not the default branch"
milestone: M3
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/init/**
  - internal/cmd/romeu/all/import_init.go
  - e2e/init_test.go
  - docs/reference/errors.md
needs:
  - T041
after: []
ask_first:
  - contracts
sitting: none
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
surface. This task writes `internal/cmd/romeu/init/errors.go` with the
three rows and their hints, switches `refuse` to them, and regenerates
the reference page.

The second part is the `--config-ref` rule of the `init` row of 04 4.2:
v1 reads a config repository from its default branch only, so `init`
asks the remote for its default branch and refuses a ref that is another
branch or a tag (slug `config-ref-not-default`) and a remote with no
default branch (slug `config-no-default-branch`), exit 2, before it
writes anything. These two are plain rule findings with no id of their
own (REQ-1258).

- Implements: REQ-1275 (04 4.2), REQ-1278 (04 4.4), REQ-1279 (04 4.4),
  REQ-1280 (04 4.4).
- Operator: no.
- Acceptance:
  - `errors.go` declares RJ-352, RJ-353 and RJ-354 with the slugs, exit
    2 and hints of 04 4.4, and `go generate ./...` leaves
    `docs/reference/errors.md` listing them.
  - The e2e cases that assert the three slugs (`e2e/init_test.go`) also
    assert the ids.
  - `init --config-ref <a tag>` and `init --config-ref <another branch>`
    against an origin whose default branch is `main` exit 2 with
    `config-ref-not-default`; against an origin with no default branch
    exit 2 with `config-no-default-branch`; neither writes settings.
  - `init --config-ref main` and no flag, against that origin, succeed.
- Verify: `go test -tags e2e ./e2e/... -run '^TestInit' -v`; the
  reviewer reads the `TestInit*` functions this task adds in the `-v`
  output (10 10.8).
