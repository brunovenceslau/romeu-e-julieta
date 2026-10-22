# The product has no subagent context reuse rule

What is at stake: low now, medium at the product's first release. An
adopter's orchestrator gets no guidance on when to resume a subagent
and when to spawn a fresh one, and the same overfull-context failure
the payload's rule prevents reaches every adopter.

Scope: every adopter of the product. The rule would live in the
product's skills (`skills/`) or the julieta kit, specified in
`docs/spec/`.

Measured on 2026-10-22: `docs/spec`, `docs/adr`, `skills/` and
`AGENTS.md` say nothing on reusing a subagent by its context usage.
The maintainer's payload has the measurement (`ctx-usage`, a 30%
threshold, ADR-0022 of the sandbox kit's repository, Decision 10).

To do: decide whether the product carries an equivalent rule, its
threshold and its measurement, and where it is written.

Reopen: planning of T079, or "The product offers subagents their
skills at the right time" (01m5gywvmtxk1kppkg3k1e1ae0), whichever
comes first.
