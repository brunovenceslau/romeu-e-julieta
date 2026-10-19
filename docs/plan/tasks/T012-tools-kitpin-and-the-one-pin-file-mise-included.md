---
id: T012
title: "`tools/kitpin` and the one pin file, mise included"
milestone: M1
module: kitpin
fate: merged
owns:
  - tools/ci/internal/lint/boundaries.go
  - tools/kitpin/**
  - kits/pins.yaml
  - kits/mise-minisign.pub
  - kits/mise-minisign.pub.license
  - kits/mise/SHASUMS256.txt
  - kits/mise/SHASUMS256.txt.license
  - kits/mise/SHASUMS256.txt.minisig
  - kits/mise/SHASUMS256.txt.minisig.license
  - LICENSES/MIT.txt
  - go.mod
  - go.sum
needs: []
after:
  - T125
  - T139
ask_first:
  - kits
  - dependencies
  - checks
sitting: none
merged: 2026-10-18
pr: 164
---

# T012 - `tools/kitpin` and the one pin file, mise included

Merged on 2026-10-18 in pull request #164. The text below is the one
written at the re-plan.

Earlier title: `tools/kitpin` and the one pin file.

Goal: `tools/kitpin` and the one pin file: the frontend, the workload,
herdr and mise.

Re-plan (2026-10-18): Changed. mise signs `SHASUMS256.txt` in the
prehashed minisign mode (the signature line starts with `ED`), the
answer to the lookup of Q31, so the check needs BLAKE2b-512:
`golang.org/x/crypto`, whose `blake2b` package `tools/kitpin` alone
imports, joins the list of REQ-1049. T012 owns `go.mod` and `go.sum`,
comes after T125, which owns them too, and after T139, whose
forbidigo allow list (`tools/ci/internal/lint/boundaries.go`) gains
`tools/kitpin`, the main package of the tool, and forecasts the
`dependencies` and `checks` surfaces. The key and the two mise files
are mise's own bytes and cannot carry an SPDX header, so each has a
REUSE `.license` file beside it naming mise's MIT license, whose text
joins `LICENSES/`.

Re-plan (2026-10-12): Kept; T052 (`kitpin mise`) is folded in: the
signature read of Q31 and the committed public key land here, in one
tool and one pull request. The acceptance rows of the earlier T052 text
are copied below.

- Implements: REQ-0117 (02 2.1), REQ-0189 (03 3.1), REQ-0221 (03 3.6),
  REQ-0262 (03 3.13), REQ-0564 (06 6.1), REQ-0565 (06 6.1), REQ-0578 (06
  6.2), REQ-0605 (06 6.4), REQ-0622 (07 7.4), REQ-0623 (07 7.4),
  REQ-0624 (07 7.4), REQ-0844 (10 10.2), REQ-1061
  (12 12.1), REQ-1064 (12 12.1), REQ-1069 (12 12.1).
- Operator: confirms mise's public key out of band; one `sbx policy
  allow network` for the building sandbox with every host named, exact
  hosts and no wildcard.
- Acceptance:
  - Lands before its module's place in the order of 12 12.2 (Q26): the
    container e2e (T013) and the kits read the pin file, and the tool
    needs nothing from the modules between. The rows are in "Toolchain
    pins" in `docs/plan.md`.
  - One committed data file under `kits/` holds the pins that more than
    one consumer reads: the kit frontend with its digest, the workload
    by digest, and herdr with a sha256 per architecture. The harness,
    the container e2e, `layout` and the kits read that file, so a probe
    and a kit cannot disagree on a pin.
  - `kitpin frontend` rewrites the frontend pin; `kitpin herdr
    <version>` takes each hash from the release API's digest and refuses
    a release that is not marked immutable (06 6.4). Each has a test
    against a recorded fixture.
  - `kitpin herdr` refuses an asset whose digest field is missing or is
    not `sha256:<64 hex>`, naming the field; a recorded fixture without
    the field tests it (06 6.4).
  - `kitpin workload <digest>` rewrites the workload entry of the pin
    file from a digest-shaped argument and reads no registry; a
    reference that is not `<registry>/<repository>@sha256:<64 hex>` (03
    3.1) is refused; tested against a fixture; the arch check stays with
    `julieta pin workload` (06 6.2, T055).
  - The workload entry carries `home`, the workload user's home, written
    by hand with `source: sitting M2`; A4 re-measures it (T026).
  - `kitpin mise` commits the signed `SHASUMS256.txt` and its signature
    next to the pin, as `kits/mise/SHASUMS256.txt` and
    `kits/mise/SHASUMS256.txt.minisig` (07 7.4).
  - The file says, in a comment the check of T020 cannot make: a pin
    that changes after block O3 needs the probe that read it run again.
- Acceptance (from T052, folded):
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
