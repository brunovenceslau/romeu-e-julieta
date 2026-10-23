# A registry on an address that is not public gets registry-unreachable

What is at stake: small. A workload reference that names a registry on a
loopback, private or link-local address is refused by the dial guard of
`internal/oci`, as v1 decides (fail closed). The refusal reaches the user
as `upstream-shape` (RJ-310, "a guard of the client refused the
response"), while the hint that says a private registry is not supported
in v1 is the one of `registry-unreachable` (RJ-322).

Scope: this repository.

Decided on 2026-10-23: a refused dial to the registry the reference
names gets the row `registry-unreachable`; a refusal of what the
registry answered (a redirect or a token realm to such an address) stays
`upstream-shape`. It is done in the task that wires `sync` to
`internal/oci`, where the message is read.

Reopen: T043 (`romeu sync`: render, gate, promote), when its step 6
calls `oci.FetchWorkload`.
