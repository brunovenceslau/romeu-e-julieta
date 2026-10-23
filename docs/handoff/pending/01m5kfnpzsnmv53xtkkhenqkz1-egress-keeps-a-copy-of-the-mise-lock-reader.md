# Egress keeps a copy of the mise.lock reader

What is at stake: low. `internal/egress` (romeu) may not import
`internal/mise` (julieta, 10 10.7), so it holds a copy of the lock reader
and its shape check; `TestShapeIsTheMiseOne` and `TestFixtureIsTheMiseOne`
hold the shape check and the fixture to the originals, so a drift fails CI
rather than diverging silently.

Scope: this repository.

Decided on 2026-10-23, in the review of #255 (T036): keep the copy. The
option is a shared package that both binaries link, which needs an edit
of 10 10.7 and a move of the T053 reader.

Reopen: the next change of the mise pin, or a drift the two tests catch.
