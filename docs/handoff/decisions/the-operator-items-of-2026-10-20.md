# The operator items of 2026-10-20

What is at stake: small; each answer orders work or sets where a trace
goes, and a wrong one is changed by a later note.

Scope: this repository.

Decided on 2026-10-20, answering the "Open for the operator" list of
the checkpoint after the swaps of ADR 0038:

- ADR 0039, with its supersedes lines for ADR 0001, and ADR 0021 are
  Accepted.
- The T112 prompts (the config repository's first, then the sandbox
  kit repository's) and the ADR 0039 prompt for the payload are pasted
  into their sessions on 2026-10-20; each session reports its pull
  request ("t112-in-the-config-and-kit-repositories",
  "rules-to-move-when-the-interim-payload-is-retired").
- The GitHub App of
  [ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md)
  is created when M3 starts; T126 waits for it.
- `gitsafe`'s fast-forward is hardened by a task on the `gitsafe`
  surface before `julieta setup` runs in daily use
  ("gitsafe-s-fast-forward-overwrites-ignored-files-and-runs").
- The removal of a corrupted cache entry in the container e2e writes a
  job annotation, a channel the infrastructure classifier of
  `tools/ci/internal/container` does not read
  ("the-spec-catches-up-with-the-hardening-of-t127-t143-and-t144").
