# 5. Security model

Back to [index](../spec.md). Reader: implementers and security
reviewers. Type: reference (invariants) with the threat model first.

Threat: an agent in a sandbox (or content it
pushed to origin) tries to execute code on the host, exfiltrate host
secrets, widen its own capabilities, forge or hide work, or destroy
work. The host operator and the romeu release (built from reviewed,
signed commits, with verified attestations) are trusted. Egress gating
is defense in depth, not containment.

## 5.1 Hardened git (`internal/gitsafe`)

Every git subprocess started by romeu runs with a fresh environment
(nothing inherited; in particular `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_*`,
`GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_DIR`, `GIT_EXEC_PATH` are never
passed through):

```
env:   GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
       GIT_ATTR_NOSYSTEM=1 PATH=/usr/bin:/bin HOME=<home> LANG=C
flags: -c core.hooksPath=/dev/null -c core.fsmonitor=false -c submodule.recurse=false
       -c protocol.allow=never -c protocol.https.allow=always -c protocol.git.allow=always
       -c core.sshCommand=false -c diff.external= -c core.pager=cat -c credential.helper=
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
  --no-recurse-submodules`, never a remote name. Destination prefixes
  are allowlisted: `refs/romeu/origin/<dir>/`, `refs/sandboxes/<name>/`,
  `refs/romeu/snapshots/<name>/`; salvage refs are created only with
  `CreateRef` (create-only `update-ref`).
- Credentials: only `-c credential.helper=<host settings value>` when
  `tools.gitCredentialHelper` is set; a `gitHosts[].caFile` becomes
  `-c http.sslCAInfo=<path>` for that host only.
