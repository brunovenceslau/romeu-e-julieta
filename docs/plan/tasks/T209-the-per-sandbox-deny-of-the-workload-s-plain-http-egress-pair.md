---
id: T209
title: "The per-sandbox deny of the workload's plain-HTTP egress pair"
milestone: M4
module: romeu-cli
fate: new
owns:
  - internal/cmd/romeu/run/egress_deny*.go
  - internal/cmd/romeu/doctor/egress_deny*.go
  - e2e/egress_deny_test.go
needs:
  - T205
  - T066
after:
  - T047
  - T203
ask_first: []
sitting: none
---

# T209 - The per-sandbox deny of the workload's plain-HTTP egress pair

Goal: where T205 measured that a per-sandbox deny on `deb.debian.org:80`
and `dhi.io:80` beats the allow rule sbx provisions from the
`docker.io/sbx/claude` descriptor and survives a swap and a recreate,
`run` step 4 applies and verifies that deny with the egress reconcile,
and `doctor` reports its absence.

Added on 2026-11-06 (scope this repository): the plain-HTTP row of 05
5.4 (REQ-1374) and Q2 of the index. This task exists only on the deny
branch of T205's decision; on the other branches it is closed with the
reason in its file, and T208 moves the pin without it.

1. The reconcile (REQ-0354): beside the allow rules, `run` step 4
   applies `sbx policy deny network --sandbox <name> <host>:<port>` for
   each pair the workload descriptor declares in its runtime egress
   over plain HTTP (the set of 07 7.5 that only a descriptor declared,
   REQ-1316, filtered to port 80), verifies each with `sbx policy check
   network`, and records each in `egressApplied` as a deny, before and
   after the calls as the allow rules are (REQ-0355).
2. `doctor` (REQ-0392): the `egress-applied` row compares the deny
   rules too, so a deny that went missing, after a swap or by hand, is
   a finding.

- Implements: REQ-1374 (05 5.4), REQ-0354 (04 4.2), REQ-0355 (04 4.2),
  REQ-0392 (04 4.2).
- Operator: no (the argv comes from T205's recording, marked
  `synthetic: true` until A9 re-records it).
- Acceptance:
  - With a fake session whose workload descriptor declares the two
    pairs, `run` step 4 applies both denies after the allows, verifies
    each, and `egressApplied` holds them as denies; a reconcile cut
    short after the first deny converges on the next `run`.
  - `doctor` reports `egress-applied` when a recorded deny is absent
    from `sbx policy ls`, and passes when both are present.
  - A workload descriptor with no plain-HTTP pair applies no deny.
- Verify: `go test -tags e2e ./e2e/... -run '^TestEgressDeny' -v`. The
  pattern names the `TestEgressDeny*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
