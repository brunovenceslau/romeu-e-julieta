# 3. File formats

Back to [index](../spec.md). Reader: implementers of `spec`, `render`,
`state`, `memstore` and `handoff`. Type: reference.

`REQ-0159` Every format in the schema list of
[02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public) has a
version field, strict decoding (a duplicate key is an error, and so is an
unknown key in a file a person writes; the versions rule below says what
a reader between binaries does with one,
[ADR 0041](../adr/0041-readers-of-formats-between-binaries-ignore-unknown-members.md)),
a JSON Schema in `schemas/`, and examples in `examples/` validated in CI;
the formats without a schema are listed, each with its reason, in 3.13.

`REQ-0160` JSON is decoded with `encoding/json/v2` (in the standard library of the
pinned Go; the v1 package is implemented over it): an owned format that a
person writes with `json.RejectUnknownMembers(true)`, an upstream output
and an owned format between binaries without it (the versions rule
below). v2's defaults, a duplicate member name
is an error and member names match exactly, are the strictness this
page means by strict. A third-party kit descriptor carried in an image
annotation is not an upstream output in this sense: it is the input of
a gate, decoded strictly by the descriptor grammar, and an unknown
member or capability type fails closed (I28, REQ-0573).

`REQ-0161` Each format the product owns names itself and its version in one form,
`schema: <name>/v<n>`, where `<name>` is its schema name in 02 2.1; the
generated schemas carry the field as a constant. `sbxenv.yaml`'s
`schemaVersion` belongs to sbx and stays. `REQ-0162` A timestamp is RFC 3339 UTC
and is named `<past participle>At` (`createdAt`, `updatedAt`).

