# Spec review round 2

Reader: reviewers checking that every round-2 finding was handled, and
maintainers tracing why the spec reads as it does. Type: reference
(history). The current state is [docs/spec.md](../spec.md); round 1 is
in [round-1.md](round-1.md).

Round 2 re-audited the round-1 fixes with the round-1 reviewers
(security, doubt-driven, testability, fidelity) and added a
craftsmanship lens. Every finding was applied in one write pass on top of
commit e503190.

Status: **A** = applied; **M** = merged into the named mechanism;
**D** = declined, with the reason; **O** = maintainer decision.

## Maintainer decisions

Decided on 2026-09-30. The maintainer's words, verbatim (pt-BR):
"palpite perfeito!" ("perfect call!").

| Id | Decision | Where |
|---|---|---|
| op-signing (sa H5) | `romeu sync` refuses to render `git-ssh-sign` unless the dedicated signing socket holds exactly one signing-only key, equal to the kit's `signingKey`; the full host agent is never forwarded | 06 6.4, 01 1.5, 04 doctor, 05 5.4 |
| op-mise-project (fid 5) | the mise project layer stays empty: one `mise.lock` per repo is the single source of tools | 07 7.2 |
| op-license (Q1) | CC0-1.0 for `examples/` and `schemas/`; GPL-3.0-or-later for everything else | 00 0.4 |
| op-rules-as-data (craft C10) | validation rules live as data; validators, docs tables and JSON Schemas are derived; this supersedes round-1 D17 (hand-written schemas kept in sync by a property test) | 03 intro, 12 12.3 |
| op-bar | the craftsmanship bar applies to everything already designed: mechanical before/after middleware, explicit state machines, simplicity, trust | spec.md Principles, 12 |

## Resolutions

