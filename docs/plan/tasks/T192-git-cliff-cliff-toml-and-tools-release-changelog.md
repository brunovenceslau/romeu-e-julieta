---
id: T192
title: "git-cliff, cliff.toml and tools/release changelog"
milestone: M3
module: ci-release
fate: new
owns:
  - cliff.toml
  - CHANGELOG.md
  - mise.toml
  - mise.lock
  - tools/ci/internal/mise/**
  - tools/ci/pinned/**
  - tools/release/internal/changelog/**
  - tools/release/main.go
needs:
  - T103
after: []
ask_first:
  - dependencies
  - checks
  - release
sitting: none
---

# T192 - git-cliff, cliff.toml and tools/release changelog

Goal: the generated `CHANGELOG.md` of 12 12.6: git-cliff pinned,
`cliff.toml`, the file with its header, and `tools/release changelog
--version <tag>`, which writes the section of a tag with its Upgrading
part.

Added on 2026-11-01 (ADR 0044; decided on 2026-10-31 and 2026-11-01,
scope this repository): the release notes become the sections of a file
in the repository, generated before the tag, so the release body and the
text romeu embeds are one text. git-cliff is the generator (ADR 0038:
git-chglog is archived and points at it; no maintained Go alternative
was found), pinned in `mise.toml` and `mise.lock` under the `aqua`
backend like `gh`, and it joins `PinnedTools` of `tools/ci/internal/mise`
(REQ-1052), so `mise which` resolves it (REQ-1053). The Upgrading
section is computed here, since only `tools/release` can read the two
commits' `errors.go` declarations, `internal/spec/versions.go` and the
embedded kit digests; it reaches git-cliff through `--with-tag-message`.
T193 (prepare, tag) and T088 (release.yml, notes) build on this task.
The template of `cliff.toml` is a reviewed part of this task, since its
output is read at every upgrade.

Built on 2026-11-01 (scope this repository), what the build measured
with git-cliff 2.14.2 and decided: `tools/release` cannot import the
mise package of `tools/ci` (the internal rule of Go), so the public
package `tools/ci/pinned` is the door to `WhichPinned` for the trees
outside `tools/ci`, and this task owns it; an in-branch merge of a
session carries the type `merge` (`merge: main into <branch> for
...`), which is in Conventional Commit form, so `cliff.toml` skips that
type too, and the entries of a section are exactly the first-parent
pull request merges (258 on `main` at 1dbc34b4: a merge under GitHub's
default subject `Merge pull request #<n> from <owner>/<branch>`, the
form of the first 49, holds the pull request's title as the first line
of its body, and a preprocessor makes that title the subject with the
number, so the 46 whose title is conventional are entries and the 3
whose title is plain are skipped; decided on 2026-11-01, so the first
section holds the foundation pull requests); `--prepend` writes the header of the configuration,
the section and two newlines before what followed the marker, so the
command normalizes the seams after it (one blank line after the marker,
one between sections, one newline at the end) and nothing inside a
section, and `--current --strip all` at the tag renders the same bytes
as `--unreleased --tag` before it; a prerelease suffix must be
`alpha.<n>` or `rc.<n>` (REQ-1337); the command refuses a render with
no entry (no merged pull request since the previous tag; the bullets of
the Upgrading part are not entries) and a repository without the remote
`origin`, where the tag is checked. The template reads the first `##
Why` section of the body, by its exact heading, after the HTML comments
of the body are removed as REQ-1146 reads it; a removed error id or kit
is not listed, since REQ-1175 asks for what was added or changed.

- Implements: REQ-1051 (12 12.1), REQ-1062 (12 12.1), REQ-1340 (12
  12.1), REQ-1173 (12 12.6), REQ-1174 (12 12.6), REQ-1175 (12 12.6),
  REQ-1176 (12 12.6), REQ-1177 (12 12.6), REQ-1341 (12 12.6), REQ-1344
  (12 12.6), REQ-1346 (12 12.6), REQ-1350 (12 12.6).
- Operator: no.
- Acceptance:
  - `mise.toml` and `mise.lock` pin git-cliff at v2.14.2 (the newest
    release, measured on 2026-10-31) with a sha256 for each of the four
    platforms of `mise.lock`; `PinnedTools` lists it, and the mise test
    of `tools/ci` (10 10.2) reads the list, so `mise which git-cliff`
    resolves and `mise exec` is never used.
  - `cliff.toml`: one entry per merge commit of `main`, every other
    commit skipped (`{ field = "merge_commit", pattern = "false", skip =
    true }`), a merge whose subject is not in Conventional Commit form
    skipped too (`Merge origin/main into <branch>`, a fixture), and one
    of the type `merge` (the in-branch merge of a session); the
    release pull request skipped by its subject (`^chore\(release\)`);
    the section heading is `## <tag>` and the static header holds no
    level-2 heading and the SPDX lines REUSE asks of the file; no date
    anywhere in a section; entries grouped by Conventional Commit type;
    a subject with `!` or a body with the `Approval: contracts` line
    listed first (`protect_breaking_commits` and a body parser); under
    each entry the `## Why` section of the body, rendered from
    `commit.body` with the template's filters; `#N` and the closing
    keywords as links; no author; the section opens with the link to
    `docs/` at the tag, a stable tag's section adds the line that links
    `CHANGELOG.md` at the tag, and `release.message`
    renders the Upgrading part. The test builds a fixture repository
    from a script of merge commits and renders a golden section; the
    golden is where the review of this task reads the template whole
    (one entry per line, the Why under it, every issue and pull request
    a link).
  - `CHANGELOG.md` is committed with the static header of `cliff.toml`
    and no section, so `--prepend` finds it at the first tag.
  - `tools/release changelog --version <tag>` refuses a tag that is not
    of the form of REQ-0899 and REQ-1337, or that exists on the local
    repository or on `origin`; starts git-cliff by `mise which` with
    `--offline --no-exec --tag <tag> --unreleased --prepend CHANGELOG.md
    --with-tag-message <upgrading>`; exits non-zero with git-cliff's
    output, escaped, when it fails; with `--stdout` prints the section
    and writes nothing; refuses a file that already holds the section,
    a render with no entry and a repository without `origin`; after
    `--prepend` it rewrites the seams of the file alone (one blank line
    after the marker and between sections, one newline at the end), so
    `Section` of the package, which `notes` reads by (REQ-1350), finds
    the rendered bytes. Its Upgrading part lists the error ids and exit
    codes added or changed, the format versions changed and the kits
    whose digest changed between the previous tag and `HEAD`, and for
    the first tag one line saying there is no previous tag; a fixture
    per case. Two runs on one commit give one file.
- Verify: `go test ./tools/release/internal/changelog/... -v`, which
  lists the fixture cases by name; `go test ./tools/ci/internal/mise/...
  -v`; `go run ./tools/ci mise`.
