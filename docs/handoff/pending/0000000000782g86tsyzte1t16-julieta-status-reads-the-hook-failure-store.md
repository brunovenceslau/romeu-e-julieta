# julieta status reads the hook failure store

What is at stake: small. Until `julieta status` lists them, a failed
SessionStart or SessionEnd hook, and a failed git hook, shows only in
the next SessionStart print; an operator who runs `julieta status` to
see what is wrong sees nothing of it. It is undone by adding one
section.

Scope: this repository.

Deferred on 2026-10-20, while building T059:

- REQ-0675 and 08 8.2 say SessionStart, `julieta status` and `romeu
  status` surface recorded hook failures until a later run of the same
  hook succeeds. T054's store, `hook-failures/` below julieta state
  (`internal/hooks`), holds the git hooks' and, since T059, the session
  hooks' failures, one file for each hook and repo.
- The `julieta status` command has no section that reads it. T059 first
  copied the failures into `setup.json` as warnings so status listed
  them, and removed the copy because two writers of that file lose each
  other's changes.
- T059's acceptance line "`julieta status` lists it" is therefore met
  only by the SessionStart print.

Reopen: the status task that adds a failures section reading
`hooks.Failures`, or an operator who misses a failed hook in `julieta
status`.
