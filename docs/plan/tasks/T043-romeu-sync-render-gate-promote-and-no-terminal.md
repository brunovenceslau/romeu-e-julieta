---
id: T043
title: "`romeu sync`: render, gate, promote, and no terminal"
milestone: M3
module: romeu-cli
fate: kept
owns:
  - internal/cmd/romeu/sync/**
  - internal/cmd/romeu/all/import_sync.go
  - e2e/scenarios/j2/**
  - e2e/scenarios/j11/**
  - e2e/sync_test.go
  - e2e/internal/romeu/sync_helpers.go
needs:
  - T040
  - T041
  - T042
  - T046
  - T029
  - T135
  - T035
  - T103
  - T039
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
in, with its rows copied below, and the I-level hostile cases of T044
ride here (the unit table of the hostile trees is T024's). Step 9 (live
changes) stays dark until T047. `signing_rule.go` belongs to T080 and
`live*.go` to T047, both after this task; `sync/steps.go` holds the
ordered step list and `sync/rules.go` the registered render-time rules
(rule 14 of `docs/plan.md`), the seams they plug into. The manifest
encoder and the size function come from T135. The helper file it adds to
the harness of T041 is `e2e/internal/romeu/sync_helpers.go`.


Re-plan (2026-10-28): `sync` also creates the project's shared memory
directory, `memory/_shared/`, 0700, and writes it into the manifest as
`sharedMemory` (REQ-0646, 03 3.6; ADR 0042, T174).

- Implements: REQ-0012 (index), REQ-0042 (01 1.1), REQ-0044 (01 1.2),
  REQ-0054 (01 1.3), REQ-0055 (01 1.3), REQ-0056 (01 1.3), REQ-0059 (01
  1.4), REQ-0074 (01 1.5), REQ-0082 (01 1.6), REQ-0084 (01 1.6),
  REQ-0085 (01 1.6), REQ-0086 (01 1.6), REQ-0087 (01 1.6), REQ-0130 (02
  2.1), REQ-0135 (02 2.2), REQ-0138 (02 2.3), REQ-0140 (02 2.3),
  REQ-0141 (02 2.3), REQ-0143 (02 2.3), REQ-0147 (02 2.3), REQ-0175 (03
  3.1), REQ-0177 (03 3.1), REQ-0178 (03 3.1), REQ-0179 (03 3.1),
  REQ-0180 (03 3.1), REQ-0185 (03 3.1), REQ-0206 (03 3.4), REQ-0220 (03
  3.6), REQ-0272 (04 4.1), REQ-0277 (04 4.1), REQ-0283 (04 4.1),
  REQ-0334 (04 4.2), REQ-0335 (04 4.2), REQ-0336 (04 4.2), REQ-0337 (04
  4.2), REQ-0338 (04 4.2), REQ-0339 (04 4.2), REQ-0341 (04 4.2),
  REQ-0342 (04 4.2), REQ-0343 (04 4.2), REQ-0345 (04 4.2), REQ-0433 (04
  4.4), REQ-0438 (04 4.4), REQ-0443 (04 4.4), REQ-0444 (04 4.4),
  REQ-0466 (05 5.1), REQ-0486 (05 5.2), REQ-0488 (05 5.2), REQ-0489 (05
  5.2), REQ-0490 (05 5.2), REQ-0491 (05 5.2), REQ-0492 (05 5.2),
  REQ-0495 (05 5.2), REQ-0512 (05 5.2), REQ-0513 (05 5.2), REQ-0514 (05
  5.2), REQ-0515 (05 5.2), REQ-0516 (05 5.2), REQ-0517 (05 5.2),
  REQ-0555 (05 5.4), REQ-0570 (06 6.1), REQ-0571 (06 6.1), REQ-0596 (06
  6.4), REQ-0642 (08 8.1), REQ-0643 (08 8.1), REQ-0646 (08 8.1),
  REQ-0736 (10 10.1), REQ-0775 (10 10.1), REQ-1109 (12 12.3), REQ-1207
  (01 1.6), REQ-1208 (01 1.6), REQ-1209 (01 1.6), REQ-1210 (01 1.6),
  REQ-1211 (01 1.6), REQ-1221 (04 4.2), REQ-1222 (05 5.2).
- Operator: no.
- Acceptance:
  - In the e2e with the fake `sbx`, a first `sync` on a terminal
    promotes after `y`, and leaves the live files byte-identical after
    `N` (I7): the pty cases "sync approves on y and promotes" and "sync
    declines on N and deletes the candidate"; the gate prompt is written
    to stderr and the TTY condition is stdin.
  - A candidate dir planted as if left by a killed command is deleted
    before the next render; after `N` no candidate dir remains; a `sync`
    whose widening digest equals the approval promotes with no TTY; at
    most one dir exists under `.romeu/candidates/`.
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
    exit 2 listing every error (J2 step 3); a file under `projects/`
    that is not `*.yaml` exits 2 with `projects-stray-file` (`RJ-345`)
    naming the path, and no spec whose primary repo URL equals
    `config.url` exits 2 with `config-project-missing` (`RJ-346`).
  - Step 4 of 04 4.2, after a first `sync`: `<name>-env/` and each
    `memory/<dir>/` with its fixed subdirectories exist, created mode
    0700; `<name>-env/.metadata_never_index` exists and lies outside
    every mount of the rendered `sbxenv.yaml`; each repo is a hardened
    clone, each origin is fetched into `refs/romeu/origin/<dir>/*`, and
    `egressCommit` resolves from `refs/romeu/origin/<dir>/<ref>` (J2
    step 3).
  - Step 4 of 04 4.2 on a `<name>-env/` whose `memory/<dir>/` already
    holds entries, handoffs and a snapshot bundle: `sync` keeps every
    file byte for byte with its mode (create only), and the idempotency
    row covers the case; a disallowed name under it is exit 2 by step 5
    and nothing is changed.
  - A memory directory with a symlink, a FIFO or a `..` name, or any
    other layout violation of 08 8.1, makes `sync` exit 2 naming the
    offending path and is not followed (I12, I24); a leftover
    `.tmp-<ULID>` makes it warn and not refuse; a `handoff/` over its
    cap makes the command warn and not refuse.
  - A personal kit with each hostile entry of I27 fails `sync` before a
    file is written outside `.romeu/candidates/`, on the macOS runners
    too, selected by the runner table of 10 10.2; a hostile tree
    delivered by a bundle is refused as `--from <sha>`, and refused by
    the walk when made reachable (I27); adding a capability to a kit
    flips gate 2 (I28).
  - Step 6 reads a cached descriptor with the registry unreachable and
    deletes a candidate dir the record does not name: a second `sync`
    with the same workload digest reads the descriptor from the cache
    record of T033 and makes no registry read; a changed digest reads
    the registry once and rewrites the cache. Step 7: an edited live
    file makes `sync` exit 4 with `drift-detected` naming it;
    `--overwrite-drift` replaces it and the hint says the edit is lost.
  - A spec whose encoded manifest exceeds 32 KiB makes `sync` exit 2
    with `manifest-size` (`RJ-313`) before gate 2, by the size function
    of `internal/manifest` (T135); a spec that sets `julietaBin` is a
    sync error naming the key.
  - `sync` warns `julieta-pin-mismatch` when `validate.yml` pins another
    julieta.
  - A romeu built with plain `go build` exits 2 with
    `julieta-not-embedded` on `sync` and `run`, and `status`, `doctor`
    and `version` work.
  - The idempotency meta-test of 10 10.1 lands here as a table with
    `sync`'s row: a second run on unchanged inputs leaves every file's
    content and mode and every ref as they were, except `lastRunAt`.
    Each later task that adds a converging or an appending command adds
    its row, a romeu row in this e2e and a julieta row in the container
    e2e; an appender's row asserts the one thing its command
    definition's side-effect fields name (12 12.3) and nothing else.
    T073 checks that no command lacks a row.
  - The J2 scenario function joins `e2e/scenarios` (started in T041) up
    to its step 3 and reads its `commands.yaml` lines; T055 adds its
    step 1 and T066 its step 4, each in a step file of its own under
    `e2e/scenarios/j2/`, after this task. J11 step 5 is this task's
    journey step.
- Acceptance (from T045, folded):
  - Without a terminal, a `sync` whose widening digest differs deletes
    the candidate dir, prints the digest and exits 3; it records no
    awaiting candidate (01 1.6). Approving a candidate rendered without
    a terminal is a deferred decision (index).
  - `romeu approve` takes `--toolchain` only; `approve <name>` is an
    unknown command (04 4.2).
- Verify: `go test -tags e2e ./e2e/... -run '^TestSync' -v`. The pattern
  names the `TestSync*` functions this task adds, and the reviewer reads
  them in the `-v` output (10 10.8).
