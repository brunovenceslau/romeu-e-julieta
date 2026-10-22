---
id: T163
title: "The manifest ignores unknown members, and a member julieta must honor bumps the protocol"
milestone: M1
module: julieta-core
fate: merged
owns:
  - internal/manifest/manifest.go
  - internal/manifest/manifest_test.go
  - internal/manifest/testdata/**
  - internal/cmd/julieta/setup/state.go
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
merged: 2026-10-22
pr: 242
---

# T163 - The manifest ignores unknown members, and a member julieta must honor bumps the protocol

Merged on 2026-10-22 in pull request #242. The text below is the one
written when it was built.

Goal: julieta's reader of `manifest.v1` skips a member it does not know,
as the other seven readers of formats between binaries do (T162), and
the spec states the one exception: a member that an older julieta could
not safely ignore bumps the protocol number, so the window of REQ-0219
refuses a julieta that is too old. Decided on
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
  - A test of the shape of `Manifest` (every field's JSON tag and Go
    type, recursively) fails on an added, removed or retyped member and
    tells the author to update it and to state in the pull request
    whether the member is must-understand or ignorable
    (`TestManifestShapeIsFingerprinted`). The expected shape is in the
    test file, so `internal/manifest/versions.go` (the `contracts`
    surface) is not touched.
  - The hook for the rule is two things: the doc comment of the
    `Manifest` type, which says that a member an older julieta must
    honor bumps `Protocol` (REQ-1257) and that review asks which kind
    each new member is, and the fingerprint test above, which forces the
    question.
  - 03 3.6 states the must-understand rule (REQ-1257), REQ-0159,
    REQ-0160 and REQ-0163 no longer except the manifest, and ADR 0041
    records the decision, its date and scope, and what it costs.
- Verify: `go test ./internal/manifest/... ./internal/cmd/julieta/...`,
  and `go run ./tools/ci all`.
