# 11. Keep the administrator's pull-request bypass and give the sandbox a GitHub App token

Date: 2026-10-12

## Status

Proposed

It becomes Accepted in a later pull request, after the App checks that
Consequences lists.

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
  asks for at its triggers, which fired with final handoffs counted
  on 2026-10-09, three when D2 was recorded and four at review
  ([round 9](../reviews/round-9.md#decisions-for-the-operator), D2;
  [round 10](../reviews/round-10.md#decisions-for-the-operator), D4);
- how the spec states the refusal of a direct push to the default
  branch, tried on 2026-10-08 under rules that changed on 2026-10-09
  ([round 10](../reviews/round-10.md#decisions-for-the-operator), D5
  and D6).

This record was first written on 2026-10-09 with the status Accepted:
its decision 1 removed the administrator role from the bypass list of
the default-branch ruleset, and its decision 2 replaced the token with
a fine-grained token of the same user. Neither maintainer step was
applied. On 2026-10-12 both decisions were made again, as below, and
the record returned to Proposed until the App checks run
([round 18](../reviews/round-18.md#session-checkpoint-2026-10-12)).

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

Facts read on 2026-10-12 from GitHub's documentation
([round 18](../reviews/round-18.md#facts-read-for-the-github-app-route)):

- a fine-grained personal access token is created only in the web UI,
  and no API creates one
  ([managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens));
- a GitHub App of a personal account can be installed on selected
  repositories
  ([about creating GitHub Apps](https://docs.github.com/en/apps/creating-github-apps/about-creating-github-apps/about-creating-github-apps));
- `POST /app/installations/{installation_id}/access_tokens` takes
  `repositories` and `permissions`, and its token expires one hour
  after creation
  ([create an installation access token for an app](https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app));
- the installation token serves git over HTTPS when the App has the
  Contents permission
  ([authenticating as a GitHub App installation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation));
- ruleset bypass modes are `always`, `pull_request` and `exempt`, and
  the actor types include Integration (an App), RepositoryRole and
  User ([rules](https://docs.github.com/en/rest/repos/rules));
- GitHub Pro includes 3,000 Actions minutes per month for private
  repositories, public repositories on standard runners are free, and
  usage is blocked when the quota runs out only if no payment method is
  on file
  ([about billing for GitHub Actions](https://docs.github.com/en/billing/managing-billing-for-github-actions/about-billing-for-github-actions));
- `sbx secret set` stores a dynamic secret with `--command` and
  `--refresh` (default 55m), scoped globally or with `--sandbox`, and
  has no per-repository scope (read from the sbx help text).

Not verified: the `[bot]` attribution of the pull requests and comments
an App makes; `gh` with an installation token in `GH_TOKEN`; how a
bypass applies to a personal access token whose owner is in the bypass
list.

### Alternatives considered

- **Remove the administrator bypass from the default branch** (option
  (a) of ADR 0010, decision 1, and this record's decision 1 as written
  on 2026-10-09). Declined on 2026-10-12: GitHub Actions minutes run
  out on private repositories, and some changes cannot fail CI.
- **Keep the token and review it again at the next trigger.** The
  review is due, and the token reaches every repository of the
  account, with administrator rights on this one and the `gist` scope,
  which this repository does not need. Declined on 2026-10-09.
- **A fine-grained token of the same user** (this record's decision 2
  as written on 2026-10-09). Declined on 2026-10-12: it is created
  only in the web UI, so no command on the host can mint it for an sbx
  dynamic secret, and it lives until it is revoked or expires, where an
  installation token expires within the hour.
- **Try the direct push again now** (option (a) of round 10's D6). The
  try belongs with the other tries after the token is replaced, so the
  four tries measure the token that will stay. Declined on 2026-10-09.

## Decision

1. **The administrator keeps its pull-request bypass on the default
   branch.** This decides decision 1 of ADR 0010 as its option (b):
   the repository role admin stays in the bypass list of ruleset
   24611273, in `pull_request` bypass mode, so the maintainer can merge
   a pull request whose required checks are not green. The reasons:
   GitHub Actions minutes run out on private repositories, and some
   changes cannot fail CI. Once decision 2 is applied, the sandbox's
   App token does not hold the administrator role, so the bypass is the
   maintainer's and not an agent's, which the App checks measure; until
   then the classic token holds that role. Reopened by the first
   measurement in which a merge with the sandbox's token lands over a
   red required check. No standing read looks for such a merge, so the
   trigger is seen only when a measurement or a sitting finds one. The
   `release-tags` ruleset keeps the administrator role as its bypass actor too.
2. **The development sandbox's classic token is replaced with a GitHub
   App installation token.** The review that deferred item 1 of ADR
   0008 asks for is done. The token is minted on the host by a helper
   from an App installed on selected repositories, this one among them,
   with the repository permissions Contents, Pull requests and
   Workflows read and write; Actions, Checks and Metadata read; and no
   Administration. It is delivered as an sbx dynamic secret,
   `sbx secret set github --command <absolute host helper path>
   --refresh 55m`, which refreshes it before its hour runs out. The
   helper and the App's private key live outside every path mounted
   into any sandbox, and are not writable from any sandbox. The
   secret is global, so every sandbox holds the same token; isolation
   per repository is deferred, and it is reopened by a repository that
   a sandbox must not reach. The forwarding of the GitHub
   authentication SSH key to the sandbox stops too. The four Deferred
   rows whose trigger is "the token is narrowed" fire when it is
   applied, among them the check that reads the rulesets again and the
   signed approval record. After it, probe A16 and the four tries of item d of the
   maintainer block run again
   ([10 10.2](../spec/10-testing-style.md#release-and-bootstrap)).
3. **The direct-push refusal is read from the rules, not tried.** The
   default-branch ruleset requires a pull request and the four checks,
   and its bypass actor bypasses for pull requests only, so a direct
   push to the default branch is refused to every actor, the bypass
   actor included: read from the rules on 2026-10-09, where the API
   answers `pull_requests_only` for the token, and not tried. The try
   of 2026-10-08 (ADR 0008) predates the ruleset change of 2026-10-09;
   it is repeated with the four tries after the token is replaced.

## Consequences

- Until this record is Accepted, the spec states what holds today: the
  classic token, and the bypass as read on 2026-10-09. It names this
  record only as the proposed route.
- The App checks run before this record is Accepted, on a throwaway
  pull request, with the token the host helper mints: a signed commit
  pushed with the token keeps its Verified mark; a pull request and a
  comment made by `gh` with the token show the App's bot as the author;
  the agent's merge with a red required check is refused; a ruleset
  update is refused; the maintainer's bypass merge works; the push
  starts the workflow; from inside a sandbox, neither the host helper
  nor the App's private key is readable; and which `gh` commands fail
  with an installation token.
- The pull request that accepts this record adds a
  `Superseded in part by` line to ADR 0008 and to ADR 0010, which stay
  Accepted with their text unedited, and this record's
  `Supersedes in part` lines, and it removes the Deferred decisions row
  of the required checks without the administrator bypass. Decision 1 then decides decision 1
  of ADR 0010; decision 2 decides deferred item 1 of ADR 0008, and
  decision 3 qualifies the direct-push row of its table, which stays
  the record of 2026-10-08.
- Once Accepted, decision 2 is applied at block O8 of the plan, and the
  first entry whose verdict is narrowed in the note on the maintainer
  block records it; the token stands until that entry.
- The token-narrowing row of the index keeps its other events, and its
  handoff count starts again after 2026-10-12: the review returns at
  the second final handoff after that day, or at the sitting that
  applies the replacement.
