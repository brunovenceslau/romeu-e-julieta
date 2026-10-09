# 11. Bind the administrator to the required checks and narrow the sandbox's GitHub token

Date: 2026-10-09

## Status

Accepted

Supersedes in part [8. Let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md)

Supersedes in part [10. Reopen the required checks without the bypass and amend the records of round 9](0010-reopen-the-required-checks-without-the-bypass-and-amend-the-records-of-round-9.md)

## Context

Three questions on how this repository is merged were open after
review rounds 9 and 10, each with its item in a decision block:

- whether the required checks bind the administrator role too, the
  decision that decision 1 of
  [ADR 0010, reopen the required checks without the bypass and amend the records of round 9](0010-reopen-the-required-checks-without-the-bypass-and-amend-the-records-of-round-9.md)
  reopened with its options open
  ([round 9](../reviews/round-9.md#decisions-for-the-operator), D4);
- whether the development sandbox's GitHub token is narrowed, the
  review that deferred item 1 of
  [ADR 0008, let the sandbox act as the maintainer on GitHub](0008-let-the-sandbox-act-as-the-maintainer-on-github.md)
  asks for at its triggers, which fired with three final handoffs
  counted on 2026-10-09
  ([round 9](../reviews/round-9.md#decisions-for-the-operator), D2;
  [round 10](../reviews/round-10.md#decisions-for-the-operator), D4);
- how the spec states the refusal of a direct push to the default
  branch, tried on 2026-10-08 under rules that changed on 2026-10-09
  ([round 10](../reviews/round-10.md#decisions-for-the-operator), D5
  and D6).

The rulesets, as read through the GitHub API on 2026-10-09:

- ruleset 24611273, `default-branch`, has one bypass actor, the
  repository role admin, in mode `pull_request`, and the rules
  `deletion`, `non_fast_forward`, `pull_request` (0 approvals, merge
  commits only) and `required_status_checks` (the four `all on ...`
  checks, with `strict` false);
- ruleset 24611279, `release-tags`, is unchanged;
- each of the 12 most recent merges, pull requests #22 to #38, had a
  `SUCCESS` status rollup, so none of them needed the bypass.

The token's scopes, `gist, read:org, repo, workflow`, were read on
2026-10-09 from the `x-oauth-scopes` header of an API response inside
the sandbox. Probe A16 reads them on the host, where the token is
held, and that read is still to be made
([11 11.1](../spec/11-host-probes.md#111-block-a---sbx-and-runtime-facts-first-in-parallel-with-the-first-build-layer)).

The final handoffs counted on 2026-10-09 were four: 2026-10-08 and
2026-10-09 in round-8.md, 2026-10-09 in round-9.md and in
round-10.md, by the command the spec's token-narrowing row names.

### Alternatives considered

- **Keep the administrator bypass on the default branch** (option (b)
  of ADR 0010, decision 1). It keeps a merge on the local gates
  possible without a ruleset edit. Declined on 2026-10-09: none of the
  last 12 merges used the bypass, and a bypass that is never needed is
  a path around the checks for an agent that holds the token.
- **Keep the token and review it again at the next trigger.** The
  review is due, and the token reaches every repository of the
  account, with administrator rights on this one and the `gist` scope,
  which this repository does not need. Declined on 2026-10-09.
- **A GitHub App in place of a token.** Still not needed; ADR 0008
  states its cost, and a fine-grained token removes the reach that
  matters now.
- **Try the direct push again now** (option (a) of round 10's D6). The
  try belongs with the other tries after the token narrowing, so the
  four tries measure the token that will stay. Declined on 2026-10-09.

## Decision

1. **The required checks bind the administrator too.** This decides
   decision 1 of ADR 0010 as its option (a): the default-branch
   ruleset has no bypass actor, so every merge, the administrator's
   included, needs the four `all on ...` checks. It applies when the
   API answer of ruleset 24611273 shows no bypass actor, after a
   maintainer step: remove "Repository admin" from the bypass list of
   ruleset 24611273, and save the API answer under the Evidence of the
   pull request that records it. The `release-tags` ruleset keeps the
   administrator role as its bypass actor. A merge on the local gates,
   when Actions cannot run, then needs a ruleset edit by the
   maintainer.
2. **The development sandbox's GitHub token is narrowed.** The review
   that deferred item 1 of ADR 0008 asks for is done, with four final
   handoffs counted on 2026-10-09, and narrowing is decided. The
   target is a fine-grained token of the same user, limited to this
   repository, with the repository permissions Contents, Pull requests
   and Workflows read and write; Actions, Checks and Metadata read; and
   no Administration. The forwarding of the GitHub authentication SSH
   key to the sandbox stops too. Applying both is a maintainer step on
   the host: create the token, set it as the sandbox's `github` secret
   with `sbx secret set`, and stop forwarding the key. Until then the
   current token stands. The Deferred rows whose trigger is "the token
   is narrowed" fire when it is applied: the check that reads the
   rulesets again and the signed approval record. After it, probe A16
   and the four tries of item d of the maintainer block run again
   ([10 10.2](../spec/10-testing-style.md#release-and-bootstrap)).
3. **The direct-push refusal is read from the rules, not tried.** The
   default-branch ruleset requires a pull request and the four checks,
   and its bypass actor bypasses for pull requests only, so a direct
   push to the default branch is refused to every actor, the bypass
   actor included: read from the rules on 2026-10-09, where the API
   answers `pull_requests_only` for the token, and not tried. The try
   of 2026-10-08 (ADR 0008) predates the ruleset change of 2026-10-09;
   it is repeated with the four tries after the token narrowing.

## Consequences

- ADR 0008 and ADR 0010 gain a `Superseded in part by` line and stay
  Accepted; their text is not edited. Decision 1 replaces decision 1
  of ADR 0010; decision 2 decides deferred item 1 of ADR 0008, and
  decision 3 qualifies the direct-push row of its table, which stays
  the record of 2026-10-08.
- The spec changes with this record: the Deferred row "required status
  checks without the administrator bypass" leaves the index, and that
  part leaves the merge-gate parts 05 5.4 lists; S2's evidence says the
  ruleset requires the checks of every merge; item a of the maintainer
  block names the administrator role as the bypass actor of the tag
  ruleset only; the local-gates row says a merge on the local gates
  needs a ruleset edit; and the token row of 05 5.4 and the
  token-narrowing row of the index say what was decided and what is
  pending.
- Until the two maintainer steps are applied, the bypass and the
  current token stand, and the spec says so where it names them.
- The token-narrowing row of the index keeps its other events, and its
  handoff count starts again after 2026-10-09: the review returns at
  the second final handoff after that day, or at the sitting that
  applies the narrowing.
