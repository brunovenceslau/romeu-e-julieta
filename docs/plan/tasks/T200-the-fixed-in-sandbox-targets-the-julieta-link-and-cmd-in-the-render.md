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
  - internal/render/kits*.go
  - internal/render/promote*.go
  - internal/render/types.go
  - internal/render/render_test.go
  - internal/render/testdata/**
  - internal/cmd/romeu/sync/rule_julieta_bin*.go
  - internal/cmd/romeu/sync/step6_render_test.go
needs:
  - T037
  - T038
  - T039
after:
  - T174
  - T188
  - T189
ask_first:
  - formats
sitting: none
---

# T200 - The fixed in-sandbox targets, the julieta link and cmd in the render

Goal: the rendered env file names a fixed in-sandbox `target` for every
additional workspace but the primary clone, writes `skills: off` and
`cmd`, and `.romeu/bin` carries a `julieta` link beside the arch binary,
so no per-project path and no host user name reach a kit or a sandbox.

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
2. `skills: off` (REQ-1358) and `cmd: [/opt/julieta/bin/julieta,
   layout, up]` (REQ-1370) in every rendered env file; no `entrypoint`.
3. The `julieta` link (REQ-1365): on promotion `.romeu/bin/julieta` is a
   relative symbolic link to `julieta-linux-<GOARCH>`; `render.json`
   records it as `".romeu/bin/julieta": "link:julieta-linux-<GOARCH>"`
   (03 3.7, REQ-0227); the drift check reads it with `readlink` and
   never follows it; a promotion replaces it by rename.
4. The reserved kit arg `julietaBin` goes: the render passes no arg to
   the `julieta-claude` kit (06 6.4 item 2), and the sync error on a
   spec that set it goes with it (T043's acceptance line is history).

- Implements: REQ-1355 (02 2.3), REQ-1357 (03 3.3), REQ-1358 (03 3.3),
  REQ-1370 (03 3.3), REQ-0227 (03 3.7), REQ-0221 (03 3.6), REQ-1365 (06
  6.3), REQ-0583 (06 6.3).
- Operator: yes, the approval line for `schemas/**` (ask-first
  `formats`): the `link:` value of the `files` map of `render.v1` is a
  new value form.
- Acceptance:
  - The env file goldens of 03 3.3 carry a `target` on every additional
    workspace but the primary, `skills: off` and `cmd`; probe A5
    confirms them later (T019), and until then they are marked as the
    facts of this task.
  - A render of a project with two secondary repos names each memory
    dir's target after its `dir`, and the manifest it encodes carries
    the same targets in `repos[].memory` and `sharedMemory`.
  - After a promotion `.romeu/bin/julieta` is a relative link to the
    arch binary and `render.json` holds its `link:` entry; a link whose
    target changed, a link replaced by a file and a missing link each
    make the drift check exit 4 naming the path; a promotion that
    replaces the link does so by rename, asserted by the interrupted
    promotion test of T039 extended with the link.
  - No rendered kit args carry `julietaBin`, and the test that asserted
    the sync error on a spec setting it is removed.
- Verify: `go test ./internal/render/... -run
  '^(TestEnvTargets|TestEnvSkillsAndCmd|TestBinLink|TestDriftLink)' -v`.
  The pattern names the functions this task adds, and the reviewer
  reads them in the `-v` output (10 10.8).