- `transfer.fsckObjects` checks objects only as they are transferred: it
  does not cover `git bundle unbundle`, objects already in the
  repository, or case-collision trees. So every tree romeu materializes
  gets two independent checks: `git fsck --strict <commit>` before the
  walk, and the tree walk's own validation (`WalkTree`: `ls-tree -r -z
  --full-tree` at the commit, then the name, mode and collision rules of
  I27 in Go), which does not rely on fsck.
- `sync --from <sha>` accepts only commits reachable from
  `refs/romeu/origin/*` or `refs/sandboxes/*`, so a commit that arrived
  through a salvage or snapshot bundle is never a spec source.
- A unit test asserts that gitsafe still refuses `file://` URLs and plain
  paths when a `gitHosts[].caFile` is set.
- Host git floor: the latest patched point release of each supported
  series, initial floor 2.45.4; the exact per-series minimums and the
  CVEs they close are verified in probe A1 and recorded in
  `internal/gitsafe/floor.go` with sources.

## 5.2 Invariants and their tests

Each guard carries `//romeu:invariant I<n> guard` and each test
`//romeu:invariant I<n> test`; `tools/ci invariants` fails when an id
lacks either, and `tools/ci mutate` builds each guard's `mutate_I<n>`
stub (build tag) and requires its tests to fail. `go run ./tools/new
invariant` scaffolds the next id with both tags. U = unit, E = e2e with
fake sbx (CI), X = hybrid e2e (fake sbx forwarding into the julieta
container, CI), C = container (CI), H = host.

| ID | Invariant | Test |
|---|---|---|
| I1 | **No pane or shell ever runs on the host.** `cmd/romeu` has no multiplexer or terminal code; `run` ends by exec'ing exactly `sbx env exec -it <dir> --env JULIETA_MANIFEST=... -- julieta layout up` | U: `go list -deps ./cmd/romeu` excludes `internal/layout`, `internal/hooks`, `internal/tools`; E: final argv asserted byte for byte |
| I2 | **The host never configures hooks** and never writes git config except the review repo's local hardening keys | U: import graph; E: after every romeu command, each clone's `.git/` (minus `objects/`, `refs/`, `logs/`, `packed-refs`, `index`, `FETCH_HEAD`, `ORIG_HEAD`) is byte-identical to before, including `hooks/`, `config`, `info/attributes`, `objects/info/alternates`; review repos equal the fixed set |
| I3 | **romeu never executes repository content or ambient git configuration** | E: plant in repos and in a hostile global config and in the process ENV: `core.fsmonitor`, `core.hooksPath=.githooks` + tracked hooks, `.gitattributes` filters with global `filter.x.*` and a global lfs filter, `include.path`, `diff.external`, `core.pager`, `alias.*`, `url.*.insteadOf`, and ENV `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_COUNT`/`KEY_0`/`VALUE_0`, `GIT_SSH_COMMAND`, `GIT_ASKPASS`, `GIT_DIR`, `GIT_EXEC_PATH`; run `init`, `sync`, `run`, `pull`, `salvage`, `rm`, `retire`, `status`, `doctor`; the marker file never exists |
| I4 | **No `--auto-approve`**, ever | U: every sbxdrv builder scanned; E: fake sbx exits 99 on it |
| I5 | **Fetched content lands only in allowlisted namespaces**; never `refs/heads`, `refs/tags`, `refs/remotes` or `HEAD`; never checked out except into `review/` | U: refspec validator table; E: pull + salvage + sync, then `refs/heads`, `refs/tags`, `refs/remotes`, `HEAD` unchanged |
| I6 | **Salvage refs are immutable** (create-only, never a fetch destination, never pruned) | E: fetch with `--prune` and a forced refspec after a same-name recreate; salvage refs survive; a second create on the same ref fails |
| I7 | **No unapproved widening is ever live or acted on**: candidates stay in `.romeu/candidates/`; every sbx path runs the preflight; approvals require a TTY and bind to the awaiting candidate id and digests; a promotion is a crash-safe commit | U: flip table generated from the struct tags: each `gate:"widening"` field flips the widening digest (repos, `ref`, secrets name and argv, workload repository/digest/capabilities, kit content/args/capabilities, gated egress, ports, `agent`, `salvage.excludeIgnored`, the normalized render), and each untagged field (`sandboxOptions`, `run`) does not; U: gate 1 flips on sbx version, romeu version, catalog digest; U: candidate state-machine table (every row, every illegal pair); E: each gate 2 change and each gate 1 change (different `sbx version`, different catalog digest) makes `run`, `stop`, `salvage`, `rm`, `retire`, `pull`, `sync --from sandbox/...` exit 3 before any mutating call; `approve` without TTY exits 2; `approve` after the candidate files changed exits 4; declined candidate leaves live files byte-identical; E: fault injection kills romeu after each promotion step, then every **P** command either finishes the promotion or refuses, and a re-sync converges |
| I8 | **Secrets by name only**; `name@project` must match the project exactly | U: decode rejects `command`/`argv`/`env` under `secrets`; golden shows `/usr/bin/env -i ...`; E: unknown name exits 2; `github@other` does not satisfy project `shop` |
| I9 | **Egress from pinned commits** in `refs/romeu/origin/<dir>/<ref>` only | E: uncommitted lock change, a commit on another branch, and a local `refs/remotes/origin` rewrite all leave egress unchanged; changing `ref` changes egress and requires gate 2 |
| I10 | **Upload-capable and unknown-flag domains are always gated**, including catalog, base and kit-declared domains | U: absent `upload` = gated; `core:node` puts `registry.npmjs.org` in gated; a kit declaring `x.example` gates it |
| I11 | **YAML and JSON only via marshallers** | U: `tools/ci imports` rejects `text/template` in `internal/render`; fuzz round-trip of every spec string field |
| I12 | **Strict validation of names, paths and file types** before use; memory reads via `os.Root` | U: table + fuzz on validators and ref names; E: memory dir with a symlink, a FIFO, a device-like file and a `..` name is refused, never followed |
| I13 | **Workspace files contain folders only**; memory dirs never; review checkouts only in `review.code-workspace` | U: golden + decoded key set == {`folders`} |
| I14 | **romeu never writes VS Code trust or settings**; doctor fails on a trusted root or disabled trust | U: filesystem-diff test over a temp HOME after every command; doctor fixtures |
| I15 | **Drift detection** before overwrite and before every sbx call | E: hand-edit `sbxenv.yaml`, a kit file, `.romeu/bin/SHA256SUMS`; `sync` and every **P** command exit 4 |
| I16 | **romeu deletes nothing it does not own** (clones, memory, salvage refs, `.attic`) | U: `tools/ci` scans romeu-linked packages (including shared ones such as `memstore`) for `os.Remove*`, rename-over, `O_TRUNC` on non-temp paths, `update-ref -d`, `branch -D`, `gc --prune`, `sbx env rm` outside `rm`; allowlist: temp files, candidate dirs, kit dirs no longer referenced by the live `sbxenv.yaml`, files under `.romeu/bin/` and under a ledger view replaced by rename; in `internal/ledger` the narrower list of I31 applies; E: fs snapshot diff |
| I17 | **Salvage is complete and verified before destruction**, over the generation's recorded repo set | U: generation state-machine table (every row, every illegal pair); E: `rm` exits 5 for: a missing repo in the manifest, a forged `complete: true` manifest, a hidden branch absent from the bundle, a detached HEAD commit, dirt in a linked worktree, an unbundled tag, a corrupted payload file, a salvage id mismatch, daemon heads not in the bundle, and a repo removed from the spec after the generation was created (J13) but absent from the manifest |
| I18 | **Salvage commits are never signed and never pushed by romeu** | C: no signature on salvage commits even with `commit.gpgsign=true` configured |
| I19 | **Daemon port re-resolution** before each sandbox fetch | U: stale-state table; E: fake sbx changes the daemon port between calls and reports stopped; fetch uses the new URL or is not attempted; H: real port change |
| I20 | **Host-only state is never under `$ROMEU_ROOT` or mounted** | U: init/doctor refuse such roots |
| I21 | **Repo hooks run only in the sandbox**, through julieta's dispatcher | C: dispatcher runs `.githooks/pre-commit` and `$GIT_DIR/hooks/post-commit` in the container |
| I22 | **Per-project isolation is not bypassed by sbx globals**: doctor fails on global secrets for used services and broad allow rules | U: doctor fixtures from block A recordings |
| I23 | **No host tool auto-trusts `$ROMEU_ROOT`**: dotfiles symlinks, mise trust paths, direnv allow lists, VS Code trust | U: doctor with temp HOME fixtures |
| I24 | **Memory dirs are data with a fixed layout**: only `entries/`, `handoff/`, `snapshot/`, `salvage/`, `import/`; regular files only (Lstat); no dotfiles, no `.git`; per-file and per-dir size caps; romeu reads them only through the `os.Root` readers of `memstore`, in three places: `romeu handoff`, the final-handoff check of `rm`, and salvage and snapshot verification; romeu never extracts archives from them | U: memstore allowlist table; U: `tools/ci imports` allows memory reads only from those three call sites; E: violations make `sync`/`run` exit 2 and `julieta memory check` exit 1 |
| I25 | **romeu never handles secret values**: no argv, env or file written by romeu contains a secret value; `sbx secret set` is only ever called with `--command` | U: sbxdrv builder table rejects `-t`/value forms; E: fake sbx exits 99 on a value form |
| I26 | **Sandbox identity**: romeu acts only on sandboxes with an open generation it recorded (at create or by `adopt`) and whose workspace path equals `<name>-env/<primary>` | E: a foreign same-name sandbox exits 2 with `RJ-203` naming `romeu adopt`, before any mutation; a path mismatch exits 2; `adopt` without TTY, with a path mismatch, or with an open generation exits 2; after `adopt`, `salvage` and `rm` succeed |
| I27 | **Kit materialization cannot escape or smuggle**: `git fsck --strict` on the commit, then the walk's own validation, independent of fsck: only modes 100644, 100755, 040000; symlinks and gitlinks rejected; names `.`, `..`, `.git` and case-insensitive or HFS-ignorable variants (for example `.GIT`, or `.git` with a U+200C ZERO WIDTH NON-JOINER inserted after the dot) rejected; case collisions under case-insensitive comparison rejected; written through `os.Root` into the candidate only; `sync --from <sha>` refuses commits not reachable from `refs/romeu/origin/*` or `refs/sandboxes/*` | U: table of hostile trees; E: a personal kit with each hostile entry fails sync before any file is written outside `.romeu/candidates/`; E: a hostile tree delivered by a bundle is refused as `--from <sha>`, and refused by the walk when made reachable; the E cases also run on the macOS runners (case-insensitive APFS) |
| I28 | **Kit and workload capabilities are in the gate, and romeu parses them the way sbx does**: descriptors parsed with the strict subset grammar (no anchors, aliases, merge keys or includes; any unmodeled key or capability type refused); normalized capability set digested; personal kits may not declare network, mount, volume or credential capabilities; `internal/oci` verifies the digest of every hop (index, platform manifest, annotation), reads anonymously (never Docker config credential helpers), over HTTPS only, follows only HTTPS redirects, and caps every response size | U: descriptor fixtures; U: one conformance fixture per capability type, recorded against the pinned frontend in probes A11 and A12, parsed identically by romeu; U: `oci` table (digest mismatch at each hop, HTTP URL, HTTPS-to-HTTP redirect, oversize body, credential helper present in a fake Docker config) all refused; E: adding a capability to a kit flips gate 2 |
| I29 | **All host output is escaped, and the gate diff is complete**: the CLI writer escapes by default; control characters (U+0000-U+001F, U+007F-U+009F), bidi codepoints, zero-width characters (U+200B-U+200D, U+2060, U+FEFF), tag characters (U+E0000-U+E007F) and U+2028/U+2029 are rendered visibly in gate diffs, julieta reports, handoff bodies and ref names; the gate diff is never truncated and shows binaries as size + sha256, flagged | U: fuzz `termsafe`; U: every escaped class has a table row; E: a handoff and a branch name with ESC sequences print escaped; E: a 10 000-line kit file change appears in full in the gate diff, with the summary above the prompt; a binary kit file is shown flagged with its size and sha256 |
| I30 | **julieta runs only as delivered**: romeu invokes julieta by absolute path in the read-only `.romeu/bin` mount and continues past the compatibility check only when the protocol is accepted, the binary sha256 equals `render.json`, and julieta reports its directory read-only | X: protocol mismatch, `SHA256SUMS` mismatch and a writable bin mount each make `run` exit 2 before `layout up`; a 32 KiB manifest round-trips; H: probe A3 |
| I31 | **romeu creates each ledger entry once and has no code that edits or removes one**: one create function writes a temporary file inside the ledger directory, syncs and closes it, and hard-links it to the entry's name, which fails when the name exists; a name that exists is compared (event region, or key and reason) and never replaced. What this does not cover is stated in [13 13.5](13-runtime-ledger.md#135-entries): a removal, and a write by anything other than this code, are undetected in v1 | U: creating an entry whose name exists fails and leaves the bytes unchanged; U: equal and different content under an existing name; U: N racing ingests give one entry per key and leave no temporary name; U: the I16 scan fails a fixture of `internal/ledger` with any write-open, `os.Link`, `os.Remove` or rename outside the four admitted call sites (13 13.10) |
| I32 | **A spool is data with a fixed layout, read once**: only lowercase `<ULID>.json` regular files; the root checked with Lstat and opened with `os.Root`; a bounded number of names examined; each file opened once without following links and without blocking, checked on the descriptor, read once up to the size cap plus one byte; romeu validates, hashes and stores those same bytes; a violation is skipped and reported, never followed; romeu opens a spool from two call sites, ingest and `doctor` | U: the hostile spool table of 13 13.10, each row with its outcome; U: `tools/ci imports` allows spool reads only from those two call sites; E: a failed ingest leaves `salvage` and `rm` with the exit status they would have had; H: probe A3 |
| I33 | **What a sandbox reads of the ledger is its own project's entries and, of other projects, closed fields only**: `type`, `tool` and `ref` are valid by membership in tables generated from the release's code and data, never from a project file; an out-of-domain value rejects the whole event; `others.jsonl` is produced by one function from the generated allowlist and carries no `path` and no key; the ledger is never mounted (I20); the view mount is read-only and julieta refuses a writable one | U: the per-field table of 13 13.10; golden: the two views of a two-project fixture; X: a writable view mount makes `julieta setup` exit 2 and `run` stop before `layout up`; H: probe A3 |

## 5.3 Ask-first surfaces

`.github/ask-first.yaml` is the single list of paths whose changes need
the maintainer's explicit approval. `go generate` derives from it the
table in `docs/reference/ask-first.md` and `.github/CODEOWNERS`, which
names `owner` for each glob; `tools/ci mutate` reads it to decide which
PRs run mutation testing (a `.go` file on a listed path); `tools/ci pr`
requires an approval line in the PR body for each surface the diff
touches. The approval line, the ruleset that requires the code-owner
review, and which revision of this file `pr` reads are defined in one
place,
[12 12.4](12-engineering.md#124-middleware-before-and-after-every-change).
The file lists itself. `owner` is the maintainer's GitHub handle, the
`<owner>` of the module path (02 2.1), which is public already. The maintainer decided that the
decision records and the code of the checks are surfaces
([round 3](../reviews/round-3.md#maintainer-decisions)): the workflows
only call `tools/ci` (10 10.2), so the check code is the gate. One
piece of gate logic sits outside these globs and is reviewed like any
other code: the test in `e2e/scenarios` that fails a command-table
entry no scenario reads (rule 13 of
[ADR 0001, the documentation standard](../adr/0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md)).

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
    globs: [go.mod, go.sum, mise.toml, mise.lock, .github/workflows/**]
    reason: Go dependencies, the pinned tools the gates run, and CI actions
  - id: contracts
    globs: [internal/cli/errors.go, internal/spec/versions.go]
    reason: exit codes, error ids and file format versions
  - id: ledger
    globs: [internal/ledger/**]
    reason: the runtime ledger's code, its event schema, the secret-pattern list, and the tags that decide which fields cross projects
  - id: decisions
    globs: [docs/adr/**, .adr-dir]
    reason: decision records; one leaves Proposed, or changes after that, with the maintainer's approval
  - id: checks
    globs: [tools/ci/**, .githooks/**, .golangci.yml]
    reason: the code of the gates, with the forbidden-name denylist (tools/ci/denylist.yaml) and the linter configuration
  - id: ask-first
    globs: [.github/ask-first.yaml]
    reason: this list
```

## 5.4 Known residual risks (accepted in v1)

| Risk | Why accepted | Mitigation |
|---|---|---|
| The operator runs plain `git` in a review checkout or host clone with a risky global config | romeu cannot control later manual commands | local hardening keys in review repos; doctor warns; Restricted Mode guidance |
| Host clones carry origin content that agents may have pushed | agents push through the project's token | open both workspaces in Restricted Mode; per-project fine-grained tokens (Q13) |
| A domain-only allow on `github.com` permits push exfiltration of anything the agent can read | the sandbox legitimately pushes | per-project tokens; egress is not containment |
| A "Verified" signature on a commit made in the sandbox means the signing key was used, not that a human wrote it | the agent commits with the forwarded signing agent | `sync` refuses `git-ssh-sign` unless the dedicated socket holds exactly one key equal to `signingKey` (06 6.4); confirm-on-use where the agent supports it; salvage commits are never signed |
| The workload image is unsigned | upstream does not sign it yet | digest pin; repository allowlist; capability set in the gate |
| Local v3 kits are unsigned | local v3 kits cannot be signed | trust = romeu release + config commits at an approved digest; publishing path in [06](06-kits.md) |
| The final `sbx env exec -it ... layout up` passes raw bytes between the sandbox and the host terminal | it is the interactive session itself; escaping it would break the terminal | romeu exec's into it as its last act and prints nothing after; the host terminal is the only consumer |
| Secondary repos have no independent daemon cross-check at salvage | sbx runs a git daemon only for the primary repo | bundle verification, the manifest's ref list, and host-provided bases; J13 keeps a removed repo in the generation's repo set |
| Between a run and the next `sync`/`run`, an agent can plant files in a memory dir; an operator who opens that dir with host tools sees them | memory dirs are shared by design and checked only when romeu reads them | read memory only through `romeu handoff`; memory dirs are never workspace folders; doctor and the preflight check trust settings (I23) |
| The approval line in a PR body is typed by the PR's author | a check can prove the line is there, not who said it | the default-branch ruleset requires a code-owner review for each ask-first path, the decision records and the check code included ([12 12.4](12-engineering.md#124-middleware-before-and-after-every-change)); the next row says what that review is worth today |
| With one GitHub account the code-owner review is not independent: agents push and open PRs as the maintainer, GitHub does not count an author's approval of their own PR, and the maintainer merges as the rulesets' bypass actor | a second account is a cost v1 does not need in order to start | the sandbox's token must not merge, push to the default branch, push a `v*` tag or change a ruleset; the maintainer block of `ci-bootstrap` saves a refusal of each (10 10.2), and Q25 holds the fallback if one is not refused. Until then the maintainer's merge is the only control |
| Nothing reads the rulesets again after the maintainer block | `tools/ci all` is offline, and a ruleset is repository configuration, not a tracked file | changing a ruleset needs the Administration permission, which the sandbox's token must not have; the same block saves the refusal |
| A `v*` tag runs `release.yml` from the tagged commit, whether or not that commit is on the default branch or was reviewed | a tag ruleset limits who pushes a tag, not which commit it names | the tag ruleset refuses a `v*` tag from anyone but its bypass actor, the maintainer, and the same block saves the refusal of the sandbox's token; `release.yml` runs `tools/ci all` on the tagged commit before it builds |
| The compatibility check executes the binary it checks | a check run inside the sandbox cannot attest itself | integrity comes from the read-only mount and the host-side drift check of `.romeu/bin`; the check only proves compatibility |
| Between a write in the sandbox and ingest on the host, an event can be forged, altered or dropped in the spool, and a dropped one leaves no trace | nothing inside a sandbox can make julieta the one writer of a directory the agent can also write | the ledger is add-only from ingest onward (I31); an entry is read as data an agent may have written; romeu stamps `project` and `ingested` itself and orders by those stamps (13 13.2) |
| A removed ledger entry is undetected in v1, so "never deleted" is a promise; nothing protects the ledger from the host | the threat model trusts the host, and a chain on the same disk is rewritten by whoever can remove an entry | romeu has no code that removes an entry (I31); the directory is 0700 under host state (I20); a hash chain is a deferred decision with three triggers |
| `doctor` verifies the event bytes of ingested entries and nothing else: romeu's stamps (`project`, `ingested`) and rejected entries are covered by no digest | the key is the digest of what julieta wrote, so that `shasum` reproduces it | the same as the row above; v1 does not claim to detect a change to a stamp |
| A secret that reaches the ledger stays there and in machine backups: the pattern check can miss one, and nothing is ever redacted | the maintainer's rule is absolute, and a redaction path is a way to delete history | an event has no free text; `path` is the one wide field and is checked against the pattern list; the remedy is to rotate the credential, which leaves a dead value |
| Each sandbox on a machine sees the other projects' names, their activity timing and the allowlisted fields of their events | the maintainer accepted cross-project reads | closed sets by membership (I33); `path` and the key stay out; finer visibility is a deferred decision |
| `path` and the rest of a project's own entries are prompt-injection surface for the sessions and the lessons loop that read them | they are the project's own data, as a memory entry is | the `path` character set and length (13 13.2); host output goes through the escaping writer (I29) |
| The ledger only grows, and an agent can mint rejected entries, one per distinct invalid spool file | v1 warns and does not refuse, because refusing would drop events | the spool caps bound what one ingest takes (13 13.8); a rejected entry is one short line; `doctor` warns on size; rotation and a hard limit are a deferred decision |

The maintainer accepted three of these rows for v1 by name
([round 4](../reviews/round-4.md#maintainer-decisions)): the
code-owner review that is not independent with one account, where the
maintainer's merge is the only control; the rulesets that no check
reads again after they are created; and the `v*` tag, whose release
depends on the tag ruleset. The maintainer also accepted the fallback
of Q25: a second account without bypass rights, for agents. It is
confirmed, or found unnecessary, when the token test of the
maintainer block runs (10 10.2, item d).

Of the seven ledger rows, the maintainer decided four by name
([round 5](../reviews/round-5.md#maintainer-decisions)): the time
between write and ingest, the removal that goes undetected, the
secret that stays, and the cross-project reads. The other three came
with the design the maintainer accepted as a whole.
