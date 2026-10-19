# Sitting M0: the kit frontend is a go

Sitting M0 ran on 2026-10-19 on the maintainer's host (macOS 27.0.1,
arm64, sbx v0.48.0, Docker Desktop with buildx v0.37.1, herdr 0.9.5). It
built a minimal local v3 mixin (`rj-probe`) with the pinned frontend
`docker/sandbox-kit:3.0.0-m.8@sha256:e6a39777...`, seven one-file
fixtures, and one sandbox from the workload by digest
(`docker.io/docker/sbx-kit-claude@sha256:c0132e89...`, tag 2.1.296),
then removed the sandbox. Verdict: go. Each fact below is `source:
sitting M0`; T113's text says sitting M2 because it was written before
the re-plan moved the go/no-go to M0.

## The facts of T113

- Descriptor file name: `<stem>.yaml` with an optional
  `<stem>.dockerfile`; a lone `kit.yaml` is accepted too, the kit then
  named after its directory. Inside the sandbox each kit is staged as
  `/usr/share/sandbox/kit/<name>/kit.yaml` and `kit.dockerfile`.
- Capability names: `com.docker.sandbox/<type>@<version>`. sbx v0.48.0
  composed `ssh-agent@1`, `lifecycle@1`, `network-policy@1`,
  `volume@1` and `privileged@1`. It does not support `host-mount@1`:
  required, `create` is refused ("unsupported capability type"); marked
  `optional: true`, it is skipped with no message.
- Strictness: an unknown top-level key fails the build ("field bogusKey
  not found in type spec.Descriptor"); a YAML anchor is accepted and
  resolved; an unknown required capability type builds and inspects
  but `create` refuses it.
- Where install steps run: inside the sandbox (its own kernel and
  hostname, its own network namespace `172.17.0.9/31`), once at create,
  before the agent. With no `user`, an install hook runs as root. It
  sees the workspace: `WORKSPACE_DIR` names it when declared, and the
  workspace is mounted at its host path. No SSH agent is reachable in
  the install phase, and egress to an undeclared host gets 403 from the
  proxy.
- Where the recipe runs: the Dockerfile `RUN` steps run in the host's
  Docker buildkit (`buildkitsandbox`, a linuxkit kernel), with no host
  path visible.
- Startup hooks run on every start, as uid 1000, after the workload's
  own hooks; the startup dispatcher prints `ok` for a hook whatever the
  hook wrote to its error output, by its exit status alone.
- Kit args reach install and file steps: `${{ kit.args.x }}` is expanded
  into the hook's command and into a `lifecycle.files` body at create,
  and an arg with `env:` reaches the sandbox environment and every hook
  that declares the name. `julietaBin` and `signingKey` can be set this
  way.
- Hook environment: declared names plus a baseline, as the spec says,
  and also the proxy and CA variables (`HTTP_PROXY`, `HTTPS_PROXY`,
  `NO_PROXY`, their lower-case forms, `SSL_CERT_FILE`, `SSL_CERT_DIR`,
  `REQUESTS_CA_BUNDLE`, `CURL_CA_BUNDLE`, `NODE_EXTRA_CA_CERTS`), which
  the spec's baseline does not list.
- The workload descriptor as bytes by digest: the annotation
  `vnd.docker.sandbox.kit.descriptor` of the arm64 manifest of tag
  2.1.296 is 14878 bytes, sha256
  `86e9133cbd2f31279b61ec638276c26102389d7de2e044ae6166fffa310c35a2`,
  built by frontend 3.0.0-m.8. It declares
  `com.docker.sandbox/agent-interactive-sessions@1`, a type the m.8
  spec does not list among its well-known types.
- The read-only mount: an extra workspace `PATH:ro` of `sbx create` is
  mounted read-only ("Read-only file system" on write); the main
  workspace stays writable.
- The SSH agent: with `ssh-agent@1` (`unrestricted: false`, `sign:
  [git]`), the sandbox lists every key of the host agent (two), and
  signs in namespace `git` only: a signature in namespace `file` is
  refused. One key is not forwarded alone.
- herdr: the socket API reports `protocol: 22`, `schema_version: 1`.
- Host git: 2.56.0, series 2.56. The workload's git is 2.47.3 (the
  Debian package).

## What the facts change

- `sbx kit inspect` is not a source of the permission surface: it
  leaves out `ssh-agent@1`, `privileged@1` and unknown types, and names
  the workload `kind: sandbox`. Gate 2 reads the descriptor (I28), as
  the spec already says.
- A personal kit's install step runs as root and sees the workspace;
  that answers the measurement Q32 waits for (REQ-0574).
- A shared host directory cannot come from `host-mount@1` on this sbx;
  the read-only bin mount comes from the sandbox's extra workspaces.
- The one-key agent is not what forwarding gives; the namespace
  restriction is what bounds the key.
- A local v3 source kit needs a host Docker with buildx and its
  credential helper on `PATH`; `sbx kit validate` refuses a source kit
  ("no kit builder configured"), while `inspect` and `create` build it.
- The workload's `agent-interactive-sessions@1` is a type the grammar
  must model, or REQ-0573 fails `sync` closed on the default workload.

These reach the tasks that read them: T032 (conformance fixtures), T076
(descriptor and kit args), T081 (the socket fact), and Q32; their
fixtures write `source: sitting M0`.

## The git floor

T113 asks for a one-line change to `internal/gitsafe/floor.go` when the
host git differs from 2.45.4. Decided on 2026-10-19: no change. A row
for series 2.56 at 2.56.0 changes no result, since 2.56.0 is above the
initial floor, and no CVE reading backs it; the per-series rows come
from probe A1 in T027.
