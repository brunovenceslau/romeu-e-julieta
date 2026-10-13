# 2. Repositories and directory layouts

Back to [index](../spec.md). Reader: implementers of `render`, `state`
and julieta's setup. Type: reference.

## 2.1 Product repo: `romeu-e-julieta` (public)

```
romeu-e-julieta/
├─ go.mod                      module github.com/<owner>/romeu-e-julieta
├─ mise.toml, mise.lock        `REQ-0111` dev tools for this repo (go, golangci-lint, ...); lock for linux-x64, linux-arm64, macos-x64, macos-arm64
├─ ARCHITECTURE.md             one page: components, trust boundaries, the state machines
├─ CONTRIBUTING.md             dev loop; how to add a command, invariant, probe or kit via tools/new
├─ cmd/                       `REQ-0112` `cmd/<binary>/main.go` is the composition root: it builds the `Env` of `internal/cli` from the real clock, entropy, the version stamp and the process streams, installs the signal handler of `internal/procgroup`, imports `internal/cmd/<binary>/all` and calls `cli.Main`, and holds no other logic
│  ├─ romeu/main.go            `REQ-0113` host binary (darwin; linux build for tests only)
│  └─ julieta/main.go          sandbox binary (linux)
├─ internal/
│  ├─ spec/                    types with gate/apply tags, rules.go (validation rules as data), strict decode
│  ├─ canon/                   canonical JSON and typed, domain-separated digests
│  ├─ termsafe/                escaping of control, bidi, zero-width and tag characters for host output
│  ├─ gitsafe/                 hardened git runner, bundle, create-only refs, validated tree walk
│  ├─ oci/                     read and verify v3 descriptors by digest (stdlib net/http); the one descriptor grammar, which render calls for local kits
│  ├─ catalog/                 embedded catalog: egress/<family>.yaml beside the package (schema catalog.v1, 03 3.5), embedded by //go:embed egress/*.yaml, and the lookups
│  ├─ egress/                  derivation from mise files at pinned SHAs
│  ├─ render/                  candidates, promotion commit, workspace files, render.json, kit materialization via os.Root
│  ├─ state/                   host state records, the candidate and generation state machines, descriptor cache, flock
│  ├─ gate/                    widening set, diff, prompt
│  ├─ preflight/              the ordered checks of 01 1.5 before a mutating sbx call
│  ├─ signing/                 ssh-agent identity listing for the git-ssh-sign rule
│  ├─ sbxdrv/                  sbx argv builders and output parsing
│  ├─ memstore/                memory readers on os.Root, layout allowlist, caps
│  ├─ memstore/write/          the Store interface with Put and Delete, fs backend writes, store lock
│  ├─ handoff/                 handoff files and the handoff reader
│  ├─ salvage/                 snapshot/salvage (julieta side) and verify/import (romeu side)
│  ├─ manifest/                the julieta manifest type (03 3.6), marshalled by romeu and decoded by julieta
│  ├─ mise/                    mise driver
│  ├─ hooks/                   git hook dispatcher
│  ├─ layout/                  run layout -> herdr layout.apply, pane step runner
│  ├─ agent/claude/            `REQ-0114` the only Claude-aware code: salvage paths, process names
│  ├─ shquote/                 POSIX shell quoter (table-tested)
│  ├─ cmd/romeu/<command>/     `REQ-0115` one package per romeu command; cmd/romeu/all/ holds one blank-import file per command (import_<command>.go) and links them in
│  ├─ cmd/julieta/<command>/   one package per julieta command; cmd/julieta/all/ the same, so tools/** and e2e/** reach every registry through one import (10 10.7)
│  └─ cli/                     the dispatcher table, generated from the command packages (12 12.3); exit codes and error ids (data), escaping writer, text/--json
├─ kits/                       `REQ-0116` v3 kit sources, local builds (see 06-kits); embed.go, package kits, the one embed.FS of the product kit directories, pins.yaml excluded, which romeu and the render seam read
│  ├─ julieta/                 product kit: mise, herdr, hook dispatcher dir, env
│  ├─ julieta-claude/          product kit: skills, SessionStart/SessionEnd hooks, memory rule
│  ├─ os-base/                 product kit: minimal apt layer, TZ arg
│  ├─ git-ssh-sign/            product kit: ssh-agent signing for git
│  └─ pins.yaml                `REQ-0117` the one pin file: kit frontend, workload, herdr, mise; written only by tools/kitpin (frontend, workload <digest>, herdr <version>, mise <version>)
├─ catalog/egress.yaml         egress catalog (data)
├─ skills/
│  ├─ julieta/SKILL.md         general rule: how to use julieta, memory instead of built-in memory
│  └─ handoff/SKILL.md         /handoff and /handoff --final
├─ schemas/                    `REQ-0118` generated JSON Schema 2020-12 (CC0-1.0): project.v1, host-settings.v1, catalog.v1, render.v1, state-*.v1, handoff.v1, memory-entry.v1, manifest.v1, salvage.v1, probe-result.v1, acceptance.v1, and one per `--json` output, `<binary>-<command>.v1` (04 4.1)
├─ examples/                   `REQ-0119` example config repo (projects/*.yaml, kits/, .github/workflows/validate.yml): the adopter's starter (J1 step 0), also used by tests and docs (CC0-1.0)
├─ e2e/
│  ├─ *_test.go                `REQ-0120` //go:build e2e - git + docker, fake sbx
│  ├─ scenarios/<journey>/     journey scenario functions shared by the CI and host suites, one directory per journey, each with its commands.yaml, the scenario's command lines (one source for scenarios and guides)
│  ├─ host/*_test.go           //go:build host - real sbx, maintainer only
│  ├─ probes/                  host probe harness (go run ./e2e/probes): pure core + thin sbx exec layer
│  ├─ fakesbx/                 fake sbx that replays recorded sessions only
│  └─ testdata/sbx/<version>/  redacted sbx help text and argv/stdout/stderr/exit sessions (*.jsonl)
├─ tools/
│  ├─ ci/                      `REQ-0121` every check CI runs and its committed data: denylist.yaml (forbidden names, hashed, and the never-tracked file names), prose.yaml, headings.yaml, testdata/; one package per check under tools/ci/internal/<check>/ plus tools/ci/check_<check>.go, whose init registers its name, usage, membership in fast and all, order key and generator outputs (12 12.3), so a new check adds files and edits none; the shared helper packages sit beside them, importable from tools/mutants and tools/new, each registering no check
│  ├─ new/                     scaffolding: adr, invariant, probe, kit, command, lesson, task, error
│  ├─ schemagen/               reflect-based JSON Schema generator from the Go types and rules.go
│  ├─ release/                 release subcommands (10 10.2)
│  ├─ kitpin/                  writes kits/pins.yaml, the one pin file
│  └─ mutants/                 mutation runner on a scratch worktree; never edits the tree (tools/ci mutate runs it)
├─ docs/
│  ├─ spec.md, spec/           this specification (current state)
│  ├─ plan.md, plan/tasks/     the build plan: a narrative page and one file per task, with its checkbox
│  ├─ reviews/                 review rounds of the specification (history)
│  ├─ adr/                     decisions (adr-tools layout, .adr-dir -> docs/adr); README.md is the generated index; an ask-first surface
│  ├─ guide/                   one page per journey, and recovery.md
│  ├─ diagrams/                generated state diagrams (12 12.3), linked from ARCHITECTURE.md
│  ├─ notes/                   explanations kept beside the specification: records of procedures that ran once, deferred designs
│  ├─ reference/               generated reference pages (12 12.3)
│  ├─ probes/                  committed probe-result.v1 files (blocks A and B)
│  ├─ acceptance.json          evidence per success criterion (acceptance.v1)
│  ├─ grants.yaml              `REQ-0122` checkpoint grants that stand in for approval lines (12 12.4); read by tools/ci pr alone; an ask-first surface
│  └─ lessons/                 `REQ-0123` one file per lesson (NNNN-<slug>.md), with the check that enforces it; README.md is the generated index
├─ .githooks/pre-push          `REQ-0124` mode 100755; runs go run ./tools/ci fast with git's arguments and stdin
├─ .golangci.yml               linter configuration (10 10.2, lint)
├─ .github/ask-first.yaml      the single list of ask-first paths and their owner (05 5.3)
├─ .github/CODEOWNERS          `REQ-0125` generated from ask-first.yaml
├─ .github/workflows/ci.yml, release.yml, fuzz.yml (scheduled; long fuzz runs and tools/ci mutate)
├─ .github/pull_request_template.md   `REQ-0126` Why / What changed / Evidence / Middleware / Lessons
├─ .github/ISSUE_TEMPLATE/     `REQ-0127` two issue forms: a bug, a catalog gap; the label `deferred` lands with them (12 12.7)
├─ COPYING (with the first code change), REUSE.toml, LICENSES/, README.md, SECURITY.md
```

