---
id: T076
title: "The `kit` kind and `tools/ci kits`"
milestone: M2
module: kits
fate: merged
owns:
  - tools/new/kit*.go
  - tools/ci/internal/kits/**
  - tools/ci/check_kits.go
needs:
  - T012
  - T105
  - T106
  - T113
after: []
ask_first:
  - checks
sitting: none
merged: 2026-10-19
pr: 182
---

# T076 - The `kit` kind and `tools/ci kits`

Merged on 2026-10-19 in pull request #182. The text below is the one
written at the re-plan.

Goal: The `kit` kind of `tools/new` and `tools/ci kits`: the scaffold,
the five-line and checksum rules, the pin equality rule, the descriptor
name from sitting M2.

Re-plan (2026-10-12): Kept. The `kit` kind registers by file (rule 4 of
`docs/plan.md`, T106). "The file name A11 recorded" in the first row
below is the descriptor file name read by hand at sitting M2 (T113),
kept as this task's fixture with `source: sitting M2`; A11 re-measures
it in M6.

- Implements: REQ-0563 (06 6.1), REQ-0564 (06 6.1), REQ-0589 (06 6.4),
  REQ-0606 (06 6.4), REQ-0616 (07 7.3), REQ-0624 (07 7.4), REQ-0843 (10
  10.2), REQ-0844 (10 10.2), REQ-1043 (12 12.1), REQ-1068 (12 12.1),
  REQ-1085 (12 12.2), REQ-1119 (12 12.3), REQ-1123 (12 12.3).
- Operator: no.
- Acceptance:
  - `go run ./tools/new kit <name>` writes `kits/<name>/` with a
    descriptor on the pinned frontend, under the file name A11 recorded.
  - `tools/ci kits` fails a second frontend pin, an install step over
    five lines, a download without a checksum line after it, a download
    line holding `|`, `;`, `&&` or `$(`, and a `-c` that reads a file
    the step downloaded rather than a kit file.
  - It also fails a kit whose frontend, mise or herdr pin differs from
    the pin file, and a pin file whose mise differs from the `version`
    input of the mise step of `ci.yml`, with a fixture (12 12.1: the two
    mise pins are one version).
  - On a change to the pin file, `tools/ci kits` verifies, offline and
    with `kits/mise-minisign.pub`, the signature of the committed
    `kits/mise/SHASUMS256.txt` and checks each pinned mise hash against
    it: fixtures for a good pin change, a checksum file with one changed
    byte, a hash absent from the file, and a pin change without the file
    (07 7.4). A mise pin bump's pull request names the re-read
    version-only backend list in its Evidence.
- Verify: `go test ./tools/ci/... -run '^TestKits'`, which runs the
  fixture kits; `go run ./tools/ci kits`.
