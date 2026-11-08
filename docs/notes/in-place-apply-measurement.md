# The in-place apply measurement on sbx 0.51

Reader: whoever decides T210 item 2 or re-records A15 with the harness.
Type: reference.

This note records two hand measurements, both run by the maintainer on
a macOS 27.2 Apple-silicon host with sbx v0.51.0 (7d55286d). Dates and
times are UTC.

- Sitting 1, 2026-11-07, 21:31 to 23:03: part C of the host measurement
  protocol, on the throwaway sandbox `rj-probe-a`, created with
  `sbx env run -d --clone <dir>`.
- Sitting 2, 2026-11-08, 01:06 to 01:13: protocol 2, on the throwaway
  sandboxes `rj2-a`, `rj2-b`, `rj2-k` and the never-created `rj2-ghost`.

Quotes replace the host's home directory with `~`, an env directory
with `<dir>`, and the em dash sbx prints
before a status word with a plain dash. Nothing in this note is a product rule; the spec pages
cite it as the source of what they record.

## What was measured

In sitting 1, for each case the sandbox went back to the base env file,
a marker was written inside, the env file was changed in one kind, and
these ran, the plan prompt answered `y`:

```sh
sbx env plan <dir>
sbx env run -d <dir>
```

The read-backs after each case were `sbx ls --json`,
`sbx inspect --json`, `sbx policy ls <sandbox> --wide`, `sbx secret ls`,
and an inside script: the markers, `git status`, the kit version files,
the login files, a GitHub probe that answers 401 Bad credentials when
the probe's dummy secret is injected, the mounts, `df`, the memory
directories, the `cmd` log and the processes.

## The result in one line

On sbx 0.51 no change kind applied on either state. Every accepted
`sbx env run -d <dir>` exited 0, kept the instance and left the change
unapplied, and sbx's own plan said so: `applies on the next create`.
Sitting 2 found the same with `-f` for a removed secrets binding: the
binding is `left in place`.

## Running sandbox (sitting 1)

"Plan shown" quotes the line sbx printed for the changed item.

| kind | exit | plan shown | instance kept | applied as sbx reports | inside | verdict |
|---|---|---|---|---|---|---|
| kit added | 0 | `waits for the next create` | yes | no | no | unapplied |
| kit content changed | 0 | no kit line, `already approved` | yes | no (same image digest) | no (`v1`) | unapplied, change not detected |
| kit removed | 0 | `applies on the next create` | yes | no: kit and its rule stay | file stays | unapplied |
| `ports` entry | 1 | parse error | yes | - | - | not provoked (invalid probe entry) |
| `rootSize` | 0 | `applies on the next create` | yes | no | no (20G) | unapplied |
| `dockerSize` | 0 | `applies on the next create` | yes | no | not read | unapplied as sbx reports |
| `clonedWorkspaceSize` | 0 | `applies on the next create` | yes | no | no (49G) | unapplied |
| repo added | 0 | `mounted by the next create` | yes | no | no | unapplied |
| repo removed | 0 | `mounted by the next create` | yes | no: mount stays | mount stays rw | unapplied |
| workspace `target` | 0 | `mounted by the next create` | yes | no | not read | unapplied as sbx reports |
| workspace `readOnly` | 0 | `mounted by the next create` | yes | no | stays rw | unapplied |
| secrets binding added | 0 | `waits for the next create` | yes | no | not injected | unapplied |
| secrets binding removed | 0 | `no longer declared, never applied` | yes | - | - | not decidable here (binding never in force); see sitting 2 |
| `agent` | 1 | warning that the sandbox is not the one the file describes, then an agent and kit mismatch error | yes | - | - | refused (probe pairing) |
| workload | 0 | the same warning, workload `applies on the next create` | yes | no | no | unapplied |

## Stopped sandbox (sitting 1)

Each accepted run started the sandbox (`✓ sandbox rj-probe-a -
started`) with the same id, creation time and markers, and with the
configuration it was created with.

| kind | exit | plan shown | instance kept | applied as sbx reports | inside | verdict |
|---|---|---|---|---|---|---|
| kit added | 0 | `waits for the next create` | yes, started | no | no | unapplied |
| kit content changed | 0 | no kit line | yes, started | no | no | unapplied |
| kit removed | 0 | `applies on the next create` | yes, started | no | no | unapplied |
| `ports` entry | 1 | parse error | yes, not started | - | - | not provoked |
| `rootSize` | 0 | `applies on the next create` | yes, started | no | no | unapplied |
| `dockerSize` | 0 | `applies on the next create` | yes, started | no | not read | unapplied as sbx reports |
| `clonedWorkspaceSize` | 0 | `applies on the next create` | yes, started | no | no | unapplied |
| repo added | 0 | `mounted by the next create` | yes, started | no | no | unapplied |
| repo removed | 1 | plan declined by an empty answer | yes, not started | - | - | not measured |
| workspace `target` | 0 | `mounted by the next create` | yes, started | no | no | unapplied |
| workspace `readOnly` | 0 | `mounted by the next create` | yes, started | no | no | unapplied |
| secrets binding added | 0 | `waits for the next create` | yes, started | no | not injected | unapplied |
| secrets binding removed | 0 | `no longer declared, never applied` | yes, started | - | - | not decidable here |
| `agent` | 1 | the agent and kit mismatch error | yes, not started | - | - | refused (probe pairing) |
| workload | 0 | `applies on the next create` | yes, started | no | no | unapplied |

