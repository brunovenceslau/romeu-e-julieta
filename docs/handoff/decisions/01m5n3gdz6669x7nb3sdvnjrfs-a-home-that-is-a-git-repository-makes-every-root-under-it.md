# A HOME that is a git repository makes every root under it refused

What is at stake: low. An operator whose `$HOME` is a git repository
cannot choose a root under it, and has to choose one outside; no data is
at risk.

Decided on 2026-10-23. Scope: this repository, the root check of
`romeu init` and `romeu doctor` (04 4.2, I20).

A `$HOME` that is itself a git repository makes every root under it
refused by the rule "a root inside a git repository", so the rule stays
as it is (I20 kept) and no exception is made for `$HOME`.

Reopen: the first operator with a versioned `$HOME`.
