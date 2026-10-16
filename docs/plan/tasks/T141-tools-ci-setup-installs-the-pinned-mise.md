---
id: T141
title: "`tools/ci setup` installs the pinned mise"
milestone: M0
module: ci-bootstrap
fate: new
owns:
  - tools/ci/internal/mise/**
  - tools/ci/internal/setup/**
  - .githooks/pre-push
needs: []
after:
  - T105
ask_first:
  - checks
sitting: none
---

# T141 - `tools/ci setup` installs the pinned mise

Goal: `go run ./tools/ci setup` installs the mise that `ci.yml` pins
when the mise `tools/ci` would run names another version or none, into
`<user cache directory>/romeu-e-julieta/mise/<version>/mise`, after it
checks the download against the sha256 of the matrix of `ci.yml`; every
mise command of `tools/ci` then runs that copy by its absolute path,
checked again on every run. `fast` and `all` keep refusing a mise of
another version, and the refusal names `setup`.

Re-plan (2026-10-16): New. A sandbox recreated that day shipped mise
2026.9.15 in `/usr/local/bin` from its image while `ci.yml` pins
2026.10.3, so `tools/ci` refused to run until the pinned mise was
installed by hand (downloaded, checked with `sha256sum -c` against the
matrix, put in `~/.local/bin`). Before this task, REQ-0859 made `setup`
refuse such a mise too; it now installs the pin instead. The pin belongs
to this repository, so the repository gets it, whatever the image
ships. `after: T105`, which owns `tools/ci/**` and moves the mise
package and the setup command into `tools/ci/internal/`; a path T105
names differently is renamed here when the task starts. T129 (setup's
module sources) and T053 (the mise driver) own the same packages and
wait for this task, which unblocks the local CI of every new sandbox
and so comes first.

- Implements: REQ-0859 (10 10.2), REQ-1065 (12 12.1),
  REQ-1236 (10 10.2), REQ-1237 (10 10.2), REQ-1238 (10 10.2),
  REQ-1239 (10 10.2).
- Operator: no
- Acceptance:
  - `setup` reads the version and the sha256 of this platform's runner
    label from the matrix of `ci.yml` and from nowhere else; the
    platform maps to the label by the Runners list of 10 10.2; the
    version and the sha256 are checked for their shape before either
    reaches a path or a URL.
  - The download is Go's HTTP client, not a subprocess, with the URL
    prefix, the redirect rule, the TLS rule, the 15-minute limit and the
    256 MiB cap of REQ-1237; the file handling is REQ-1239's: an
    exclusive new file of mode 0600 beside the final path, hashed as it
    is written, removed on any error, never made executable before the
    hash matches, put in place by rename, in directories of mode 0700
    with no symbolic link among them.
  - The URL is the fixed prefix of REQ-1237 and
    `mise-v<version>-<os>-<arch>` of the runner label of this platform,
    and a test pins the name for each of the four labels.
  - Every mise command of `tools/ci` (`mise --version`, `mise trust`,
    `mise install`, `mise which`) starts the one absolute path of
    REQ-1236.
  - The tests of REQ-1238 run against a fake release served locally,
    one case per clause of that requirement.
  - The refusal of `fast` and `all` names `go run ./tools/ci setup`.
  - Two facts were measured on 2026-10-16, before any code:
    - The sha256 of each of the four runners in the matrix of `ci.yml`
      is that of the bare binary `mise-v2026.10.3-<os>-<arch>` in the
      release's `SHASUMS256.txt` (`linux-x64`, `linux-arm64`,
      `macos-arm64`, `macos-x64`), so `setup` downloads that binary and
      reads no archive (REQ-1237 says so).
    - A mise shim in `~/.local/share/mise/shims` is a symbolic link to
      the absolute path of the mise that wrote it: in a sandbox whose
      image ships mise in `/usr/local/bin`, the shims pointed at the
      pinned mise in `~/.local/bin` that last ran. After `setup`, the
      `mise install` it runs with the cached mise writes the shims, so
      they point at the cache path until another mise rewrites them.
      A shim that points at the older mise of the image (2026.9.15)
      still starts the go of `mise.toml`: `go version` and `go list -m`
      exit 0, with a warning on standard error that this mise cannot
      read `mise.lock` version 3. `tools/ci` starts mise by the path of
      REQ-1236 and never through a shim, and the pre-push hook needs
      only `go`, so the hook and its hint (REQ-1046) do not change.
- Verify: `go test ./tools/ci/... -run '^TestSetupInstallsThePinnedMise'`;
  on a machine whose mise is another version, `go run ./tools/ci setup`
  then `go run ./tools/ci all` exit 0 with nothing installed by hand.
