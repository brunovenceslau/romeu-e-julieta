# Library first in the sandbox payload

What is at stake: the rule of ADR 0038 holds in this repository only; a
builder in any other repository still writes code a library holds, and
the cost is the one ADR 0038 states.

The rule belongs in the global instructions of every sandbox, which
`kits/claude-home/spec.yaml` of the payload's source repository authors.
A prompt was pasted into that repository's session on 2026-10-20: under "How to work", before writing code that
parses, encodes, matches or does a general job, search for a maintained
library; import it, copy a small proven piece with its license and
origin, or write it by hand with the reason stated in the pull request;
and a builder brief names the rule.

Where the rule lives, by scope:

- this repository: ADR 0038;
- every sandbox today: the payload of `kits/claude-home`, which is
  interim;
- every sandbox once romeu and julieta replace that payload: the
  personal agent context of 06 (REQ-0608, a personal kit in the config
  repository), or a product skill under `skills/` if adopters should
  get it too. Which one is open; the rule moves with the rest of the
  payload when that migration is planned.

Reopen: that session reports its pull request, or the payload's
migration to the config repository is planned.
