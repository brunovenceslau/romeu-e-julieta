# 9. Drop the required review and amend the records that assumed it

Date: 2026-10-08

## Status

Accepted

Supersedes in part [1. Adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)

Supersedes in part [2. Develop on the trunk with short-lived branches](0002-develop-on-the-trunk-with-short-lived-branches.md)

Supersedes in part [3. Adopt six Extreme Programming practices and review as pairing](0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md)

Supersedes in part [5. Decide at the last responsible moment and record the trigger](0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)

Supersedes in part [7. Adopt testify assert and require in tests](0007-adopt-testify-assert-and-require-in-tests.md)

Supersedes in part [8. Let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md)

## Context

Six texts said that a pull request needs an approving, code-owner
review before it merges, and the maintainer block measured on
2026-10-08 that the default-branch ruleset requires none: it requires
CI green and refuses a direct push to the default branch
([10 10.2](../spec/10-testing-style.md#release-and-bootstrap), item a).
[ADR 0008, let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md)
saw the gap and chose "We do not rewrite them": the old sentences
stayed, each beside a note that says what was measured. That breaks
rule 3 of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md),
where stale content is deleted, not caveated, and a reader who meets
the old sentence first learns the wrong rule. The spec now states the
merge requirements once, in
[05 5.4](../spec/05-security.md#risks-of-how-this-repository-is-developed),
and the records have to follow it.

Review round 8 found, in the same records and in
[ADR 0007, adopt testify assert and require in tests](0007-adopt-testify-assert-and-require-in-tests.md),
smaller texts that the spec has since corrected or that point at
something that moved. Each is listed in Decision with the finding or
the decision behind it, so a reader can see why it changed. They were
accepted as one record, the decision DR1 of
[round 8](../reviews/round-8.md#decisions-for-the-operator), on
2026-10-08, together with the decisions DR2 to DR8 and the items AR4,
AR6, AR12 and AR16 whose record part lands here. Decision 21, and new
wording in decisions 12, 17 and 18, were added on 2026-10-09, after
the ship gate of the pull request that carries this record (the form
of DR3, F20, F6 and R8-05-35), under the same approval line, extended.

### Alternatives considered

- **Keep the pointer notes of ADR 0008.** Cheapest today, and every
  later reader pays for it: the records keep a rule that no longer
  holds. Declined on 2026-10-08.
- **One new record per amended record.** Six records for one event,
  each restating the same measurement. A single record that names
  each part it replaces keeps the cause in one place.
- **Edit the accepted records in place.** Faster to read, and it would
  break the rule that an accepted record is not rewritten
  ([12 12.5](../spec/12-engineering.md#125-decisions-and-history)).
  The records gain one status line each, and their text stays as it
  was decided.

## Decision

### The review requirement

1. **No review is required to merge.** A merge needs CI green on the
   full list of 10 10.2, and the maintainer's merge at a sitting, by
   working agreement. Review stays a practice: an agent or a person
   reads each change before it merges, and that judgment is worth what
   05 5.4 says it is worth while one account acts for everyone. This
   replaces, on the review requirement only, rule 8 of ADR 0001 (its
   "code-owner review before it merges" and the ruleset that requires
   it), decision 3 of
   [ADR 0002, develop on the trunk with short-lived branches](0002-develop-on-the-trunk-with-short-lived-branches.md)
   on what a merge needs (CI green, which the ruleset requires and the
   token's administrator role can bypass, and no review), and the
   ruleset sentence of
   [ADR 0003, adopt six Extreme Programming practices and review as pairing](0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md)
   ("requires one approving review"). The analogy of ADR 0003, review
   as pairing, stands. It also supersedes, in part, three earlier
   maintainer decisions that assumed the review:
   [N1 of round 3](../reviews/round-3.md#resolutions),
   [op-residual-risks of round 4](../reviews/round-4.md#maintainer-decisions)
   and [op-ruleset-approvals of round 6](../reviews/round-6.md#maintainer-decisions).
2. **ADR 0008's choice not to rewrite the old texts is reversed** by
   this record. Its Consequences bullet that lists the texts left by
   pointer is replaced by this record, which names each part it
   changes.
3. **The approval line keeps its job.** An ADR change still needs the
   approval line of the `decisions` surface, typed by the PR's author,
   as rule 8 of ADR 0001 says; what the line proves is in 05 5.4. A
   merge gate without a review rests on the exception to rule 1 of
   [ADR 0005, decide at the last responsible moment and record the trigger](0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)
   that ADR 0008 recorded on 2026-10-07: a merge gate that does not
   bind the token, accepted by name on that date.

### ADR 0001

4. **Rule 7: numbers are unique; a gap is allowed.** Contiguity goes:
   records written in parallel would collide on the next number, and
   rule 9's cite-by-number makes each renumber cost every citing file
   (R8-adr-10). A record that replaces part of another says so in its
   Status section with one line per record,
   `Supersedes in part [N. Title](NNNN-title.md)`, and the other
   record's Status section gains
   `Superseded in part by [M. Title](MMMM-title.md)` under its status
   word, which does not change. The links match in both directions, as
   for a whole supersede, and the superseding record's Decision names
   each part it replaces. `tools/ci sequences` checks both forms and
   uniqueness.
5. **Rule 5: `tools/ci all` is offline except the `vulnerabilities`
   step**, which reads the Go vulnerability database over the network;
   a local `all` without the network is not the equivalent run, and its
   output says so ([10 10.2](../spec/10-testing-style.md#102-ci)). The
   rule said `all` stays offline, and `govulncheck` was already inside
   it (DR8).
6. **Rule 12: the README's level-2 headings are** what it is and who
   it is for; when not to use it; prerequisites; install and verify;
   first run; trust model; data and uninstall; license; links to
   guides, reference and ADRs. "License" carries the product's license
   and what it means for a config repo (R8-12-20); "data and uninstall"
   says what romeu keeps on the machine and how to stop using it, with
   links to the table of
   [01 1.2](../spec/01-system-model.md#where-data-lives) and to the
   guide of [J14](../spec/09-journeys.md#j14-stop-using-romeu-and-julieta)
   (R8-adr-20). The outlines are in
   [12 12.8](../spec/12-engineering.md#128-contributor-docs-outlines).
7. **The lead-practice table** splits its last text row: commits and
   PRs keep Conventional Commits and the PR sections of 12 12.4, and
   issues get the forms in `.github/ISSUE_TEMPLATE/`
   ([12 12.7](../spec/12-engineering.md#127-text-standard-and-lessons)),
   a bug and a catalog gap, with the prose rules (rule 4) for both
   (R8-12-19).
8. **Consequences, deferred items.** Four of ADR 0001's items are
   deferred, not three, each a row of the spec's
   [Deferred decisions](../spec.md#deferred-decisions): a scheduled
   workflow that checks external links, which Markdown linter and which
   spell checker `tools/ci docs` runs, generating part of the
   spell-check word list from the vocabulary table (those three under
   rule 5), and docs versioning per release (R8-adr-12).
9. **Consequences, external tools.** "It enters pinned through
   `mise.lock` like `reuse`" is replaced by a pointer: the tools mise
   may install for this repository are listed once, in
   [12 12.1](../spec/12-engineering.md#121-tech-stack), and `reuse` is
   not one of them (DR7).

### ADR 0002

10. **The reason for merge commits.** The Consequences bullet said the
    fix marker and the release notes read the commits a merge commit
    keeps on `main`. They read each pull request's commits from GitHub
    and do not depend on it. The reason that holds is the one of
    [12 12.9](../spec/12-engineering.md#129-delivery-practices): a pull
    request's commits reach `main` as they were written and signed, so
    `main`'s history is the reviewed commits (R8-12-11).

### ADR 0005

11. **Rule 1 is narrowed.** A security invariant, a merge gate and
    anything a step of the first-day path (J1 to J3, J5, J6) depends
    on are needs of the start; a dependency that exists only because a
    later section was written does not count. A part of one is
    deferred only as a risk the operator accepted by name in 05 5.4,
    and its Deferred row says so. "Anything a later step depends on"
    grew with every section written (DR4); the spec's
    [principle](../spec.md#decide-at-the-last-responsible-moment)
    holds the same text.

### ADR 0007

12. **The Threat model's pointer.** The two escapes that act on `go
    run` before `tools/ci` starts, an untracked `go.work` and a
    caller's `GOFLAGS`, were deferred to Q25 and to a plan row that
    exists nowhere. Q25 was decided by ADR 0008 on 2026-10-07, so the
    trigger fired. Closing them was decided on 2026-10-08 (DR3), and
    the form on 2026-10-09: the pre-push hook starts `tools/ci` with
    `GOENV=off`, `GOWORK=off` and `GOFLAGS=-mod=readonly`, which also
    closes a `go env -w` file and an untracked `vendor/`. The change is
    pending, a tooling pull request on the `checks` surface with its
    own approval line.
13. **Rule 8's tool list.** "`go` and `golangci-lint` the only tools"
    is replaced by a pointer to the one list in 12 12.1 (DR7).
14. **Where mechanism lives.** ADR 0007 carries pinned versions,
    function names and commit ids that `tools/ci` and `mise.lock`
    already hold. The rule of 12 12.5 applies from now on: a record
    holds the decision, the alternatives and the consequences, and
    mechanism, versions and identifiers belong to the spec or the
    generated reference. ADR 0007's text stays as decided; where it and
    `tools/ci` differ, `tools/ci` is right (R8-12-26, R8-adr-3).

### ADR 0008

15. **Scope.** ADR 0008 describes the development sandbox of this
    repository, which romeu does not manage, and the risks of how this
    repository is developed (05 5.4, the second group). A sandbox romeu
    renders forwards one key and holds the `github@<project>` binding
    (AR6). In its Context, "the sandbox" reads as that development
    sandbox (AR16).
16. **Reach.** The token's reach includes the repositories of every
    organization the operator's account is a member of, an employer's
    among them (AR16). The agent's reach, which the Consequences of ADR
    0008 give as the account's, also includes the maintainer's host,
    which runs agent-authored `tools/ci` through the pre-push hook and `hygiene
    add`. The host runs `tools/ci` only from a commit whose diff since
    the last host run the maintainer has read, or from the last
    released tag; a trusted `hooksPath` checkout is a Deferred row
    (AR12).
17. **Decision 5, the security log.** Its coverage was read on
    2026-10-09 (read after the decision) in GitHub's documentation (the page "Reviewing your
    security log" and the event list `src/audit-logs/data/fpt/user.json`
    of `github/docs` at commit
    `9f651797567230e844373870fce8b14427ad47ad`): the log of a personal
    account keeps 90 days and, for repositories the account owns,
    records ruleset creation, update and deletion, visibility, merge
    setting, default-branch, rename and archive changes, Actions
    settings, secrets and variables, and members. It records no pull
    request merge, no push, no tag creation, no branch deletion and no
    release or release-asset change. So the reading catches ruleset and
    those settings changes, and not merges, tags, branch deletions or
    releases. It stays a judgment, not a detector, and it is a habit,
    which the spec's
    [first principle](../spec.md#deterministic-where-it-can-be-the-model-where-it-adds-value)
    does not count as a control: DR5, which names that collision, was
    accepted on 2026-10-08. A live account's log was not read.
18. **Deferred item 1, its triggers.** "Every 2 autonomous work
    sessions" is replaced by a counted event: the second final handoff
    of this repository's development sessions since the last review,
    whose date the row holds and each review rewrites, counted by hand
    from the handoff record those sessions keep until julieta runs in
    the development sandbox. The count is of agent-written handoffs, and
    checkpoint C7 is the backstop. The first release-candidate tag is
    replaced by the plan's checkpoint before its last build phase
    (checkpoint C7), so the token is settled before the release work. The first issue
    or pull request from an account other than the operator's is a new
    trigger (AR4). The row in the spec's Deferred decisions holds the
    list; the security log is read as a judgment and is not a trigger
    of its own (DR5).
19. **Decision 6, release signing.** v1 releases carry `checksums.txt`
    and a keyless build provenance attestation, and no other signature.
    The attestation proves that `release.yml` built the archive, not
    that the maintainer approved the release. An operator signature
    over `checksums.txt`, made outside GitHub and out of the sandbox's
    reach, is a Deferred row; a release workflow still holds no signing
    credential (DR2).
20. **Consequences.** ADR 0007 joins the records whose stale pointer
    is corrected here (DR3). Text from issues, pull requests and
    comments is data to an agent that holds the token: it takes no
    merge, close, label, workflow or settings action on it without the
    maintainer's word (12 12.9, AR4).

### ADR 0007, its Consequences

21. **Upgrading mise.** Beside the bullet on upgrading golangci-lint,
    the counterpart for mise holds: upgrading mise can change which
    files it reads as its configuration, so the upgrade measures that
    discovery again at the new version, names the result in its
    Evidence, and changes the `dependencies` surface in the same pull
    request when the set changed ([05 5.3](../spec/05-security.md#53-ask-first-surfaces);
    R8-05-35).

## Consequences

- The six records named in Status gain a `Superseded in part by` line
  and stay Accepted. Their text is not
  edited, so a reader of one of them follows the line here for the
  parts that changed.
- `tools/ci sequences` has to accept the partial form and stop
  requiring contiguous numbers when it lands; the generated index
  already reads a record with that line as Accepted.
- Review stops being a claimed barrier and stays a practice. What
  holds against an agent that holds the token is in 05 5.4; this
  record adds no control.
- The pre-push hook change of decision 12 is pending. Until it lands,
  the escapes it closes stay open, and the pull request's diff review
  is the only guard against them.
- [ADR 0004, measure delivery with the five DORA metrics computed by a tool](0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md)
  stays Accepted as the design of a deferred tool, and
  [ADR 0006, record runtime events in an add-only ledger ingested on the host](0006-record-runtime-events-in-an-add-only-ledger-ingested-on-the-host.md)
  as the design of the deferred ledger; neither is amended here.
