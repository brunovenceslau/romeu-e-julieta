---
id: T183
title: "gitsafe fetches the remote HEAD and lists the refs/romeu/origin namespace"
milestone: M3
module: gitsafe
fate: merged
owns:
  - internal/gitsafe/refs*.go
  - internal/gitsafe/fetch*.go
  - internal/gitsafe/gitsafe_test.go
needs:
  - T023
after: []
ask_first:
  - gitsafe
sitting: none
merged: 2026-10-29
pr: 342
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
3. `refs/romeu/origin/<dir>/HEAD` is written only by a fetch whose
   source is `HEAD`, checked by `ls-remote` just before, and no fetch
   into `refs/romeu/origin/` mirrors a ref named `HEAD`: gitsafe adds
   one negative refspec for each name git reads `HEAD` as
   (`refs/heads/HEAD`, `refs/tags/HEAD` and the rest of its lookup) to
   every such fetch, whatever the caller passes; a pattern destination
   under the namespace has its `*` as a whole last component, and
   gitsafe derives one more negative per pattern source, the source
   with its `*` read as `HEAD` (`refs/foo/*` gives `^refs/foo/HEAD`);
   and gitsafe refuses a fetch of `HEAD` from an origin that advertises
   no HEAD naming a commit (unborn, or a symref to a missing branch)
   before it runs (decided on 2026-10-29, scope this repository: the
   rule is in gitsafe, not in each caller).
   Deferred: on a case-insensitive file system a pattern can write a
   case variant of `HEAD` (`head`, or `hEAD` through `h*`) into the
   same loose ref file, and git compares names case-sensitively.
   Reopened by the first macOS run of the `sync` e2e (T043) or T170,
   whichever comes first.

- Implements: REQ-0492 (05 5.2), REQ-0336 (04 4.2), REQ-0466 (05 5.1).
- Operator: no.
- Acceptance:
  - A fetch with the refspec `+HEAD:refs/romeu/origin/<dir>/HEAD`
    writes that ref and no other outside the namespace; a refspec whose
    source is any other name outside `refs/` is still refused.
  - `ListRefs` over `refs/romeu/origin/` lists the fetched refs and
    refuses a name outside the namespace, as it does for `refs/heads/`.
  - A remote branch or tag named `HEAD` never lands in `<dir>/HEAD`,
    whether or not the call fetches `HEAD`, nor does `refs/foo/HEAD`
    through `+refs/foo/*:refs/romeu/origin/<dir>/*`; a pattern such as
    `<dir>/H*` is refused before git runs; and a fetch of `HEAD` from
    an origin whose HEAD is unborn fails before any fetch runs and
    writes nothing, on real git.
  - The table tests of `internal/gitsafe` cover both, on real git.
- Verify: `go test ./internal/gitsafe/ -run 'Refspec|ListRefs' -v`.

Re-plan (2026-10-29): Item 3 is replaced by the rule above, scope this
repository. A first rule, that a pattern into `refs/romeu/origin/<dir>/`
needs `HEAD:refs/romeu/origin/<dir>/HEAD` in the same call, was tried
and dropped: it broke the prerequisite fetch of `internal/salvage`
(T062), and the git check it relied on (a fetch that maps two sources
to one ref fails) never fired once the negative refspecs existed. The
review of the same day added the whole-component rule and the derived
negatives: a destination `<dir>/H*` built `<dir>/HEAD` from
`refs/heads/EAD`, and `refs/foo/*` mirrored `refs/foo/HEAD`.
