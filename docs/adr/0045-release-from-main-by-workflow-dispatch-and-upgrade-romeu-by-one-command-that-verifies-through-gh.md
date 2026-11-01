# 45. Release from main by workflow_dispatch and upgrade romeu by one command that verifies through gh

Date: 2026-11-01

## Status

Proposed

## Context

Two things had been left open until a release was near. First, how
`release.yml` is started: the development sandbox's token can push a
`v*` tag (05 5.4), so a workflow started by the tag would take its own
file from the tagged commit, and the index kept a deferred row with one
constraint fixed, that the file must not come from that commit. Second,
how a release reaches a host: J9 had the operator download, check and
install by hand with the entry of J1 step 2, then run `romeu approve
--toolchain` and `romeu sync --all`; and REQ-1060 said romeu itself
never starts `gh`, so the attestation could only be checked by hand.

On 2026-10-31 release automation and `romeu upgrade` went in front of
the M3 critical path, because the binaries in use were old: building
and installing one by hand is impractical, and a release made from one
host had no path to another.

What is at stake: the verification contract (10 10.2) is what an
operator and `romeu upgrade` trust a release by; a workflow started
from a branch, or a `gh` romeu runs with the wrong flags, would pass an
archive that `main` did not build. Starting `gh` from romeu widens the
one program list of I16 (05 5.2) and opens an HTTPS surface in a
binary that had none; both are hard to take back once a release
depends on them. The upgrade's prompt is gate 1 (01 1.4), so where it
sits decides whether a `no` can undo anything.

Scope: this repository; written in 10 10.2 (REQ-1338, REQ-1339), 04
4.1 and 04 4.2, 05 I16, 03 3.4 and J9.

### Alternatives considered

- **Start `release.yml` on the tag push.** The file would come from
  the tagged commit, which the deferred row forbade.
- **Start by `workflow_dispatch` on `main` and check the tag's commit
  is an ancestor.** The attestation records the commit and the ref the
  run started on, so an older tag commit would fail `--source-digest`;
  only equality works, and `main` moving between the tag and the start
  means a new tag.
- **Verify the attestation in Go, with a sigstore library.** One more
  dependency in the host binary, and a second implementation of the
  verification contract beside `gh`'s, with its own flags to keep in
  step; refused on 2026-10-31.
- **Ask after the swap, and restore the old binary on `no`.** A
  restore path that can itself fail, and one more I16 admission to
  delete the copy.
- **Keep the swap-first flow and document that `no` only skips gate
  1.** The prompt would not be a decision about the version.

## Decision

Decided on 2026-10-31 and 2026-11-01:

1. `release.yml` starts by `workflow_dispatch` with the tag as its one
   input; a first job refuses unless the workflow's ref is
   `refs/heads/main`, the tag exists and its commit is the commit the
   run started on. That job guards against a mistake; the enforcement
   is on the consumer side: the verification contract gains
   `--source-ref refs/heads/main`, which the `gh` floor of 12 12.1 has,
   beside the signer workflow path and `--source-digest`. The deferred
   row on how `release.yml` is started leaves the index.
2. Every tag before v1.0.0 is a prerelease of it, `v1.0.0-alpha.<n>`
   then `v1.0.0-rc.<n>`, so `VERSION` stays `1.0.0` and `tools/release
   prepare` writes nothing but the changelog; a prerelease tag skips
   the fuzz gate of REQ-0902 while `fuzz.yml` does not exist. A
   `v0.<minor>.<patch>-alpha.<n>` form was chosen on 2026-10-31 and
   dropped on 2026-11-01, since it would have had `prepare` rewrite
   `VERSION` and the `$schema` hints of `examples/` at every release.
3. `romeu upgrade` installs a published release on the host: it reads
   the releases of the repository compiled into romeu over HTTPS with
   no credential, checks the sha256 in Go and the attestation through
   the `gh` named by the new host setting `tools.gh`, which `init`
   resolves on `PATH` under the file rule of the credential helper and
   `doctor` holds to the floor (`gh-floor`). `gh` runs for `gh
   --version` and the two commands of the verification contract only,
   with the scrubbed environment; REQ-1060 and I16 admit those call
   sites and nothing else.
4. The prompt comes before the swap: the running romeu runs the new
   binary once as a preview subprocess (`upgrade --preview <running
   version>`), which prints the changelog sections since the running
   version and what `approve --toolchain` prints; a `no` deletes the
   download and changes nothing; a `yes` renames the new binary over
   the running one and replaces the process (`upgrade --finish
   <running version>`), which writes `toolchain.json` as gate 1 and
   runs the steps of `sync --all`. The two argv forms are kept by every
   later release, since the old binary chooses them.
5. `romeu upgrade` takes the newest published release that is not a
   draft, a prerelease included; a `--stable` default is a deferred row
   of the index, reopened by a second adopter.

## Consequences

- J9's first step is `romeu upgrade`; installing by hand with the entry
  of J1 step 2 stays the path for a host without `tools.gh`.
- The error ids `RJ-362` to `RJ-366` and the `gh-floor` row of `doctor`
  join 04; `tools.gh` joins 03 3.4 and the schema `host-settings.v1`.
- The tasks are T088 (slice 1), T193, T195 and T196.
- A `main` that moves between `tools/release prepare` and `tools/release
  tag` means a new `prepare`; the drift check of `notes` and the
  equality check of the first job refuse the rest.
