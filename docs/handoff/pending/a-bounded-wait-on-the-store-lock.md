# A bounded wait on the store lock

What is at stake: small. While the store lock waits forever, a process
of the same user that holds the flock of a memory dir blocks every
memory write of that dir, and the command hangs instead of failing; a
timeout set too short fails honest writes behind a slow one. It is
undone by changing one call.

Scope: this repository.

Deferred on 2026-10-20, while building T035:

- REQ-0655 (08 8.1) says `Put` takes the store lock,
  `$HOME/.local/state/julieta/locks/<dir>.lock`, and a second process
  waits for the first. `memstore/write` takes an exclusive flock and
  waits without a bound (`Store.locked`).
- The lock file is the user's own, below `$HOME`, so only a process of
  the sandbox's user can hold it; nothing outside the sandbox can.
- A bounded wait needs an error of its own (an id in 04 4.4, on the
  `contracts` surface) and a change of REQ-0655, which says the second
  process waits.

Reopen: a hung memory write is seen, or `handoff write` lands and holds
the lock for a whole command, git reads included (REQ-0655).
