# Go version and pin policy decided on 2026-10-08

Decided on 2026-10-08, when the Go bump of #26 was open and the
`vulnerabilities` leg of main's CI was red:

- The latest Go always makes sense. Any 1.27 patch is fine; a 1.28
  needs a decision of its own.
- The Go pin is exact. The `vulnerabilities` gate is the trigger for the
  next patch.
- Bump automation is deferred (the spec holds it in the Deferred
  decisions row of the pin-freshness check, docs/spec.md). The policy
  reopens if manual patch bumps become a burden or the gate warns too
  late.

The exact-pin rule itself is REQ-1061 (12 12.1).
