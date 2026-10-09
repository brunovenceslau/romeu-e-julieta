# 9. Developer journeys

Back to [index](../spec.md). Reader: implementers of the scenario
functions. Each guide page in `docs/guide/` rewrites its journey for the
operator: the commands typed and what the terminal shows, without the
step tags or internal states. Type: how-to per journey.

Legend: **[H]** romeu on the host, **[X]** sbx invoked by romeu, **[J]**
julieta in the sandbox, **[A]** agent, **[O]** operator
([01 1.7](01-system-model.md#17-vocabulary)). Each journey is a scenario function in `e2e/scenarios/`
run by the host suite (`e2e/host`, real sbx) and, for J2, J3b, J6, J7,
J10, J11, also by CI with the fake sbx; each has a guide page in
`docs/guide/`. A scenario function runs the [H], [X] and [J] steps,
replays an [A] step as commits on the fixture origin, and treats an
[O] step that installs something or opens an editor as a precondition
it checks. In CI, J6 forwards `julieta salvage` through `sbx env exec`
to julieta at the hybrid level, and a fixture process stands in for the
agent. J12 runs J1 steps 3-5 and J12 step 2 under a fresh root,
settings and state on the same host, after the first run's sandboxes
are removed (J1 steps 1-2 are installs, preconditions it checks); a
second physical machine stays a documented path. In CI a journey that
reaches a julieta call runs at the hybrid level, on the Linux runners
([10 10.1](10-testing-style.md#101-test-levels)).

What a journey passing means in CI: a scenario asserts, at each [X]
step, the argv romeu passes to the fake sbx, and at each [H] step the
state records the step writes; a step the fake cannot perform (attach,
a live daemon fetch) is asserted by its argv only and listed in the
scenario's omitted steps, which the host suite covers.

Journeys are told from the operator's side. What the agent is told is
asserted by the hook and command tests of [08](08-memory-handoff-salvage.md)
and [04](04-cli.md), and the maintainer's release flow is the ordered
checklist of
[12 12.2](12-engineering.md#122-development-commands-and-capability-map),
which names each step's command and output path.

## J1 Onboarding (new machine, first time)

0. [O] Create the config repo from the starter in `examples/` (CC0-1.0,
   [00 0.4](00-scope.md#04-license)): copy it into a new repository,
   edit `projects/<cfg>.yaml` by hand (its name and the config repo's
   own entry under `repos`, [03 3.2](03-formats.md#32-project-spec-projectsnameyaml-schema-projectv1)),
   push. The config repo names private repositories, secret names and
   internal domains: keep it private unless every project in it is
   public ([02 2.2](02-layouts.md#22-config-repo)).
1. [O] Read the README's trust paragraph. Install sbx at or above the
   floor of [10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract)
   (initial v0.46.0), git at or above the floor of
   [05 5.1](05-security.md#51-hardened-git-internalgitsafe) (initial
   floor 2.45.4), and `gh` at or above the floor of 12 12.1 (initial
   floor 2.68.0, the first with both flags of the `gh attestation
   verify` line of step 2). The verification of step 2 needs network
   access and a `gh` logged in to GitHub.
2. [O] Download the `romeu` darwin archive for the host arch and
   `checksums.txt` with `curl`, which sets no quarantine attribute, so
   Gatekeeper is not asked about an unsigned binary (inferred; B1
   confirms it on both hosts). Verify with
   `shasum -a 256 -c checksums.txt`, then read the commit the release's
   tag names from GitHub with
   `gh api repos/<owner>/romeu-e-julieta/commits/<tag> --jq .sha` and
   verify with
   `gh attestation verify <archive> --repo <owner>/romeu-e-julieta
   --signer-workflow <owner>/romeu-e-julieta/.github/workflows/release.yml
   --source-digest <commit> --deny-self-hosted-runners`, `<commit>`
   being that output (both checks required; the verification contract
   of [10 10.2](10-testing-style.md#release-and-bootstrap), one entry of
   `e2e/scenarios/commands.yaml` that J9 uses too, which gives the three
   commands as one block to paste, with the tag as its one variable),
   put `romeu` on `PATH`. Skipping the verification is the user's
   choice; it costs the one proof that the archive is the one
   `release.yml` built from the tagged commit.
3. [O] `romeu init --config-url https://github.com/<owner>/<config-repo>`
   - [H] applies `doctor`'s root check before writing settings and
     refuses a root inside a git repository (a `$HOME` that holds
     dotfiles in git, for example) with that check's error id and a hint
     to pass another root; then writes host settings (root default
     `$HOME/dev`, absolute tool paths) and clones the config repo to
     `$ROMEU_ROOT/<cfg>-env/<dir>`.
4. [O] `romeu doctor`; fix every failure; apply its recommendations
   (`sbx settings set env.rememberHostCommands true`);
   `romeu approve --toolchain` (gate 1).
5. [O] Fill the host settings the config project needs
   ([03 3.4](03-formats.md#34-host-settings-settingsyaml-schema-host-settingsv1)):
   `gitHosts`, `workloadRepositories`, `signing.agentSocket` when a
   project uses `git-ssh-sign`, and `secrets` entries named
   `<name>@<project>` (romeu prints the missing names on sync). Prefer
   one fine-grained token per project, limited to that project's
   repositories and the scopes it needs: the sandbox holds whatever the
   entry binds ([05 5.4](05-security.md#54-known-residual-risks-accepted-in-v1)). For a
   project that uses `git-ssh-sign`, register the key on GitHub as a
   signing key and never as an authentication key, and start the
   dedicated signing agent with exactly that one key, added with
   `ssh-add -c` so each use asks for confirmation; that needs a program
   `SSH_ASKPASS` can name, which macOS does not ship, so install one
   first and set `SSH_ASKPASS` for the agent; what a signing use does
   without one is the outcome probe B2 records
   ([06 6.4](06-kits.md#64-product-kits)); sync
   exits 2 with `RJ-204 signing-socket` until it is reachable and holds
   that key.
6. [O] `romeu sync <cfg>` -> gate 2 on TTY (repos, secrets, kits and
   their capabilities, workload, gated egress) -> approve.
7. [O] `romeu run <cfg>` -> sandbox created -> tools installed -> herdr
   layout with the agent pane focused; the operator logs the agent in
   inside the sandbox, once, on this first run.

## J2 New project

1. [A] in the config project's sandbox: write `projects/foo.yaml`,
   `julieta spec validate projects/foo.yaml`, commit, push a branch, open
   a PR.
2. [O] bind each secret the spec names to a host command, one
   `<name>@foo` entry per secret in host settings, for example
   `github@foo: {service: github, argv: [/usr/bin/security, find-generic-password, -s, romeu/foo/github, -w]}`,
   after storing the value with
   `security add-generic-password -s romeu/foo/github -a "$USER" -w`
   (a missing entry makes sync exit 2, and its fix hint prints the
   entry to paste); as in J1 step 5, prefer a fine-grained token limited
   to `foo`'s repositories. Then either after merge `romeu sync foo`, or before
   merge `romeu sync --from <sha> foo`, with the head of the branch
   pushed in step 1 (the spec is read from a commit of origin, never
   from a working tree). On the `--from` path the gate 2 diff is the
   only review before the spec applies; fields outside the widening set
   (for example `run`) apply unreviewed inside the sandbox.
3. [H] clones repos, creates memory dirs, derives egress, renders,
   gate -> [O] approve.
4. [O] `romeu run foo`.

## J3 Run, sandbox exists (J3a) and absent (J3b)

On the host the operator opens one terminal window and runs
`romeu run <p>` in it; that process becomes the
sandbox's herdr client. Nothing else runs on the host. See `romeu run` in
[04](04-cli.md#how-romeu-run-reaches-the-run-layout). J3a: run step 3
starts a stopped sandbox or reattaches to a running one without
re-provisioning, and refuses with `recreate-needed` a stopped one whose
env file changed; `julieta setup` is a no-op when locks are unchanged
(S8). J3b: create, record generation, egress,
`julieta setup` installs everything, then attach. If the tree has an
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
   The narrative handoff is the operator's step: romeu cannot ask a
   stopped agent for one.
2. [O] `romeu recreate foo` (or `romeu rm foo`).
3. [H] `romeu salvage foo` (mandatory, deterministic): [H] preflight;
   the handoff reader checks for a `final` handoff of the generation
   before anything is stopped: on a TTY romeu asks whether to continue
   without one, and without a TTY it warns and continues; generation
   `open -> salvaging`, new salvage id and host base SHAs for the
   generation's recorded repo set; [X->J] `julieta salvage
   --stop-agents`; [H] verify, unbundle, cross-check daemon heads,
   import; complete (or accepted, step 4) -> generation
   `salvaging -> removing`, written before `sbx env rm`.
4. [H] incomplete -> exit 5 listing what would be lost, each with its
   reason; the operator fixes each reason as the guide page says for it
   (pushes the work, frees the space, starts the sandbox) or reruns with
   `--accept-loss=<reason>[,<reason>]`, naming each reason it accepts.
5. [X] `sbx env rm <dir>`; [H] remove romeu-applied egress rules;
   generation `removing -> closed-removed`. If `sbx env rm` fails, the
   generation stays `removing` and romeu exits 1; the same command run
   again resumes at `sbx env rm` when a fresh fingerprint of the daemon
   heads and each worktree's HEAD and full status equals the one written
   with `removing` ([03 3.8](03-formats.md#38-host-state-schemas-state-v1)),
   and otherwise salvages again first. For `recreate`, continue with J3b. If that create fails because an upstream artifact is gone
   (a workload digest, the frontend, a kit download host), the
   generation stays closed, the salvage refs and memory are intact and
   no sandbox exists: the error is `kit-build-failed`, and the operator
   fixes the pin or the catalog, then runs `romeu run`.
6. [O] restore salvaged work in the new sandbox: push the salvage
   ref's worktree commit from the host clone into the new sandbox's
   daemon remote and check it out there, or bring a branch back through
   origin as J10 step 3 does; apply `stash-<n>`; extract
   `ignored.tar.gz` inside the sandbox only, never on the host
   ([08 8.5](08-memory-handoff-salvage.md#recovery)).

## J7 Retire a project

1. [A] remove `projects/foo.yaml` in the config repo (PR, merge).
2. [O] `romeu sync` reports `orphaned: foo`.
3. [O] `romeu retire foo`: rm (with salvage) if a sandbox exists; list
   per repo: dirty tree, stashes, branches not on origin, salvage refs
   not reachable from origin; confirm on TTY; move `foo-env/` to
   `$ROMEU_ROOT/.attic/foo/<ts>/`; drop from workspace files; move state
   and approvals to the state attic.
4. [O] Once, in each attic clone, `git -C <clone> status` shows a clean
   tree, `git -C <clone> log --branches --glob='refs/romeu/salvage/*'
   --not --remotes --oneline` prints nothing, `git -C <clone> stash
   list` lists no stash and `git -C <clone> worktree list` lists no
   linked worktree, the operator may delete
   `$ROMEU_ROOT/.attic/foo/<ts>/` and
   `$XDG_STATE_HOME/romeu/attic/foo/<ts>/`; romeu never does, and
   `status` and `doctor` do not read them.

## J8 Tool bump (inside a repo)

1. [A] edit `mise.toml`; `julieta lock`; commit (the `pre-commit`
   `lock --check` warns when the catalog does not know a new
   `backend:tool`, naming the two fixes of
   [07 7.5](07-mise-egress.md#75-egress-derivation-internalegress)
   step 2); open the PR.
2. [O] after the PR merges into the branch named by the spec's `ref`,
   `romeu sync foo`: new non-upload catalog domains apply live;
   upload-capable or unknown hosts -> gate.
3. [A] `julieta install`: egress follows `ref`, so a tool whose domains
   are not already allowed installs once the change has merged and
   sync has applied it ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress));
   other machines get it on their next `run` (`julieta setup`). Before
   the lock merges, egress follows the `ref` branch, so a new host is
   approved once through sbx for this sandbox (the install error prints
   the command, and the approval is lost at recreate).

## J9 Kit or workload bump

- romeu release: install and verify it with the same
  `e2e/scenarios/commands.yaml` entry as J1 step 2 ->
  `romeu approve --toolchain`
  (gate 1, once per machine; it prints the julieta version change and the
  catalog diff per affected project) -> `romeu sync --all`; julieta binaries
  update through the read-only mount without recreate; only projects
  whose kit content or capabilities changed see gate 2 and need
  `romeu recreate <name>`.
- Workload: [A] `julieta pin workload projects/foo.yaml` in the config
  sandbox, whose `egress.extra` holds the registry hosts
  ([07 7.5](07-mise-egress.md#75-egress-derivation-internalegress)) -> PR -> merge -> [O] `romeu sync foo` (gate: new digest) ->
  `romeu recreate foo`.
- Personal kit: [A] edit `kits/<id>/` in the config repo -> PR -> sync ->
  gate -> recreate.
- sbx update on the host: the next `romeu sync` or `run` exits 3 (gate
  1, [04 4.2](04-cli.md#42-romeu-host)); [O] `romeu doctor` shows the
  new sbx version against the floor and the recording set
  ([10 10.3](10-testing-style.md#103-fake-sbx-fidelity-contract));
  `romeu approve --toolchain`. A version the recording set does not
  cover is untested: `doctor` says so, and the operator either accepts
  it or reinstalls the previous sbx; a later command that fails on a
  changed sbx output is recovered with J6 (salvage, `--accept-loss`) or
  J10.

## J10 Recover after sandbox death

1. [O] `romeu status foo` shows `absent; generation <G> open; snapshot
   <T>; refs/sandboxes last fetched <T2>`.
2. [O] `romeu run foo`: [H] preserves generation G as the closed-lost
   row of [01 1.6](01-system-model.md#generation) says (it keeps a lost
   record `salvage --from-host` already wrote, else imports
   `refs/sandboxes/foo/*` and every `snapshot/heads.bundle` into
   `refs/romeu/salvage/foo/<G>/<salvage-id>/`), moves it to
   `closed-lost` with `result: lost`, prints the lost record with its
   salvage ref prefix and exits 1; the next `romeu run foo` creates a
   new sandbox (new ULID). What the last snapshot holds is kept:
   committed work. Uncommitted work since that snapshot is lost
   ([08](08-memory-handoff-salvage.md)).
3. [O] recover a branch on the host, with the hooks, the fsmonitor and
   the protocols of [05 5.1](05-security.md#51-hardened-git-internalgitsafe)
   switched off on the command line:
   `git -c core.hooksPath=/dev/null -c core.fsmonitor=false -C <clone> branch recover/<b> refs/romeu/salvage/foo/<G>/<salvage-id>/<dir>/<b>`,
   then
   `git -c core.hooksPath=/dev/null -c core.fsmonitor=false -c protocol.allow=never -c protocol.https.allow=always -C <clone> push origin recover/<b>`.
   Both lines are an entry of `e2e/scenarios/commands.yaml`, which
   J10's CI scenario runs.

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
5. When `sync` ran without a TTY, it printed the widening digest and
   exited 3 with nothing kept; the operator runs `romeu sync foo` again
   on a terminal.

## J12 Second machine

1. J1 steps 1-5 on the new machine (approvals, secret bindings and
   memory are machine-local by design). Agent memory and handoffs stay
   on the machine that wrote them (Q14).
2. `romeu sync --all` -> one gate per project -> `romeu run <name>`.
3. Unpushed work from the other machine is not available (push first).
   A memory entry file is self-contained: copying
   `memory/<dir>/entries/` whole is a complete move, and the store lock
   is recreated on first use
   ([03 3.9](03-formats.md#39-memory-entry-schema-memory-entryv1)).

## J13 Remove, rename or split a repo inside a project (v1 manual path)

1. [A] edit the spec (remove the repo, change its URL, or add the new
   repos); PR; merge.
2. [O] `romeu sync foo` -> gate 2 (repos changed) -> `romeu recreate foo`
   (salvage runs first, over the generation's recorded repo set, so the
   removed repo is still salvaged).
3. [H] new repos get new clone dirs; a removed repo's clone and memory
   dir stay in place and are reported by `status` as `orphaned repo dir`
   with any unpushed or salvage-only work listed.
4. [O] after `status` shows nothing unpushed and no salvage-only refs,
   the operator moves or deletes the orphaned dirs by hand (romeu never
   does). A new command for this is out of v1 (Q6).

## J14 Stop using romeu and julieta

1. [O] push or recover unpushed work (J10 step 3).
2. [O] `romeu retire` each project (J7).
3. What stays, on purpose: the host clones, as plain clones; memory, as
   plain Markdown ([03 3.9](03-formats.md#39-memory-entry-schema-memory-entryv1));
   `$ROMEU_ROOT/.attic`, which holds salvage payloads that can carry
   credentials: delete it as J7 step 4 says once no longer needed. A root excluded from Time Machine, as the
   `root-indexed` check of [04 4.2](04-cli.md#42-romeu-host) advises, is
   in no Time Machine backup, so the operator keeps another copy of the
   memory dirs, the handoffs and any work not pushed.
4. [O] What the operator removes by hand, as the guide page shows: host
   settings, host state, romeu-applied egress rules, the refs under
   `refs/romeu/` and `refs/sandboxes/` (`git for-each-ref`, then
   `git update-ref -d`), the keychain entries named by the secret argv
   in host settings, the signing agent socket key (01 1.2 marks both
   kept), and any remaining sbx sandboxes and volumes.

The scenario runs after J7 and checks `git fsck` on each host clone,
that no ref is left under `refs/romeu/` or `refs/sandboxes/` after step
4, and that every memory file parses as 03 3.9 says without julieta.
