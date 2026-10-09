# 5. Security model

Back to [index](../spec.md). Reader: implementers and security
reviewers. Type: reference (invariants) with the threat model first.

Threat: an agent in a sandbox (or content it
pushed to origin) tries to execute code on the host, exfiltrate host
secrets, widen its own capabilities, forge or hide work, or destroy
work; or it is steered by text it reads from GitHub or the web; or it
persists instructions to a later session through memory. The host
operator and the romeu release are trusted. A release is what
`release.yml` built and attested
([10 10.2](10-testing-style.md#release-and-bootstrap)); what review,
commit signatures and the attestation are worth today is in 5.4.
Egress gating is defense in depth, not containment.

## 5.1 Hardened git (`internal/gitsafe`)

gitsafe is the one runner of git in both binaries, with two named
modes. **Host mode** is every git subprocess romeu starts. It runs with
a fresh environment (nothing inherited; in particular
`GIT_EXTERNAL_DIFF`, `GIT_CONFIG_*`, `GIT_SSH_COMMAND`, `GIT_ASKPASS`,
`GIT_DIR`, `GIT_EXEC_PATH` are never passed through) and with every
flag of this list, which is the one list of the keys gitsafe
neutralizes:

```
env:   GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
       GIT_ATTR_NOSYSTEM=1 PATH=/usr/bin:/bin HOME=<home> LANG=C
flags: -c core.hooksPath=/dev/null -c core.fsmonitor=false -c submodule.recurse=false
       -c protocol.allow=never -c protocol.https.allow=always -c protocol.git.allow=always
       -c core.sshCommand=false -c core.gitProxy= -c diff.external= -c core.pager=cat -c credential.helper=
       -c fetch.prune=false -c transfer.fsckObjects=true -c transfer.bundleURI=false
       -c gc.auto=0 -c maintenance.auto=false -c core.symlinks=false
       -c advice.detachedHead=false
```

- `protocol.git.allow` is needed only for the sandbox daemon on
  `127.0.0.1`; gitsafe refuses a `git://` URL whose host is not
  `127.0.0.1`, and refuses `file://` URLs and plain paths everywhere
  (bundles are read with `git bundle verify`/`unbundle`, not fetched by
  path). There is **no test override**: tests serve origins over HTTPS
  (see [10](10-testing-style.md)).
- Fetch always uses an explicit URL and refspecs, `--no-tags --no-prune
  --no-recurse-submodules --no-write-fetch-head`, never a remote name
  (a fetch that brings nothing then writes nothing, which the
  idempotency rule of 04 4.1 needs). Destination prefixes
  are allowlisted: `refs/romeu/origin/<dir>/`, `refs/sandboxes/<name>/`,
  `refs/romeu/snapshots/<name>/`; salvage refs are created only with
  `CreateRef` (create-only `update-ref`).
- One key is not neutralized: a host clone's local `url.<base>.insteadOf`
  rewrites an explicit fetch URL, and `-c` cannot remove it, because it
  appends to a multi-valued key. A host clone's local config is written
  only by the operator and sbx and is trusted, so a local rewrite to
  another HTTPS host redirects a fetch and is not guarded. A rewrite
  to a protocol gitsafe refuses is refused, and so is a rewrite whose
  target is `git://127.0.0.1` for a URL that was not.
- Credentials: only `-c credential.helper=<host settings value>` when
  `tools.gitCredentialHelper` is set; a `gitHosts[].caFile` becomes
  `-c http.sslCAInfo=<path>` for that host only (also how the E2E
  origins of [10 10.1](10-testing-style.md#101-test-levels) trust their
  test CA).
- `transfer.fsckObjects` checks objects only as they are transferred: it
  does not cover `git bundle unbundle`, objects already in the
  repository, or case-collision trees. So every tree romeu materializes
  gets two independent checks: `git fsck --strict <commit>` before the
  walk, and the tree walk's own validation (`WalkTree`: `ls-tree -r -z
  --full-tree` at the commit, then the name, mode and collision rules of
  I27 in Go), which does not rely on fsck.
- `sync --from <sha>` accepts only commits reachable from
  `refs/romeu/origin/*`, so a commit that arrived from the sandbox
  daemon or through a salvage or snapshot bundle is never a spec
  source.
- A unit test asserts that gitsafe still refuses `file://` URLs and plain
  paths when a `gitHosts[].caFile` is set.
- A review checkout's local config holds exactly these keys, so a
  plain `git` the operator runs there later is hardened too:
  `core.hooksPath=/dev/null`, `core.fsmonitor=false`,
  `submodule.recurse=false`, `core.symlinks=false`,
  `protocol.allow=never`, `protocol.https.allow=always`,
  `core.sshCommand=false`, `core.pager=cat`, an empty
  `credential.helper` and `transfer.fsckObjects=true`. romeu writes no
  other git config (I2).
- Host git floor: the latest patched point release of each supported
  series, initial floor 2.45.4; the exact per-series minimums and the
  CVEs they close are verified in probe A1 and recorded in
  `internal/gitsafe/floor.go` with sources.

**Sandbox mode** is every git call julieta makes (clone, fetch,
fast-forward, bundle, unbundle, `commit-tree`). It uses the same fresh
environment (no ambient `GIT_*` variable), explicit URLs and refspecs,
and `-c core.hooksPath=/dev/null -c core.fsmonitor=false
-c submodule.recurse=false`. It keeps the sandbox's global config,
which the sandbox owns and the agent can write, so nothing in that
config is trusted beyond the sandbox; this is why salvage passes
`-c commit.gpgsign=false`
([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)).
One gitsafe table test pins the environment and flags of each mode,
and gitsafe exports the neutralized list above, which `doctor`'s
`host-git-config` check and the I3 plant list read.

## 5.2 Invariants and their tests

Each guard carries `//romeu:invariant I<n> guard` and each test
`//romeu:invariant I<n> test`. The table below is the one list of
invariants: `tools/ci invariants` reads the ids from it and pairs the
tags it finds in the code. An id with a guard tag has a test tag and
the reverse, and each tagged id is a row of the table. That each row
has both tags is checked once, by `tools/ci acceptance` for S6 in the
final plan task
([10 10.5](10-testing-style.md#105-acceptance-evidence)), so a row can
be written before its code without turning `all` red; until then a row
with no code is caught by the plan and by review. A guard deleted
together with its test and both tags is caught by the diff instead:
`tools/ci pr` fails a pull request that removes the last guard tag or
the last test tag of an id whose row it keeps. Removing an invariant
means removing its row in the same pull request, on the specification
surface (5.3).

`tools/ci mutate` builds each guard's `mutate_I<n>` stub (build tag)
and requires at least one tagged test of that id to fail, at the test
levels the runner has. A stub is a pass-through of the guarded
decision, the guard admitting what it refuses, and nothing else; the
red must come from a tagged test's own assertion, and a panic or a
build failure does not count. An id with no tagged test that can run on
a runner is reported as not run there, and it must fail on a Linux
runner, which has every CI level. One stub per id covers one guard
site; a stub per site is a row of
[Deferred decisions](../spec.md#in-the-product). Where the invariant is
an absence (I1, I2, I14), the guard tag sits on the check that observes
it (the `imports` rule, the scan, the filesystem-diff helper) and the
tagged test is a fixture with one violation. An absence test also has a
positive control: the same plant, run against unhardened git or under
the guard's pass-through stub, must produce the marker, the `.git`
diff, the trust write or the removal, and the test fails if it does
not, so an inert plant cannot pass for a working guard (I2, I3, I14,
I16). `go run ./tools/new invariant` scaffolds the next id with both
tags.

Where a static scan admits a call (I16), an admitted call site is a
named function, by package and function name, in one table under
`tools/ci`, so on the `checks` surface. The scan fails each listed call
kind outside a listed function. Whether a path is a temporary file, a
candidate dir or an unreferenced kit dir is known only at run time, so
those functions' unit tests prove it and the scan does not. An admitted
call site widens I16, so the table stays on the `checks` surface and
each admission carries the `checks` approval line; a marker comment in
the admitted function would let a pull request admit itself.

Test levels: U = unit, E = e2e with
fake sbx (CI), X = hybrid e2e (fake sbx forwarding into the julieta
container, CI), C = container (CI), H = host.

| ID | Invariant | Test |
|---|---|---|
| I1 | **No pane or shell ever runs on the host.** `cmd/romeu` has no multiplexer or terminal code; `run` ends by exec'ing exactly `sbx env exec -it <dir> --env JULIETA_MANIFEST=... -- julieta layout up` | U: `go list -deps ./cmd/romeu` excludes `internal/layout`, `internal/hooks`, `internal/mise`; E: final argv asserted byte for byte |
| I2 | **The host never configures hooks** and never writes git config except a review checkout's local keys, the one set of 5.1 | U: import graph; E: after every romeu command, each clone's `.git/` that existed before the command (minus `objects/`, `refs/`, `logs/`, `packed-refs`, `index`, `FETCH_HEAD`, `ORIG_HEAD`) is byte-identical to before, including `hooks/`, `config`, `info/attributes`, `objects/info/alternates`; in `config` the `remote.sandbox-<name>.*` keys are left out of the comparison, because sbx writes them (01 1.2); a review repo's local config holds exactly the set of 5.1; positive control (above): the same run with the guard's stub changes `config` |
| I3 | **romeu never executes repository content or ambient git configuration** | E: plant in repos, in a hostile global config and in the process ENV: every key of the 5.1 neutralized list (a test holds the plant list equal to the list gitsafe exports), `core.hooksPath=.githooks` + tracked hooks, `.gitattributes` filters with global `filter.x.*` and a global lfs filter, `include.path`, `alias.*`, a global and a local `url.*.insteadOf` that rewrite to a refused protocol and to `git://127.0.0.1` (a rewrite to another HTTPS host is the stated limit of 5.1), and ENV `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_COUNT`/`KEY_0`/`VALUE_0`, `GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_DIR`, `GIT_EXEC_PATH`; run every romeu command; the marker file never exists, and the positive control (above) makes it exist |
| I4 | **No `--auto-approve`**, ever | U: every sbxdrv builder scanned; E: fake sbx exits 99 on it |
| I5 | **Fetched content lands only in allowlisted namespaces**; never `refs/heads`, `refs/tags`, `refs/remotes` or `HEAD`; never checked out except into `review/` | U: refspec validator table; E: pull + salvage + sync, then `refs/heads`, `refs/tags`, `refs/remotes`, `HEAD` unchanged |
| I6 | **Salvage refs are immutable** (create-only, never a fetch destination, never pruned) | E: fetch with `--prune` and a forced refspec after a same-name recreate; salvage refs survive; a second create on the same ref fails |
| I7 | **No unapproved widening is ever live or acted on**: candidates stay in `.romeu/candidates/`; every command that makes a mutating sbx call runs the preflight first; an approval needs a TTY and binds to the candidate `sync` rendered and its digests; a promotion is a crash-safe commit. A gate 2 change is an approval record whose widening digest no longer matches the live files, or live files with no approval. Fields deliberately left ungated are on one committed list beside the gate tags, and `go generate` fails on a field that is on neither (01 1.4) | U: flip table generated from the struct tags: each `gate:"widening"` field flips the widening digest (repos, `ref`, secrets name and argv, workload repository/digest/capabilities, kit content/args/capabilities, gated egress, ports, `agent`, `salvage.excludeIgnored`, the normalized render), and each field without the tag (`sandboxOptions`, `run`) does not; U: gate 1 flips on sbx version, romeu's version and sha256, the embedded julieta's sha256 and the catalog digest, and the prompt prints `downgrade` beside a version lower than the recorded one; U: candidate state-machine table (every row, every illegal pair); E: for each gate 2 change and each gate 1 change (different `sbx version`, different catalog digest), across every romeu command: `run`, and a `sync` whose sandbox is running, exit 3 before any mutating call; `stop`, `salvage`, `salvage --from-host` and `pull` print the failure and go on, widening nothing; `rm`, `recreate` and `retire` run their salvage half and exit 3 before `sbx env rm` (01 1.5); a `sync` without a TTY whose digest differs exits 3 and leaves no candidate; a declined candidate leaves live files byte-identical; E: fault injection kills romeu after each promotion step, then every **P** command either finishes the promotion or refuses, and a re-sync converges |
| I8 | **Secrets by name only**; `name@project` must match the project exactly | U: decode rejects `command`/`argv`/`env` under `secrets`; golden shows `/usr/bin/env -i ...`; E: unknown name exits 2; `github@other` does not satisfy project `shop` |
| I9 | **Egress from pinned commits** in `refs/romeu/origin/<dir>/<ref>` only | E: uncommitted lock change, a commit on another branch, and a local `refs/remotes/origin` rewrite all leave egress unchanged; changing `ref` changes egress and requires gate 2 |
| I10 | **Upload-capable and unknown-flag domains are always gated**, including catalog, base and kit-declared domains | U: absent `upload` = gated; `core:node` puts `registry.npmjs.org` in gated; a kit declaring `x.example` gates it |
| I11 | **YAML and JSON only via marshallers** | U: `tools/ci imports` rejects `text/template` in `internal/render`; fuzz round-trip of every spec string field |
| I12 | **Strict validation of names, paths and file types** before use; memory reads via `os.Root` | U: table + fuzz on validators and ref names; E: memory dir with a symlink, a FIFO and a `..` name is refused, never followed (a device node needs root to create, so probe A3 plants it) |
| I13 | **Workspace files contain folders only**; memory dirs never; review checkouts only in `review.code-workspace` | U: golden + decoded key set == {`folders`} |
| I14 | **romeu never writes VS Code trust or settings**; doctor fails on a trusted root or disabled trust (`vscode-trust`) | U: filesystem-diff test over a temp HOME after every command, with the positive control (above); doctor fixtures |
| I15 | **Drift detection** before overwrite and before every sbx call of a **P** command | E: hand-edit `sbxenv.yaml`, a kit file, `.romeu/bin/SHA256SUMS`; `sync` and every **P** command exit 4 |
| I16 | **romeu deletes nothing it does not own** (clones, memory, salvage refs, `.attic`) | U: `tools/ci` scans romeu-linked packages, shared ones included, for `os.Remove*`, rename-over, `O_TRUNC`, `update-ref -d`, `branch -D`, `gc --prune` and `sbx env rm`, and fails each outside the functions the admitted-call-site table names (above). Those functions remove temp files, candidate dirs and kit dirs no longer referenced by the live `sbxenv.yaml`, replace files under `.romeu/bin/` by rename, and run `sbx env rm` for `rm`; their unit tests prove what they touch; the scan's positive control is a fixture with one call outside the table; E: fs snapshot diff |
| I17 | **Salvage is complete and verified before destruction**, over the generation's recorded repo set | U: generation state-machine table (every row, every illegal pair); E: `rm` exits 5 for: a missing repo in the manifest, a forged `complete: true` manifest, a hidden branch absent from the bundle, a reflog-only commit absent from the bundle, a detached HEAD commit, dirt in a linked worktree, an unbundled tag, a corrupted payload file, a salvage id mismatch, daemon heads not in the bundle, a salvage ref of an earlier generation re-bundled under `refs/salvage/<generation>/`, and a repo removed from the spec after the generation was created (J13) but absent from the manifest; E: destructive-command fault injection: a test hook kills romeu after each step of `salvage`, `rm`, `recreate` and `retire`; the next `run`, `salvage` or `rm` converges, no salvage record says lost after a complete one, and `sbx env rm` is never reached without a complete or accepted record of this generation; E: `sbx env rm` fails after `removing` is written: the command exits 1, the generation stays `removing`, and `rm` run again passes the identity step and resumes at `sbx env rm` |
| I18 | **Salvage commits are never signed and never pushed**: julieta signs none, and romeu pushes none | C (julieta): no signature on salvage commits even with `commit.gpgsign=true` configured; U (romeu): gitsafe builds no push argv |
| I19 | **Daemon port re-resolution** before each sandbox fetch | U: stale-state table; E: fake sbx changes the daemon port between calls and reports stopped; fetch uses the new URL or is not attempted; H: real port change |
| I20 | **Host-only state is never under `$ROMEU_ROOT` or mounted** | U: init/doctor refuse such roots |
| I21 | **Repo hooks run only in the sandbox**, through julieta's dispatcher | C: dispatcher runs `.githooks/pre-commit` and `$GIT_DIR/hooks/post-commit` in the container |
| I22 | **Per-project isolation is not bypassed by sbx globals**: doctor fails on global secrets for used services and broad allow rules (`sbx-global-secret`, `sbx-broad-allow`); a listing whose shape is not a recorded shape, or that cannot be parsed, fails the check (`upstream-shape`, [03](03-formats.md) opening) | U: doctor fixtures from block A recordings, plus a changed container shape and an unparseable body, each exiting 2 with `upstream-shape`; on the fixtures with a global secret or a broad allow rule, doctor exits 6 (`check-findings`) |
| I23 | **None of four host tools auto-trusts `$ROMEU_ROOT`**: dotfiles symlinks under `$HOME`, mise trust paths, direnv allow lists, VS Code trust (`home-symlink`, `auto-trust`, `vscode-trust`); another tool's trust model is the operator's to check | U: doctor with temp HOME fixtures |
| I24 | **Memory dirs are data with a fixed layout**: the layout of [08 8.1](08-memory-handoff-salvage.md#81-memory-store), its one home; romeu reads them only through the `os.Root` readers of `memstore`; romeu extracts no archive from them other than a git bundle, through gitsafe (`bundle verify`, `unbundle`, `fsck --strict`) | U: the memstore allowlist table, whose expected members come from the one Go list that the reference renders; U: `tools/ci imports` fails a romeu-linked package that imports `memstore/write` (10.7). That no other romeu code opens a memory path on its own is **[review]**: a scan cannot tell what a path is at run time; E: violations make `sync`/`run` exit 2 and `julieta memory check` exit 6 (`check-findings`), and exit 1 on an unmounted or unwritable dir; E: a leftover julieta temp file (08 8.1) makes them warn, never refuse; E: a `<name>-env/` or `.attic` its group or others can access fails `doctor` (`tree-mode`) |
| I25 | **romeu never handles secret values**: no argv, env or file written by romeu contains a secret value; `sbx secret set` is only ever called with `--command` | U: sbxdrv builder table rejects `-t`/value forms; E: fake sbx exits 99 on a value form. A host-settings argv that looks like a token is refused at load (`secret-in-argv`, [03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)), a best-effort guard on the shapes it knows |
| I26 | **Sandbox identity**: romeu acts only on sandboxes with an open generation it recorded (at create or by `adopt`) and whose workspace path equals `<name>-env/<primary>`. The generation also records the sandbox's id or creation time when probe A4 shows that `sbx ls --json` reports one, and I26 then compares it; until then a sandbox recreated by hand under the same name and path passes | E: a foreign same-name sandbox exits 2 with `RJ-203` naming `romeu adopt`, before any mutation; a path mismatch exits 2; `adopt` without TTY, with a path mismatch, or with an open generation exits 2; after `adopt`, `salvage` and `rm` succeed; a sandbox recreated by hand under the same name and path passes, a known pass until A4 reports |
| I27 | **Kit materialization cannot escape or smuggle**: `git fsck --strict` on the commit, then the walk's own validation, independent of fsck: only modes 100644, 100755, 040000; symlinks and gitlinks rejected; names `.`, `..`, `.git` and case-insensitive or HFS-ignorable variants (for example `.GIT`, or `.git` with a U+200C ZERO WIDTH NON-JOINER inserted after the dot) rejected; case collisions under case-insensitive comparison rejected; written through `os.Root` into the candidate only; `sync --from <sha>` refuses commits not reachable from `refs/romeu/origin/*` | U: table of hostile trees; E: a personal kit with each hostile entry fails sync before any file is written outside `.romeu/candidates/`; E: a hostile tree delivered by a bundle is refused as `--from <sha>`, and refused by the walk when made reachable; the E cases also run on the macOS runners (case-insensitive APFS) |
| I28 | **Kit and workload capabilities are in the gate, and romeu parses them the way sbx does**: descriptors parsed with the strict subset grammar (no anchors, aliases, merge keys or includes; any unmodeled key or capability type refused); normalized capability set digested; a personal kit that declares a network, mount, volume, credential or ssh-agent capability is refused; `internal/oci` verifies the digest of every hop (index, platform manifest, annotation), reads anonymously (never Docker config credential helpers), over HTTPS only, follows only HTTPS redirects, and caps every response size | U: descriptor fixtures; U: one conformance fixture per capability type, recorded against the pinned frontend in probes A11 and A12, parsed identically by romeu; each fixture records the sbx version and frontend digest it was made with, and a frontend pin change, or an sbx version entering the tested window, runs the A11 and A12 conformance subset again, named in the PR's Evidence; U: `oci` table (digest mismatch at each hop, HTTP URL, HTTPS-to-HTTP redirect, oversize body, credential helper present in a fake Docker config) all refused; E: adding a capability to a kit flips gate 2 |
| I29 | **All host output is escaped, and the gate diff is complete**: the CLI writer escapes by default; control characters (U+0000-U+001F, U+007F-U+009F), bidi codepoints, zero-width characters (U+200B-U+200D, U+2060, U+FEFF), tag characters (U+E0000-U+E007F) and U+2028/U+2029 are rendered visibly in gate diffs, julieta reports, handoff bodies and ref names; the gate diff is never truncated and shows binaries as size + sha256, flagged | U: fuzz `termsafe`; U: every escaped class has a table row; E: a handoff and a branch name with ESC sequences print escaped; E: a 10 000-line kit file change appears in full in the gate diff, with the summary above the prompt; a binary kit file is shown flagged with its size and sha256 |
| I30 | **julieta is delivered read-only and checked for compatibility**: romeu invokes julieta by absolute path in the read-only `.romeu/bin` mount and continues past the compatibility check only when the protocol is accepted, the binary sha256 equals `render.json`, and julieta reports its directory read-only; the check proves compatibility, not integrity (5.4). julieta also runs in the config repo's CI from a release pinned by version and checked by `julieta pin check --workflows` ([02 2.2](02-layouts.md#22-config-repo)), where no romeu exists | X: protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each make `run` exit 2 before `layout up`; a 32 KiB manifest round-trips; H: probe A3 |
| I34 | **The signing socket forwards one key**: `git-ssh-sign` renders only when `signing.agentSocket` holds exactly one key, equal to `signingKey`; `SSH_AUTH_SOCK` is set only for that project's sbx calls ([06 6.4](06-kits.md#64-product-kits)) | U: identity listing table (no key, two keys, one other key, the one key); E: `sync` refuses with `RJ-204` |

I31 to I33 are reserved for the runtime ledger, which is designed and
not built in v1; their text is in
[13 13.10](13-runtime-ledger.md#1310-tests), and the ids stay unused
until the ledger lands.

## 5.3 Ask-first surfaces

`.github/ask-first.yaml` is the single list of paths whose changes need
the maintainer's explicit approval. `go generate` derives from it the
table in `docs/reference/ask-first.md` and `.github/CODEOWNERS`, which
names `owner` for each glob; `tools/ci mutate` reads it to decide which
PRs run mutation testing (a `.go` file on a listed path, or a file
holding an invariant guard tag, so a guard outside every surface is
tested too); `tools/ci pr` requires an approval line in the PR body for
each surface the diff touches. A path may match several surfaces; an
approval line names one surface id, so such a path needs one line per
surface it matches. The
approval line, the ruleset's requirements, and which revision of this
file `pr` reads are defined in one place,
[12 12.4](12-engineering.md#124-middleware-before-and-after-every-change);
what an approval is worth with one account is in 5.4. At a plan
checkpoint the maintainer may grant the approval lines of a list of
tasks on some surfaces, recorded in `docs/grants.yaml`, which is on the
`ask-first` surface; which surfaces can be granted, and the form of a
grant, are in 12 12.4. The `approvals` surface holds
`tools/ci/askfirst.go`, `tools/ci/denylist.yaml` and
`.github/workflows/**`, the check of approval lines and grants, the
forbidden-name denylist and the files that choose what runs, which no
checkpoint grant covers (they stay on `checks` and `dependencies` too).
`tools/ci pr` builds and runs the code of the default branch against
the head, so no pull request is judged by code it changes (round 8, G5,
decided on 2026-10-09); its job is defined in the default branch too, so
a change to a workflow file does not change the job that judges it
([12 12.4](12-engineering.md#124-middleware-before-and-after-every-change)).

The file lists itself and `.github/CODEOWNERS`, which `go generate`
derives from it. The `dependencies` surface holds every path where
mise reads its configuration when it starts at the top of the tree
(`miseFiles` in `tools/ci/misefiles.go`, which a test holds equal to
these globs), as measured at the pinned mise; a change of the mise pin
measures config discovery again at the new version, named in the PR's
Evidence. `owner` is the maintainer's GitHub handle, the `<owner>` of
the module path (02 2.1), which is public already. Each surface has
that one owner; an outside pull request on a surface is reviewed by
the one maintainer, best effort, and a second owner is a row of
[Deferred decisions](../spec.md#in-how-this-repository-is-run).

Round 3 decided that the decision records and the code of the
checks are surfaces
([round 3](../reviews/round-3.md#maintainer-decisions)): the workflows
only call `tools/ci` (10 10.2), so the check code is the gate. The steps
of `all` check a change to `tools/ci` with the changed `tools/ci`, so
there its only check is the `checks` approval line, or a checkpoint
grant that stands in for that line, while `pr` runs
the default branch's code (above); running the base branch's `tools/ci`
for the steps of `all` too is a row of
[Deferred decisions](../spec.md#in-how-this-repository-is-run). The
`spec` surface was added on 2026-10-08
([round 8](../reviews/round-8.md#decisions-for-the-operator), AR13):
the three pages of this specification whose tables the checks read
(the vocabulary of 01 1.7, read through
`docs/reference/vocabulary.yaml`, and the tables of the index) and the
security model on this page. The other pages are not on it, so most
spec edits need no approval line. Two paths wait for a surface:
`internal/sbxdrv` (with `e2e/fakesbx`), which holds the I4 and I25
guards, joins a new `sbxdrv` surface, and `skills/**`, the text agents
follow, joins `kits`, both in their own tooling pull request on the
`ask-first` surface. That pull request is the trigger; until it
merges, a change to them needs no approval line, and `tools/ci mutate`
already runs on the `sbxdrv` guards through their tags. One piece of
gate logic sits outside these globs and is reviewed like any other
code: the test in `e2e/scenarios` that fails a command-table entry no
scenario reads (rule 13 of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)).

```yaml
version: 1
owner: "@<owner>"
surfaces:
  - id: gates
    globs: [internal/gate/**, internal/state/**, internal/spec/project.go]
    reason: the gates, the state machines, and the gate/apply struct tags that define the widening set
  - id: gitsafe
    globs: [internal/gitsafe/**]
    reason: every git invocation on the host, ref namespaces including salvage
  - id: termsafe
    globs: [internal/termsafe/**, internal/cli/writer.go]
    reason: host output escaping
  - id: digests
    globs: [internal/canon/**]
    reason: every digest the gates compare
  - id: signing
    globs: [internal/signing/**, kits/git-ssh-sign/**]
    reason: commit signing key isolation
  - id: catalog
    globs: [catalog/egress.yaml]
    reason: upload flags and the base and meta sets
  - id: kits
    globs: [kits/**, tools/kitpin/**]
    reason: what enters every sandbox, and its pins
  - id: release
    globs: [.github/workflows/release.yml, tools/release/**]
    reason: release and publishing paths
  - id: dependencies
    globs: [go.mod, go.sum, mise.toml, mise.lock, .mise.toml, mise.*.toml, .mise.*.toml, mise/**, .mise/**, .config/mise.toml, .config/mise.*.toml, .config/mise/**, .tool-versions, .miserc.toml, .miserc.local.toml, .config/miserc.toml, .github/workflows/**]
    reason: Go dependencies, the pinned tools the gates run with every file mise reads as its configuration, and CI actions
  - id: contracts
    globs: [internal/cli/errors.go, internal/spec/versions.go]
    reason: exit codes, error ids and file format versions
  - id: ledger
    globs: [internal/ledger/**]
    reason: the runtime ledger's code, its event schema, and the tags that decide which fields cross projects
  - id: decisions
    globs: [docs/adr/**, .adr-dir]
    reason: decision records; one leaves Proposed, or changes after that, with the maintainer's approval
  - id: spec
    globs: [docs/spec.md, docs/spec/01-system-model.md, docs/spec/05-security.md]
    reason: the gates read tables in these pages (the vocabulary in 01 1.7 and the index), and 05 holds the security model
  - id: approvals
    globs: [tools/ci/askfirst.go, tools/ci/denylist.yaml, .github/workflows/**]
    reason: the approval-line and grant check, the forbidden-name denylist, and the workflows that choose what runs
  - id: checks
    globs: [tools/ci/**, .githooks/**, .golangci.yml, docs/acceptance.json]
    reason: the code of the gates and reports, with the forbidden-name denylist (tools/ci/denylist.yaml), the linter configuration, and the acceptance file, whose command items tools/ci acceptance runs
  - id: ask-first
    globs: [.github/ask-first.yaml, .github/CODEOWNERS, docs/grants.yaml]
    reason: this list, the CODEOWNERS file generated from it, and the checkpoint grants that stand in for approval lines
```

## 5.4 Known residual risks (accepted in v1)

The risks fall in two groups: those of running romeu, which every
operator carries, and those of how this repository is developed, which
the maintainer carries. No row is wider or narrower for being in one
group or the other.

### Risks of running romeu

| Risk | Why accepted | Mitigation |
|---|---|---|
| The operator runs plain `git` in a review checkout or host clone with a risky global config | romeu cannot control later manual commands | the local keys of 5.1 in review repos; doctor warns (`host-git-config`); Restricted Mode guidance. A review checkout of a secondary repo is checked with `git fsck --strict` before it is written (04, `romeu pull`); names that collide on a case-insensitive disk are left to git's checkout and to Restricted Mode |
| Host clones carry origin content that agents may have pushed | agents push through the project's token | open both workspaces in Restricted Mode; the sandbox holds whatever credential the operator binds for `name@project` ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1), Q13), and a broad one widens this risk, so the guide advises one fine-grained token per project, limited to its repositories (J1 step 5); the `julieta-claude` kit refuses `gh pr merge`, a push of a `v*` tag and `gh api` calls to rulesets and repository settings in a PreToolUse hook ([06 6.4](06-kits.md#64-product-kits)), a guard against mistakes and not against an agent that edits its own hook |
| A domain-only allow on `github.com` permits push exfiltration of anything the agent can read | the sandbox legitimately pushes | egress is not containment; the sandbox holds whatever credential the operator binds for `name@project` (03 3.4), and a broad one widens this risk |
| A "Verified" signature on a commit made in the sandbox means the signing key was used, not that a human wrote it | the agent commits with the forwarded signing agent | `sync` refuses `git-ssh-sign` unless the dedicated socket holds exactly one key equal to `signingKey` (I34, 06 6.4); the guide adds that key with `ssh-add -c`, so each use asks for confirmation (06 6.4, J1 step 5); salvage commits are never signed (I18). A signing key per project, or a signing path that is not forwarded, is a row of [Deferred decisions](../spec.md#in-the-product) |
| The workload image is unsigned | upstream does not sign it yet | digest pin; repository allowlist; capability set in the gate |
| Local v3 kits are unsigned | local v3 kits cannot be signed | trust = romeu release + config commits at an approved digest; publishing and signing product kits is a row of [Deferred decisions](../spec.md#in-the-product) |
| `os-base`'s apt delta floats with the Debian mirror at build ([06 6.4](06-kits.md#64-product-kits)) | `os-base` exists only if A12 reports packages the workload lacks, and its set is that minimum | pinning it is a row of [Deferred decisions](../spec.md#in-the-product), reopened by the first breakage traced to the delta or by `os-base`'s set growing past the A12 minimum |
| The final `sbx env exec -it ... layout up` passes raw bytes between the sandbox and the host terminal | it is the interactive session itself; escaping it would break the terminal | romeu exec's into it as its last act and prints nothing after; the host terminal is the only consumer |
| Secondary repos have no independent daemon cross-check at salvage | sbx runs a git daemon only for the primary repo | bundle verification, the manifest's ref list, and host-provided bases; J13 keeps a removed repo in the generation's repo set |
| Salvage proves that nothing the sandbox still holds is lost. It does not prove what an agent removed before salvage: the manifest, the bundles, the snapshots and the daemon's repository all live inside the sandbox, where the agent is root | the threat model lists hidden and destroyed work, and nothing inside a sandbox can witness the agent | a push to origin is a copy the agent can also take back when the project's token can delete branches and tags; I17 holds against loss and against a forged manifest; a create-only host ref per fetched origin head is a row of [Deferred decisions](../spec.md#in-the-product) |
| Salvage payloads of closed generations (`ignored.tar.gz`, `agent/`, opt-in transcripts) can hold credentials and code; they stay in the mounted memory dir, readable and writable by later sandboxes of the project, and in any backup of `$ROMEU_ROOT` | accepted on 2026-10-08; moving them out of the mount waits for a second adopter or a shared host | dirs 0700 and files 0600; salvage prints that the payload is secret-bearing when it stores one ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)); transcripts only with `--include-transcripts`; moving closed salvage out of the mount is a row of [Deferred decisions](../spec.md#in-the-product) |
| An adopted generation is accepted as it stands until it is recreated: its mounts, policies, secrets and image were not rendered or gated by romeu | refusing `adopt` would lose the J3 recovery | `adopt` prints, escaped, the inventory sbx reports before its TTY prompt; `status` shows `adopted, not gated` until a recreate ([01 1.6](01-system-model.md#generation)); probe A4 records which fields `sbx ls --json` gives |
| A protective command (`stop`, `salvage`, `salvage --from-host`, `pull`, and the salvage half of `rm`, `recreate` and `retire`) goes on past a failed gate 1, gate 2 or step 5 check ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)), so it runs an sbx whose version the operator has not acknowledged | refusing would let a sandbox die with unsaved work (decided on 2026-10-09) | it widens nothing and forwards no socket (01 1.5); drift and identity still stop it; the failure is printed before the first sbx call |
| `sandboxOptions` is ungated: a merged spec can size the sandbox up to what sbx allows | every spec change is a pull request only the operator merges | it is on the committed list of ungated fields (01 1.4, I7); reopened by the first spec merged without the operator merging it |
| `herdr` is pinned by version and per-arch sha256, and its upstream publishes no checksum file and no signature: the hash proves the binary has not changed since publication, not who built it | the upstream offers nothing stronger (measured 2026-10-01, 06 6.4) | `tools/kitpin` takes the hash from the release API's digest and refuses a release that is not marked immutable; the pin is a reviewed diff on the `kits` surface; herdr runs inside the sandbox only; a second run-layout renderer is a row of [Deferred decisions](../spec.md#in-the-product) |
| Between a run and the next `sync`/`run`, an agent can plant files in a memory dir; an operator who opens that dir with host tools, or a host indexer that parses it (Spotlight, Quick Look, Time Machine), sees them | memory dirs are shared by design and checked only when romeu reads them | read memory through `romeu handoff`, or with a pager that shows bytes and runs nothing (`less`, `cat -v`); never open a memory dir as a workspace folder; `sync` writes the `.metadata_never_index` marker and `doctor` warns on an indexed or synced root (`root-indexed`); doctor and the preflight check trust settings (I23); `romeu memory list\|show` is a row of [Deferred decisions](../spec.md#in-the-product) |
| Memory and handoff text is agent text that later sessions and the operator read: SessionStart prints it into every later session, and `romeu handoff` prints it to the operator | memory exists to carry text across sessions | the SessionStart print and `romeu handoff` label the text as written by an agent, and SessionStart bounds its size ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)); the operator runs nothing a handoff asks without reading it as data |
| The compatibility check executes the binary it checks | a check run inside the sandbox cannot attest itself | integrity comes from the read-only mount and the host-side drift check of `.romeu/bin`; the check only proves compatibility |

**Release proof.** v1 releases carry `checksums.txt` and a keyless
build provenance attestation
([10 10.2](10-testing-style.md#release-and-bootstrap)), and no other
signature. The attestation proves that `release.yml` built the archive,
not that the maintainer approved the release. An operator signature
over `checksums.txt`, made outside GitHub, is a row of
[Deferred decisions](../spec.md#in-the-product).

**Safe to paste.** The details of an error
([04 4.1](04-cli.md#41-conventions-both-binaries)) and the output of
`romeu version --json` hold no secret value, since romeu never handles
one (I25), so both can be pasted into an issue.

### Risks of how this repository is developed

These rows describe the development sandbox (01 1.7), the sandbox this
repository is developed in today, which romeu does not manage. A
sandbox romeu renders forwards one key through `signing.agentSocket`
(06 6.4), and its GitHub token is the `github@<project>` binding (Q13).
The record is
[ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md).

| Risk | Why accepted | Mitigation |
|---|---|---|
| The approval line in a PR body is typed by the PR's author, and it names what was approved, not who approved it | a check can prove that the approval was recorded, not who gave it; with one account and one token for everyone (ADR 0008, decision 4) the line is a record and a guard against mistakes, not a barrier against an agent. A signed approval record kept on the host is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) | the default-branch ruleset requires CI green and refuses a direct push to the default branch; it requires no review (measured 2026-10-08, ADR 0008). The approval line, checked by `tools/ci pr` ([12 12.4](12-engineering.md#124-middleware-before-and-after-every-change)), is the only recorded approval; the next row says what it is worth |
| The development sandbox's GitHub token is the operator's own: a merge, a `v*` tag and a change to a ruleset or a repository setting are process only against an agent that holds it. Its reach is every repository the account can reach, those of every organization the account is a member of included; that reach is inferred from the account's rights until probe A16 reads the token's type and scopes ([11 11.1](11-host-probes.md#111-block-a---sbx-and-runtime-facts-first-in-parallel-with-the-first-build-layer)) | it was decided on 2026-10-07, for now, and a second account was declined. It collides with rule 1 of [ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md), which says a merge gate is a need of the start; the collision was accepted by name on 2026-10-07. The merge-gate parts deferred with it are accepted with it, each a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) that says so: a check that reads the rulesets again, a signed approval record, required status checks without the administrator bypass, and running the change's tests in a separate job. Of four tries with that token, only a direct push to the default branch was refused ([10 10.2](10-testing-style.md#release-and-bootstrap), item d) | the GitHub security log is read at each sitting, as a judgment and not as a detector. Its documented coverage, read on 2026-10-09 in [GitHub's documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/reviewing-your-security-log) at commit `9f651797567230e844373870fce8b14427ad47ad` of `github/docs`: for repositories the account owns, 90 days of ruleset creation, update and deletion, visibility, merge-setting, default-branch, rename and archive changes, Actions settings, secrets and variables, and members; no pull request merge, push, tag creation, branch deletion or release change. A live account's log was not read ([ADR 0009, drop the required review and amend the records that assumed it](../adr/0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md), decision 17). That an agent can disable the ruleset or change the settings is inferred; a fifth try, made only with the operator present and confirming live and the revert step ready (10 10.2, item g), measures it. A moved `v*` tag is found by the acceptance check that a tag's commit equals the source digest in its attested provenance ([10 10.5](10-testing-style.md#105-acceptance-evidence)), which step 7 of the release checklist of [12 12.2](12-engineering.md#122-development-commands-and-capability-map) runs at every release after v1.0.0; a tag moved between two releases is found at the next one. The development sandbox has no PreToolUse guard of its own until it is romeu-managed (Deferred decisions). The decision returns to the operator at the events of the token narrowing row in [Deferred decisions](../spec.md#in-how-this-repository-is-run); what the token can reach is listed in classes in [ADR 0008](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md#decision) |
| Issue, PR and comment text from any account can steer an agent that holds the operator's token | the token was decided on 2026-10-07 (ADR 0008) | agents read such text as data and take no merge, close, label, workflow or settings action on it without the maintainer's word ([12 12.9](12-engineering.md#129-delivery-practices)); egress and capability widenings still meet the gates; the first issue or pull request from an account other than the operator's is an event of the token narrowing row |
| Under a `checks` grant, a change can make a step of `all` pass, and the required checks stay green; and a green `pr` result from before a fix to `pr` landed still counts until an event runs the job again | the steps of `all` run the head's `tools/ci`, and running the base's for them is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) | the `pr` job runs the default branch's code and enforces the approval lines, the grants and the forbidden names (5.3, [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change)); re-run the `pr` job of each open pull request when `pr` changes, a step for the task that builds `pr` |
| The maintainer runs agent-authored code on the host: the pre-push hook and `hygiene add` run `tools/ci`, and blocks O3 and O6 run it on a host clone; that code is merged by the token | the gates must run where the maintainer pushes | the host runs `tools/ci` only from a commit whose diff since the last host run the maintainer has read, or from the last released tag; the `checks` surface and its approval line (5.3); a trusted `hooksPath` checkout is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) |