## A removed secrets binding (sitting 2)

`rj2-b` was created with an env file that binds `secrets.openai` with a
`command:` source. sbx provisioned the secret at sandbox scope
(`Scope: rj2-b`); `sbx secret ls --sandbox rj2-b` listed it, and no
global line for it appeared at any point. The binding did not reach
`rj2-a`. Then the binding was dropped from the file:

```text
$ sbx env run -d -f <dir>
  openai - no longer declared, left in place
  1 to forget
[exit 0]
```

The secret stayed listed and stayed injected, while running and after
`sbx stop` and a start. A second identical run printed `1 to forget`
again. Both runs carried `-f`; that a run without it, after a `y`, does
the same is inferred from the plan's wording `left in place`. Whether a
binding entry is written to sbx's credentials file that
`sbx secret ls -g` does not list was not read.

## The other cases

- **Exec on a stopped sandbox** (sitting 1). `sbx env exec <dir> --
  true` with a changed env file (a kit added, a repo added): exit 0, the
  sandbox started, no prompt, and the mounts, policy rules, marker and
  id stayed as created.
- **Exec on a running sandbox.** `sbx env exec <dir> -- true` with a
  changed env file (sitting 1): exit 0, no prompt, nothing changed. In
  sitting 2, `sbx env exec -it <dir>` with a kit added to the file, run
  twice: no plan, no prompt, exit 0, nothing applied, and `cmd` did not
  run. Only the kit-added kind was tried with `-it`.
- **A kit added to a present sandbox** (sitting 2). sbx plans it as
  `waits for the next create`; after `y` and exit 0 the instance is kept
  (the same id) and the kit is not applied. Dropped from the file again,
  it becomes `no longer declared, never applied` and `1 to forget`.
- **A declined plan.** Not measured: in both sittings the deliberate `n`
  answers were typed `y`. In sitting 2 the stopped case was not prompted
  at all, most likely because sbx remembered the approval given 6 s
  earlier; a start path that never prompts for `waits for the next
  create` items is not excluded. Two accidental answers of sitting 1
  stand in: `yy` on a running sandbox and an empty answer on a stopped
  one. Both exited 1 with `error: the plan for ... was not approved` and
  changed nothing; the stopped sandbox stayed stopped.
- **A declined create.** Not measured (sitting 2, the same typing slip).
  REQ-1212 of 01 1.6 stays an unmeasured row.
- **The same changed file applied twice** (sitting 1): the second run
  asked nothing, exited 0 and changed nothing. Nothing applied either
  time.
- **A killed apply.** Not provoked in sitting 1: the run ended with exit
  0 after 4 s. In sitting 2 a SIGKILL of a create's process group after
  kit assembly exited 137 and left `rj2-k` `stopped` with a partial
  `sbx inspect` record; it was gone by the read 30 s later, and the
  rerun created a new sandbox with a new id and no leftover prompt. That
  sbx's daemon rolled it back is an inference from one sample.
- **The plan prompt under `--no-input`** (both sittings): exit 1 with
  `changes things outside the sandbox that have not been approved;
  approve them by running this in a terminal, or approve this invocation
  with --force (-f)`, and sbx's hint `try: <original command> --force`;
  nothing changed, and a stopped sandbox stayed stopped. On an equal,
  approved file, `sbx env run --no-input <dir>` attaches.
- **Clone mode from the file** (sitting 2). An env file with
  `workspace:` / `path:` / `clone: true`, created with no `--clone`
  flag, gives clone mode: a file written inside does not appear on the
  host, the source is mounted read-only at `/run/sandbox/source`, and
  the next plan shows `0 to change`. In sitting 1 every plan listed
  `clone: true -> null` against a sandbox created with `--clone` and no
  `clone` key in the file, which explains that drift. sbx's removal
  warning still says `(--clone)` for a clone set in the file.
- **The attached `sbx env run <dir>`.** In sitting 1 it exited 1 with
  `error: unknown option '-lc'` before attaching, because `cmd` alone
  kept the workload's entrypoint. In sitting 2, with both `entrypoint`
  and `cmd` in the file, it attached and ran them with a TTY, and
  changed nothing in `sbx policy ls` or `sbx inspect`. The first
  attached run after a create with `-f` prompted
  `Approve this plan? (y/N)` for the five pending items plus
  `+ entrypoint` and `+ cmd`.
