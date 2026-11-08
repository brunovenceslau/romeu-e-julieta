---
id: T209
title: "The per-sandbox deny of the workload's plain-HTTP egress pair"
milestone: M4
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/run/egress_deny*.go
  - internal/cmd/romeu/doctor/egress_applied*.go
  - e2e/egress_deny_test.go
  - internal/sbxdrv/policy_deny*.go
  - internal/sbxdrv/guard.go
  - internal/sbxdrv/placeholders.go
  - e2e/fakesbx/**
  - e2e/testdata/sbx/0.51.0/egress.jsonl
  - internal/state/records*.go
  - docs/spec/03-formats.md
  - docs/spec/04-cli.md
  - docs/spec/11-host-probes.md
needs:
  - T205
  - T066
after:
  - T047
  - T203
  - T210
  - T213
ask_first:
  - sbxdrv
  - gates
  - formats
sitting: none
---

# T209 - The per-sandbox deny of the workload's plain-HTTP egress pair

Goal: where T205 measured that a per-sandbox deny on `deb.debian.org:80`
and `dhi.io:80` beats the allow rule sbx provisions from the
`docker.io/sbx/claude` descriptor and survives a recreate under the
same name,
`run` step 4 applies and verifies that deny with the egress reconcile,
and `doctor` reports its absence.

Added on 2026-11-06 (scope this repository): the plain-HTTP row of 05
5.4 (REQ-1374) and Q2 of the index. This task exists only on the deny
branch of T205's decision; on the other branches it is closed with the
reason in its file, and T208 moves the pin without it.

Amended on 2026-11-08 (scope this repository): measured by hand on
2026-11-07, sbx v0.51.0 ([the in-place apply
measurement](../../notes/in-place-apply-measurement.md)), the deny beats
the provisioned allow but does not survive a recreate under the same
name; as decided on 2026-11-07, this is the deny branch with that
condition: step 4 applies the deny after each create and recreate, the
same reconcile that runs on every `run`, and the survival clause of the
goal above is history.

1. The reconcile (REQ-0354): beside the allow rules, `run` step 4
   applies `sbx policy deny network --sandbox <name> <host>:<port>` for
   each pair the workload descriptor declares in its runtime egress
   over plain HTTP (the set of 07 7.5 that only a descriptor declared,
   REQ-1316, filtered to port 80), verifies each with `sbx policy check
   network`, and records each, before and after the calls as the
   allow rules are (REQ-0355), and checks each recorded deny again after
   the reconcile's `policy rm` calls, until A9 records whether
   `--resource <host>` matches a `<host>:80` deny (REQ-0354, added on
   2026-11-08); "in `egressApplied` as a deny" and "every
   pair over plain HTTP" are history: see the amendment below.
2. `doctor` (REQ-0392): the `egress-applied` row compares the deny
   rules too, so a deny that went missing, after a recreate or by hand, is
   a finding.

Amended on 2026-11-07 (as decided on 2026-11-07, decision P with C1-14,
C1-19 and V1.13, V2.10, V3.1): the deny's spec text is this task's own,
written first in its pull request under the exception 07 REQ-1316
makes for the deny of REQ-1374: the deny argv in REQ-0997 (11 11.1);
in REQ-0354 and REQ-0355, the deny applied after the allows, verified
with `sbx policy check network`, and recorded before and after the
call; a typed member of 03 3.8 for the denies, apart from
`egressApplied`, so the allow reconcile of `run` (T066) and of `sync`
(T047) never removes a deny; and the `doctor` row of REQ-0392 comparing
the denies too. The set is the two pairs REQ-1374 names, taken as
fixed data from T205's note, which read them from the descriptor, and
applied only while the pin is the digest T205 measured; T190's
derivation keeps no port, so this task does not read it (RP-7,
amended on 2026-11-07). The denies live in the typed member alone,
never in `egressApplied` (RP-8). The code owns its `sbx policy deny`
builder, its guard and placeholder rows, its fake session and the
generation record's new member, after T202 (the fake), T203 (the
`egress-applied` row's file and the policy session) and T213 (the
record file), and after T210, which edits the same spec pages; it no
longer needs T190, since T066 already orders it after T190. This task is never an edge target: T205's
other branches remove it.

- Implements: REQ-1374 (05 5.4), REQ-0354 (04 4.2), REQ-0355 (04 4.2),
  REQ-0392 (04 4.2).
- Operator: yes, the approval lines for `internal/sbxdrv/**` and
  `e2e/fakesbx/**` (ask-first `sbxdrv`), `internal/state/**` (ask-first
  `gates`) and `schemas/**` (ask-first `formats`, the regenerated
  `state-project.v1.json`); `contracts` too if the state format's
  version moves. The argv comes from T205's recording, marked
  `synthetic: true` until A9 re-records it.
- Acceptance:
  - The spec pages of REQ-0997, REQ-0354, REQ-0355, REQ-0392 and 03 3.8
    carry the deny's text, and `go run ./tools/ci reqs` passes on them.
  - With a fake session whose workload descriptor declares the two
    pairs, `run` step 4 applies both denies after the allows, verifies
    each, and the typed member holds them; a reconcile cut short after
    the first deny converges on the next `run`; the allow reconcile of
    a later `run` leaves both denies in place.
  - `doctor` reports `egress-applied` when a recorded deny is absent
    from `sbx policy ls`, and passes when both are present.
  - A workload descriptor with no plain-HTTP pair applies no deny.
- Verify: `go run ./tools/ci reqs && go test -tags e2e ./e2e/... -run
  '^TestEgressDeny' -v`. The
  pattern names the `TestEgressDeny*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
