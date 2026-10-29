# 7. mise integration and derived egress

Back to [index](../spec.md). Reader: implementers of `egress`,
`catalog` and julieta's mise driver. Type: reference with decision
records.

## 7.1 Roles

| Where | What mise does | Who runs it |
|---|---|---|
| sandbox, at create | `REQ-0609` install every repo's locked tools | romeu drives `sbx env exec ... -- julieta setup` right after `sbx env run -d`, before any pane or agent starts |
| sandbox, later | reinstall/add versions; relock | agent or developer via `julieta lock` + `julieta install` |
| host | `REQ-0610` **nothing**. romeu parses `mise.lock` (and checks `mise.toml` presence) as data at pinned commits | romeu (Go parser) |
| CI (product repo) | same `mise.lock` as the sandbox | GitHub Actions |

*Why for us (install after create, not in a kit):* a kit install step
runs before the clone exists and a startup step races the agent; romeu
owns ordering, so tools exist before the first pane. DevPod builds and
attaches in one step because its IDE attaches last; ours is a terminal
the agent starts in, so we split create from attach.

## 7.2 Layering

| mise layer | Our meaning | Source |
|---|---|---|
| global | `REQ-0611` julieta's base: only `mise` and `herdr` (installed by the kit, not by mise config) | `julieta` kit |
| project | **empty**: one `mise.lock` per repo is the single source of tools. Tools belong to repos so one lockfile serves sandbox, CI and host; a tool list in the spec would have no lockfile and no per-arch checksums | - |
| repo | `mise.toml` + `mise.lock` in each repo | the repo |

`REQ-0612` Multi-repo, multi-version: julieta runs `mise install` in each repo
directory; two repos pinning `go 1.24` and `go 1.25` get both
(~240 MB disk, ~75 MB download per Go SDK); mise activates per directory
via shims and `mise exec -C`.

## 7.3 Lockfile rules

- `REQ-0613` `mise.lock` must list `linux-x64` and `linux-arm64`, the platforms a
  sandbox runs; `REQ-0614` `julieta lock` passes `--platform` with those two and,
  when `mise.toml` sets `lockfile_platforms`, the platforms it lists
  too, since an explicit `--platform` makes mise ignore that setting
  (measured at mise 2026.10.3); `--check` requires only the two linux
  ones.
- `REQ-0615` `julieta install` runs `mise install` locked, in the environment of
  [04 4.3](04-cli.md#43-julieta-sandbox), so mise installs only what the
  lock names; a repo with `mise.toml` but no `mise.lock` fails with the
  error `lock-missing` (`RJ-340`) and the hint "run `julieta lock`".
- `REQ-0616` Backends the pinned mise exempts from the lock's URL check (at
  2026.10.3: `asdf`, `cargo`, `gem`, `go`, `npm`, `pypi`/`pipx`, `ubi`,
  `core:dotnet`, `core:rust`, `core:swift` and vfox plugins, read from
  its lock documentation at that tag) are version-only; `REQ-0617` `julieta
  install` warns once per such tool (`version-only-tool`), the catalog
  must still carry their domains, and a mise pin bump reads the list
  again, named in the pull request's Evidence.
