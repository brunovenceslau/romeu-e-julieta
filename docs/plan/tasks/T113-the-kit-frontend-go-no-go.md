---
id: T113
title: "The kit-frontend go/no-go"
milestone: M2
module: none
fate: new
owns: []
needs:
  - T012
after: []
ask_first: []
sitting: M2
---

# T113 - The kit-frontend go/no-go

Goal: A minimal local kit built from a directory source, on one host,
about 40 minutes: the descriptor file name, the capability names, where
install steps run and what they reach, whether kit args reach the
install and config steps (the `julietaBin` and `signingKey` args that
sitting M2's definitions set by hand, rule 9), the workload descriptor
as bytes by digest, the read-only mount, the one-key agent forwarding,
the herdr protocol number.

No file of this repository: the work lands in the maintainer's config
repository, and this file is its record and its checkbox.

- Implements: 06 6.1; 06 6.2; 11 11.1 (the facts of A11, A12 and A13,
  read by hand first).
- Operator: runs it as part 1 of sitting M2.
- Acceptance:
  - Each fact lands as a fixture in the task that reads it (T032's
    conformance fixture, T076's descriptor and kit-args fixtures, T081's
    socket fact)
    with a `source: sitting M2` field, and in that pull request's
    Evidence; the formal probes re-measure them in M6.
  - On a no-go: a decision record with the options and their triggers;
    M2 stops while M3's lane A continues.
- Verify: the facts list of the goal, each fact with its value and
  `source: sitting M2`, in the runbook and pasted as Evidence of the
  pull request of the first task that reads it; a fact with no value is
  a no-go. The fixtures themselves are verified by the tasks that own
  them (T032, T076, T081).
