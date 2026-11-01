# 44. Generate a changelog file with git-cliff and make each section the release body

Date: 2026-10-31

## Status

Proposed

Supersedes in part [1. Adopt a documentation standard with checkable rules and a voice](0001-adopt-a-documentation-standard-with-checkable-rules-and-a-voice.md) (rule 11)

## Context

Until this record the repository kept no changelog file: rule 11 of
ADR 0001 said that release notes are generated from Conventional
Commit subjects at publish time, and 12 12.6 had `tools/release notes`
read the merged pull requests, their authors and the issues they close
through the GitHub API inside `release.yml`. No release has been made
yet (measured on 2026-10-31: no tag, no release), and the binaries in
use are old because building and installing one by hand is impractical.
The work that fixes that, an automatic release on a tag and
`romeu upgrade` on any host, needs a text that tells the operator what
changed before they confirm the new toolchain (gate 1, 01 1.4), and
that text has to be the same wherever it is read: in the repository,
on the release page and inside the binary.

What is at stake: notes generated at publish time exist only on
GitHub, differ from one run to the next when the API state changes,
and cannot be embedded in the binary that is being built from the same
commit. An operator upgrading on a host without network would see
nothing. A hand-written changelog would drift from the merged pull
requests, which is the reason rule 11 forbade it.

Scope: this repository; written in 12 12.6, 12 12.1, 10 10.2 and
rule 11 of ADR 0001.

### Alternatives considered

- **Keep the notes generated at publish time, no file.** The state
  before this record. The notes cannot be embedded, and the GitHub
  integration of any generator depends on API state at generation
  time, so two builds of one commit can give two texts.
- **A hand-written `CHANGELOG.md`.** Forbidden by rule 11 for a reason
  that still holds: nothing checks it against what merged.
- **A generated file, written by `tools/release` itself.** Writing a
  changelog generator is the kind of generic work
  [ADR 0038](0038-search-for-a-maintained-library-before-writing-one.md)
  sends to a maintained tool first. `git-chglog` (Go) is archived and
  points at git-cliff; no maintained Go alternative was found.
- **A generated file, written by git-cliff, in a release pull request
  before the tag.** Taken. git-cliff (Rust, Apache-2.0, v2.14.2 of
  2026-09-18, active) is in the mise registry under the `aqua`
  backend, the one `mise.lock` already uses for `gh` and
  `golangci-lint`, so it is pinned with a sha256 per platform like
  them. Run with `--offline` and `--no-exec`, its output is a function
  of the git history and `cliff.toml` alone. The repository merges by
  merge commit only, so a parser that keeps merge commits gives one
  entry per pull request, with the pull request's title as subject
  and its body, including its `## Why` section, as body; no API is
  read.

## Decision

Decided on 2026-10-31 and 2026-11-01:

1. `CHANGELOG.md` lives at the root of the repository, holds one
   section per tag, newest first, and git-cliff writes it from
   `cliff.toml`. Nobody writes it by hand; rule 11 of ADR 0001 is
   reworded to say so.
2. The file is written before the tag, in a release pull request
   (`chore(release): <tag>`), so the tagged commit holds the section
   of its own tag. The section is the body of the GitHub release and
   the text romeu embeds and prints in `romeu upgrade`, byte for byte.
   `release.yml` refuses a section that differs from what git-cliff
   renders at the tag, so a pull request merged after the release pull
   request, or a hand edit, cannot pass.
3. Each section covers the merged pull requests since the previous
   tag, a prerelease or not. The section of a stable tag adds one
   generated line linking the earlier sections of the file, so its
   release page points at what the prerelease sections hold. A stable
   section that sums everything since the last stable tag was refused:
   it needs the prerelease tags ignored, which drops their sections,
   or entries written twice.
4. The `## Why` section of the pull request body is the Why of the
   entry, so `tools/ci ask-first` requires that literal heading
   (12 12.4, REQ-1131). The author is not listed while the repository
   has one contributor; issues named in the body become links.
5. The Upgrading section of 12 12.6 (error ids, exit codes, format
   versions, kit digests) stays computed by `tools/release`, which only
   it can do, and enters the section through `--with-tag-message`.
6. git-cliff joins the closed list of mise tools of REQ-1051
   (surface `dependencies`), started by the path `mise which` resolves
   (REQ-1053).

## Consequences

- 12 12.6 is rewritten around the file; `tools/release notes` reads the
  file instead of the GitHub API, and the publish job of `release.yml`
  needs no `pull-requests` or `issues` permission.
- 12 12.1 (REQ-1051, REQ-1062) and `mise.toml`, `mise.lock` gain
  git-cliff; the task that adds the pin carries the `dependencies`
  approval line.
- The task of `tools/release changelog`, `prepare` and `tag` writes
  `cliff.toml`; its template is a reviewed part of that task, since the
  operator reads its output at every upgrade.
- The release pull request is one more merge per release, by the
  maintainer; `tools/release tag` refuses a tag that is not on its
  merge commit, and the drift check of `release.yml` refuses a section
  that no longer matches.
- Pull requests whose body writes "Why" in prose and not as `## Why`
  stop passing `tools/ci ask-first` once that check lands.
