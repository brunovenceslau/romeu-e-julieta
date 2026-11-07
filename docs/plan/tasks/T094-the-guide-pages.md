---
id: T094
title: "The guide pages"
milestone: M6
module: docs
fate: deferred
owns: []
needs:
  - T075
after: []
ask_first: []
sitting: none
---

# T094 - The guide pages

Re-plan (2026-10-12): deferred to M6; release and docs, before the
candidate: it needs T075, which completes the scenario functions the
pages cite. Reopened by: the first adopter other than the maintainer; or
the maintainer's environments all on the new tools. The text below is
the one written before the re-plan; `owns` is written when the task
reopens.

Amended on 2026-11-01 (the release and upgrade amendments, ADR 0044):
REQ-0909 (10 10.2) gained the ref that started the workflow,
`refs/heads/main`, checked by `--source-ref`; the entry the guide pages
show comes from J1's `commands.yaml` (T088).

Amended on 2026-11-07 (the sbx 0.51 and kit schema m.12 amendments,
as decided on 2026-11-07): the in-place apply is a row of Deferred
decisions, so the J3 page says that with a changed env file `run` warns
`recreate-needed` and attaches on a running sandbox (unless step 6
refuses with `mounts-mismatch`), and refuses a stopped one with exit 5;
the hints are those of REQ-1377, quoted as the page's expected output:
`romeu recreate <name>` while it runs, and for a stopped sandbox to undo
the change, run `romeu sync <name>` and rerun, with its warnings that
undoing re-approves what the change removed and that a hand `sbx env
run -d` or `sbx env rm` bypasses the gate or the salvage. The J3 page
also gives `adopt`'s warning before its prompt (REQ-0312). The J9 and J13
pages say to start the sandbox (`romeu run --no-attach <name>`) before
the `romeu upgrade` or the `romeu sync` that promotes a recreate-class
change, and to recreate it while it runs. No page says that sbx applies
an env file in place, and the residue of a removed kit (REQ-1375) is
described only once that Deferred row fires; the earlier proposal to
read A15's in-place result here is dropped while the gate is closed, so
this task gains no edge to T026.

- Implements: REQ-0719 (08 8.5), REQ-0723 (09 (opening)), REQ-0730 (09
  (opening)), REQ-0909 (10 10.2), REQ-1080 (12 12.2), REQ-1088 (12
  12.2), REQ-1108 (12 12.3), REQ-1184 (12 12.8), REQ-1187 (12 12.8),
  REQ-1188 (12 12.8).
- Operator: no.
- Ask-first, as forecast before the re-plan: none (`docs/guide/` is on
  no surface).
- Acceptance:
  - Each `## J<n>` heading of 09 is named by the `journey:` of a page,
    J14's among them.
  - Each `romeu` or `julieta` command in an `sh` block equals an entry
    of the journey's `commands.yaml`, or J1 step 2's verification entry,
    which this task does not change: the completeness test of T041 kept
    those files whole as the commands landed.
  - `docs/guide/recovery.md` has one section per id whose recovered-in
    cell names it; each anchor a cell names resolves; the J1 page says
    what to check by hand for an editor other than VS Code; the J14 page
    lists the paths and rules the operator removes.
  - The J3, J9 and J13 pages hold the sentences of the amendment above,
    and no page names `romeu run <name>` as the recovery of
    `recreate-needed`; **[review]**.
  - The J5 page's sentence on the hook order C5 recorded (Q22) is T093's
    post-block-B edit.
  - Each page gives the expected output and the recovery from each
    expected error id. **[review]**: no check reads which error ids a
    page names.
- Verify: `go run ./tools/ci docs`.
