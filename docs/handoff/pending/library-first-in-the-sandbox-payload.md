# Library first in the sandbox payload

What is at stake: the rule of ADR 0038 holds in this repository only; a
builder in any other repository still writes code a library holds, and
the cost is the one ADR 0038 states.

The rule belongs in the global instructions of every sandbox, which
`kits/claude-home/spec.yaml` of the payload's source repository authors.
A prompt went to the maintainer on 2026-10-20 to paste into that
repository's session: under "How to work", before writing code that
parses, encodes, matches or does a general job, search for a maintained
library; import it, copy a small proven piece with its license and
origin, or write it by hand with the reason stated in the pull request;
and a builder brief names the rule.

Reopen: that session reports its pull request.
