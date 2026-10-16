---
type: explanation
reader: anyone who wants a picture of romeu, julieta, the host and the sandbox before reading the system model
---

# Architecture diagrams

These four diagrams draw one reading of the specification, made on
2026-10-16, as a picture to look at before the
[system model](../spec/01-system-model.md). They are not part of the
specification: where a diagram and a spec page disagree, the spec page
holds, and the diagram is fixed.
Each file is an [Excalidraw](https://excalidraw.com) scene, kept
under `docs/notes/` by
[ADR 0013](../adr/0013-admit-excalidraw-scenes-as-diagrams-under-docs-notes-and-amend-rule-17-of-the-documentation-standard.md);
GitHub shows its JSON, so open it at excalidraw.com or in an editor
extension that reads `.excalidraw` files.

They go from the widest view to the narrowest:

1. [Context](architecture-diagrams/1-context.excalidraw): the
   operator, the macOS host with romeu, the host tree, host settings
   and state, the keychain and the sbx daemon; GitHub with the product
   repo, the config repo and the worked-on repos; one sandbox with the
   agent, julieta, the clones and the tools; egress to the network.
   Source: [01 1.1](../spec/01-system-model.md#11-components-one-job-each).
2. [Host and sandbox](architecture-diagrams/2-host-and-sandbox.excalidraw):
   what crosses between the host and a sandbox, each crossing marked
   with its trust boundary, A to F. Source:
   [01 1.3](../spec/01-system-model.md#13-trust-boundaries) and the
   table of where data lives in
   [01 1.2](../spec/01-system-model.md#12-sources-of-truth-vs-derived).
3. [Sync and run](architecture-diagrams/3-sync-and-run.excalidraw):
   `romeu sync` from a merged spec to the live tree through gate 2 and
   the promotion commit, and `romeu run` through the preflight to the
   run layout. Source:
   [01 1.4](../spec/01-system-model.md#14-the-gates),
   [01 1.5](../spec/01-system-model.md#15-preflight-before-every-mutating-sbx-call)
   and [01 1.6](../spec/01-system-model.md#16-state-machines).
4. [Generation](architecture-diagrams/4-generation.excalidraw): the
   states of one sandbox lifetime, the write order of a destructive
   command, and the sandbox half of salvage. Source: the generation
   table of [01 1.6](../spec/01-system-model.md#16-state-machines).

The diagrams leave out detail the tables keep. Diagram 3 names the
preflight exit codes only by class. Diagram 4 draws the main rows of the
generation table and omits these: `open` back to none when the create
fails and no sandbox is left, the rows that stay in `open`, `salvaging`
or `removing` (a failed create, a timeout, a restarted salvage, a failed
`sbx env rm`, a resumed removal), and `removing` to `closed-removed`
when the sandbox is found absent. Its arrow from `salvaging` back to
`open` covers the verified salvage of `salvage` and an incomplete
salvage without `--accept-loss`. REQ-0079 generates the state diagrams
of `internal/state` from its transition tables; when that package
lands, its generated diagram replaces diagram 4, and this note says so.
