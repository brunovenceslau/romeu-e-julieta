# A hygiene rule for non-ASCII bytes in Go sources

Tool parameters decode \uXXXX escapes into literal characters, so an
agent can write a bidi or invisible character into a Go source without
seeing it (a Trojan Source risk). Builders were told to keep every Go
source free of bytes >= 0x80 and write escapes instead; nothing checks
it. Proposed: a `tools/ci hygiene` rule that fails on any byte >= 0x80
in a Go source.

Also seen: `tools/mutants -worktree` left dangling objects in the shared
gitdir; check whether it still does.

Reopen: the next change to tools/ci hygiene, or T106.
