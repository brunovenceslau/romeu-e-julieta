---
id: T170
title: "e2e on the support matrix: the macOS job, the pairs of host and sandbox, and the Mac pair at the sitting"
milestone: M3
module: ci-release
fate: new
owns:
  - .github/workflows/ci.yml
  - .github/workflows/x64.yml
  - tools/ci/internal/workflows/**
  - docs/spec/10-testing-style.md
needs: []
after:
  - T181
  - T186
ask_first:
  - approvals
  - checks
  - dependencies
sitting: none
---

# T170 - e2e on the support matrix: the macOS job, the pairs of host and sandbox, and the Mac pair at the sitting

Re-plan (2026-10-29): after T181, which also owns
`tools/ci/internal/workflows/**` and `.github/workflows/ci.yml`.

Goal: the end-to-end tests run on the pairs of host and sandbox the
product supports, and each pair the hosted CI cannot run is named with
where it runs instead. Decided on 2026-10-23, scope this repository.

romeu runs on the host and julieta runs in the sandbox, which is always
Linux. The supported pair is a Mac on Apple silicon with a linux/arm64
sandbox (decided on 2026-10-23). A Linux host with a sandbox of its own
architecture, arm64 and amd64, is a pair the CI runs because its
runners are Linux; it is a test vehicle, not a supported pair, and
whether it becomes one is not decided now.

What runs today, measured on 2026-10-23: the e2e of git and the fake sbx
runs on `ubuntu-26.04-arm` alone, although the row of 10 10.1 names
macOS arm64 too, so the e2e of `romeu init` (T041) and `approve
--toolchain` (T042) never ran on a Mac in CI; the hybrid level (romeu on
the host, julieta in a Linux container) runs on `ubuntu-26.04-arm` and,
scheduled, on `ubuntu-26.04`; and the Mac pair, romeu darwin/arm64 with
julieta linux/arm64, runs nowhere. GitHub documents that its hosted
macOS arm64 runners have no nested virtualization, which a Linux
container on a runner that is itself a virtual machine needs; a Linux
container does run in an sbx sandbox on a Mac (measured on 2026-10-23:
`docker version` reports linux/arm64 inside the sandbox), where the
sandbox is the only virtual machine. Whether `macos-26` runs one is
measured by this task, not taken from the documentation.

- Implements: none (it aligns `ci.yml` with REQ-0735 and REQ-0736, and
  adds the support-matrix rule below to 10 10.1).
- Operator: yes, for the Mac pair at the sitting.
- Acceptance:
  - First, measured on `macos-26` in a run of this task's pull request:
    whether a Linux container starts there (Docker, or sbx itself). If
    it does, the Mac pair runs at the hybrid level in `ci.yml` on every
    pull request and the sitting item below is dropped; if it does not,
    the run's output is the evidence, linked from 10 10.1.
  - `ci.yml` runs the e2e of git and the fake sbx on `macos-26` on every
    pull request, the level whose romeu commands reach no julieta call,
    as the row of 10 10.1 already says; `tools/ci workflows` holds the
    new shape (the `e2e` matrix names the Linux and macOS labels), and
    `all on macos-26` needs it.
  - 10 10.1 holds one table of the pairs (host OS and architecture,
    sandbox architecture), each marked supported or test vehicle, with
    the level that runs it and where: a job of `ci.yml`, a job of
    `x64.yml`, or the sitting. A check fails when a pair has neither a
    job nor a stated reason, so a pair added later is a red check until
    it is placed.
  - If `macos-26` runs no Linux container, the Mac pair runs at a
    sitting on the maintainer's Mac with the real sbx: the hybrid journeys with romeu built for darwin/arm64 and
    julieta for linux/arm64, the outcome recorded with the sbx version
    and the date, as the probes of 11 record theirs.
  - The Linux pairs: arm64 with arm64 on every pull request, amd64 with
    amd64 in `x64.yml`, at the hybrid level.
  - The I27 E cases of `e2e/sync_test.go` (a personal kit with each
    hostile entry, and the hostile tree delivered by a bundle) run and
    pass on `macos-26` (case-insensitive APFS), as 05 5.2 I27 and
    REQ-0737 say (from T043, 2026-10-30).
- Verify: the run of `ci.yml` on the pull request shows `e2e on
  macos-26`; `go run ./tools/ci workflows` passes and fails on a copy of
  `ci.yml` without the macOS leg.

Open, not changed by this task: 00 0.2 and REQ-0868 name an Intel Mac
as a supported host with a linux/amd64 sandbox, and sbx does not run on
an Intel Mac (stated on 2026-10-23). The correction of 0.2, REQ-0868
and the darwin/amd64 leg of `x64.yml` is a decision of its own; it
reopens when this task places the pairs, or at the next sitting,
whichever comes first.