`REQ-0163` Versions: formats evolve by the rules of protobuf
([ADR 0041](../adr/0041-readers-of-formats-between-binaries-ignore-unknown-members.md)).
Adding a member does not change a format's version; removing or
renaming a member, or changing its type or meaning, is a new version,
and the name of a removed member is never used again. `REQ-1310` A
member that is retired, or whose meaning would change, is not changed
within a version: it stays in the format, its writer writes it as its
zero value from that release on, it is marked deprecated where its
type is declared, and a new optional member carries the new meaning,
whose absence a reader takes as unknown; the next version removes the
old member, and its name is never used again (decided on 2026-10-28, `julieta-setup/v1`'s
`fastForward`, `from` and `to` the first case). A reader of a
format one binary writes and another reads (the `--json` documents of
`julieta`, `snapshot/heads.json`, the memory entry, the handoff file and
the manifest) skips a member it does not know, at any depth, and keeps every
other rule of strict decoding; a reader of a file a person writes (the
project spec, host settings, the catalog, the files of `tools/ci`)
refuses an unknown key, since one there is most often a typing error.
The manifest (3.6) is written by romeu and read by julieta, so it
follows the first rule, with the must-understand exception of 3.6. A reader
refuses a version newer than its own with the error `format-newer`
(`RJ-311`, [04 4.4](04-cli.md#44-error-ids)) and the fix hint "install
the release that wrote it". `REQ-0164` The release that bumps a stored format reads
the previous version and writes the new one on its next whole-file write
(forward-only; a read-only consumer, such as romeu reading memory
entries, never rewrites). `REQ-0165` A record romeu cannot decode is never
overwritten: every reader that needs the record exits 2 with
`record-unreadable` (`RJ-334`); `status` reports the record as
unreadable and exits 1, and `doctor` reports it as the failed check
`record-unreadable` (exit 6); `salvage --from-host` still runs from the
tree and the clones into a new record beside it. `REQ-0166` The user-authored
formats (`project.v1`, `host-settings.v1`) change only by a new version
of their `schema` field, announced first in the release notes, which
carry the edit to make. `REQ-0167` The window for these formats is the current and
the previous version; a reader refuses an older one with the error
`format-unsupported` (`RJ-337`, [04 4.4](04-cli.md#44-error-ids)) and
the fix hint "decide: apply the edit in the release notes of each
skipped release". The manifest keeps its own N-1 window, with its
reason, in 3.6.

`REQ-0168` Every format decoded from a mount or a repository (the memory entry,
the handoff, `snapshot/heads.json`, `mise.lock`, and v3 descriptors) is
parsed with a strict subset grammar: no YAML anchors, aliases, merge
keys or includes, and a decoded size bounded by the file cap (07 7.5,
step 1). `REQ-0169` `mise.lock` is TOML, and its limits are these: no duplicate
keys or tables, and the same file cap. Its top level is strict (an unknown key is an error), because the file is
agent-writable and drives egress, so a partial read could widen what a
sandbox may reach; a lock that does not parse is `lock-unparsed`
(`RJ-325`, exit 2), not `upstream-shape`.

`REQ-0170` Outputs of tools the product does not own (`sbx ls --json`, sbx policy
and secret listings, herdr replies, registry manifests,
GitHub API JSON) are decoded tolerantly and checked strictly: unknown
fields are allowed; every field romeu or julieta reads is required and
typed; a body that does not decode, or a missing or mistyped field, is
the error `upstream-shape` (`RJ-310`, exit 2), naming the upstream
and its live version; no parser returns absent, none or an empty list
for a shape it did not recognize. `REQ-0171` An sbx call that exits non-zero is
not a shape error: it is `sbx-unknown` (`RJ-308`, exit 1) where the
command needs to know whether a sandbox is absent. Gate 1, the
generation machine and preflight step 5 rest on this rule
([01 1.4](01-system-model.md#gate-1-toolchain-acknowledgement-per-machine),
[01 1.6](01-system-model.md#16-state-machines),
[01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)).

`REQ-0172` Validation rules live as data in `internal/spec/rules.go`: each rule
names a field path, its constraint (pattern, length, enum, reserved
values, uniqueness scope), a message, an error id and its scope (both
validators, or romeu only). From the Go types and those rules,
`go generate` derives the validators, the rule table in the reference
docs and the JSON Schemas (`tools/schemagen`, an in-repo reflect-based
generator; no dependency), so types, validators, docs and schemas
cannot drift. The tables below are the v1 design input to `rules.go`
and the struct tags. `REQ-0173` The pull request that lands a format's generator
replaces that format's field and rule tables here by a link to its
generated page under `docs/reference/`; this page then keeps only the intent,
the annotated example and the rules that are design decisions.

A table cell holds one obligation and at most three sentences; what does
not fit becomes a numbered list under the table, one item per
obligation, and the cell cites the list.

## 3.1 Name and path rules (shared)

`julieta spec validate` in the config repo's CI cannot see host
settings, so the rules that read them are checked by romeu only; `REQ-0174` the
"Checked by" column says which, and `spec validate` prints one line
naming the host-only rules it did not check.

| Field | Rule | Checked by |
|---|---|---|
| project `name` | `REQ-0175` `^[a-z][a-z0-9-]{0,38}[a-z0-9]$`; unique across the config repo; equals the file stem (satisfies sbx's <= 63 chars, no trailing `-` or `.`) | both |
| repo `dir` | `REQ-0176` `^[a-z0-9][a-z0-9._-]{0,62}$`, not `.` or `..`, no `/`, not a name romeu reserves inside `<name>-env/`: `memory`, `review`, `ledger`, `.romeu`, `sbxenv.yaml`, `.git` (compared case-insensitively); unique within the project, also case-insensitively. This row is the one list of reserved names | both |
| repo `url` | `REQ-0177` `https://<host>/<owner>/<repo>[.git]` only: v1 supports GitHub-shaped URLs (one owner segment, so no GitLab subgroups), and repos, CI and releases assume GitHub; a URL may appear in at most one project, compared after one normalization: host and path lowercased, a trailing `.git` removed | both |
| `projects/` | `REQ-0178` holds `*.yaml` files only; a file of another name or extension is an error naming the path (`RJ-345`) | both |
| the config project | `REQ-0179` exactly one spec's primary repo URL equals `config.url` of host settings; none is an error naming `config.url` (`RJ-346`) | romeu only |
| repo `url` host | `REQ-0180` `<host>` must be a host-settings `gitHosts` entry | romeu only |
| repo `ref` | `REQ-0181` a branch name valid under `git check-ref-format --branch` rules, implemented in Go; `@`, which `--branch` reads as the current branch, is refused (narrowest form, decided on 2026-10-22) | both |
| kit id | `REQ-0182` `^[a-z0-9][a-z0-9-]{0,62}$` | both |
| kit `args` | `REQ-0183` key `^[a-z][a-zA-Z0-9_]{0,31}$`; value printable UTF-8, no newline, <= 1 KiB | both |
| secret name | `REQ-0184` `^[a-z0-9][a-z0-9_-]{0,62}$` | both |
| secret binding | `REQ-0185` a `name@project` entry in host settings for every secret name of a project `sync` renders (3.4; checked at [04 4.2](04-cli.md#how-romeu-sync-renders-gates-and-promotes), sync step 3) | romeu only |
| run layout pane id / tab label | `REQ-0186` `^[a-z0-9][a-z0-9_-]{0,31}$` | both |
| relative `cwd` in the run layout | `REQ-0187` cleaned path, no leading `/`, no `..` element | both |
| egress domain (every source of [07 7.5](07-mise-egress.md#75-egress-derivation-internalegress): `egress.extra`, catalog, lock-derived, kit-declared) | `REQ-0188` lowercase hostname, IDNA A-labels, no wildcard, no port, no IP literal, no label of `0x` and hex digits; not under a special-use name a resolver may map to the host or its local network: the last labels `localhost`, `test` and `invalid` (RFC 6761), `local` (RFC 6762), `onion` (RFC 7686), `internal` (ICANN's private-use reservation), `localdomain`, `lan`, `home`, `corp`, `intranet` and `private` (names networks use with no registration), and the suffix `home.arpa` (RFC 8375); `example` is not refused (decided on 2026-10-22) | both |
| git host (`gitHosts[].host` of host settings, and the host of a repo `url` and of `config.url`) | `REQ-1268` the same hostname form as an egress domain (lowercase, IDNA A-labels, at least two labels, no port, no IP literal, no label of `0x` and hex digits) but without the special-use ban, so a self-hosted git server at `git.corp.internal` or `git.corp.local` is valid; an egress domain keeps the strict rule (decided on 2026-10-22) | both for a repo `url`, romeu only for `gitHosts` |
| workload reference | `REQ-0189` `<registry>/<repository>@sha256:<64 hex>` | both |
| workload repository | `REQ-0190` `<registry>/<repository>` must be in host settings `workloadRepositories` | romeu only |
| `agent` | `REQ-1259` `^[a-z0-9][a-z0-9._-]{0,62}$`: sbx takes a built-in agent's name or a kit's, and publishes no pattern; narrowest form, decided on 2026-10-22 | both |
| `sandboxOptions.cpus` | `REQ-1260` a whole number from 0 to 256, 0 being sbx's default (the host's); that the host has that many CPUs is checked at run time. Source: [sbx environment files](https://docs.docker.com/ai/sandboxes/configuration/environment-files/), `sandboxOptions` | both |
| `sandboxOptions.memory` | `REQ-1261` a whole number followed by `m` or `g`, binary units, at least `512m`; the host's maximum is checked at run time. Source: [sbx create](https://docs.docker.com/reference/cli/sbx/create/), `--memory` | both |
| `ports[]` | `REQ-1262` `[[HOST_IP:]HOST_PORT:]SANDBOX_PORT[/PROTOCOL]`, protocols `tcp`, `tcp4`, `tcp6`, `udp`, `udp4`, `udp6`, each port from 1 to 65535 with no leading zero; `HOST_IP` only `127.0.0.1`, because another address would expose the sandbox's port to the network; none is loopback, as in sbx. Deferred: the IPv6 loopback as `HOST_IP`; trigger: the IPv6 host-IP form measured against a real sbx (T029), when the row widens (decided on 2026-10-22). Source: [sbx ports](https://docs.docker.com/reference/cli/sbx/ports/) | both |
| `salvage.excludeIgnored[]` | `REQ-1263` a clean relative path, an optional trailing `/`, no `..` element, no leading `-`; narrowest form, decided on 2026-10-22 | both |
| `egress.tools` key | `REQ-1264` `<backend>:<tool>` as `mise.lock` writes it, `^[a-z0-9][a-z0-9-]*:[A-Za-z0-9@][A-Za-z0-9._/@-]*$`; narrowest form, decided on 2026-10-22 | both |
| run layout step | `REQ-1265` no argv element holds a control character, and `argv[0]` is not empty and does not start with `-`; a pane's `env` value holds no control character; narrowest form, decided on 2026-10-22 | both |
| host settings `secrets{}.service` | `REQ-1266` the secret-name pattern of this table; narrowest form, decided on 2026-10-22 | romeu only |
| host settings `tools.gitCredentialHelper` | `REQ-1267` a `git-credential-<name>` helper's name, `^[a-z0-9][a-z0-9-]{0,62}$`, or an absolute clean path of letters, digits and `._/+-` only, since git runs it through `sh`; narrowest form, decided on 2026-10-22. It answers the host of `config.url` only (05 5.1, REQ-0464), and an absolute path names a file only the user or root can write (04 4.2, `romeu init`), decided on 2026-10-23 | romeu only |

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
memory:
  remote: https://github.com/example/shop-memory   # the private repository that keeps <name>-env/memory/ (08 8.1); `none` opts out
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
points at a readable tag URL, which a moved tag could change. In a file
under `examples/`, the tag is that of the release that ships it, the
one `VERSION` names ([02 2.1](02-layouts.md#21-product-repo-romeu-e-julieta-public)),
as `v` and the version.

`REQ-0191` The classification is the struct tags on the spec types
(`gate:"widening"`, `apply:"recreate|live"`, 01 1.4). The field table,
with each field's tags, and the rule table are generated in
[docs/reference/project.md](../reference/project.md), from the types
and `rules.go`, beside [schemas/project.v1.json](../../schemas/project.v1.json).
What the tags decide:

- `ref` decides which commit egress derives from, so it is widening
  with the rest of `repos`.
- `workload` and `kits` are widening with their capabilities.
- `secrets` applies at recreate; it becomes `live` if probe A10
  confirms it (Q18).
- `salvage.excludeIgnored` is widening, since it defines acknowledged
  loss.
- `REQ-0192` `sandboxOptions` (`cpus` and `memory` only) is ungated and
  applies at recreate: `status` reports recreate needed, it is stripped
  from `renderDigest`, and it is on the list of deliberately ungated
  fields (01 1.4), an accepted risk of
  [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1).
- `run` is ungated and applies live: pane commands run only inside the
  sandbox, and it is on the same list.

`REQ-0193` There is no field for a host command, an absolute path, a mount, or an
env var for the sandbox as a whole or for sbx; a pane's `env` reaches
that pane's steps only, which is why `PATH`, `HOME`, `LD_*`, `BASH_ENV`
and `ENV` are refused there. A `command`, `argv` or `env` key under
`secrets` is a decode error.

### Run layout (`run`)

The run layout is romeu's own format, which julieta renders for herdr,
the one renderer; a second renderer is a row of
[Deferred decisions](../spec.md#in-the-product).

The fields of `run` (`tabs[]`, a binary tree of nodes per tab, and
`panes[]`) and their rules are in the field and rule tables of
[docs/reference/project.md](../reference/project.md).

`REQ-0196` Execution rule: every step runs as `mise exec -C <pane dir> -- <argv>`;
steps run in order; each must exit 0 before the next; the last step is
exec'd; a failing step prints its exit status and drops to a login shell
in the pane dir. No steps = login shell. `REQ-0197` Every pane id appears exactly
once across all tabs.

*Why for us:* mise daemons add `ready`/`deps`, which is
supervisor territory; a layout-and-steps list is what we need, and a
later version could delegate readiness to mise daemons.

## 3.3 Rendered `sbxenv.yaml` (sbx env file, `schemaVersion: "1"`)

`REQ-0198` Produced only by `internal/render` through the YAML marshaller. The key
names below follow the documented env-file schema and the documented v3
workload pattern; `REQ-0199` the golden files are **probe-confirmed**: probe A5
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
  - ./memory/_shared
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

`REQ-0200` Rules: no `x-*` keys (metadata lives in `render.json`); no `lifecycle`
hooks (they re-prompt on every invocation and `preRemove` only warns);
`REQ-0201` the secret command is built with `internal/shquote` from host-settings
argv with a scrubbed environment; romeu only ever renders `command`,
never a literal value; `REQ-0202` clone mode comes from romeu's `sbx env run
--clone`. `./.romeu/bin` is a fixed mount, rendered for each project
from no spec field. `REQ-0203` romeu names this file by path; probe A5 records
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
  gitCredentialHelper: osxkeychain      # optional; a git-credential-<name> helper on git's exec-path or /usr/bin:/bin, or an absolute path; hardened git uses this helper and no other, for the host of config.url only
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

The field table and the rule table are generated in
[docs/reference/host-settings.md](../reference/host-settings.md), from
the types and `rules.go`, beside
[schemas/host-settings.v1.json](../../schemas/host-settings.v1.json).

`REQ-0204` Values you set: `romeu init` writes `root`, `config` and `tools`; the
operator fills `gitHosts`, `workloadRepositories`, `signing` when a
project uses `git-ssh-sign`, and `secrets`
([J1](09-journeys.md#j1-onboarding-new-machine-first-time) step 5).

`REQ-0205` Rules: every `argv[0]` is absolute and exists; no shell strings; an argv
is a command that fetches the value and never carries it. `REQ-0206` An
argv element that holds, at its start or after whitespace or `=` (so
`--token=<t>` and `Bearer <t>` too), `ghp_`, `gho_`, `ghs_`, `ghu_`,
`ghr_` or `github_pat_`, or 40 hex digits of either case that end the
element or a word, is refused with the error
`secret-in-argv` (`RJ-314`; a best-effort guard against the common token
shapes, not a detector); a spec secret name of a target without a matching
`name@project` entry is exit 2 naming the key; `REQ-0207` a `name@other-project` entry never
satisfies a different project; entries for unknown projects are
reported by `doctor`. `REQ-0208` A secret argv and the credential helper run with
exactly `HOME` and `PATH=/usr/bin:/bin`, so a manager must work from
`HOME` alone (Keychain, or a CLI whose session lives in a file under
`HOME`); a manager that needs a session variable is not supported in
v1. `REQ-0209` `doctor` runs each secret argv once in that environment and fails
on a non-zero exit or empty output; it never prints, keeps or writes
the value. `REQ-0210` There is no registry credential: descriptors are read
anonymously, and a credential for a private workload registry is a row
of [Deferred decisions](../spec.md#in-the-product). *Why for us:*
devcontainer `initializeCommand` runs repo-controlled commands on the
host; names in the spec and commands in host settings remove that class.

## 3.5 Egress catalog (`internal/catalog/egress/*.yaml`, schema `catalog.v1`)

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

`REQ-0211` One list per tool: because tools can be reinstalled from inside a
running sandbox, install and use domains are both needed at runtime, so
the distinction has no consumer. The catalog is embedded in both romeu
and julieta. `REQ-0212` CI requires every domain used anywhere to have an explicit
`domains` entry. The entries above are examples; the v1 catalog was
built from the reference config repo's locks (S4) and measured in probe
B4. The `meta` and `base` sets live in
`internal/catalog/egress/base.yaml`; a domain declared by two files with
different entries is a decode error. `REQ-0214` A tool outside it is declared with
`egress.tools` in the project spec (always gated) until a catalog entry
ships in a release.

## 3.6 julieta manifest (schema `manifest.v1`)

`REQ-0215` Passed by romeu on every `sbx env exec` as
`--env JULIETA_MANIFEST=<base64url(canonical JSON)>`, cached by julieta
at `$HOME/.local/state/julieta/manifest.json`. `REQ-0216` romeu checks the encoded
size at `sync`, before the gate, and refuses a manifest over 32 KiB with
the error `manifest-size` (`RJ-313`); `julieta spec validate` reports
the same rule. `REQ-0217` The Go type lives in `internal/manifest`, the one
package both binaries import for it. `REQ-1257` julieta's reader skips a
member of `manifest.v1` it does not know, at any depth, as the other
readers between binaries do (the opening of this page; ADR 0041), and
keeps every other rule: a duplicate member, the size cap, the canonical
form of the members it knows, and the protocol window below. The one
exception is the must-understand rule: a member that an older julieta
could not safely ignore, one that restricts, denies or changes what
julieta must do, is not added as an optional member; it bumps the
protocol number (`protocol`, below), and the window refuses a julieta
that is too old. Today julieta acts on `repos[].path` and
`repos[].memory` (where it clones, fetches and writes), `sharedMemory`
(the project's shared memory dir, 08 8.1; an optional member and no
protocol bump, since a julieta that does not know it loses a feature and
breaks nothing: it keeps writing the handoff files of before, which the
host warns about and never refuses, 08 8.1), `repos[].dir`
(names, locks and work directories), `repos[].url`, `repos[].primary`,
`repos[].base`, `generation`, `agent` and `project`, so a new member
that restricts or gates one of those uses, and any new behaviour
julieta must enforce, bumps the protocol. It only reads or prints
`egress` (romeu enforces it on the host), `specCommit`, `run`,
`runDigest`, `repos[].ref`, `salvage` and `salvageRun`; a feature that
makes julieta honor one of these is a new behaviour in this sense. A
member julieta may safely ignore (an informational one, or an addition
that a newer julieta uses for a new feature) is added without a bump.
`internal/manifest` holds a test of the shape of `Manifest`, so adding,
removing or retyping a member fails it until the author updates it and
says in the pull request which kind the member is.

```json
{
  "schema": "manifest/v1",
  "protocol": 1,
  "project": "shop",
  "specCommit": "<sha>",
  "generation": "01j9zc2q8x7w4t6y3r5e1v9k0p",
  "sharedMemory": "/abs/root/shop-env/memory/_shared",
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

The field table is generated in
[docs/reference/manifest.md](../reference/manifest.md), from the Go
types of `internal/manifest`. The rules of its fields that are design
decisions:

1. `REQ-0218` `protocol` is the romeu/julieta contract number;
   `REQ-0219` julieta accepts a manifest of protocol N (its own) or N-1
   (a cached manifest written before a romeu upgrade) and refuses
   anything else with the error `manifest-protocol` (`RJ-342`, exit 2).
2. `REQ-0220` `specCommit` is the config repo commit of the live spec,
   fresh at each exec; the agent compares it with the commit it pushed
   (01 1.2).
3. `REQ-0221` `repos[].path` is the primary's host path (clone mode
   keeps it); a secondary's is `<home>/src/<project>/<dir>`, where
   `<home>` is the workload user's home, measured by probe A4 and
   stored with the workload pin in `kits/pins.yaml`.
4. `REQ-0222` `repos[].base` are origin SHAs the host clone already
   has, each held by the create-only ref
   `refs/romeu/base/<name>/<generation>/<dir>`
   ([02 2.3](02-layouts.md#23-host-tree)); snapshot and salvage bundles
   exclude objects reachable from them (08).
5. `REQ-0223` `salvageRun` is present only on salvage calls: the
   salvage id romeu expects back.
6. `REQ-0224` `egress` holds the domains romeu applied to this sandbox,
   `auto` plus approved `gated`
   ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress));
   the pending gated set lives on the host and is not carried.
   `julieta status` prints it.

`REQ-0225` The compatibility check that precedes every romeu command that execs
julieta, and in which romeu accepts julieta protocols N and N-1 the way
julieta accepts manifests, is described in
[06 6.3](06-kits.md#63-julieta-delivery).

## 3.7 `render.json` (in `<name>-env/.romeu/`, schema `render.v1`)

```json
{
  "schema": "render/v1",
  "romeuVersion": "v1.0.0",
  "specSource": {"kind": "origin", "ref": "refs/romeu/origin/my-config/main", "commit": "<sha>"},
  "repos": [{"dir": "shop-api", "url": "...", "egressRef": "refs/romeu/origin/shop-api/main", "egressCommit": "<sha>", "lockSha256": "<hex>"}],
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

`REQ-0226` `promoting` is omitted except inside a promotion commit
([01 1.6](01-system-model.md#promotion-commit)), when it is
`{"candidateId": "<ULID>", "filesDigest": "<hex>", "previousFiles": {...}}`.
`REQ-1273` A member that has no value is omitted and never written as
`null`: `promoting` outside a promotion commit, and `lockSha256` of a
repo with no `mise.lock`. The schema generator admits no `null` for a
pointer.
`REQ-0227` `previousFiles` is the `files` map before the promotion, which
recovery writes back when the candidate no longer verifies. The julieta
binary digest here is what the compatibility check compares against.

## 3.8 Host state (schemas `state-*.v1`)

`REQ-0228` `toolchain.json` (gate 1):

```json
{
  "schema": "state-toolchain/v1",
  "sbxVersion": "0.49.2",
  "romeu": {"version": "v1.0.0", "sha256": "<hex>"},
  "julieta": {"version": "v1.0.0", "protocol": 1, "sha256": "<hex>"},
  "catalogDigest": "<hex>",
  "catalog": {"...": "the acknowledged catalog, decoded, so the next gate 1 can print a diff"},
  "acknowledgedAt": "2026-10-02T09:00:00Z"
}
```

`REQ-0229` `projects/<name>.json` (one record per project; hosts the generation
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

A member that holds nothing may be absent from the file: an empty list
(`generations`, `egressApplied`, `salvage`, `reasons`, `acceptedLoss`), an
absent object (`approval`, `retiring`) and an absent time (`lastRunAt`,
`closedAt`), and so may `createdWith` and `fingerprint`, where the example
above writes `null` or `[]`. romeu writes them absent, and a reader accepts
both forms.

| Field | Values |
|---|---|
| `retiring` | `REQ-0230` `null`, or `{"attic": "<UTC-ts>"}` while the retire steps of 01 1.6 run |
| `generations[]` | `REQ-0231` every generation of the project; v1 keeps them all, so the record grows by one generation per recreate and has no bound. `doctor` warns past 256 KiB, and compaction is a row of [Deferred decisions](../spec.md#in-the-product) |
| `generations[].state` | `REQ-0232` `open`, `salvaging`, `removing`, `closed-removed`, `closed-lost` |
| `generations[].createdWith` | `REQ-0233` the recreate digest at create; `null` for an adopted generation (status reports "recreate digest unknown") |
| `salvage[].manifestSha256` | `REQ-0234` plain sha256 of the salvage `manifest.json` bytes (3.12, raw) |
| `salvage[].id` | `REQ-0235` a ULID, as the generation's id is ([01 1.6](01-system-model.md#generation)); 01 1.6 appends one entry per salvage, so the newest entry is the last of `salvage[]` |
| `salvage[].fingerprint` | `REQ-0236` `null`, or, written in the same whole-file write as the state `removing` (01 1.6), the `fingerprint` digest (3.12) of the values the list **Fingerprint** after this table names, not read again at that write. That list is the one home of what a fingerprint is; the pages that compare one cite it. A field of the state format, under the version rule of the opening of this page |
| `salvage[].result` | `REQ-0237` `complete`, `incomplete`, `lost`; `reasons[]` lists each skipped item, `sandbox-half-failed`, or `sandbox-lost`, and for a generation `run` closed as lost, `dirty-at-last-stamp:<n>` and `stashes-at-last-stamp:<n>` from the newest stamp (03 3.10; [04 4.2](04-cli.md#how-romeu-run-reaches-the-run-layout), run step 2); `acceptedLoss[]` lists the reasons `--accept-loss` named |

**Fingerprint.** What `salvage[].fingerprint` digests, and what its
reader learns:

1. The values that steps 2 and 4 of
   [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)
   and the host verification read, the sandbox-side ones taken from the
   fingerprint object in the `--json` document of `julieta salvage`
   ([04 4.3](04-cli.md#43-julieta-sandbox)), the same object
   `--fingerprint` prints: the daemon heads, each worktree's HEAD and a
   digest of its status, which holds tracked changes and untracked
   paths, each with the digest of its file, and ignored paths, minus
   `salvage.excludeIgnored`, each by path, size and modification time;
   for a worktree that step 2 read more than once, the last read.
2. How the rerun reads and compares it, and how it ends, is the `RJ-338`
   row of [04 4.4](04-cli.md#44-error-ids).
3. `REQ-0238` The details of `RJ-338` name the kind of entry that changed, as far
   as the manifests ([3.11](#311-salvage-manifest-schema-salvagev1), one
   per repo) of the last entry tell it: a daemon head or a worktree's
   HEAD whose fresh value differs from the one the manifest holds (the
   daemon heads are among the primary repo's `refs[]`, each worktree's
   HEAD in `worktrees[]`), and otherwise a worktree's status or an
   ignored path, which the one stored sha256 does not tell apart
   (decided on 2026-10-12: the details name the kind, not a path).

`REQ-0239` Every file the product writes in place is written as a temp file in the
same dir, fsync of the file, rename, fsync of the dir; host state files
have mode 0600. `REQ-0240` `descriptors/<digest>.json` holds the parsed, normalized
capability set of one workload or kit descriptor; entries are immutable.
A workload's descriptor is keyed by the digest of the platform manifest
its annotation was read from, which is the digest `internal/oci`
verified last, and the record also holds the digest of the workload
reference (the index) and the platform it was chosen for; a local
kit's is keyed by the sha256 of its descriptor file, with neither. The
record's `capabilities` member is required: a record without it, or
with `null`, is unreadable and is never read as an empty set. When a
second index lists a manifest already cached, the entry keeps the first
index.

## 3.9 Memory entry (schema `memory-entry.v1`)

`REQ-0241` One file per entry: `memory/<dir>/entries/<ULID>.md`. julieta stamps
`id`, `createdAt` and `updatedAt` on `add` and `edit`; the agent
supplies only `title`, `body`, `tags` and `status` (a supplied id or
timestamp is an error).

```markdown
---
schema: memory-entry/v1
id: 01j9z3k8m6t2q4w5e7r9y1v3k5
title: Flaky integration test on arm64
status: open            # open | done
tags: [ci, feedback]    # one type tag (user, feedback, project, reference, handoff) plus free tags, sorted
createdAt: 2026-10-02T09:20:00Z
updatedAt: 2026-10-02T09:20:00Z
---
Free-form markdown body.
```

`REQ-0242` Tags with behaviour are a closed set, the five type tags. Four
are the kinds of note the pinned agent's own memory records, used in
the same cases: `user` (the operator's role, expertise and working
preferences), `feedback` (the session's own feedback on how the session
itself is going: friction, a tool that misbehaved, a gap in a check, a
process that slowed the work, written so that another session can triage
it and implement the improvement in parallel; SessionStart lists the
open entries of every type (08 8.1, REQ-0659), so that session sees it.
It also holds a correction the operator gave or an approach the operator
confirmed), `project` (ongoing
work, decisions and ordering that the code and the git history do not
hold) and `reference` (where to find something outside the repository).
The fifth, `handoff`, is the narrative a session leaves for the next
(08 8.3); `tombstone`, which only a `handoff` carries, is the one free
tag with a reading (below). `REQ-1287` Every entry carries exactly one type tag, as every
note of the agent's own memory carries its kind: `add` without one,
`add` or `edit` with two, and an `edit` that would leave none, is a
usage error (exit 2) that names the five. `REQ-1292` An entry file written before this rule,
whose tags hold none, is read as it is: listed with the type `-`,
matched by `list --tag` on its free tags and never on a type tag, and
typed by the first `edit` that gives it a type tag; no entry file is
rewritten for the rule.

`REQ-1295` An entry is filed in the memory dir of one repo, or, with
`--shared`, in the project's shared dir `_shared/` (08 8.1), for what
holds for the project as a whole; the index of 08 8.2 shows `shared` in
place of the repo; `--shared` with `--repo` is a usage error. `REQ-1302`
A manifest without `sharedMemory` (3.6) makes `--shared` and a `handoff`
a usage error (exit 2) naming the member, so nothing is filed in the
wrong place, and makes `julieta stamp` and `memory index --hook` warn
`shared-memory-missing` and exit 0, since a hook must not fail the
session. `REQ-1296` A `handoff` entry is always shared: `add`
files it there without the flag, and `--repo` with it is a usage error.
`REQ-1297` A `handoff` body holds the headings the `handoff` skill names,
`## Goal`, `## State`, `## Decisions`, `## Next steps` and `## Open
questions`, in that order, and `## What would be lost` last when the
entry also carries the tag `tombstone`, the handoff written before the
sandbox is destroyed (08 8.5); `add` and `edit` refuse a body without
them (exit 2), and `tombstone` on any other type is refused the same
way. `REQ-1298` A `handoff` entry's front matter carries one more key,
`stamp`, the stamp of 3.10 julieta measures at `add`: a caller cannot
supply it, `edit` leaves it, and the key is refused on any other type.
`REQ-1299` One `handoff` is open at a time: `add` of a `handoff` first
closes the open one of the shared dir (status `done`), after the
duplicate rule of 04 4.1 (a duplicate prints the existing id and closes
nothing); an open `handoff` is edited as any entry, and a done one
refuses every `edit`, so a closed narrative is the record of its moment. `REQ-0243` An entry file is self-contained: copying `memory/<dir>/entries/`
whole is a complete move, and the store lock is recreated on first use.

## 3.10 Stamp (schema `stamp.v1`)

`REQ-0244` A stamp is the position of every manifest repo at one moment,
measured by julieta and never written by an agent: for each repo its
branch, head, upstream, the commits ahead of and behind it, the dirty
count defined below and the stashes, with the project, the generation,
the agent and the time. `REQ-1300` It is written in two places by the
same code: inside a `handoff` entry, under the `stamp` key of its front
matter, when the entry is added (3.9), and as a file
`memory/_shared/stamps/<ULID>.yaml` by `julieta stamp`, the SessionEnd
hook (08 8.2), so the newest file says where the trees were when the
last session ended and the entry says where they were when the
narrative was written; the gap between the two is the resume line of
08 8.3. There is no narrative in a stamp file and no pointer file.

```yaml
schema: stamp/v1
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
```

`dirty` is the number of distinct paths the working tree changes:
staged, unstaged, unmerged, untracked, or marked assume-unchanged. Each
path counts once, a rename counts as two, an untracked directory that
holds no tracked file counts once, and a skip-worktree entry does not
count, so a clean sparse checkout reads 0. The count stops at 100,000:
a working tree that changes more paths reads 100,000.

## 3.11 Salvage manifest (schema `salvage.v1`)

`REQ-0246` `memory/<dir>/salvage/<generation>/<salvage-id>/manifest.json`:
`REQ-0247` `salvageId` (must equal the id romeu passed; `REQ-0248` another is the error
`salvage-id-mismatch`, `RJ-339`), `agentStop` (`matched`, the number of
processes signalled; `killed`, the number that needed SIGKILL; absent
without `--stop-agents`), `generation`, `repo`, `worktrees[]` (path,
HEAD, branch or detached, dirty), `refs[]` (every ref in the bundles
with SHA: branches, the current generation's
`refs/salvage/<generation>/*`, the reflog-only commits as
`refs/salvage-reflog/<branch>/<n>`, tags, notes, each worktree HEAD),
`bundles[]` (file, sha256, size, embedded-repo path if any), `files[]`
(path, size, sha256), `skipped[]` (item, reason: `excluded-by-spec`,
`over-cap`, `transcript-excluded`, `unreadable`, `disk-full`,
`worktree-changing`, `in-progress-operation`, `agent-not-matched`,
`agent-path-missing`; `REQ-0249` the host half adds `fsck-failed`,
`daemon-url-refused`, `repo-missing` (a repo of the generation's
recorded set without a manifest), `daemon-heads-missing` (a daemon head
absent from the bundle refs or with another SHA) and `payload-mismatch`
(a file whose size or sha256 differs from the manifest); `REQ-0250` a `salvageId`
other than the id romeu passed is not a reason but the error
`salvage-id-mismatch` (`RJ-339`, exit 5), raised by the host half's step
1, which `--accept-loss` does not cover to the salvage record's
`reasons[]`, 3.8; `REQ-0251` the recovery of each is the table of
[08 8.5](08-memory-handoff-salvage.md#skipped-reasons-and-their-recovery))
and `complete`. romeu recomputes completeness itself; the field is
advisory. `REQ-0252` Every path (`bundles[].file`, `files[].path`,
`worktrees[].path`, an embedded-repo path) is relative, cleaned, has no
`..` element and no symlink component, and resolves below the salvage
dir (the error `salvage-path`, `RJ-307`); `REQ-0253` the total bytes romeu verifies
are bounded by `salvage.capBytes`, and a manifest over it makes the
salvage incomplete with reason `over-cap`.

## 3.12 Digests (`internal/canon`)

`REQ-0254` Structured values are digested with one function,
`canon.Digest(kind, v)`: sha256 over `"romeu/<kind>/v1"`, a NUL byte,
and the canonical JSON of `v`: what `encoding/json/v2` writes for `v`,
nil lists and maps as null, in the form of RFC 8785, the JSON
Canonicalization Scheme (keys sorted by their UTF-16 code units,
minimal string escapes, with U+2028 and U+2029 written as they are,
UTF-8, no insignificant whitespace). A number with a fraction or an
exponent takes its float64 form in ECMAScript notation, while an
integer keeps its digits, so the form equals RFC 8785 for every I-JSON
value and differs only for an integer beyond 2^53. A value the writer
cannot write is refused: NaN, an infinity, a number out of the float64
range or one a float64 flushes to zero, a channel, a `time.Duration`. A
duplicate key is refused, and a string or key that is not UTF-8, or
that holds U+FFFD, the mark the writer leaves of such text, is refused,
so two strings never share a digest. The domain stays
`"romeu/<kind>/v1"` through this form, decided on 2026-10-20, because
no release had written a digest. `REQ-0255` Each Go type has exactly one kind, so a value of one kind
can never be compared with a digest of another: the kind is a typed
constant named after the kind in Go case (for example `canon.Widening`,
`canon.KitTree`), the value's type fixes its one
kind, and the compiler checks it. `REQ-0256` Raw file bytes
(`sbxenv.yaml`, binaries, bundles, payload files, probe artifacts,
salvage manifests as `manifestSha256`) use plain sha256, because
external tools (`sha256sum`, `shasum`) must reproduce them.

`REQ-0257` The kinds table is closed for v1 and lands whole with `internal/canon`;
a new kind is a change to this table first. `REQ-0258` Every ULID the product
writes is lowercase Crockford base32 (the reason of
[13 13.3](13-runtime-ledger.md#133-spool) holds for every file on the
host disk: two names that differ only in case collide on APFS), and
decoders refuse uppercase.

| Kind | Input | Used by |
|---|---|---|
| `widening` | the widening set | gate 2 |
| `recreate` | every field tagged `apply:"recreate"`: a gated one in its resolved form, the member of the widening set that holds it (so kit content, workload capabilities and secret argv count), and an ungated one (`sandboxOptions`) as the spec holds it; decided on 2026-10-23 | "recreate needed" |
| `render` | the normalized render (01 1.4) | the widening set's `renderDigest` |
| `egress` | the sorted effective domain union | `render.json` |
| `catalog` | the decoded catalog | gate 1 |
| `kit-tree` | sorted `{path, mode, sha256}` of a kit | widening set, kit dir name |
| `candidate-files` | sorted `{path, sha256}` of a candidate dir | promotion recovery |
| `run` | the validated run layout | stale layout detection |
| `lock-set` | sorted `{dir, mise.lock sha256}` | `julieta install` skip rule |
| `secret-argv` | one secret's argv | widening set |
| `memory-content` | `REQ-0259` an entry's `title`, `body`, `tags` and `status`, never its `stamp` | `julieta memory add` duplicate rule (04 4.1) |
| `fingerprint` | `REQ-0260` the fingerprint object of 3.8 | the `removing` write and its rerun |

## 3.13 Other schemas

`probe-result.v1` is defined in [11](11-host-probes.md#113-probe-result-format);
`acceptance.v1` in [10](10-testing-style.md#105-acceptance-evidence).
The runtime ledger's formats are designed in [13](13-runtime-ledger.md)
and are not in v1.

Formats without a schema, each with its reason:

| Format | Reason |
|---|---|
| `snapshot/heads.json` | `REQ-0261` read by romeu alone, decoded under the opening's rules (an unknown member is skipped, per the versions rule of the opening), and pinned by a golden |
| `.romeu/bin/SHA256SUMS` | `REQ-0262` the `sha256sum` line format, which external tools read |
| `e2e/scenarios/<journey>/commands.yaml` | read by one program, the scenario runner, and pinned by its test |
| `kits/pins.yaml` | written by `tools/kitpin` and read by the product's own consumers, pinned by their tests |
| `docs/grants.yaml` | read by one program, `tools/ci pr`, and pinned by its test; its form is in [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change) |
| `.github/ask-first.yaml` | `REQ-0263` `version: 1`; read by `tools/ci pr` and by `go generate`, pinned by their tests (05 5.3) |
| `tools/ci/denylist.yaml`, `prose.yaml`, `headings.yaml` | `REQ-0264` read by `tools/ci` alone, each pinned by its step's test |
| `docs/reference/vocabulary.yaml` | `REQ-0265` generated from 01 1.7 and read by `tools/ci vocabulary` |
| `e2e/testdata/sbx/<version>/*.jsonl` | `REQ-0266` written by the recorder and read by the fake sbx, pinned by their tests (10 10.3) |
