---
id: T055
title: "`julieta spec validate` and `julieta pin`"
milestone: M3
module: julieta-core
fate: kept
owns:
  - internal/cmd/julieta/spec/**
  - internal/cmd/julieta/pin/**
  - internal/cmd/julieta/all/import_spec.go
  - internal/cmd/julieta/all/import_pin.go
  - e2e/julieta_spec_test.go
  - e2e/julieta_pin_test.go
  - e2e/scenarios/j2/step1_*.go
  - examples/.github/workflows/validate.yml
  - internal/cmd/romeu/sync/step10_report.go
  - internal/cmd/romeu/sync/step10_report_test.go
  - docs/spec/04-cli.md
  - schemas/julieta-spec.v1.json
  - tools/ci/internal/schema/julieta.go
  - .gitattributes
  - internal/gitsafe/fetch_tip.go
  - internal/gitsafe/fetch_tip_test.go
needs:
  - T021
  - T032
  - T031
  - T036
  - T051
  - T043
after: []
ask_first:
  - checks
  - contracts
  - formats
  - gitsafe
sitting: none
---

# T055 - `julieta spec validate` and `julieta pin`

Goal: `julieta spec validate` and `julieta pin`.

Re-plan (2026-10-12): Kept. It feeds the config repo's `validate.yml`.
Its J2 step lives in its own file, `e2e/scenarios/j2/step1_*.go`, in the
directory T043 owns, so this task comes `after` T043 (rule 1 of
`docs/plan.md`). Since 2026-10-30 T043 is in `needs` instead
(the re-plan below).

Re-plan (2026-10-22): Changed, decided by the maintainer. This task
takes from T021 the example workflow
`examples/.github/workflows/validate.yml`, which pins a julieta release
by version and archive sha256, verifies it against `checksums.txt`
before it runs, and passes `julieta pin check --workflows`; it is the
fixture of that check's test. It moved because no julieta release
exists to pin yet and this task builds `julieta pin check`. The file is
a YAML file under `examples/`, so the hint rule of REQ-0836 applies to
it; this task settles how it meets it.

Re-plan (2026-10-30): takes from T043 the warning
`julieta-pin-mismatch` of step 10 of 04 4.2 (REQ-0345), since it reads
the pin of `validate.yml` whose format and reader this task defines;
it owns `step10_report.go` of `sync` and its test for it, and T043
moves from `after` to `needs`. Decided by the operator on 2026-10-30,
scope this repository.

Re-plan (2026-10-30): built in three slices, decided by the operator
on 2026-10-30, scope this repository. Slice A is `julieta spec
validate`: the first acceptance line, with `--json`
(`internal/cmd/julieta/spec/`, `import_spec.go`,
`e2e/julieta_spec_test.go`). Slice B is `julieta pin workload` and
`pin check` for spec workloads; it needs a tag-to-digest read through
the guarded client of `internal/oci`, outside this file set. Slice C
is `pin check --workflows`, `examples/.github/workflows/validate.yml`,
the warning `julieta-pin-mismatch` of `romeu sync` and the J2 step; it
needs a rule of `tools/ci schema` for a workflow under `examples/`
(`checks`), a world of `e2e/internal/romeu` whose `Run` accepts a
julieta line, with one line each in `e2e/scenarios/j2/commands.yaml`,
`j2.go` and `j2_test.go`, and the reader of `validate.yml`. That reader
goes in a new package, `internal/pin`, linked by both binaries, which
needs a row in the import table of 10 10.7 and in
`tools/ci/internal/imports/rows.go` (`checks`); slice C builds it.
Decided for slice A, the same day:
- A file under `projects/` that is not `*.yaml` is found by reading
  the directory of each file given in `projects/`, since the glob of
  the config repo's CI never names one, and the summary says
  `spec validate <n> files (<k> from projects/)` when it added some.
- The manifest of the size check is built from the spec alone with
  stand-ins, the root `/` among them, a lower bound that sync measures
  again.
- `--catalog` reads each lock with a shallow fetch, the one commit at
  the tip of the ref, through the new `TipFile` of gitsafe
  (`internal/gitsafe/fetch_tip.go`, `gitsafe`), decided by the
  operator on 2026-10-30 in place of a full clone; what gitsafe refuses
  of the origin's content is the finding `lock-unparsed`, and a failed
  fetch, a missing ref or an origin with no default branch is
  `fetch-failed`.
- An origin `--catalog` cannot read is the new error `fetch-failed`
  (`RJ-360`, exit 1, a row of 04 4.4 declared in the command's
  `errors.go`); a lock host the catalog does not produce is the new
  warning `lock-host-gated` of 04 4.1; a lock that does not parse, or
  that gitsafe refuses, is the finding `lock-unparsed`, exit 6, now in
  the command's exit-code cell of 04 4.3.
- `--json` prints the format `julieta-spec.v1`, whose schema is
  registered in `tools/ci/internal/schema/julieta.go`: the files judged,
  the findings, each `{file, message, path, rule}` as internal/spec
  names one, the rules not checked and the warnings, each text escaped
  (I29); under `--json` stderr holds nothing but an error.
- A lock artifact URL with no host julieta can read is the warning
  `lock-host-gated` too, and a pass of `--catalog` is no gate on lock
  hosts, which sync gates; the config repo's CI runs it for trusted
  authors only, or under restricted egress (from the security review
  of 2026-10-30).
This task now owns `docs/spec/04-cli.md`, which T185, T188 and T189
own too; T055 is on the critical path, so it comes first: T185 takes
T055 in its `after`, and T188 and T189, which come after T185, follow
it through that edge.

- Implements: REQ-0009 (index), REQ-0038 (00 0.4), REQ-0117 (02 2.1),
  REQ-0119 (02 2.1), REQ-0134 (02 2.2), REQ-0135 (02 2.2), REQ-0174 (03
  3.1), REQ-0178 (03 3.1), REQ-0211 (03 3.5), REQ-0420 (04 4.3),
  REQ-0421 (04 4.3), REQ-0422 (04 4.3), REQ-0423 (04 4.3), REQ-0433 (04
  4.4), REQ-0435 (04 4.4), REQ-0519 (05 5.2), REQ-0577 (06 6.2),
  REQ-0639 (07 7.6), REQ-0728 (09 (opening)).
- Operator: feeds block O4.
- Acceptance:
  - `spec validate` exits 6 (`check-findings`) on any decode, schema or
    rule error, per file and across files, and with `--catalog` on an
    unknown `backend:tool`; a missing lock host is a warning that
    changes no exit code; exit 2 is only a bad flag or an unreadable
    file. It prints one line naming the host-only rules of 03 3.1 it did
    not check, and reports a manifest over 32 KiB as a finding
    (`RJ-313`, 03 3.6), by the one size function of `internal/manifest`
    (T135). A file under `projects/` that is not `*.yaml` is a finding
    naming the path.
  - `pin workload` rewrites the digest after it verifies that the
    manifest list covers linux/amd64 and linux/arm64, through the
    guarded client of `oci`; the arch check lives here (06 6.2).
  - `pin check` reports a pin that is not a digest or is behind, and
    with `--workflows` the julieta release pinned in the config repo's
    workflows (04 4.3), reading
    `examples/.github/workflows/validate.yml` (T021) as its fixture; it
    exits 6 (`check-findings`) when it reports one.
  - `romeu sync` warns `julieta-pin-mismatch` when the julieta release
    the config repo's `validate.yml` pins is not the version romeu
    embeds, through the reader of `pin check --workflows`, and does not
    warn when they are equal (from T043, 2026-10-30).
  - The J2 scenario function gains its step 1, `julieta spec validate`
    in the config project's sandbox, at the hybrid level.
- Verify: `go test ./internal/... -run '^Test(SpecValidate|Pin)'`, which
  lists the tests by name; `go test -tags e2e ./e2e/... -run
  '^(TestJulietaSpec|TestJulietaPin)' -v`. The pattern names the
  `TestJulietaSpec*` and `TestJulietaPin*` functions this task adds, and
  the reviewer reads them in the `-v` output (10 10.8).
