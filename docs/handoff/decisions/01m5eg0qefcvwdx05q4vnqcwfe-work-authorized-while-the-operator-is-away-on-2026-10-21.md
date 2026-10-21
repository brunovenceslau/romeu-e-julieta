# Work authorized while the operator is away on 2026-10-21

Decided on 2026-10-21, for the run that starts with T060 and lasts until
the operator is back. Scope: this repository.

- The work, in this order, one pull request each, with a code review
  before it opens and CI green after: T060; a task for "julieta status
  reads the hook failure store" (0000000000782g86tsyzte1t16), planned
  and built in one pull request; the plan and build of "One race run,
  test tags and parallel CI jobs" (01m5drzqsp6zg34xzmywckwb7b); T150;
  the e2e cache annotation of "The spec catches up with the hardening
  of T127, T143 and T144" (0000000000pz8zh0x006xkqa4w); the file of "A
  short CLAUDE.md of pointers for this repository"
  (01m5e1majbx0hq6p710g9dmtr2); a task for the part of T060's
  `disabled` item outside its owns; T077; the plan, docs only, of
  "Dependency bumps run as a script, not by hand"
  (01m5e14094yhqhsxq2g941pwrf); a checkpoint at the end.
- An ask-first surface gets its approval line in the pull request body,
  for the operator to tick at merge.
- A task that needs an unmerged pull request is stacked on it, as
  `~/.claude/stacked-prs.md` says.
- At most two builder agents at once.
- A patch release of Go or of a pinned tool that fixes a finding of the
  `vulnerabilities` step may be bumped in its own pull request, the
  published checksums compared, with the `dependencies` approval line
  left for the operator.
- The CI task keeps the required check names of `main` through one
  aggregator job per host, and the macOS job runs only the race with
  coverage.
- The instructions file is an `AGENTS.md` of pointers with a one-line
  `CLAUDE.md` that imports it; no rule is restated.
- The bump plan measures what a maintained bot covers of the mise files
  and proposes a `tools/ci` subcommand only for the rest; it also
  proposes how the deferred scheduled run of the pin-freshness check is
  decided.
- A snapshot that meets an input `gitsafe` refuses whole (a ref name, a
  ref count, a reflog) skips that item, snapshots the rest and records
  the skipped item as a hook failure of the repo.
- No new dependency other than the patch bump above; merging stays the
  operator's.