- mise's monorepo mode is not used in v1; it is a row of
  [Deferred decisions](../spec.md#in-the-product).
- `REQ-0618` Skip rule: `julieta install` records the `lock-set` digest of all lock
  files after a success and does nothing when unchanged (`--force`
  overrides).
- `REQ-0619` Freshness is checked deterministically, not remembered: `julieta lock
  --check` (the lock parses with the pinned layout, `lockfile_version`
  3; every tool in `mise.toml` has a lock entry; every lockable entry
  covers the two linux platforms) runs in the git hook dispatcher's
  `pre-commit` when `mise.toml`, `mise.lock` or the exceptions file is
  staged, on what the commit records, and in the product repo's CI
  (`tools/ci mise`); `julieta install` runs the same check on each repo
  before it installs and refuses on its findings. `REQ-0620` Every
  platform of a lockable entry also holds a checksum of the form
  `sha256:` and 64 lowercase hexadecimal digits: a lockable entry
  without one is the finding `lock-no-checksum` and one of another
  form (`md5:zz`, `sha256:` alone) the finding `lock-bad-checksum`,
  each exit 6, not a warning (decided on 2026-10-22: a lock without
  checksums names what mise downloads and proves nothing about it). A
  lockable entry is one whose tool is not version-only, judged by the
  key of `mise.toml`, never by the backend the lock claims. `lock
  --check` looks up each `backend:tool` in the embedded catalog: an
  unknown one is the warning `lock-unknown-tool`, naming the two fixes of
  7.5 step 2, at commit time rather than at the next `romeu sync`. It
  does not fail on it, since a spec `egress.tools` entry is a valid fix
  outside the repo.
- `REQ-1271` **The exceptions file.** A tool whose vendor publishes no
  checksum, so that mise cannot record one, is excused from the missing
  checksum by a line "`<key of mise.toml> <reason>`" in
  `.julieta/lock-exceptions` at the top of its repo (decided on
  2026-10-22). The file is the narrowest place the specification can
  hold: it is tracked, so the exception is reviewed in the diff that adds
  it; it is per tool, so one URL-only tool excuses no other; and it is
  at hand where the check runs, in the pre-commit action and in CI,
  where the project spec, which lives in the config repo, is not. It
  excuses a missing checksum only: a checksum of the wrong form, a
  missing platform and a missing entry stay findings, and a reason is
  required. The file stays a file of the repo (decided on 2026-10-22):
  its format is one line for each tool, the key of `mise.toml`, one
  space and a reason, with blank lines and lines that start with `#`
  left out, a tool at most once, and at most the size cap of the mise
  files; its location is `.julieta/lock-exceptions` at the top of the
  repo that uses the tool. It is reviewed on two paths: in the review of
  the pull request that adds the line, since the file is tracked, and by
  the pre-commit check, which reads the file from the index of the
  commit and judges the lock against it, so a line that does not parse
  is a finding before the commit exists. The file is one of the mise
  files of the ask-first `dependencies` surface of the repo that uses it.
- `REQ-0621` julieta trusts mise config only in the manifest's repo paths
  (`MISE_TRUSTED_CONFIG_PATHS`, 04 4.3) and runs mise with
  `MISE_PARANOID=1`, so a config file planted anywhere else in the
  sandbox, a parent directory of a repo included, is refused as not
  trusted, and with the global and the system configuration pointed at
  `/dev/null` and the data, cache and state dirs set by julieta, no
  configuration of the caller reaches mise. What this does not do is
  judge the trusted files: mise runs the `exec()` templates, tasks and
  hooks of a trusted repo's `mise.toml` as that repo's own code, by
  design (the residual of [05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)).

## 7.4 mise bootstrap and its own egress

| Item | Rule |
|---|---|
| Version | `REQ-0622` pinned in the `julieta` kit (`tools/kitpin mise <version>` rewrites version + both linux sha256 values from the release `SHASUMS256.txt`) |
| Verification | `REQ-0623` sha256 at kit build; minisign verification of `SHASUMS256.txt` is done by `tools/kitpin` at pin time (maintainer machine or CI), not in the sandbox, against mise's minisign public key committed in the repository (`kits/mise-minisign.pub`, committed beside `kits/pins.yaml`); `REQ-0624` `tools/kitpin` commits the signed `SHASUMS256.txt` and its signature as `kits/mise/SHASUMS256.txt` and `kits/mise/SHASUMS256.txt.minisig`, beside the pin, so `tools/ci kits` verifies them again offline on every change to the pin file ([10 10.2](10-testing-style.md#102-ci)) |
| Update checks | off (04 4.3): `REQ-0625` no self-update check and no update warning; the versions host is still read for version lists and attestations (the Versions host row) |
| Attestation verification | `REQ-1272` on, with `MISE_PARANOID=1` (04 4.3): mise verifies the attestation of an aqua or github download, which contacts `tuf-repo-cdn.sigstore.dev` for the sigstore trust root (measured with mise 2026.10.3, through a proxy that logs each host an install of `gh` reaches: `github.com`, `release-assets.githubusercontent.com`, `mise-versions.jdx.dev`, and with the variable also `tuf-repo-cdn.sigstore.dev`). Decided on 2026-10-22: the host is allowed. It is in the catalog `meta` set, an entry that lands with the catalog (`internal/catalog`, T031), and the egress derivation adds it with that set (7.5, step 4); a sandbox without it cannot install an attested tool |
| Tracking | off (04 4.3) |
| Versions host | `REQ-0626` kept on (`mise-versions.jdx.dev` serves version lists and attestation bundles); it is in the catalog `meta` set |
| GitHub rate limit | `REQ-0627` aqua/github downloads fall back to `api.github.com`; julieta's mise environment holds no token (decided on 2026-10-22): the proxy of the sandbox authenticates the requests to `api.github.com`, which probe B4 records, measuring a download with and without a `GITHUB_TOKEN` set. The sandbox holds whatever `github@<project>` binds ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1), [J1](09-journeys.md#j1-onboarding-new-machine-first-time) step 5); in the development sandbox of this repository that is the operator's own token until the token narrowing row of the index's [Deferred decisions](../spec.md#in-how-this-repository-is-run) fires ([ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md)), so this download path carries its reach |

## 7.5 Egress derivation (`internal/egress`)

Input per repo: `(url, egressCommit)`. Output: `auto` and `gated` sets.

1. `REQ-0628` Read `mise.lock` (and `mise.toml` for presence) at `egressCommit`,
   the commit of `refs/romeu/origin/<dir>/<ref>`, via `git cat-file`,
   under one size cap of 1 MiB, the file cap, that every repository blob
   romeu decodes shares. No lock with a `mise.toml` present: the sync
   error `lock-missing` (`RJ-340`), with the hint "run `julieta lock`".
   No mise files: the repo contributes nothing. romeu reads the lock
   layout written by the mise version the `julieta` kit pins; `REQ-0629` a lock it
   cannot parse, or one with an unknown top-level key, is the sync error
   `lock-unparsed` naming the repo and the fix "relock with `julieta
   lock`", never a partial read. One fixture of the pinned layout backs
   the parser test.
2. `REQ-0630` For each tool entry take its `backend` (for example `core:go`,
   `aqua:jqlang/jq`) and look it up in the catalog:
   - found: add its domains;
   - not found and the spec has `egress.tools["<backend>"]`: add those
     domains to `gated`;
   - not found otherwise: sync error (exit 2, `catalog-unknown-tool`)
     naming the tool and the fix (a catalog PR or a spec `egress.tools`
     entry).
3. `REQ-0631` Add every artifact URL host recorded in the lock; hosts not already
   produced by the catalog go to `gated`. Each host must pass the
   egress domain rule of [03 3.1](03-formats.md#31-name-and-path-rules-shared);
   one that fails is the sync error `lock-host-invalid` naming the lock
   entry.
4. `REQ-0632` Add `meta` and `base` sets, `tuf-repo-cdn.sigstore.dev` of 7.4 among the `meta` domains.
5. Add spec `egress.extra` to `gated`.
6. Add kit-declared network domains (from the normalized capability
   sets, [06](06-kits.md#61-decisions)), each host through
   `egress.Split`, which strips a numeric port, lowers the case and
   refuses a host that is no egress domain, naming the kit
   (REQ-0069). `REQ-1315` Only the hosts of the runtime phase enter the
   derivation; the hosts of the install phase stay apart, since the
   rules of this derivation last the life of the sandbox and an install
   host in them would stay open to the agent: `github.com`, which the
   `julieta` kit needs at install alone (REQ-0592), takes gists,
   issues and pushes. A host the kit needs at runtime too is declared
   in the runtime phase. The install hosts still go through
   `egress.Split` and are shown in gate 2 with the capability set
   (REQ-0569). Measured at sitting M0 (sbx v0.48.0): the install
   steps run inside the sandbox, once at create and before the agent,
   and the proxy refuses an undeclared host there. Not measured yet:
   that sbx allows a host of the install phase during the install and
   closes it before the agent starts. The first real `sbx` build of
   the `julieta` or `os-base` kit measures all three on the sbx it runs
   (the host ran v0.50.1 on 2026-10-29, after M0); if the install
   cannot reach its hosts, or a host still answers at runtime, this
   rule and the kits are revisited (decided on 2026-10-29, scope this
   repository).
7. Move every domain with `upload: true`, or without a `domains` entry,
   to `gated`.
8. `REQ-0633` `effectiveDigest` = `canon.Digest(canon.Egress, sorted union)`; recorded
   in `render.json`.

julieta's own network commands follow the same rule. The config
project's spec carries in `egress.extra` the registry hosts of its
`workloadRepositories`, which `julieta pin workload` and `julieta pin
check --workflows` read; `api.github.com` comes from the base set
(step 4) and is not listed
([04 4.3](04-cli.md#43-julieta-sandbox)); they are gated like any extra
and approved once.

`egress.tools` and `egress.extra` live in `projects/*.yaml` in the
config repo, so how often a project needs them is read there; v1 adds
no counter, a row of [Deferred decisions](../spec.md#in-the-product).

Deliberate deviation: the lock, not `mise.toml`, is the input. The lock
names the exact backend and artifact hosts per platform at a pinned
blob; `mise.toml` short names need mise's registry to resolve, which
would mean running mise on the host.

`REQ-0634` Application (romeu): per-sandbox rules via
`sbx policy allow network --sandbox <name> <domain>` for `auto` plus
approved `gated`; removal via `sbx policy rm network --sandbox <name>
--resource <domain>` for domains romeu applied earlier and that left the set;
verification with `sbx policy check network --sandbox <name> <domain>`
(exact argv from the block A recordings).
`REQ-0635` Applied on create (before `julieta setup`), on every `run` (re-applies
after an `sbx reset`), and on `sync` for a running sandbox. `REQ-0636` The
manifest carries the applied set
([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)), and
`julieta status` prints it, so an agent can tell an unknown host from a
gated one.

Egress follows `ref`, so a tool added on a branch installs once the
change merges and `sync` applies it. Before that, runtime blocks
surface in sbx's native approval queue. `REQ-0637` julieta's install error names
the host, the sbx command that approves it once for this sandbox, that
the approval is lost at recreate, and the durable fix, a spec edit plus
the gate. Listing the native queue in `status` is a row of
[Deferred decisions](../spec.md#in-the-product).

*Why for us (catalog keyed on backend:tool):* a `github:` or `http:`
backend can point anywhere, so a tool name is not an identity; lock URL
hosts are exact, the catalog adds registries and redirect CDNs the lock
cannot know.

## 7.6 Catalog maintenance

- `REQ-0638` `internal/catalog/egress/` (one file per tool family) is embedded in
  romeu and julieta; a catalog change ships only in a release and shows
  up once per machine in gate 1, which prints the domain and upload-flag
  diff per affected project (01 1.4). A project sees gate 2 only when
  its gated egress changes.
- `REQ-0639` `julieta spec validate --catalog projects/*.yaml` fetches every
  project's repos' `mise.lock` at their `ref` and lists unknown
  `backend:tool` keys and lock hosts missing from the catalog. Its exit
  rule is the row of [04 4.3](04-cli.md#43-julieta-sandbox). `REQ-0640` It runs in
  the config repo's CI from a pinned julieta release, so that CI is red
  until an unknown tool is resolved.
- `REQ-0641` Every domain used anywhere has an explicit `upload` flag (CI check);
  multi-tenant object stores are `upload: true`.
