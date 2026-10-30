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
needs:
  - T021
  - T032
  - T031
  - T036
  - T051
  - T043
after: []
ask_first: []
sitting: none
---

# T055 - `julieta spec validate` and `julieta pin`

Goal: `julieta spec validate` and `julieta pin`.

Re-plan (2026-10-12): Kept. It feeds the config repo's `validate.yml`.
Its J2 step lives in its own file, `e2e/scenarios/j2/step1_*.go`, in the
directory T043 owns, so this task comes `after` T043 (rule 1 of
`docs/plan.md`).

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
