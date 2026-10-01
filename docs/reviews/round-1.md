# Spec review round 1

Historical record of how review round 1 changed the specification
(applied in commit e503190). Section references point at the spec as it
was at that commit; the current state is [docs/spec.md](../spec.md).
Round 2 superseded some of these resolutions; see
[round-2.md](round-2.md).

Status: A = applied, M = merged into another item, D = declined with
reason.

| Id | Resolution | Status | Section |
|---|---|---|---|
| op-license | GPL-3.0-or-later, COPYING with first code, SPDX, REUSE 3.3, `reuse lint`; CC0 sub-question as Q1 | A | 0 License, S11, Q1 |
| op-principle | principle kept; applications listed; items in section 5 of the review applied | A | 0 Principle |
| op-tools-are-code | operator addition: principle "tools are code" added; applied in Boundaries, 10 (tools/* under the same lint/test/coverage/review gates, text standard, PR template, `docs/lessons.md`, lessons become checks) | A | 0 Principles, 5, 10 |
| op-supersede | operator addition: "very high and pragmatic" bar; decisions (spec or ADR) revisited on evidence and superseded explicitly, never silently worked around | A | 0 Principles |
| op-S3 | S3 met across Intel host (amd64) and Apple silicon host (arm64) | A | S3 |
| fid R1 | operator appendix removed from the repo (copied to scratch); links removed | A | index |
| fid R2 | kit mapping rewritten generically; memory import generic; process-history phrases removed | A | 06 6.4, 08, 04 |
| D1 | v3 go/no-go (A11), herdr hand-written `layout.apply` (A13) and the create-time step check (A14) moved to block A; block B only accepts built kits; rework claim narrowed | A | 11, 0 capability map |
| D2 | julieta delivered by a romeu-owned read-only mount (`.romeu/bin`), verified by sha256 and a protocol handshake; not in kit content | A | 06 6.3, 02, 03 3.6 |
| D3 | per-machine toolchain acknowledgement (sbx version, romeu version, catalog digest); run rechecks live sbx version | A | 01 1.4 |
| D4 | merged with sa H1 | M | 01 1.4, 06 |
| D5 | merged with sa H2 | M | 01 1.4 |
| D6 | origin fetched into `refs/romeu/origin/<dir>/*`; render.json cites it | A | 05 5.1, 03 3.7, 07 |
| D7 | `julieta setup` fetches origin and fast-forwards the default branch when clean | A | 04 |
| D8 | memory layout allowlist, refuse on violation, host reads only via `romeu handoff`, no host extraction, size caps | A | 08 8.1, I24 |
| D9 | no debounce; chain to tracked and `$GIT_DIR` hooks; repo-local `core.hooksPath` reported as snapshots disabled | A | 04, 08 8.3 |
| D10 | `agent: sbx-kit-claude` handle + workload kit by digest; goldens probe-confirmed (A5) | A | 03 3.2-3.3 |
| D11 | merged with te C3 | M | 10 10.3 |
| D12 | secrets: recreate-class by default, live path keyed on A10 (Q18); invariant "romeu never handles secret values" | A | 01, 03, 09 J11, I25 |
| D13 | run checks workspace path and refuses sandboxes romeu did not create | A | 04, I26 |
| D14 | herdr survival across host terminal loss added to block A (A13) | A | 11 |
| D15 | `MISE_TRUSTED_CONFIG_PATHS` = manifest repo paths | A | 04, 07 |
| D16 | dispatcher pre-commit and CI check lock freshness and 4 platforms | A | 07 7.3 |
| D17 | schemas hand-written, property test generates instances from Go types and validates both ways (no generator dependency) | A | 03, 10 |
| D18 | catalog collapsed to one domain list per tool + per-domain upload flag | A | 03 3.5, 07 |
| D19 | merged with sa M2 | M | 04 doctor |
| D21 | SessionEnd hook writes a facts-only handoff; `rm` warns when a generation has no final handoff | A | 08 8.2, 04 |
| D22 | generation id is a ULID | A | 01, 03 |
| D23 | state timestamps exempt from the idempotency rule | A | 04 4.1 |
| D24 | A3 refocused on memory dirs and the read-only bin mount | A | 11 |
| D25 | `layout up` reports a stale layout by run-config hash | A | 04 |
| D26 | merged with te H5 | M | 10 |
| D27 | `julieta pin check` covers the config repo's CI workflow julieta pin | A | 04 |
| sa C1 | fsck on transfer; tree walk validated (modes, names, HFS/case variants, collisions); write via `os.Root`; only after approval | A | 05 I27, 06 6.1 |
| sa H1 | kit and workload capabilities parsed and normalized into the widening set; kit domains through `egress.Split`; file-level kit diff; config kits may not declare network or mounts | A | 01 1.4, 06 6.1, I28 |
| sa H2 | candidates in `.romeu/pending/`, promoted only on approval; approval+drift preflight on every sbx path | A | 01 1.4, 04, I7 |
| sa H3 | `termsafe` escaping on all host output; ref names checked with check-ref-format rules; fuzz | A | 05 I29 |
| sa H4 | salvage covers all worktrees, HEADs, tags, notes, embedded repos; host-provided base SHAs; expected salvage id; daemon cross-check; missing repo = incomplete | A | 08 8.4, I17 |
| sa H5 | dedicated signing-only key and socket, confirm-on-use; B measures `ssh-add -L`; residual risk listed | A | 06, 05 5.4, Q19 |
| sa M1 | `ref` in widening set; absent `upload` = true; object stores upload-capable | A | 01, 03 3.5 |
| sa M2 | doctor fails on global sbx secrets for used services, broad allow rules, permissive default; A9 extended | A | 04, 11 |
| sa M3 | git floor per patched series (Q10); `transfer.bundleURI=false`, `gc.auto=0`, `maintenance.auto=false` | A | 05 5.1 |
| sa M4 | doctor checks mise trust paths, direnv, workspace trust disabled, global lfs filter; both workspaces opened in Restricted Mode | A | 04, 02 |
| sa M5 | approval records the rendered `sbxenv.yaml` hash; toolchain ack holds romeu version; run recomputes from disk and live `sbx version`; `agent` and `salvage.excludeIgnored` gated | A | 01 1.4 |
| sa M6 | host-config allowlist of workload repositories; repository change highlighted | A | 03 3.4, 01 |
| sa L1 | after a daemon fetch, heads compared with `julieta status` | A | 04 pull |
| sa L2 | merged into D8 and D13 | M | 08, 04 |
| sa L3 | attestation verify mandatory in release.yml and J1; govulncheck; herdr pinned by checksum | A | 10, 09 J1 |
| sa IT | invariant test adequacy list applied to I2, I3, I5, I7, I8, I9, I12, I14, I16, I19 | A | 05 5.2 |
| te C1 | e2e origins via `git http-backend` over httptest TLS (host config `caFile`); gitsafe refuses `file://` and paths; no test override | A | 10, 05 |
| te C2 | `probe-result.v1` schema; `tools/ci probes` | A | 11 |
| te C3 | fake sbx replays recorded argv only; recordings per sbx version; known argv fixed | A | 10 10.3 |
| te C4 | `docs/acceptance.json` + `tools/ci acceptance` as final task | A | 10 10.5 |
| te C5 | product side: generic `julieta memory import` + `memory verify`; operator-side verification noted outside the repo | A | 08, 04 |
| te H1 | `ci-bootstrap` first task; interim evidence = recorded local `tools/ci all` | A | 0, 10 |
| te H2 | Intel macOS runner (label verified in `ci-bootstrap`); arm64 on `ubuntu-24.04-arm`; no QEMU | A | 10 10.2 |
| te H3 | J2, J3b, J7, J10, J11 in CI; ptmx helper for TTY | A | 10 |
| te H4 | S8 measured in CI with fake sbx | A | S8 |
| te H5 | guard/test tags; `tools/ci invariants` and `mutate`; 10.1 claims corrected | A | 10 |
| te H6 | golden governance and determinism seams | A | 10 10.4 |
| te M1-M6, M8 | idempotency meta-test, concurrency tests, cache keyed by lock sha, exit codes from data, I19 port test, catalog check in config repo CI, scheduled fuzz | A | 10 |
| te M7 | merged into sa L3 | M | 10 |
| te L1-L3 | deterministic parts of Q8 are the gate; hygiene denylist self-test; doctor tests with temp HOME | A | 10 |
| det-memory | julieta stamps id/created/updated | A | 08 8.1 |
| det-session | SessionStart prints open memory entries and the handoff | A | 08 8.2 |
| det-agentmem | agent memory dir made non-writable and checked by hook and status | A | 08 8.1 |
| det-seq | ADR and spec sequence numbers checked by `tools/ci` | A | 10 |
| fid submodules | "plain clones, never submodules" stated | A | 0, 02 |
| fid herdr-host | probe re-confirming host herdr cannot see sandbox processes (A13) | A | 11 |
| fid keybindings | julieta kit configures herdr's tmux-style prefix | A | 06 |
| fid cmux | the host window runs only `romeu run <p>` | A | 09 J3 |
| fid mise-project | D: the project level stays empty; tools belong to repos so one lock serves sandbox, CI and host, and a spec-level tool list would have no lockfile | D | 07 7.2 |
| fid lock-over-toml | stated as a deliberate deviation | A | 07 7.5 |
| fid removed-project | `orphaned` + explicit `retire`; no `--prune` (sync never deletes, so there is nothing for a flag to allow) | A | 04 |
| fid repo-rename | v1 manual path J13 | A | 09 |
| scope-salvage | justified | A | 0 section 6 |
| scope-extras | features traced; extras marked v1.1 | A | 0 section 6 |

Closed during round 1: Q5 (`sbx env plan` output is not part of romeu's
gate; sbx shows its own plan and prompts during `sbx env run`).
