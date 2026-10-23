# The pre-commit action does not warn lock-unknown-tool

What is at stake: low. A `backend:tool` the embedded catalog lacks is
warned by `julieta lock --check` but not at commit time, where 07 7.3
asks for it; the egress gate still holds the domain, so nothing widens.

Scope: this repository.

Decided on 2026-10-23, in the review of #255 (T036): a follow-up task on
the hooks surface calls the lookup of `julieta lock --check` from the
pre-commit action in `internal/hooks/actions`, which reads the lock from
the index, moving the lookup out of the command package.

Reopen: the next task that touches `internal/hooks/actions`, or the
first unknown tool a commit lets through unwarned.
