---
id: T215
title: "`SSH_AUTH_SOCK` on a generation whose digests differ or that was adopted"
milestone: M6
module: romeu-cli
fate: deferred
owns: []
needs:
  - T080
  - T066
  - T026
after: []
ask_first:
  - sbxdrv
  - gates
  - formats
sitting: none
---

# T215 - `SSH_AUTH_SOCK` on a generation whose digests differ or that was adopted

Goal: the execs and the attach of a project forward `SSH_AUTH_SOCK` to a
sandbox only when the kit set it was created with and the live render
both use `git-ssh-sign`, and never to an adopted generation.

Added on 2026-11-07 (scope this repository, as decided on 2026-11-07),
deferred from the start: the row of 05 5.4 that records the gap
(REQ-1394) names no task, and no open task can build it. T080 forwards
`SSH_AUTH_SOCK` from the live render's kit set; on a generation whose
recreate digests differ, or that was adopted, `run` attaches by `sbx env
exec` to a sandbox that may never have been created with
`git-ssh-sign`, or whose kit was removed for safety. The creating
render's kit set is not recorded: 03 3.8 holds `createdWith`, `repos`
and `sbxVersion` for a generation, and the `generations[].kits` record
of the in-place apply is in the scope T210 decides. So the member this
task needs, the kit ids recorded at create, lands with its spec text
(03 3.8, ask-first `formats`, and its write in `internal/state`, ask-first
`gates`) when the task reopens. Reopened by: the first report of a
signing prompt from a sandbox that was not created with `git-ssh-sign`;
or T210 opening the in-place apply (REQ-1394 no longer cites A4;
amended on 2026-11-07). `owns` is written when the task
reopens: T080's `internal/sbxdrv/env_signing.go`, the state member and
the files of `run` that pass the environment.

- Implements: REQ-1394 (05 5.4).
- Operator: yes, the approval lines for `internal/sbxdrv/**` (ask-first
  `sbxdrv`, `env_signing.go`), `internal/state/**` (ask-first `gates`,
  the creating kit set) and `schemas/**` (ask-first `formats`, the
  regenerated `state-project.v1.json`) when the task reopens.
Amended on 2026-11-08 (as decided on 2026-11-08; scope every adopter
of the product): withholding `SSH_AUTH_SOCK` revokes nothing, since sbx
keeps the last agent forwarded into a sandbox (measured on 2026-11-07
and 2026-11-08, sbx v0.51.0). Where this task does not forward the
signing socket, the exec or the attach forwards an agent with no keys,
which replaces the signing key in the sandbox; its form (an agent romeu
serves through `golang.org/x/crypto/ssh/agent`, or a socket where
nothing listens) is measured first, since `sbx env run -d` refuses a
start of an `ssh-agent@1` kit without a host agent. Measured on
2026-11-08 (sbx v0.51.0): both forms replace the forwarded agent on
exec and on a start; the served agent lists `no identities`, the
empty path makes the agent inside fail.

- Acceptance:
  - Inside the sandbox, `ssh-add -L` lists no key after such an exec,
    where the signing key was listed before it.
  - On a generation whose recreate digests differ, an exec and the
    attach carry `SSH_AUTH_SOCK` only when the recorded creating kit set
    and the live render both hold `git-ssh-sign`; a unit table covers
    the four combinations.
  - On an adopted generation no exec and no attach carries it, whatever
    the live render holds.
- Verify: `go test ./internal/sbxdrv/... -run '^TestSigningForward' -v`.
  The pattern names the functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
