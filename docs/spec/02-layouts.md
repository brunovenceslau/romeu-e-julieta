# 2. Repositories and directory layouts

Back to [index](../spec.md). Reader: implementers of `render`, `state`
and julieta's setup. Type: reference.

## 2.1 Product repo: `romeu-e-julieta` (public)

```
romeu-e-julieta/
├─ go.mod                      module github.com/<owner>/romeu-e-julieta
├─ mise.toml, mise.lock        dev tools for this repo (go, golangci-lint, ...); lock for linux-x64, linux-arm64, macos-x64, macos-arm64
├─ ARCHITECTURE.md             one page: components, trust boundaries, the state machines
├─ CONTRIBUTING.md             dev loop; how to add a command, invariant, probe or kit via tools/new
├─ cmd/
│  ├─ romeu/main.go            host binary (darwin; linux build for tests only)
│  └─ julieta/main.go          sandbox binary (linux)
├─ internal/
│  ├─ spec/                    types with gate/apply tags, rules.go (validation rules as data), strict decode
│  ├─ canon/                   canonical JSON and typed, domain-separated digests
│  ├─ termsafe/                escaping of control, bidi, zero-width and tag characters for host output
│  ├─ gitsafe/                 hardened git runner, bundle, create-only refs, validated tree walk
│  ├─ oci/                     read and verify v3 descriptors by digest (stdlib net/http)
│  ├─ catalog/                 embedded catalog (go:embed ../../catalog/egress.yaml) + lookups
│  ├─ egress/                  derivation from mise files at pinned SHAs
│  ├─ render/                  candidates, promotion commit, workspace files, render.json, kit materialization via os.Root
│  ├─ state/                   host state records, the candidate and generation state machines, descriptor cache, flock
│  ├─ gate/                    widening set, diff, prompt
│  ├─ signing/                 ssh-agent identity listing for the git-ssh-sign rule (romeu only)
│  ├─ sbxdrv/                  sbx argv builders and output parsing (romeu only)
│  ├─ memstore/                memory Store interface, fs backend (os.Root), layout allowlist, JSONL import/verify
│  ├─ handoff/                 handoff files and the handoff reader
│  ├─ salvage/                 snapshot/salvage (julieta side) and verify/import (romeu side)
│  ├─ tools/                   mise driver (julieta only)
│  ├─ hooks/                   git hook dispatcher (julieta only)
│  ├─ layout/                  run layout -> herdr layout.apply, pane step runner (julieta only)
│  ├─ agent/claude/            the only Claude-aware code: salvage paths, process names
│  ├─ shquote/                 POSIX shell quoter (table-tested)
│  └─ cli/                     subcommand dispatcher, exit codes and error ids (data), escaping writer, text/--json
├─ kits/                       v3 kit sources, local builds (see 06-kits)
│  ├─ julieta/                 mixin: mise, herdr, hook dispatcher dir, env
│  ├─ julieta-claude/          mixin: skills, SessionStart/SessionEnd hooks, memory rule
│  ├─ os-base/                 mixin: minimal apt layer, TZ arg
│  └─ git-ssh-sign/            mixin: ssh-agent signing for git
├─ catalog/egress.yaml         egress catalog (data)
├─ skills/
│  ├─ julieta/SKILL.md         general rule: how to use julieta, memory instead of built-in memory
│  └─ handoff/SKILL.md         /handoff and /handoff --final
├─ schemas/                    generated JSON Schema 2020-12 (CC0-1.0): project.v1, host-settings.v1, catalog.v1, render.v1, state-*.v1, handoff.v1, memory-entry.v1, manifest.v1, salvage.v1, probe-result.v1, acceptance.v1
├─ examples/                   example config repo (projects/*.yaml, kits/) used by tests and docs (CC0-1.0)
├─ e2e/
│  ├─ *_test.go                //go:build e2e - git + docker, fake sbx
│  ├─ scenarios/               journey scenario functions shared by the CI and host suites; commands.yaml, each scenario's command lines (one source for scenarios and guides)
│  ├─ host/*_test.go           //go:build host - real sbx, maintainer only
│  ├─ probes/                  host probe harness (go run ./e2e/probes): pure core + thin sbx exec layer
│  ├─ fakesbx/                 fake sbx that replays recorded sessions only
│  └─ testdata/sbx/<version>/  redacted sbx help text and argv/stdout/stderr/exit sessions (*.jsonl)
├─ tools/
│  ├─ ci/                      every check CI runs, the delivery metrics report (12 12.10), and their committed data: denylist.yaml (forbidden names, hashed), prose.yaml, headings.yaml, testdata/
│  ├─ new/                     scaffolding: adr, invariant, probe, kit, command, lesson
│  ├─ schemagen/               reflect-based JSON Schema generator from the Go types and rules.go
│  ├─ release/                 release subcommands (10 10.2)
│  └─ kitpin/                  rewrites pinned versions/digests across kits
├─ docs/
│  ├─ spec.md, spec/           this specification (current state)
│  ├─ reviews/                 review rounds of the specification (history)
│  ├─ adr/                     decisions (adr-tools layout, .adr-dir -> docs/adr); README.md is the generated index; an ask-first surface
│  ├─ guide/                   one page per journey
│  ├─ reference/               generated reference pages (12 12.3)
│  ├─ probes/                  committed probe-result.v1 files (blocks A, B and C)
│  ├─ acceptance.json          evidence per success criterion (acceptance.v1)
│  └─ lessons.md               one entry per lesson, with the check that enforces it
├─ .githooks/pre-push          mode 100755; runs go run ./tools/ci fast with git's arguments and stdin
├─ .golangci.yml               linter configuration (10 10.2, lint)
├─ .github/ask-first.yaml      the single list of ask-first paths and their owner (05 5.3)
├─ .github/CODEOWNERS          generated from ask-first.yaml
├─ .github/workflows/ci.yml, release.yml, fuzz.yml (scheduled; long fuzz runs and tools/ci mutate)
├─ .github/pull_request_template.md   Why / What changed / Evidence / Middleware / Lessons
├─ COPYING (with the first code change), REUSE.toml, LICENSES/, README.md, SECURITY.md
```

