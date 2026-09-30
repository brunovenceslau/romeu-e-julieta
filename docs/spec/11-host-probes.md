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
go run ./e2e/probes --block A --out docs/probes/A-<host>.json
```

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
(Intel and Apple silicon).

## 11.1 Block A - sbx and runtime facts (first, in parallel with the first build layer)

About 75 minutes per host, including one idle wait that overlaps the
other probes.

| ID | Probe | Expect (written in the harness before the run) | Settles |
|---|---|---|---|
| A1 | `sbx version`, `git --version`, host global git config, VS Code trust settings and `task.allowAutomaticTasks`, symlinks from `$HOME` into the planned root; git CVE floor per series checked against the git release notes | sbx >= 0.46.0; git at or above the per-series patched release | Q10, doctor checks |
| A2 | `sbx settings set env.rememberHostCommands true`; env with a `secrets.<svc>.command`; `sbx env run -d` twice; change the command; run again | second run silent; changed command re-prompts | never `--auto-approve` is workable |
| A3 | memory dirs and the read-only mount: from inside, plant a symlink, a FIFO, a dotfile, a `.git` dir and a `.vscode/tasks.json` in a memory dir; write to the `readOnly` `.romeu/bin` mount, also as root after `mount -o remount,rw`; check the mount path equals the host path | romeu/julieta checks see every plant; no write reaches the host through the read-only mount | I24, I30, julieta delivery (06 6.3) |
| A4 | clone-mode env started with `sbx env run -d`; `sbx env exec -it <dir> --env K=<32 KiB value> -- sh -c ...`; two concurrent `exec -it`; `exec -- claude`; `uname -m` inside vs the host arch | exec works on a detached clone sandbox; `--env` delivers 32 KiB; concurrent execs coexist; the sandbox arch equals the host arch | Q15, Q21, `run` attach model |
| A5 | the tree shape of 02 2.3 and the `sbxenv.yaml` goldens: relative paths, file outside mounts, `agent: sbx-kit-claude` + workload kit by digest, `readOnly` additional workspace; a top-level `x-probe` key; `sbx env plan` on each golden | goldens accepted; `x-*` rejected; golden sha256 recorded | render goldens |
| A6 | branches `feat` and `salvage/1` in the sandbox; host fetch; `sbx env rm`; list `refs/sandboxes`; same-name recreate; fetch and `fetch --prune` via the sbx remote; `mv` a clone dir with the sandbox removed, then `git fetch --all` | refs survive rm; the sbx refspec is forced; moved clone healthy | I6, `retire` move |
| A7 | `sbx setup ssh`; `ssh <p>.sbx 'pwd; git -C <path> log -1'`; VS Code Remote-SSH | SSH works on clone mode; path equality | Q11 |
| A8 | `sbx env run -d`, idle > 1 h (started first, checked last); `sbx ls --json`; host fetch from the daemon | behavior recorded (stop or not) | Q17 |
| A9 | `sbx policy allow network --sandbox <p> example.test`; `check`; `curl` inside; `rm --resource`; `sbx env rm`; `sbx policy ls`; trigger a block; `sbx policy approval ls`; list global secrets and global allow rules as doctor would read them | immediate effect; exact argv; how globals are reported | egress application (07), I22 |
| A10 | on a live env sandbox `sbx secret set <svc> --sandbox <p> --command '<new>'`; check inside; `sbx env run -d` again | recorded: live and kept, live and reverted, or re-plan prompt | Q18 |
| A11 | build a minimal local v3 mixin with the pinned frontend: files, a binary from the build context, `args`, one fixture per capability type, an unknown key (expect strict error), a YAML anchor (expect rejection or record acceptance) | local v3 builds on this sbx; descriptor file name and capability names recorded; each capability fixture's sbx interpretation recorded as a conformance fixture for I28 | Q7 go/no-go, kit format, I28 |
| A12 | inspect the workload by digest: descriptor annotation (and any `sbx kit inspect`-style command), declared capabilities and agent handle, missing apt packages vs `os-base`, skills path, agent memory dir location; with an ssh-agent capability and `SSH_AUTH_SOCK` set to a one-key agent for the `sbx` process, `ssh-add -L` inside | capability set readable and recorded as conformance fixtures; handle is `sbx-kit-claude`; inside, only the one key is listed | Q2, Q20, Q19, `os-base` list, Q8, I28 |
| A13 | hand-written herdr `layout.apply` through `sbx env exec -it <dir> -- herdr`; agent detection through the exec pty; OSC 9 reaching the host terminal/cmux; kill the host terminal, re-exec, check the agent process survived; host herdr cannot see sandbox processes; socket `protocol` number | layout applies; agent survives terminal loss; host herdr sees nothing | Q16, run-layout renderer, "no host panes" model |
| A14 | a kit install step printing `$PWD` and listing the workspace, timestamped against the clone | install step does not see the clone | confirms the romeu-driven `julieta setup` ordering |

## 11.2 Block B - acceptance on real hosts (last)

About 90 minutes per host.

| ID | Step | Settles |
|---|---|---|
| B1 | install the release candidate from the GitHub release; verify checksums and attestation | S1 |
| B2 | product kits build and a sandbox starts: linux/amd64 on the Intel host, linux/arm64 on the Apple silicon host; skills, SessionStart/SessionEnd hooks and the non-writable agent memory dir present; `ssh-add -L` shows only the signing key when `git-ssh-sign` is used | S3, S5, Q19 |
| B3 | `go test -tags host -json ./e2e/host/...` (J1-J13 on throwaway projects, including `/clear` with the real agent) | S7, Q22 |
| B4 | `julieta setup` under the catalog-derived policy only, for the reference config repo's tools, on both arches, with and without a GitHub token | S4, catalog entries |
| B5 | `romeu run --timings --json` on a warm sandbox, median of 5 | S8 confirmation |

Each block B step writes a `probe-result.v1` file,
`docs/probes/B<n>-<host>.json`; for B3 the `go test -json` output is an
artifact whose sha256 the result records, and the result has one
observation per journey.

## 11.3 Probe result format

`schemas/probe-result.v1.json`, one file per probe per host
(`docs/probes/<id>-<host>.json`), for blocks A, B and C alike:

```json
{
  "version": 1,
  "id": "A10",
  "block": "A",
  "kind": "observe",
  "commit": "<product repo commit the harness ran from>",
  "host": {"os": "darwin", "arch": "arm64", "sbxVersion": "0.46.0", "gitVersion": "2.51.0"},
  "startedAt": "2026-10-03T10:00:00Z",
  "expect": "live-set secret is kept by the next env run without a prompt",
  "observations": [{"key": "nextRunPrompted", "type": "bool", "value": false}],
  "verdict": "pass",
  "settles": ["Q18"],
  "decision": "default-overturned",
  "affects": [{"path": "internal/spec/project.go", "anchor": "Secrets"},
              {"path": "docs/spec/03-formats.md", "anchor": "32-project-spec-projectsnameyaml-schema-projectv1"}],
  "resolvedBy": null,
  "artifacts": [{"path": "e2e/testdata/sbx/0.46.0/secret-set.jsonl", "sha256": "<hex>"}]
}
```

| Field | Rule |
|---|---|
| `kind` | `check` (a predicate over observations) or `observe` (record what happens) |
| `verdict` | `pass | fail | inconclusive`; a `check` passes when its predicate holds; an `observe` probe passes when every declared observation was recorded |
| `decision` | `default-kept | default-overturned`, computed, never typed: each probe declares in the harness, before the run, `onPass` and `onFail` decisions (`check`) or a table from observed values to decisions (`observe`) |
| `affects` | every file and anchor that must change if the decision is `default-overturned`, declared with the probe |
| `resolvedBy` | `null`, or the commit that applied an overturned decision |

Sandbox-side checks use the same schema with `block: "C"`, run by the
maintainer inside a sandbox:

| ID | Check |
|---|---|
| C1 | `aqua:gohugoio/hugo` provides the extended variant |
| C2 | `mise exec -C <dir> -- <cmd>` vs shims in a fresh non-interactive zsh and bash |
| C3 | the pinned Claude Code version's setting to disable its own memory, and whether a non-writable memory dir is tolerated |
| C4 | `git bundle verify`/`unbundle` under the hardened flags |
| C5 | the order in which the pinned Claude Code version fires SessionEnd and SessionStart on `/clear` |

## 11.4 Probe lifecycle

A probe result moves through one explicit lifecycle, checked by
`tools/ci probes`:

| From | Event | To | Check |
|---|---|---|---|
| (none) | result committed | recorded | schema valid; `decision` equals the harness's computed decision; for an arch-sensitive probe, results from both hosts |
| recorded | `decision: default-kept` | kept | nothing else to do; the open question is settled |
| recorded | `decision: default-overturned` | overturned | `tools/ci probes` fails until the result is resolved |
| overturned | a commit sets `resolvedBy` to the earlier commit that applied the decision | resolved | the `resolvedBy` commit descends from the result's `commit`, touches every `affects` path, and adds an ADR that supersedes the default (12 12.5) |

`tools/ci probes` also requires a committed result for every probe named
in the open questions' "Settled by" column, and flags an `sbxenv.yaml`
golden whose sha256 moved since probe A5 recorded it.
