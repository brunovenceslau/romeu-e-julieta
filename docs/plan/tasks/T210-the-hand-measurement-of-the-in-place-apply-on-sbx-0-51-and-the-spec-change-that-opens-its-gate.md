---
id: T210
title: "The hand measurement of the in-place apply on sbx 0.51, and the spec change that opens its gate"
milestone: M4
module: romeu-cli
fate: new
owns:
  - docs/notes/in-place-apply-measurement.md
  - docs/spec.md
  - docs/spec/01-system-model.md
  - docs/spec/03-formats.md
  - docs/spec/04-cli.md
  - docs/spec/05-security.md
  - docs/spec/09-journeys.md
  - docs/spec/11-host-probes.md
needs: []
after:
  - T189
  - T205
ask_first:
  - spec
sitting: none
---

# T210 - The hand measurement of the in-place apply on sbx 0.51, and the spec change that opens its gate

Goal: the operator's hand measurement on sbx 0.51 that the in-place
apply row of the index's Deferred decisions names as its trigger, and,
where it passes, the spec change that opens the gate of `run` step 3
and lifts the refusal of a stopped sandbox whose recreate digests
differ.

Added on 2026-11-07 (scope this repository), as decided on 2026-11-07
(decision G and the later decisions 1 and 2, with N5): nothing shows
that an exit 0 of `sbx env run -d <dir>` on a changed env file applied
the change and kept the instance, so the in-place apply waits behind a
closed gate (04 4.2, REQ-1376; the Deferred row of the index), and
`run` (T066) builds the gate only. This task is the one way to open
it. It blocks nothing on the critical path: no task needs it, and its
`after` edges are the file overlaps on the spec pages it may edit, the
pages T189 and T205 own (T055 and T188 come before T189). A15 (T019,
T026) re-records the same measurement with the harness later, and again
at each floor bump.

1. The measurement, by hand on the maintainer's host on sbx 0.51,
   recorded in the note this task owns, each argv with its output, its
   exit status and the sbx version:
   - on a running and on a stopped sandbox, `sbx env run -d <dir>` with
     an env file changed in one kind at a time: a kit added, a kit's
     content changed, a kit removed; the workload; a `ports` entry;
     each of `rootSize`, `dockerSize` and `clonedWorkspaceSize`;
     `agent`; a repo added, a repo removed; an additional workspace's
     `target` or `readOnly`; a secrets binding added, a secrets binding
     removed; each preceded by a marker file written inside and
     followed by `sbx ls --json` (its mount fields), `sbx inspect`,
     `sbx policy ls`, the marker, `git status` in the primary clone
     inside, the agent's login and the sandbox id (I26). An exit 0
     that keeps the instance and leaves the change unapplied is
     recorded as such, never as an apply;
   - a declined plan's exit status, on a running and on a stopped
     sandbox, and that nothing changed; the same changed env file
     applied twice; an apply whose sbx client was killed mid-apply;
     the attached `sbx env run <dir>` with an equal env file, with and
     without `--no-input`;
   - with a changed env file in `<dir>`, `sbx stop`, then `sbx env exec
     <dir> -- true`, followed by the same read-backs: whether it starts
     the sandbox, and whether the configuration it was created with
     stays in force (N5);
   - on a running sandbox with a changed env file in `<dir>`, `sbx env
     exec <dir> -- true` and `sbx env exec -it <dir>`, the read-backs
     taken before and after: whether the configuration stays unchanged
     and whether sbx prompts (RX-1, the exec row of 05 5.4, REQ-1388).
2. The decision, in this task's pull request, under the `spec` line:
   - where the stopped-start reading holds (the exec starts the
     sandbox and the created configuration stays in force), that call
     becomes the start of a stopped sandbox whose digests differ, and
     the refusals of REQ-1376 and REQ-1387 are lifted in 04 4.2, 01
     1.7, 09 J3a and the index's Deferred row;
   - where the apply passes for every kind on both states, the in-place
     apply leaves the Deferred row: the scope that row lists (the
     generation's `repos` on a repo change, a `generations[].kits`
     record and the status line of REQ-1375, the row of 01 1.6 that
     moves `createdWith`, the apply's timeout class, the order of the
     compatibility check, the snapshot and the apply, the failure ids
     and the hint, the read of the sandbox id after exit 0, a trigger
     on the env file's own digest, REQ-1395) becomes spec text;
   - where it passes some kinds and not others, the row stays and
     records a gate per kind as its next scope;
   - where the running exec reads the changed file, the running branch
     of REQ-1376 is reopened before the next release (REQ-1388);
   - where nothing passes, the note records it and the row stays
     deferred with this task's result as its evidence.
   The code that follows a lift is a task of its own, added by a
   re-plan pull request (`tools/new task`); this task changes no code.

Outcome (2026-11-08): the apply passed for no kind, so the in-place
apply row stays deferred with the note as its evidence; the
stopped-start reading held for six kinds, and the refusals were lifted:
REQ-1387 except for an adopted generation, and `run`'s (REQ-1376) also
except for a kit or a secrets binding removed since the create, which
`pull` keeps too (04 4.2, REQ-1405, REQ-1406); T217 is the code task. The running exec read no
changed file for the kinds tried, so REQ-1388's running branch stays.
A killed apply on a present sandbox and `agent` alone were not
measured; A15 records them (decided on 2026-11-08).

- Implements: REQ-1388 (05 5.4), REQ-1395 (05 5.4), REQ-1375 (05 5.4).
- Operator: yes: the measurement of item 1 on the host with sbx 0.51,
  and the approval line for `docs/spec.md`, `docs/spec/01-system-model.md`
  and `docs/spec/05-security.md` (ask-first `spec`) for the edits of
  item 2.
- Acceptance:
  - The note records each case of item 1 with its argv, output, exit
    status and sbx version, and for each case whether the instance
    stayed (the sandbox id, the marker, `git status`, the login) and
    whether the change took effect as read back through sbx.
  - The Deferred row of the index, REQ-1376, REQ-1387 and the rows of
    REQ-1375, REQ-1388 and REQ-1395 in 05 5.4 say which outcome of item
    2 was taken, with the date and the note as the source.
  - Where a refusal is lifted, the pull request names the code task
    that the re-plan adds for it.
- Verify: `go run ./tools/ci static` (the note and the spec edits pass
  hygiene, prose and `reqs`; no test is added).
