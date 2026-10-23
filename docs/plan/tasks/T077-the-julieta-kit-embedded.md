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
  - e2e/kit_os_base_test.go
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
  '^TestKit(Julieta|OSBase)' -v`. The pattern names the
  `TestKitJulieta*` and `TestKitOSBase*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).

Re-plan (2026-10-22): resolving `mise` by a fixed path is deferred to
this task. The manifest carries the path of the kit's mise, and julieta
runs mise by that path instead of finding `mise` on `PATH`. Reopened by:
T077 defining where the kit installs mise. Until then julieta runs the
`mise` that `PATH` resolves (T053).

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
- A12 measured, and `os-base` lands with tzdata and zsh (decided on
  2026-10-23). A12 was measured at a sitting on the supported pair (a Mac
  on Apple silicon, sbx v0.49.2, a linux/arm64 sandbox) against the
  pinned workload
  `docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5`,
  with `sbx create` and `sbx exec`. The workload has bash, coreutils,
  grep, curl, ca-certificates, git, sudo, openssh-client and locales, and
  lacks tzdata (no `/usr/share/zoneinfo`) and zsh; the login shell of
  `agent` is `/bin/sh`, and `apt-get` works on the Debian 13 (trixie)
  sources. So the missing set is not empty, and REQ-0593 gives `os-base`
  that set: `kits/os-base/os-base.yaml` installs tzdata and zsh as root
  at install time, and carries the arg `tz` (default `Etc/UTC`, as
  `TZ`), which 02 2.1 gives it ("minimal apt layer, TZ arg"); the
  `julieta` kit no longer has the arg. The apt delta is not pinned (05
  5.4); the descriptor's description says so. Its install phase opens
  `deb.debian.org:443` only: apt reads only a copy of the workload's
  `/etc/apt/sources.list.d/debian.sources` with each `http://` URI made
  `https://` (HTTPS always, decided on 2026-10-23, scope: the product
  kits of this repository, written here and in the descriptor; reopened
  by a source that serves no HTTPS), whose two URIs
  (`http://deb.debian.org/debian` and `.../debian-security`) share that
  host, and not its other source file (`dhi.io`), which the packages do
  not need. `apt-get update` ends with status 0 when an index fails, so
  it runs with `--error-on=any` (`TestKitOSBaseMirrorDown`). The
  container e2e composes `os-base` after `julieta`, as the project spec
  of 03 lists them, so `TestKitJulietaInstall` asserts that
  `Europe/Lisbon` resolves (WET or WEST) and that a non-interactive zsh
  finds julieta and the shims through `~/.zshenv`; it no longer asserts
  the absence of `/usr/share/zoneinfo`. `TestKitOSBaseInstall` asserts
  instead that the workload still lacks both packages before the step, so
  it fails the day the workload brings one, and the set is read again.
  The descriptor's provides list (`e2e/testdata/workload-descriptor.yaml`,
  `providesTzdata` and `providesZsh` false) agrees and is unchanged. The
  rest of A12 that sitting measured is not this task's: the handle
  `sbx-kit-claude`, kind sandbox, schema v3; the volumes the workload
  declares, of which none survives `sbx rm` and a same-name `sbx create`
  (what survives `sbx env rm` and a recreate is not measured); the
  skills path, a read-only host mount; and the agent memory dir,
  `~/.claude/projects`. The ssh-agent part of A12 was not measured: the
  workload declares no ssh-agent capability, so `SSH_AUTH_SOCK` was unset
  inside with or without a host agent. It is the pending note "The open
  parts of A12". Reopened by: A12's re-measure in M6, or a workload pin
  that changes the set.
- The acceptance lines this build holds, each by a test: the file name
  and capability names (`TestKitJulietaDescriptor`); mise and herdr at
  the pins and the install-phase domains (`TestKitJulietaInstall`,
  `TestKitJulietaDescriptor`, and `TestFSHoldsEveryProductKit` and
  `TestJulietaSumsEqualThePins` in `kits/embed_test.go`); the `os-base`
  package list and its install-phase domain (`TestKitOSBaseDescriptor`,
  `TestKitOSBaseInstall`, `TestKitOSBaseMirrorDown`); the `~/.zshenv`
  and `BASH_ENV` lines, the herdr config and `TZ`
  (`TestKitJulietaInstall`, `TestKitJulietaKeepsWhatIsThere`); that the
  steps run once in the container e2e (`TestKitJulietaInstall`), stop on
  a sha256 that is not the pin (`TestKitJulietaSumsGuardTheInstall`) and
  on an arg that is not a version (`TestKitJulietaHostileArg`); and
  `kits/embed.go`, the one `embed.FS`, whose kits all parse with the
  product's descriptor grammar (`TestEveryKitParses`). The line "With no
  `os-base`, the kit sets `TZ`" is the branch A12 did not take; `TZ` is
  set by `os-base` instead (above). These wait: the line "romeu embeds `kits/*` ... and `sync`
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
  - that the install phase of `os-base` opens `deb.debian.org:443` to apt
    (the sandbox at the sitting reached the mirror under the workload's
    own policy, not under this kit's), and that an arg's `env` sets `TZ`
    in the sandbox's own processes and not only in the hooks that
    declare it;
  - how sbx expands `${{ }}` in a command (the kit uses none now), that
    `HOME` is set in a hook that runs as `agent` (the workload's own hook
    relies on it), and that a failed install hook stops the build;
  - that the owner of `/etc/sandbox-persistent.sh` is unchanged by a
    sandbox that builds it differently from the image.
- `go:embed` (decided on 2026-10-22, rebased on 2026-10-23): the lint
  step refuses the directive outside `scopedDirectives`, so
  `kits/embed.go` cannot pass `go run ./tools/ci all` without a reviewed
  entry there, as ADR 0007 provides. The entry is in its own first
  commit: `kits/embed.go`, a file entry (a scope with no trailing slash
  names one file, so the rest of `kits/` stays refused), beside
  `internal/catalog/`; and a `go:embed` pattern with the `all:` prefix
  is refused wherever the directive is allowed. It is outside this
  task's `owns` and is the patch to review apart from this task.
- The embed takes every product kit of the tree (decided on
  2026-10-23): `julieta`, `os-base` and the descriptor of
  `git-ssh-sign` (T081), whose `testdata/` holds its tests and is not
  embedded. Reopened by: a kit whose descriptor names a file of its
  directory other than the descriptor.
- Where the kit installs mise (decided on 2026-10-23): at
  `/usr/local/bin/mise`, a fixed path, which is the trigger of the
  re-plan of 2026-10-22 above. The manifest field and julieta running
  mise by that path are julieta's code (T053's package), outside this
  task's `owns`, and are left to a task that names them. Reopened by: a
  kit that installs mise elsewhere.
