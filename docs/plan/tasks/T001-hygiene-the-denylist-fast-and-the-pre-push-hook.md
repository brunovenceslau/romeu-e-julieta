---
id: T001
title: "Hygiene, the denylist, `fast` and the pre-push hook"
milestone: M0
module: ci-bootstrap
fate: merged
owns: []
needs: []
after: []
ask_first: []
sitting: none
merged: 2026-10-02
pr: 7
---

# T001 - Hygiene, the denylist, `fast` and the pre-push hook

Merged on 2026-10-02 in pull request #7. The text below is the one
written before the re-plan.

- Implements: REQ-0016 (index), REQ-0021 (index), REQ-0022 (index),
  REQ-0028 (index), REQ-0032 (index), REQ-0034 (00 0.4), REQ-0035 (00
  0.4), REQ-0111 (02 2.1), REQ-0124 (02 2.1), REQ-0542 (05 5.3),
  REQ-0767 (10 10.1), REQ-0790 (10 10.1), REQ-0800 (10 10.2), REQ-0825
  (10 10.2), REQ-0861 (10 10.2), REQ-0865 (10 10.2), REQ-0866 (10 10.2),
  REQ-0882 (10 10.2), REQ-0884 (10 10.2), REQ-0885 (10 10.2), REQ-0886
  (10 10.2), REQ-0887 (10 10.2), REQ-0888 (10 10.2), REQ-0889 (10 10.2),
  REQ-0891 (10 10.2), REQ-0892 (10 10.2), REQ-0893 (10 10.2), REQ-0894
  (10 10.2), REQ-0895 (10 10.2), REQ-0896 (10 10.2), REQ-0897 (10 10.2),
  REQ-0915 (10 10.2), REQ-0926 (10 10.3), REQ-1040 (12 12.1), REQ-1046
  (12 12.1), REQ-1061 (12 12.1), REQ-1062 (12 12.1), REQ-1083 (12 12.2),
  REQ-1128 (12 12.4), REQ-1142 (12 12.4), REQ-1143 (12 12.4), REQ-1240
  (10 10.2).
- Operator: block O1 finishes and pushes it.
- Ask-first, as forecast before the re-plan: none exists yet; the
  maintainer's review is the guard (10 10.2).
- Acceptance:
  - `go.mod`, `mise.toml` and `mise.lock` (Go and `golangci-lint`
    pinned, four platforms, 02 2.1), `COPYING`, `REUSE.toml` and
    `LICENSES/` exist (00 0.4).
  - `go run ./tools/ci hygiene` fails a fixture for each of its rules in
    10 10.2: U+2014, a listed prose word (with the self-test of
    `tools/ci/prose.yaml`), a `TODO` without an issue, a personal
    absolute path, a tracked `go.work` or `vendor/`, a second file in
    `.githooks/`, and a missing or empty denylist.
  - The matcher also reads one file given as `go run ./tools/ci hygiene
    --file <path>`, with a fixture for a file that holds a listed name
    and one that does not; that flag is the plan's addition.
  - The matcher's self-test plants a made-up name written together, with
    each separator and with a slash, against its own denylist, and
    expects a failure for each form except the slash.
  - `hygiene add` refuses to start without a terminal and takes no name
    as an argument; a `/dev/ptmx` test drives it (10 10.1).
  - The pushed-range walk passes its fixture-repository table: a new
    ref, a ref the remote lacks, a URL that is no remote, a merge
    commit, a deleted ref (10 10.2).
  - `.githooks/pre-push` is the one line of 12 12.1, mode 100755.
- Verify: `go run ./tools/ci fast` exits 0 after block O1 adds the
  denylist entries, and exits non-zero before.