- **A detached create or start runs no `entrypoint` or `cmd`** (both
  sittings): the `cmd` log got lines only inside the attached runs, PID
  1 stayed sbx's `sleep infinity` wrapper, and the id was unchanged on a
  start.
- **An exit 0 is not evidence of an apply**: the workload case of
  sitting 1 exited 0 after sbx itself said the sandbox is not the one
  the file describes.

## Removals and their exit statuses (sitting 2)

- `sbx policy rm network --sandbox rj2-a --resource example.test
  --force --no-input` removed exactly that sandbox's rule, with no
  prompt; the identical rule on `rj2-b` stayed. `policy rm network`
  offers `-f, --force` ("Skip confirmation prompts") and `--id`;
  `policy allow network` has no `--force`. In sitting 1 the same call
  without `--force` failed under `--no-input` with a hint to pass
  `--force`.
- `sbx secret rm <service> --sandbox <sandbox> --force --no-input`
  removed the named secret with no prompt.
- `sbx env rm <dir> --force --no-input` printed a destroy plan and a
  clone warning, then removed the sandbox with no prompt: exit 0. On a
  sandbox that does not exist it printed `error: sandbox '<name>' not
  found` and exited **0**, with the flags and without them.
- `sbx env rm --force --no-input --prune-bindings` removed the sandbox
  and printed `Removed secrets scoped to rj2-b`, with the file declaring
  the secret again; whether the flag is what removed it was not
  measured.

## The offer to create a sandbox (sitting 2)

`sbx env exec <dir> -- true` on a sandbox that never existed:

- on a TTY: `Sandbox "rj2-ghost" does not exist. Create it? (y/N)`; a
  decline printed `error: operation cancelled by user` and exited 125,
  and nothing was created;
- with `--no-input`: no prompt, `error: sandbox 'rj2-ghost' not found
  for this environment`, exit 125.

Both shapes exit 125; only stderr tells them apart. A sandbox lost
during an exec was not provoked.

## SSH agent forwarding (both sittings)

With `SSH_AUTH_SOCK` unset in the calling shell, `rj2-a` listed the key
of one host agent and `rj2-b` the key of another in the same second, so
forwarding is per sandbox. Sitting 1 showed that the last client to
call into a sandbox wins over the one that created it. Withholding
`SSH_AUTH_SOCK` does not stop forwarding on exec; on a start of a
sandbox whose kit has `ssh-agent@1`, `sbx env run -d` refuses:
`ssh-agent@1 is not available ... needs a host SSH agent`, exit 1. This
holds with sbx's `ssh.agentSocketPath` setting empty, which was not
read in either sitting.

## The workload's Docker engine (sitting 1, part D)

On `docker.io/sbx/claude`: the agent user is in group 999 `docker`, the
socket is `srw-rw---- root docker`, dockerd runs as uid 0 with the full
capability set, and `docker run hello-world` ran as the agent. On the
current pin `docker.io/docker/sbx-kit-claude` dockerd and containerd
also run as root, with the agent in the `sudo` and `docker` groups.
`sudo -n` works on both workloads.

A per-sandbox `sbx policy deny network` on `deb.debian.org:80` beat the
provisioned allow and held beside a portless allow of the same host; it
did not survive a recreate under the same name.

## Outcome for T210 item 2

- The stopped-start reading holds for the two kinds measured with
  `sbx env exec`: a kit added and a repo added. Whether that is enough
  to lift the refusals of REQ-1376 and REQ-1387 is the maintainer's
  decision.
- Nothing passes the apply, so the in-place apply row of Deferred
  decisions stays deferred, with this note as its evidence.
- On a running sandbox, `sbx env exec <dir> -- true` and `sbx env exec
  -it <dir>` did not read the changed file for the kinds tried; `-it`
  was tried for a kit added only, so REQ-1388 stays open for the other
  kinds.

## Not measured

- The `ports` entry on both states. Sitting 1 refused a string; sitting
  2 refused the mapping `{host_ip, host_port, sandbox_port, protocol}`
  guessed from `sbx ls --json` with `unknown field`, exit 1. T037's
  shape `{sandbox, host, protocol, hostIP}` was not tried, and no
  accepted form is known.
- `agent` alone, with an agent name that matches the workload kit.
- A repo removed on a stopped sandbox (the plan was declined).
- The inside read of a workspace `target` change on a running sandbox.
- `dockerSize` inside the sandbox (the script reads no Docker volume).
- A deliberately declined plan, on both states, and a declined create.
- An apply killed mid-run on a present sandbox.
- `sbx env exec -it <dir>` with a changed env file of any kind other
  than a kit added.
- Whether a create without `-f` records approval of the items other
  than the launch, which is romeu's path.
- The Docker restart of A15 and `ssh-add -L` after it (left out, since
  it reaches every sandbox on the host).
- sbx's `ssh.agentSocketPath` setting.
- Intel hosts: one host only, which part C allows; REQ-0979 stays open.
