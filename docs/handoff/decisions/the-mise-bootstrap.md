# The mise bootstrap

Decided on 2026-10-16, when the mise pin of the repository (2026.10.3)
and the mise of a sandbox image (2026.9.15) disagreed:

- The repository owns its pin. `go run ./tools/ci setup` installs it
  whatever the environment ships (T141; REQ-1236 to REQ-1239).
- The version and the sha256 come from the matrix of ci.yml for now.
  Reading the signed kits/mise/SHASUMS256.txt is deferred until T012
  merges (docs/spec.md, deferred decisions).
- The install happens inside `setup`, not in a separate command or
  hook.
- The installed mise is cached per version under the user cache
  directory and run by its absolute path.
- The download is the bare binary of each runner, so no archive is read
  (measured; #61).
