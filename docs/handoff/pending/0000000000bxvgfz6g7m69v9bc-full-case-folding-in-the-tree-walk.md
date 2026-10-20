# Full case folding in the tree walk

`gitsafe.WalkTree` folds names with simple case folding (one rune to
one rune) before it compares them, so two names that are equal only
under full case folding are not refused. The pairs are `straße` and
`STRASSE`, `ﬁ` (U+FB01) and `fi`, and `ŉ` (U+0149) and `ʼn`.
Whether APFS joins any of them is not known; the macOS runners of the
E case of I27 can measure it.

Reopen: the macOS E case of T044 shows APFS joins one of them.
