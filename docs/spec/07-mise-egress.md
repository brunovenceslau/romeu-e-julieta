# 7. mise integration and derived egress

Back to [index](../spec.md). Reader: implementers of `egress`,
`catalog` and julieta's mise driver. Type: reference with decision
records.

## 7.1 Roles

| Where | What mise does | Who runs it |
|---|---|---|
| sandbox, at create | install every repo's locked tools | romeu drives `sbx env exec ... -- julieta setup` right after `sbx env run -d`, before any pane or agent starts |
| sandbox, later | reinstall/add versions; relock | agent or developer via `julieta lock` + `julieta install` |
| host | **nothing**. romeu parses `mise.lock` (and checks `mise.toml` presence) as data at pinned commits | romeu (Go parser) |
| CI (product repo) | same `mise.lock` as the sandbox | GitHub Actions |

*Why for us (install after create, not in a kit):* a kit install step
runs before the clone exists and a startup step races the agent; romeu
owns ordering, so tools exist before the first pane. DevPod builds and
attaches in one step because its IDE attaches last; ours is a terminal
the agent starts in, so we split create from attach.

## 7.2 Layering

| mise layer | Our meaning | Source |
|---|---|---|
| global | julieta's base: only `mise` and `herdr` (installed by the kit, not by mise config) | `julieta` kit |
| project | **empty, decided by the maintainer**: one `mise.lock` per repo is the single source of tools. Tools belong to repos so one lockfile serves sandbox, CI and host; a tool list in the spec would have no lockfile and no per-arch checksums | - |
| repo | `mise.toml` + `mise.lock` in each repo | the repo |

Multi-repo, multi-version: julieta runs `mise install` in each repo
directory; two repos pinning `go 1.24` and `go 1.25` get both
(~240 MB disk, ~75 MB download per Go SDK); mise activates per directory
via shims and `mise exec -C`.

## 7.3 Lockfile rules

- `mise.lock` must list `linux-x64` and `linux-arm64`, the platforms a
  sandbox runs, and `julieta lock` passes those two. A repo whose CI
  runs on macOS adds those platforms in its own mise settings; `--check`
  requires only the two linux ones.
