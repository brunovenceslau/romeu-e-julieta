# The grants of 2026-10-19

What is at stake: each grant lets a session change an ask-first surface
without asking again; a wrong change there is caught in review and
undone by a revert, and the merge stays the maintainer's.

Decided on 2026-10-19, after sitting M0:

- T013 may change the `checks`, `approvals` and `dependencies`
  surfaces, and the workload is pinned to tag 2.1.296 by digest,
  `docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5`,
  the image sitting M0 ran on, through `go run ./tools/kitpin workload`.
- T076 may change the `checks` surface.
- T135 may change the `contracts` surface, and REQ-0971 gains a sentence
  like the one for `tools/new`, so `tools/ci` imports
  `internal/manifest`.
- T103 may change the `release` surface.
- T112 is built in the sandbox kit repository and the config
  repository, under their own rules.
- ADR 0036 and ADR 0037 are Accepted.
- The behaviour of `romeu init` on a non-empty root is decided in T041's
  re-plan, before T041 starts.

## Standing grants for the next tasks

Decided on 2026-10-19, after the batch above: every task with fate
kept or new that forecasts one of these surfaces may change it, with
its review, its approval line and the maintainer's merge. A task
re-planned later onto one of them needs a new grant.

- `checks`, `contracts`, `dependencies` and `catalog`, the grantable
  surfaces of 12 12.4: written in `docs/grants.yaml` as grants of
  sitting M0, each with the open tasks that forecast it.
- `spec`, `kits`, `sbxdrv` and `gates`: not grantable under 12 12.4, so
  each pull request still carries its approval line; this note is what
  the line rests on.
- Not granted, asked each time, except the grants above for T013 and
  T103: `procgroup`, `release`, `signing`, `approvals`, `decisions` and
  `ask-first`.
