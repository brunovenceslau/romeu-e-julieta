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
round 8 moved it here, word for word.

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
