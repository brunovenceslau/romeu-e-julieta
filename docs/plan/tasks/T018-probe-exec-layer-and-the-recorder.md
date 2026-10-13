---
id: T018
title: "Probe exec layer and the recorder"
milestone: M6
module: probes
fate: deferred
owns: []
needs:
  - T014
after: []
ask_first: []
sitting: none
---

# T018 - Probe exec layer and the recorder

Re-plan (2026-10-12): deferred to M6; a recording of the maintainer's
host committed to a public repository before the redaction package has
its unit tests is an irreversible-harm risk; hand-written sessions hold
no host data by construction. Reopened by: a real-sbx behaviour the fake
cannot model breaks a daily run; or the first release candidate. The
text below is the one written before the re-plan; `needs` lists its
earlier dependencies with a folded id replaced by the id that absorbed
it, and `owns` is written when the task reopens.

- Implements: REQ-0752 (10 10.1), REQ-0753 (10 10.1), REQ-0754 (10
  10.1), REQ-0755 (10 10.1), REQ-0756 (10 10.1), REQ-0757 (10 10.1),
  REQ-0758 (10 10.1), REQ-0759 (10 10.1), REQ-0760 (10 10.1), REQ-0761
  (10 10.1), REQ-0762 (10 10.1), REQ-0763 (10 10.1), REQ-0764 (10 10.1),
  REQ-0765 (10 10.1), REQ-0766 (10 10.1), REQ-0767 (10 10.1), REQ-0768
  (10 10.1), REQ-0769 (10 10.1), REQ-0770 (10 10.1), REQ-0771 (10 10.1),
  REQ-0772 (10 10.1), REQ-0773 (10 10.1), REQ-0774 (10 10.1), REQ-0924
  (10 10.3), REQ-0925 (10 10.3), REQ-0926 (10 10.3), REQ-0929 (10 10.3),
  REQ-0932 (10 10.3), REQ-0933 (10 10.3), REQ-0934 (10 10.3), REQ-0935
  (10 10.3), REQ-0936 (10 10.3), REQ-0937 (10 10.3), REQ-0939 (10 10.3),
  REQ-0977 (11 (opening)), REQ-0978 (11 (opening)), REQ-1041 (12 12.1).
- Operator: feeds block O3.
- Ask-first, as forecast before the re-plan: none expected.
- Acceptance:
  - The exec layer is tested against a helper binary re-executed from
    the test (10 10.1).
  - The recorder scrubs positively. At start it computes the host's own
    identifiers, the home path, `$USER`, the host name, the root path,
    the git `user.name` and `user.email`, and every sandbox name `sbx ls
    --json` reports, holds them in memory and never writes them: each is
    replaced by a placeholder before a file is written, and the recorder
    refuses to write a file that still holds one (10 10.3, Redaction).
    Each U+2014 becomes `-`.
  - The recorder replaces the same four values as the details of 04 4.1
    (the home path, the user name, the host name and the root path),
    asserted by one fixture shared with T029's details table.
  - Of a listing from the maintainer's real host, an `sbx ls --json`, a
    policy listing, the policy log, an approval listing or a settings
    read, the recorder keeps only the entries of the harness's own
    throwaway sandboxes and env dirs and keeps, for each of the rest,
    its field names and its scope marker with every value replaced by a
    typed placeholder, so the shape of a row survives and no name does.
    A placeholder parses as the type it replaces, so a recording still
    parses; T029 round-trips each placeholder through its parsers and
    lists the placeholder types, and the test here uses a stand-in
    parser until then. The reason: a name on that host may be one this
    repository must not hold (index, Boundaries).
  - The host global git config, the editor's trust settings and an
    `ssh-add -L` listing pass through typed parsers that keep only what
    a doctor row reads (04 4.2): for the git config, one yes or no per
    row pattern of the doctor table (04 4.2), with a count of the keys
    that matched a wildcard pattern and never the text the wildcard
    stood for: no subsection, alias name, URL base or `includeIf`
    condition is written; for the trust settings, each key name with a
    yes or no, never a value, a path or an identity; for a key listing,
    the key type and the count, never the key line or its comment (11
    11.1, A1 and A12).
  - Its tests plant a fake token, a home path, a host name, a user name,
    an email, a git identity, an ssh public key with a comment, a
    foreign sandbox name and a foreign workspace path, and assert that
    none reaches any file.
  - The git-config test plants
    `url.https://u:tok@git.example/.insteadOf`,
    `includeIf.gitdir:/work/acme/.path` and `alias.acme`, and asserts
    that none of `tok`, `git.example` and `acme` reaches a file.
  - Every file the harness writes, results and artifacts included, goes
    through one write function that scrubs and refuses, on the file's
    path as well as its content. A test writes a result whose
    observation holds each planted identifier and expects the refusal.
  - The harness refuses an `--out` inside a clone whose `core.hooksPath`
    is not `.githooks`. A failed or interrupted run removes its
    sandboxes, scoped rules and scoped secrets, and a start-of-run sweep
    by prefix removes leftovers; both are tested against the helper
    binary. The refusal, the cleanup and the sweep are the plan's
    additions.
  - It keeps stdout only for the allowlist of read commands whose output
    the parsers need, and no secret output (10 10.3).
- Verify: `go test ./e2e/probes/...`.
