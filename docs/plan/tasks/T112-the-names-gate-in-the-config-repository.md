---
id: T112
title: "The names gate in the config repository"
milestone: M1
module: none
fate: new
owns: []
needs: []
after: []
ask_first: []
sitting: M1
---

# T112 - The names gate in the config repository

Goal: One forbidden-names gate for every push from a sandbox to a
public repository, on
[ADR 0036, keep the forbidden names as a convenience list in the config repository](../../adr/0036-keep-the-forbidden-names-as-a-convenience-list-in-the-config-repository.md):
the list as plain text, once, in the maintainer's private config
repository; the sandbox kit that installs the system hook dispatcher
also installs the gate and the list, read-only, and the dispatcher runs
the gate before a repository's own pre-push hook when the remote is
public or its visibility cannot be measured; and `--no-verify` on `git
push` or `git commit` (and `git commit -n`) asks the maintainer first in
every sandbox. The phrase list of the lesson of 2026-10-12 (pending item
aea4a5dd; its phrases stay in the private home, never written here)
joins the same list as a second section.

No file of this repository: the work lands in the sandbox kit
repository and in the config repository, as ask-first changes under
their own rules, and this file is its record and its checkbox. The pull
request whose Evidence carries the verification names this task in its
body and writes `merged:` and `pr:` into this file, the one write rule
1 allows outside its own file.

Re-plan (2026-10-19): Rewritten on ADR 0036. The plan of 2026-10-12
called this an irreversible-harm surface and chose a hashed denylist
and a tracked hook in each guarded repository (item 18 of Decisions of
the re-plan, mechanism (a)); a name that slips costs one correcting
commit, so one plain list and one gate replace them. The hashed
denylist of this repository and the CI check over pull request text go
once this gate runs, in a pull request of this repository on the
`checks` and `spec` surfaces.

- Implements: none (12 12.11: the names gate of the config repository is
  no obligation of this specification).
- Operator: approves the kit and config changes, and runs the
  verification.
- Acceptance:
  - A push from a sandbox to a public repository runs the gate through
    the dispatcher, with no file of that repository involved.
  - A push of a commit that holds a planted made-up name, added to the
    list for the test, is refused, and the hook output names the gate;
    the same push to a private repository passes.
  - A push whose remote's visibility cannot be measured is gated, and
    the refusal says the visibility could not be measured.
  - A public repository that sets its own `core.hooksPath` is still
    gated, or the runbook names the case and how it is covered.
  - `git push --no-verify` and `git commit --no-verify` from an agent
    session ask before they run.
- Verify: from a sandbox, one push of a throwaway branch that holds the
  planted name to a public repository exits non-zero with the gate's
  refusal in the hook output, and the same push without it exits 0.
