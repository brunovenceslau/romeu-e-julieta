---
id: T113
title: "The kit-frontend go/no-go"
milestone: M2
module: none
fate: merged
owns: []
needs:
  - T012
after: []
ask_first: []
sitting: M0
merged: 2026-10-19
pr: 179
---

# T113 - The kit-frontend go/no-go

Merged on 2026-10-19 in pull request #179: sitting M0 ran on that day
and the verdict is go ("sitting-m0-the-kit-frontend-go"). The text
below is the one written at the re-plan.

Goal: A minimal local kit built from a directory source, on one host,
about 40 minutes: the descriptor file name, the capability names, where
install steps run and what they reach, whether kit args reach the
install and config steps (the `julietaBin` and `signingKey` args that
sitting M2's definitions set by hand, rule 9), the workload descriptor
as bytes by digest, the read-only mount, the one-key agent forwarding,
the herdr protocol number, and the host git version and series.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox. The pull
request whose Evidence carries the sitting's output names this task in
its body and writes `merged:` and `pr:` into this file, the one write
rule 1 allows outside its own file.

Re-plan (2026-10-12): Runs at sitting M0, once T012 merged: it needs the
pin file only, and a record (T123) is a review order, not a dependency;
the earlier plan ran it as part 1 of sitting M2.

- Implements: REQ-0566 (06 6.1), REQ-0572 (06 6.1).
- Operator: runs it at sitting M0.
- Acceptance:
  - Each fact lands as a fixture in the task that reads it (T032's
    conformance fixture, T076's descriptor and kit-args fixtures, T081's
    socket fact) with a `source: sitting M2` field, and in that pull
    request's Evidence; the formal probes re-measure them in M6.
  - The host git version and series are read at the sitting; when the
    value differs from 2.45.4, a one-line pull request on the `gitsafe`
    surface updates `floor.go` with `source: sitting M2`, named in the
    runbook, until T027 re-measures it.
  - On a no-go: a decision record with the options of 06 6.1 (wait for
    the next frontend milestone; v2 kits; a published kit of our own,
    Q7) and the trigger of each; it names the tasks that wait on this go
    (T032, T076, T081 directly; T077, T079, T038, T043, T066, T115, T116
    through them) and which of them starts under each option; M3's lane
    A continues.
- Verify: the facts list of the goal, each fact with its value and
  `source: sitting M2`, in the runbook and pasted as Evidence of the
  pull request of the first task that reads it; a fact with no value is
  a no-go. The fixtures themselves are verified by the tasks that own
  them (T032, T076, T081).
