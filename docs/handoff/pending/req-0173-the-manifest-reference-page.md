# REQ-0173: the manifest reference page

What is at stake: small; the field table of 03 3.6 stays in the
specification until a generated page replaces it, so nothing is lost,
only duplicated once the page exists.

Decided on 2026-10-20, in the re-plan of T051: REQ-0173 replaces a
format's field and rule tables by a link to its generated page under
`docs/reference/`. No generator writes such a page for the manifest:
`schemas/manifest.v1.json` holds the types and not the meaning of each
field (REQ-0218 to REQ-0224). T051 no longer edits 03 3.6, and no task
cites REQ-0173 now; `go run ./tools/ci reqs` does not report an id that
no task cites.

Reopen: when a task plans the generated `docs/reference/` page of a
format (the rule table of `internal/spec/rules.go`, REQ-0172), give it
REQ-0173 and the edit of 03 3.6; or when `tools/ci reqs` starts to
report uncited ids.
