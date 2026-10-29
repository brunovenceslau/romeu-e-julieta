---
id: T184
title: "egress.Split splits the network hosts a kit declares"
milestone: M3
module: egress
fate: new
owns:
  - internal/egress/split*.go
  - docs/spec/07-mise-egress.md
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
The part of T043 that links `sync` into the binary
(`import_sync.go`) calls Split before the derivation.

1. Split takes the network hosts of a kit's capability set as the
   descriptor writes them and returns the domains the derivation reads:
   a numeric port stripped, the host lower-cased; a wildcard, an IP
   literal or a name `spec.ValidDomain` refuses is an error naming the
   kit and the host. That is all Split does, since no spec text defines
   it beyond REQ-0069: the catalog's upload flags and the gating stay in
   the derivation (`internal/egress/derive.go`).
2. Decide in 07 7.5 whether the hosts of a kit's install phase and of
   its runtime phase stay apart: T043 merges both into one list, so a
   host allowed at install only (the julieta kit chose the install phase
   so the runtime would not open `github.com`) is open at runtime too.

- Implements: REQ-0069 (01 1.4).
- Operator: no.
- Acceptance:
  - A table test of Split: a port stripped, upper case lowered, a
    wildcard, an IP literal and a bad name refused.
  - 07 7.5 says whether a kit's install and runtime hosts stay apart.
- Verify: `go test ./internal/egress/ -run Split -v`.
