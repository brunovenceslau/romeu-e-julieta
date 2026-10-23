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

A table cell holds one obligation and at most three sentences; what does
not fit becomes a numbered list under the table, one item per
obligation, and the cell cites the list (03, opening).

## 5.1 Hardened git (`internal/gitsafe`)

`REQ-0458` gitsafe is the one runner of git in both binaries, with two named
modes. `REQ-0459` **Host mode** is every git subprocess romeu starts. It runs with
a fresh environment (nothing inherited; in particular
`GIT_EXTERNAL_DIFF`, `GIT_CONFIG_*`, `GIT_SSH_COMMAND`, `GIT_ASKPASS`,
`GIT_DIR`, `GIT_EXEC_PATH` are never passed through) and with every
flag of this list, which is the one list of the keys gitsafe
neutralizes:

```
env:   GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
       GIT_ATTR_NOSYSTEM=1 GIT_NO_LAZY_FETCH=1 PATH=/usr/bin:/bin HOME=<home> LANG=C
flags: -c core.hooksPath=/dev/null -c core.fsmonitor=false -c submodule.recurse=false
       -c protocol.allow=never -c protocol.https.allow=always -c protocol.git.allow=always
       -c core.sshCommand=false -c core.gitProxy= -c core.askPass= -c core.alternateRefsCommand=
       -c diff.external= -c core.pager=cat -c credential.helper=
       -c fetch.prune=false -c transfer.fsckObjects=true -c transfer.bundleURI=false
       -c fetch.bundleURI= -c gc.auto=0 -c maintenance.auto=false -c core.symlinks=false
       -c advice.detachedHead=false
```

`GIT_NO_LAZY_FETCH=1` and `-c fetch.bundleURI=` stop the repository's own
config from reaching a promisor remote or a bundle URI that CheckURL and
the gitHosts allowlist did not see.

- `REQ-0460` `protocol.git.allow` is needed only for the sandbox daemon on
  `127.0.0.1`; gitsafe refuses a `git://` URL whose host is not
  `127.0.0.1`, and refuses `file://` URLs and plain paths everywhere
  (bundles are read with `git bundle verify`/`unbundle`, not fetched by
  path). There is **no test override**: tests serve origins over HTTPS
  (see [10](10-testing-style.md)).
- `REQ-0461` Fetch always uses an explicit URL and refspecs, `--no-tags --no-prune
  --no-recurse-submodules --no-write-fetch-head`, never a remote name
  (a fetch that brings nothing then writes nothing, which the
  idempotency rule of 04 4.1 needs). `REQ-0462` Destination prefixes
  are allowlisted: `refs/romeu/origin/<dir>/`, `refs/sandboxes/<name>/`,
  `refs/romeu/snapshots/<name>/`; salvage refs are created only with
  `CreateRef` (create-only `update-ref`).
- `REQ-0463` One key is not neutralized: a host clone's local `url.<base>.insteadOf`
  rewrites an explicit fetch URL, and `-c` cannot remove it, because it
  appends to a multi-valued key. A host clone's local config is written
  only by the operator and sbx and is trusted, so a local rewrite to
  another HTTPS host redirects a fetch and is not guarded. A rewrite
  to a protocol gitsafe refuses is refused, and so is a rewrite whose
  target is `git://127.0.0.1` for a URL that was not. The same trust
  covers the rest of that local config that steers a fetch without
  running a program: the `http.*` keys (a proxy, TLS verification, a
  header) and a symbolic ref planted in a fetch destination. The list
  above still neutralizes every key that makes git run a program
  but the filter drivers below (decided 2026-10-16), so trusted config
  never becomes a way to run code. The filter drivers are the one case
  the list cannot name: `filter.<name>.clean`, `.smudge` and
  `.process` have a dynamic `<name>`, so each verb whose git command
  can run a filter, `Status`, `FastForward` and `Clone`, first lists
  the drivers the config defines (`git config --get-regexp`, run in the
  repository the command acts on, so it reads every config file the
  mode lets git read, the files they include and every conditional
  include that applies there) and runs its command with each driver's
  `clean`, `smudge` and `process` empty and `required` false, through
  `GIT_CONFIG_COUNT`; a driver with an empty name (`[filter ""]`) is
  listed and emptied too. `Clone` therefore clones with
  `--no-checkout`, lists the drivers in the new repository and then
  checks out HEAD. The listing and the command are two calls, so the
  verbs hold only while no other process writes the config between
  them, as during `julieta setup`; a driver written in that window is
  not emptied, which only a process that can already write the config
  can do. `FastForward` runs `git merge --ff-only
  --no-overwrite-ignore --no-verify-signatures`: an ignored, untracked
  file at a path the move would write a tracked file to makes the
  merge fail and name the path instead of being overwritten, and
  `merge.verifySignatures` never starts `gpg.program` or
  `gpg.ssh.program`. In sandbox mode the keys that run a program and
  that gitsafe neutralizes are `core.hooksPath`, `core.fsmonitor`, the
  filter drivers, the signature check of the merge and
  `log.showSignature` of the history reads. Every other key that runs
  a program (for example `core.pager`, `diff.external`,
  `merge.<driver>.driver`, `core.alternateRefsCommand`) stays the
  sandbox's own, among them the transport keys `core.gitProxy`,
  `core.sshCommand`, `core.askPass`, `credential.helper` and `http.*`,
  which the sandbox may need to reach origin: the agent can write
  them, and julieta runs at the agent's trust, so a program they name
  gains the agent nothing. CheckURL still admits only `https://` and
  `git://127.0.0.1` URLs.
