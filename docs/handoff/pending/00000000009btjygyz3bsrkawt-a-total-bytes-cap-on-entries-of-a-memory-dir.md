# A total-bytes cap on entries/ of a memory dir

What is at stake: small, and easy to undo. A memory dir with no cap on
its total size lets a planted store fill the host disk the dir lives
on; a cap set too low refuses honest entries. Adding one is a row of 08
8.6 and one check in `memstore`.

Scope: this repository.

Deferred on 2026-10-20, while building T035:

- `entries/` is capped by count, 10 000 entries, and each entry file by
  size, 1 MiB (08 8.6), so it can hold about 10 GiB. The caps bound what
  one read holds: `memstore` reads one entry at a time (Walk), and
  `memory add` streams its duplicate check, so the memory risk is gone.
- What is left is disk use: an agent, or anything that writes the
  mount, can fill `entries/` to that bound. 08 8.6 has no total-bytes
  value, and `memory check` and `romeu sync` do not sum sizes.
- A cap would be one more starting value in 08 8.6 and a finding of
  `memstore.Check` (a sum of the sizes it already reads with Lstat).

Reopen: a memory dir is measured near that size, or a sitting reports a
full disk.
