---
id: T217
title: "The stopped start by exec and the create's kit and secret lists"
milestone: M4
module: romeu-cli
fate: new
owns:
  - internal/preflight/stopped_start*.go
  - internal/sbxdrv/env_exec_start*.go
  - e2e/stopped_start_test.go
needs:
  - T203
  - T066
  - T067
  - T068
after: []
ask_first:
  - gates
  - formats
  - sbxdrv
sitting: none
---

# T217 - The stopped start by exec and the create's kit and secret lists

Goal: the code of the lift decided on 2026-11-08. A stopped sandbox
whose recreate digests differ is started with `sbx env exec <dir> --
true` by `run`, `salvage`, `rm`, `recreate`, `retire` and the
fingerprint read, and refused by `run` and `pull` when the spec removed
a kit or a secrets binding since the create; the create records the kit
ids and the secret names that tell it.

Added on 2026-11-08 (scope this repository), as decided on 2026-11-08:
T210's measurement found that `sbx env exec <dir> -- true` starts a
stopped sandbox with the configuration it was created with, applies
nothing of the changed env file and asks nothing, for six kinds (the
in-place apply measurement note, sitting 3). The spec text is in
04 4.2 (`run` step 3, REQ-1405, REQ-1406; the `salvage` row, REQ-1387;
REQ-1377), 03 3.8 (REQ-1403, REQ-1404) and 05 5.4 (REQ-1389,
REQ-1390). It replaces the refusal that T050, T066, T067, T068, T069,
T075 and T203 describe for a stopped sandbox whose digests differ;
where their text still says that such a sandbox is refused, this
task's text wins.

1. The state members `generations[].createdKits` and
   `generations[].createdSecrets` (03 3.8), written by `run`'s create in
   the same whole-file write as `createdWith`, names only, and the
   regenerated `state-project.v1.json`. An adopted generation and one
   written before the members lack them. T215 reads the same kit list
   as the creating render's kit set.
2. One function, beside T203's refusal helper, that decides for a
   stopped sandbox whose digests differ: start (no kit id and no secret
   name of the lists missing from the live render), refuse for `run`
   and `pull` (a removal, or a list absent), or refuse for every
   command (an adopted generation). The start is the sbxdrv call
   `sbx env exec <dir> -- true`, which the preflight's comparison of
   REQ-1382 follows; a failed `sbx-env-base` or `sbx-global-secret`
   check stops it (REQ-1390).
3. `run` warns `recreate-needed` after the start, saying the change
   waits for the next recreate, with the hint `romeu recreate <name>`
   (REQ-1377), and goes on as for a running sandbox. The re-read of
   REQ-1389 before each exec applies to adopted generations and to a
   removal only.

- Implements: REQ-1403 (03 3.8), REQ-1404 (03 3.8), REQ-1405 (04 4.2),
  REQ-1406 (04 4.2).
- Operator: yes, the approval lines for `internal/state/**` (ask-first
  `gates`), `schemas/**` (ask-first `formats`) and `internal/sbxdrv/**`
  (ask-first `sbxdrv`).
- Acceptance:
  - A unit table covers the decision of item 2: equal lists, a kit
    added, a kit removed, a secret added, a secret removed, the lists
    absent, and an adopted generation, for `run`, `pull` and `salvage`.
  - Against the fake sbx, `run` on a stopped sandbox whose digests
    differ and whose lists hold makes `sbx env exec <dir> -- true`,
    then the steps of a running sandbox, and never `sbx env run` with
    the changed file; with a kit removed it exits 5 with
    `recreate-needed` (`RJ-305`) and records no starting call;
    `recreate` on the same sandbox starts it for the salvage.
  - The create writes both lists, and a state file without them reads
    back with the lists absent.
- Verify: `go test ./internal/preflight/... -run '^TestStoppedStart' -v`
  and `go test ./e2e/ -run '^TestStoppedStart' -v`. The patterns name
  the functions this task adds.
