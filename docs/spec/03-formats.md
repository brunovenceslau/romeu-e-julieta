# 3. File formats

Back to [index](../spec.md). Reader: implementers of `spec`, `render`,
`state`, `memstore` and `handoff`. Type: reference.

Every format in the schema list of
[02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public) has a
version field, strict decoding (unknown keys are errors), a JSON Schema
in `schemas/`, and examples in `examples/` validated in CI; the formats
without a schema are listed, each with its reason, in 3.13.

Each format the product owns names itself and its version in one form,
`schema: <name>/v<n>`, where `<name>` is its schema name in 02 2.1; the
generated schemas carry the field as a constant. `sbxenv.yaml`'s
`schemaVersion` belongs to sbx and stays. A timestamp is RFC 3339 UTC
and is named `<past participle>At` (`createdAt`, `updatedAt`).

Versions: any change to a format's fields bumps its version. A reader
refuses a version newer than its own with the error `format-newer`
(`RJ-311`, [04 4.4](04-cli.md#44-error-ids)) and the fix hint "install
the release that wrote it". The release that bumps a stored format reads
the previous version and writes the new one on its next whole-file write
(forward-only; a read-only consumer, such as romeu reading memory
entries, never rewrites). A record romeu cannot decode is never
overwritten: `status` and `doctor` report it as the error
`record-unreadable` (`RJ-334`), and `salvage --from-host` still runs
from the tree and the clones into a new record beside it. The
user-authored formats (`project.v1`, `host-settings.v1`) change only by
a new version of their `schema` field, announced first in the release
notes, which carry the edit to make. The window for these formats is
the current and the previous version; a reader refuses an older one
with the error `format-unsupported` (`RJ-337`,
[04 4.4](04-cli.md#44-error-ids)) and the fix hint "apply the edit in
the release notes of each skipped release". The manifest keeps its own N-1
window, with its reason, in 3.6.

Every format decoded from a mount or a repository (the memory entry,
the handoff, `snapshot/heads.json`, `mise.lock`, and v3 descriptors) is
parsed with a strict subset grammar: no YAML anchors, aliases, merge
keys or includes, and a decoded size bounded by the file cap (07 7.5,
step 1). `mise.lock` is TOML, and its limits are these: no duplicate
keys or tables, and the same file cap. Its top level is strict (an unknown key is an error), because the file is
agent-writable and drives egress, so a partial read could widen what a
sandbox may reach; a lock that does not parse is `lock-unparsed`
(`RJ-325`, exit 2), not `upstream-shape`.

Outputs of tools the product does not own (`sbx ls --json`, sbx policy
and secret listings, herdr replies, registry manifests,
GitHub API JSON) are decoded tolerantly and checked strictly: unknown
fields are allowed; every field romeu or julieta reads is required and
typed; a body that does not decode, or a missing or mistyped field, is
the error `upstream-shape` (`RJ-310`, exit 2), naming the upstream
and its live version; no parser returns absent, none or an empty list
for a shape it did not recognize. An sbx call that exits non-zero is
not a shape error: it is `sbx-unknown` (`RJ-308`, exit 1) where the
command needs to know whether a sandbox is absent. Gate 1, the
generation machine and preflight step 5 rest on this rule
([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine),
[01 1.6](01-system-model.md#16-state-machines),
[01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)).

Validation rules live as data in `internal/spec/rules.go`: each rule
names a field path, its constraint (pattern, length, enum, reserved
values, uniqueness scope), a message, an error id and its scope (both
validators, or romeu only). From the Go types and those rules,
`go generate` derives the validators, the rule table in the reference
docs and the JSON Schemas (`tools/schemagen`, an in-repo reflect-based
generator; no dependency), so types, validators, docs and schemas
cannot drift. The tables below are the v1 design input to `rules.go`
and the struct tags. The pull request that lands a format's generator
replaces that format's field and rule tables here by a link to its
generated page under `docs/reference/`; this page then keeps only the intent,
the annotated example and the rules that are design decisions.

## 3.1 Name and path rules (shared)

`julieta spec validate` in the config repo's CI cannot see host
settings, so the rules that read them are checked by romeu only; the
"Checked by" column says which, and `spec validate` prints one line
naming the host-only rules it did not check.

| Field | Rule | Checked by |
|---|---|---|
| project `name` | `^[a-z][a-z0-9-]{0,38}[a-z0-9]$`; unique across the config repo; equals the file stem (satisfies sbx's <= 63 chars, no trailing `-` or `.`) | both |
| repo `dir` | `^[a-z0-9][a-z0-9._-]{0,62}$`, not `.` or `..`, no `/`, not a name romeu reserves inside `<name>-env/`: `memory`, `review`, `ledger`, `.romeu`, `sbxenv.yaml`, `.git` (compared case-insensitively); unique within the project, also case-insensitively. This row is the one list of reserved names | both |
| repo `url` | `https://<host>/<owner>/<repo>[.git]` only: v1 supports GitHub-shaped URLs (one owner segment, so no GitLab subgroups), and repos, CI and releases assume GitHub; a URL may appear in at most one project, compared after one normalization: host and path lowercased, a trailing `.git` removed | both |
| repo `url` host | `<host>` must be a host-settings `gitHosts` entry | romeu only |
| repo `ref` | a branch name valid under `git check-ref-format --branch` rules, implemented in Go | both |
| kit id | `^[a-z0-9][a-z0-9-]{0,62}$` | both |
| kit `args` | key `^[a-z][a-zA-Z0-9_]{0,31}$`; value printable UTF-8, no newline, <= 1 KiB | both |
| secret name | `^[a-z0-9][a-z0-9_-]{0,62}$` | both |
| secret binding | a `name@project` entry in host settings for every spec secret name (3.4) | romeu only |
| run layout pane id / tab label | `^[a-z0-9][a-z0-9_-]{0,31}$` | both |
| relative `cwd` in the run layout | cleaned path, no leading `/`, no `..` element | both |
| egress domain (every source of [07 7.5](07-mise-egress.md#75-egress-derivation-internalegress): `egress.extra`, catalog, lock-derived, kit-declared) | lowercase hostname, IDNA A-labels, no wildcard, no port, no IP literal | both |
| workload reference | `<registry>/<repository>@sha256:<64 hex>` | both |
| workload repository | `<registry>/<repository>` must be in host settings `workloadRepositories` | romeu only |

## 3.2 Project spec (`projects/<name>.yaml`, schema `project.v1`)

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/<owner>/romeu-e-julieta/v1.0.0/schemas/project.v1.json
schema: project/v1
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
run:                          # versioned with the spec's schema
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

The `$schema` line is an editor hint only; `julieta spec validate` in
the config repo's CI, at the pinned julieta, is the validation. It
points at a readable tag URL, which a moved tag could change.

The classification is the struct tags on the spec types
(`gate:"widening"`, `apply:"recreate|live"`, 01 1.4);
`docs/reference/project.md`, generated from them, replaces this table
(opening).

| Field | `gate` | `apply` | Notes |
|---|---|---|---|
| `repos` (incl. `ref`) | widening | recreate | `ref` decides which commit egress derives from |
| `workload`, `kits` | widening (with capabilities) | recreate | |
| `agent` | widening | recreate | |
| `ports` | widening | recreate | |
| `secrets` | widening | recreate; `live` if probe A10 confirms (Q18) | |
| `egress.extra`, `egress.tools` | widening | live | |
| `salvage.excludeIgnored` | widening | live | it defines acknowledged loss |
| `sandboxOptions` (`cpus`, `memory` only) | - | recreate | `status` reports recreate needed; stripped from `renderDigest`; on the list of deliberately ungated fields (01 1.4), an accepted risk of [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1) |
| `run` | - | live | pane commands run only inside the sandbox; on the list of deliberately ungated fields (01 1.4) |

There is no field for a host command, an absolute path, a mount, or an
env var for the sandbox as a whole or for sbx; a pane's `env` reaches
that pane's steps only, which is why `PATH`, `HOME`, `LD_*`, `BASH_ENV`
and `ENV` are refused there. A `command`, `argv` or `env` key under
`secrets` is a decode error.

### Run layout (`run`)

The run layout is romeu's own format, which julieta renders for herdr,
the one renderer; a second renderer is a row of
[Deferred decisions](../spec.md#in-the-product).

| Key | Meaning |
|---|---|
| `tabs[]` | `label` + `root`; a node is `{pane: <id>}` or `{split: row|column, ratio: 0.1..0.9, first, second}` (binary tree) |
| `panes[]` | `id`, `repo` (a project `dir`), `cwd` (relative to the repo), `env` (keys `^[A-Z_][A-Z0-9_]*$`, not `PATH`, `HOME`, `LD_*`, `BASH_ENV`, `ENV`), `steps` (list of argv lists; no shell), `focus` (at most one per tab) |

Execution rule: every step runs as `mise exec -C <pane dir> -- <argv>`;
steps run in order; each must exit 0 before the next; the last step is
exec'd; a failing step prints its exit status and drops to a login shell
in the pane dir. No steps = login shell. Every pane id appears exactly
once across all tabs.

*Why for us:* mise daemons add `ready`/`deps`, which is
supervisor territory; a layout-and-steps list is what we need, and a
later version could delegate readiness to mise daemons.

## 3.3 Rendered `sbxenv.yaml` (sbx env file, `schemaVersion: "1"`)

Produced only by `internal/render` through the YAML marshaller. The key
names below follow the documented env-file schema and the documented v3
workload pattern; the golden files are **probe-confirmed**: probe A5
runs `sbx env plan` on them and records the hash of each golden's
normalized form (decoded and re-encoded with sorted keys by one
function in `internal/canon`), and `tools/ci probes` compares that
normalized hash, so a byte the marshaller emits differently does not
force a re-probe; render's own test keeps the raw golden. The set is
confirmed at block A; a golden added later needs a re-probe before it
counts as confirmed, and `tools/ci probes` prints a notice for it
([11 11.4](11-host-probes.md#114-probe-lifecycle)).

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
kits:
  - source: docker.io/docker/sbx-kit-claude@sha256:<64 hex>
  - source: ./.romeu/kits/julieta-<digest12>
  - source: ./.romeu/kits/julieta-claude-<digest12>
    args: {julietaBin: "<abs root>/<name>-env/.romeu/bin"}
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
--clone`. `./.romeu/bin` is a fixed mount, rendered for each project
from no spec field. romeu names this file by path; probe A5 records
whether sbx also reads a sandbox configuration from the workspace tree,
and if it does, that file joins the plant list of I3 and the drift
check refuses it ([04 4.2](04-cli.md#how-romeu-run-reaches-the-run-layout),
run step 3).

## 3.4 Host settings (`settings.yaml`, schema `host-settings.v1`)

```yaml
schema: host-settings/v1
root: /abs/path/to/dev                  # absolute; symlinks resolved once, real path stored
config:
  project: my-config
  url: https://github.com/<owner>/my-config
  ref: main
tools:                                  # absolute paths, resolved by `romeu init`, re-checked by doctor
  sbx: /abs/path/sbx
  git: /abs/path/git
  gitCredentialHelper: osxkeychain      # optional; a git-credential-<name> helper on /usr/bin:/bin, or an absolute path; hardened git uses this helper and no other
gitHosts:
  - host: github.com
    caFile: ""                          # optional absolute path to a CA bundle (private hosts)
workloadRepositories:                   # allowlist for spec `workload`
  - docker.io/docker/sbx-kit-claude
signing:
  agentSocket: /abs/path/signing-agent.sock   # dedicated agent holding exactly one signing-only key (06 6.4)
secrets:
  github@my-config:
    service: github                     # sbx service name
    argv: [/usr/bin/security, find-generic-password, -s, romeu/my-config/github, -w]
  github@shop:
    service: github
    argv: [/usr/bin/security, find-generic-password, -s, romeu/shop/github, -w]
  npm@shop:                             # a manager other than Keychain (the 1Password CLI); doctor's run shows it works from HOME alone
    service: npm
    argv: [/opt/homebrew/bin/op, read, "op://dev/npm/token"]
```

Values you set: `romeu init` writes `root`, `config` and `tools`; the
operator fills `gitHosts`, `workloadRepositories`, `signing` when a
project uses `git-ssh-sign`, and `secrets`
([J1](09-journeys.md#j1-onboarding-new-machine-first-time) step 5).

Rules: every `argv[0]` is absolute and exists; no shell strings; an argv
is a command that fetches the value and never carries it, so an argv
element that starts with `ghp_`, `gho_`, `ghs_`, `ghu_`, `ghr_` or
`github_pat_`, or
matches `^[0-9a-f]{40}$`, is refused with the error `secret-in-argv`
(`RJ-314`; a best-effort guard against the common token shapes, not a
detector); a spec secret name without a matching `name@project`
entry is exit 2 naming the key; a `name@other-project` entry never
satisfies a different project; entries for unknown projects are
reported by `doctor`. A secret argv and the credential helper run with
exactly `HOME` and `PATH=/usr/bin:/bin`, so a manager must work from
`HOME` alone (Keychain, or a CLI whose session lives in a file under
`HOME`); a manager that needs a session variable is not supported in
v1. `doctor` runs each secret argv once in that environment and fails
on a non-zero exit or empty output; it never prints, keeps or writes
the value. There is no registry credential: descriptors are read
anonymously, and a credential for a private workload registry is a row
of [Deferred decisions](../spec.md#in-the-product). *Why for us:*
devcontainer `initializeCommand` runs repo-controlled commands on the
host; names in the spec and commands in host settings remove that class.

## 3.5 Egress catalog (`catalog/egress.yaml`, schema `catalog.v1`)

```yaml
schema: catalog/v1
meta: [mise-versions.jdx.dev, github.com, objects.githubusercontent.com, api.github.com]   # mise itself, every project
base: [github.com, api.github.com, codeload.github.com]                                    # git over HTTPS, every project
domains:                    # upload flag per domain; CI refuses a domain without an entry
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
v1 table covers the tools listed in `catalog/egress.yaml`; it was built
from the reference config repo's locks (S4) and measured in probe B4. A
tool outside it is declared with `egress.tools` in the project spec
(always gated) until a catalog entry ships in a release.

## 3.6 julieta manifest (schema `manifest.v1`)

Passed by romeu on every `sbx env exec` as
`--env JULIETA_MANIFEST=<base64url(canonical JSON)>`, cached by julieta
at `$HOME/.local/state/julieta/manifest.json`. romeu checks the encoded
size at `sync`, before the gate, and refuses a manifest over 32 KiB with
the error `manifest-size` (`RJ-313`); `julieta spec validate` reports
the same rule. The Go type lives in `internal/manifest`, the one
package both binaries import for it.

```json
{
  "schema": "manifest/v1",
  "protocol": 1,
  "project": "shop",
  "specCommit": "<sha>",
  "generation": "01j9zc2q8x7w4t6y3r5e1v9k0p",
  "repos": [
    {"dir": "shop-api", "url": "https://github.com/example/shop-api", "primary": true, "ref": "main",
     "path": "/abs/root/shop-env/shop-api", "memory": "/abs/root/shop-env/memory/shop-api", "base": ["<sha>"]},
    {"dir": "shop-web", "url": "https://github.com/example/shop-web", "primary": false, "ref": "main",
     "path": "/home/agent/src/shop/shop-web", "memory": "/abs/root/shop-env/memory/shop-web", "base": ["<sha>"]}
  ],
  "salvage": {"excludeIgnored": ["node_modules/", ".venv/", "target/"], "capBytes": 1073741824},
  "salvageRun": {"id": "01j9zd0a1b2c3d4e5f6g7h8j9k"},
  "run": {"...": "the validated run layout"},
  "runDigest": "<hex>",
  "egress": ["dl.google.com", "github.com"],
  "agent": "sbx-kit-claude"
}
```

| Field | Meaning |
|---|---|
| `protocol` | the romeu/julieta contract number; julieta accepts a manifest of protocol N (its own) or N-1 (a cached manifest written before a romeu upgrade) and refuses anything else |
| `specCommit` | the config repo commit of the live spec, fresh at each exec; the agent compares it with the commit it pushed (01 1.2) |
| `repos[].path` | the primary's host path (clone mode keeps it); a secondary's `<home>/src/<project>/<dir>`, where `<home>` is the workload user's home, measured by probe A4 and stored with the workload pin in `kits/pins.yaml` |
| `repos[].base` | origin SHAs the host clone already has, each held by the create-only ref `refs/romeu/base/<name>/<generation>/<dir>` ([02 2.3](02-layouts.md#23-host-tree)); snapshot and salvage bundles exclude objects reachable from them (08) |
| `salvageRun` | present only on salvage calls: the salvage id romeu expects back |
| `egress` | the domains romeu applied to this sandbox, `auto` plus approved `gated` ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress)); the pending gated set lives on the host and is not carried. `julieta status` prints it |

The compatibility check that precedes every romeu command that execs
julieta, and in which romeu accepts julieta protocols N and N-1 the way
julieta accepts manifests, is described in
[06 6.3](06-kits.md#63-julieta-delivery).

## 3.7 `render.json` (in `<name>-env/.romeu/`, schema `render.v1`)

```json
{
  "schema": "render/v1",
  "romeuVersion": "v1.0.0",
  "promoting": null,
  "specSource": {"kind": "origin", "ref": "refs/romeu/origin/my-config/main", "commit": "<sha>"},
  "repos": [{"dir": "shop-api", "url": "...", "egressRef": "refs/romeu/origin/shop-api/main", "egressCommit": "<sha>", "lockSha256": "<hex|null>"}],
  "files": {
    "sbxenv.yaml": "<sha256>",
    ".romeu/kits/julieta-3f9a1c2b7d4e": "<kit-tree digest>",
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
`{"candidateId": "<ULID>", "filesDigest": "<hex>", "previousFiles": {...}}`,
`previousFiles` being the `files` map before the promotion, which
recovery writes back when the candidate no longer verifies. The julieta
binary digest here is what the compatibility check compares against.

## 3.8 Host state (schemas `state-*.v1`)

`toolchain.json` (gate 1):

```json
{
  "schema": "state-toolchain/v1",
  "sbxVersion": "0.46.0",
  "romeu": {"version": "v1.0.0", "sha256": "<hex>"},
  "julieta": {"version": "v1.0.0", "protocol": 1, "sha256": "<hex>"},
  "catalogDigest": "<hex>",
  "catalog": {"...": "the acknowledged catalog, decoded, so the next gate 1 can print a diff"},
  "acknowledgedAt": "2026-10-02T09:00:00Z"
}
```

`projects/<name>.json` (one record per project; hosts the generation
state machine of [01 1.6](01-system-model.md#16-state-machines), which
says which states persist; the candidate states do not):

```json
{
  "schema": "state-project/v1",
  "approval": {"wideningDigest": "<hex>", "set": {"...": "the full widening set"}, "renderDigest": "<hex>",
               "specCommit": "<sha>", "candidateId": "01j9zbx...", "approvedAt": "2026-10-02T09:10:00Z"},
  "retiring": null,
  "generations": [
    {"id": "01j9zc2q8x7w4t6y3r5e1v9k0p", "state": "open", "adopted": false,
     "createdAt": "2026-10-02T09:14:07Z", "createdWith": "<recreateDigest>",
     "workspacePath": "/abs/root/shop-env/shop-api",
     "repos": [{"dir": "shop-api", "url": "https://github.com/example/shop-api", "primary": true},
               {"dir": "shop-web", "url": "https://github.com/example/shop-web", "primary": false}],
     "closedAt": null,
     "salvage": [{"id": "01j9zd0a1b2c3d4e5f6g7h8j9k", "createdAt": "2026-10-05T18:00:00Z", "manifestSha256": "<hex>",
                  "result": "complete", "reasons": [],
                  "refsPrefix": "refs/romeu/salvage/shop/01j9zc2q.../01j9zd0a.../",
                  "fingerprint": null}]}
  ],
  "egressApplied": ["dl.google.com", "github.com"],
  "lastRunAt": "2026-10-05T17:00:00Z"
}
```

| Field | Values |
|---|---|
| `retiring` | `null`, or `{"attic": "<UTC-ts>"}` while the retire steps of 01 1.6 run |
| `generations[]` | every generation of the project; v1 keeps them all, so the record grows by one generation per recreate and has no bound. `doctor` warns past 256 KiB, and compaction is a row of [Deferred decisions](../spec.md#in-the-product) |
| `generations[].state` | `open`, `salvaging`, `removing`, `closed-removed`, `closed-lost` |
| `generations[].createdWith` | the recreate digest at create; `null` for an adopted generation (status reports "recreate digest unknown") |
| `salvage[].manifestSha256` | plain sha256 of the salvage `manifest.json` bytes (3.12, raw) |
| `salvage[].fingerprint` | `null`, or, written in the same whole-file write as the state `removing` (01 1.6), the plain sha256 (3.12) of the values that step 2 of [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction) and the host verification read, not read again at that write: the daemon heads, each worktree's HEAD and a digest of its status, which holds tracked changes and untracked paths, each with the digest of its file, and ignored paths, minus `salvage.excludeIgnored`, each by path, size and modification time. The rerun of `rm`, `recreate` or `retire` that finds the generation `removing` with its sandbox present computes it again and resumes at `sbx env rm` only when the two are equal. A rerun that salvages again prints the first path whose entry changed, and the third salvage in a row that a changed fingerprint starts for one generation exits 1, naming that path as `worktree-changing`. A field of the state format, under the version rule of the opening of this page |
| `salvage[].result` | `complete`, `incomplete`, `lost`; `reasons[]` lists each skipped item, `sandbox-half-failed`, or `sandbox-lost`, and for a generation `run` closed as lost, `dirty-at-last-facts:<n>` and `stashes-at-last-facts:<n>` from the newest facts ([04 4.2](04-cli.md#how-romeu-run-reaches-the-run-layout), run step 2); `acceptedLoss[]` lists the reasons `--accept-loss` named |

Every file the product writes in place is written as a temp file in the
same dir, fsync of the file, rename, fsync of the dir; host state files
have mode 0600. `descriptors/<digest>.json` holds the parsed, normalized
capability set of one workload or kit descriptor, keyed by its OCI
digest; entries are immutable.

## 3.9 Memory entry (schema `memory-entry.v1`)

One file per entry: `memory/<dir>/entries/<ULID>.md`. julieta stamps
`id`, `createdAt` and `updatedAt` on `add` and `edit`; the agent
supplies only `title`, `body`, `tags` and `status` (a supplied id or
timestamp is an error).

```markdown
---
schema: memory-entry/v1
id: 01j9z3k8m6t2q4w5e7r9y1v3k5
title: Flaky integration test on arm64
status: open            # open | done
tags: [ci, arm64]       # the tag "lesson" makes SessionStart print the entry
createdAt: 2026-10-02T09:20:00Z
updatedAt: 2026-10-02T09:20:00Z
---
Free-form markdown body.
```

Tags with behaviour are a closed set: `lesson` (SessionStart prints the
entry). An entry file is self-contained: copying `memory/<dir>/entries/`
whole is a complete move, and the store lock is recreated on first use.

## 3.10 Handoff file (schema `handoff.v1`)

`memory/<dir>/handoff/<ULID>-<kind>.md`, where `<kind>` is `clear`,
`final` or `facts`. The kind is in the file name so the handoff reader
selects files without parsing them; there is no pointer file.

```markdown
---
schema: handoff/v1
kind: clear            # clear | final | facts   (facts = written by the SessionEnd hook, no narrative)
createdAt: 2026-10-02T18:30:00Z
project: shop
generation: 01j9zc2q8x7w4t6y3r5e1v9k0p
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
ref in the bundles with SHA: branches, the current generation's
`refs/salvage/<generation>/*`, the reflog-only commits as
`refs/salvage-reflog/<branch>/<n>`, tags, notes, each worktree HEAD),
`bundles[]` (file, sha256, size, embedded-repo path if any), `files[]`
(path, size, sha256), `skipped[]` (item, reason: `excluded-by-spec`,
`over-cap`, `transcript-excluded`, `unreadable`, `disk-full`,
`worktree-changing`, `in-progress-operation`, `agent-not-matched`,
`agent-path-missing`; the host half adds `fsck-failed` and
`daemon-url-refused` to the salvage record's `reasons[]`, 3.8; the
recovery of each is the table of
[08 8.5](08-memory-handoff-salvage.md#skipped-reasons-and-their-recovery))
and `complete`. romeu recomputes completeness itself; the field is advisory.
Every path (`bundles[].file`, `files[].path`, `worktrees[].path`, an
embedded-repo path) is relative, cleaned, has no `..` element and no
symlink component, and resolves below the salvage dir (the error
`salvage-path`, `RJ-307`); the total bytes romeu verifies are bounded by
`salvage.capBytes`, and a manifest over it makes the salvage incomplete
with reason `over-cap`.

## 3.12 Digests (`internal/canon`)

Structured values are digested with one function,
`canon.Digest[T](kind, v)`: sha256 over `"romeu/<kind>/v1"`, a NUL byte,
and the canonical JSON of `v` (keys sorted, UTF-8, no insignificant
whitespace). Each Go type has exactly one kind, so a value of one kind
can never be compared with a digest of another. Raw file bytes
(`sbxenv.yaml`, binaries, bundles, payload files, probe artifacts,
salvage manifests as `manifestSha256`) use plain sha256, because
external tools (`sha256sum`, `shasum`) must reproduce them.

The kinds table is closed for v1 and lands whole with `internal/canon`;
a new kind is a change to this table first. Every ULID the product
writes is lowercase Crockford base32 (the reason of
[13 13.3](13-runtime-ledger.md#133-spool) holds for every file on the
host disk: two names that differ only in case collide on APFS), and
decoders refuse uppercase.

| Kind | Input | Used by |
|---|---|---|
| `widening` | the widening set | gate 2 |
| `recreate` | every field tagged `apply:"recreate"` | "recreate needed" |
| `render` | the normalized render (01 1.4) | the widening set's `renderDigest` |
| `egress` | the sorted effective domain union | `render.json` |
| `catalog` | the decoded catalog | gate 1 |
| `kit-tree` | sorted `{path, mode, sha256}` of a kit | widening set, kit dir name |
| `candidate-files` | sorted `{path, sha256}` of a candidate dir | promotion recovery |
| `run` | the validated run layout | stale layout detection |
| `lock-set` | sorted `{dir, mise.lock sha256}` | `julieta install` skip rule |
| `secret-argv` | one secret's argv | widening set |

## 3.13 Other schemas

`probe-result.v1` is defined in [11](11-host-probes.md#113-probe-result-format);
`acceptance.v1` in [10](10-testing-style.md#105-acceptance-evidence).
The runtime ledger's formats are designed in [13](13-runtime-ledger.md)
and are not in v1.

Formats without a schema, each with its reason:

| Format | Reason |
|---|---|
| `snapshot/heads.json` | read by romeu alone, decoded strictly under the opening's rules, and pinned by a golden |
| `.romeu/bin/SHA256SUMS` | the `sha256sum` line format, which external tools read |
| `e2e/scenarios/commands.yaml` | read by one program, the scenario runner, and pinned by its test |
| `kits/pins.yaml` | written by `tools/kitpin` and read by the product's own consumers, pinned by their tests |
| `docs/grants.yaml` | read by one program, `tools/ci pr`, and pinned by its test; its form is in [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change) |