| Id | Resolution | Status | Section |
|---|---|---|---|
| sa N1 | fsck-on-transfer limits stated; `git fsck --strict <commit>` before the walk; walk validation independent of fsck; `--from <sha>` restricted to commits reachable from `refs/romeu/origin/*` or `refs/sandboxes/*`; bundle-delivered hostile tree case in I27 | A | 05 5.1, I27, 04 sync |
| sa N2 | strict descriptor subset grammar; per-capability conformance fixtures from A11/A12; `oci` verifies every hop, HTTPS only with HTTPS-only redirects, size caps, never Docker credential helpers; reads are anonymous only (the explicit-credential option is deferred as Q23) | A | 01 1.4, 06 6.1, I28, 11 A11/A12 |
| sa N3 | gate diff never truncated; summary at the top and repeated above the prompt; binaries as size + sha256, flagged; tests in I29 | A | 01 1.4, I29 |
| sa H5 | see op-signing | O | 06 6.4 |
| sa M2/M4 | I22 and I23 checks run in the preflight as well as in `doctor` (one implementation) | A | 01 1.5, 04 doctor |
| sa N4 | gate 1 prints the julieta version change and the catalog domain/upload-flag diff per affected project; the acknowledged catalog is stored in `toolchain.json` | A | 01 1.4, 03 3.8, 07 7.6 |
| dd N6 | merged into sa N4 | M | 01 1.4 |
| sa N5 | termsafe escapes zero-width, tag characters, U+2028/2029; the CLI writer escapes by default with one named raw exception; raw final attach listed as a residual risk | A | I29, 04 4.1, 05 5.4 |
| sa N6 | julieta invoked by absolute path in the read-only mount; the handshake renamed "compatibility check" and scoped to compatibility; binary digests in `render.json`; A3 tests root remount | A | 06 6.3, 03 3.7, I30, 11 A3 |
| sa N7 | merged into the candidate state machine and the promotion commit (craft C2, C5); `approve` binds to candidate id and digests and recomputes from files; see conflict 1 for render.json ordering | M | 01 1.6 |
| te N9 | merged into the promotion commit: fault-injection test after each step; stale candidates from a crashed sync deleted by the next sync | M | 01 1.6, 10 10.1, I7 |
| sa H4 residual | secondary repos have no daemon cross-check: listed | A | 05 5.4, 08 8.5 |
| fid U+200C | the invisible character in the I27 example replaced by the words "U+200C ZERO WIDTH NON-JOINER" | A | I27 |
| dd N1 | the pointer file is gone: handoff files are named `<ULID>-<kind>.md`; the handoff reader selects the latest narrative and the newest facts separately, so a facts file never hides a narrative in either hook order; container test runs the hooks in `/clear` order; the real order is probe C5 (Q22) | A | 03 3.10, 08 8.3, 09 J5, 11 C5 |
| dd N2 | no build-time link: `julieta setup` links `$HOME/.local/bin/julieta` at runtime; binary names use Go arch names chosen by romeu's `GOARCH`, so no `uname -m` mapping exists; the probe for a build-step symlink is unnecessary (see conflict 5) | A | 06 6.3, 02 2.5, Q21 |
| dd N3 | `romeu adopt <name>` (TTY; workspace path must match; records an open generation with `adopted: true`); the identity error `RJ-203` names adopt | A | 04, 01 1.6, I26, 09 J3 |
| dd N4 | `renderDigest` digests the normalized render with every untagged key (today `sandboxOptions`) removed; the I7 table is generated from the struct tags | M | into craft C4; 01 1.4, I7 |
| dd N5 | salvage uses the generation's recorded repo set; I17 case after J13 | M | into craft C6; 01 1.6, 08 8.5, I17 |
| dd D8 residual | the post-run plant window in memory dirs listed; read memory only via `romeu handoff` | A | 05 5.4 |
| dd N7 | descriptors cached content-addressed in host state; the preflight never touches the network, with an e2e test | A | 01 1.5, 02 2.4, 04 sync, 10 10.1 |
| dd N8 | julieta accepts manifest protocols N and N-1; hook failures recorded and surfaced by SessionStart, `julieta status` and `romeu status` | A | 03 3.6, 08 8.2, 04 |
| dd N9 | probe harness split into a pure core and a thin exec layer, both unit-tested in CI (the exec layer against a helper binary), with no coverage exemption | A | 11 intro, 10 10.1 |
| dd N10 | snapshots exclude the same host-provided base as salvage (`repos[].base` in every manifest) | A | 03 3.6, 08 8.4 |
| dd N11 | `rm`'s final-handoff check named as one of the three `os.Root` memory read sites | M | into craft C6; I24 |
| dd N12 | "Kit-declared network needs stay inside the kits" deleted | A | 07 7.5 |
| dd N13 | `status` reports the gate-1 state and never exits 3 | A | 01 1.4, 04 |
| dd stale 7.6 | catalog changes show up in gate 1 | A | 07 7.6 |
| dd stale 7.5 | `sbx policy rm network ... --resource <domain>` | A | 07 7.5 |
| dd stale J1 | J1 cites the git floor of 05 5.1 (2.45.4) | A | 09 J1 |
| te N1 | `probe-result.v1` gains `commit`, `affects[]` and `resolvedBy`; `tools/ci probes` checks `resolvedBy` descends from `commit` and touches every `affects` path | A | 11 11.3, 11.4 |
| dd N14 | merged into te N1 | M | 11 11.4 |
| te N3 | the recorder redacts (placeholders, stdout allowlist, never secrets); unit test with a planted token and home path; hygiene scans the recordings | A | 10 10.3, 10 10.2 |
| te N7 | hybrid e2e for the romeu-julieta contract, tagged as the new invariant I30 | A | 10 10.1, I30 |
| te N2 | decisions computed from per-probe `onPass`/`onFail` or value tables; `observe` probes pass when every observation is recorded; arch-sensitive probes need both hosts | A | 11 11.3, 11.4 |
| te N4 | stateful replay of ordered sessions keyed by prior mutating calls; normalizer unit test | A | 10 10.3 |
| te N5 | `tools/ci acceptance --file` validates any acceptance.v1 file; an operator ledger in a config repo is verified from a product checkout; new evidence kind `repo`; see conflict 2 | A | 10 10.5, 02 2.2 |
| te N6 | every block B step writes `probe-result.v1`; B3 records its `go test -json` output as an artifact | A | 11 11.2 |
| te C5 residual | `memory verify` compares `status` separately from the content digest | A | 04 4.3, 08 8.1 |
| te H4 residual | `julieta setup` no-op <= 3 s timed in the container e2e (median of 5) | A | S8, 10 10.1 |
| te N8 | S7 cites J1-J13; S9 scope matches the CI coverage rule; `tools/ci sequences` checks journey ranges | A | spec.md S7, S9; 10 10.2 |
| te N10 | gate 1 e2e: a different `sbx version` or catalog digest makes every sbx-path command exit 3 before any mutating call | A | I7 |
| te N11 | `tools/ci acceptance` is out of `all`; unit tests use a recorded API fixture; live in the final task and `release.yml` | A | 10 10.2 |
| te N12 | `tools/ci lessons` resolves the named check to an existing subcommand or test | A | 10 10.2, 12 12.7 |
| te N13 | merged into craft C3 (`tools/ci pr`) | M | 12 12.4 |
| te N14 | I27 hostile-tree e2e cases also run on the macOS runners | A | I27, 10 10.1 |
| te C1 residual | unit test: gitsafe refuses `file://` and plain paths when `caFile` is set | A | 05 5.1 |
| craft C1 | `.github/ask-first.yaml` is the single ask-first list; it generates CODEOWNERS and the reference table and drives `mutate` and `tools/ci pr`; the two diverging lists are gone | A | 05 5.3, spec.md Boundaries |
| craft C2 | candidate lifecycle as a state machine: rendered, awaiting, approved, promoted, declined, superseded; at most one candidate; "pending" no longer names two things | A | 01 1.6 |
| craft C3 | tracked `.githooks/pre-push` runs `tools/ci fast`; `tools/ci pr` checks the PR sections, the Middleware line, goldens under Evidence, and a quoted approval for ask-first paths | A | 12 12.4 |
| craft C4 | struct tags `gate`/`apply` are the single classification source; `go generate` derives the widening set, recreate digest, I7 flip table and field tables | A | 01 1.4, 03 3.2, 12 12.3 |
| craft C5 | promotion commit: content-addressed immutable kit dirs, `render.json` first with `promoting`, `sbxenv.yaml` last as the commit point, recovery at lock time; `.prev` dropped (no consumer). Rollback narrowed to roll-forward or refuse, because nothing live is modified before the commit point | A | 01 1.6, 02 2.3, I16 |
| craft C6 | generation lifecycle state machine (open, salvaging, closed-removed, closed-lost) with a transition table; salvage `result` + `reasons[]`; final handoff computed at rm time; `status` is a projection | A | 01 1.6, 03 3.8 |
| craft C7 | stable error ids `RJ-<exit><nn>` with slug, message and fix hint in the exit-code table; generated `errors.md`; tests assert ids; `--json` errors carry them | A | 04 4.1, 10 10.6 |
| craft C8 | `ARCHITECTURE.md` and `CONTRIBUTING.md` in the tree with outlines; spec is current state, ADRs immutable, `Supersedes` checked both ways | A | 02 2.1, 12 12.5, 12.8 |
| craft C9 | `tools/new` scaffolding with the next free id; one `go generate ./...`; `tools/ci generated` | A | 12 12.3 |
| craft C10 | see op-rules-as-data | O | 03 intro |
| craft C11 | probe lifecycle recorded, kept or overturned, resolved | M | into te N1; 11 11.4 |
| craft C12 | `memory done` became `edit --status done`; `layout render` became `layout up --dry-run`; `hooks install` became internal to `setup` | A | 04 4.3 |
| craft C13 | host state is `toolchain.json` plus one `projects/<name>.json` per project, hosting the C2 and C6 machines; one atomic write per transition | A | 02 2.4, 03 3.8 |
| craft C14 | catalog embedded in julieta; `julieta spec validate --catalog` replaces the config repo's use of `tools/ci` | A | 04 4.3, 07 7.6, S4, 02 2.2 |
| craft C15 | `internal/canon` with `Digest[T](kind, v)` for 11 kinds; canonical JSON defined once; plain sha256 kept for file bytes | A | 03 3.12 |
| craft C16 | review tables moved to `docs/reviews/`; settled questions leave the spec and become ADRs (list below) | A | this file, spec.md |
| craft C17 | vocabulary: host settings, personal kit, run layout, candidate/awaiting; sandbox probes C1-C5; `[X]` for sbx in journeys; memory `snapshot/`; `tools/ci vocabulary` from the 1.7 "Not" column | A | 01 1.7, all pages |
| craft C18 | one tag syntax `//romeu:invariant I<n> guard|test` | A | 05 5.2, 10 10.6 |
| craft C19 | `tools/release notes` generated from Conventional Commits and PR Why sections | A | 12 12.6 |
| craft user-side middleware | `lesson`-tagged entries and julieta warnings printed at SessionStart | A | 08 8.2 |
| fid 5 | see op-mise-project | O | 07 7.2 |
| license | see op-license | O | 00 0.4 |