- `REQ-0464` Credentials: only `-c credential.helper=<host settings value>` when
  `tools.gitCredentialHelper` is set; a `gitHosts[].caFile` becomes
  `-c http.sslCAInfo=<path>` for that host only (also how the E2E
  origins of [10 10.1](10-testing-style.md#101-test-levels) trust their
  test CA).
- `REQ-0465` `transfer.fsckObjects` checks objects only as they are transferred: it
  does not cover `git bundle unbundle`, objects already in the
  repository, or case-collision trees. So every tree romeu materializes
  gets two independent checks: `git fsck --strict <commit>` before the
  walk, and the tree walk's own validation (`WalkTree`: `ls-tree -r -z
  --full-tree` at the commit, then the name, mode and collision rules of
  I27 in Go), which does not rely on fsck.
- `REQ-0466` `sync --from <sha>` accepts only commits reachable from
  `refs/romeu/origin/*`, so a commit that arrived from the sandbox
  daemon or through a salvage or snapshot bundle is never a spec
  source.
- `REQ-0467` A unit test asserts that gitsafe still refuses `file://` URLs and plain
  paths when a `gitHosts[].caFile` is set.
- `REQ-0468` A review checkout's local config holds exactly these keys, so a
  plain `git` the operator runs there later is hardened too:
  `core.hooksPath=/dev/null`, `core.fsmonitor=false`,
  `submodule.recurse=false`, `core.symlinks=false`,
  `protocol.allow=never`, `protocol.https.allow=always`,
  `core.sshCommand=false`, `core.pager=cat`, an empty
  `credential.helper` and `transfer.fsckObjects=true`. romeu writes no
  other git config (I2).
- `REQ-0469` Host git floor: the latest patched point release of each supported
  series, initial floor 2.45.4; the exact per-series minimums and the
  CVEs they close are verified in probe A1 and recorded in
  `internal/gitsafe/floor.go` with sources.

`REQ-0470` **Sandbox mode** is every git call julieta makes (clone, fetch,
fast-forward, bundle, unbundle, `commit-tree`). It uses the fresh
environment plus one closed network pass-through list, exactly
`HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`, `https_proxy`, `http_proxy`,
`no_proxy` and `SSL_CERT_FILE`, each passed when set and not empty
(the set `tools/ci` passes as `networkPassThrough`), and still no
`GIT_*` variable from the caller's environment. The list is the one exception to "nothing
inherited": every egress of a sandbox goes through a proxy that the
proxy variables name, so a scrubbed environment would break the clone
and fetch of `julieta setup` ([04 4.3](04-cli.md#43-julieta-sandbox)).
The proxy's CA is in the system bundle of the sandbox image, and
`SSL_CERT_FILE` names that bundle (measured in an sbx sandbox on
2026-10-16); git's libcurl on that image reads the system bundle and
ignores `SSL_CERT_FILE`, which the list keeps for a git whose TLS
library reads it. The exception gives the agent nothing new: it can
set these variables, but it can already point git elsewhere through
the sandbox's global config (`url.<base>.insteadOf`,
`http.sslCAInfo`), which this mode keeps and does not trust, and what
julieta fetches from origin is content the agent can push anyway. No
credential travels in these variables: the proxy adds it. Host mode
has no such list. Sandbox mode also uses explicit URLs and refspecs, and
`-c core.hooksPath=/dev/null -c core.fsmonitor=false
-c submodule.recurse=false -c fetch.bundleURI=`, and `GIT_NO_LAZY_FETCH=1`
(git 2.45.0 and later, below the floor), so a promisor remote named by
the repository's own config is never contacted for a missing object. It keeps the sandbox's global config,
which the sandbox owns and the agent can write, so nothing in that
config is trusted beyond the sandbox; this is why salvage passes
`-c commit.gpgsign=false`
([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)).
`REQ-0471` One gitsafe table test pins the environment (the network
pass-through list included) and flags of each mode, and gitsafe exports
the neutralized list above, which `doctor`'s `host-git-config` check and
the I3 plant list read.

## 5.2 Invariants and their tests

`REQ-0472` Each guard carries `//romeu:invariant I<n> guard` and each test
`//romeu:invariant I<n> test`. `REQ-0473` The tag is a directive in the sense of
ADR 0007 rule 8, and the one the allowlist of 10 10.6 admits besides
`go:build` and `go:generate`. `REQ-0474` The table below is the one list of
invariants: `tools/ci invariants` reads the ids from it and pairs the
tags it finds in the code. An id with a guard tag has a test tag and the
reverse, and each tagged id is a row of the table. `REQ-0475` That each row has
both tags is checked once, by `tools/ci acceptance` for S6 in the final
plan task ([10 10.5](10-testing-style.md#105-acceptance-evidence)), so a
row can be written before its code without turning `all` red; until then
a row with no code is caught by the plan and by review. A guard deleted
together with its test and both tags is caught by the diff instead:
`REQ-0476` `tools/ci pr` fails a pull request that removes the last guard tag or
the last test tag of an id whose row it keeps. Removing an invariant
means removing its row in the same pull request, on the specification
surface (5.3).

`REQ-0477` `tools/ci mutate` runs `tools/mutants` over one committed mutants entry
per guard, whose `new` text is the pass-through of the guarded decision,
the guard admitting what it refuses, and nothing else; the entry is the
stub. It requires at least one tagged test of that id to fail, at the
test levels the runner has: `REQ-0478` a test tag sits on a `Test*` function; for
each id `tools/ci mutate` runs the tagged functions by name under each
level's build tag the runner has (`e2e` on the Linux runners), and the
id is red when one of them fails by its own assertion; a panic or a
build failure does not count. An id with no tagged test that can run on
a runner is reported as not run there, and it must fail on a Linux
runner, which has every CI level. One entry per id covers one guard
site; an entry per site is a row of
[Deferred decisions](../spec.md#in-the-product). Where the invariant is
an absence (I1, I2, I14), the guard tag sits on the check that observes
it (the `imports` rule, the scan, the filesystem-diff helper) and the
tagged test is a fixture with one violation. `REQ-0479` An absence test also has a
positive control: the same plant, run against unhardened git or under
the guard's pass-through stub, must produce the marker, the `.git` diff,
the trust write or the removal, and the test fails if it does not, so an
inert plant cannot pass for a working guard (I2, I3, I14, I16). `REQ-0480` `go run
./tools/new invariant` scaffolds the next id with both tags.

`REQ-0481` Where a static scan admits a call (I16), an admitted call site is a
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
| I1 | `REQ-0482` **No pane or shell ever runs on the host.** `cmd/romeu` has no multiplexer or terminal code; `run` ends by exec'ing exactly `sbx env exec -it <dir> --env JULIETA_MANIFEST=... -- <abs julieta path> layout up`, the path of 06 6.3 step 3 | U: `go list -deps ./cmd/romeu` excludes `internal/layout`, `internal/hooks`, `internal/mise`; E: final argv asserted byte for byte |
| I2 | `REQ-0483` **The host never configures hooks** and never writes git config except a review checkout's local keys, the one set of 5.1 | U: import graph; E: after every romeu command, each clone's `.git/` that existed before the command (minus `objects/`, `refs/`, `logs/`, `packed-refs`, `index`, `FETCH_HEAD`, `ORIG_HEAD`) is byte-identical to before, including `hooks/`, `config`, `info/attributes`, `objects/info/alternates`; in `config` the `remote.sandbox-<name>.*` keys are left out of the comparison, because sbx writes them (01 1.2); a review repo's local config holds exactly the set of 5.1; positive control (above): the same run with the guard's stub changes `config` |
| I3 | `REQ-0484` **romeu never executes repository content or ambient git configuration** | E: plant in repos, in a hostile global config and in the process ENV: every key of the 5.1 neutralized list (a test holds the plant list equal to the list gitsafe exports), `core.hooksPath=.githooks` + tracked hooks, `.gitattributes` filters with global `filter.x.*`, a global lfs filter and a driver defined only in an `includeIf` file, `include.path`, `alias.*`, a global and a local `url.*.insteadOf` that rewrite to a refused protocol and to `git://127.0.0.1` (a rewrite to another HTTPS host is the stated limit of 5.1), and ENV `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_COUNT`/`KEY_0`/`VALUE_0`, `GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_DIR`, `GIT_EXEC_PATH`; run every romeu command; the marker file never exists, and the positive control (above) makes it exist |
| I4 | `REQ-0485` **No `--auto-approve`**, ever | U: every sbxdrv builder scanned; E: fake sbx exits 99 on it |
| I5 | `REQ-0486` **Fetched content lands only in allowlisted namespaces**; never `refs/heads`, `refs/tags`, `refs/remotes` or `HEAD`; never checked out except into `review/` | U: refspec validator table; E: pull + salvage + sync, then `refs/heads`, `refs/tags`, `refs/remotes`, `HEAD` unchanged |
| I6 | `REQ-0487` **Salvage refs are immutable** (create-only, never a fetch destination, never pruned) | E: fetch with `--prune` and a forced refspec after a same-name recreate; salvage refs survive; a second create on the same ref fails |
| I7 | `REQ-0488` **No unapproved widening is ever live or acted on**: candidates stay in `.romeu/candidates/`; every command that makes a mutating sbx call runs the preflight first; an approval needs a TTY and binds to the candidate `sync` rendered and its digests; a promotion is a crash-safe commit. A gate 2 change is an approval record whose widening digest no longer matches the live files, or live files with no approval. Fields deliberately left ungated are on one committed list beside the gate tags, and `go generate` fails on a field that is on neither (01 1.4) | U: flip table generated from the struct tags: each `gate:"widening"` field flips the widening digest (repos, `ref`, secrets name and argv, workload repository/digest/capabilities, kit content/args/capabilities, gated egress, ports, `agent`, `salvage.excludeIgnored`, the normalized render), and each field without the tag (`sandboxOptions`, `run`) does not; U: the gate 1 flip table, the cases of [I7 unit cases](#i7-unit-cases) below; U: candidate state-machine table (every row, every illegal pair); E: for each gate 2 change and each gate 1 change (different `sbx version`, different catalog digest), across every romeu command: `run`, and a `sync` whose sandbox is running, exit 3 before any mutating call; `stop`, `salvage`, `salvage --from-host` and `pull` print the failure and go on, widening nothing; `rm`, `recreate` and `retire` run their salvage half and exit 3 before `sbx env rm` (01 1.5); a `sync` without a TTY whose digest differs exits 3 and leaves no candidate; a declined candidate leaves live files byte-identical; E: fault injection kills romeu after each promotion step, then every **P** command either finishes the promotion or refuses, and a re-sync converges |
| I8 | `REQ-1222` **Secrets by name only**; `name@project` must match the project exactly | `REQ-0489` U: decode rejects `command`/`argv`/`env` under `secrets`; `REQ-0490` golden shows `/usr/bin/env -i ...`; `REQ-0491` E: unknown name exits 2; `github@other` does not satisfy project `shop` |
| I9 | `REQ-0492` **Egress from pinned commits** in `refs/romeu/origin/<dir>/<ref>` only | E: uncommitted lock change, a commit on another branch, and a local `refs/remotes/origin` rewrite all leave egress unchanged; changing `ref` changes egress and requires gate 2 |
| I10 | `REQ-0493` **Upload-capable and unknown-flag domains are always gated**, including catalog, base and kit-declared domains | U: absent `upload` = gated; `core:node` puts `registry.npmjs.org` in gated; a kit declaring `x.example` gates it |
| I11 | `REQ-0494` **YAML and JSON only via marshallers** | U: `tools/ci imports` rejects `text/template` in `internal/render`; fuzz round-trip of every spec string field |
| I12 | `REQ-0495` **Strict validation of names, paths and file types** before use; memory reads via `os.Root` | U: table + fuzz on validators and ref names; E: memory dir with a symlink, a FIFO and a `..` name is refused, never followed (a device node needs root to create, so probe A3 plants it) |
| I13 | `REQ-0496` **Workspace files contain folders only**; memory dirs never; review checkouts only in `review.code-workspace` | U: golden + decoded key set == {`folders`} |
| I14 | `REQ-0497` **romeu never writes VS Code trust or settings**; doctor fails on a trusted root or disabled trust (`vscode-trust`) | U: doctor fixtures; E: filesystem-diff test over a temp HOME after every command, with the positive control |
| I15 | `REQ-0498` **Drift detection** before overwrite and before every sbx call of a **P** command | E: hand-edit `sbxenv.yaml`, a kit file, `.romeu/bin/SHA256SUMS`; `sync` and every **P** command exit 4 |
| I16 | `REQ-0499` **romeu deletes nothing it does not own** (clones, memory, salvage refs, `.attic`) | U: `tools/ci` scans romeu-linked packages, shared ones included, for `os.Remove*`, rename-over, `O_TRUNC`, `update-ref -d`, `branch -D`, `gc --prune` and `sbx env rm`, and fails each outside the functions the admitted-call-site table of this section names. Those functions remove temp files, candidate dirs and kit dirs no longer referenced by the live `sbxenv.yaml`, replace files under `.romeu/bin/` by rename, and run `sbx env rm` for `rm`; their unit tests prove what they touch; the scan's positive control is a fixture with one call outside the table; `REQ-0500` the scan also lists `os/exec` (`exec.Command`, `exec.CommandContext`), admitted in `internal/procgroup` only, and its runner, admitted in `sbxdrv`, `gitsafe` and the secret-argv row of `doctor` for romeu, and in `mise`, `hooks` and `layout` for julieta (04 4.1, Subprocesses); a fixture package that starts `gh` fails, and a fixture package that calls `os/exec` outside `procgroup` fails; E: fs snapshot diff |
| I17 | `REQ-0501` **Salvage is complete and verified before destruction**, over the generation's recorded repo set | U: generation state-machine table (every row, every illegal pair); E: the cases of [I17 end-to-end cases](#i17-end-to-end-cases) |
| I18 | `REQ-0502` **Salvage commits are never signed and never pushed**: julieta signs none, and romeu pushes none | C (julieta): no signature on salvage commits even with `commit.gpgsign=true` configured; U (romeu): gitsafe builds no push argv |
| I19 | `REQ-0503` **Daemon port re-resolution** before each sandbox fetch | U: stale-state table; E: fake sbx changes the daemon port between calls and reports stopped; fetch uses the new URL or is not attempted; H: real port change |
| I20 | `REQ-0504` **Host-only state is never under `$ROMEU_ROOT` or mounted** | U: init/doctor refuse such roots |
| I21 | `REQ-0505` **Repo hooks run only in the sandbox**, through julieta's dispatcher | C: dispatcher runs `.githooks/pre-commit` and `$GIT_DIR/hooks/post-commit` in the container |
| I22 | `REQ-0506` **Per-project isolation is not bypassed by sbx globals**: doctor fails on global secrets for used services and broad allow rules (`sbx-global-secret`, `sbx-broad-allow`); a listing whose shape is not a recorded shape, or that cannot be parsed, fails the check (`upstream-shape`, [03](03-formats.md) opening) | U: doctor fixtures from block A recordings, plus a changed container shape and an unparseable body: the preflight exits 2 with `upstream-shape` (`RJ-310`), and `doctor` reports `upstream-shape` on both I22 rows and exits 1, a check that could not run; on the fixtures with a global secret or a broad allow rule, doctor exits 6 (`check-findings`) |
| I23 | `REQ-0507` **None of four host tools auto-trusts `$ROMEU_ROOT`**: dotfiles symlinks under `$HOME`, mise trust paths, direnv allow lists, VS Code trust (`home-symlink`, `auto-trust`, `vscode-trust`); another tool's trust model is the operator's to check | U: doctor with temp HOME fixtures |
| I24 | `REQ-0508` **Memory dirs are data with a fixed layout**: the layout of [08 8.1](08-memory-handoff-salvage.md#81-memory-store), its one home; romeu reads them only through the `os.Root` readers of `memstore`; romeu extracts no archive from them other than a git bundle, through gitsafe (`bundle verify`, `unbundle`, `fsck --strict`) | U: the memstore allowlist table, whose expected members come from the one Go list that the reference renders; U: `tools/ci imports` fails a romeu-linked package that imports `memstore/write` (10.7). That no other romeu code opens a memory path on its own is **[review]**: a scan cannot tell what a path is at run time; E: violations make `sync`/`run` exit 2 and `julieta memory check` exit 6 (`check-findings`), and exit 1 on an unmounted or unwritable dir; E: a leftover julieta temp file (08 8.1) makes them warn, never refuse; E: a `<name>-env/` or `.attic` its group or others can access fails `doctor` (`tree-mode`) |
| I25 | `REQ-0509` **romeu never handles secret values**: no argv, env or file written by romeu contains a secret value; `sbx secret set` is only ever called with `--command` | U: sbxdrv builder table rejects `-t`/value forms; E: fake sbx exits 99 on a value form. A host-settings argv that looks like a token is refused at load (`secret-in-argv`, [03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)), a best-effort guard on the shapes it knows; `REQ-0510` U: a doctor fixture whose secret argv prints a sentinel on stdout and on stderr and exits 0, and one that prints it and exits 1; the sentinel appears in no stdout, stderr or `--json` output of `doctor` and in no file under the temp `HOME` and `XDG_*` directories after the run; positive control: the guard's stub prints it (the case "doctor never shows a secret argv's output") |
| I26 | `REQ-0511` **Sandbox identity**: romeu acts only on sandboxes with an open generation it recorded (at create or by `adopt`) and whose workspace path equals `<name>-env/<primary>`. The generation also records the sandbox's id or creation time when probe A4 shows that `sbx ls --json` reports one, and I26 then compares it; until then a sandbox recreated by hand under the same name and path passes | E: a foreign same-name sandbox exits 2 with `RJ-203` naming `romeu adopt`, before any mutation; a path mismatch exits 2; `adopt` without TTY, with a path mismatch, or with an open generation exits 2; after `adopt`, `salvage` and `rm` succeed; a sandbox recreated by hand under the same name and path passes, a known pass until A4 reports |
| I27 | `REQ-0512` **Kit materialization cannot escape or smuggle**: `git fsck --strict` on the commit, then the walk's own validation, independent of fsck: only modes 100644, 100755, 040000; symlinks and gitlinks rejected; names `.`, `..`, `.git` and case-insensitive or HFS-ignorable variants (for example `.GIT`, or `.git` with a U+200C ZERO WIDTH NON-JOINER inserted after the dot) rejected; case collisions under case-insensitive comparison rejected; two names in one tree whose NFD forms (Unicode canonical decomposition), taken after the same case folding and with the HFS-ignorable characters removed, are equal rejected, for example a precomposed and a decomposed é, because APFS treats them as one file; a name whose NFD form equals `.git` under the same folding rejected like `.git`; the case folding is simple folding, so names equal only under full case folding (for example `straße` and `STRASSE`, `ﬁ` and `fi`, `ŉ` and `ʼn`) are not rejected, and whether APFS joins any of them is what the macOS E case measures; written through `os.Root` into the candidate only; `sync --from <sha>` refuses commits not reachable from `refs/romeu/origin/*` | U: table of hostile trees; E: a personal kit with each hostile entry fails sync before any file is written outside `.romeu/candidates/`; E: a hostile tree delivered by a bundle is refused as `--from <sha>`, and refused by the walk when made reachable; the E cases also run on the macOS runners (case-insensitive APFS) |
| I28 | **Kit and workload capabilities are in the gate, and romeu parses them the way sbx does**: `REQ-0513` descriptors parsed with the strict subset grammar (no anchors, aliases, merge keys or includes; any unmodeled key or capability type refused); normalized capability set digested; `REQ-0514` a personal kit that declares a network, mount, volume, credential or ssh-agent capability is refused; `REQ-0515` `internal/oci` verifies the digest of every hop (index, platform manifest, annotation), reads anonymously (never Docker config credential helpers), over HTTPS only, follows only HTTPS redirects, and caps every response size | U: descriptor fixtures; `REQ-0516` U: one conformance fixture per capability type, recorded against the pinned frontend in probes A11 and A12, parsed identically by romeu; `REQ-0517` each fixture records the sbx version and frontend digest it was made with, and a frontend pin change, or an sbx version entering the tested window, runs the A11 and A12 conformance subset again, named in the PR's Evidence; U: `oci` table (digest mismatch at each hop, HTTP URL, HTTPS-to-HTTP redirect, oversize body, credential helper present in a fake Docker config) all refused; E: adding a capability to a kit flips gate 2 |
| I29 | `REQ-0518` **All host output is escaped, and the gate diff is complete**: the CLI writer escapes by default; control characters (U+0000-U+001F, U+007F-U+009F), bidi codepoints, zero-width characters (U+200B-U+200D, U+2060, U+FEFF), tag characters (U+E0000-U+E007F), U+2028/U+2029 and every code point with the Unicode property Default_Ignorable_Code_Point (the soft hyphen, the combining grapheme joiner, the invisible math operators, the variation selectors, the Hangul fillers, and the rest of the property; the version of the property is the one `unicode.Version` reports for the Go release pinned in `mise.toml`, Unicode 17.0.0 for Go 1.27.2; the annotation characters U+FFF9-U+FFFB and the Egyptian hieroglyph format characters U+13430-U+13440 are excluded from the property, and the text they annotate stays displayed, so they are not escaped) are rendered visibly in gate diffs, julieta reports, handoff bodies and ref names; the gate diff is never truncated and shows binaries as size + sha256, flagged | U: fuzz `termsafe`; U: every escaped class has a table row; E: a handoff and a branch name with ESC sequences print escaped; E: a 10 000-line kit file change appears in full in the gate diff, with the summary above the prompt; a binary kit file is shown flagged with its size and sha256 |
| I30 | `REQ-0519` **julieta is delivered read-only and checked for compatibility**: romeu invokes julieta by absolute path in the read-only `.romeu/bin` mount and continues past the compatibility check only when the protocol is accepted, the binary sha256 equals `render.json`, and julieta reports its directory read-only; the check proves compatibility, not integrity (5.4). julieta also runs in the config repo's CI from a release pinned by version and checked by `julieta pin check --workflows` ([02 2.2](02-layouts.md#22-config-repo)), where no romeu exists | X: protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each make `run` exit 2 before `layout up`; a 32 KiB manifest round-trips; H: probe A3 |
| I34 | `REQ-0520` **The signing socket forwards one key**: `git-ssh-sign` renders only when `signing.agentSocket` holds exactly one key, equal to `signingKey`; `SSH_AUTH_SOCK` is set only for that project's sbx calls ([06 6.4](06-kits.md#64-product-kits)) | U: identity listing table (no key, two keys, one other key, the one key); E: `sync` refuses with `RJ-204` |

I31 to I33 are reserved for the runtime ledger, which is designed and
not built in v1; their text is in
[13 13.10](13-runtime-ledger.md#1310-tests), and the ids stay unused
until the ledger lands.

### I17 end-to-end cases

Each case is a test cited by its title; the sandbox half's cases run
in the container (10 10.1).

1. `REQ-0521` `rm` exits 5 for each of: a missing repo in the manifest
   (`repo-missing`), a forged `complete: true` manifest, a hidden branch
   absent from the bundle, a reflog-only commit absent from the bundle,
   a detached HEAD commit, dirt in a linked worktree, an unbundled tag,
   a corrupted payload file (`payload-mismatch`), a salvage id mismatch
   (`salvage-id-mismatch`, `RJ-339`), daemon heads not in the bundle
   (`daemon-heads-missing`), a salvage ref of an earlier generation
   re-bundled under `refs/salvage/<generation>/`, and a repo removed
   from the spec after the generation was created (J13) but absent from
   the manifest.
2. `REQ-0522` Destructive-command fault injection: a test hook kills romeu after
   each step of `salvage`, `rm`, `recreate` and `retire`; the next
   `run`, `salvage` or `rm` converges, no salvage record says lost after
   a complete one, and `sbx env rm` is never reached without a complete
   or accepted record of this generation.
3. `REQ-0523` `sbx env rm` fails after `removing` is written: the command exits 1,
   the generation stays `removing`, and `rm` run again passes the
   identity step and resumes at `sbx env rm` when a fresh fingerprint
   equals the newest `fingerprint` written with `removing`.
4. `REQ-0524` A commit made after a failed `sbx env rm` is in the bundle of the
   rerun of `rm`.
5. `REQ-0525` One test per transition of [01 1.6](01-system-model.md#generation):
   `salvaging -> removing` on a complete salvage, `removing ->
   closed-removed` when `sbx env rm` succeeds, `removing -> salvaging`
   with a new salvage id when the fingerprint differs, and `removing`
   resumed at `sbx env rm` when they match.
6. `REQ-0526` `run` on a `removing` generation with its sandbox present exits 2
   with `removal-pending`.
7. `REQ-0527` An uncommitted edit made after a failed `sbx env rm` makes the rerun
   of `rm` salvage again.
8. `REQ-0528` A worktree already dirty when `removing` was written, edited again
   after a failed `sbx env rm`, makes the rerun of `rm` salvage again,
   because the fingerprint of 03 3.8 digests each changed file.
9. `REQ-0529` A daemon head moved after a failed `sbx env rm` makes the rerun of
   `rm` salvage again.
10. `REQ-0530` An ignored file outside `salvage.excludeIgnored` whose size or
    modification time changed after a failed `sbx env rm` makes the
    rerun of `rm` salvage again.
11. `REQ-0531` After a failed `sbx env rm`, a changed fingerprint makes the rerun
    salvage, record its entry and `fingerprint`, and exit 5 with
    `fingerprint-unstable` (`RJ-338`, [04 4.4](04-cli.md#44-error-ids)),
    whose details name the kind of entry that changed
    ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)), without
    reaching `sbx env rm`, the state read after the exit being
    `removing`; `--accept-loss` naming any reason does not change that,
    and the next rerun with an equal fingerprint resumes at `sbx env rm`
    and writes no new salvage entry (running `sbx env rm` after that
    salvage, or naming no kind, turns this red).
12. `REQ-0532` A writer that keeps changing one ignored path outside
    `salvage.excludeIgnored`, which the recheck of
    [08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)
    step 2 does not see, so each salvage completes: each rerun salvages
    and exits 5 with `RJ-338`, the state `removing`; after an edit of
    `salvage.excludeIgnored` that excludes that path, with the writer
    still running, the next rerun salvages and exits 5 once, and the
    rerun after it resumes at `sbx env rm` (leaving
    `salvage.excludeIgnored` out of the fingerprint turns this red).
13. `REQ-0533` Write during the salvage: after a failed `sbx env rm`, a writer
    keeps changing a tracked file through the repeat of 08 8.5 step 2
    while the rerun salvages, so the recheck skips the worktree as
    `worktree-changing`: the rerun exits 5 with `RJ-306`,
    `salvage-incomplete`, and the state is `open` (returning `RJ-338`
    turns this red).
14. `REQ-0534` Kill the salvage: a salvage that a changed fingerprint started is
    killed before it ends, and the rerun, which restarts it from
    `salvaging`, goes on to `sbx env rm` (exiting 5 with `RJ-338` turns
    this red).

### I7 unit cases

One case per flip of gate 1 (01 1.4), each titled by its flip:

1. the sbx version;
2. romeu's version;
3. romeu's sha256;
4. the embedded julieta's sha256;
5. the catalog digest;
6. the `downgrade` marker: the prompt prints `downgrade` beside a
   version lower than the recorded one.

## 5.3 Ask-first surfaces

`REQ-0535` [`.github/ask-first.yaml`](../../.github/ask-first.yaml) is the single
list of paths whose changes need the maintainer's explicit approval:
each surface with its id, its globs and its reason. This page links the
list and does not copy it. `go generate` derives `.github/CODEOWNERS`
from it, which names `owner` for each glob; `REQ-0536` `tools/ci mutate` reads it to decide which
PRs run mutation testing (a `.go` file on a listed path, or a file
holding an invariant guard tag, so a guard outside every surface is
tested too); `REQ-0537` `tools/ci pr` requires an approval line in the PR body for
each surface the diff touches. A path may match several surfaces; an
approval line names one surface id, so such a path needs one line per
surface it matches. The approval line, the ruleset's requirements, and
which revision of this file `pr` reads are defined in one place,
[12 12.4](12-engineering.md#124-middleware-before-and-after-every-change);
what an approval is worth with one account is in 5.4. `REQ-0538` At a plan
checkpoint the maintainer may grant the approval lines of a list of
tasks on some surfaces, recorded in `docs/grants.yaml`, which is on the
`ask-first` surface; which surfaces can be granted, and the form of a
grant, are in 12 12.4. `REQ-0539` The `approvals` surface holds
`tools/ci/internal/askfirst/**`, `tools/ci/denylist.yaml` and
`.github/workflows/**`, the check of approval lines and grants, the
forbidden-name denylist and the files that choose what runs, which no
checkpoint grant covers. Each path on `approvals` is also on `checks` or
`dependencies`, and a pull request that touches it carries a line for
each surface it matches. `REQ-0540` The `pr` job holds one invariant: no code that
a pull request changes judges that pull request. `pr` runs from a
workflow defined in the default branch, builds the default branch's
commit, and reads the head only as data (decided on 2026-10-09). So a
change to a workflow file does not change the job that judges it; the
job's shape (its event, its checkout and the git flags it reads the head
with) is in
[12 12.4](12-engineering.md#124-middleware-before-and-after-every-change).
Which code `pr` builds, how it reads the YAML and TOML files of the
head, and how it holds the head's version pins and the files the go
command reads from the checkout (a `go.work`, `go.work.sum` or
`vendor/`) are decided by the task that builds `pr`, whose acceptance
lists the requirements that make the invariant hold (decided on
2026-10-09,
[round 11](../reviews/round-11.md#decisions-for-the-operator)): until
that task lands, the invariant is a requirement, not a property of code
that exists. `REQ-0541` The toolchain pinned by `mise.toml` and `mise.lock`, on
`dependencies`, and the `go` line of `go.mod`, which sets the language
version `pr` is compiled at, stay grantable inputs under the job, and
that task lists each of them in 5.4; keeping them off the grantable list
is a row of
[Deferred decisions](../spec.md#in-how-this-repository-is-run).

The file lists itself and `.github/CODEOWNERS`, which `go generate`
derives from it. `REQ-0542` The `dependencies` surface holds every path where
mise reads its configuration when it starts at the top of the tree
(`miseFiles` in `tools/ci/internal/mise/misefiles.go`, which a test holds equal to
these globs), as measured at the pinned mise; `REQ-0543` a change of the mise pin
measures config discovery again at the new version, named in the PR's
Evidence. `owner` is the maintainer's GitHub handle, the `<owner>` of
the module path (02 2.1), which is public already. Each surface has
that one owner; an outside pull request on a surface is reviewed by
the one maintainer, best effort, and a second owner is a row of
[Deferred decisions](../spec.md#in-how-this-repository-is-run).

The decision records and the code of the checks are surfaces (decided in
[round 3](../reviews/round-3.md#maintainer-decisions)): the workflows
only call `tools/ci` (10 10.2), so the check code is the gate. The steps
of `all` check a change to `tools/ci` with the changed `tools/ci`, so
there its only check is the `checks` approval line, or a checkpoint
grant that stands in for that line, while `pr` runs the default branch's
code (the `pr` invariant of this section); running the base branch's
`tools/ci` for the steps of `all` too is a row of
[Deferred decisions](../spec.md#in-how-this-repository-is-run).
`checks` holds the runner of the gates (`tools/ci/*.go` and
`tools/ci/internal/check/**`), the steps whose result decides whether
`all` passes (coverage, license, vulnerabilities, race, vet and
lint) with the fixtures their tests share (`tools/ci/internal/citest`),
the packages a security rule rests on, the mutation runner
(`tools/mutants/**`), the hooks, the linter configuration and the
acceptance file. The checks of format, prose and requirement ids, and
the helpers `tools/ci/adrdir` and `tools/ci/slug`, are not on it
(decided on 2026-10-17). The
`spec` surface was added on 2026-10-08
([round 8](../reviews/round-8.md#decisions-for-the-operator)): the
pages whose tables a gate reads for a decision that widens something
(the vocabulary of 01 1.7, read through `docs/reference/vocabulary.yaml`,
and the tables of the index) and the security model on this page. A
table a check reads for a report, such as the retired requirements of
12 12.11 or the step table of 10 10.2, puts no page on it. The other
pages are not on it, so most edits of this specification need no
approval line. The `procgroup` surface was added on 2026-10-13:
`internal/procgroup` starts every subprocess, kills its process group
and gives a child only the environment its caller names (04 4.1,
Subprocesses), and the call-site scan and its admitted list decide
which other code may start one. `internal/egress`, which derives the
egress sets from the catalog (07 7.5), joined `catalog` the same day,
a grantable surface (12 12.4): its output follows from the catalog's
sets, and a grantable surface is strictly better than none (decided
on 2026-10-13).
`REQ-0544` `internal/sbxdrv` (with `e2e/fakesbx`), which holds the I4 and I25
guards, is the `sbxdrv` surface, and `skills/**`, the text agents
follow, is on `kits`. Both joined on 2026-10-17, before the first file
under those paths, so the first pull request that adds one carries
their approval line.
`REQ-0545` One piece of gate logic sits outside these globs and is reviewed like
any other code: the test in `e2e/scenarios` that fails a command-table
entry no scenario reads (rule 13 of
[ADR 0001, adopt a documentation standard with checkable rules and a voice](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)).

## 5.4 Known residual risks (accepted in v1)

The risks fall in two groups: those of running romeu, which every
operator carries, and those of how this repository is developed, which
the maintainer carries. No row is wider or narrower for being in one
group or the other.

### Risks of running romeu

| Risk | Why accepted | Mitigation |
|---|---|---|
| The operator runs plain `git` in a review checkout or host clone with a risky global config | romeu cannot control later manual commands | `REQ-0546` the local keys of 5.1 in review repos; doctor warns (`host-git-config`); Restricted Mode guidance. A review checkout of a secondary repo is checked with `git fsck --strict` before it is written (04, `romeu pull`); names that collide on a case-insensitive disk are left to git's checkout and to Restricted Mode |
| Host clones carry origin content that agents may have pushed | agents push through the project's token | `REQ-0547` open both workspaces in Restricted Mode; the sandbox holds whatever credential the operator binds for `name@project` ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1), Q13), and a broad one widens this risk, so the guide advises one fine-grained token per project, limited to its repositories (J1 step 5); the `julieta-claude` kit refuses `gh pr merge`, a push of a `v*` tag and `gh api` calls to rulesets and repository settings in a PreToolUse hook ([06 6.4](06-kits.md#64-product-kits)), a guard against mistakes and not against an agent that edits its own hook, nor a gate: a guard that hangs past its timeout or whose wrapper cannot start lets the call proceed ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)) |
| A domain-only allow on `github.com` permits push exfiltration of anything the agent can read | the sandbox legitimately pushes | egress is not containment; the sandbox holds whatever credential the operator binds for `name@project` (03 3.4), and a broad one widens this risk |
| A "Verified" signature on a commit made in the sandbox means the signing key was used, not that a human wrote it | the agent commits with the forwarded signing agent | `REQ-0548` `sync` refuses `git-ssh-sign` unless the dedicated socket holds exactly one key equal to `signingKey` (I34, 06 6.4); the key is added with `ssh-add -c`, so each use asks for confirmation through the askpass program J1 step 5 installs (macOS ships none); probe B2 measures that on both hosts, and the outcome with no askpass ([06 6.4](06-kits.md#64-product-kits), [11 11.2](11-host-probes.md#112-block-b---acceptance-on-real-hosts-last); decided on 2026-10-09); salvage commits are never signed (I18). A signing key per project, or a signing path that is not forwarded, is a row of [Deferred decisions](../spec.md#in-the-product) |
| I34 is enforced by no code while the `git-ssh-sign` kit is in daily use before the signing rule lands | the kit enters daily use before `sync` renders it (the plan's recreate wave) | `REQ-0549` the host socket is a dedicated signing-only agent loaded with `ssh-add -c`, and every recreated sandbox reads `ssh-add -L`: exactly one key; reopened by a sandbox that lists more than one key, or by the signing rule landing |
| The workload image is unsigned | upstream does not sign it yet | digest pin; repository allowlist; capability set in the gate |
| Local v3 kits are unsigned | local v3 kits cannot be signed | trust = romeu release + config commits at an approved digest; publishing and signing product kits is a row of [Deferred decisions](../spec.md#in-the-product) |
| `os-base`'s apt delta floats with the Debian mirror at build ([06 6.4](06-kits.md#64-product-kits)) | `os-base` exists only if A12 reports packages the workload lacks, and its set is that minimum | pinning it is a row of [Deferred decisions](../spec.md#in-the-product), reopened by the first breakage traced to the delta or by `os-base`'s set growing past the A12 minimum |
| The final `sbx env exec -it ... layout up`, and the create and start calls `sbx env run -d [--clone] <dir>`, pass raw bytes between the sandbox or sbx and the host terminal | each is an interactive session: the layout is the session itself, and sbx prompts in the terminal during a create or a start (01 1.4); escaping either would break it | `REQ-0550` romeu exec's into the layout as its last act and prints nothing after; `REQ-0551` during a create or a start romeu writes nothing to the terminal; the host terminal is the only consumer (04 4.1, Output) |
| Secondary repos have no independent daemon cross-check at salvage | sbx runs a git daemon only for the primary repo | bundle verification, the manifest's ref list, and host-provided bases; J13 keeps a removed repo in the generation's repo set |
| Salvage proves that nothing the sandbox held at the salvage read is lost; the next row covers what is written after that read. It does not prove what an agent removed before salvage: the manifest, the bundles, the snapshots and the daemon's repository all live inside the sandbox, where the agent is root | the threat model lists hidden and destroyed work, and nothing inside a sandbox can witness the agent | a push to origin is a copy the agent can also take back when the project's token can delete branches and tags; I17 holds against loss and against a forged manifest; a create-only host ref per fetched origin head is a row of [Deferred decisions](../spec.md#in-the-product) |
| A write made inside the sandbox after a salvage's read and before its `sbx env rm` is not salvaged when that salvage ran from `open` or was restarted from `salvaging`, the salvage that a changed fingerprint started and a killed romeu left in `salvaging` among them ([01 1.6](01-system-model.md#generation)) | the fingerprint of [03 3.8](03-formats.md#38-host-state-schemas-state-v1) guards only the rerun on `removing`, which has an earlier read to compare with; a first removal and a restart have none, and a stored marker or a second read before `sbx env rm` is mechanism the start does not need (Decide at the last responsible moment, the [index](../spec.md#decide-at-the-last-responsible-moment)) | `--stop-agents` stops the agent's processes before the read ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction), step 1), and the recheck of step 2 skips a worktree that changes during the capture; the part of I17 so deferred is a row of [Deferred decisions](../spec.md#in-the-product), and the first report of such a loss, as an issue labelled `deferred`, reopens both rows |
| Salvage payloads of closed generations (`ignored.tar.gz`, `agent/`, opt-in transcripts) can hold credentials and code; they stay in the mounted memory dir, readable and writable by later sandboxes of the project, and in any backup of `$ROMEU_ROOT` | accepted on 2026-10-08; moving them out of the mount waits for a second adopter or a shared host | `REQ-0552` dirs 0700 and files 0600; salvage prints that the payload is secret-bearing when it stores one ([08 8.5](08-memory-handoff-salvage.md#85-salvage-complete-before-destruction)); transcripts only with `--include-transcripts`; moving closed salvage out of the mount is a row of [Deferred decisions](../spec.md#in-the-product) |
| An adopted generation is accepted as it stands until it is recreated: its mounts, policies, secrets and image were not rendered or gated by romeu | refusing `adopt` would lose the J3 recovery | `REQ-0553` `adopt` prints, escaped, the inventory sbx reports before its TTY prompt; `status` shows `adopted, not gated` until a recreate ([01 1.6](01-system-model.md#generation)); probe A4 records which fields `sbx ls --json` gives |
| A protective command (`stop`, `salvage`, `salvage --from-host`, `pull`, and the salvage half of `rm`, `recreate` and `retire`) goes on past a failed gate 1, gate 2 or step 5 check ([01 1.5](01-system-model.md#15-preflight-before-every-mutating-sbx-call)), so it runs an sbx whose version the operator has not acknowledged | refusing would let a sandbox die with unsaved work (decided on 2026-10-09) | `REQ-0554` it widens nothing and forwards no socket (01 1.5); drift and identity still stop it; the failure is printed before the first sbx call |
| `sandboxOptions` and `run` are ungated: a merged spec can size the sandbox up to what sbx allows and change the pane commands that run inside it | a spec change reaches a sandbox through a merged pull request, which the operator merges by working agreement, or through `sync --from <sha>` on a TTY, where the gate 2 diff is the only review (J2 step 2); the merge is a record, not a barrier, since the credential the sandbox holds for the config repo may merge it (the token row of this table) | it is on the committed list of ungated fields (01 1.4, I7); reopened by the first spec merged without the operator merging it |
| `herdr` is pinned by version and per-arch sha256, and its upstream publishes no checksum file and no signature: the hash proves the binary has not changed since publication, not who built it | the upstream offers nothing stronger (measured 2026-10-01, 06 6.4) | `tools/kitpin` takes the hash from the release API's digest and refuses a release that is not marked immutable; the pin is a reviewed diff on the `kits` surface; herdr runs inside the sandbox only; a second run-layout renderer is a row of [Deferred decisions](../spec.md#in-the-product) |
| Between a run and the next `sync`/`run`, an agent can plant files in a memory dir; an operator who opens that dir with host tools, or a host indexer that parses it (Spotlight, Quick Look, Time Machine), sees them | memory dirs are shared by design and checked only when romeu reads them | `REQ-0555` read memory through `romeu handoff`, or with a pager that shows bytes and runs nothing (`less`, `cat -v`); never open a memory dir as a workspace folder; `sync` writes the `.metadata_never_index` marker and `doctor` warns on an indexed or synced root (`root-indexed`); doctor and the preflight check trust settings (I23); `romeu memory list\|show` is a row of [Deferred decisions](../spec.md#in-the-product) |
| Memory and handoff text is agent text that later sessions and the operator read: SessionStart prints it into every later session, and `romeu handoff` prints it to the operator | memory exists to carry text across sessions | the SessionStart print and `romeu handoff` label the text as written by an agent, and SessionStart bounds its size ([08 8.2](08-memory-handoff-salvage.md#82-session-hooks-user-side-middleware)); the operator runs nothing a handoff asks without reading it as data |
| mise executes the configuration of every repo julieta trusts: an `exec()` template, a task or a hook of a `mise.toml` runs in the sandbox, at `install` and at every later mise call of a pane or an agent | `REQ-1270` julieta trusts the `mise.toml` of each repo of the manifest as it trusts the repo's hooks and pane steps, which run repository content on purpose (I21, [04 4.1](04-cli.md#41-conventions-both-binaries) Subprocesses); a `mise.toml` is code of the repo, and the repo is what the sandbox exists to run | `MISE_PARANOID=1` with `MISE_TRUSTED_CONFIG_PATHS` limits it to the repos of the manifest, the global and the system configuration are pointed at `/dev/null`, and `lock --check` and the review of the diff that edits `mise.toml` and `mise.lock` (the ask-first `dependencies` surface) judge what it names ([07 7.3](07-mise-egress.md#73-lockfile-rules), REQ-0621) |
| The compatibility check executes the binary it checks | a check run inside the sandbox cannot attest itself | integrity comes from the read-only mount and the host-side drift check of `.romeu/bin`; the check only proves compatibility |

`REQ-0556` **Release proof.** v1 releases carry `checksums.txt` and a keyless
build provenance attestation
([10 10.2](10-testing-style.md#release-and-bootstrap)), and no other
signature. The attestation proves that `release.yml` built the archive,
not that the maintainer approved the release. An operator signature
over `checksums.txt`, made outside GitHub, is a row of
[Deferred decisions](../spec.md#in-the-product).

**Safe to paste.** The output of `romeu version --json` and the argv,
status, step and versions of an error's details can be pasted into an
issue, and the stderr is looked over first, as
[04 4.1](04-cli.md#41-conventions-both-binaries) states.

### Risks of how this repository is developed

These rows describe the development sandbox (01 1.7), the sandbox this
repository is developed in today, which romeu does not manage. A
sandbox romeu renders forwards one key through `signing.agentSocket`
(06 6.4), and its GitHub token is the `github@<project>` binding (Q13).
The record is
[ADR 0008, let the sandbox act as the maintainer on GitHub](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md).

| Risk | Why accepted | Mitigation |
|---|---|---|
| The approval line in a PR body is typed by the PR's author, and it names what was approved, not who approved it | a check can prove that the approval was recorded, not who gave it; with one account and one token for everyone (ADR 0008, decision 4) the line is a record and a guard against mistakes, not a barrier against an agent. The required checks are worth the same against a token holder, for the reasons of the list **Why the required checks guard against mistakes only** after this table. A signed approval record kept on the host is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) | the default-branch ruleset requires a pull request and the four `all on ...` checks, and no review; its bypass actor, the administrator role, bypasses for pull requests only ([10 10.2](10-testing-style.md#release-and-bootstrap), item a), so a direct push to the default branch is refused to every actor, the bypass actor included. The approval line, checked by `tools/ci pr` ([12 12.4](12-engineering.md#124-middleware-before-and-after-every-change)), is the only recorded approval, and until `tools/ci pr` lands, the line and the grant file are read by the maintainer at merge; the next row says what it is worth |
| The development sandbox's GitHub token is the operator's own: a merge, a `v*` tag and a change to a ruleset or a repository setting are process only against an agent that holds it. Its reach is every repository the account can reach, those of every organization the account is a member of included; that reach is inferred from the account's rights until probe A16 reads the token's type and scopes ([11 11.1](11-host-probes.md#111-block-a---sbx-and-runtime-facts); the scopes read in the sandbox are an entry of the [review log](../notes/maintainer-block-of-ci-bootstrap.md#review-log)) | it was decided on 2026-10-07, for now, and a second account was declined. It collides with rule 1 of [ADR 0005, decide at the last responsible moment and record the trigger](../adr/0005-decide-at-the-last-responsible-moment-and-record-the-trigger.md), which says a merge gate is a need of the start; the collision was accepted by name on 2026-10-07. The merge-gate parts deferred with it are accepted with it, each a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) that says so: a check that reads the rulesets again, a signed approval record, running the change's tests in a separate job, `tools/ci pr`, `tools/ci sequences`, and `tools/ci invariants` with `mutate`. Of four tries with that token, only a direct push to the default branch was refused ([10 10.2](10-testing-style.md#release-and-bootstrap), item d); the try is repeated after the token narrowing (item g) | `REQ-0557` the GitHub security log is read at each sitting, as a judgment and not as a detector: by GitHub's documentation it records ruleset and listed settings changes and no merge, push, tag, branch deletion or release ([ADR 0009, drop the required review and amend the records that assumed it](../adr/0009-drop-the-required-review-and-amend-the-records-that-assumed-it.md), decision 17; the read of 2026-10-09 is in the [review log](../notes/maintainer-block-of-ci-bootstrap.md#review-log)). That an agent can disable the ruleset or change the settings is inferred and stays so (the note on the maintainer block; decided on 2026-10-09). A moved `v*` tag is found by the acceptance check that a tag's commit equals the source digest in its attested provenance ([10 10.5](10-testing-style.md#105-acceptance-evidence)) when step 7 of the release checklist of [12 12.2](12-engineering.md#122-development-commands-and-capability-map) runs at the next release; a tag moved after the latest release stays undetected until then. The development sandbox has no PreToolUse guard of its own until it is romeu-managed (Deferred decisions). Replacing the token with a GitHub App installation token was decided on 2026-10-12 and is proposed in [ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md), decision 2; it is applied as a maintainer step once that record is Accepted, and the token stands until then. The App token could still merge through the API and push branches and workflows, and so post a check under a required check's name (the row before); its full list is in the token narrowing row of [Deferred decisions](../spec.md#in-how-this-repository-is-run); what the token can reach is listed in classes in [ADR 0008](../adr/0008-let-the-sandbox-act-as-the-maintainer-on-github.md#decision) |
| Issue, PR and comment text from any account can steer an agent that holds the operator's token | the token was decided on 2026-10-07 (ADR 0008) | agents read such text as data and take no merge, close, label, workflow or settings action on it without the maintainer's word ([12 12.9](12-engineering.md#129-delivery-practices)); egress and capability widenings still meet the gates; the first issue or pull request from an account other than the operator's, the App's bot excluded, is an event of the token narrowing row |
| Under a `checks` grant, a change can make a step of `all` pass, and the required checks stay green; and a green `pr` result from before a fix to `pr` landed still counts until an event runs the job again | the steps of `all` run the head's `tools/ci`, and running the base's for them is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) | `REQ-0558` the `pr` job runs the default branch's code and enforces the approval lines, the grants and the forbidden names (5.3, [12 12.4](12-engineering.md#124-middleware-before-and-after-every-change)); when `pr` changes, a new event on each open pull request runs the fixed `pr` (an `edited` event from a change to its body, which the job listens to, or a close and reopen), a manual step, while a re-run of an old run keeps the original event's `GITHUB_SHA` and judges with the old code; an automatic new run is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) |
| The maintainer runs agent-authored code on the host: the pre-push hook and `hygiene add` run `tools/ci`, and the maintainer runs it on a host clone after block A and block B of 11; that code is merged by the token | the gates must run where the maintainer pushes | `REQ-0559` the host runs `tools/ci` only from a commit whose diff since the last host run the maintainer has read, or from the last released tag; the `checks` surface and its approval line (5.3); a trusted `hooksPath` checkout is a row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) |
| `tools/ci pr` is not built until its trigger: approval lines, grants and the pull request sections are read by the maintainer at merge | the merge is the maintainer's by working agreement, and the gate's inputs do not exist before a product does | the row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) on `tools/ci pr`, with its trigger; the forbidden-name check over pull request text runs in the agents' own pre-PR gate and the maintainer's read until then |
| `tools/ci sequences` is not built until its trigger | `tools/new adr` and `tools/ci plan` catch the main errors it would find | the row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) on `tools/ci sequences`, with its trigger |
| `tools/ci invariants` and `mutate` are not built until their trigger | the guards land with their packages, are tagged by hand and are never narrowed; `tools/mutants` runs by hand on a security invariant | the row of [Deferred decisions](../spec.md#in-how-this-repository-is-run) on `tools/ci invariants` and `mutate`, with its trigger |

**Why the required checks guard against mistakes only.** The
approval-line row above cites this list:

1. A required check is matched by name, the current token can post a
   commit status itself, and the App token that ADR 0011, decision 2,
   proposes keeps Contents and Workflows write, so a workflow it pushes
   on any branch can post a check or a status under a required check's
   name on another pull request's head.
2. The required-name try of
   [10 10.2](10-testing-style.md#release-and-bootstrap), item h,
   measures which of the two the ruleset counts, and whether the
   narrowed token can post a status itself (decided on 2026-10-09,
   [round 12](../reviews/round-12.md#decisions-for-the-operator)).
3. The reads that settled the ruleset's bypass and the direct-push
   refusal are dated entries of the
   [review log](../notes/maintainer-block-of-ci-bootstrap.md#review-log).
