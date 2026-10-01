# 6. Kits (schema v3, local builds)

Back to [index](../spec.md). Reader: implementers of `kits`, `render`
and `oci`. Type: reference with the decision record for each choice.

## 6.1 Decisions

| Topic | v1 decision | Why for us |
|---|---|---|
| Kit schema | **v3** only; v3 never mixes with v1/v2 kits, so every project uses v3 end to end | one grammar to test; the direction sbx documents for new kits |
| Frontend | every kit descriptor starts with `# syntax=docker/sandbox-kit:3.0.0-m.<N>@sha256:<digest>`; one constant, rewritten by `go run ./tools/kitpin frontend` | the v3 spec has only milestone tags; a digest pin makes a bump a reviewed diff |
| Go/no-go | probe A11 builds a minimal local v3 mixin on the host before any product kit is written; it fixes the descriptor file name and capability names | no kit is written against an unmeasured format |
| Build | **local directory sources**: on approval romeu promotes kit sources into content-addressed dirs `<name>-env/.romeu/kits/<kit>-<digest12>/`; sbx builds them at create and reuses its cache when unchanged | no registry needed in v1; a kit change applies on recreate; immutable dirs make promotion crash-safe (01 1.6) |
| Materialization | product kits from romeu's embedded copy; personal kits from the config commit's tree through `git fsck --strict` and `gitsafe.WalkTree` (I27), written with `os.Root` into the candidate | agent-authored trees never reach a live path before the gate |
| Capabilities | romeu parses every descriptor (local kits from their files, the workload from its image annotation via `internal/oci`, verified by digest at every hop) with a strict subset grammar into a normalized capability set that enters gate 2 (I28); kit-declared domains go through `egress.Split`; parsed descriptors are cached by digest in host state | a kit is a capability grant, so its capabilities are what the gate must show; romeu and sbx must read the same descriptor the same way |
| Personal kits | network, mount, volume, credential and ssh-agent capabilities are refused (sync error). The other capability types, install steps, privileges and lifecycle hooks, are not refused: a personal kit that declares one is shown in the gate 2 diff | personal kits should not widen a sandbox silently, and a refusal we state is one the code makes |
| Signing | **none in v1** (local v3 kits cannot be signed); trust = romeu release + approved config commits | honest about the limit; the gate hashes kit content |
| Kit sets | not used (they need published refs) | - |

## 6.2 Workload choice

| Option | For | Against | Decision |
|---|---|---|---|
| **`docker.io/docker/sbx-kit-claude` pinned by digest** | maintained upstream with Claude Code releases; multi-arch; carries Go, gh, jq, docker CLI; per-name volumes reattach on recreate | unsigned today; Debian base; built outside our review | **default (Q2)**, with `workloadRepositories` allowlist in host settings and its capabilities in gate 2 |
| Own workload | full control; signable once published | we would track Claude Code releases ourselves | rejected for v1 |

`julieta pin workload` moves the digest, verifying the manifest list
covers linux/amd64 and linux/arm64; the change is gated and
recreate-class.

## 6.3 julieta delivery

The julieta binary is **not** kit content, so a romeu release does not
change any kit digest and does not force a recreate of every project.

1. On promotion romeu writes `julieta-linux-amd64`, `julieta-linux-arm64`
   and `SHA256SUMS` into `<name>-env/.romeu/bin/` (from its embedded,
   release-built copies) and records each binary's sha256 in
   `render.json`.
2. The env file mounts `./.romeu/bin` read-only (a fixed mount, so its
   content can change without widening or recreate).
