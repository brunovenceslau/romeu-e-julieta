---
type: explanation
reader: the maintainer running or re-reading the maintainer block of ci-bootstrap
---

# The maintainer block of ci-bootstrap, as it ran

Reader: anyone who wants to know how this repository's GitHub settings
were made and measured. Type: explanation.

This note keeps the record of a procedure that ran once: the
maintainer block of `ci-bootstrap`, with the dates and results of its
measurements. It is not part of the specification. What the block set
up and what stays to do are in
[10 10.2](../spec/10-testing-style.md#release-and-bootstrap); what the
rulesets are worth against an agent that holds the operator's token is
in [05 5.4](../spec/05-security.md#risks-of-how-this-repository-is-developed).
The text below is the block as the specification held it before review
round 8 moved it here, word for word, up to the section
[The token narrowing](#the-token-narrowing), which is a record kept
since (item g of 10 10.2).

**The maintainer block.** The parts of `ci-bootstrap` that only the
maintainer can do are one sitting around step 1 and one call after
step 2. The maintainer runs every item. The output of each goes under
the Evidence of the PR it belongs to; no check reads that text, so it
is **[review]**.

The sitting starts with item a, before the sandbox that writes step 1
receives its token, or at the latest before item b: until the rulesets
exist, nothing refuses that token a push to the default branch, a push
of a `v*` tag or a merge. Once they exist, of the four tries only the
push is refused (item d), a control against an agent that does not edit the ruleset or the repository settings (05 5.4). Nothing of step 1 is pushed before the
sitting. The agent that wrote step 1 does not push its branch. The
maintainer fetches the branch from the sandbox into a plain clone on
the host and reads its diff there before checking it out, because
items b and c run code and a hook of that branch on the host, outside
any sandbox and any agent session.

- a. Set the repository up and save the API's answer for each setting.
  Two rulesets. On the default branch: changes arrive by pull request,
  with one approving review and a code-owner review; an approval is
  dismissed when a new commit is pushed, and the most recent
  reviewable push must be approved; a merge commit is the only merge
  method; force pushes and deletion are refused; updates to the
  default branch are restricted to the bypass actor, so a merge by any
  other account is refused whether or not it is approved (a GitHub
  behaviour that item d measured against the sandbox's token: it does
  not hold for a merge through the API,
  [05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1)). On
  tags matching `v*`: creation, update and deletion are refused for
  everyone except the bypass actor. The administrator role is the
  bypass actor of both, and the operator's token holds that role. Two repository settings, which bind a
  bypass actor too: squash merging and rebase merging off, so every
  merge is a merge commit (12 12.9); and private vulnerability
  reporting on, the channel `SECURITY.md` names (12 12.8). If GitHub
  refuses a branch ruleset on a repository whose default branch does
  not exist yet (not tested against GitHub), the maintainer creates
  the repository with an empty initial commit first.

  As measured on 2026-10-08, the default-branch ruleset requires no
  review: no approving review, no code-owner review, no approval of
  the most recent push, and a push dismisses nothing; that requirement
  was removed. This note supersedes the rest of this item where they
  differ. As measured on 2026-10-09 (through the GitHub API), the
  restriction of updates to the bypass actor was removed too. With it
  in place, pull requests showed `mergeStateStatus` `BLOCKED`, and
  after it was removed they showed `CLEAN`, and the stack of #28, #29
  and #30 then merged atomically as one merge commit. The ruleset
  (id 24611273), read again that day, holds these rules: deletion and
  non-fast-forward refused, a pull request required (no approving
  review, no code-owner review, a merge commit the only method), and a
  required-status-checks rule that lists the four `all on ...` checks.
  The bypass actor is unchanged (administrator role, for pull
  requests). By the pull-request rule, a direct push to the default
  branch is still refused (the rule's effect, not re-measured on that
  day). Read from these rules, and not tried with a second account:
  since the update restriction is gone and no review is required, an
  account with write access can merge a pull request whose required
  checks are green. The rest of this item is as measured when it was
  set up. The rules that lean on the code-owner review (05 5.4,
  12 12.4, 12 12.9, ADR 0001 rule 8) stand as written until the
  decision on what replaces the code-owner review, in the
  [Deferred decisions](../spec.md#deferred-decisions) table, is made.
- b. In that clone, run `go run ./tools/ci hygiene add` once for each
  forbidden name, and commit `tools/ci/denylist.yaml`.
- c. Enable the hook in that clone (`git config core.hooksPath
  .githooks`), push the branch, so the hook reads the bootstrap
  commits themselves (their messages, identities, paths and ref name),
  and open step 1's PR.
- d. From inside the sandbox, where the one credential is the
  operator's token, try four things with throwaway payloads and read
  what each does. Each push is tried from a clean working tree with HEAD
  at the commit it pushes, so that `fast` passes and a refusal comes from
  GitHub, not from the hook. The result is a measurement and not a gate: the
  sandbox acts with the operator's account, and
  [05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1) states
  the rule once. After the four tries the maintainer checks from their
  own session what took effect: the default branch head, the tag list,
  the throwaway pull request, and each ruleset's JSON against the answer
  saved in item a, whatever the sandbox printed. The maintainer undoes
  what can be undone (the tag deleted, the merge reverted, the ruleset
  restored; an empty commit stays and harms nothing), and then closes
  the throwaway pull request and deletes its branch. Measured on
  2026-10-07 ([ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md));
  the evidence is kept outside this repository:

  | Try | Result |
  |---|---|
  | push an empty commit to the default branch | refused; a control only against an agent that does not edit the ruleset or the repository settings (an edit is inferred, not measured) |
  | merge a throwaway pull request through the API, without a bypass request | accepted |
  | create and delete the tag `v0.0.0-try` | accepted, through the bypass |
  | update the default-branch ruleset with its own unchanged body | accepted; the ruleset did not change |

  Of the four tries, only the direct push was refused.
- e. After step 2: add the CI jobs of the green run to the
  default-branch ruleset as required status checks, and save the API's
  answer.

## The token narrowing

The record of the narrowing of the development sandbox's GitHub token
to the GitHub App installation token that
[ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md),
decision 2, proposes and which is applied once that record is
Accepted, as item g of
[10 10.2](../spec/10-testing-style.md#release-and-bootstrap) asks for
it, and the one home of the rule below. The maintainer adds one dated
entry when the narrowing is applied, and one for each later read, which
is due at each sitting of blocks O3, O6 and O7 of the
[plan](../plan.md#operator-blocks). An entry is a heading
`### <date>: narrowed` or `### <date>: not narrowed`, followed by the
three reads; when two entries share a date, each heading carries the
time after the date, and the entries are in the order of their
headings, oldest first. The first entry whose verdict is narrowed has a
fourth part, the answers of the required-name try of item h of
[10 10.2](../spec/10-testing-style.md#release-and-bootstrap), which runs
only while the newest entry's verdict is narrowed; the fourth part
opens with the date the try was run. The first entry replaces the line "No entry yet"
at the end of this section. The first entry whose verdict is narrowed
is the event of the four
[Deferred decisions](../spec.md#in-how-this-repository-is-run) rows
whose trigger is "the token is narrowed". The token narrowing row of
the index describes the token before and after the narrowing; the
token row of
[05 5.4](../spec/05-security.md#risks-of-how-this-repository-is-developed)
describes the state before it, in its present words, and points to
the index row for the state after. No entry edits either: the
newest entry's verdict says which of the two states holds. A
later entry whose verdict is not narrowed puts the token back in the
state before the narrowing, and is an event of those four rows too,
so the decision each took on the narrowed premise is reviewed
again; the rows the first narrowed entry fired stay fired. No check
reads the entries, so they are **[review]**.

Each entry holds three reads, made on the host or checked from the
maintainer's own session, never only from what the sandbox printed:

1. Probe A16
   ([11 11.1](../spec/11-host-probes.md#111-block-a---sbx-and-runtime-facts)):
   the token's type, from its prefix, and, for a token with no scopes
   header, its repository list and permissions from GitHub's settings
   page of that token or of the App's installation. Expected: an App
   installation token, an installation on selected repositories, this
   one among them, Contents, Pull requests and Workflows read and write,
   Actions, Checks and Metadata read, and no Administration.
2. The agent the host forwards into the sandbox, read on the host, in
   three parts: the host's forwarding setting, the sbx setting or kit
   that names the socket it forwards; the keys of that socket, listed by
   `ssh-add -l` on the host with `SSH_AUTH_SOCK` set to it; and the
   GitHub account's SSH keys page, read in the maintainer's session.
   Whether sbx forwards the agent of the shell that starts it when no
   setting names a socket is inferred, not measured, so the read does
   not rely on the setting alone: when no setting names a socket, the
   socket the second part lists, whose keys the third part checks
   against the page, is the `SSH_AUTH_SOCK` of the shell that starts
   the sandbox, on the host. Expected: every socket so listed holds
   the signing key alone, or no agent is behind it; and the
   fingerprint of each key it holds is absent from the page's
   authentication
   keys, the signing key's being listed only under signing keys, the
   rule
   [06 6.4](../spec/06-kits.md#64-product-kits) states for romeu's
   sandboxes. The sandbox's own `ssh-add -l` may be read too, as a
   second read, never as the proof.
3. The four tries of item d above, repeated from inside the sandbox
   with the narrowed token, with throwaway payloads and the same undo
   and check from the maintainer's session. The tag try runs only while
   no `release.yml` exists, since a `v*` tag would start a release once
   it does; a read made once `release.yml` exists leaves it out and
   records the tag answer as not measured:

   | Try | Answer in item d, with the operator's token | Expected with the narrowed token |
   |---|---|---|
   | push an empty commit to the default branch | refused | refused, by the pull-request rule; this answer does not tell the two tokens apart |
   | merge a throwaway pull request through the API | accepted | accepted once its required checks are green, since the token keeps Contents and Pull requests write; this answer does not tell the two tokens apart |
   | create and delete the tag `v0.0.0-try` | accepted, through the bypass | not predicted: whether the tag ruleset's bypass for the administrator role applies to an App installation token is not measured, and the answer is recorded either way |
   | update the default-branch ruleset with its own unchanged body | accepted | refused, since the token holds no Administration permission |

The token is narrowed when the A16 read and the three parts of read 2
match what is expected and the ruleset update is refused. Any other answer
on those three means it is not, and the entry says which read
differed.

Limits of the entry, stated and not closed:

1. Nothing acts on an accepted tag try; the pull request that adds
   `release.yml` reopens it (round 15).
2. No read looks for deploy keys, webhooks or collaborators that the
   operator's token could have added before the narrowing, a class
   ADR 0008 lists; the review of the token narrowing row of
   [Deferred decisions](../spec.md#in-how-this-repository-is-run)
   reopens it (round 15).
3. How the forwarding stops is the maintainer's choice: ADR 0011,
   decision 2, says only that it stops. An entry whose read 2 finds
   an authentication key, or a second key in a forwarded socket,
   reopens it (round 16, R16-11).
4. A later read is due at a sitting, not at the events that change what
   it reads (a re-created sandbox, `sbx secret set` run again, a
   replaced token, the maintainer's pushes from the host), and none is
   due after block O7; the required-name try of item h is not run again
   for a replaced token either, so its answers describe the token of
   the first narrowed entry (round 17, R17-13; round 18, R18-A12).
5. Nothing writes into the token narrowing row of the index or the
   token row of 05 5.4 that the narrowing holds; an entry whose
   verdict is not narrowed after a narrowed one asks for no decision of
   its own, and a narrowed entry that follows a not narrowed one after
   the first narrowed entry is no event, since the rows the first
   narrowed entry fired stay fired: the rows
   are reviewed at their events (round 17, R17-15, R17-20; round 18,
   R18-A12).
6. Read 3's merge try, made once the required checks are green, is
   accepted with either token, so it cannot show whether a token
   bypasses those checks (round 17, R17-18).

Limits 4 to 6 are reopened, after the sitting of block O8 of the
[plan](../plan.md#operator-blocks) has written the first entry, by the
first of: a re-created development sandbox, a replaced token, the
first release-candidate tag, or the plan's checkpoint C7 (round 18,
R18-A13).

No entry yet: the narrowing waits for the acceptance of ADR 0011 and
a maintainer step.

## Review log

The dated reads and reviews that the index's token narrowing row and
the token rows of 05 5.4 used to carry, one entry each, newest last;
the rows cite this section.

- 2026-10-08: of four tries with the operator's token, only a direct
  push to the default branch was refused (10 10.2, item d).
- 2026-10-09: the ruleset read through the API answers
  `pull_requests_only` for the bypass actor, and the ruleset-disable
  try is not made; the token's scopes, `gist, read:org, repo, workflow`,
  read from a response header in the sandbox, not on the host (A16's
  host read is pending); GitHub's documentation of the security log
  read at commit `9f651797567230e844373870fce8b14427ad47ad` of
  `github/docs`: ruleset and listed settings changes, no merge, push,
  tag, branch deletion or release.
- 2026-10-09: review of the token narrowing row; four final handoffs
  counted (2026-10-08 and 2026-10-09 in round-8.md, 2026-10-09 in
  round-9.md and in round-10.md): narrowing decided.
- 2026-10-12: review of the token narrowing row; the GitHub App route in
  place of the fine-grained token, ADR 0011, decision 2, which stays
  proposed until the App checks its Consequences lists.