Which binary may import each package is
[10 10.7](10-testing-style.md#107-module-boundaries-enforced-by-toolsci-imports);
module ids are [12 12.2](12-engineering.md#122-development-commands-and-capability-map);
which files are generated is the inventory of
[12 12.3](12-engineering.md#123-generators).

`REQ-0128` Embedding: a `go:embed` pattern cannot leave its package directory, so
each embedded input sits beside the package that embeds it. `REQ-0129` `romeu`
embeds the catalog (`internal/catalog/egress/*.yaml`, from
`internal/catalog`), the product kit directories through `kits/embed.go`
(package `kits`, one `embed.FS`, `pins.yaml` excluded) and, at release
time, the `julieta` linux binary of romeu's own `GOARCH`, which
`tools/release build` writes into the embed directory under `internal/`
that its task names (the release builds both, one per romeu archive);
`julieta` embeds the catalog too (for `spec validate --catalog`). `REQ-0130` A
source build without `tools/release build` embeds a placeholder: romeu
built that way refuses `run`, `sync` and promotion with exit 2 and the
error id `julieta-not-embedded`, and every other command works.
`REQ-0131` `tools/release build` is the only supported way to embed julieta, and
contributor e2e builds go through it
([10 10.2](10-testing-style.md#102-ci)). `REQ-0132` The julieta binaries are not
kit content: romeu writes them to `<name>-env/.romeu/bin/` and the
sandbox sees that directory through a read-only mount (see 2.3 and
[06](06-kits.md#63-julieta-delivery)), so a romeu release does not
change any kit digest. *Why for us:* one release, built and attested by
`release.yml` ([05](05-security.md), threat model), is the trust root
for what enters a sandbox, without making every upgrade a recreate.

## 2.2 Config repo

```
my-config/
├─ projects/
│  └─ <name>.yaml             `REQ-0133` one project spec per file; file stem == spec name
├─ kits/
│  └─ <kit>/                  personal v3 kits (same format as product kits)
├─ mise.toml, mise.lock       tools for agents editing this repo (julieta comes from the sandbox)
├─ acceptance.json            optional: the operator's own acceptance file (acceptance.v1), checked by the config repo's own scripts; the product does not verify it
├─ .github/workflows/validate.yml   `REQ-0134` runs `julieta spec validate --catalog projects/*.yaml` from a julieta release pinned by version and archive sha256, verified against the release's checksums.txt before it runs (checked by `julieta pin check`)
└─ README.md
```

`REQ-0135` Rules: `projects/` holds only `*.yaml`; one of them is the config
project itself (its primary repo URL equals the host settings'
`config.url`); both are rows of
[03 3.1](03-formats.md#31-name-and-path-rules-shared), checked by `sync`
step 3 and, for the `*.yaml` rule, by `julieta spec validate`
([04 4.2](04-cli.md#42-romeu-host),
[04 4.3](04-cli.md#43-julieta-sandbox)). romeu reads this repo only
through git objects at a named commit (`git cat-file`), never from a
working tree. The registry hosts the config project's spec must carry in
`egress.extra` are the rule of
[07 7.5](07-mise-egress.md#75-egress-derivation-internalegress). The
config repo names private repositories, secret names and internal
domains: keep it private unless every project in it is public. The
maintainer's own config repo is the reference config repo the catalog is
built from (03 3.5).

## 2.3 Host tree

```
$ROMEU_ROOT/                            `REQ-0136` default $HOME/dev; never a VS Code trusted folder; never inside a git repo
├─ dev.code-workspace                   `REQ-0137` derived: folders = every host clone; open in Restricted Mode
├─ review.code-workspace                derived: folders = every review checkout; open in Restricted Mode
├─ .attic/<name>/<UTC-ts>/              retired projects (moved, never deleted by romeu); .attic/ is mode 0700
└─ <name>-env/                          one per project; <name> = sandbox name; mode 0700
   ├─ .metadata_never_index             `REQ-0138` written by sync, outside every mount; keeps Spotlight from parsing agent-written files (probe A17)
   ├─ sbxenv.yaml                       derived, never hand-edited; the promotion commit point
   ├─ .romeu/
   │  ├─ render.json                    derived: spec source, SHAs, file digests, egress, digests, promotion marker
   │  ├─ candidates/<candidate-id>/     `REQ-0139` at most one unapproved candidate (sbxenv.yaml, render.json, kits/); never referenced by a live file
   │  ├─ kits/<kit>-<digest12>/         approved kit sources, content-addressed and immutable, referenced as ./.romeu/kits/<kit>-<digest12>
   │  └─ bin/                           julieta-linux-<GOARCH>, SHA256SUMS; mounted read-only into the sandbox
   ├─ <dir>/                            host clone of each repo (primary is the sbx workspace, clone mode)
   ├─ review/<dir>/                     hardened review checkout (created by `romeu pull`)
   └─ memory/<dir>/                     `REQ-0140` per-repo memory, mounted rw into this project's sandbox only; sync creates it 0700, julieta writes its files 0600
```

The supported daily setup: edit and debug inside the sandbox; the host
clones are for reading, in Restricted Mode.

The tree is an illustration: the names romeu reserves inside
`<name>-env/` are listed once, in the repo `dir` rule of
[03 3.1](03-formats.md#31-name-and-path-rules-shared). Who owns each
store and what `rm` and `retire` do to it is the table of
[01 1.2](01-system-model.md#where-data-lives). `REQ-0141` romeu creates `.attic/`
and each `<name>-env/` with mode 0700, so everything under them sits
behind a closed parent, and `doctor` fails when either is group- or
world-accessible.

`REQ-0142` Mounts: the sandbox sees `./<primary>` (clone mode: sbx clones it; the
host clone is read-only from the sandbox), `./memory/<dir>` for every
repo (direct, rw) and `./.romeu/bin` (direct, read-only). `.romeu/bin`
is a fixed mount: no spec field names it and each project has it.
`sbxenv.yaml` and the rest of `.romeu/` sit outside every mount; paths
in the file are `./x`, relative to the file; no `..`, no absolute path.
Confirmed by probes A3 and A5. The runtime ledger's two mounts of
[13 13.1](13-runtime-ledger.md#131-stores) are not in v1.

`REQ-0143` Host git refs written by romeu in each host clone, all in romeu-owned
namespaces: `refs/romeu/origin/<dir>/*` (origin fetches used for
derivation; `REQ-0144` romeu never touches `refs/remotes/*`),
`refs/sandboxes/<name>/*` (primary, from the sandbox daemon),
`refs/romeu/snapshots/<name>/*`, `refs/romeu/salvage/<name>/*`, and the
create-only `refs/romeu/base/<name>/<generation>/<dir>` at each base SHA
romeu puts in a manifest ([03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)),
kept for as long as the generation and its salvage records exist, so
the prerequisites of thin bundles survive a
force-moved `refs/romeu/origin` and `git gc`.

These refs belong to the clone they are in: deleting or re-cloning it
loses them, and `git push --mirror` publishes them (salvage commits can
hold untracked files). List them with
`git for-each-ref refs/romeu refs/sandboxes`. `REQ-0145` `doctor` fails when a ref
under a salvage record's `refsPrefix` is missing from its clone.

`REQ-0146` Clones are plain clones, never git submodules of any repo (submodules
freeze versions and fight worktrees); romeu never runs submodule
commands and gitsafe sets `submodule.recurse=false`.

Invariants: the host-clones row of
[01 1.2](01-system-model.md#where-data-lives) holds the rule that
`sync`, `run` and `rm` never move, re-clone or delete a clone directory;
the reason is that moving drops the `sandbox-<name>` remote and the path
to unpushed work. `REQ-0147` Only `retire` moves a whole `<name>-env/`, and only
with no sandbox.

*Why for us (path as identity):* ghq derives a clone path from identity
and later paid for changing the root default as an incompatible change;
we fix `$ROMEU_ROOT/<name>-env/<dir>` before v1.

## 2.4 Host settings and state

```
$XDG_CONFIG_HOME/romeu/settings.yaml    `REQ-0148` (default $HOME/.config/romeu/settings.yaml), mode 0600, operator-edited
$XDG_STATE_HOME/romeu/                  (default $HOME/.local/state/romeu/), mode 0700
├─ romeu.lock                           flock: one mutating romeu at a time
├─ toolchain.json                       gate 1 record, including the acknowledged catalog
├─ projects/<name>.json                 one record per project: approval, generations, applied egress
├─ descriptors/<digest>.json            content-addressed cache of parsed workload and kit descriptors
└─ attic/<name>/<UTC-ts>/               records of retired projects
```

`REQ-0149` Each record write is atomic; a transition that changes more than one
file follows its step list in
[01 1.6](01-system-model.md#16-state-machines). `REQ-0150` `ROMEU_SETTINGS` (path
to `settings.yaml`, used by tests) is the only romeu-specific override
of the settings path; `XDG_CONFIG_HOME` and `XDG_STATE_HOME` are
honoured as shown; the root comes only from the settings file. A
machine has one root and one state; separate instances per machine are
a row of [Deferred decisions](../spec.md#in-the-product). Who owns each
store is the table of [01 1.2](01-system-model.md#where-data-lives).

## 2.5 In-sandbox paths

| What | Path | Owner |
|---|---|---|
| primary repo | same absolute path as the host clone (sbx clone mode) | sbx |
| julieta binaries | same absolute path as `<name>-env/.romeu/bin/` (read-only mount); `REQ-0151` romeu always invokes them by that absolute path | romeu |
| `julieta` on `PATH` | `REQ-0152` `$HOME/.local/bin/julieta`, a symlink to the running binary, created by `julieta setup` at runtime | julieta |
| secondary repos | `REQ-0153` `$HOME/src/<name>/<dir>` | julieta (`julieta setup`) |
| memory dirs | same absolute path as on the host (direct mount) | host tree |
| julieta state | `REQ-0154` `$HOME/.local/state/julieta/` (manifest cache, install digests, snapshot timestamps, recorded hook failures) | julieta |
| julieta hook dispatcher | `$HOME/.local/share/julieta/hooks/` (global `core.hooksPath`) | julieta |
| mise | `$HOME/.local/bin/mise`, data in `$HOME/.local/share/mise` | julieta product kit |
| herdr | `$HOME/.local/bin/herdr`, config `$HOME/.config/herdr/` | julieta product kit |

`REQ-0155` julieta takes repo, memory and binary paths from the manifest (see
[03 3.6](03-formats.md#36-julieta-manifest-schema-manifestv1)) or from its
own executable path. The rows owned by julieta or the julieta product
kit derive from `$HOME` by rule, and the mise and herdr rows are those
tools' upstream defaults at the pinned versions, which the julieta
product kit sets explicitly through the environment (`MISE_DATA_DIR`,
the herdr config path), so a moved default changes nothing.

## 2.6 Multi-repo projects: the secondary-repo decision

| Option | Host exec risk | Agent can edit | Salvage path | Verdict |
|---|---|---|---|---|
| sbx additional workspace, direct rw mount | **high**: agent writes `.git/config`, hooks, `.vscode/tasks.json` in a tree host tools open | yes | bytes already on host | rejected |
| direct read-only mount | none from that tree | no | n/a | rejected as default (not useful for work) |
| one sandbox per repo | none | yes | per sandbox | rejected: breaks "a project may hold several repos" |
| **sandbox-private clone by julieta from origin** | `REQ-0156` none: nothing agent-written is mounted to the host except memory data | yes | snapshot/salvage bundles in the memory dir, verified and imported by romeu | **decided** |

`REQ-0157` Consequences: secondary repos need the project's git credential inside
the sandbox to clone private repos and push (the `github` secret by name);
work reaches GitHub by push as usual; unpushed work reaches the host only
through snapshot and salvage bundles. The credential is documented sbx
behavior: sbx's proxy injects the named secret into HTTPS requests to
GitHub, so git, gh and mise authenticate without a credential file in
the sandbox; probe A3 checks it by cloning a private secondary, pushing
a branch and calling the GitHub API. The host clone of each secondary is
an operator clone used for navigation and egress derivation, holding
romeu's refs (2.3).

The decision rests on sbx's documented behavior at the time of writing
(additional workspaces are direct mounts) and waits for no probe. `REQ-0158` A3
re-confirms it at the sbx version A1 records and measures what is still
mounted: that memory dirs refuse or survive planted content as 08
expects, that the `readOnly` mount `.romeu/bin` is enforced, and that a
write from the sandbox to the primary host clone is refused.
