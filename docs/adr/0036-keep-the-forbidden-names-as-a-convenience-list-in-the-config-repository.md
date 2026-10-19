# 36. Keep the forbidden names as a convenience list in the config repository

Date: 2026-10-19

## Status

Accepted

The spec text it changes (REQ-0885, REQ-0886, REQ-1139) changes when
the sandbox gate runs.

## Context

The forbidden names are the names of earlier tools the maintainer built
to try out the ideas that became romeu, julieta and the config
repository. To the maintainer they are earlier versions of the same
application, and several were never published, so a reader who meets
one of those names in a public repository is left looking for a tool
that does not exist. The list keeps them out of public text for that
reason only: it is a convenience, and it is not secret.

What is at stake: a name that slips into a public repository confuses a
reader for a while. A correcting commit takes it out of the current
text; the history and any fork keep it, and that is accepted as a
convenience lost, since no money and no credential ride on it.

That context was never written down with the rule. The spec says only
that the list "holds names of other projects this repository must not
mention" (10 10.2, Forbidden names), and the working rules call the
gate a step before every push because a public repository is hard to
clean. Read without the stakes, each session raised the rigor: names
stored as segment hashes so the list could sit in this public
repository (REQ-0885), the same check over pull request text in CI
(REQ-1139), and a plan for a hashed copy and a tracked hook in every
guarded repository (item 18 of Decisions of the re-plan, T112). Every
public repository of the maintainer may receive a push from an agent,
so a gate per repository also means one list to keep in step per
repository.

### Alternatives considered

- **A hashed denylist and a tracked hook in each public repository**
  (item 18, mechanism (a)). Not taken: one copy per repository to keep
  in step, and a hash guards a list that is not secret; short names
  hash to values that are easy to reverse anyway.
- **The config repository cloned into every sandbox** (item 18,
  mechanism (b)). Not taken: it brings every private file of that
  repository into every sandbox, where the gate needs one list.
- **One plain list in the config repository and one gate in the
  sandbox, for every push to a public remote.** Taken.

## Decision

Decided on 2026-10-19:

1. The list lives once, as plain text, in the maintainer's private
   config repository. Adding a name is a commit there.
2. The sandbox kit that installs the system hook dispatcher also
   installs the gate and the list, read-only. On every push the
   dispatcher runs the gate before the repository's own hook.
3. The gate acts when the push goes to a public repository, and when
   the visibility cannot be measured, which counts as public; a push to
   a private repository passes it. Visibility is read with `gh repo
   view --json visibility` for the remote's repository; a remote that
   is not on GitHub, or a read that fails, is unmeasured. The gate's
   refusal says which it was: a name found, or a visibility it could
   not measure.
4. A `git push` or `git commit` with `--no-verify` (or `git commit -n`)
   that an agent session runs asks the maintainer first, in every
   sandbox, through the agent's permission settings, so the gate is
   skipped only on a decision made in the session.
5. The hashed denylist of this repository and the CI check over pull
   request text stay until the sandbox gate runs, and then go.

## Consequences

- T112 is re-planned on this record: its work lands in the sandbox kit
  repository and in the config repository, under their own rules.
- The spec text that states the hashed form (REQ-0885, REQ-0886) and the
  CI check over pull request text (REQ-1139) changes in the pull request
  that removes them, after the sandbox gate runs; until then they hold.
- Item 18 of Decisions of the re-plan and the deferred row "a Go port
  of the names gate of the config repository" are amended to point
  here.
- A public repository added later is covered from its first push, with
  no setup of its own.
- The gate stops a mistake in a session; a push made outside the
  sandboxes is not covered, which the stakes above accept.
- Once the CI check goes, the text of pull requests, issues and
  comments is no longer gated; it can be edited after the fact, which
  the stakes above accept.
- A repository that sets its own `core.hooksPath` takes the system
  dispatcher out of its pushes; T112 checks that case.