3. romeu always invokes julieta by absolute path:
   `<abs .romeu/bin>/julieta-linux-<GOARCH>`, where `GOARCH` is romeu's
   own (a sandbox runs the host's architecture, Q21). The names use Go's
   architecture names, so no `uname -m` mapping exists anywhere.
4. Before the first call of a session romeu runs the **compatibility
   check** (`version --json`): julieta's protocol must be one romeu
   accepts, its sha256 must equal `render.json`, and julieta must report
   its own directory as not writable. Any failure exits 2 before
   `layout up` (I30). The check proves compatibility, not integrity: it
   runs the binary it checks. Integrity comes from the read-only mount
   and the host-side drift check of `.romeu/bin`.
5. `julieta setup` points `$HOME/.local/bin/julieta` at its own absolute
   path, so hooks, panes and the agent find `julieta` on `PATH`. No kit
   build step references the per-project path, which a kit cannot know
   at build time.

Probe A3 confirms the read-only mount is enforced, visible at the same
path, and that root inside the sandbox cannot make it writable with
`mount -o remount,rw`.

## 6.4 Product kits

| Kit | Kind | Contents | Language |
|---|---|---|---|
| `julieta` | mixin | `mise` and `herdr` downloaded at build, version + per-arch sha256 pinned (`tools/kitpin`); `~/.zshenv` and `BASH_ENV` add `$HOME/.local/bin` and the mise shims dir; herdr config: `[update] version_check=false, manifest_check=false`, and an explicit tmux-style prefix key binding (`ctrl+b`) so muscle memory carries over; network capability (install phase): `github.com`, `objects.githubusercontent.com` | descriptor + sh install steps (<= 5 lines each: download, `sha256sum -c`, install) |
| `julieta-claude` | mixin | the only agent-aware kit: installs `skills/julieta` and `skills/handoff` into `~/.claude/skills/`; Claude Code hooks `SessionStart` -> `julieta handoff show --hook` and `SessionEnd` -> `julieta handoff write --facts` (invoked through `$HOME/.local/bin/julieta`); makes the agent's own memory dir non-writable (and sets the disabling setting if the pinned version has one, Q8) | descriptor + files |
| `os-base` | mixin | apt delta over the workload's Debian base, kept minimal (the list is the workload's missing set, measured in probe A12); arg `tz` (default `Etc/UTC`) written as `TZ` | descriptor + sh |
| `git-ssh-sign` | mixin | ssh-agent capability used for git signing only, from the dedicated host signing socket (`signing.agentSocket`); git config `gpg.format=ssh`, `commit.gpgsign=true`, `user.signingkey` from arg `signingKey` (a public key); `gpg.ssh.allowedSignersFile` generated from the same arg; opt-in per project | descriptor + files; **ask-first surface** |

**Signing rule (decided by the maintainer).** `romeu sync` refuses to
render `git-ssh-sign` (exit 2, `RJ-204 signing-socket`) unless
`signing.agentSocket` is set, reachable, and holds exactly one key, and
that key equals the kit's `signingKey` arg. The full host agent is never
forwarded. `internal/signing` checks this with the ssh-agent protocol's
identity listing (no dependency, no subprocess); the same check runs in
`doctor` and in the preflight of that project's **P** commands (01 1.5). How
sbx is made to forward that socket is Q19.

**herdr.** herdr is the terminal multiplexer that runs inside the
sandbox and that julieta renders a run layout into (01 1.1). Its
upstream is [github.com/herdrdev/herdr](https://github.com/herdrdev/herdr).
Measured there on 2026-10-01, over its last three releases: each
publishes one binary per platform, with no checksum file and no
signature; each is marked immutable; the release API returns a sha256
digest for each asset; and the release workflow has no signing or
build-attestation step. So the pin is a version and a sha256 per
architecture, and `go run ./tools/kitpin herdr <version>` writes both:
it takes each hash from the release API's digest and refuses a release
that is not marked immutable. The kit's install step checks the
download against that hash. What the hash does not prove is a residual
risk ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)).
mise's pin, which has a signed checksum file, is in
[07 7.4](07-mise-egress.md#74-mise-bootstrap-and-its-own-egress).

The `julieta` mixin carries nothing agent-specific, so a future
`julieta-<agent>` mixin is all another agent needs.

## 6.5 Where capabilities live

| Capability | Home | Reason |
|---|---|---|
| agent install and agent egress | the workload | maintained upstream with the agent |
| developer tools (languages, linters, CLIs) | mise, per repo (`mise.toml`/`mise.lock`) | one lockfile for sandbox, CI and host; apt versions drift between images |
| language and registry egress | derived by romeu from locks and the catalog | egress is derived, never hand-written in kits |
| git commit signing | `git-ssh-sign` | stricter agent forwarding; the key stays an argument |
| git hooks | `julieta hooks` (sandbox only) | hooks never run on the host |
| memory and handoff | `julieta memory`, `julieta handoff` | one agent-neutral store |
| personal agent context (instructions, plugins) | a personal kit in the config repo | personal, not product |
| OS packages tied to libc/root (git, curl, ca-certificates, sudo) | the workload base; gaps in `os-base` | rarely change |

## 6.6 Publishing path (designed, not built in v1)

| Step | Design |
|---|---|
| Registry | Docker Hub (the default `kit.allowedSources`), one public repository per kit: `docker.io/<namespace>/romeu-kit-<id>` |
| Tag | the kit version (`vX.Y.Z`); consumers pin `:<tag>@sha256:<digest>` |
| Build | `tools/release` builds each kit as a multi-arch OCI image with the pinned frontend |
| Sign | `sbx kit sign` on the published image; consumers set `kit.requireSignature true` |
| Consumer switch | `product: <id>` resolves to the published reference when host settings set `kits.source: published`; gate 2 records the digest instead of a content hash |
| Trigger | mandatory when the product is promoted for public use (maintainer decision) |

Docker Hub free plan limits relevant later: unlimited public repos, pull
rate 200/6h authenticated and 100/6h anonymous, images marked stale after
6 months without pulls.
