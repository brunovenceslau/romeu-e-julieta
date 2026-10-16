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
  - Every mise command of `tools/ci` (`mise --version`, `mise trust`,
    `mise install`, `mise which`) starts the one absolute path of
    REQ-1236.
  - The tests of REQ-1238 run against a fake release served locally,
    one case per clause of that requirement.
  - The refusal of `fast` and `all` names `go run ./tools/ci setup`.
  - Two facts are measured when the task starts, before any code, and
    written into the spec by a docs pull request that merges first when
    either changes its text (deferred decisions; trigger: T141 starts):
    - which release asset of mise for `macos-26` and `macos-26-intel`
      has the sha256 the matrix holds, the bare binary or an archive;
      for linux arm64 the bare binary `mise-v2026.10.3-linux-arm64`
      matched on 2026-10-16, and the comment of the matrix in `ci.yml`
      calls it "the mise binary". REQ-1237 already covers an archive.
    - whether the mise shims (`~/.local/share/mise/shims`) that the
      cached mise writes start that mise and not another one on the
      search path. If they start another, the pre-push hook and its
      hint in 12 12.1 (REQ-1046) change with this task: the hint names
      `setup`, or the hook starts go through the cached mise.
- Verify: `go test ./tools/ci/... -run '^TestSetupInstallsThePinnedMise'`;
  on a machine whose mise is another version, `go run ./tools/ci setup`
  then `go run ./tools/ci all` exit 0 with nothing installed by hand.
