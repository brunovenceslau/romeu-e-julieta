---
id: T189
title: "`render`: the promotion fails closed on the candidate digest and recopies an edited kit dir"
milestone: M3
module: render
fate: new
owns:
  - internal/render/promote.go
  - internal/render/promote_test.go
  - internal/cmd/romeu/sync/step8_gate.go
  - internal/cmd/romeu/sync/step8_gate_test.go
  - docs/spec/01-system-model.md
  - docs/spec/04-cli.md
needs:
  - T039
  - T043
after:
  - T188
ask_first:
  - spec
sitting: none
---

# T189 - `render`: the promotion fails closed on the candidate digest and recopies an edited kit dir

Goal: the bytes a promotion writes are the ones gate 2 showed, checked
where the copy happens, and `--overwrite-drift` replaces an edited kit
dir as REQ-0342 says it replaces an edited live file. Two findings of
the review of T043's slice D (2026-10-29, scope this repository), both
in `internal/render/promote.go`, which T043 does not own:

1. `Live.Promote` takes the expected files digest of the candidate and
   fails closed on the bytes it reads: a candidate whose files differ
   from the digest is `ErrNotApproved` and nothing is written. An
   approved candidate (after `y`) is left for the next render to delete,
   since the candidate machine has no row from approved to declined
   (REQ-0082); a rendered one, on the no-prompt path, is declined, as
   step 8's own check does today. Step 8 of `sync` passes the digest
   step 6 kept; its check before the call, which narrows the window
   between the answer and the copy without closing it, goes.
2. Under `--overwrite-drift`, a live kit dir that differs from its
   digest is removed and copied again from the candidate; without the
   flag it stays `ErrKitDirDiffers`, as REQ-0086 step 2 says (a dir
   that exists is verified, never rewritten). The dir is content
   addressed, so the copy is the one a first promotion would make.
   REQ-0086 gains the sentence, and the drift row of REQ-0342 (04 4.2
   step 7) says the flag covers a kit dir.

- Implements: REQ-0086 (01 1.6), REQ-0342 (04 4.2).
- Operator: approves the `spec` surface for the sentence of REQ-0086.
- Acceptance:
  - A candidate dir rewritten between the answer and the promotion (a
    test rewrites one file after the digest is taken) fails with
    `ErrNotApproved`, every live file is byte for byte as before (I7),
    and the project record names no promotion; after `y` the dir is
    left and the next `sync` deletes it, and on the no-prompt path it
    is declined and deleted in the same run.
  - A live kit dir with one edited file: `sync` exits 4 naming the dir;
    with `--overwrite-drift` the dir holds the kit's files again, byte
    for byte with their modes, and the run reports the lost edit among
    the warnings that close the output.
  - `go run ./tools/ci reqs` passes.
- Verify: `go test -race ./internal/render/ ./internal/cmd/romeu/sync/`.
