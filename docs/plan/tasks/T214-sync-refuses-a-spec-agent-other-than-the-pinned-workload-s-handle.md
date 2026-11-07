---
id: T214
title: "Sync refuses a spec agent other than the pinned workload's handle"
milestone: M4
module: romeu-cli
fate: new
owns:
  - internal/oci/handle*.go
  - internal/oci/capabilities*.go
  - internal/cmd/romeu/sync/rule_agent*.go
  - internal/oci/normalization*.go
  - internal/cmd/romeu/sync/errors.go
needs:
  - T213
after:
  - T208
ask_first:
  - contracts
sitting: none
---

# T214 - Sync refuses a spec agent other than the pinned workload's handle

Goal: a project spec whose `agent` differs from the handle the pinned
workload declares is a `sync` error, so an `agent` cannot bring a
built-in agent of sbx 0.51, a v3 kit romeu does not pin.

Added on 2026-11-07 (scope this repository), as decided on 2026-11-07
(the 05 5.4 row of REQ-1392, RX-10): no open task owns the spec's rules
against the workload descriptor (T021 is merged), so the row gets this
task. It needs T213 for `internal/oci`, which both edit: the handle
enters the normalized set beside the engine grant.

1. `internal/oci` reads the agent handle the workload descriptor
   declares into the normalized set (A12 records the field; until block
   A, the sitting fact of T208, marked with its source).
2. A sync rule, registered from its own file (rule 4 of
   `docs/plan.md`), compares the spec's `agent` with that handle and
   fails `sync` naming `agent`, the spec's value and the handle.

Amended on 2026-11-07 (RC-8, RP-11, RC2): every check that stops a
command has an error row with an id (04 4.1, Warnings), and T188 gives
`kit-refused` one, so this refusal is one more cause of T188's
`kit-refused` row, exit 2, declared in the `errors.go` of `sync`
(ask-first `contracts`); the cause is named in that row of 04 4.4 by a
docs fixup before this task builds. T214 already comes after T188:
T213 comes after T066, which needs T200, which comes after T188. Since
the handle enters the normalized set, the task raises the
normalization version of REQ-0240 that T213 introduces, the constant
in `internal/oci/normalization.go`, so every cached record is derived
again with the handle; it needs T213, which creates that file.
It comes after T208, whose pin is the workload that declares the
handle, so no sync runs the rule against a pin without one; where
T205's decision defers T208, it gives this task the same fate and
milestone.

- Implements: REQ-1392 (05 5.4), REQ-0240 (03 3.8).
- Operator: yes, the approval line for `internal/cmd/romeu/sync/errors.go`
  (ask-first `contracts`), for the new cause of `kit-refused`.
- Acceptance:
  - A spec whose `agent` equals the pinned workload's handle syncs; one
    that names another agent makes `sync` exit 2 with `kit-refused`
    and its id, naming both values, before gate 2.
  - The handle is read from the descriptor fixture of the pinned
    workload, and a descriptor without the field is `upstream-shape`.
  - The normalization version is one above T213's, and a record cached
    with T213's version is derived again and then carries the handle.
- Verify: `go test ./internal/oci/... ./internal/cmd/romeu/sync/... -run
  '^(TestWorkloadHandle|TestRuleAgent)' -v`. The pattern names the
  functions this task adds, and the reviewer reads them in the `-v`
  output (10 10.8).
