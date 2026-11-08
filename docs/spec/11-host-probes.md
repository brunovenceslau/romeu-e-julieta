# 11. Host probes

Back to [index](../spec.md). Reader: the maintainer running probes and
implementers of `e2e/probes`. Type: reference.

`sbx` runs only on the host, so these measurements are maintainer steps,
grouped into **two blocks**. Block A needs no product code (only the
probe harness) and is the last word on every sbx fact: the facts arrive
first from the maintainer's daily use and from the kit-frontend
go/no-go, each marked with its source, and block A re-measures them
before the release candidate. Block B only accepts built artifacts, so a
failure there is fixed inside the artifact (a kit, a catalog entry), not
in the architecture.

```
go run ./e2e/probes --block A --out docs/probes/
```

`REQ-0976` `--out` names a directory: the harness writes one file per probe there
(11.3).

`REQ-0977` The harness creates throwaway repos and env dirs under a temp root, runs
each step, pauses only where a human must answer an sbx prompt or look at
VS Code, records sbx help text and every argv/stdout/stderr/exit into
`e2e/testdata/sbx/<version>/` (the redacted fake-sbx recordings, [10
10.3](10-testing-style.md#103-fake-sbx-fidelity-contract)), writes
`probe-result.v1` JSON, and cleans up. Benign payloads only (`touch
<marker>`).

`REQ-0978` The harness has two layers: a pure core (probe definitions, predicates,
decision functions, result writing, redaction) and a thin exec layer
that only starts `sbx` and captures its output. Both are unit-tested in
CI (the exec layer against a helper binary), so `tools/ci all` passes
before any host run. `REQ-0979` The arch-sensitive probes (A4, A5, A11, A12, A13),
and A17, whose answer can differ by macOS release, count as settled only
with results from both hosts (Intel and Apple silicon). `REQ-0980` The A5 goldens
are written by hand with the probe definitions; `render`
([12 12.2](12-engineering.md#122-development-commands-and-capability-map))
must reproduce them byte for byte, and `tools/ci probes` already flags a
moved hash of their normalized form (03 3.3).

The Settles column of the tables below names what a probe decides
besides open questions: success criteria, invariants, formats. Which
open question a probe settles is written once, in the "Settled by"
column of the index's [Open questions](../spec.md#open-questions).

## 11.1 Block A - sbx and runtime facts

At most about 75 minutes per host.

| ID | Probe | Expect (written in the harness before the run) | Settles |
|---|---|---|---|
| A1 | `REQ-0981` `sbx version`, `git --version`, host global git config, VS Code trust settings and `task.allowAutomaticTasks`, symlinks from `$HOME` into the planned root; git CVE floor per series checked against the git release notes; the sbx settings read `doctor` uses, for `env.rememberHostCommands`, `kit.allowedSources` and `skills.defaultMode` (added on 2026-11-07) | sbx at or above the floor of 12 12.1; git at or above the per-series patched release; the argv and output of the settings read recorded | the git floor, doctor checks |
| A2 | `REQ-0982` `sbx settings set env.rememberHostCommands true`; env with a `secrets.<svc>.command`, the command a `security find-generic-password` call, started from a non-interactive script; `sbx env run -d` twice; change the command; run again | second run silent; changed command re-prompts; whether a keychain dialog appears recorded | that romeu works without `--auto-approve` (I4); whether J1 or `doctor` must cover the keychain prompt |
| A3 | `REQ-0983` memory dirs and the read-only mounts: from inside, plant a symlink, a FIFO, a dotfile, a `.git` dir and a `.vscode/tasks.json` in a memory dir; write to the `readOnly` `.romeu/bin` mount, also as root after `mount -o remount,rw`; write to the primary clone's source mount, as the agent and as root; write, as the agent and as root, also after `mount -o remount,rw` of the bind and through the share's root mounted anew, to each bind sbx makes itself (the env file, each kit source, each skill dir of an `agent-skill@1` entry of a rendered kit, and the host's skills store); check each mount is at its target of 02 2.5 (the primary clone at its host path), and that `/opt/julieta/bin/julieta` resolves to the binary through the mount; as root, create a setuid file and a device node in a memory dir, and read their mode and owner on the host; record the modes of `<name>-env/` and of the memory files as the sandbox sees them through the direct mounts; with a scoped `github` secret, clone a private secondary, push a branch and call the GitHub API | each plant is seen on the host as planted; no write reaches the host through a `readOnly` mount, also as root after the remount; no write reaches the primary host clone; no write reaches the host's skills store, also as root after the remount and through the share's root; whether each bind sbx makes itself is read-only recorded (02 2.3, REQ-0142; 01 1.5 step 6); each mount is at its target and the link resolves; what the host sees of the setuid file and the device node recorded (06 6.3, REQ-0588); the modes are recorded; the clone, the push and the API call succeed with no credential file in the sandbox | the read-only host clone (02 2.3), I30, the mount half of I24, the modes of 02 2.3, the secondary-repo credential of 02 2.6 |
| A4 | `REQ-0984` clone-mode env started with `sbx env run -d`; `sbx ls --json`; `sbx env exec -it <dir> --env K=<32 KiB value> -- sh -c ...`; two concurrent `exec -it`; `exec -- claude`; `uname -m` inside vs the host arch; `echo $HOME` as the workload user; `sbx inspect` of the sandbox; `sbx env exec` of a command that cannot run (a path that does not exist), and `sbx env exec` on a sandbox sbx cannot reach (its daemon stopped); each argv shape of REQ-0997 with `--no-input`, one that would prompt among them, and the two offers of sbx 0.51 (`env exec` on a sandbox that does not exist, `env run` on one whose sbx records were removed by hand) provoked where the harness can; with `skills.defaultMode` set to `off`, a create through `sbx env run -d --clone <dir>` and its binds read back (whether the env-file create honours the setting; inferred from the setting's help until then); `sbx stop`, then `sbx ls --json` again; the state `sbx ls --json` shows for a sandbox whose state it cannot read, provoked where the harness can | exec works on a detached clone sandbox; `--env` delivers 32 KiB; concurrent execs coexist; the sandbox arch equals the host arch; the workload user's home recorded; `sbx ls --json` reports each sandbox's state and workspace path, field names recorded, including the mount fields (source, target, the marker of a missing source; 01 1.5 step 6), the mount fields it lists for a stopped sandbox (an empty list included), the source, target and read-only flag of each bind sbx adds itself (the env file, each kit source, each skill dir of an `agent-skill@1` of a rendered kit, and the host's skills store under the `skills.defaultMode` in force, seen read-only at `/run/sbx/skills-shared` under `readonly` on 2026-11-07, sbx v0.51.0; 01 1.5 step 6, REQ-1381, 02 2.3), and the secret and image fields `adopt` prints; the field of `sbx inspect` that names the creating sbx version (03 3.8) recorded; whether `env exec` returns 125 for a command it cannot run, and what it returns when sbx cannot reach the sandbox, recorded (06 6.3); per argv shape, whether `--no-input` is accepted and whether a prompt under it ends in an error or in sbx's default, that no prompt under it ends as a yes, and that the call changed nothing (04 4.1, I4) recorded; what each offer looks like with and without the flag recorded; the unreadable state's shape, when provoked, recorded (01 1.6); the `sbx stop` argv recorded; an observation the harness could not provoke on a host is recorded as `not-provoked`, which counts as recorded and keeps the default, so that shape carries no `--no-input` (04 4.1), and the result names each shape provoked on neither host, whose synthetic session 10 10.3 admits (REQ-0924) | `run` attach model; the fields I26 and boundary F read; the recording `romeu stop` replays |
| A5 | `REQ-0985` the tree shape of 02 2.3 and the `sbxenv.yaml` goldens: relative paths, file outside mounts, `agent: sbx-kit-claude` + workload kit by digest, `readOnly` additional workspace with its `target`, `entrypoint` and `cmd`, and no `skills` key (03 3.3); a top-level `x-probe` key; a variant of a golden with a `skills: "off"` key added, which is no golden and whose hash is not recorded; `sbx env plan` on each golden; a repository-local sandbox configuration planted in the primary repo; a `~/.sbxenv.yaml` holding a `ports` entry on a `127.0.0.1` port nothing listens on, planted under a home the harness creates for the sbx process, never the operator's `$HOME`, then `sbx env plan` again, and once more with the file moved and an environment variable of sbx naming it, where the notes name one; the harness refuses to start A5 while the operator's own `~/.sbxenv.yaml` exists and removes its plant on every exit path, and it never plants a file in the operator's home: where sbx cannot be pointed at another home, those observations are recorded as `not-provoked` (amended on 2026-11-07) | goldens accepted; the `skills` variant rejected or accepted recorded (refused with `unknown field "skills"`, exit 1, measured by hand on 2026-11-07, sbx v0.51.0), an acceptance being the event that reopens the skills row of [Deferred decisions](../spec.md#in-the-product) (03 3.3, REQ-0200; amended on 2026-11-07); `x-*` rejected; the sha256 of each golden's normalized form recorded (03 3.3); whether `sbx env plan` and `sbx env run` read only the env file romeu names, or also the planted file, recorded; whether the planted user-level file's entry appears in the plan, and whether any other location or variable reaches the plan, recorded (04 4.2, `sbx-env-base`) | render goldens; run step 3 of 04 4.2 |
| A6 | `REQ-0986` branches `feat` and `salvage/1` in the sandbox; host fetch; `sbx env rm`; list `refs/sandboxes`; same-name recreate; fetch and `fetch --prune` via the sbx remote; `mv` a clone dir with the sandbox removed, then `git fetch --all` | refs survive rm; the sbx refspec is forced; moved clone healthy | I6, `retire` move |
| A9 | `REQ-0987` `sbx policy allow network --sandbox <p> example.test`; `check`; `curl` inside; `rm --resource`; `sbx env rm`; `sbx policy ls`; trigger a block; `sbx policy approval ls`; list global secrets and global allow rules as doctor would read them; `sbx policy ls --created-via` (sbx 0.45) with a rule made by `sbx policy allow` and one sbx provisioned from a kit descriptor present; on a sandbox whose kit descriptor declares a runtime egress host and port, `sbx policy deny network --sandbox <p> <host>:<port>` on that pair, then a request to it from inside; the same request again with a portless `sbx policy allow network --sandbox <p> <host>` of the same host present, and again after `sbx env rm` and `sbx env run -d --clone` under the same name | immediate effect; exact argv; how globals are reported; the `--created-via` value of each of the two rules recorded (04 4.2, `egress-applied`); whether the per-sandbox deny beats the provisioned allow, whether it still holds beside a portless allow of the same host, and whether it survives the same-name recreate, recorded (Q2, the plain-HTTP row of 05 5.4) | egress application (07), I22 |
| A10 | `REQ-0988` on a live env sandbox `sbx secret set <svc> --sandbox <p> --command '<new>'`; check inside; `sbx env run -d` again | recorded: live and kept, live and reverted, or re-plan prompt | the `apply` tag of `secrets` |
| A11 | `REQ-0989` build a minimal local v3 kit (`kind: mixin`) with the pinned frontend: files, a binary from the build context, `args`, one fixture per capability type, an unknown key (expect strict error), a YAML anchor (expect rejection or record acceptance); an install step that records where it runs | local v3 builds on this sbx; descriptor file name and capability names recorded; each capability fixture's sbx interpretation, read with `sbx kit inspect --offline --format json` on the fixture kit (sbx 0.49.1), recorded as a conformance fixture for I28, the sbx-side reading romeu's normalized set is compared with (Q20); where a kit install step executes (host engine or microVM) recorded, as far as the harness can observe it | go/no-go for the kits, kit format, I28 |
| A12 | `REQ-0990` inspect the workload by digest: descriptor annotation (and any `sbx kit inspect`-style command), declared capabilities and agent handle, missing apt packages vs `os-base`, skills path, agent memory dir location; the volumes the workload declares, their mount paths, and what of them survives `sbx env rm` and a recreate; with an ssh-agent capability and `SSH_AUTH_SOCK` set to a one-key agent for the `sbx` process, `ssh-add -L` inside, and again with `SSH_AUTH_SOCK` unset for the `sbx` process | capability set readable and recorded as conformance fixtures, beside the reading of `sbx kit inspect --format json` on the pinned reference (Q20); the derived `provides` of the descriptor (kit spec m.5), with `deb/tzdata` and `deb/zsh` present or absent, recorded beside the missing-package measurement, since `julieta pin workload` reads it (04 4.3); handle is the one the descriptor declares (`sbx-kit-claude` today, Q2); inside, only the one key is listed; with the variable unset, `ssh-add -L` lists no key | `os-base` list, I28, what holds for the socket forwarding of 06 6.4 until it is settled, the volume list of 06 6.2 and the volume row of 01 1.2 |
| A13 | `REQ-0991` hand-written herdr `layout.apply` through `sbx env exec -it <dir> -- herdr`; agent detection through the exec pty; OSC 9 reaching the host terminal (cmux is the one the maintainer runs); kill the host terminal, re-exec, check the agent process survived; host herdr cannot see sandbox processes; socket `protocol` number | layout applies; agent survives terminal loss; host herdr sees nothing | run-layout renderer, "no host panes" model |
| A14 | `REQ-0992` a kit install step printing `$PWD` and listing the workspace, timestamped against the clone; from inside an install step and a lifecycle hook of a local kit, the hostname, the uid, whether a host path is visible and whether the network namespace is the host's; for a `lifecycle.startup` hook, whether it sees the clone, the bin and memory mounts and a declared `JULIETA_MANIFEST`, whether the session waits for it, whether its non-zero exit stops the boot, and the sandbox's egress policy at the moment it runs on a create and on a restart | install step does not see the clone; where install steps and lifecycle hooks run and what they reach recorded | confirms the romeu-driven `julieta setup` ordering; whether personal kits may carry install steps and lifecycle hooks (06 6.1) |
| A15 | `REQ-0993` on a detached env sandbox: `sbx stop`, then `sbx env exec`; with the env file in `<dir>` changed, `sbx stop`, then `sbx env exec <dir> -- true`, followed by `sbx ls --json`, `sbx inspect`, `sbx policy ls`, a marker file written before the stop, and the sandbox id (T210 records this first by hand too); on a running sandbox with the env file in `<dir>` changed, `sbx env exec <dir> -- true` and `sbx env exec -it <dir>`, with `sbx ls --json`, `sbx inspect`, `sbx policy ls`, the marker and the sandbox id read before and after (T210 records this first by hand too); `sbx stop` again, then `sbx env run -d` through romeu's argv with the env file unchanged; then the in-place apply measurement, which T210 records first by hand on sbx 0.51, each argv with its output, exit status and sbx version, and which this probe re-records with the harness: on a running and on a stopped sandbox, `sbx env run -d <dir>` with an env file changed in one kind at a time (a kit added, a kit's content changed, a kit removed; the workload; a `ports` entry; each of `rootSize`, `dockerSize` and `clonedWorkspaceSize` of 03 3.1; `agent`; a repo added, a repo removed; an additional workspace's `target` or `readOnly`; a secrets binding added, a secrets binding removed), each preceded by a marker file written inside and followed by `sbx ls --json`, `sbx inspect`, `sbx policy ls`, the marker, `git status` in the primary clone inside, and the agent's login; the same changed env file applied twice, and once more after an apply whose sbx client was killed mid-apply; sbx's plan prompt declined, on a running and on a stopped sandbox, then `sbx ls --json` and `sbx inspect`; `sbx env run <dir>` without `-d`, with and without `--no-input`, on a running sandbox whose env file equals the one it runs and carries the `entrypoint` and `cmd` of 03 3.3 (REQ-1370; a `cmd` with no `entrypoint` failed with `unknown option '-lc'` and `sbx env plan` accepted the two keys, measured by hand on 2026-11-07, sbx v0.51.0, so the attached run with both is what stays to record), with `sbx policy ls`, the marker and the process list inside read before and after; `sbx env run -d --clone` and `sbx env run -d` on that env file, with the process list inside read after each; restart Docker with the sandbox running and run `sbx ls --json` during and after; `ssh-add -L` inside after the restart | what exec does on a stopped sandbox recorded (it starts it or fails); with a changed env file in `<dir>`, whether it starts the sandbox and whether the configuration it was created with stays in force (the mount fields, the policy rules, the marker and the sandbox id unchanged) recorded: if both hold, that call becomes the start of a stopped sandbox whose recreate digests differ, and the refusal of 04 4.2 (REQ-1376, REQ-1387) is lifted (decided on 2026-11-07); on a running sandbox, whether `sbx env exec <dir>` with a changed env file in `<dir>` leaves the configuration unchanged (mount fields, policy rules, marker, sandbox id) and prompts for nothing, recorded (04 4.2 step 3, the exec row of 05 5.4; added on 2026-11-07); whether `sbx env run -d` with an unchanged env file starts, recreates, refuses or prompts on a stopped sandbox recorded; for each change kind on each state, the exit status, whether the instance stays (the sandbox id or creation time I26 reads, the marker, `git status` and the login unchanged), and whether the change took effect as read back through sbx (`sbx inspect`, `sbx policy ls`, the mount fields of `sbx ls --json`), or whether sbx recreated or refused, recorded (`run` step 3, the guide); an exit 0 that keeps the instance and leaves the change unapplied is recorded as such, never as an apply; for a removed repo, whether its sandbox-side state stays reachable, for a removed secrets binding, whether it stops reaching the sandbox, and for a removed kit, what it leaves in the filesystem and in the policy rules, recorded; whether the repeated apply is a no-op, applies again or fails, and what the interrupted one leaves, recorded; the exit status of a declined plan, and that nothing changed, recorded; whether the attached `sbx env run <dir>` attaches, runs the `entrypoint` with its `cmd`, provisions nothing and leaves the policy rules and the marker unchanged, with and without the flag, or what it does instead, recorded (`run` step 7); whether a detached create or start runs the `entrypoint` and `cmd` recorded (03 3.3); state and fields reported across the restart recorded; the signing socket returns | the in-place apply row of [Deferred decisions](../spec.md#in-the-product), reopened when T210's measurement passes (as decided on 2026-11-07); 01 1.6, 08 8.5, run steps 3 and 7 and the salvage start (REQ-1387) of 04 4.2, `entrypoint` and `cmd` of 03 3.3, the removed-kit row of 05 5.4 (REQ-1375) |
| A16 | `REQ-0994` run in the maintainer's host shell, not through romeu: the type of the token the sandbox's `github` secret holds, read from its prefix, and its scopes, read from the `x-oauth-scopes` header of one authenticated API call; for a token with no scopes header, a fine-grained token or a GitHub App installation token, the result is its repository list and permissions, read from GitHub's settings page of that token or of the App's installation and recorded by hand **[review]**; the value is never printed or recorded | type and scopes (or repository list and permissions) recorded | the token rows of 05 5.4 and [ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md) say measured, not inferred |
| A17 | `REQ-0995` in a `<name>-env/` that holds its `.metadata_never_index` marker, plant a file carrying a unique token under `memory/<dir>/`; wait for the indexer, then `mdfind -onlyin <name>-env <token>`; again with the marker removed | with the marker present, `mdfind` does not find the file; the result counts only when, with the marker removed, `mdfind` finds the file; otherwise it is inconclusive and run again; if the marker does not keep the file out, the form that does is measured the same way and replaces it | the indexer marker of 02 2.3 and boundary D (01 1.3), the `root-indexed` check of 04 4.2, the indexer row of 05 5.4 |
| A18 | `REQ-1368` the caches across sandboxes and recreates: a probe kit declaring a `volume@1` at `/home/agent/.local/share/mise`, a sandbox created with `sbx env run -d --clone`, a marker file written into the volume, then `sbx env rm` and `sbx env run -d --clone` under the same name; and two sandboxes sharing one host directory (a `host-mount@1` of one probe kit, or the same directory as an additional workspace of both): a `flock` held in one and tried in the other, then two `mise install` of one tool version started at once with the shared directory as both data dir and cache dir | whether the marker survives the same-name rm and create recorded (kit spec m.10 says the volume goes with the instance, the sbx 0.50 notes say it stays); whether the second `flock` blocks recorded; whether both installs end with one usable tool and no `incomplete` marker recorded (added on 2026-11-06) | the caches row of [Deferred decisions](../spec.md#in-the-product); the volume rows of 06 6.2 and 01 1.2 |
| A19 | `REQ-1369` the engine of the `docker.io/sbx/claude` workload (Q2), in a sandbox created with `sbx env run -d --clone`: inside, the uid and capabilities the engine runs with, the group of its socket, what the agent user reaches of it, `docker run hello-world` as the agent user, and the size and owner of `/var/lib/docker` with and without `sandboxOptions.dockerSize`; then a local v3 kit declaring `com.docker.sandbox/dockerd@1` (kit spec m.12), built with the pinned frontend, on a workload without an engine | how the workload starts its engine and what the agent reaches recorded; whether sbx advertises `dockerd@1` recorded (a required entry refused or admitted, and an optional one skipped); the size and owner of `/var/lib/docker` with and without `dockerSize` recorded; the elevation each costs recorded, for Q33 of the index (added on 2026-11-06); whether the agent user reaches the engine's socket recorded, which the pin move of Q2 waits on (a reach needs a 05 5.4 row that accepts it by name, or the pin waits); the workload part is measured first by hand inside the pin's measurement of Q2, beside A9's deny, before the pin moves, and this probe re-records it (decided on 2026-11-07) | the `dockerd@1` row of [Deferred decisions](../spec.md#in-the-product); `sandboxOptions.dockerSize` of 03 3.1 |

Two ids between A6 and A9 are not used: they named probes that left this
specification, and an id is not given a second meaning. An sbx upgrade
under existing sandboxes cannot be staged on demand; it is measured when
the floor moves, since the version rule of 11.4 then runs block A again.

### Recorded sessions

Block A records, besides each probe's own calls, the sessions the fake
sbx replays ([10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract)).
`REQ-0996` Each argv builder of `sbxdrv` has a row here; a builder with no row is
found by a unit test before block A runs.

| Kind | What is recorded |
|---|---|
| argv shapes | `REQ-0997` every sbx call of [04 4.2](04-cli.md#42-romeu-host) and [07 7.5](07-mise-egress.md#75-egress-derivation-internalegress): `sbx version`, `sbx ls --json`, `sbx inspect`, `sbx env run -d --clone <dir>`, `sbx env run -d <dir>`, `sbx env run <dir>` (the attach of `run` step 7), `sbx env exec` without a terminal (the julieta calls), `sbx env exec -it` (the shell), `sbx env plan`, `sbx stop`, `sbx env rm`, `sbx policy allow`, `check` and `rm network --sandbox <name>`, `sbx policy ls` (with `--created-via`), `sbx secret set`, `sbx secret ls`, `sbx settings set` and the settings listing `doctor` reads (A1), each with `--no-input` where A4 records the shape accepts it |
| starting sessions | `REQ-0998` the journeys CI runs with the fake sbx (10 10.1), and `adopt`, `stop`, `pull`, `salvage` on its own and `recreate` |
| failure sessions | `REQ-0999` one per failure row of [01 1.6](01-system-model.md#16-state-machines): an `sbx ls` error, an `sbx env exec` timeout, an `sbx env run` timeout, and a sandbox VM that does not start |

## 11.2 Block B - acceptance on real hosts (last)

About 90 minutes per host for B1 to B6; the sandbox-side checks below
add to that.

| ID | Step | Settles |
|---|---|---|
| B1 | `REQ-1000` install the release candidate from its GitHub release with the J1 commands of `e2e/scenarios/j1/commands.yaml`; verify checksums and attestation; record the `gh` version, each asset's sha256 and whether the downloaded archive carries a quarantine attribute | the install-and-verify step of J1, on the candidate |
| B2 | `REQ-1001` product kits build and a sandbox starts: linux/amd64 on the Intel host, linux/arm64 on the Apple silicon host; the skills present at the agent's discovery directory, with the `skills.defaultMode` in force recorded and, where the host's store is shared, whether it holds a skill of the same name as a bundled one (06 6.4, REQ-0595; amended on 2026-11-07), SessionStart/SessionEnd hooks and the non-writable agent memory dir present; `ssh-add -L` shows only the signing key when `git-ssh-sign` is used, and lists no key in a project without `git-ssh-sign`; with `git-ssh-sign`, the harness starts the dedicated signing agent with `SSH_ASKPASS` pointing at a wrapper that records each call and answers yes, adds the key with `ssh-add -c`, makes a commit in the sandbox, and records the wrapper's calls and whether the commit is signed; `REQ-1002` it then records the outcome of the same signing use with no askpass installed: refused, or signed with no confirmation, and the error text the agent sees, and the outcome of a confirmation that is not answered within the agent's tool timeout: whether the commit fails with no object written or waits, and for how long (05 5.4, [06 6.4](06-kits.md#64-product-kits)) | S3, S5 |
| B3 | `REQ-1003` `go test -tags host -json ./e2e/host/...` (J1-J14 on throwaway projects, including `/clear` with the real agent); for J1 from a clean host, J2 and the daily resume of a stopped sandbox (J3a), the result records the count of operator commands and prompts and the wall clock, which the acceptance PR's review compares with [00 0.2](00-scope.md#02-users) | S7 |
| B4 | `REQ-1004` `julieta setup` under the catalog-derived policy only, for the reference config repo's tools, on both arches, with and without a GitHub token | S4, catalog entries |
| B5 | `REQ-1005` the harness times `romeu run` on a warm sandbox, median of 5, and records the wall clock and romeu's own share of it (the harness points the `sbx` path of its own host settings at a wrapper that times each `sbx` subprocess, and takes that time out); as an observation, the wall clock of `romeu run` on a stopped sandbox, `sbx` time included | S8, for the romeu figure |
| B6 | `REQ-1006` the harness replays each argv shape of the recorded sessions (11.1) that only reads (`sbx version`, `sbx ls --json`, `sbx inspect`, `sbx env plan`, `sbx policy ls`, `sbx secret ls`, the settings listing of A1) against the candidate's sbx, in its own throwaway sandbox and env dir and in nothing else, and compares the shape of each output with its recording | the tested window of [10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract) at the candidate's sbx |

`REQ-1007` Each of B1 to B6 is `kind: check` (11.3): its predicate is every
property its Step cell names, B3's is that every journey passes,
B5's is the S8 bound, and B6's is that every replayed shape matches.
B6 records one observation per shape, the shape and the result of its
comparison, and a fail names each shape that differs. `REQ-1008` Each writes a
`probe-result.v1` file, `docs/probes/B<n>-<host>.json`; for B3 the `go test -json` output is an
artifact whose sha256 the result records, and the result has one
observation per journey. That artifact is
`docs/probes/B3-<host>-test.json`, one JSON object per line, written
through the recorder's redaction (10 10.3) before it is hashed. `REQ-1009` Every
block B result, the sandbox-side checks included, carries two more
observations: the candidate's tag and the commit that tag names,
copied by the harness from the release B1 installed in the same run.
`REQ-1010` Block B records
nothing under `e2e/testdata/`; its outputs are the results and
artifacts under `docs/probes/`, so committing them does not break the
rule below.

**The release candidate.** `REQ-1011` Block B accepts built artifacts, so it needs
a release before v1.0.0 exists. The candidate is a `v*` tag with a
suffix (`v1.0.0-rc.1`), which the maintainer pushes after `ci-release`
([12 12.2](12-engineering.md#122-development-commands-and-capability-map));
`REQ-1012` `release.yml` publishes it as a prerelease (10 10.2). A result
measured on the candidate counts for v1.0.0 when, from the candidate's
commit to the v1.0.0 tag, the only paths that differ are under `docs/`
or are Markdown files at the repository root. The candidate's commit
is the one each block B result records, not the result's own
`commit`, which names the checkout the harness ran from; results of
one host that name different candidates fail.
`REQ-1013` `tools/ci acceptance` checks that for each block B
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
rule. `REQ-1014` Nothing compares the bytes. So after the v1.0.0 tag the
maintainer runs B1 and B2 once more, on v1.0.0 and on one host, before
the acceptance commit (12 12.2). That run's `--out` is a directory
outside the repository, so the candidate's B1 and B2 results stay: the
run is not a block B result of a candidate, and the comparison above
does not read it. Its output goes under the Evidence of the acceptance
PR and is **[review]**. Releases after v1.0.0 follow the deferred rule
in the index's [Deferred decisions](../spec.md#deferred-decisions).

`REQ-1015` **Sandbox-side checks.** Five checks, C2 to C6, run inside a sandbox.
They are part of block B and of its sitting: the same harness runs them
through `sbx env exec` in the sandbox B2 started, so their observations
are recorded and their decisions computed like any other probe's
(11.3). C3 and C5 need the pinned agent, which that sandbox has. Each
declares its `kind` in the harness.

| ID | Check |
|---|---|
| C2 | `REQ-1016` `mise exec -C <dir> -- <cmd>` vs shims in a fresh non-interactive zsh and bash |
| C3 | `REQ-1017` the pinned Claude Code version's setting to disable its own memory, whether a non-writable memory dir is tolerated, and the error text it shows on a refused memory write, so the julieta skill can name it |
| C4 | `REQ-1018` `git bundle verify`/`unbundle` under the hardened flags |
| C5 | `REQ-1019` for the pinned Claude Code version: the order in which it fires SessionEnd and SessionStart on `/clear`; which SessionStart sources fire (startup, resume, clear, compact); that the hook's output reaches the agent's context, and the size at which it is cut; what a hook's non-zero exit does, and that the stdout of a failing hook still reaches the context; and the time a SessionEnd hook is given; that a PreToolUse wrapper that exits 2 because its program cannot run blocks the tool call; and that each required agent-profile path of [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction) step 5 exists |
| C6 | `REQ-1020` from inside, the A3 plants in a memory dir; `julieta memory check` and the romeu checks report every plant. It settles I24 and julieta delivery (06 6.3) |

C1 is not used: it named a check that left this specification.

## 11.3 Probe result format

`REQ-1021` `schemas/probe-result.v1.json`, one file per probe per host
(`docs/probes/<id>-<host>.json`), for both blocks; `REQ-1022` a sandbox-side check
has `block: "B"`. A block A result at an sbx version above the floor,
which widening the tested window records (11.4), sits at
`docs/probes/<id>-<host>-<sbxVersion>.json` beside it:

```json
{
  "schema": "probe-result/v1",
  "id": "A10",
  "block": "A",
  "kind": "observe",
  "commit": "<product repo commit the harness ran from>",
  "host": {"os": "darwin", "arch": "arm64", "sbxVersion": "0.51.0", "gitVersion": "2.51.0",
           "upstream": {}},
  "startedAt": "2026-10-03T10:00:00Z",
  "expect": "live-set secret is kept by the next env run without a prompt",
  "observations": [{"key": "nextRunPrompted", "type": "bool", "value": false}],
  "verdict": "pass",
  "decision": "default-overturned",
  "affects": [{"path": "internal/spec/project.go", "anchor": "Secrets"},
              {"path": "docs/spec/03-formats.md", "anchor": "32-project-spec-projectsnameyaml-schema-projectv1"}],
  "resolvedBy": null,
  "artifacts": [{"path": "e2e/testdata/sbx/0.51.0/secret-set.jsonl", "sha256": "<hex>"}]
}
```

| Field | Rule |
|---|---|
| `host.upstream` | `REQ-1023` a map keyed by pin, `frontend`, `workload`, `herdr`, `mise` and `agent`, recorded by the harness: for every probe the table of 11.4 lists under a pin, the key of that pin holds the version or digest the probe read, and `agent`, which no pin names and no check compares, holds the agent version C3 and C5 read; empty for the others |
| `kind` | `check` (a predicate over observations) or `observe` (record what happens) |
| `verdict` | `REQ-1024` `pass \| fail \| inconclusive`; a `check` passes when its predicate holds; an `observe` probe passes when every declared observation was recorded. A `check` records as observations the values its predicate evaluated (file names, versions, counts), through the recorder's redaction, so the verdict can be recomputed from the file |
| `decision` | `REQ-1025` `default-kept \| default-overturned`, computed, never typed: each probe declares in the harness, before the run, `onPass` and `onFail` decisions (`check`) or a table from observed values to decisions (`observe`) |
| `affects` | `REQ-1026` every file and anchor that must change if the decision is `default-overturned`, declared with the probe, including each `docs/guide/` page that narrates the result. `tools/ci probes` fails when a guide page names a probe id its `affects` does not list, and resolves each docs anchor in `affects` as `tools/ci docs` resolves links, so a heading rename updates the result in the same commit |
| `resolvedBy` | `REQ-1027` `null`, or the commit that applied an overturned decision |

## 11.4 Probe lifecycle

`REQ-1028` `tools/ci probes`, a step of `all`, checks that each committed result
is schema-valid and that its `decision` equals the harness's computed
one, and, when `resolvedBy` is set, that the `resolvedBy` commit
descends from the result's `commit`, touches every `affects` path and
adds an ADR that records the decision (12 12.5), superseding the
default's ADR when one exists. An overturned result does not fail it,
so results land as soon as they are measured and the fix lands in its
own pull request, like an S6 row before its code (10 10.5).
`REQ-1029` `tools/ci acceptance` fails while a `default-overturned` result has no
`resolvedBy`. `REQ-1030` A `default-kept` result settles its open question once
each host it needs has one; an ADR follows the rule of the index's
[Open questions](../spec.md#open-questions).

`REQ-1031` A result is a fact about the versions its host records. A pin change
runs again the probes that read that pin: those of block A before the
change merges, and those of block B, sandbox-side checks included, at
the next candidate:

| Pin | Probes run again |
|---|---|
| the workload digest | A9 (the descriptor-provisioned rule and the deny), A12, A15, A19, C3, C5, and B2's S5 part |
| the kit frontend | A11, A18, A19's kit part, and the conformance subset of A12, with their conformance fixtures (I28) |
| herdr | A13 |
| mise | C2, B4 |
| the sbx floor | block A |

`REQ-1032` `tools/ci probes` fails when a committed block A result of a probe this
table lists records, under any key of `host.upstream` (11.3), a version
or digest other than that pin's current one; it compares each key on
its own.

`REQ-1033` `tools/ci acceptance` fails when a block A result's `sbxVersion` is
outside the tested window of
[10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract), when
A4, A5, A9 or A15 has no result at the window's newest version, or when a
recorded `upstream` key differs from its current pin; a floor or pin
bump therefore runs the probes that read it again before the next
release, and the new result settles their open questions again.
`REQ-1034` `acceptance --pre-tag` runs the `resolvedBy` check above and these
checks as two of its six
([10 10.5](10-testing-style.md#105-acceptance-evidence)). Git history
keeps the results it replaces.

`tools/ci probes` checks the results that are committed and says
nothing about the ones that are not: block B cannot run before a
release exists. `REQ-1035` That every probe id of this page has a result, from
both hosts when the probe is arch-sensitive,
is checked by `tools/ci acceptance`
([10 10.5](10-testing-style.md#105-acceptance-evidence)), which joins
`all` once the plan's last task has landed. Until then the tables of
this page are the reminder, and a probe nobody ran is noticed by
reading them.

`REQ-1036` `tools/ci probes` also flags an `sbxenv.yaml` golden whose normalized
hash ([03 3.3](03-formats.md#33-rendered-sbxenvyaml-sbx-env-file-schemaversion-1))
moved since probe A5 recorded it, and prints a notice, which does not
fail the check, for a golden with no recorded hash yet: it needs a
probe before it counts as confirmed.
