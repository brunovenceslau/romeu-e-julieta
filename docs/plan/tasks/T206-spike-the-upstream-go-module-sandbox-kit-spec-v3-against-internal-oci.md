---
id: T206
title: "Spike: the upstream Go module sandbox-kit-spec/v3 against internal/oci"
milestone: M4
module: oci
fate: new
owns:
  - docs/notes/sandbox-kit-spec-v3-spike.md
needs:
  - T032
after: []
ask_first: []
sitting: none
---

# T206 - Spike: the upstream Go module sandbox-kit-spec/v3 against internal/oci

Goal: measure what of I28 the `spec`, `load` and `fetch` packages of
`github.com/docker/sandbox-kit-spec/v3` cover at the pinned kit
frontend, and what of `internal/oci` would go if the product imported
them, so the deferred row of the index can be decided.

Added on 2026-11-06 (scope this repository): the row "importing
`github.com/docker/sandbox-kit-spec/v3`" of the index's Deferred
decisions, under ADR 0038 (search for a maintained library before
writing one), names this task as its trigger. The maintainer's stated
preference, recorded on 2026-11-06: the official library wherever it
covers I28. Adding the dependency stays ask-first (`go.mod`, ask-first
`dependencies`) and is not this task's; the strict fail-closed subset
grammar of 06 6.1 stays the gate's rule whichever way.

1. In a scratch module outside the tree, pin the module at the version
   that matches the kit frontend of `kits/pins.yaml` (3.0.0-m.12) and
   read its API: which descriptor keys and capability types it models,
   whether it refuses an unknown key or capability type, whether it
   reads a descriptor from a registry by digest over HTTPS as
   `internal/oci` does, and whether `load` reads a local kit dir as
   `internal/render` does.
2. Feed it the conformance fixtures of I28 (the A11 and A12 fixtures,
   the sitting M2 facts until block A) and compare its reading with
   `internal/oci`'s normalized capability set and with `sbx kit
   inspect --offline` (Q20).
3. Count what of `internal/oci` the module would replace, what it
   would not (the fail-closed rule, the digest pin, the egress split
   of 07 7.5), and how its API moved between m.7 and m.12 (the breaks
   of m.7, m.8, m.9 and m.10), so the maintenance cost is a measured
   number.

- Implements: none: a spike that measures; it changes no behaviour and
  cites the index row it serves.
- Operator: yes, the decision that follows the note, as a memory entry
  and an edit of the deferred row (ask-first `spec`, in the pull
  request that decides).
- Acceptance:
  - The note records the module version, the API read, the fixture
    comparison (equal, differing, refused) per capability type, the
    count of `internal/oci` files the import would retire, and the
    recommendation with its reason.
  - Nothing under `internal/` changes, and `go.mod` is untouched.
- Verify: `go run ./tools/ci static` (the note passes hygiene and
  prose; no test is added).
