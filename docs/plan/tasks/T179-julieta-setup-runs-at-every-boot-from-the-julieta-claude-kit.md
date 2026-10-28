---
id: T179
title: "julieta setup runs at every boot from the julieta-claude kit"
milestone: M2
module: kits
fate: new
owns:
  - kits/julieta-claude/**
  - internal/agent/claude/**
  - internal/cmd/julieta/setup/setup.go
  - internal/cmd/julieta/setup/report.go
  - e2e/kit_julieta_claude_test.go
needs:
  - T079
  - T177
after:
  - T178
ask_first:
  - kits
sitting: none
---

# T179 - julieta setup runs at every boot from the julieta-claude kit

Goal: a recreated or restarted sandbox comes up with `julieta` on
`PATH`, the hook dispatcher, the trusted mise configuration, the locked
tools and each repo's `setup` task done, with no command by hand, and
says so loudly when it did not. Decided on 2026-10-28 with the
specification amended first: REQ-1308 (06 6.4), the SessionStart list
of 08 8.2 and probe A14 (11). It replaces step 7 of the sandbox
definitions repository's julieta runbook. `needs: T177`, because only a
`setup` that moves no branch may run at every boot.

1. The kit (REQ-1308): `lifecycle.startup`, the agent user, in the
   foreground, declaring only `HOME` and `JULIETA_MANIFEST`, runs a
   wrapper of `<julietaBin> setup --json` that always exits 0; on a
   non-zero exit it prints one line that the sandbox did not come up
   clean, naming the log. The wrapper keeps the kit step rules (bash -e
   -u, at most five lines, no arg pasted into script text) and the
   digest of the hook lines (REQ-0587) covers it.
2. `setup` writes `$HOME/.local/state/julieta/setup.log`, the escaped
   report of the run, at every run, through the atomic write of 03 3.8;
   a run that leaves the sandbox unusable records a hook failure of the
   stage `julieta setup` (`internal/hooks`), whose line names the log,
   and a later clean run clears it, so SessionStart and `julieta
   status` show it until then (08 8.2). The log never goes to a memory
   dir.
3. Probe A14's startup half runs on the host at the next recreate: does
   the hook see the clone, both mounts and `JULIETA_MANIFEST`, does the
   session wait, does a non-zero exit stop the boot. Its result is
   recorded in docs/probes/; a result that contradicts the spec reopens
   REQ-1308 before this task merges.

- Implements: REQ-1308 (06 6.4).
- Operator: yes, the approval line for `kits/**` (ask-first `kits`),
  the sandbox definitions repository's pin of the kit at the merge
  commit, a recreate, and the probe A14 reading.
- Acceptance:
  - In the container e2e of the kit, a boot runs `setup` once before the
    session's first command; a `setup` forced to fail (a dotfile in the
    memory dir) still boots, prints the not-clean line, writes the log,
    and the next SessionStart prints the hook failure naming the log; a
    clean `setup` after it clears the line.
  - A second boot of the same sandbox changes no checkout commit and no
    tool version.
  - The hook declares only `HOME` and `JULIETA_MANIFEST`;
    `go run ./tools/ci kits` passes.
- Verify: `go test -race ./internal/cmd/julieta/setup/... ./internal/agent/claude/...`,
  `go test ./e2e -run KitJulietaClaude`, `go run ./tools/ci all`, and
  the probe A14 record.
