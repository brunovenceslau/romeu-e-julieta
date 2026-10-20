---
id: T151
title: "`gitsafe`: the hardened fast-forward and clone"
milestone: M1
module: gitsafe
fate: new
owns:
  - internal/gitsafe/fetch*.go
  - internal/gitsafe/clone*.go
  - internal/gitsafe/status*.go
  - internal/gitsafe/gitsafe_test.go
  - internal/gitsafe/helper_test.go
  - docs/spec/04-cli.md
  - docs/handoff/pending/000000000098hfz7arr3swy84e-gitsafe-s-fast-forward-overwrites-ignored-files-and-runs.md
needs:
  - T051
  - T143
after: []
ask_first:
  - gitsafe
  - spec
sitting: none
---

# T151 - `gitsafe`: the hardened fast-forward and clone

Goal: harden the two `gitsafe` verbs that write a checkout,
`FastForward` and `Clone`, before `julieta setup` runs in daily use, as
decided on 2026-10-20 (a task on the `gitsafe` surface, with an
approval line). Two security audits of 2026-10-20 found, and the
pending note
[gitsafe's fast-forward overwrites ignored files and runs smudge filters](../../handoff/pending/000000000098hfz7arr3swy84e-gitsafe-s-fast-forward-overwrites-ignored-files-and-runs.md)
records, four gaps:

1. `FastForward` (`merge --ff-only`) overwrites an ignored, untracked
   file when the incoming commit adds a tracked file at its path, and
   `Status.Clean()`, the caller's check, does not cover ignored files.
   The fast-forward now refuses it: git's own `--no-overwrite-ignore`
   makes the merge fail and name the paths, and `setup` reports the
   repo as `failed` with that error. Refusing, rather than moving and
   naming the paths, follows the spec: REQ-0399 fast-forwards only a
   clean default branch and reports otherwise, a refused fast-forward
   is a warning, the threat model of 05 counts destroying work, and
   REQ-0316 counts ignored files as work that salvage must recover.
2. The merge's checkout runs `filter.<name>.smudge` and `.process`
   from the clone's or the global config. `FastForward` now lists the
   drivers as `Status` does and runs the merge with each emptied.
3. `filterOverrides` dropped a driver with an empty name (`[filter
   ""]`, key `filter..clean`), which git lists and runs for the
   attribute `filter=` with no value.
4. A sandbox `Clone` checked out HEAD with no filter override. It now
   clones with `--no-checkout`, lists the drivers in the new
   repository, where every conditional include of the global config
   (`includeIf "gitdir:"`, `"onbranch:"`, `"hasconfig:remote.*.url:"`)
   resolves as for the checkout, and checks out HEAD with `git reset
   --hard` and each driver emptied. A HEAD that names no commit is left
   without a checkout, as `git clone` leaves it.

The security audit of this task added one more, older gap: with
`merge.verifySignatures=true` in the clone's or the global config,
`merge` starts `gpg.program` or `gpg.ssh.program` for an incoming
commit that carries any `gpgsig` header. `FastForward` now passes
`--no-verify-signatures`. No other verb verifies a signature: the
history and stash reads run with `log.showSignature=false` and a
format with no signature field, and `Status`, fetch and clone verify
none.

One list, `filterNeutral` (`clean`, `smudge` and `process` empty,
`required` false), serves `Status`, `FastForward` and `Clone`. The
maintainer approved on 2026-10-20 amending the specification in the
same pull request: REQ-0463 (05 5.1) says what the verbs empty, that
`FastForward` refuses to overwrite an ignored file and verifies no
signature, which program-running keys sandbox mode neutralizes and
that every other one, the transport keys among them, stays the
sandbox's own, and the precondition
that no other process writes the config between the listing of the
drivers and the command; REQ-0399 (04 4.3) counts an ignored file the
move would overwrite as a refused fast-forward. `owns` does not list
`docs/spec/05-security.md`, which the open task T039 (M3) owns: T151
amends one requirement there and must not wait for T039.

- Implements: none: the hardening serves REQ-0399 and REQ-0463, which
  T051 and T023 implement.
- Operator: no.
- Acceptance:
  - Each gap is reproduced first by a test that fails before the fix,
    as a user hits it, with real git: an ignored `local.env` (`*.env`
    in `.git/info/exclude`) and an incoming commit that tracks it; a
    smudge driver in the clone's local config, one with a dot in its
    name, a long-running process and two in the sandbox's global
    config, one with an empty name, each required, for the
    fast-forward; a required `[filter ""]` clean driver for `Status`;
    smudge drivers in the sandbox's global config for `Clone`, plain
    and behind each of the three conditional includes;
    `merge.verifySignatures=true` with `gpg.program` and with
    `gpg.ssh.program` and an incoming commit with a fake PGP or SSH
    signature.
  - After the fix the fast-forward fails, names `local.env`, and leaves
    the file and HEAD as they were; no planted filter or signature
    program runs; the files hold the tree's content; the fast-forward
    over a signed commit succeeds. Each test keeps its positive
    control: plain git overwrites the file or runs the program.
  - A clone of an empty repository still succeeds with an unborn HEAD
    and no checkout, and a clone still has its default branch checked
    out. A failure after `git clone --no-checkout` (a test refuses the
    listing of the drivers) removes the destination, so the next
    `julieta setup` clones again.
  - The argv and environment tests pin the clone's five commands (the
    URL resolution, the clone with `--no-checkout`, the read of HEAD,
    the listing in the new repository and the checkout with the
    drivers emptied) and the four keys per driver.
  - REQ-0463 and REQ-0399 say what the code does; `go run ./tools/ci
    reqs` passes.
  - The tests of `internal/cmd/julieta/setup` pass unchanged.
- Verify: `go test -race ./internal/gitsafe/... ./internal/cmd/julieta/...`
  and `go run ./tools/ci reqs`.
