# T107, one source for the ask-first list: in progress

Started on 2026-10-17. The decisions it applies are in
"the-ask-first-surfaces-of-t107". A local draft commit holds the first
step only: `.github/ask-first.yaml` with the narrowed `checks`, the
`contracts` globs, the `sbxdrv` surface and `skills/**` on `kits`,
then `go run ./tools/ci generate`. The pre-push refuses it while
`TestSpecListIsTheCommittedList` compares the copy of 05 5.3, which
T107 replaces; the step is redone from the decision file if the draft
is gone.

Left to do, each from T107's file:

- 05 5.3: link `.github/ask-first.yaml` in place of the copy; REQ-0535
  without the reference page; REQ-0542 names
  `tools/ci/internal/mise/misefiles.go`; REQ-0544 says `sbxdrv` and
  `skills/**` joined; a sentence on the narrowed `checks`.
- Drop `docs/reference/ask-first.md`: `askfirst.ReferencePage` and
  `PagePath`, its output in `generated`, its `.gitattributes` line and
  the tests; the CODEOWNERS header names the yaml.
- The 05 5.3 test asserts the link, not the copy.
- `docs/grants.yaml`, `version: 1` and `grants: []`.
- `.github/pull_request_template.md` points at `.github/ask-first.yaml`.
- 12 12.4 REQ-1162 lists `sbxdrv`.
- The 5.4 rows T107 names exist since ac488e2; nothing to add there.
- Run `go run ./tools/ci plan`: tasks that own `skills/**` or the
  `sbxdrv` paths must forecast `kits` and `sbxdrv`.
- Rebase on #72 first: it changes `GlobMatch` in `askfirst.go`.

Reopen: the next session.
