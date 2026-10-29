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
  - docs/spec/04-cli.md
needs:
  - T038
  - T043
after:
  - T185
ask_first:
  - contracts
  - spec
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
`errors.go` of `sync`.

1. `internal/render` exports `ErrKitUnknown` (a kit id the spec names
   that no product kit and no personal kit dir of the config commit
   matches), `ErrKitDescriptorMissing` and `ErrPersonalTooLarge`,
   wrapped where the plain errors are made today; every other error of
   the materializer stays as it is.
2. `kitRefused` of step 6 maps the three to exit 2 with the slug
   `kit-refused`; 04 4.4 gains the row of `kit-refused` with its id,
   naming each cause and `sync` as the command that raises it; the
   `errors.go` of `sync` gains the error (contracts).
3. A kit error that is none of these, such as a read that fails, stays
   exit 1 and names the file.

- Implements: none: the exit of the kit errors of REQ-0573 and REQ-0574
  (06 6.1), which T038 and T043 implement.
- Operator: no.
- Acceptance:
  - A spec naming a kit id that no product kit and no personal kit dir
    matches makes `sync` exit 2 with `kit-refused` naming the id; a
    personal kit dir without its descriptor, and one over
    `MaxPersonalKit` (lowered by the test), the same, each naming the
    kit.
  - A kit file whose read fails for another reason exits 1 naming the
    file.
  - The row of `kit-refused` in 04 4.4 has an id, the contract test of
    `sync` checks it, and `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/render/ ./internal/cmd/romeu/sync/`.
