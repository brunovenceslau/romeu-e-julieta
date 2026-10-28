---
id: T175
title: "a durable home for memory"
milestone: M2
module: romeu-cli
fate: new
owns:
  - internal/memgit/**
  - internal/gitsafe/push*.go
  - internal/gitsafe/separate*.go
  - internal/cmd/romeu/all/memory_push.go
  - internal/cmd/romeu/all/memory_push_test.go
  - internal/spec/memory*.go
  - e2e/memory_git_test.go
needs:
  - T041
  - T151
after:
  - T153
  - T167
ask_first:
  - gitsafe
  - gates
  - contracts
sitting: none
---

# T175 - a durable home for memory

Goal: the durable copy of a project's memory, ADR 0043: a private git
repository per project, committed and pushed by romeu on the host as
the last step of its commands. Decided on 2026-10-28, with the
specification amended in the same pull request as T171: 03 3.2
(`memory.remote`), 08 8.1 (REQ-1303, REQ-1304, REQ-1305), 04 4.1 (the
slugs), 04 4.2 (the last step of seven commands), 05 5.2 (REQ-0502, I18)
and 05 5.4. The step is wired once, in `internal/cmd/romeu/all`, around
the commands 08 8.1 names, so no command's own files change; `gitsafe`
gains the two verbs below (`after: T153` for its files).

1. The project spec member (REQ-1305): `memory.remote`, a URL gitsafe
   accepts or the word `none`; a spec without it fails `sync` with
   `memory-remote-missing` (exit 2, an error id of `internal/spec`);
   `none` makes `sync`, `run` and `doctor` warn `memory-local` every
   time. The validation rule joins `internal/spec` (ask-first `gates`).
2. The git dir (REQ-1303): `<name>-env/memory.git`, made by `sync` with
   `git init --separate-git-dir` over `<name>-env/memory/` when absent,
   or by a clone of the remote into place when the memory dir is
   absent and the remote has history; the memory dir never holds a
   `.git`, and `memory check` keeps refusing one.
3. The commit and push (REQ-1304): `internal/memgit` stages `entries/`
   and `stamps/` of every memory dir and nothing else (an ignore rule
   the package writes into the git dir's `info/exclude`, never into the
   memory dir), commits when the index changed, signed through the
   signing socket of host settings with the project's key, the author
   the project's name, unsigned with `memory-unsigned` warned when host
   settings hold no socket; fetches and fast-forwards first, warns
   `memory-diverged` and stops when it cannot; pushes `refs/heads/main`
   with gitsafe's one push verb, whose argv names the memory remote
   only (I18); a refused push is `memory-push-failed`. Every call in
   host mode, keys passed per call, none written (I2).
4. The wiring: one post-step in `internal/cmd/romeu/all` for `sync`,
   `run`, `pull`, `salvage`, `rm`, `recreate` and `retire`, after the
   command's own work and inside its lock, whose warnings close the
   output; a command that failed before its work was done runs no
   push.

- Implements: REQ-1303 (08 8.1), REQ-1304 (08 8.1), REQ-1305 (08 8.1),
  REQ-0502 (05 5.2).
- Operator: creates one private repository per project and names it in
  the spec; approves the `gitsafe`, `gates` and `contracts` surfaces.
- Acceptance:
  - A spec without `memory.remote` fails `sync` with
    `memory-remote-missing`; with `none`, `sync`, `run` and `doctor`
    each print `memory-local`; with a URL, `sync` makes
    `<name>-env/memory.git` and no `.git` under `memory/`, and `memory
    check` on the dirs passes.
  - After `sync` adds an entry and a stamp, the commit holds exactly
    those paths (a planted `salvage/x` and `snapshot/heads.json` are
    absent), is signed with the project's key when a socket is
    configured, carries the project's name as author, and is on the
    remote's `main`; a second run with no change commits nothing and
    pushes nothing (REQ-0277).
  - With the remote moved on by one commit, the next command
    fast-forwards and then pushes; with a conflicting history,
    `memory-diverged` is warned, nothing is pushed and nothing is
    rewritten; with the remote refusing the push, `memory-push-failed`
    is warned and the command exits 0.
  - On an empty `<name>-env/memory/` and a remote with history, `sync`
    clones into place with the separate git dir, and the entries are
    listed by `julieta memory list` in the sandbox.
  - The argv and environment tests pin the push argv (the memory remote
    and `refs/heads/main`) and that no other call site of `procgroup`
    builds a push; every call runs with the host-mode neutralized
    config and no written key (I2, I3).
  - `rm` and `recreate` push after their salvage half, so the tombstone
    handoff is on the remote before the sandbox is removed.
  - The ids above say what the code does; `go run ./tools/ci reqs`
    passes.
- Verify: `go test -race ./internal/memgit/... ./internal/gitsafe/...
  ./internal/spec/... ./internal/cmd/romeu/...`, `go test -tags e2e
  ./e2e/... -run '^TestMemoryGit' -v` (10 10.8), `go run ./tools/ci
  reqs`.
