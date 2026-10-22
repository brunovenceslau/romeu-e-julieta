---
id: T163
title: "The manifest ignores unknown members, and a member julieta must honor bumps the protocol"
milestone: M1
module: julieta-core
fate: new
owns:
  - internal/manifest/manifest.go
  - internal/manifest/manifest_test.go
  - internal/manifest/testdata/**
  - docs/spec/03-formats.md
  - docs/adr/0041-readers-of-formats-between-binaries-ignore-unknown-members.md
  - docs/plan/tasks/T162-readers-of-formats-between-binaries-ignore-unknown-members.md
  - docs/handoff/decisions/01m5egnpxhheh7197j08zdr618-formats-evolve-by-the-rules-of-protobuf.md
needs:
  - T135
after:
  - T162
ask_first:
  - decisions
sitting: none
---

# T163 - The manifest ignores unknown members, and a member julieta must honor bumps the protocol

Goal: julieta's reader of `manifest.v1` skips a member it does not know,
as the other seven readers of formats between binaries do (T162), and
the spec states the one exception: a member that an older julieta could
not safely ignore bumps the protocol number, so the window of REQ-0219
refuses a julieta that is too old. Decided by the maintainer on
2026-10-22, scope this repository, written in
[ADR 0041](../../adr/0041-readers-of-formats-between-binaries-ignore-unknown-members.md)
(Decision) and in 03 3.6. It closes the open question that T162 left in
the ADR and the spec.

Planned on 2026-10-22 from the code at `6d804131`. The decoder
(`internal/manifest/manifest.go`, `Decode`) used
`RejectUnknownMembers(true)` and then required the document to be the
bytes `Encode` writes. Dropping the first alone would make every
document with an extra member fail the second, so `Decode` compares
`Encode` of the decoded value with the document without the members the
format's types do not name (`withoutUnknownMembers`). That keeps the
canonical-form rule for every member the reader knows: a missing or null
member, another order, spacing, escape or number spelling is still
refused, and so is a member of another case beside a missing real one.
The content of `run` is open and is not looked into; its members are
the value, covered by `runDigest`.

The JSON Schema `schemas/manifest.v1.json` is generated from the Go type
and keeps `additionalProperties: false`: it describes the manifest this
build writes, as T162 left the other schemas. The fixture
`invalid-unknown-member.json` stays under that name, as T162 left the
others: the schema check requires a fixture so named not to validate,
and `Decode` now reads it.

- Implements: REQ-0163 (03 (opening)), REQ-1257 (03 3.6).
- Operator: no
- Acceptance:
  - A manifest with a member the format does not name at every level of
    its types (the top, each repository, `salvage`, `salvageRun`)
    decodes to the same value as the one without it
    (`TestDecodeIgnoresAnUnknownMember`, also through `DecodeEnv`).
  - A skipped member cannot change a known one (a name of another case,
    before or after, at the top and in a repository), carries an escape
    without effect, and cannot hide a duplicate, a nest past the depth
    limit, an excess over the size cap, a known member that is missing,
    null or in another spelling; no raw control character reaches an
    error (`TestDecodeHostileUnknownMembers`).
  - The protocol window is as before with an unknown member present:
    N-2, N+1 and zero are `manifest-protocol` (`RJ-342`, exit 2), N is
    read (`TestUnknownMemberKeepsTheProtocolWindow`; the table of
    `TestProtocolWindow` is unchanged).
  - 03 3.6 states the must-understand rule (REQ-1257), REQ-0159,
    REQ-0160 and REQ-0163 no longer except the manifest, and ADR 0041
    records the decision, its date and scope, and what it costs.
- Verify: `go test ./internal/manifest/... ./internal/cmd/julieta/...`,
  and `go run ./tools/ci all`.