## Conflicts and how they were decided

1. **Promotion order (sa N7 vs craft C5).** sa N7 asked for `render.json`
   renamed last; C5 asked for `render.json` first with a `promoting`
   marker and `sbxenv.yaml` last. Chose C5: `sbxenv.yaml` is what sbx
   reads, so it is the natural commit point, and the marker lets every
   command detect and finish an interrupted promotion. sa N7's safety
   goal (no command acts on a half-promoted tree) holds because recovery
   runs at lock time and the preflight refuses otherwise.
2. **Acceptance ledger (te N5 vs craft C14).** te N5 had the config repo
   run `tools/ci acceptance`; C14 removes `tools/ci` from the config
   repo. The ledger file lives in the config repo and is verified with
   `tools/ci acceptance --file` from a product checkout; the config
   repo's CI uses only julieta.
3. **Registry credentials (sa N2 vs simplicity).** sa N2 allowed an
   explicit host-config credential. v1 reads anonymously only: the
   descriptor cache (dd N7) keeps reads rare, and a credential adds a
   secret-handling path to romeu. Recorded as open question Q23.
4. **Long diffs (sa N3).** Paging or a separate `romeu diff` command
   would add a concept; the diff is printed in full with the summary
   repeated above the prompt instead.
5. **Finding julieta in the sandbox (dd N2 options).** A shim with an
   env var or a recreate-class kit arg both keep a build-time step that
   cannot know the per-project path. A runtime link made by
   `julieta setup`, which romeu already calls by absolute path, removes
   the step and the probe for it.
