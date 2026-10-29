---
id: T184
title: "egress.Split splits the network hosts a kit declares"
milestone: M3
module: egress
fate: new
owns:
  - internal/egress/split*.go
needs: []
after: []
ask_first:
  - catalog
sitting: none
---

# T184 - egress.Split splits the network hosts a kit declares

Goal: `egress.Split`, which REQ-0069 names and no code has. The product
kits declare hosts with a port (`github.com:443` in
`kits/julieta/julieta.yaml`, `deb.debian.org:443` in
`kits/os-base/os-base.yaml`), and the derivation refuses a port, so
`romeu sync` step 6 fails on every project that uses them (found in the
review of T043, #336; decided on 2026-10-29, scope this repository).
T043's last slice calls Split before the derivation.

1. Split takes the network hosts of a kit's capability set as the
   descriptor writes them and returns the domains the derivation reads:
   a numeric port stripped, the host lower-cased; a wildcard, an IP
   literal or a name `spec.ValidDomain` refuses is an error naming the
   kit and the host.
2. Decide in 07 7.5 whether the hosts of a kit's install phase and of
   its runtime phase stay apart: T043 merges both into one list, so a
   host allowed at install only (the julieta kit chose the install phase
   so the runtime would not open `github.com`) is open at runtime too.

- Implements: REQ-0069 (01 1.4).
- Operator: no.
- Acceptance:
  - A table test of Split: a port stripped, upper case lowered, a
    wildcard, an IP literal and a bad name refused.
  - The test `TestRenderCandidatesRendersEveryProductKit`, kept in the
    T043 memory entry on egress.Split, joins
    `internal/cmd/romeu/sync/step6_render_test.go` with T043's last
    slice and passes: every product kit renders through step 6.
- Verify: `go test ./internal/egress/ -run Split -v`.
