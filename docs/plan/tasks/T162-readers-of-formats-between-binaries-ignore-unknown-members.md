---
id: T162
title: "Readers of formats between binaries ignore unknown members"
milestone: M1
module: julieta-core
fate: new
owns:
  - internal/handoff/decode.go
  - internal/handoff/decode_test.go
  - internal/handoff/testdata/**
  - internal/memstore/entry.go
  - internal/memstore/entry_test.go
  - internal/memstore/testdata/**
  - internal/salvage/snapshot.go
  - internal/salvage/snapshot_test.go
  - internal/cmd/julieta/status/report.go
  - internal/cmd/julieta/status/status_test.go
  - internal/cmd/julieta/setup/report.go
  - internal/cmd/julieta/setup/setup_test.go
  - internal/cmd/julieta/handoff/handoff.go
  - internal/cmd/julieta/handoff/handoff_test.go
  - internal/cmd/julieta/memory/memory.go
  - internal/cmd/julieta/memory/memory_test.go
  - docs/spec/03-formats.md
  - docs/spec/04-cli.md
  - docs/plan/tasks/T154-julieta-status-lists-the-recorded-hook-failures.md
  - internal/manifest/manifest.go
  - docs/handoff/decisions/01m5egnpxhheh7197j08zdr618-formats-evolve-by-the-rules-of-protobuf.md
  - docs/handoff/pending/01m5egnpyfywadcsxpy0naz3hj-readers-of-formats-between-binaries-ignore-unknown-members.md
needs:
  - T051
  - T054
  - T035
  - T059
  - T060
  - T154
after:
  - T021
  - T135
  - T155
  - T160
ask_first:
  - decisions
sitting: none
---

# T162 - Readers of formats between binaries ignore unknown members

Goal: the readers of the formats that one binary writes and another
reads skip a member they do not know, as the decision of 2026-10-21
says ("Formats evolve by the rules of protobuf",
01m5egnpxhheh7197j08zdr618), and the readers of files a person writes
stay strict. It closes the pending note "Readers of formats between
binaries ignore unknown members" (01m5egnpyfywadcsxpy0naz3hj). The rule
is recorded in
[ADR 0041](../../adr/0041-readers-of-formats-between-binaries-ignore-unknown-members.md),
and REQ-0159, REQ-0160, REQ-0163, REQ-0261 and REQ-0271 are amended.

Planned on 2026-10-22. The formats and readers below were measured from
the code at `c8dab846`, by a search of every `RejectUnknownMembers` and
`KnownFields` outside tests, then of each reader's callers.

Formats one binary writes and another reads, and what changed in the
reader (each decoder keeps its duplicate-member, size, UTF-8 and
schema rules):

| Format | Writer | Reader | Change |
|---|---|---|---|
| `julieta-status/v1` (`--json`) | `internal/cmd/julieta/status/status.go:66` | `internal/cmd/julieta/status/report.go:150` | `RejectUnknownMembers` dropped |
| `julieta-setup/v1` (`--json`) | `internal/cmd/julieta/setup/setup.go:127` | `internal/cmd/julieta/setup/report.go:90` | the same |
| `julieta-handoff/v1` (`--json`) | `internal/cmd/julieta/handoff/handoff.go:184`, `write.go:124` | `internal/cmd/julieta/handoff/handoff.go:68` | the same |
| `julieta-memory/v1` (`--json`) | `internal/cmd/julieta/memory/memory.go:177` | `internal/cmd/julieta/memory/memory.go:72` | the same |
| `snapshot/heads.json` (`snapshot/v1`) | `internal/salvage/snapshot.go:93` (`EncodeHeads`; called at `internal/hooks/actions/snapshot.go:178` and `disabled.go:91`) | `internal/salvage/snapshot.go:124` (`DecodeHeads`) | `RejectUnknownMembers` dropped |
| handoff file (`handoff/v1`) | `internal/handoff/decode.go:170` (`Encode`; called at `internal/cmd/julieta/handoff/write.go:181`) | `internal/handoff/decode.go:202` (`Decode`) | `KnownFields(true)` dropped; `exactKeys` became `requireKeys`, which still requires every key of v1 and no longer refuses another, in the front matter and in a repo |
| memory entry (`memory-entry/v1`) | `internal/memstore/entry.go:190` (`Encode`; called at `internal/memstore/write/write.go:292`) | `internal/memstore/entry.go:229` (`Decode`) | `KnownFields(true)` dropped; `checkKeys` refuses a duplicate or a missing key and no longer an unknown one |

Measured and left strict:

- The files a person writes: `.github/ask-first.yaml`
  (`tools/ci/internal/askfirst/askfirst.go:84`), `tools/ci/prose.yaml`
  (`tools/ci/prose/prose.go:64`), `tools/ci/denylist.yaml`
  (`tools/ci/names/names.go:100`), the license allowlist
  (`tools/ci/internal/license/modules.go:72`), `kits/pins.yaml`
  (`tools/kitpin/pins/pins.go:116`) and the mutants file
  (`tools/mutants/mutants.go:54`). The tests that hold it already
  exist: `askfirst_test.go:31`, `prose_test.go:108`, `names_test.go:198`,
  `modules_test.go:58`, `pins_test.go:62` and `mutants_test.go:48` each
  refuse an unknown key. The project spec and host settings readers are
  not built yet; their tasks inherit the rule of REQ-0163.
- The state julieta keeps for itself, written and read by the one
  binary: `setup.json` (`internal/cmd/julieta/setup/state.go:143`), the
  hooks record (`internal/cmd/julieta/handoff/state.go:59`), the system
  hooks path (`internal/hooks/system.go:66`) and the hook failures
  (`internal/hooks/failures.go:112`). A newer member there is a reader
  of the same binary, so the rule does not reach it.
- The manifest (`internal/manifest/manifest.go`, decoded at
  `internal/cmd/julieta/setup/state.go:74`): romeu writes it and julieta
  reads it, so it is a format between binaries, but it has its own
  protocol window (REQ-0219) and the decision note lists it with the
  strict readers. Stopped on and left strict here; the maintainer
  decided on 2026-10-22 that it skips an unknown member too, with the
  must-understand exception, and T163 built it.

The JSON Schemas in `schemas/` keep `additionalProperties: false`: they
describe the document this version writes and the examples CI validates.
The fixtures `invalid-unknown-member.json` under the status, setup and
memory packages stay for that check, and no longer for the decoder; the
handoff `--json` decoder has no fixture file.

- Implements: REQ-0159 (03 (opening)), REQ-0163 (03 (opening)), REQ-0261 (03 3.13), REQ-0271 (04 4.1).
- Operator: no.
- Acceptance:
  - For each of the seven readers above, a test gives it a document or
    file to which a newer writer added a member, at every level of the
    document, and gets the same value as for the document without it:
    `TestDecodeIgnoresAnUnknownMember` in the four `--json` packages,
    `TestDecodeHeadsIgnoresAnUnknownMember`, and
    `TestDecodeIgnoresAnUnknownKey` for the handoff file (front matter
    and repo) and the memory entry.
  - Each of them still refuses a duplicate member or key, another
    schema, and, for the files, a missing key; a newer version is still
    `format-newer`.
  - A member a reader skips cannot change a known member (a name of
    another case is another name, before or after the real one), carries
    an escape or control character without effect, and cannot hide a
    duplicate, a nest past the depth cap or an excess over the file cap
    from the rules that still apply; no raw control character reaches an
    error. `TestDecodeHostileUnknownMembers` in the four `--json`
    packages, `TestDecodeHeadsHostileUnknownMembers` and
    `TestDecodeHostileUnknownKeys` in `internal/handoff` and
    `internal/memstore` hold it. The memory entry decoder refused a
    duplicate at the top level only; it now refuses one at any depth, as
    the handoff decoder does. Measured and left: the `--json` decoders
    have no size cap of their own.
  - The readers of files a person writes still refuse an unknown key, by
    the tests named above. The manifest decoder refused one when T162
    merged; T163 changed it.
  - REQ-0163 states the rule and names the two groups; the ADR states
    its scope and what it left strict.
- Verify: `go test ./internal/handoff/... ./internal/memstore/...
  ./internal/salvage/... ./internal/cmd/julieta/...`, and `go run
  ./tools/ci all`.
