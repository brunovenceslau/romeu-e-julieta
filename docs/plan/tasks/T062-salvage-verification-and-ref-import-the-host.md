---
id: T062
title: "Salvage verification and ref import, the host half"
milestone: M4
module: salvage
fate: merged
owns:
  - internal/salvage/verify*.go
  - internal/salvage/import*.go
  - internal/salvage/errors.go
needs:
  - T061
  - T033
  - T035
  - T024
after: []
ask_first:
  - contracts
sitting: none
merged: 2026-10-29
pr: 337
---

# T062 - Salvage verification and ref import, the host half

Goal: Salvage verification and ref import, the host half, dark.

Re-plan (2026-10-12): Kept as written before the re-plan; its files are
the two globs above, in the package T061 owns by glob.

Re-plan (2026-10-29): `owns` gains `internal/salvage/errors.go`, and
`ask_first` gains `contracts`: the rows `salvage-path` (`RJ-307`) and
`salvage-id-mismatch` (`RJ-339`) this task raises are declared in the
package's `errors.go`, the one file name the error table reads.

- Implements: REQ-0050 (01 1.2), REQ-0157 (02 2.6), REQ-0234 (03 3.8),
  REQ-0247 (03 3.11), REQ-0248 (03 3.11), REQ-0249 (03 3.11), REQ-0250
  (03 3.11), REQ-0251 (03 3.11), REQ-0252 (03 3.11), REQ-0253 (03 3.11),
  REQ-0256 (03 3.12), REQ-0271 (04 4.1), REQ-0300 (04 4.1), REQ-0462 (05
  5.1), REQ-0487 (05 5.2), REQ-0501 (05 5.2), REQ-0521 (05 5.2),
  REQ-0522 (05 5.2), REQ-0523 (05 5.2), REQ-0524 (05 5.2), REQ-0525 (05
  5.2), REQ-0526 (05 5.2), REQ-0527 (05 5.2), REQ-0528 (05 5.2),
  REQ-0529 (05 5.2), REQ-0530 (05 5.2), REQ-0531 (05 5.2), REQ-0532 (05
  5.2), REQ-0533 (05 5.2), REQ-0534 (05 5.2), REQ-0708 (08 8.5),
  REQ-0714 (08 8.5), REQ-0715 (08 8.5), REQ-0717 (08 8.5).
- Operator: no.
- Acceptance:
  - The manifest is read through `os.Root`; completeness is recomputed
    and the manifest's own `complete` ignored (I17); `agentStop` is read
    as 03 3.11 defines it (zero matched with no open session recorded as
    0).
  - A manifest whose `salvageId` differs from the id romeu passed fails
    with `salvage-id-mismatch` (`RJ-339`); a table of hostile paths
    (absolute, `..`, a symlink component, outside the dir) each fails
    with `salvage-path` (`RJ-307`); a manifest whose total bytes exceed
    `salvage.capBytes` makes the salvage incomplete with `over-cap`.
  - A bundle ref name the validator refuses fails the salvage; a bundle
    with missing prerequisites verifies after the origin fetch into
    `refs/romeu/origin/`; a malformed object in an unbundled head is the
    skipped item `fsck-failed`; a payload whose size or sha256 differs
    is `payload-mismatch`.
  - Refs are created only under `refs/romeu/salvage/...` with
    `CreateRef` (I6).
  - The package is **dark** until T068.
- Verify: `go test ./internal/salvage/...`.
