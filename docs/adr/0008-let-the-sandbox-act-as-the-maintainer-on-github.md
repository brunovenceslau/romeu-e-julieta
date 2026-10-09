# 8. Let the sandbox act as the maintainer on GitHub

Date: 2026-10-07

## Status

Accepted

Superseded in part by [9. Drop the required review and amend the records that assumed it](0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md)

## Context

Inside the sandbox, GitHub sees the operator's own GitHub account, and
the operator is today the project's only maintainer. So the sandbox acts
as the maintainer on GitHub: it is the operator's account, not a role
the sandbox holds apart from it. We would rather know, before it
matters, what that lets the sandbox do.

The spec had assumed otherwise: a fine-grained token that could not
merge, tag or change a ruleset, with a second account as the fallback
if the maintainer block of `ci-bootstrap` showed a try that took effect
([Q25](../spec.md#open-questions), and
[round 4](../reviews/round-4.md#maintainer-decisions), row
op-second-account).

Item d of that block
([10 10.2](../spec/10-testing-style.md#release-and-bootstrap)) was run
on 2026-10-07 from inside the sandbox, with the operator's token and
the rulesets active. The evidence is kept outside this repository.

| Try | Result |
|---|---|
| push an empty commit to the default branch | refused |
| merge a throwaway pull request through the API, without asking for bypass | accepted |
| create and delete a `v*` tag | accepted, through the bypass |
| update the default-branch ruleset with its own unchanged body | accepted |

Only the direct push was refused. That an agent can disable a ruleset
or change repository settings is **inferred, not measured**: the token
holds administrator rights here, and the operator chose not to run a
try that changes a ruleset's body. The token reaches every repository
the operator's account can, with administrator rights on most of them
(inferred from the account's rights, not measured by these tries).
The forwarded SSH agent holds the signing key and a separate GitHub
authentication key.

## Decision

1. **For now, the sandbox acts with the operator's account on GitHub:**
   git identity, token and every API it reaches. This record says so
   so the record matches what was measured.
2. **The rule is written once**, in
   [05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1).
   Other pages link to it.
3. **What holds**, against an agent that does not edit rulesets or
   repository settings: no direct push to the default branch (measured),
   and merge commits only (a repository setting, not measured against the
   token). Q13 and I22 are unchanged: the project's token is today
   the operator's.
4. **What is process only**, against an agent that holds the token: a
   merge, a `v*` tag, a change to a ruleset or a setting, and with them
   the independence of the code-owner review (only an unchanged-body
   ruleset update was measured; real ruleset and settings edits are
   inferred). Approval lines, reviews
   and required checks stay as a record and a guard against mistakes,
   not as a barrier against an agent that ignores them.
5. **The operator reads the GitHub security log at each sitting**, for
   an action by this token that was not asked for. It is a judgment, not
   a check, and whether the log covers merges, tag pushes and ruleset
   updates on a personal account is **unverified**.
6. **The credential that signs release artifacts is held outside GitHub
   and out of the sandbox's reach.** Signing is an operator step, and a
   release workflow may not hold that credential. GitHub's keyless
   provenance attestation (`actions/attest-build-provenance`, described
   in [10](../spec/10-testing-style.md)) is an extra record, not the
   signing credential.

What the token can reach, in classes:

- the `workflow` scope runs branch workflows with repository secrets;
- settings, webhooks, deploy keys and collaborators persist after the
  sandbox is gone;
- branches, tags and releases can be deleted;
- gists are an exfiltration channel, and domain-only egress does not
  stop exfiltration to GitHub itself;
- issue and PR text, fetched pages and dependencies can steer the agent.

### Alternatives considered

- **A second GitHub account without bypass rights** (the fallback of
  Q25). It would restore the independence of the code-owner review.
  Declined by the operator; the cost it avoids is an account to create,
  secure and keep in step with the rulesets.
- **A fine-grained token now**, limited to this repository and without
  Administration. It would stop ruleset edits and the reach to other
  repositories, but a token with write access to pull requests can still
  merge. Deferred by the operator (first deferred decision).
- **A GitHub App.** The cleanest "who is acting", at the price of an
  application to register, host keys for and rotate. Not needed now; it
  reopens with the first deferred decision.
- **Keep the spec as it was.** It would describe refusals that were not
  found.

### Deferred, each with its trigger

Each goes to the spec's
[Deferred decisions](../spec.md#deferred-decisions).

1. **Narrow the token and stop forwarding the authentication SSH key.**
   Reopened by the first of: an agent action nobody asked for (seen only
   if the security log covers it, unverified); a second contributor; the
   first release-candidate tag; every 2 autonomous work sessions (from a
   resume to its handoff checkpoint), moving to every 10 once review is
   cheap. The interval change is itself deferred to the operator's word.
2. **Releases from `main` only.** Reopened by the pull request that adds
   `release.yml`. One constraint is fixed now: the workflow file must
   not come from the tagged commit, or whoever pushes a tag chooses the
   code that builds and signs. The rule also stops a stray tag, not the
   token, which can merge through the API; decision 6 covers that.
3. **Re-read the rulesets in a check**, since `tools/ci all` is offline.
   Reopened by the first of: the token is narrowed, or `release.yml`
   lands.
4. **Required checks without the bypass.** Reopened by item e of the
   maintainer block.

## Consequences

- **Rule 1 of [ADR 0005, decide at the last responsible moment and
  record the trigger](0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md)**
  says a merge gate is never deferred. The gate exists and does not bind
  the sandbox's token, and this record defers narrowing that token. The
  operator accepted that collision by name on 2026-10-07, and with it
  that a merge, a `v*` tag and a ruleset change are process only until a
  trigger above reopens the token narrowing.
- The agent's reach is the operator's account's. A mistake or a hostile
  instruction inside a sandbox can act across it. Egress rules are the
  other limit ([05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1),
  the row on `github.com`).
- A `v*` tag will run `release.yml`, which does not exist yet. Until
  then the release path is process only.
- Round 4's row op-second-account is superseded by pointer: its
  fallback is declined here, and the row stays as history.
- Q25 is decided by this record. The residual-risk rows of 05 5.4 that
  said the token must not merge, tag or change a ruleset are replaced by
  the one row of decision 2.
- Texts that relied on those refusals now point to 05 5.4, which says
  what the review is worth. We do not rewrite them. They are the
  code-owner and merge-gate text of
  [12 12.4](../spec/12-engineering.md#124-middleware-before-and-after-every-change)
  and, where they cite the review, these records:
  [ADR 0001, adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md),
  [ADR 0003, adopt six Extreme Programming practices and review as pairing](0003-adopt-six-extreme-programming-practices-and-review-as-pairing.md)
  and [ADR 0004, measure delivery with the five DORA metrics computed by a tool](0004-measure-delivery-with-the-five-dora-metrics-computed-by-a-tool.md).
