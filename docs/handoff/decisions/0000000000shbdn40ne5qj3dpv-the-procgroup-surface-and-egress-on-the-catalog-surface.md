# The procgroup surface and egress on the catalog surface

Decided on 2026-10-13: a new ask-first surface `procgroup` covers
`internal/procgroup/**`, and `internal/egress/**` joins the `catalog`
surface in the same edit (this resolves the earlier reminder-store item
that proposed it).

Decided on 2026-10-13: the os/exec call-site scan stays a test in
`internal/procgroup` until T039 moves it to `tools/ci callsites`; when
it moves, the admitted list's new file joins the `procgroup` globs.

Decided on 2026-10-13: REQ-0305 and the second-signal row of the
Deferred decisions are closed in the surface-and-spec docs PR, which sits
at the bottom of the stack with T137 on top, merged atomically.
