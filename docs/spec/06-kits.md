# 6. Kits (schema v3, local builds)

Back to [index](../spec.md). Reader: implementers of `kits`, `render`
and `oci`. Type: reference with the decision record for each choice.

## 6.1 Decisions

| Topic | v1 decision | Why for us |
|---|---|---|
| Kit schema | **v3** only; v3 never mixes with v1/v2 kits, so every project uses v3 end to end | one grammar to test; the direction sbx documents for new kits |
| Frontend | every kit descriptor starts with `# syntax=docker/sandbox-kit:3.0.0-m.<N>@sha256:<digest>`; one pin in `kits/pins.yaml`, rewritten by `go run ./tools/kitpin frontend`. A frontend pin change runs again the probes that [11 11.4](11-host-probes.md#114-probe-lifecycle) lists for it | the v3 spec has only milestone tags; a digest pin makes a bump a reviewed diff |
| Go/no-go | probe A11 builds a minimal local v3 kit (`kind: mixin`) on the host before any product kit is written; it fixes the descriptor file name and capability names. On a no-go, kit work stops and an ADR records the A11 result and the options, each with its trigger: wait for the next frontend milestone, or reconsider v2 kits or a published kit of our own (Q7 reopens) | no kit is written against an unmeasured format |
| Build | **local directory sources**: on approval romeu promotes kit sources into content-addressed dirs `<name>-env/.romeu/kits/<kit>-<digest12>/`; sbx builds them at create and reuses its cache when unchanged. Where sbx runs a local kit's install steps and lifecycle hooks, and what they reach (network, host paths, uid), is measured by A11 and A14 ([11 11.1](11-host-probes.md#111-block-a---sbx-and-runtime-facts-first-in-parallel-with-the-first-build-layer)) | no registry needed in v1; a kit change applies on recreate; immutable dirs make promotion crash-safe (01 1.6) |
| Materialization | product kits from romeu's embedded copy; personal kits from the config commit's tree through `git fsck --strict` and `gitsafe.WalkTree` (I27), written with `os.Root` into the candidate | agent-authored trees never reach a live path before the gate |
| Capabilities | romeu parses every descriptor (local kits from their files, the workload from its image annotation via `internal/oci`, verified by digest at every hop) with a strict subset grammar into a normalized capability set that enters gate 2 (I28); one package, `internal/kitcap`, owns that grammar and the normalized set, and `oci` and `render` call it; kit-declared domains go through `egress.Split`; parsed descriptors are cached by digest in host state. The conformance fixtures of I28 record the sbx version and frontend digest they were made with, and a frontend pin change, or an sbx version entering the tested window, runs the A11 and A12 conformance subset again, named in the PR's Evidence. A capability type the grammar does not model fails `sync` closed, a row of [Deferred decisions](../spec.md#in-the-product) | a kit is a capability grant, so its capabilities are what the gate must show; romeu and sbx must read the same descriptor the same way. For the workload, a digest change alone does not show a reviewer that the new image added a mount, a credential or a network domain; the capability set does, and the gated egress of the workload's declared domains depends on it |
| Personal kits | network, mount, volume, credential and ssh-agent capabilities are refused (sync error). The other capability types, install steps, privileges and lifecycle hooks, are not refused: a personal kit that declares one is shown in the gate 2 diff. Whether install steps and lifecycle hooks are refused too is decided once A14 reports where they run (Q32) | personal kits should not widen a sandbox silently, and a refusal we state is one the code makes |
| Signing | **none in v1** (local v3 kits cannot be signed); trust = romeu release + approved config commits. Publishing and signing product kits is a row of [Deferred decisions](../spec.md#in-the-product); the one constraint it puts on today's design is that gate 2 can record a digest in place of a content hash | honest about the limit; the gate hashes kit content |
| Kit sets | not used (they need published refs) | - |

## 6.2 Workload choice

| Option | For | Against | Decision |
|---|---|---|---|
| **`docker.io/docker/sbx-kit-claude` pinned by digest** | maintained upstream with Claude Code releases; multi-arch; carries Go, gh, jq, docker CLI; agent state in volumes of its own (below) | unsigned today; Debian base; built outside our review | **default (Q2)**, with `workloadRepositories` allowlist in host settings and its capabilities in gate 2 |
| Own workload | full control; signable once published | we would track Claude Code releases ourselves | rejected for v1 |

`julieta pin workload` moves a project spec's digest, verifying the
manifest list covers linux/amd64 and linux/arm64; that check is why the
command exists, since a hand-run inspect skips it. The change is gated
and recreate-class. The product's own workload pin, the one its tests
and examples use, is the workload entry of `kits/pins.yaml`, written by
`go run ./tools/kitpin workload <digest>`
([02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)). A
workload pin change runs again the probes that
[11 11.4](11-host-probes.md#114-probe-lifecycle) lists for it.

The workload's volumes, which A12 lists with their mount paths, are
outside salvage ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)).
What a recreate keeps and loses of them, the agent's login included, is
the A12 record; `rm` and `recreate` print that list before salvage
starts, and `retire` says whether they are left
([04 4.2](04-cli.md#42-romeu-host)).

## 6.3 julieta delivery

The julieta binary is **not** kit content, so a romeu release does not
change any kit digest and does not force a recreate of every project.

1. On promotion romeu writes `julieta-linux-<GOARCH>` (romeu's own
   `GOARCH`) and `SHA256SUMS` into `<name>-env/.romeu/bin/` (from its
   embedded, release-built copy) and records the binary's sha256 in
   `render.json`.
2. The env file mounts `./.romeu/bin` read-only (a fixed mount, so its
   content can change without widening or recreate).
3. romeu always invokes julieta by absolute path:
   `<abs .romeu/bin>/julieta-linux-<GOARCH>`, where `GOARCH` is romeu's
   own (a sandbox runs the host's architecture, Q21, which A4 may
   reopen). The names use Go's architecture names, so no `uname -m`
   mapping exists anywhere.
4. Before every command that execs julieta (`run`, `stop`, `salvage`,
   `pull`, and `status` for its read) romeu runs the **compatibility
   check** (`version --json`): julieta's protocol must be one romeu
   accepts, N or N-1 where N is the one it embeds (the window of the
   manifest, [03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)),
   its sha256 must equal `render.json`, and julieta must report its own
   directory as not writable. Any failure exits 2 before `layout up`
   (I30), with `julieta-protocol`, `julieta-digest` or
   `julieta-bin-writable`. The check proves compatibility, not
   integrity: it runs the binary it checks. Integrity comes from the
   read-only mount and the host-side drift check of `.romeu/bin`.
5. `julieta setup` points `$HOME/.local/bin/julieta` at its own absolute
   path, so hooks, panes and the agent find `julieta` on `PATH`. No kit
   build step references the per-project path, which a kit cannot know
   at build time.
6. julieta embeds the digest of the skills and hook lines the matching
   `julieta-claude` kit installs; `julieta setup` compares it with the
   installed ones and reports `skills-stale`, which SessionStart prints
   like "layout stale" ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)).
   The skills are kit content and change only at recreate, while
   julieta changes at every release.

Probe A3 confirms the read-only mount is enforced, visible at the same
path, and that root inside the sandbox cannot make it writable with
`mount -o remount,rw`.

## 6.4 Product kits

| Kit | sbx kind | Contents | Language |
|---|---|---|---|
| `julieta` | `mixin` | `mise` and `herdr` downloaded at build, version + per-arch sha256 pinned (`tools/kitpin`); `~/.zshenv` and `BASH_ENV` add `$HOME/.local/bin` and the mise shims dir; herdr config: `[update] version_check=false, manifest_check=false`, and an explicit tmux-style prefix key binding (`ctrl+b`), chosen on purpose: inside a host tmux, press the prefix twice, or override it in a personal kit's herdr config; network capability (install phase): `github.com`, `objects.githubusercontent.com` | descriptor + sh install steps (<= 5 lines each: download, `sha256sum -c`, install) |
| `julieta-claude` | `mixin` | the only agent-aware kit: installs `skills/julieta` and `skills/handoff` into `~/.claude/skills/`; Claude Code hooks `SessionStart`, for every source the pinned version fires (startup, resume, clear, compact), -> `julieta handoff show --hook` and `SessionEnd` -> `julieta handoff write --facts`, each invoking julieta by the absolute path of the read-only `.romeu/bin` mount, which romeu renders as a reserved kit arg, `julietaBin` (a spec that sets it is a sync error), written into the hook command through `internal/shquote`, so a hook works before `julieta setup` has made the link of [02 2.5](02-layouts.md#25-in-sandbox-paths) ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)); a PreToolUse hook that refuses `gh pr merge`, a push of a `v*` tag and `gh api` calls to rulesets and repository settings, a guard against mistakes and not against an agent that edits its own hook ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)), whose refusal text says the action is the maintainer's and names the route (an issue, or the handoff), asserted by a container e2e case per refusal, and by the case "PreToolUse with the `.romeu/bin` mount absent refuses `gh pr merge`" (it fails closed, [08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)); makes the agent's own memory dir non-writable (and sets the disabling setting if the pinned version has one, Q8) | descriptor + files |
| `os-base` | `mixin` | exists only if A12 reports packages the workload lacks, which it then carries; otherwise the `julieta` kit sets `TZ` (arg `tz`, default `Etc/UTC`) and `os-base` does not exist. Its apt delta is not pinned: a residual risk of [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1), with pinning a row of [Deferred decisions](../spec.md#in-the-product) | descriptor + sh |
| `git-ssh-sign` | `mixin` | ssh-agent capability used for git signing only, from the dedicated host signing socket (`signing.agentSocket`); git config `gpg.format=ssh`, `commit.gpgsign=true`, `user.signingkey` from arg `signingKey` (a public key); `gpg.ssh.allowedSignersFile` generated from the same arg; opt-in per project, needed now because the maintainer signs every commit and will not run an agent that cannot ([00 0.5](00-scope.md#05-scope-justification)), so other adopters skip it | descriptor + files; **ask-first surface** |

`julieta-claude` replaces the agent's built-in memory with
`julieta memory` on purpose
([08 8.1](08-memory-handoff-salvage.md#how-julieta-replaces-the-agents-built-in-memory)).

**Signing rule (decided in round 2, op-signing; I34).** `romeu sync` refuses
to render `git-ssh-sign` (exit 2, `RJ-204 signing-socket`) unless
`signing.agentSocket` is set, reachable, and holds exactly one key, and
that key equals the kit's `signingKey` arg. The full host agent is never
forwarded; the rule governs the sandboxes romeu renders.
`internal/signing` checks this with the ssh-agent protocol's identity
listing (no dependency, no subprocess); the same check runs in `doctor`
and in the preflight of that project's **P** commands (01 1.5). How sbx
is made to forward that socket is Q19. `signingKey` is registered on
GitHub as a signing key and never as an authentication key, and it is
added to the dedicated agent with `ssh-add -c`, so each use asks for
confirmation (J1 step 5).

**herdr.** herdr is the terminal multiplexer that runs inside the
sandbox and that julieta renders a run layout into (01 1.1). Its
upstream is [github.com/herdrdev/herdr](https://github.com/herdrdev/herdr).
Measured there on 2026-10-01, over its last three releases: each
publishes one binary per platform, with no checksum file and no
signature; each is marked immutable; the release API returns a sha256
digest for each asset; and the release workflow has no signing or
build-attestation step. So the pin is a version and a sha256 per
architecture, and `go run ./tools/kitpin herdr <version>` writes both:
it takes each hash from the release API's digest, without downloading
the binary, and refuses a release that is not marked immutable, or an
asset whose digest field is missing or not a sha256, naming the field
(a fixture without the field tests it). The
kit's install step checks the download against that hash with
`sha256sum -c`, which is therefore the check that the digest matches
the bytes; it fails closed. What the hash does not prove is a residual
risk ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)).
herdr is pre-1.0 and kept by one upstream; a second run-layout
renderer is a row of [Deferred decisions](../spec.md#in-the-product),
which is why the run layout format names no multiplexer.
mise's pin, which has a signed checksum file, is in
[07 7.4](07-mise-egress.md#74-mise-bootstrap-and-its-own-egress).

The `julieta` kit carries nothing agent-specific, so a future
`julieta-<agent>` kit is all another agent needs.

## 6.5 Where capabilities live

| Capability | Home | Reason |
|---|---|---|
| agent install and agent egress | the workload | maintained upstream with the agent |
| developer tools (languages, linters, CLIs) | mise, per repo (`mise.toml`/`mise.lock`) | one lockfile for sandbox, CI and host; apt versions drift between images |
| language and registry egress | derived by romeu from locks and the catalog | egress is derived, never hand-written in kits |
| git commit signing | `git-ssh-sign` | stricter agent forwarding; the key stays an argument |
| git hooks | `julieta hooks` (sandbox only) | hooks never run on the host |
| memory and handoff | `julieta memory`, `julieta handoff` | one agent-neutral store |
| personal agent context (instructions, plugins) | a personal kit in the config repo; it may add to the agent's settings and never removes `julieta-claude`'s hooks, which `julieta status` reports if it does (04 4.3) | personal, not product |
| OS packages tied to libc/root (git, curl, ca-certificates, sudo) | the workload base; gaps, if A12 finds any, in `os-base` | rarely change |