Embedding: `romeu` embeds `catalog/egress.yaml`, `kits/*` and, at release
time, the two `julieta` linux binaries; `julieta` embeds the catalog too
(for `spec validate --catalog`). The julieta binaries are not kit
content: romeu writes them to `<name>-env/.romeu/bin/` and the sandbox
sees that directory through a read-only mount (see 2.3 and
[06](06-kits.md#63-julieta-delivery)), so a romeu release does not
change any kit digest. *Why for us:* one signed-commit release is the
trust root for what enters a sandbox, without making every upgrade a
recreate.

## 2.2 Config repo (reference instance: `verona`)

```
verona/
├─ projects/
│  └─ <name>.yaml             one project spec per file; file stem == spec name
├─ kits/
│  └─ <kit>/                  personal v3 kits (same format as product kits)
├─ mise.toml, mise.lock       tools for agents editing this repo (julieta comes from the sandbox)
├─ acceptance.json            optional: the operator's own acceptance ledger (acceptance.v1), verified from a product checkout
├─ .github/workflows/validate.yml   runs `julieta spec validate --catalog projects/*.yaml` from a pinned julieta release (checked by `julieta pin check`)
└─ README.md
```

Rules: `projects/` holds only `*.yaml`; one of them is the config project
itself (its primary repo URL equals the host settings' `config.url`).
romeu reads this repo only through git objects at a named commit
(`git cat-file`), never from a working tree.

## 2.3 Host tree

```
$ROMEU_ROOT/                            default $HOME/dev; never a VS Code trusted folder; never inside a git repo
├─ dev.code-workspace                   derived: folders = every host clone; open in Restricted Mode
├─ review.code-workspace                derived: folders = every review checkout; open in Restricted Mode
├─ .attic/<name>/<UTC-ts>/              retired projects (moved, never deleted)
└─ <name>-env/                          one per project; <name> = sandbox name
   ├─ sbxenv.yaml                       derived, never hand-edited; the promotion commit point
   ├─ .romeu/
   │  ├─ render.json                    derived: spec source, SHAs, file digests, egress, digests, promotion marker
   │  ├─ candidates/<candidate-id>/     at most one unapproved candidate (sbxenv.yaml, render.json, kits/); never referenced by a live file
   │  ├─ kits/<kit>-<digest12>/         approved kit sources, content-addressed and immutable, referenced as ./.romeu/kits/<kit>-<digest12>
   │  └─ bin/                           julieta-linux-amd64, julieta-linux-arm64, SHA256SUMS; mounted read-only into the sandbox
   ├─ <dir>/                            host clone of each repo (primary is the sbx workspace, clone mode)
   ├─ review/<dir>/                     hardened review checkout (created by `romeu pull`)
   └─ memory/<dir>/                     per-repo memory, mounted rw into this project's sandbox only
```

Mounts: the sandbox sees `./<primary>` (clone mode: sbx clones it; the
host clone is read-only from the sandbox), `./memory/<dir>` for every
repo (direct, rw) and `./.romeu/bin` (direct, read-only). `sbxenv.yaml`
and the rest of `.romeu/` sit outside every mount; paths in the file are
`./x`, relative to the file; no `..`, no absolute path. Confirmed by
probes A3 and A5.

Host git refs written by romeu in each host clone, all in romeu-owned
namespaces: `refs/romeu/origin/<dir>/*` (origin fetches used for
derivation; romeu never touches `refs/remotes/*`),
`refs/sandboxes/<name>/*` (primary, from the sandbox daemon),
`refs/romeu/snapshots/<name>/*`, `refs/romeu/salvage/<name>/*`.

Clones are plain clones, never git submodules of any repo (submodules
freeze versions and fight worktrees); romeu never runs submodule
commands and gitsafe sets `submodule.recurse=false`.

Invariants: a clone directory is never moved, re-cloned or deleted by
`sync`, `run` or `rm` (moving drops the `sandbox-<name>` remote and the
path to unpushed work); only `retire` moves a whole `<name>-env/`, and
only with no sandbox.

*Why for us (path as identity):* ghq derives a clone path from identity
and later paid for changing the root default as an incompatible change;
we fix `$ROMEU_ROOT/<name>-env/<dir>` before v1.

## 2.4 Host settings and state

```
$XDG_CONFIG_HOME/romeu/settings.yaml    (default $HOME/.config/romeu/settings.yaml), mode 0600, operator-edited
$XDG_STATE_HOME/romeu/                  (default $HOME/.local/state/romeu/), mode 0700
├─ romeu.lock                           flock: one mutating romeu at a time
├─ toolchain.json                       gate 1 record, including the acknowledged catalog
├─ projects/<name>.json                 one record per project: approval, awaiting candidate, generations, applied egress
├─ descriptors/<digest>.json            content-addressed cache of parsed workload and kit descriptors
└─ attic/<name>/<UTC-ts>/               records of retired projects
```

Each record is one file written atomically, so every state transition of
a project is a single atomic write. `ROMEU_SETTINGS` (path to
`settings.yaml`) is the only environment override, used by tests; the
root comes only from the settings file.

## 2.5 In-sandbox paths

| What | Path | Owner |
|---|---|---|
| primary repo | same absolute path as the host clone (sbx clone mode) | sbx |
| julieta binaries | same absolute path as `<name>-env/.romeu/bin/` (read-only mount); romeu always invokes them by that absolute path | romeu |
| `julieta` on `PATH` | `$HOME/.local/bin/julieta`, a symlink to the running binary, created by `julieta setup` at runtime | julieta |
| secondary repos | `$HOME/src/<name>/<dir>` | julieta (`julieta setup`) |
| memory dirs | same absolute path as on the host (direct mount) | host tree |
| julieta state | `$HOME/.local/state/julieta/` (manifest cache, install digests, snapshot timestamps, recorded hook failures) | julieta |
| julieta hook dispatcher | `$HOME/.local/share/julieta/hooks/` (global `core.hooksPath`) | julieta |
| mise | `$HOME/.local/bin/mise`, data in `$HOME/.local/share/mise` | julieta mixin |
| herdr | `$HOME/.local/bin/herdr`, config `$HOME/.config/herdr/` | julieta mixin |

julieta never guesses paths: every path comes from the manifest (see
[03](03-formats.md#36-julieta-manifest-schema-manifestv1)) or from its own executable path.

## 2.6 Multi-repo projects: the secondary-repo decision

| Option | Host exec risk | Agent can edit | Salvage path | Verdict |
|---|---|---|---|---|
| sbx additional workspace, direct rw mount | **high**: agent writes `.git/config`, hooks, `.vscode/tasks.json` in a tree host tools open | yes | bytes already on host | rejected |
| direct read-only mount | none from that tree | no | n/a | rejected as default (not useful for work) |
| one sandbox per repo | none | yes | per sandbox | rejected: breaks "a project may hold several repos" |
| **sandbox-private clone by julieta from origin** | none: nothing agent-written is mounted to the host except memory data | yes | snapshot/salvage bundles in the memory dir, verified and imported by romeu | **decided** |

Consequences: secondary repos need the project's git credential inside
the sandbox to clone private repos and push (the `github` secret by name);
work reaches GitHub by push as usual; unpushed work reaches the host only
through snapshot and salvage bundles. The host clone of each secondary is
a plain operator clone used for navigation and egress derivation.

The decision rests on documented sbx behavior (additional workspaces are
direct mounts) and does not wait for a probe. Probe A3 measures what
still is mounted: that memory dirs refuse or survive planted content as
08 expects, and that `readOnly` mounts (`.romeu/bin`) are enforced.