- `julieta install` runs `mise install` locked, in the environment of
  [04 4.3](04-cli.md#43-julieta-sandbox), so mise installs only what the
  lock names; a repo with `mise.toml` but no `mise.lock` fails with
  "run `julieta lock`".
- Backends whose artifacts mise cannot URL-lock (`go:`, `cargo:`,
  `npm:`, `pipx:`/`pypi:`, `ubi:`, `asdf:`, `core:rust`, vfox) are
  version-only; `julieta install` warns once per such tool
  (`version-only-tool`), and the catalog must still carry their domains.
- mise's monorepo mode is not used in v1; it is a row of
  [Deferred decisions](../spec.md#in-the-product).
- Skip rule: `julieta install` records the `lock-set` digest of all lock
  files after a success and does nothing when unchanged (`--force`
  overrides).
- Freshness is checked deterministically, not remembered: `julieta lock
  --check` (the lock parses with the pinned mise; every tool in
  `mise.toml` has a lock entry; every lockable entry covers the two
  linux platforms) runs in the git hook dispatcher's `pre-commit` when
  `mise.toml` or `mise.lock` is staged, and in the product repo's CI
  (`tools/ci mise`). It also warns on a lockable entry without a
  checksum, and looks up each `backend:tool` in the embedded catalog: an
  unknown one is a warning naming the two fixes of 7.5 step 2, at commit
  time rather than at the next `romeu sync`. It does not fail on it,
  since a spec `egress.tools` entry is a valid fix outside the repo.
- julieta trusts mise config only in the manifest's repo paths
  (`MISE_TRUSTED_CONFIG_PATHS`, 04 4.3), so a config file planted
  anywhere else in the sandbox is never read.

## 7.4 mise bootstrap and its own egress

| Item | Rule |
|---|---|
| Version | pinned in the `julieta` kit (`tools/kitpin mise <version>` rewrites version + both linux sha256 values from the release `SHASUMS256.txt`) |
| Verification | sha256 at kit build; minisign verification of `SHASUMS256.txt` is done by `tools/kitpin` at pin time (maintainer machine or CI), not in the sandbox, against mise's minisign public key committed in the repository (its path is named by the plan's kit pin task); `tools/kitpin` commits the signed `SHASUMS256.txt` and its signature next to the pin, so `tools/ci kits` verifies them again offline on every change to the pin file ([10 10.2](10-testing-style.md#102-ci)) |
| Update checks | off (04 4.3): no request to the version host |
| Tracking | off (04 4.3) |
| Versions host | kept on (`mise-versions.jdx.dev` serves version lists and attestation bundles); it is in the catalog `meta` set |
| GitHub rate limit | aqua/github downloads fall back to `api.github.com`; mise reads `GITHUB_TOKEN`. How sbx exposes the project's `github` secret (by name) as that variable is a fact probe B4 records, measuring behavior with and without it. Until the token narrowing row of the index's [Deferred decisions](../spec.md#in-how-this-repository-is-run) fires, that secret is the operator's own token ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1), ADR 0008), so this download path carries its reach |

## 7.5 Egress derivation (`internal/egress`)

Input per repo: `(url, egressCommit)`. Output: `auto` and `gated` sets.

1. Read `mise.lock` (and `mise.toml` for presence) at `egressCommit`,
   the commit of `refs/romeu/origin/<dir>/<ref>`, via `git cat-file`,
   under one size cap of 1 MiB that every repository blob romeu decodes
   shares. No lock with a `mise.toml` present: sync error ("run julieta
   lock"). No mise files: the repo contributes nothing. romeu reads the
   lock layout written by the mise version the `julieta` kit pins; a
   lock it cannot parse, or one with an unknown top-level key, is the
   sync error `lock-unparsed` naming the repo and the fix "relock with
   `julieta lock`", never a partial read. One fixture of the pinned
   layout backs the parser test.
2. For each tool entry take its `backend` (for example `core:go`,
   `aqua:jqlang/jq`) and look it up in the catalog:
   - found: add its domains;
   - not found and the spec has `egress.tools["<backend>"]`: add those
     domains to `gated`;
   - not found otherwise: sync error (exit 2, `catalog-unknown-tool`)
     naming the tool and the fix (a catalog PR or a spec `egress.tools`
     entry).
3. Add every artifact URL host recorded in the lock; hosts not already
   produced by the catalog go to `gated`. Each host must pass the
   egress domain rule of [03 3.1](03-formats.md#31-name-and-path-rules-shared);
   one that fails is the sync error `lock-host-invalid` naming the lock
   entry.
4. Add `meta` and `base` sets.
5. Add spec `egress.extra` to `gated`.
6. Add kit-declared network domains (from the normalized capability
   sets, [06](06-kits.md#61-decisions)).
7. Move every domain with `upload: true`, or without a `domains` entry,
   to `gated`.
8. `effectiveDigest` = `canon.Digest("egress", sorted union)`; recorded
   in `render.json`.

julieta's own network commands follow the same rule. The config
project's spec carries in `egress.extra` the registry hosts of its
`workloadRepositories` and `api.github.com`, which `julieta pin
workload` and `julieta pin check --workflows` read
([04 4.3](04-cli.md#43-julieta-sandbox)); they are gated like any extra
and approved once.

`egress.tools` and `egress.extra` live in `projects/*.yaml` in the
config repo, so how often a project needs them is read there; v1 adds
no counter, a row of [Deferred decisions](../spec.md#in-the-product).

Deliberate deviation: the lock, not `mise.toml`, is the input. The lock
names the exact backend and artifact hosts per platform at a pinned
blob; `mise.toml` short names need mise's registry to resolve, which
would mean running mise on the host.

Application (romeu): per-sandbox rules via
`sbx policy allow network --sandbox <name> <domain>` for `auto` plus
approved `gated`; removal via `sbx policy rm network --sandbox <name>
--resource <domain>` for domains romeu applied earlier and that left the set;
verification with `sbx policy check network --sandbox <name> <domain>`
(exact argv from the block A recordings).
Applied on create (before `julieta setup`), on every `run` (re-applies
after an `sbx reset`), and on `sync` for a running sandbox. The
manifest carries the applied set
([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)), and
`julieta status` prints it, so an agent can tell an unknown host from a
gated one.

Egress follows `ref`, so a tool added on a branch installs once the
change merges and `sync` applies it. Before that, runtime blocks
surface in sbx's native approval queue. julieta's install error names
the host, the sbx command that approves it once for this sandbox, that
the approval is lost at recreate, and the durable fix, a spec edit plus
the gate. Listing the native queue in `status` is a row of
[Deferred decisions](../spec.md#in-the-product).

*Why for us (catalog keyed on backend:tool):* a `github:` or `http:`
backend can point anywhere, so a tool name is not an identity; lock URL
hosts are exact, the catalog adds registries and redirect CDNs the lock
cannot know.

## 7.6 Catalog maintenance

- `catalog/egress.yaml` is embedded in romeu and julieta; a catalog
  change ships only in a release and shows up once per machine in gate 1,
  which prints the domain and upload-flag diff per affected project
  (01 1.4). A project sees gate 2 only when its gated egress changes.
- `julieta spec validate --catalog projects/*.yaml` fetches every
  project's repos' `mise.lock` at their `ref` and lists unknown
  `backend:tool` keys and lock hosts missing from the catalog. It exits
  6 on an unknown `backend:tool` only, the case 7.5 step 2 makes a sync
  error, and reports a missing lock host as a warning, since 7.5 step 3
  gates it. It runs in the config repo's CI from a pinned julieta
  release, so that CI is red until an unknown tool is resolved (7.5,
  step 2, names the two fixes).
- Every domain used anywhere has an explicit `upload` flag (CI check);
  multi-tenant object stores are `upload: true`.
