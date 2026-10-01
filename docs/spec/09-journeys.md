# 9. Developer journeys

Back to [index](../spec.md). Reader: implementers of the scenario
functions and the guide pages; the operator for the order of steps.
Type: how-to per journey.

Legend: **[H]** romeu on the host, **[X]** sbx invoked by romeu, **[J]**
julieta in the sandbox, **[A]** agent, **[O]** operator. Each journey is a scenario function in `e2e/scenarios/`
run by the host suite (`e2e/host`, real sbx) and, for J2, J3b, J7, J10,
J11, also by CI with the fake sbx; each has a guide page in
`docs/guide/`. A scenario function runs the [H], [X] and [J] steps,
replays an [A] step as commits on the fixture origin, and treats an
[O] step that installs something or opens an editor as a precondition
it checks. J12 runs J1 steps 3-6 under a fresh root, settings and
state on the same host, after the first run's sandboxes are removed;
a second physical machine stays a documented path. In CI a journey
that reaches a julieta call runs at the hybrid level, on the Linux
runners ([10 10.1](10-testing-style.md#101-test-levels)).

## J1 Onboarding (new machine, first time)

1. [O] Install sbx (>= v0.46.0), git at or above the floor of
   [05 5.1](05-security.md#51-hardened-git-internalgitsafe) (initial
   floor 2.45.4), and a `gh` that has the `attestation` command, which
   step 2 needs (12 12.1).
2. [O] Download the `romeu` darwin archive for the host arch, verify with
   `shasum -a 256 -c checksums.txt` and
   `gh attestation verify <archive> --repo <owner>/romeu-e-julieta
   --signer-workflow <owner>/romeu-e-julieta/.github/workflows/release.yml`
   (both required), put `romeu` on `PATH`.
3. [O] `romeu init --config-url https://github.com/<owner>/<config-repo>`
   - [H] writes host settings (root default `$HOME/dev`, absolute tool
     paths), clones the config repo to `$ROMEU_ROOT/<cfg>-env/<dir>`.
4. [O] `romeu doctor`; fix every failure; apply its recommendations
   (`sbx settings set env.rememberHostCommands true`);
   `romeu approve --toolchain` (gate 1).
5. [O] Add `secrets: <name>@<project>` entries to host settings for the
   config project (romeu prints the missing names on sync).
6. [O] `romeu sync <cfg>` -> gate 2 on TTY (repos, secrets, kits and
   their capabilities, workload, gated egress) -> approve.
7. [O] `romeu run <cfg>` -> sandbox created -> tools installed -> herdr
   layout with the agent pane focused.

## J2 New project

1. [A] in the config project's sandbox: write `projects/foo.yaml`,
   `julieta spec validate projects/foo.yaml`, commit, push a branch, open
   a PR.
2. [O] either after merge `romeu sync foo`, or before merge
   `romeu sync --from <sha> foo`, with the head of the branch pushed
   in step 1 (the spec is read from a commit of origin, never from a
   working tree).
3. [H] clones repos, creates memory dirs and the ledger's spool and
   view dirs, ingests the spool (empty on a first sync,
   [13 13.4](13-runtime-ledger.md#134-ingest)), derives egress,
   renders, gate -> [O] approve.
4. [O] `romeu run foo`.

## J3 Run, sandbox exists (J3a) and absent (J3b)

On the host the operator opens one terminal window (for example a cmux
window) and runs `romeu run <p>` in it; that process becomes the
sandbox's herdr client. Nothing else runs on the host. See `romeu run` in
[04](04-cli.md). J3a: `sbx env run -d` starts or
reattaches without re-provisioning; `julieta setup` is a no-op when
locks are unchanged (S8). J3b: create, record generation, egress,
`julieta setup` installs everything, then attach. In both, romeu
ingests the project's spool right after the preflight, and
`julieta setup` drains the spool files that now have an entry; an
ingest that fails is reported and `run` goes on (13 13.4). If the tree has an
open generation with no sandbox, romeu preserves it first (J10). If a
sandbox of the project's name exists that romeu has no generation for
(for example after the host state was lost), `run` exits 2 with
`RJ-203 unknown-sandbox`; the operator checks it and runs
`romeu adopt <name>` on a TTY, after which `run`, `salvage` and `rm`
work as usual.

## J4 Pull and review agent work on the host

1. [O] `romeu pull -C $ROMEU_ROOT/foo-env/foo-api` (or from inside that dir).
2. [H] primary: hardened fetch from the live daemon into
   `refs/sandboxes/foo/*`; secondary: `julieta snapshot --all`, then
   import `snapshot/heads.bundle` into `refs/romeu/snapshots/foo/*`.
3. [H] compare fetched heads with `julieta status`; update
   `review/<dir>/` (a standalone repo with the host clone as alternate,
   local hardening config) to the sandbox's current branch head,
   detached; print path and commit.
4. [O] open `review.code-workspace` in VS Code Restricted Mode (never
   trusted).

## J5 Handoff before `/clear` (sandbox survives)

1. [O] `/handoff` in the agent pane.
2. [A] writes the narrative -> [J] `julieta handoff write` stamps facts,
   writes `handoff/<ULID>-clear.md`.
3. [O] `/clear`: [J] the `SessionEnd` hook writes a `facts` handoff,
   then the new session's `SessionStart` hook prints the latest
   narrative (the one from step 2, never displaced by the facts file),
   the facts that changed since, open memory entries, `lesson` entries
   and julieta's warnings; the agent resumes. Without `/handoff`, the
   facts handoff is still there. The container e2e runs the two hooks in
   both orders, so the rule does not wait for Q22; the guide describes
   the order probe C5 records.

## J6 Handoff and salvage before recreate or rm (nothing lost)

1. [O] optional `/handoff --final` (agent half; the agent may be dead).
2. [O] `romeu recreate foo` (or `romeu rm foo`).
3. [H] `romeu salvage foo` (mandatory, deterministic): [H] preflight,
   generation `open -> salvaging`, new salvage id and host base SHAs for
   the generation's recorded repo set; [H] ingest the spool (a failed
   ingest is recorded as `ledger-incomplete` and blocks nothing,
   13 13.4); [X->J] `julieta salvage
   --stop-agents`; [H] verify, unbundle, cross-check daemon heads,
   import; the handoff reader warns if the generation has no `final`
   handoff.
4. [H] incomplete -> exit 5 listing what would be lost; the operator
   fixes it or reruns with `--accept-loss`.
5. [X] `sbx env rm <dir>`; [H] remove romeu-applied egress rules;
   generation `salvaging -> closed-removed`. For `recreate`, continue
   with J3b.

## J7 Retire a project

1. [A] remove `projects/foo.yaml` in the config repo (PR, merge).
2. [O] `romeu sync` reports `orphaned: foo`.
3. [O] `romeu retire foo`: rm (with salvage) if a sandbox exists; list
   per repo: dirty tree, stashes, branches not on origin, salvage refs
   not reachable from origin; confirm on TTY; move `foo-env/` to
   `$ROMEU_ROOT/.attic/foo/<ts>/`; drop from workspace files; move state
   and approvals to the state attic. Before the move romeu ingests
   the spool once more, for the events julieta wrote during the
   sandbox half. The spool and the view then move with `foo-env/`;
   the project's ledger entries stay in the ledger (13 13.5).

## J8 Tool bump (inside a repo)

1. [A] edit `mise.toml`; `julieta lock`; `julieta install`; commit; PR.
2. [O] after the PR merges into the branch named by the spec's `ref`,
   `romeu sync foo`: new non-upload catalog domains apply live;
   upload-capable or unknown hosts -> gate.
3. Running sandboxes already have the tool (installed from inside);
   other machines get it on their next `run` (`julieta setup`).

## J9 Kit or workload bump

- romeu release: install and verify it -> `romeu approve --toolchain`
  (gate 1, once per machine; it prints the julieta version change and the
  catalog diff per affected project) -> `romeu sync --all`; julieta binaries
  update through the read-only mount without recreate; only projects
  whose kit content or capabilities changed see gate 2 and need
  `romeu recreate <name>`.
- Workload: [A] `julieta pin workload projects/foo.yaml` in the config
  sandbox -> PR -> merge -> [O] `romeu sync foo` (gate: new digest) ->
  `romeu recreate foo`.
- Personal kit: [A] edit `kits/<id>/` in the config repo -> PR -> sync ->
  gate -> recreate.

## J10 Recover after sandbox death

1. [O] `romeu status foo` shows `absent; generation <G> open; snapshot
   <T>; refs/sandboxes last fetched <T2>`.
2. [O] `romeu run foo`: [H] preserves generation G (imports
   `refs/sandboxes/foo/*` and every `snapshot/heads.bundle` into
   `refs/romeu/salvage/foo/<G>/<salvage-id>/`), moves it to
   `closed-lost` with `result: lost`, creates a new sandbox (new ULID).
   The dead sandbox's spool is a host directory, so the same `run`
   ingests the events it left (13 13.4).
3. [O] recover a branch on the host:
   `git -C <clone> branch recover/<b> refs/romeu/salvage/foo/<G>/<salvage-id>/<dir>/<b>`
   and push it (docs show the hardened form).

## J11 Egress or secret change approval

1. [A] spec edit (`secrets: [+npm]`, `egress.extra: [+api.example.com]`).
2. [O] `romeu sync foo`: gate shows the new secret name with its
   host-settings argv, or the new domain with its upload flag.
3. On approval with a running sandbox: [H] `sbx policy allow network
   --sandbox foo <d>` (live, no recreate). Secrets: by default a new or
   changed secret is recreate-class and `status` says so; if probe A10
   confirms the live path (Q18), romeu runs `sbx secret set <svc>
   --sandbox foo --command <rendered command>` instead (never a value).
4. If host settings lack `npm@foo`: exit 2 naming the key; the operator
   adds it and reruns.
5. When `sync` ran without a TTY, the candidate is awaiting and
   `romeu approve foo` promotes it. `approve` makes no sbx call, so it
   prints that the live changes of step 3 apply on the next
   `romeu run` or `romeu sync`.

## J12 Second machine

1. J1 steps 1-5 on the new machine (approvals, secret bindings and
   memory are machine-local by design).
2. `romeu sync --all` -> one gate per project -> `romeu run <name>`.
3. Unpushed work from the other machine is not available (push first).

## J13 Remove, rename or split a repo inside a project (v1 manual path)

1. [A] edit the spec (remove the repo, change its URL, or add the new
   repos); PR; merge.
2. [O] `romeu sync foo` -> gate 2 (repos changed) -> `romeu recreate foo`
   (salvage runs first, over the generation's recorded repo set, so the
   removed repo is still salvaged).
3. [H] new repos get new clone dirs; a removed repo's clone and memory
   dir stay in place and are reported by `status` as `orphaned repo dir`
   with any unpushed or salvage-only work listed.
4. [O] after `status` shows nothing unpushed, the operator moves or
   deletes the orphaned dirs by hand (romeu never does). A new command
   for this is out of v1 (Q6).
