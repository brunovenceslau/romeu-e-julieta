---
id: T052
title: "`kitpin mise`"
milestone: none
module: kits
fate: folded
absorbed_by: T012
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T052 - `kitpin mise`

Re-plan (2026-10-12): folded into T012, which carries its acceptance
rows. This file keeps the id so that earlier citations resolve; the text
below is the one written before the re-plan, and T012 is where it is
built.

- Implements: 07 7.4.
- Operator: an agent looks up the signature format first; the maintainer
  confirms mise's public key out of band, and decides Q31 only if the
  lookup adds a dependency.
- Ask-first, as forecast before the re-plan: `kits`, `dependencies` (if
  the lookup adds one).
- Acceptance:
  - `kitpin mise <version>` writes the version and both linux sha256
    values from the release's `SHASUMS256.txt` into the pin file, after
    it verifies that file's minisign signature (07 7.4).
  - The trust root is explicit: mise's minisign public key is committed
    next to the pin file, and the maintainer confirms it out of band,
    which the pull request records. There is no way to skip the
    verification, no flag and no environment variable, and a missing key
    is an error.
  - A test with a recorded fixture covers a good signature, a bad one, a
    checksum file signed with another key, and a checksum file that
    lacks an architecture.
  - How the signature is verified follows the lookup of Q31.
- Verify: `go test ./tools/kitpin/...`.
