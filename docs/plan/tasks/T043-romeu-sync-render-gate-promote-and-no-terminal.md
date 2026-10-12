---
id: T043
title: "`romeu sync`: render, gate, promote, and no terminal"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/sync/**
  - cmd/romeu/import_sync.go
  - e2e/scenarios/j2/**
  - e2e/scenarios/j11/**
  - e2e/sync_test.go
needs:
  - T040
  - T041
  - T042
  - T046
  - T029
  - T051
  - T035
  - T103
after: []
ask_first: []
sitting: none
---

# T043 - `romeu sync`: render, gate, promote, and no terminal

Earlier title: `romeu sync`: render, gate and promote.

Goal: `romeu sync`: steps 1 to 10 of 04 4.2, gate 2, promote, the no-TTY
exit 3, the memory directories, the manifest, julieta delivery into
`.romeu/bin`, the idempotency field of the definitions with its first
row, J2 and J11.

Re-plan (2026-10-12): Kept; T045 (`sync` without a terminal) is folded
in, with its acceptance rows as written there, and the I-level hostile
cases of T044 ride here. Step 9 (live changes) stays dark until T047.
`signing_rule.go` belongs to T080 and `live*.go` to T047, both after
this task.

- Implements: 04 4.2 ("How `romeu sync` renders, gates and promotes",
  steps 1 to 8 and 10), 01 1.6 ("Candidate"), 09 (J2), 10 10.3 (Shared
  scenarios), ADR 0001 rule 13, 05 5.2 (I5, I7, I8, I9, I12).
- Operator: no.
- Acceptance:
  - In the e2e with the fake `sbx`, a first `sync` on a terminal
    promotes after `y`, and leaves the live files byte-identical after
    `N` (I7).
  - A stale toolchain makes `sync` exit 3 at step 1, before it promotes.
  - `--from <sha>` refuses a commit not reachable from
    `refs/romeu/origin/*`.
  - A spec secret without its `name@project` binding is exit 2 naming
    the key, and `github@other` does not satisfy project `shop` (I8).
  - An uncommitted lock change, a commit on another branch and a local
    `refs/remotes/origin` rewrite leave egress unchanged; changing `ref`
    changes egress and meets gate 2 (I9).
  - After `sync`, `refs/heads`, `refs/tags`, `refs/remotes` and `HEAD`
    of each clone are unchanged (I5).
  - An absent project is reported as `orphaned`, and nothing is removed.
  - Step 3 of 04 4.2: two specs with one name or one URL make `sync`
    exit 2 listing every error (J2 step 3).
  - Step 4 of 04 4.2, after a first `sync`: `<name>-env/`,
    `memory/<dir>/` with its fixed subdirectories exist, each repo is a
    hardened clone, each origin is fetched into
    `refs/romeu/origin/<dir>/*`, and `egressCommit` resolves from
    `refs/romeu/origin/<dir>/<ref>` (J2 step 3).
  - The idempotency meta-test of 10 10.1 lands here as a table with
    `sync`'s row: a second run on unchanged inputs leaves every file's
    content and mode and every ref as they were, except `lastRun`. Each
    later task that adds a converging or an appending command adds its
    row, a romeu row in this e2e and a julieta row in the container e2e;
    an appender's row asserts the one thing 04 4.1 names and nothing
    else. T073 checks that no command lacks a row.
  - The J2 scenario function joins `e2e/scenarios` (started in T041) up
    to its step 3 and reads its `commands.yaml` lines; T055 adds its
    step 1 and T066 its step 4, each in a step file of its own under
    `e2e/scenarios/j2/`, after this task.
- Verify: `go test -tags e2e ./e2e/... -run '^TestSync' -v`. The pattern
  names the `TestSync*` functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
