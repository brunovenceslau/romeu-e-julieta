---
id: T213
title: "Gate 2 shows the workload's engine: the start-docker annotation, its interim refusal and the root-equivalent mark"
milestone: M4
module: oci
fate: new
owns:
  - internal/oci/**
  - internal/state/records.go
  - internal/state/load.go
  - internal/state/records_test.go
  - internal/state/load_test.go
  - internal/gate/diff*.go
  - e2e/gate_engine_test.go
needs: []
after:
  - T066
ask_first:
  - gates
  - formats
  - contracts
sitting: none
---

# T213 - Gate 2 shows the workload's engine: the start-docker annotation, its interim refusal and the root-equivalent mark

Goal: `internal/oci` reads the workload image's annotation
`com.docker.sandboxes.start-docker` and gate 2 shows the engine it
starts as the `dockerd@1` grant; until that lands, `sync` refuses a
workload that carries the key; the descriptor cache cannot keep a set
read before the change; and gate 2's summary marks `privileged@1` from
any kit as root-equivalent.

Added on 2026-11-07 (scope this repository), as decided on 2026-11-07
(decision S, items 1 and 3 of the later decisions, RX-9 and RX-11):
`internal/oci` has no open owner (T032 is merged), so REQ-1385 and the
05 5.4 rows of REQ-1391 and REQ-1396 get this task. T208 needs it: the
pin of `docker.io/sbx/claude`, whose engine starts through the
annotation, does not move before gate 2 shows the grant. It comes
after T066 for `internal/state/records.go`, which both edit.

1. The interim refusal, the first slice (REQ-1391): `sync` refuses any
   workload whose manifest carries the start-docker key, naming the key
   and the image; it may merge alone first (12 12.9).
2. The normalization (REQ-1385): `true` enters the normalized set as
   the `dockerd@1` grant, privileged plus private storage at
   `/var/lib/docker`; `false` adds nothing; any other value fails
   `sync` closed as REQ-0573 does. The refusal of item 1 then goes.
3. The cache (REQ-1391): each descriptor cache entry carries the
   normalization version of `internal/oci`, one constant in
   `internal/oci/normalization.go`; an entry of another version
   is a cache miss, which `sync` parses again (03 3.8, the regenerated
   `state-descriptor.v1.json`).
4. The gate 2 summary (REQ-1396, 01 1.4 REQ-0071): a `privileged@1`
   grant from any kit is marked root-equivalent, among the changed
   capabilities listed first; the workload's `dockerd@1` grant is
   listed there as a changed capability and carries the mark only if
   the spec gives it one (REQ-0071 names `privileged@1` alone; amended
   on 2026-11-07, RC-4).

Amended on 2026-11-07 (RC-3, RC-4, RP-12): the task also builds the
amended REQ-0068 (the start-docker entry of the normalized list),
REQ-0071 (the root-equivalent mark) and REQ-0240 (the `normalization`
member of the cache record), so it cites them. A cache record whose
`normalization` differs from the running romeu's, or that lacks the
member, which is every record written before this task, is a cache
miss and is derived again, never read as it is and never
`record-unreadable`.

- Implements: REQ-1385 (06 6.1), REQ-1391 (05 5.4), REQ-1396 (05 5.4),
  REQ-0068 (01 1.4), REQ-0071 (01 1.4), REQ-0240 (03 3.8).
- Operator: yes, the approval lines for `internal/state/**` and
  `internal/gate/**` (ask-first `gates`), `schemas/**` (ask-first
  `formats`) and any `errors.go` the refusal adds an id to (ask-first
  `contracts`).
- Acceptance:
  - With item 1 alone, a workload manifest that carries the key makes
    `sync` exit 2 naming the key; one without it syncs as before.
  - With item 2, a workload whose annotation is `true` normalizes to a
    set with the `dockerd@1` grant, `false` to one without it, and
    `yes` makes `sync` exit 2 as for a malformed descriptor.
  - A cache entry written with an older normalization version, and one
    without the `normalization` member, are each read as a miss and
    parsed again from the digest, never `record-unreadable`, and gate 2
    then shows `dockerd@1`.
  - The gate 2 diff of a project whose personal kit declares
    `privileged@1` marks that grant root-equivalent and lists it among
    the changed capabilities first; the diff of a project whose
    workload gains the engine lists the `dockerd@1` grant there.
- Verify: `go test ./internal/oci/... ./internal/state/... ./internal/gate/...
  -run '^(TestStartDocker|TestDescriptorCacheVersion|TestDiffRootEquivalent)'
  -v && go test -tags e2e ./e2e/... -run '^TestGateEngine' -v`. The
  patterns name the functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
