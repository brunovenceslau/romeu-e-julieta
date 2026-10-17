# T107, one source for the ask-first list: in progress

Started on 2026-10-17. The decisions it applies are in
"the-ask-first-surfaces-of-t107". One commit on `main` after #72 holds
the whole list of the task file:

- 05 5.3 links `.github/ask-first.yaml` in place of the copy; REQ-0535,
  REQ-0542 and REQ-0544 are rewritten, with a sentence on the narrowed
  `checks`.
- `docs/reference/ask-first.md` leaves the generated set, with
  `askfirst.ReferencePage`, `PagePath`, its `.gitattributes` line and
  its tests; the CODEOWNERS header names the yaml.
- The 05 5.3 test asserts the link and the `approvals` paths.
- `docs/grants.yaml` is `version: 1` with `grants: []`; the PR template
  points at the yaml; 12 12.4 REQ-1162 lists `sbxdrv`.
- T023 forecasts `contracts`, since `internal/*/errors.go` matches its
  `owns`. `go run ./tools/ci plan`, `generated` and `fast` exit 0.

Left to do: the review gate, the forbidden-names gate, the push and the
pull request with the `ask-first`, `spec`, `checks` and `approvals`
lines.

Reopen: the next session.
