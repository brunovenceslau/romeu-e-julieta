---
id: T187
title: "`state`: the descriptor cache finds a workload by its index digest and platform"
milestone: M3
module: state
fate: new
owns:
  - internal/state/load.go
  - internal/state/load_test.go
  - internal/state/records.go
  - internal/state/records_test.go
needs:
  - T033
after: []
ask_first: []
sitting: none
---

# T187 - `state`: the descriptor cache finds a workload by its index digest and platform

Goal: a second `sync` of a multi-arch workload reads no registry. Step
6 of `sync` (T043, slice C) looks the descriptor cache up by the digest
the spec names, and for a multi-arch image that is the index digest,
while the entry is keyed by the platform manifest's digest
(`Descriptor.Digest`, with `Workload` the index and `Platform` the
os/arch), so the lookup never hits: the registry is read at every
`sync`, and a `sync` without the network fails with
`registry-unreachable`. Decided on 2026-10-29 (scope this repository): a
task of its own on `internal/state`, which T043 does not own, before the
J2 end-to-end test of the slice that links `sync` into the binary.

1. `Store.FindDescriptor(workload, platform)` returns the cached entry
   whose `Workload` and `Platform` are the pair, reading the entries of
   `descriptors/` as `GetDescriptor` reads one (an entry that does not
   decode, or holds another digest than its name says, is
   record-unreadable, `RJ-334`); an absent pair is the error
   `GetDescriptor` gives an absent digest, wrapping `fs.ErrNotExist`. A
   local kit's entry, which has no `Workload`, never matches. The key
   stays the manifest digest, and `state-descriptor/v1` does not change.
2. Step 6 of `sync` asks `FindDescriptor` first, and `GetDescriptor` for
   a workload of one manifest: T043 makes the call in
   `step6_render.go`, which it owns, and rewrites the test of slice C
   that pins the miss.

- Implements: none: the cache-first read of REQ-0341 (04 4.2), which
  T043 implements, over the cache of T033.
- Operator: no.
- Acceptance:
  - A store holding an entry whose `Workload` is an index digest and
    whose `Platform` is `linux/arm64` returns it for that pair, and
    `fs.ErrNotExist` for `linux/amd64`, for another index and for an
    empty workload; an entry with no `Workload` is never returned.
  - An entry that does not decode makes the lookup record-unreadable
    (`RJ-334`), as `GetDescriptor` does.
  - After T043: a second `sync` of a project whose workload is a
    multi-arch index makes no registry read; T043's e2e asserts it with
    the fake registry.
- Verify: `go test -race ./internal/state/`.
