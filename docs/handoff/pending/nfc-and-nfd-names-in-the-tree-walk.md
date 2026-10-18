# NFC and NFD names in the tree walk

`gitsafe.WalkTree` (T024, #139) refuses names that are one under case
folding and with the characters HFS+ ignores taken out, but not two
names that are one only under Unicode normalization (NFC and NFD),
which APFS treats as one file. On a macOS host one would overwrite the
other inside the candidate. Catching it needs
`golang.org/x/text/unicode/norm`, which the complete dependency list
of 12 12.1 (REQ-1049) does not hold, so it is a spec change first.

Recommended: add `golang.org/x/text` to REQ-1049 and key the collision
map on the NFD form, with a row in the I27 table.

Reopen: the macOS E case of I27 (T044), or the operator's choice.
