---
id: T012
title: "`tools/kitpin` and the one pin file, mise included"
milestone: M1
module: kitpin
fate: kept
owns:
  - tools/kitpin/**
  - kits/pins.yaml
  - kits/mise-minisign.pub
needs: []
after: []
ask_first:
  - kits
sitting: none
---

# T012 - `tools/kitpin` and the one pin file, mise included

Earlier title: `tools/kitpin` and the one pin file.

Goal: `tools/kitpin` and the one pin file: the frontend, herdr, mise,
and the workload entry written by hand.

Re-plan (2026-10-12): Kept; T052 (`kitpin mise`) is folded in: the
signature read of Q31 and the committed public key land here, in one
tool and one pull request. The acceptance rows of the earlier T052 text
apply as written there.

- Implements: 06 6.1 (Frontend), 06 6.4 (herdr), 03 3.1 (the workload
  reference), 02 2.1 (`tools/kitpin`).
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
  - The workload entry is written in this pull request by hand, since
    the specification gives `tools/kitpin` no subcommand for it;
    `julieta pin workload` moves it in a spec later (06 6.2).
  - The file says, in a comment the check of T020 cannot make: a pin
    that changes after block O3 needs the probe that read it run again.
- Verify: `go test ./tools/kitpin/...`.
