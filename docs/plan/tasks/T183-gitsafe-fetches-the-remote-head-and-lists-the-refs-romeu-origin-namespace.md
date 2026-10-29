---
id: T183
title: "gitsafe fetches the remote HEAD and lists the refs/romeu/origin namespace"
milestone: M3
module: gitsafe
fate: new
owns:
  - internal/gitsafe/refs*.go
needs:
  - T023
after: []
ask_first:
  - gitsafe
sitting: none
---

# T183 - gitsafe fetches the remote HEAD and lists the refs/romeu/origin namespace

Goal: the two reads `romeu sync` needs from `refs/romeu/origin/`, which
gitsafe refuses today. Decided on 2026-10-29, scope this repository,
with priority, from the review of T043 (#329, #333); the part of T043
that links `sync` into the binary (`import_sync.go`) uses both.

1. A fetch may take the remote's HEAD into
   `refs/romeu/origin/<dir>/HEAD` (`checkRefspec` refuses a source that
   does not start with `refs/`, `internal/gitsafe/refs.go`), so a repo
   whose spec has no `ref` resolves its egress commit from the origin,
   not from the clone's local `refs/remotes/origin/HEAD`, which a local
   rewrite moves (I9).
2. `ListRefs` admits the namespace `refs/romeu/origin/`, so `sync
   --from <sha>` checks that the commit is reachable from at least one
   fetched ref of the config repo, as REQ-0336 and REQ-0466 say, not
   only from the tip of the config ref. The namespace is admitted per
   repo dir, `refs/romeu/origin/<dir>/`, its segment checked as the
   salvage generation is.
3. A remote branch named `HEAD` would map to the same ref as the
   remote's HEAD: the fetch refuses it.

- Implements: REQ-0492 (05 5.2), REQ-0336 (04 4.2), REQ-0466 (05 5.1).
- Operator: no.
- Acceptance:
  - A fetch with the refspec `+HEAD:refs/romeu/origin/<dir>/HEAD`
    writes that ref and no other outside the namespace; a refspec whose
    source is any other name outside `refs/` is still refused.
  - `ListRefs` over `refs/romeu/origin/` lists the fetched refs and
    refuses a name outside the namespace, as it does for `refs/heads/`.
  - The table tests of `internal/gitsafe` cover both, on real git.
- Verify: `go test ./internal/gitsafe/ -run 'Refspec|ListRefs' -v`.
