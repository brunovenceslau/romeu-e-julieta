# Canonical JSON from canon for the manifest

What is at stake: a divergence between the manifest's canonical bytes
and `canon.Digest` would let a changed manifest pass as approved; tests
and a fuzz test tie them today (#184).

`internal/manifest` writes its canonical JSON itself, because exporting
the function from `canon` touches the `digests` surface. One function in
`canon` used by both would remove the duplicate.

Reopen: the next change to either encoder, or a second package that
needs canonical JSON.
