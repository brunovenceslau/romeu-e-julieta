---
id: T141
title: "`tools/ci setup` installs the pinned mise"
milestone: M0
module: ci-bootstrap
fate: new
owns:
  - tools/ci/internal/mise/**
  - tools/ci/internal/setup/**
needs: []
after:
  - T105
ask_first:
  - checks
sitting: none
---

# T141 - `tools/ci setup` installs the pinned mise

Goal: `go run ./tools/ci setup` installs the mise that `ci.yml` pins
when the mise it would run names another version or none, into
`<user cache directory>/romeu-e-julieta/mise/<version>/mise`, after it
checks the download against the sha256 of the matrix of `ci.yml`; every
mise command of `tools/ci` then runs that copy by its absolute path,
checked again on every run. `fast` and `all` keep refusing a mise of
another version, and the refusal names `setup`.

Added on 2026-10-16: a sandbox recreated that day shipped mise
2026.9.15 in `/usr/local/bin` from its image while `ci.yml` pins
2026.10.3, so `tools/ci` refused to run until the pinned mise was
installed by hand (downloaded, checked with `sha256sum -c` against the
matrix, put in `~/.local/bin`). The pin belongs to this repository, so
the repository gets it, whatever the image ships. `after: T105`,
which owns `tools/ci/**` and moves the mise package and the setup
command into `tools/ci/internal/`; the `owns` globs follow that move,
and a path T105 names differently is renamed here when the task starts.

- Implements: REQ-0859 (10 10.2), REQ-1065 (12 12.1),
  REQ-1236 (10 10.2), REQ-1237 (10 10.2), REQ-1238 (10 10.2).
- Operator: no
- Acceptance:
  - `setup` reads the version and the sha256 of this platform's runner
    label from the matrix of `ci.yml` and from nowhere else; the
    platform maps to the label by the Runners list of 10 10.2.
  - The download goes through `internal/procgroup` or Go's HTTP client
    under the class `tool-step`, with the proxy variables and
    `SSL_CERT_FILE`; the file is written beside its final path, its
    sha256 is compared before it is made executable or run, a mismatch
    removes it and fails `setup` naming both hashes, and a match puts it
    in place by rename.
  - Every mise command of `tools/ci` (`mise --version`, `mise trust`,
    `mise install`, `mise which`) starts one absolute path, the copy in
    the cache when it is there and its sha256 still equals the
    matrix's, or else the mise on the search path.
  - The tests of REQ-1238 run against a fake release served locally: a
    wrong hash, a right hash, a cached copy changed after the install,
    and a mise on the search path at the pin with no download.
  - The refusal of `fast` and `all` names `go run ./tools/ci setup`.
  - Two facts are measured when the task starts and recorded in its
    pull request (deferred decisions; trigger: T141 starts):
    - which release asset of mise for `macos-26` and `macos-26-intel`
      has the sha256 the matrix holds, the bare binary or an archive;
      for linux arm64 the bare binary `mise-v2026.10.3-linux-arm64`
      matched on 2026-10-16. If an archive, `setup` checks the archive
      and extracts the one binary, and the acceptance above reads
      "archive" for "file".
    - whether the mise shims (`~/.local/share/mise/shims`) that the
      cached mise writes start that mise and not another one on the
      search path; if they start another, the task states what
      `setup` does about it (rewrites the shims, or prints the
      `PATH` line to add), and the push hook's hint of 12 12.1 follows.
- Verify: `go test ./tools/ci/... -run '^TestSetupInstallsThePinnedMise'`;
  on a machine whose mise is another version, `go run ./tools/ci setup`
  then `go run ./tools/ci all` exit 0 with nothing else installed by
  hand.
