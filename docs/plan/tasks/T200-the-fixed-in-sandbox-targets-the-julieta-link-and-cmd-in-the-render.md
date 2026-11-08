---
id: T200
title: "The fixed in-sandbox targets, the julieta link and cmd in the render"
milestone: M3
module: render
fate: new
owns:
  - internal/render/env*.go
  - internal/render/bin*.go
  - internal/render/json*.go
  - internal/render/drift*.go
  - internal/render/promote*.go
  - internal/render/types.go
  - internal/render/render_test.go
  - internal/render/testdata/**
  - internal/cmd/romeu/sync/rule_manifest_size*.go
  - internal/cmd/julieta/spec/manifest*.go
needs:
  - T037
  - T038
  - T039
  - T174
after:
  - T188
  - T189
ask_first:
  - formats
sitting: none
---

# T200 - The fixed in-sandbox targets, the julieta link and cmd in the render

Goal: the rendered env file names a fixed in-sandbox `target` for every
additional workspace but the primary clone, writes `entrypoint` and
`cmd` and no `skills` key, and `.romeu/bin` carries a `julieta` link beside the arch binary,
so no per-project path reaches a kit, and no host path reaches a
sandbox but the primary clone's.

Added on 2026-11-06 (scope this repository), with the specification
amended first: the sbx 0.51 and kit schema m.12 amendments of 02 2.3,
02 2.5, 03 3.3, 03 3.7 and 06 6.3. Until then the mounts kept their
host paths, the `julieta-claude` kit took the path through the reserved
kit arg `julietaBin`, and `romeu run` attached by an `sbx env exec -it`
of `layout up`. No sandbox of romeu exists before T066, so nothing
migrates.

1. Targets (REQ-1355, REQ-1357): `./.romeu/bin` at `/opt/julieta/bin`,
   `./memory/_shared` at `/var/lib/julieta/memory/_shared` and
   `./memory/<dir>` at `/var/lib/julieta/memory/<dir>`, each as
   `additionalWorkspaces[].target`; the primary clone keeps its host
   path (clone mode). The manifest's `repos[].memory` and
   `sharedMemory` carry the targets (03 3.6, REQ-0221).
2. `entrypoint: [/opt/julieta/bin/julieta]` and `cmd: [layout, up]`
   (REQ-1370) in every rendered env file, and no `skills` key
   (REQ-0200); amended on 2026-11-07, below.
3. The `julieta` link (REQ-1365): on promotion `.romeu/bin/julieta` is a
   relative symbolic link to `julieta-linux-<GOARCH>`; `render.json`
   records it as `".romeu/bin/julieta": "link:julieta-linux-<GOARCH>"`
   (03 3.7, REQ-0227); the drift check reads it with `readlink` and
   never follows it; a promotion replaces it by rename.
4. The reserved kit arg `julietaBin` goes: the render passes no arg to
   the `julieta-claude` kit (06 6.4 item 2), and the sync error on a
   spec that set it goes with it (T043's acceptance line is history).

Amended on 2026-11-07 (as decided on 2026-11-07, C1-12): item 4 leaves
this task. The render already passes `julietaBin` only to a descriptor
that declares it, so once T201 removes the declaration from the kit the
arg is never rendered, and no sandbox of romeu exists before T066;
T212 removes the dead branch, the sync rule and the constant after T200
and T201, so this task takes no edge to T201 and the critical path
stays as it was. The memory targets are exported from `render`
(`render.MemoryTarget(dir)`, `render.SharedMemoryTarget`,
`render.BinTarget`), and the two manifest builders that measure the
manifest before `run` exists, the size rule of `sync`
(`rule_manifest_size.go`) and the stand-in of `julieta spec validate`
(`internal/cmd/julieta/spec/manifest.go`), switch to them; that the
manifest `run` encodes carries them is T066's acceptance (V2.7). T174
moves from `after` to `needs`: the `_shared` mount and `sharedMemory`
are T174's. The `link:` value of `render.v1`'s `files` map keeps the
schema's string type, so the `formats` check does not ask: the pull
request records the `formats` approval by name, and an older romeu
reads a newer `render.json` as drift (03 3.7, 03 3.12; V2.37).

Amended on 2026-11-07 (as decided on 2026-11-07, after the hand
measurement of sbx v0.51.0 that day; scope this repository): item 2's
`skills: "off"` and its `cmd`-only shape go. The env file refuses the
key (`unknown field "skills"`, exit 1), so REQ-1358 is retired and the
render writes no `skills` key (REQ-0200; the share it would have turned
off is a 05 5.4 row and `doctor`'s `skills-shared`, T191); and a `cmd`
with no `entrypoint` reached the workload's entrypoint `claude` as
arguments, so the render writes `entrypoint: [/opt/julieta/bin/julieta]`
and `cmd: [layout, up]` (REQ-1370), which `sbx env plan` accepts; the
attached run with that shape is A15's (T026), and T211 waits on it.

- Implements: REQ-1355 (02 2.3), REQ-1357 (03 3.3), REQ-0200 (03 3.3),
  REQ-1370 (03 3.3), REQ-0227 (03 3.7), REQ-0221 (03 3.6), REQ-1365 (06
  6.3), REQ-0583 (06 6.3).
- Operator: yes, the approval line for `schemas/**` (ask-first
  `formats`): the `link:` value of the `files` map of `render.v1` is a
  new value form.
- Acceptance:
  - The env file goldens of 03 3.3 carry a `target` on every additional
    workspace but the primary, `entrypoint` and `cmd` and no `skills`
    key, and
    `sandboxOptions` in the flow form of 03 3.3's example; probe A5
    confirms them later (T019), and until then they are marked as the
    facts of this task.
  - A render of a project with two secondary repos names each memory
    dir's target after its `dir`, through the exported target
    functions; the size rule of `sync` and the stand-in manifest of
    `julieta spec validate` measure those targets, not host paths
    (amended on 2026-11-07).
  - After a promotion `.romeu/bin/julieta` is a relative link to the
    arch binary and `render.json` holds its `link:` entry; a link whose
    target changed, a link replaced by a file and a missing link each
    make the drift check exit 4 naming the path; a promotion that
    replaces the link does so by rename, asserted by the interrupted
    promotion test of T039 extended with the link.
  - Item 4 is T212's (amended on 2026-11-07).
- Verify: `go test ./internal/render/... -run
  '^(TestEnvTargets|TestEnvEntrypointAndCmd|TestBinLink|TestDriftLink|TestRecoverFinishesAPromotionKilledAfterEachStep)'
  -v && go test ./internal/cmd/romeu/sync/... ./internal/cmd/julieta/spec/...
  -run '^(TestManifestSizeTargets|TestSpecManifestTargets)' -v`.
  The patterns name the functions this task adds, and the recovery test
  of T039 it extends with the link, and the reviewer reads them in the
  `-v` output (10 10.8).
