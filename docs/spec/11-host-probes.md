# 11. Host probes

Back to [index](../spec.md). Reader: the maintainer running probes and
implementers of `e2e/probes`. Type: reference.

`sbx` runs only on the host, so these
measurements are maintainer steps, grouped into **two blocks**. Block A
needs no product code (only the probe harness) and settles every sbx
fact the code depends on before `sbxdrv`, `render` and the kits are
finalized. Block B only accepts built artifacts, so a failure there is
fixed inside the artifact (a kit, a catalog entry), not in the
architecture.

```
go run ./e2e/probes --block A --out docs/probes/
```

`--out` names a directory: the harness writes one file per probe there
(11.3).

The harness creates throwaway repos and env dirs under a temp root, runs
each step, pauses only where a human must answer an sbx prompt or look at
VS Code, records sbx help text and every argv/stdout/stderr/exit into
`e2e/testdata/sbx/<version>/` (the redacted fake-sbx recordings, [10
10.3](10-testing-style.md#103-fake-sbx-fidelity-contract)), writes
`probe-result.v1` JSON, and cleans up. Benign payloads only (`touch
<marker>`).

The harness has two layers: a pure core (probe definitions, predicates,
decision functions, result writing, redaction) and a thin exec layer
that only starts `sbx` and captures its output. Both are unit-tested in
CI (the exec layer against a helper binary), so the first plan task can
pass `tools/ci all` before any host run. The arch-sensitive probes (A4,
A5, A11, A12, A13) count as settled only with results from both hosts
(Intel and Apple silicon). The A5 goldens are written by hand with the
probe definitions; `render`
([12 12.2](12-engineering.md#122-development-commands-and-capability-map))
must reproduce them byte for byte, and `tools/ci probes` already flags
a moved hash of their normalized form (03 3.3).

The Settles column of the tables below names what a probe decides
besides open questions: success criteria, invariants, formats. Which
open question a probe settles is written once, in the "Settled by"
column of the index's [Open questions](../spec.md#open-questions).

## 11.1 Block A - sbx and runtime facts (first, in parallel with the first build layer)

At most about 75 minutes per host.

| ID | Probe | Expect (written in the harness before the run) | Settles |
|---|---|---|---|
| A1 | `sbx version`, `git --version`, host global git config, VS Code trust settings and `task.allowAutomaticTasks`, symlinks from `$HOME` into the planned root; git CVE floor per series checked against the git release notes; the sbx settings read `doctor` uses, for `env.rememberHostCommands` and `kit.allowedSources` | sbx >= 0.46.0; git at or above the per-series patched release; the argv and output of the settings read recorded | the git floor, doctor checks |
| A2 | `sbx settings set env.rememberHostCommands true`; env with a `secrets.<svc>.command`, the command a `security find-generic-password` call, started from a non-interactive script; `sbx env run -d` twice; change the command; run again | second run silent; changed command re-prompts; whether a keychain dialog appears recorded | never `--auto-approve` is workable; whether J1 or `doctor` must cover the keychain prompt |
| A3 | memory dirs and the read-only mounts: from inside, plant a symlink, a FIFO, a dotfile, a `.git` dir and a `.vscode/tasks.json` in a memory dir; write to the `readOnly` `.romeu/bin` mount, also as root after `mount -o remount,rw`; write to the primary clone's source mount, as the agent and as root; check each mount path equals the host path; record the modes of `<name>-env/` and of the memory files as the sandbox sees them through the direct mounts; with a scoped `github` secret, clone a private secondary, push a branch and call the GitHub API | each plant is seen on the host as planted; no write reaches the host through a `readOnly` mount, also as root after the remount; no write reaches the primary host clone; each mount path equals the host path; the modes are recorded; the clone, the push and the API call succeed with no credential file in the sandbox | the read-only host clone (02 2.3), I30, the mount half of I24, the modes of 02 2.3, the secondary-repo credential of 02 2.6 |
| A4 | clone-mode env started with `sbx env run -d`; `sbx ls --json`; `sbx env exec -it <dir> --env K=<32 KiB value> -- sh -c ...`; two concurrent `exec -it`; `exec -- claude`; `uname -m` inside vs the host arch; `echo $HOME` as the workload user; `sbx stop`, then `sbx ls --json` again | exec works on a detached clone sandbox; `--env` delivers 32 KiB; concurrent execs coexist; the sandbox arch equals the host arch; the workload user's home recorded; `sbx ls --json` reports each sandbox's state and workspace path, field names recorded, including the mount, secret and image fields `adopt` prints; the `sbx stop` argv recorded | `run` attach model; the fields I26 and boundary F read; the recording `romeu stop` replays |
| A5 | the tree shape of 02 2.3 and the `sbxenv.yaml` goldens: relative paths, file outside mounts, `agent: sbx-kit-claude` + workload kit by digest, `readOnly` additional workspace; a top-level `x-probe` key; `sbx env plan` on each golden; a repository-local sandbox configuration planted in the primary repo | goldens accepted; `x-*` rejected; the sha256 of each golden's normalized form recorded (03 3.3); whether `sbx env plan` and `sbx env run` read only the env file romeu names, or also the planted file, recorded | render goldens; run step 3 of 04 4.2 |
| A6 | branches `feat` and `salvage/1` in the sandbox; host fetch; `sbx env rm`; list `refs/sandboxes`; same-name recreate; fetch and `fetch --prune` via the sbx remote; `mv` a clone dir with the sandbox removed, then `git fetch --all` | refs survive rm; the sbx refspec is forced; moved clone healthy | I6, `retire` move |
| A9 | `sbx policy allow network --sandbox <p> example.test`; `check`; `curl` inside; `rm --resource`; `sbx env rm`; `sbx policy ls`; trigger a block; `sbx policy approval ls`; list global secrets and global allow rules as doctor would read them | immediate effect; exact argv; how globals are reported | egress application (07), I22 |
| A10 | on a live env sandbox `sbx secret set <svc> --sandbox <p> --command '<new>'`; check inside; `sbx env run -d` again | recorded: live and kept, live and reverted, or re-plan prompt | the `apply` tag of `secrets` |
| A11 | build a minimal local v3 kit (`kind: mixin`) with the pinned frontend: files, a binary from the build context, `args`, one fixture per capability type, an unknown key (expect strict error), a YAML anchor (expect rejection or record acceptance); an install step that records where it runs | local v3 builds on this sbx; descriptor file name and capability names recorded; each capability fixture's sbx interpretation recorded as a conformance fixture for I28; where a kit install step executes (host engine or microVM) recorded, as far as the harness can observe it | go/no-go for the kits, kit format, I28 |
| A12 | inspect the workload by digest: descriptor annotation (and any `sbx kit inspect`-style command), declared capabilities and agent handle, missing apt packages vs `os-base`, skills path, agent memory dir location; the volumes the workload declares, their mount paths, and what of them survives `sbx env rm` and a recreate; with an ssh-agent capability and `SSH_AUTH_SOCK` set to a one-key agent for the `sbx` process, `ssh-add -L` inside, and again with `SSH_AUTH_SOCK` unset for the `sbx` process | capability set readable and recorded as conformance fixtures; handle is `sbx-kit-claude`; inside, only the one key is listed; with the variable unset, `ssh-add -L` lists no key | `os-base` list, I28, what holds for Q19 until it is settled, the volume list of 06 6.2 |
| A13 | hand-written herdr `layout.apply` through `sbx env exec -it <dir> -- herdr`; agent detection through the exec pty; OSC 9 reaching the host terminal/cmux; kill the host terminal, re-exec, check the agent process survived; host herdr cannot see sandbox processes; socket `protocol` number | layout applies; agent survives terminal loss; host herdr sees nothing | run-layout renderer, "no host panes" model |
| A14 | a kit install step printing `$PWD` and listing the workspace, timestamped against the clone; from inside an install step and a lifecycle hook of a local kit, the hostname, the uid, whether a host path is visible and whether the network namespace is the host's | install step does not see the clone; where install steps and lifecycle hooks run and what they reach recorded | confirms the romeu-driven `julieta setup` ordering; Q32 |
| A15 | on a detached env sandbox: `sbx stop`, then `sbx env exec`; `sbx stop` again, then `sbx env run -d` through romeu's argv, and once more after a kit, a workload and a ports change of the env file; restart Docker with the sandbox running and run `sbx ls --json` during and after; `ssh-add -L` inside after the restart; `sbx env rm` and list the workload's volumes | what exec does on a stopped sandbox recorded (it starts it or fails); whether `sbx env run -d` starts, recreates, applies, refuses or prompts on a stopped sandbox, with and without each change, recorded; state and fields reported across the restart recorded; the signing socket returns; the volumes left after `rm` recorded | 01 1.6, 08 8.5, run step 3 of 04 4.2 |
| A16 | from the maintainer's own development session, not through romeu: the type of the token the sandbox's `github` secret holds, read from its prefix, and its scopes, read from the `x-oauth-scopes` header of one authenticated API call; the value is never printed or recorded | type and scopes recorded | the token rows of 05 5.4 and ADR 0008 say measured, not inferred |

Two ids between A6 and A9 are not used: they named probes that left
the spec, and an id is not given a second meaning. An sbx upgrade under
existing sandboxes cannot be staged on demand; it is measured when the
floor moves, since the version rule of 11.4 then runs block A again.

### Recorded sessions

Block A records, besides each probe's own calls, the sessions the fake
sbx replays ([10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract)).
Each argv builder of `sbxdrv` has a row here; a builder with no row is
found by a unit test before block A runs.

| Kind | What is recorded |
|---|---|
| argv shapes | every sbx call of [04 4.2](04-cli.md#42-romeu-host) and [07 7.5](07-mise-egress.md#75-egress-derivation-internalegress): `sbx version`, `sbx ls --json`, `sbx env run -d --clone <dir>`, `sbx env run -d <dir>`, `sbx env exec` without a terminal (the julieta calls), `sbx env exec -it` (the run layout and the shell), `sbx env plan`, `sbx stop`, `sbx env rm`, `sbx policy allow`, `check` and `rm network --sandbox <name>`, `sbx policy ls`, `sbx secret set` and `sbx settings set` |
| starting sessions | the journeys CI runs with the fake sbx (10 10.1), and `adopt`, `stop`, `pull`, `salvage` on its own and `recreate` |
| failure sessions | one per failure row of [01 1.6](01-system-model.md#16-state-machines): an `sbx ls` error, an `sbx env exec` timeout, and a sandbox VM that does not start |

## 11.2 Block B - acceptance on real hosts (last)

About 90 minutes per host for B1 to B5; the sandbox-side checks below
add to that.

| ID | Step | Settles |
|---|---|---|
| B1 | install the release candidate from its GitHub release with the J1 commands of `e2e/scenarios/commands.yaml`; verify checksums and attestation; record each asset's sha256 and whether the downloaded archive carries a quarantine attribute | the install-and-verify step of J1, on the candidate |
| B2 | product kits build and a sandbox starts: linux/amd64 on the Intel host, linux/arm64 on the Apple silicon host; skills, SessionStart/SessionEnd hooks and the non-writable agent memory dir present; `ssh-add -L` shows only the signing key when `git-ssh-sign` is used, and lists no key in a project without `git-ssh-sign` | S3, S5 |
| B3 | `go test -tags host -json ./e2e/host/...` (J1-J14 on throwaway projects, including `/clear` with the real agent); for J1 from a clean host, J2 and the daily resume of a stopped sandbox (J3a), the result records the count of operator commands and prompts and the wall clock, which the acceptance PR's review compares with [00 0.2](00-scope.md#02-users) | S7 |
| B4 | `julieta setup` under the catalog-derived policy only, for the reference config repo's tools, on both arches, with and without a GitHub token | S4, catalog entries |
| B5 | the harness times `romeu run` on a warm sandbox, median of 5, and records the wall clock and romeu's own share of it (the harness points the `sbx` path of its own host settings at a wrapper that times each `sbx` subprocess, and takes that time out); as an observation, the wall clock of `romeu run` on a stopped sandbox, `sbx` time included | S8, for the romeu figure |

Each of B1 to B5 is `kind: check` (11.3): its predicate is every
property its Step cell names, B3's is that every journey passes, and
B5's is the S8 bound. Each writes a `probe-result.v1` file,
`docs/probes/B<n>-<host>.json`; for B3 the `go test -json` output is an
artifact whose sha256 the result records, and the result has one
observation per journey. That artifact is
`docs/probes/B3-<host>-test.json`, one JSON object per line, written
through the recorder's redaction (10 10.3) before it is hashed. Every
block B result, the sandbox-side checks included, carries two more
observations: the candidate's tag and the commit that tag names,
copied by the harness from the release B1 installed in the same run.
Block B records
nothing under `e2e/testdata/`; its outputs are the results and
artifacts under `docs/probes/`, so committing them does not break the
rule below. Block B also replays every argv shape of the recorded
sessions (11.1) against the candidate's sbx and compares the shape of
each output with its recording
([10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract), the
tested window).

**The release candidate.** Block B accepts built artifacts, so it needs
a release before v1.0.0 exists. The candidate is a `v*` tag with a
suffix (`v1.0.0-rc.1`), which the maintainer pushes after `ci-release`
([12 12.2](12-engineering.md#122-development-commands-and-capability-map));
`release.yml` publishes it as a prerelease (10 10.2). A result
measured on the candidate counts for v1.0.0 when, from the candidate's
commit to the v1.0.0 tag, the only paths that differ are under `docs/`
or are Markdown files at the repository root. The candidate's commit
is the one each block B result records, not the result's own
`commit`, which names the checkout the harness ran from; results of
one host that name different candidates fail.
`tools/ci acceptance` checks that for each block B
result ([10 10.5](10-testing-style.md#105-acceptance-evidence)). The
maintainer runs that check before pushing the v1.0.0 tag, as a step of
the release checklist of 12 12.2 and outside `release.yml`; a failure
found after the tag is fixed by a patch release. Any other change, a
fix to a kit or to the catalog included, needs a new candidate and
block B again.

The rule compares sources: the v1.0.0 binaries are a second build and
differ from the candidate's at least in their version stamp; the pins
(`go.sum`, `mise.lock`, the SHA-pinned actions) and the build contract
of [10 10.2](10-testing-style.md#release-and-bootstrap) aim to make the
second build the same as the first, and a change to any pin fails the
rule. Nothing compares the bytes. So after the v1.0.0 tag the
maintainer runs B1 and B2 once more, on v1.0.0 and on one host, before
the acceptance commit (12 12.2). That run's `--out` is a directory
outside the repository, so the candidate's B1 and B2 results stay: the
run is not a block B result of a candidate, and the comparison above
does not read it. Its output goes under the Evidence of the acceptance
PR and is **[review]**. Releases after v1.0.0 follow the deferred rule
in the index's [Deferred decisions](../spec.md#deferred-decisions).

**Sandbox-side checks.** Five checks, C2 to C6, run inside a sandbox.
They are part of block B and of its sitting: the same harness runs them
through `sbx env exec` in the sandbox B2 started, so their observations
are recorded and their decisions computed like any other probe's
(11.3). C3 and C5 need the pinned agent, which that sandbox has. Each
declares its `kind` in the harness.

| ID | Check |
|---|---|
| C2 | `mise exec -C <dir> -- <cmd>` vs shims in a fresh non-interactive zsh and bash |
| C3 | the pinned Claude Code version's setting to disable its own memory, whether a non-writable memory dir is tolerated, and the error text it shows on a refused memory write, so the julieta skill can name it |
| C4 | `git bundle verify`/`unbundle` under the hardened flags |
| C5 | for the pinned Claude Code version: the order in which it fires SessionEnd and SessionStart on `/clear`; which SessionStart sources fire (startup, resume, clear, compact); that the hook's output reaches the agent's context, and the size at which it is cut; what a hook's non-zero exit does; and the time a SessionEnd hook is given |
| C6 | from inside, the A3 plants in a memory dir; `julieta memory check` and the romeu checks report every plant. It settles I24 and julieta delivery (06 6.3) |

C1 is not used: it named a check that left the spec.

## 11.3 Probe result format

`schemas/probe-result.v1.json`, one file per probe per host
(`docs/probes/<id>-<host>.json`), for both blocks; a sandbox-side check
has `block: "B"`:

```json
{
  "schema": "probe-result/v1",
  "id": "A10",
  "block": "A",
  "kind": "observe",
  "commit": "<product repo commit the harness ran from>",
  "host": {"os": "darwin", "arch": "arm64", "sbxVersion": "0.46.0", "gitVersion": "2.51.0",
           "upstream": {}},
  "startedAt": "2026-10-03T10:00:00Z",
  "expect": "live-set secret is kept by the next env run without a prompt",
  "observations": [{"key": "nextRunPrompted", "type": "bool", "value": false}],
  "verdict": "pass",
  "decision": "default-overturned",
  "affects": [{"path": "internal/spec/project.go", "anchor": "Secrets"},
              {"path": "docs/spec/03-formats.md", "anchor": "32-project-spec-projectsnameyaml-schema-projectv1"}],
  "resolvedBy": null,
  "artifacts": [{"path": "e2e/testdata/sbx/0.46.0/secret-set.jsonl", "sha256": "<hex>"}]
}
```

| Field | Rule |
|---|---|
| `host.upstream` | the version of each upstream the probe reads beyond sbx, recorded by the harness: the frontend pin for A11 to A13, the agent version for C3 and C5; empty for the others |
| `kind` | `check` (a predicate over observations) or `observe` (record what happens) |
| `verdict` | `pass | fail | inconclusive`; a `check` passes when its predicate holds; an `observe` probe passes when every declared observation was recorded. A `check` records as observations the values its predicate evaluated (file names, versions, counts), through the recorder's redaction, so the verdict can be recomputed from the file |
| `decision` | `default-kept | default-overturned`, computed, never typed: each probe declares in the harness, before the run, `onPass` and `onFail` decisions (`check`) or a table from observed values to decisions (`observe`) |
| `affects` | every file and anchor that must change if the decision is `default-overturned`, declared with the probe, including each `docs/guide/` page that narrates the result. `tools/ci probes` fails when a guide page names a probe id its `affects` does not list, and resolves each docs anchor in `affects` as `tools/ci docs` resolves links, so a heading rename updates the result in the same commit |
| `resolvedBy` | `null`, or the commit that applied an overturned decision |

## 11.4 Probe lifecycle

`tools/ci probes`, a step of `all`, checks that each committed result
is schema-valid and that its `decision` equals the harness's computed
one, and, when `resolvedBy` is set, that the `resolvedBy` commit
descends from the result's `commit`, touches every `affects` path and
adds an ADR that records the decision (12 12.5), superseding the
default's ADR when one exists. An overturned result does not fail it,
so results land as soon as they are measured and the fix lands in its
own pull request, like an S6 row before its code (10 10.5).
`tools/ci acceptance` fails while a `default-overturned` result has no
`resolvedBy`. A `default-kept` result settles its open question once
each host it needs has one; an ADR follows the rule of the index's
[Open questions](../spec.md#open-questions).

A result is a fact about the versions its host records. A pin change
runs again the probes that read that pin: those of block A before the
change merges, and those of block B, sandbox-side checks included, at
the next candidate:

| Pin | Probes run again |
|---|---|
| the workload digest | A12, A15, C3, C5, and B2's S5 part |
| the kit frontend | A11 and its conformance fixtures |
| herdr | A13 |
| mise | C2, B4 |
| the sbx floor | block A |

`tools/ci probes` fails when a committed block A result of a probe this
table lists records an upstream version or digest other than its
current pin.

`tools/ci acceptance` fails when a block A result's `sbxVersion`
differs from the floor of
[10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract), or a
recorded `upstream` version differs from its current pin; a floor or
pin bump therefore runs the probes that read it again before the next
release, and the new result settles their open questions again. Git
history keeps the results it replaces.

`tools/ci probes` checks the results that are committed and says
nothing about the ones that are not: block B cannot run before a
release exists. That every probe id of this page has a result, from
both hosts when the probe is arch-sensitive,
is checked by `tools/ci acceptance`
([10 10.5](10-testing-style.md#105-acceptance-evidence)), which joins
`all` once the plan's last task has landed. Until then the tables of
this page are the reminder, and a probe nobody ran is noticed by
reading them.

`tools/ci probes` also flags an `sbxenv.yaml` golden whose normalized
hash ([03 3.3](03-formats.md#33-rendered-sbxenvyaml-sbx-env-file-schemaversion-1))
moved since probe A5 recorded it, and prints a notice, which does not
fail the check, for a golden with no recorded hash yet: it needs a
probe before it counts as confirmed.
