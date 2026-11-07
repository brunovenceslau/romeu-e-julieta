---
id: T188
title: "`render`: a kit error the spec causes is told apart and exits 2"
milestone: M3
module: render
fate: new
owns:
  - internal/render/kits.go
  - internal/render/kits_test.go
  - internal/cmd/romeu/sync/step6_render.go
  - internal/cmd/romeu/sync/step6_render_test.go
  - internal/cmd/romeu/sync/errors.go
  - internal/cmd/romeu/sync/step2_fetch.go
  - internal/cmd/romeu/sync/step2_fetch_test.go
  - e2e/sync_test.go
  - docs/spec/04-cli.md
needs:
  - T038
  - T043
after:
  - T055
  - T185
ask_first:
  - contracts
sitting: none
---

# T188 - `render`: a kit error the spec causes is told apart and exits 2

Goal: `sync` exits 2 on every kit error the spec or the config commit
causes, as it does on its other spec errors. Step 6 (T043, slice C)
maps to exit 2 only the errors render names with a sentinel
(`ErrPersonalCapability`, `ErrPersonalKind`) and `oci.ErrInvalid`; an
unknown kit id, a kit without its descriptor and a personal kit over
`MaxPersonalKit` (64 MiB) are plain errors of `internal/render`, which
T043 does not own, so they exit 1 like an I/O failure. Decided on
2026-10-29 (scope this repository): a task of its own, after the slice
that links `sync` into the binary, since it edits step 6 and
`errors.go` of `sync`, and after T185, which edits 04 too.

1. `internal/render` exports `ErrKitUnknown` (a kit id the spec names
   that no product kit and no personal kit dir of the config commit
   matches), `ErrKitDescriptorMissing` (a personal kit dir without its
   descriptor; a product kit without one is a build fault and stays
   exit 1) and `ErrPersonalTooLarge`, wrapped where the plain errors
   are made today; every other error of the materializer stays as it
   is.
2. `kitRefused` of step 6 maps the three to exit 2 with the slug
   `kit-refused`, which it already raises as an id-less usage error
   for `ErrPersonalCapability`, `ErrPersonalKind`, `oci.ErrInvalid`
   and a git read gitsafe refuses or that exits non-zero: every cause
   shares the one row that 04 4.4 gains, with its id, naming each
   cause and `sync` as the command that raises it; the `errors.go` of
   `sync` gains the error (contracts) in place of the `usage` calls.
3. A kit error that is none of these, such as a file read that fails
   on the host, stays exit 1 and names the file.

Re-plan (2026-10-30): takes from T043 the exit of a config origin
whose objects the hardened fetch of step 2 refuses, found by the e2e of
I27 in T043's last slice; it owns `step2_fetch.go` of `sync`, its test
and `e2e/sync_test.go` for it, and stays `after` T185, which edits
`step2_fetch.go` too. Proposed in that slice on 2026-10-30, for the
operator to confirm at review; scope this repository.

Amended on 2026-11-07 (the sbx 0.51 and kit schema m.12 amendments,
as decided on 2026-11-07): a personal kit that declares `dockerd@1` is
refused, beside docker-auth and host-port, until A19 records the
elevation and Q33 is amended or a 05 5.4 row accepts it by name
(REQ-0070, REQ-0514, REQ-0574): `refusedCapability` of
`internal/render/kits.go` gains a `dockerd` case after `host-port`, so
the refusal is `ErrPersonalCapability` and exits 2 with `kit-refused`
as the other refused capabilities do. `privileged@1` stays admitted in a
personal kit. The exit of the other kit errors of REQ-0573 and REQ-0574
stays T038's and T043's; this task cites only the dockerd refusal, and
REQ-0514 (05 5.2), which says the same, keeps its five implementers.

- Implements: REQ-0070 (01 1.4), REQ-0574 (06 6.1).
- Operator: no.
- Acceptance:
  - A spec naming a kit id that no product kit and no personal kit dir
    matches makes `sync` exit 2 with `kit-refused` naming the id; a
    personal kit dir without its descriptor, and one over
    `MaxPersonalKit` (lowered by the test), the same, each naming the
    kit.
  - The causes `kit-refused` carries today (a refused personal kind or
    capability, a descriptor the grammar refuses, a git read gitsafe
    refuses) still exit 2, now with the row's id.
  - A config origin that sends an object `transfer.fsckObjects` refuses
    (a `.GIT`, `.git` with a U+200C or `..` name) makes step 2 exit 2
    with an id that names the refusal, in place of the error `internal`
    (`RJ-348`), exit 1, of today; the cases of `TestSyncHostileKit`
    assert that exit (from T043, 2026-10-30).
  - A kit file whose read fails on the host for another reason exits 1
    naming the file.
  - A unit case of `refusedCapability` returns `dockerd` for a personal
    kit whose descriptor declares `dockerd@1`, and `""` for one that
    declares only `privileged@1`; an e2e case of `sync` with such a
    personal kit exits 2 with `kit-refused` naming the kit and
    `dockerd`, and promotes nothing.
  - The row of `kit-refused` in 04 4.4 has an id, the contract test of
    `sync` checks it, and `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/render/ ./internal/cmd/romeu/sync/`.