6. **Coverage of the probe exec layer (dd N9 vs "tools are code").** No
   coverage exemption: the exec layer is tested against a helper
   binary.
7. **Handoff pointer (dd N1 options).** Instead of guarding `LATEST`,
   the pointer file was removed: the kind is in the file name, so the
   reader selects by kind and ULID order.

## Settled questions

These left the open-questions table. The `docs` module writes one ADR
for each.

| Question | Decision | Settled by |
|---|---|---|
| Q1 license of examples and schemas | CC0-1.0 for `examples/` and `schemas/` | maintainer (op-license) |
| Q3 secondary repos | sandbox-private clones by julieta from origin | round 1 decision (02 2.6) |
| Q5 `sbx env plan` in the gate | not part of romeu's gate | round 1 |
| Q19 (partial) signing key isolation | the refusal rule is decided (op-signing); only the socket forwarding stays open | maintainer |

New open questions: Q21 (sandbox arch equals host arch, A4), Q22
(Claude Code hook order on `/clear`, C5 and B3), Q23 (anonymous
registry reads suffice). Q19 was narrowed to the forwarding mechanism.

## Simplicity: counts before and after

| Measure | Before (e503190) | After |
|---|---|---|
| romeu commands | 14 | 15 (+ `adopt`, required by dd N3) |
| julieta leaf commands | 28 | 25 (- `memory done`, `layout render`, `hooks install`) |
| host state record schemas | 4 (toolchain, approvals, pending, sandboxes) | 2 (toolchain, project) |
| files kept only for recovery or history | 2 (`sbxenv.yaml.prev`, `handoff/LATEST`) | 0 |
| ask-first lists | 2 (spec.md Boundaries, 05 5.3) | 1 (`.github/ask-first.yaml`) |
| sources of the gate/recreate classification | 3 (03 table, 01 table, I7 table) | 1 (struct tags) |
| sources of validation rules | 2 (hand-written schemas, Go validators) | 1 (`rules.go`) |
| single sources with generated consumers | 1 (exit codes) | 7 (12 12.3) |
| probe result formats | 2 (`probe-result.v1`, host JSONL) | 1 |
| names for "pending" | 2 (candidate dir, state file) | 0 (candidate, awaiting) |
| invariants | 29 | 30 (+ I30, required by te N7) |
| `internal` packages | 19 | 21 (+ `canon`, `signing`) |
| explicit state machines | 0 | 4 (candidate, promotion commit, generation, probe) |
