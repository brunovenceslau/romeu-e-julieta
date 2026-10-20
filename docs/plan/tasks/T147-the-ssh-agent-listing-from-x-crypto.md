---
id: T147
title: "The ssh-agent listing from x/crypto"
milestone: M2
module: signing
fate: new
owns:
  - internal/signing/signing.go
  - internal/signing/signing_test.go
  - docs/spec/06-kits.md
  - docs/spec/12-engineering.md
needs:
  - T025
after: []
ask_first:
  - signing
sitting: none
---

# T147 - The ssh-agent listing from x/crypto

Goal: the second swap of
[ADR 0038, search for a maintained library before writing one](../../adr/0038-search-for-a-maintained-library-before-writing-one.md),
decision 5, approved on 2026-10-20: `internal/signing` reads the
identity listing through `golang.org/x/crypto/ssh/agent`, a package of
a module `go.mod` already requires (v0.58.0, BSD-3-Clause), so no module
is added. `signing` keeps what the library does not hold: the check of
the socket path (absolute, not a symbolic link, a socket of this user),
the dial with the caller's context and its deadline, the rule of I34
(exactly one key, the signing key) and the errors of the package.

Read under ADR 0038, decision 3, before the swap: the exact behavior of
the hand-written parser is not the requirement. REQ-0601 asks for the
identity listing with no subprocess, and I34 for its outcome. Two parts
of the old behavior are kept by a thin wrapper, because the library
does not hold them: at most 256 KiB is read from the agent (the
library's own bound is 16 MiB), and the answer must be exactly the keys
it lists. The library parses only the keys its count names, so an
answer that counts one key and sends two would otherwise pass I34 with
the second unseen; the wrapper counts the bytes read and refuses any
left over. The cap of 1024 keys goes; 256 KiB bounds the count.

- Implements: none: a swap under ADR 0038; the requirements stay T025's.
- Operator: no.
- Acceptance:
  - The API of `signing` (`Identities`, `Check`, `ParseSigningKey`,
    `Identity` and its `Type`, the errors) does not change, and no
    caller changes.
  - The tests of T025 pass, bar the bound on the key count, and new
    cases refuse an answer that counts fewer keys than it sends and one
    over 256 KiB.
  - The client is given the connection without its `Close`, so the
    library serializes the call and starts no goroutine of its own
    (`agent.NewClient`), and a context that ends stops the listing, as
    before.
  - REQ-0601 (06 6.4) and REQ-1049 and REQ-1246 (12 12.1) name the
    package.
- Verify: `go test ./internal/signing/...`.
