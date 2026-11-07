---
id: T201
title: "The julieta-claude kit by fixed path: the link, the bundled skills and the mount-point check"
milestone: M2
module: kits
fate: new
owns:
  - kits/julieta-claude/**
  - internal/agent/claude/**
  - internal/cmd/julieta/setup/hooks_path*.go
  - internal/cmd/julieta/setup/link*.go
  - e2e/kit_julieta_claude_test.go
needs:
  - T079
  - T179
  - T199
after: []
ask_first:
  - kits
sitting: none
---

# T201 - The julieta-claude kit by fixed path: the link, the bundled skills and the mount-point check

Goal: the `julieta-claude` kit names julieta by one constant path,
`/opt/julieta/bin/julieta`, takes no `julietaBin` arg, installs a
root-owned link at `/usr/local/bin/julieta` in place of the `sh`
launcher, ships its two skills as bundled `agent-skill@1` entries, and
its hook wrappers refuse to run a file that is not on the mount.

Added on 2026-11-06 (scope this repository), with the specification
amended first: the sbx 0.51 and kit schema m.12 amendments of 02 2.5,
06 6.3, 06 6.4 and 08 8.2. The mount's target of T200 is what makes the
path a constant; until T200 and T066 land, the bridge of T111 sets the
same target in its hand-written env file, so one kit serves both.

1. The path (REQ-0596, REQ-1308): the two session hooks, the PreToolUse
   guard and the `lifecycle.startup` hook invoke
   `/opt/julieta/bin/julieta`; the kit declares no `julietaBin` arg, so
   its content is the same for every project.
2. The link (REQ-1353, REQ-1373): a kit step writes
   `/usr/local/bin/julieta` as a root-owned symbolic link to
   `/opt/julieta/bin/julieta`; the `sh` launcher of T199 goes. The mount
   and this link are julieta's two entry points (06 6.3 item 5):
   `julieta setup` makes no `$HOME/.local/bin/julieta` link (REQ-0586
   retired, 12 12.11), and the dispatcher scripts of T054 exec
   `/opt/julieta/bin/julieta hooks run <hook> "$@"` (REQ-0397).
3. The mount-point check (REQ-1372): before it runs julieta, each
   wrapper of REQ-0673 reads `/proc/self/mountinfo` and treats
   `/opt/julieta/bin` not being a mount point as the absent-mount case,
   printing the alert block with `cause: /opt/julieta/bin is not a mount
   point`; the guard still exits 2.
4. The bundled skills (REQ-0595): the kit's recipe
   (`julieta-claude.dockerfile`, `FROM scratch`, one `COPY` per skill)
   lays `skills/julieta` and `skills/handoff` under
   `/usr/share/julieta-claude/skills/<skill>`, and the descriptor
   carries one `com.docker.sandbox/agent-skill@1` per skill naming that
   directory; the base64 payload step is kept until the first recreate
   on sbx 0.51 shows the files at the discovery directory, then
   removed in a pull request of its own. That the files reach the
   discovery directory with `skills: off` is B2's measurement (T082,
   T092); until block B, the first recreate on sbx 0.51 is the route's
   go/no-go, recorded as a fact of this task (REQ-0595).
5. `skills-stale` (REQ-0587): the digest julieta embeds covers the same
   skill files and hook lines; `julieta setup` reads the installed
   skills at the discovery directory sbx exposes, and the comparison is
   unchanged; a bundled skill that sbx did not expose is `skills-stale`,
   the line SessionStart prints.

- Implements: REQ-1373 (06 6.3), REQ-0587 (06 6.3), REQ-0397 (04 4.3),
  REQ-0595 (06 6.4), REQ-0596 (06 6.4),
  REQ-1308 (06 6.4), REQ-1353 (06 6.4), REQ-1372 (08 8.2), REQ-0673 (08
  8.2).
- Operator: yes, the approval line for `kits/**` and `skills/**`
  (ask-first `kits`), then the sandbox definitions repository's pin of
  the kit, the `target` in its hand-written env file, and a recreate.
- Acceptance:
  - The kit e2e asserts each hook, the guard and the startup hook name
    `/opt/julieta/bin/julieta` and that the descriptor declares no
    `args`; a kit built from the tree carries no `julietaBin` string.
  - The kit e2e installs `/usr/local/bin/julieta` as a root-owned link
    to `/opt/julieta/bin/julieta`, the agent cannot rewrite it, and
    `julieta version` runs by name through it.
  - Each session wrapper, run with `/opt/julieta/bin` present as a plain
    directory holding an executable `julieta`, prints the alert block
    with the mount-point cause and exits 0 without running the file;
    the guard case prints it on stderr and exits 2; with the path a
    mount point (a bind mount in the container e2e) the wrapper runs
    julieta.
  - `julieta setup` creates no `$HOME/.local/bin/julieta`, and the
    dispatcher scripts it writes exec `/opt/julieta/bin/julieta`, the
    fixed path (04 4.3, REQ-0397); the T051 and T054 cases that
    asserted the link are replaced.
  - With the skills exposed at the discovery directory, `setup` reports
    no `skills-stale`; with one skill's directory missing there, it
    does.
  - The descriptor's `agent-skill@1` entries parse in `internal/oci`'s
    grammar, and `sbx kit inspect --offline` on the built kit lists the
    two skills (the fact recorded by hand until A11).
- Verify: `go test -tags e2e ./e2e/... -run '^TestKitJulietaClaude' -v`.
  The pattern names the `TestKitJulietaClaude*` functions this task
  adds or changes, and the reviewer reads them in the `-v` output (10
  10.8).
