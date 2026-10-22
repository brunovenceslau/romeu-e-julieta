---
id: T077
title: "The `julieta` kit, embedded"
milestone: M2
module: kits
fate: kept
owns:
  - kits/julieta/**
  - kits/os-base/**
  - e2e/kit_julieta_test.go
  - kits/embed.go
  - kits/embed_test.go
  - e2e/testdata/workload-descriptor.yaml
needs:
  - T076
  - T012
  - T013
after:
  - T123
ask_first:
  - kits
sitting: none
---

# T077 - The `julieta` kit, embedded

Earlier title: The `julieta` and `os-base` kits, embedded.

Goal: The `julieta` kit: mise and herdr at the pins, the install-phase
domains, the env lines; `kits/embed.go`, the one `embed.FS` of the
product kit directories.

Re-plan (2026-10-12): Kept. `os-base` lands only if sitting M2 shows a
missing package. herdr is downloaded and verified as 06 6.4 says while
`layout` (T056) stays deferred. Acceptance added: the kit builds and
starts in a daily sandbox (sitting M2); the install steps run once in
the container e2e, whose harness (T013) this task needs and whose cases
live in `e2e/kit_julieta_test.go` (rule 1 of `docs/plan.md`). "The
descriptor file name and the capability names A11 recorded" in the rows
below are the ones read by hand at sitting M2 (T113), fixtures with
`source: sitting M2`; "the missing set A12 measured" is the set sitting
M2 shows, and `os-base` lands only if it is not empty; A11 and A12
re-measure both in M6. The embed and materialization row below is
asserted by T038 and T043 (M3) when the embed seam and the `sync`
sessions land; at this task the kit is built and started by hand at
sitting M2 (T113) and used by the wave 1 sandboxes (T115).
`kits/embed.go` is the one `embed.FS` of the product kit directories,
`pins.yaml` excluded, and lands with the first kit; `after: T123`, since
the v3-kits record is merged before the first kit lands (12 12.5):
[ADR 0019, support kit schema v3 only](../../adr/0019-support-kit-schema-v3-only.md).

- Implements: REQ-0008 (index), REQ-0043 (01 1.1), REQ-0116 (02 2.1),
  REQ-0128 (02 2.1), REQ-0129 (02 2.1), REQ-0155 (02 2.5), REQ-0589 (06
  6.4), REQ-0590 (06 6.4), REQ-0591 (06 6.4), REQ-0592 (06 6.4),
  REQ-0593 (06 6.4), REQ-0606 (06 6.4), REQ-0611 (07 7.2).
- Operator: no.
- Acceptance:
  - The two kits use the descriptor file name and the capability names
    A11 recorded.
  - `julieta` installs mise and herdr at the pins of the pin file and
    declares the two install-phase domains of 06 6.4; the `os-base`
    package list is the missing set A12 measured.
  - The kit writes `~/.zshenv` and the `BASH_ENV` file with lines adding
    `$HOME/.local/bin` and the mise shims dir, so a non-interactive bash
    and zsh find julieta and the shims; asserted in the container e2e.
  - The herdr config sets `[update] version_check=false,
    manifest_check=false` and the prefix `ctrl+b`; asserted in the
    container e2e.
  - With no `os-base`, the kit sets `TZ` from arg `tz`, default
    `Etc/UTC`; asserted in the container e2e.
  - romeu embeds `kits/*` through `kits/embed.go`, and `sync`
    materializes them into content-addressed directories; the e2e reuses
    the `sync` sessions of T043.
  - The install steps of the `julieta` kit run once in the container e2e
    as a smoke test, under the network rule of 10 10.1 and Q27; a kit
    build itself is what B2 measures (10 10.3, Cannot model).
  - That they build and start is measured by B2 (S3).
- Verify: `go run ./tools/ci kits`; `go test -tags e2e ./e2e/... -run
  '^TestKitJulieta' -v`. The pattern names the `TestKitJulieta*`
  functions this task adds, and the reviewer reads them in the `-v`
  output (10 10.8).

Decisions taken while building (2026-10-22), each with what reopens
it:

- The facts of A11 come from sitting M0 (decided on 2026-10-22): the
  descriptor file name (`<stem>.yaml`, here `kits/julieta/julieta.yaml`)
  and the capability names (`com.docker.sandbox/<type>@<version>`) are
  those of the record
  `docs/handoff/decisions/00000000005z9f102d4ptbtjbm-sitting-m0-the-kit-frontend-is-a-go.md`,
  whose "sitting M2" wording predates the re-plan that moved the
  go/no-go to M0. The keys of `network-policy@1`, of the hook `env` and
  `user`, and of an arg's `pattern` and `env` are read from the workload
  descriptor, the annotation `vnd.docker.sandbox.kit.descriptor` of the
  pinned workload (14878 bytes, sha256 `86e9133c...`, as the M0 record
  says, checked again), kept as the conformance fixture
  `e2e/testdata/workload-descriptor.yaml`. Reopened by: A11's re-measure
  in M6.
- `os-base` does not land, and the A12 gap stands (decided on
  2026-10-22): the packages the workload lacks were never measured, so
  the acceptance line "the `os-base` package list is the missing set A12
  measured" stands unmet and the kit takes the branch the task and
  REQ-0593 give for an empty set: `TZ` from arg `tz`, default `Etc/UTC`.
  `os-base` waits for the full A12 measurement at the sitting. One
  missing package is seen already: the workload has no tzdata (its
  descriptor lists no `deb/tzdata`, and `/usr/share/zoneinfo` is absent).
  So the arg `tz` is inert for any real zone: `TZ=Europe/Lisbon` reads
  as an unknown zone and gives UTC. `Etc/UTC` gives the offset +0000 and
  the abbreviation `Etc`, which is the zone wanted by accident.
  `TestKitJulietaInstall` asserts the absence of `/usr/share/zoneinfo`,
  so it fails the day tzdata arrives and the case is read again.
  Reopened by: the A12 measurement, or the first missing package that
  matters seen in a sandbox, which then plans `os-base`.
- The acceptance lines this build holds, each by a test: the file name
  and capability names (`TestKitJulietaDescriptor`); mise and herdr at
  the pins and the install-phase domains (`TestKitJulietaInstall`,
  `TestKitJulietaDescriptor`, and `TestFSHoldsEveryProductKit` and
  `TestJulietaSumsEqualThePins` in `kits/embed_test.go`); the `~/.zshenv`
  and `BASH_ENV` lines, the herdr config and `TZ`
  (`TestKitJulietaInstall`, `TestKitJulietaKeepsWhatIsThere`); that the
  steps run once in the container e2e (`TestKitJulietaInstall`), stop on
  a sha256 that is not the pin (`TestKitJulietaSumsGuardTheInstall`) and
  on an arg that is not a version (`TestKitJulietaHostileArg`); and
  `kits/embed.go`, the one `embed.FS`. These wait: the `os-base`
  package list (above); the line "romeu embeds `kits/*` ... and `sync`
  materializes them", whose materialization half is asserted by T038 and
  T043 (M3), since `sync` does not exist yet; and the line "they build
  and start", measured by B2 (S3) and by hand at sitting M2 (T113,
  T115).
- No arg is pasted into script text (decided on 2026-10-22): the install
  steps run as root, so the version args reach them as environment
  variables (`env:` on the arg, declared by the hook), carry a `pattern`
  of digits and dots, and each step checks its value with `[[ ... =~ ]]`
  before it uses it. The arg `bashEnv` is gone: the file is the
  workload's `BASH_ENV`, `/etc/sandbox-persistent.sh`, a fixed path that
  `TestKitJulietaInstall` reads from the image's own environment, and
  the kit appends to it and does not set `BASH_ENV`. Reopened by: a
  workload whose `BASH_ENV` is another file.
- The install steps are `bash -euc` scripts of at most five lines
  (decided on 2026-10-22): bash, since its `HOSTTYPE` picks the
  architecture with no command substitution, which the `kits` check
  forbids; `-e` and `-u`, so a failed command stops the step and an arg
  that did not arrive is an error. The downloads go to a root-only
  directory made with `mkdir -m 0700` at a fixed path, not a `mktemp -d`
  one: naming a `mktemp` directory needs `$(`, which the check forbids,
  and `mkdir` fails if the path exists. It is removed by the last step,
  with no `trap`, since five lines hold no more and the download and its
  check stay in one step, as the check demands. The last step checks the
  files once more before it installs them, since a hook that failed may
  not stop the build (not measured). `curl` takes `--proto '=https'
  --proto-redir '=https' -fsSL --retry 3 --max-time 120`. The hashes are
  in `kits/julieta/SHA256SUMS`, held equal to the pin file by
  `TestJulietaSumsEqualThePins`; `sha256sum -c --ignore-missing` fails
  when no line names a downloaded file ("no file was verified", GNU
  coreutils 9.7), which `TestKitJulietaSumsGuardTheInstall` holds with a
  sums file that has no line for the name. The sums file is not a
  `lifecycle.files` body: M0 measured that a `files` body expands an
  arg and did not record the keys of a `files` entry, so the kit has no
  grammar to follow; the file is a file of the kit directory, which sbx stages beside the descriptor (the workload's
  `agent-context@1` reads `/usr/share/sandbox/kit/claude/claude-context.md`,
  a file of its own kit).
- The lines appended to other kits' files (decided on 2026-10-22): the
  PATH line is appended to `~/.zshenv` and to the `BASH_ENV` file once,
  with `if ! grep -qxF ... then printf >> ... fi`, which the `kits` check
  allows, so a second run adds nothing and the lines of other kits stay
  (`TestKitJulietaKeepsWhatIsThere`). The herdr config cannot be
  appended to, since a second `[update]` table is invalid TOML; it is
  written only when there is none, so a config another kit wrote first
  wins, and one written later replaces it.
- Not measured, each with the build (B2, or sitting M2) as the trigger
  that reads it:
  - the config of `network-policy@1`: the workload's descriptor shows the
    phase `runtime` with `allow` entries of `host:port` and shows no
    other phase. The kit names the phase `install`, so github.com is not
    open to the agent at runtime; whether sbx takes `install` as a phase
    is not measured, and a refusal at the build is read as that;
  - that sbx stages `SHA256SUMS` at `/usr/share/sandbox/kit/julieta/` for
    a mixin kit;
  - how sbx expands `${{ }}` in a command (the kit uses none now), that
    `HOME` is set in a hook that runs as `agent` (the workload's own hook
    relies on it), and that a failed install hook stops the build;
  - that the owner of `/etc/sandbox-persistent.sh` is unchanged by a
    sandbox that builds it differently from the image.
- `go:embed` (decided on 2026-10-22): the lint step refuses the
  directive, so `kits/embed.go` cannot pass `go run ./tools/ci all`
  without a reviewed line in `allowedDirectives`' file, as ADR 0007
  provides. The line is in its own first commit, scoped by path
  (`embedSites`, today `kits/embed.go`; T031's file joins it), with the
  `all:` prefix refused; it is outside this task's `owns` and is the
  patch to review apart from this task.
