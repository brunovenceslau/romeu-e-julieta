# 3. File formats

Back to [index](../spec.md). Reader: implementers of `spec`, `render`,
`state`, `memstore` and `handoff`. Type: reference.

Every format has a version field, strict
decoding (unknown keys are errors), a JSON Schema in `schemas/`, and
examples in `examples/` validated in CI.

Validation rules live as data in `internal/spec/rules.go`: each rule
names a field path, its constraint (pattern, length, enum, reserved
values, uniqueness scope), a message and an error id. From the Go types
and those rules, `go generate` derives the validators, the rule table in
the reference docs and the JSON Schemas (`tools/schemagen`, an in-repo
reflect-based generator; no dependency), so types, validators, docs and
schemas cannot drift. The tables below are the v1 design input to
`rules.go` and the struct tags; in the product they are generated.

## 3.1 Name and path rules (shared)

| Field | Rule |
|---|---|
| project `name` | `^[a-z][a-z0-9-]{0,38}[a-z0-9]$`; unique across the config repo; equals the file stem (satisfies sbx's <= 63 chars, no trailing `-` or `.`) |
| repo `dir` | `^[a-z0-9][a-z0-9._-]{0,62}$`, not `.` or `..`, no `/`, not `memory`, `review`, `.romeu`, `sbxenv.yaml`, `.git` (compared case-insensitively); unique within the project, also case-insensitively |
| repo `url` | `https://<host>/<owner>/<repo>[.git]` only; `<host>` must be a host-settings `gitHosts` entry; a URL may appear in at most one project |
| repo `ref` | a branch name valid under `git check-ref-format --branch` rules, implemented in Go |
| kit id | `^[a-z0-9][a-z0-9-]{0,62}$` |
| secret name | `^[a-z0-9][a-z0-9_-]{0,62}$` |
| run layout pane id / tab label | `^[a-z0-9][a-z0-9_-]{0,31}$` |
| relative `cwd` in the run layout | cleaned path, no leading `/`, no `..` element |
| extra egress domain | lowercase hostname, IDNA A-labels, no wildcard, no port, no IP literal |
| workload reference | `<registry>/<repository>@sha256:<64 hex>`; `<registry>/<repository>` must be in host settings `workloadRepositories` |

## 3.2 Project spec (`projects/<name>.yaml`, schema `project.v1`)

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/<owner>/romeu-e-julieta/v1.0.0/schemas/project.v1.json
apiVersion: romeu/v1
name: shop
repos:
  - url: https://github.com/example/shop-api
    primary: true             # exactly one
    dir: shop-api             # default: URL basename without .git
    ref: main                 # branch used for egress derivation; default: origin HEAD
  - url: https://github.com/example/shop-web
workload: docker.io/docker/sbx-kit-claude@sha256:<64 hex>
agent: sbx-kit-claude         # the v3 handle the workload declares (never a v2 built-in agent)
kits:                         # order is kept; product kits come from the romeu build
  - product: julieta
  - product: julieta-claude
  - product: os-base
    args: {tz: Etc/UTC}
  - product: git-ssh-sign
    args: {signingKey: "ssh-ed25519 AAAA... "}
  - personal: my-home         # from the config repo's kits/my-home at the spec commit
sandboxOptions:
  cpus: 4
  memory: 8g
secrets: [github]             # names only; resolved via host settings "github@shop"
egress:
  extra: [api.stripe.com]     # always gated
  tools:                      # domains for a backend:tool the catalog lacks; always gated
    "http:internal-cli": [dl.example.com]
ports: []                     # e.g. ["8080:8080/tcp"]
salvage:
  excludeIgnored: [node_modules/, .venv/, target/]   # acknowledged loss on salvage
run:
  version: 1
  tabs:
    - label: main
      root:
        split: row            # row = side by side, column = stacked
        ratio: 0.6
        first: {pane: agent}
        second:
          split: column
          ratio: 0.5
          first: {pane: shell}
          second: {pane: web}
  panes:
    - id: agent
      repo: shop-api
      steps: [[claude]]
      focus: true
    - id: shell
      repo: shop-api          # no steps: login shell
    - id: web
      repo: shop-web
      cwd: app
      env: {PORT: "3000"}
      steps: [[npm, ci], [npm, run, dev]]
```

The classification is the struct tags on the spec types
(`gate:"widening"`, `apply:"recreate|live"`, 01 1.4); this table is
generated from them.

| Field | `gate` | `apply` | Notes |
|---|---|---|---|
| `repos` (incl. `ref`) | widening | recreate | `ref` decides which commit egress derives from |
| `workload`, `kits` | widening (with capabilities) | recreate | |
| `agent` | widening | recreate | |
| `ports` | widening | recreate | |
| `secrets` | widening | recreate; `live` if probe A10 confirms (Q18) | |
| `egress.extra`, `egress.tools` | widening | live | |
| `salvage.excludeIgnored` | widening | live | it defines acknowledged loss |
| `sandboxOptions` (`cpus`, `memory` only) | - | recreate | `status` reports recreate needed; stripped from `renderDigest` |
| `run` | - | live | pane commands run only inside the sandbox |

There is no field for a host command, an absolute path, a mount, or an
env var for the sandbox. A `command`, `argv` or `env` key under
`secrets` is a decode error.

### Run layout (`run`, multiplexer-neutral)

| Key | Meaning |
|---|---|
| `version` | `1` |
| `tabs[]` | `label` + `root`; a node is `{pane: <id>}` or `{split: row|column, ratio: 0.1..0.9, first, second}` (binary tree; renderers with n-ary splits nest it) |
| `panes[]` | `id`, `repo` (a project `dir`), `cwd` (relative to the repo), `env` (keys `^[A-Z_][A-Z0-9_]*$`, not `PATH`, `HOME`, `LD_*`, `BASH_ENV`, `ENV`), `steps` (list of argv lists; no shell), `focus` (at most one per tab) |

Execution rule: every step runs as `mise exec -C <pane dir> -- <argv>`;
steps run in order; each must exit 0 before the next; the last step is
exec'd; a failing step prints its exit status and drops to a login shell
in the pane dir. No steps = login shell. Every pane id appears exactly
once across all tabs.

*Why for us:* dekit and mise daemons add `ready`/`deps`, which is
supervisor territory; a layout-and-steps list is what we need, and a
later version could delegate readiness to mise daemons.

## 3.3 Rendered `sbxenv.yaml` (sbx env file, `schemaVersion: "1"`)

Produced only by `internal/render` through the YAML marshaller. The key
names below follow the documented env-file schema and the documented v3
workload pattern; the golden files are **probe-confirmed**: probe A5
runs `sbx env plan` on them and records the sha256 of each golden in its
result; `tools/ci probes` flags a golden whose hash changed since the
recorded probe.

```yaml
schemaVersion: "1"
name: shop
agent: sbx-kit-claude
workspace: ./shop-api
additionalWorkspaces:
  - ./memory/shop-api
  - ./memory/shop-web
  - path: ./.romeu/bin
    readOnly: true
  - ./ledger/spool
  - path: ./ledger/view
    readOnly: true
kits:
  - source: docker.io/docker/sbx-kit-claude@sha256:<64 hex>
  - source: ./.romeu/kits/julieta-<digest12>
  - source: ./.romeu/kits/julieta-claude-<digest12>
  - source: ./.romeu/kits/os-base-<digest12>
    args: {tz: Etc/UTC}
  - source: ./.romeu/kits/git-ssh-sign-<digest12>
    args: {signingKey: "ssh-ed25519 AAAA... "}
  - source: ./.romeu/kits/my-home-<digest12>
sandboxOptions: {cpus: 4, memory: 8g}
secrets:
  github:
    command: "/usr/bin/env -i HOME=<abs home> PATH=/usr/bin:/bin <abs argv0> <args...>"
ports: []
```

Rules: no `x-*` keys (metadata lives in `render.json`); no `lifecycle`
hooks (they re-prompt on every invocation and `preRemove` only warns);
the secret command is built with `internal/shquote` from host-settings
argv with a scrubbed environment; romeu only ever renders `command`,
never a literal value; clone mode comes from romeu's `sbx env run
--clone`. `./.romeu/bin`, `./ledger/spool` and `./ledger/view` are
fixed mounts, rendered for each project from no spec field
([13 13.1](13-runtime-ledger.md#131-stores)).

## 3.4 Host settings (`settings.yaml`, schema `host-settings.v1`)

```yaml
apiVersion: romeu/v1
root: /abs/path/to/dev                  # absolute; symlinks resolved once, real path stored
config:
  project: verona
  url: https://github.com/<owner>/verona
  ref: main
tools:                                  # absolute paths, resolved by `romeu init`, re-checked by doctor
  sbx: /abs/path/sbx
  git: /abs/path/git
  gitCredentialHelper: osxkeychain      # optional; the only credential helper hardened git uses
gitHosts:
  - host: github.com
    caFile: ""                          # optional absolute path to a CA bundle (private hosts)
workloadRepositories:                   # allowlist for spec `workload`
  - docker.io/docker/sbx-kit-claude
signing:
  agentSocket: /abs/path/signing-agent.sock   # dedicated agent holding exactly one signing-only key (06 6.4)
secrets:
  github@verona:
    service: github                     # sbx service name
    argv: [/usr/bin/security, find-generic-password, -s, romeu/verona/github, -w]
  github@shop:
    service: github
    argv: [/usr/bin/security, find-generic-password, -s, romeu/shop/github, -w]
```

Rules: every `argv[0]` is absolute and exists; no shell strings; a spec
secret name without a matching `name@project` entry is exit 2 naming the
key; a `name@other-project` entry never satisfies a different project;
entries for unknown projects are reported by `doctor`. There is no
registry credential: descriptors are read anonymously (Q23). *Why for us:*
devcontainer `initializeCommand` runs repo-controlled commands on the
host; names in the spec and commands in host settings remove that class.

## 3.5 Egress catalog (`catalog/egress.yaml`, schema `catalog.v1`)

```yaml
version: 1
meta: [mise-versions.jdx.dev, github.com, objects.githubusercontent.com, api.github.com]   # mise itself, every project
base: [github.com, api.github.com, codeload.github.com]                                    # git over HTTPS, every project
domains:                    # upload flag per domain; a domain without an entry is treated as upload: true
  mise-versions.jdx.dev: {upload: false}
  objects.githubusercontent.com: {upload: false}
  codeload.github.com: {upload: false}
  github.com: {upload: true}
  api.github.com: {upload: true}
  dl.google.com: {upload: false}
  proxy.golang.org: {upload: false}
  sum.golang.org: {upload: false}
  storage.googleapis.com: {upload: true}      # multi-tenant object store
  nodejs.org: {upload: false}
  registry.npmjs.org: {upload: true}
  pypi.org: {upload: false}
  files.pythonhosted.org: {upload: false}
tools:                      # key: mise backend:tool as written in mise.lock; value: domains it needs (install and use)
  core:go: [dl.google.com, proxy.golang.org, sum.golang.org, storage.googleapis.com]
  core:node: [nodejs.org, registry.npmjs.org]
  core:python: [pypi.org, files.pythonhosted.org]
  aqua:jqlang/jq: []
  aqua:koalaman/shellcheck: []
  aqua:gohugoio/hugo: []
  aqua:cli/cli: []
```

One list per tool: because tools can be reinstalled from inside a
running sandbox, install and use domains are both needed at runtime, so
the distinction has no consumer. The catalog is embedded in both romeu
and julieta. CI requires every domain used anywhere
to have an explicit `domains` entry. The entries above are examples; the
v1 table is built from the reference config repo's locks (S4) and
measured in probe B4.

## 3.6 julieta manifest (schema `manifest.v1`)

Passed by romeu on every `sbx env exec` as
`--env JULIETA_MANIFEST=<base64url(canonical JSON)>` (<= 32 KiB), cached
by julieta at `$HOME/.local/state/julieta/manifest.json`.

```json
{
  "version": 1,
  "protocol": 1,
  "project": "shop",
  "generation": "01J9ZC2Q8X7W4T6Y3R5E1U9I0P",
  "repos": [
    {"dir": "shop-api", "url": "https://github.com/example/shop-api", "primary": true, "ref": "main",
     "path": "/abs/root/shop-env/shop-api", "memory": "/abs/root/shop-env/memory/shop-api", "base": ["<sha>"]},
    {"dir": "shop-web", "url": "https://github.com/example/shop-web", "primary": false, "ref": "main",
     "path": "/home/agent/src/shop/shop-web", "memory": "/abs/root/shop-env/memory/shop-web", "base": ["<sha>"]}
  ],
  "salvage": {"excludeIgnored": ["node_modules/", ".venv/", "target/"], "capBytes": 1073741824},
  "salvageRun": {"id": "01J9ZD0A1B2C3D4E5F6G7H8J9K"},
  "ledger": {"spool": "/abs/root/shop-env/ledger/spool", "view": "/abs/root/shop-env/ledger/view"},
  "run": {"...": "the validated run layout"},
  "runDigest": "<hex>",
  "agent": "sbx-kit-claude"
}
```

| Field | Meaning |
|---|---|
| `protocol` | the romeu/julieta contract number; julieta accepts a manifest of protocol N (its own) or N-1 (a cached manifest written before a romeu upgrade) and refuses anything else |
| `repos[].base` | origin SHAs the host clone already has; snapshot and salvage bundles exclude objects reachable from them (08) |
| `salvageRun` | present only on salvage calls: the salvage id romeu expects back |
| `ledger` | the mount paths of the project's spool and view ([13](13-runtime-ledger.md)); julieta emits events only when it is present |

The compatibility check that precedes the first call of a session is
described in [06 6.3](06-kits.md#63-julieta-delivery).

## 3.7 `render.json` (in `<name>-env/.romeu/`, schema `render.v1`)

```json
{
  "version": 1,
  "romeuVersion": "v1.0.0",
  "promoting": null,
  "specSource": {"kind": "origin", "ref": "refs/romeu/origin/verona/main", "commit": "<sha>"},
  "repos": [{"dir": "shop-api", "url": "...", "egressRef": "refs/romeu/origin/shop-api/main", "egressCommit": "<sha>", "lockSha256": "<hex|null>"}],
  "files": {
    "sbxenv.yaml": "<sha256>",
    ".romeu/kits/julieta-3f9a1c2b7d4e": "<kit-tree digest>",
    ".romeu/bin/julieta-linux-amd64": "<sha256>",
    ".romeu/bin/julieta-linux-arm64": "<sha256>",
    ".romeu/bin/SHA256SUMS": "<sha256>"
  },
  "egress": {"auto": ["dl.google.com"], "gated": [{"domain": "github.com", "reason": "upload"}], "effectiveDigest": "<hex>"},
  "wideningDigest": "<hex>",
  "recreateDigest": "<hex>",
  "catalogDigest": "<hex>"
}
```

`promoting` is `null` except inside a promotion commit
([01 1.6](01-system-model.md#promotion-commit)), when it is
`{"candidateId": "<ULID>", "filesDigest": "<hex>"}`. The julieta binary
digests here are what the compatibility check compares against.

## 3.8 Host state (schemas `state-*.v1`)

`toolchain.json` (gate 1):

```json
{
  "version": 1,
  "sbxVersion": "0.46.0",
  "romeuVersion": "v1.0.0",
  "julieta": {"version": "v1.0.0", "protocol": 1},
  "catalogDigest": "<hex>",
  "catalog": {"...": "the acknowledged catalog, decoded, so the next gate 1 can print a diff"},
  "acknowledgedAt": "2026-10-02T09:00:00Z"
}
```

`projects/<name>.json` (one record per project; hosts the candidate and
generation state machines of [01 1.6](01-system-model.md#16-state-machines)):

```json
{
  "version": 1,
  "approval": {"wideningDigest": "<hex>", "set": {"...": "the full widening set"}, "renderDigest": "<hex>",
               "specCommit": "<sha>", "candidateId": "01J9ZBX...", "approvedAt": "2026-10-02T09:10:00Z"},
  "candidate": null,
  "generations": [
    {"id": "01J9ZC2Q8X7W4T6Y3R5E1U9I0P", "state": "open", "adopted": false,
     "createdAt": "2026-10-02T09:14:07Z", "createdWith": "<recreateDigest>",
     "workspacePath": "/abs/root/shop-env/shop-api",
     "repos": [{"dir": "shop-api", "url": "https://github.com/example/shop-api", "primary": true},
               {"dir": "shop-web", "url": "https://github.com/example/shop-web", "primary": false}],
     "closedAt": null,
     "salvage": [{"id": "01J9ZD0A1B2C3D4E5F6G7H8J9K", "at": "2026-10-05T18:00:00Z", "manifestSha256": "<hex>",
                  "result": "complete", "reasons": [],
                  "refsPrefix": "refs/romeu/salvage/shop/01J9ZC2Q.../01J9ZD0A.../"}]}
  ],
  "egressApplied": ["dl.google.com", "github.com"],
  "lastRun": "2026-10-05T17:00:00Z"
}
```

| Field | Values |
|---|---|
| `candidate` | `null`, or the awaiting record `{"id", "state": "awaiting", "wideningDigest", "filesDigest", "specCommit", "renderedAt"}` |
| `generations[].state` | `open`, `salvaging`, `closed-removed`, `closed-lost` |
| `generations[].createdWith` | the recreate digest at create; `null` for an adopted generation (status reports "recreate digest unknown") |
| `salvage[].result` | `complete`, `incomplete`, `lost`; `reasons[]` lists each skipped item or `sandbox-lost`, and `ledger-incomplete` when the ingest before the sandbox half did not finish, which changes no `result` (13 13.4) |

Files are written atomically (temp file in the same dir, fsync, rename),
mode 0600. `descriptors/<digest>.json` holds the parsed, normalized
capability set of one workload or kit descriptor, keyed by its OCI
digest; entries are immutable.

## 3.9 Memory entry (schema `memory-entry.v1`)

One file per entry: `memory/<dir>/entries/<ULID>.md`. julieta stamps
`id`, `created` and `updated` on `add` and `edit`; the agent
supplies only `title`, `body`, `tags` and `status` (a supplied id or
timestamp is an error).

```markdown
---
version: 1
id: 01J9Z3K8M6T2Q4W5E7R9Y1U3I5
title: Flaky integration test on arm64
status: open            # open | done
tags: [ci, arm64]       # the tag "lesson" makes SessionStart print the entry
created: 2026-10-02T09:20:00Z
updated: 2026-10-02T09:20:00Z
---
Free-form markdown body.
```

JSONL import (`julieta memory import --format jsonl`), one object per
line: `{"title": "...", "body": "...", "tags": ["..."], "status": "open|done", "created": "RFC3339"}`.
`created` is kept; `id` and `updated` are stamped by julieta. The content
digest used for idempotency and verification is
`canon.Digest("memory-entry", {title, body, created})`; `status` is
compared as a separate field (see `memory verify` in 04).

## 3.10 Handoff file (schema `handoff.v1`)

`memory/<dir>/handoff/<ULID>-<kind>.md`, where `<kind>` is `clear`,
`final` or `facts`. The kind is in the file name so the handoff reader
selects files without parsing them; there is no pointer file.

```markdown
---
version: 1
kind: clear            # clear | final | facts   (facts = written by the SessionEnd hook, no narrative)
created: 2026-10-02T18:30:00Z
project: shop
generation: 01J9ZC2Q8X7W4T6Y3R5E1U9I0P
agent: sbx-kit-claude  # informational; the format is agent-neutral
repos:
  - dir: shop-api
    branch: feat/cart
    head: <sha>
    upstream: origin/feat/cart
    ahead: 2
    behind: 0
    dirty: 3
    stashes: 0
---
## Goal
## State
## Decisions
## Next steps
## Open questions
## What would be lost   (kind: final only)
```

Kinds `clear` and `final` require the headings (six for `final`);
`facts` has no body. julieta stamps everything in the front matter.

## 3.11 Salvage manifest (schema `salvage.v1`)

`memory/<dir>/salvage/<generation>/<salvage-id>/manifest.json`:
`salvageId` (must equal the id romeu passed), `generation`, `repo`,
`worktrees[]` (path, HEAD, branch or detached, dirty), `refs[]` (every
ref in the bundles with SHA: branches, `salvage/*`, tags, notes, each
worktree HEAD), `bundles[]` (file, sha256, size, embedded-repo path if
any), `files[]` (path, size, sha256), `skipped[]` (item, reason:
`excluded-by-spec`, `over-cap`, `transcript-excluded`, `unreadable`) and
`complete`. romeu recomputes completeness itself; the field is advisory.

## 3.12 Digests (`internal/canon`)

Structured values are digested with one function,
`canon.Digest[T](kind, v)`: sha256 over `"romeu/<kind>/v1"`, a NUL byte,
and the canonical JSON of `v` (keys sorted, UTF-8, no insignificant
whitespace). Each Go type has exactly one kind, so a value of one kind
can never be compared with a digest of another. Raw file bytes
(`sbxenv.yaml`, binaries, bundles, payload files, probe artifacts,
spool files of the runtime ledger) use
plain sha256, because external tools (`sha256sum`, `shasum`) must
reproduce them.

| Kind | Input | Used by |
|---|---|---|
| `widening` | the widening set | gate 2 |
| `recreate` | every field tagged `apply:"recreate"` | "recreate needed" |
| `render` | the normalized render (01 1.4) | the widening set's `renderDigest` |
| `egress` | the sorted effective domain union | `render.json` |
| `catalog` | the decoded catalog | gate 1 |
| `kit-tree` | sorted `{path, mode, sha256}` of a kit | widening set, kit dir name |
| `candidate-files` | sorted `{path, sha256}` of a candidate dir | awaiting record, promotion recovery |
| `run` | the validated run layout | stale layout detection |
| `lock-set` | sorted `{dir, mise.lock sha256}` | `julieta install` skip rule |
| `secret-argv` | one secret's argv | widening set |
| `memory-entry` | `{title, body, created}` | memory import and verify |

## 3.13 Other schemas

`probe-result.v1` is defined in [11](11-host-probes.md#113-probe-result-format);
`acceptance.v1` in [10](10-testing-style.md#105-acceptance-evidence).
The runtime ledger's three formats are defined in
[13](13-runtime-ledger.md): the event (`runtime-event.v1`, written in
an event as `runtime-event/v1`, [13.2](13-runtime-ledger.md#132-events)),
the ledger entry ([13.5](13-runtime-ledger.md#135-entries)) and the
view files ([13.6](13-runtime-ledger.md#136-view)).
