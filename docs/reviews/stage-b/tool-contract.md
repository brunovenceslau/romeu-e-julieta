# `tools/ci reqs`: the tool contract (second synthesis)

Go, under `tools/ci`, in the idiom `tools/ci` has at 1e63d6a (read from `tools/ci/main.go`, `tools/ci/adrindex.go`, `tools/ci/hygiene.go` with its `add` verb, and `tools/new/main.go`): one subcommand in `main.go`'s `switch`, a `runReqs(ctx, e, args)` in `tools/ci/reqs.go` returning `(ok bool, err error)`, exit 0 / 1 / 2 as every check does, findings printed one per line through `say` and closed by `summary("reqs", n)`, the tree read and written through `os.OpenRoot` of the repository root so a link cannot send it outside, `git.SafeLines` on error text, and output through the writer the dispatcher hands it (never `os.Stdout`), so a test captures it. T105 later moves it to `tools/ci/internal/reqs/` with `tools/ci/check_reqs.go` like every other check, registering itself as a tree-judging check (decision 17 of plan-changes.md section 2 and skill-go-code-7); the behaviour below does not change. It uses the standard library alone (12 12.1's dependency list is complete), and stdlib `flag` for its one verb, as the repository decides (the `golang-cli` skill's cobra default is not followed for that reason).

verify-4 refuted the alternative (`tools/new req`, a `docs/spec/reqs.yaml` ledger, skill-go-code-task-T122-reqs-tooling): the allocator edits existing pages, a ledger is a second list of what the pages hold, and a collision is covered by the rebase-and-rerun rule of 12 12.3.

## Inline format

- A live id is the code span `` `REQ-nnnn` `` with exactly four digits, matched by `` `REQ-[0-9]{4}` ``; the first token of the sentence, bullet, numbered item or table cell it governs (12 12.11).
- The placeholder is the code span `` `REQ-new` ``.
- Document order: `docs/spec.md`, then `docs/spec/00-*.md` to `docs/spec/13-*.md` in filename order; within a file, by line, then left to right.
- The retired table is the table under the heading `**Retired requirements.**` of 12 12.11, columns `Id | Retired on | Last text | Replaced by`.

## `go run ./tools/ci reqs allocate` (writes)

1. Collect the live ids of the specification pages and the ids of the retired table; `max` is the highest of both, 0 when there is none.
2. Walk the pages in document order; replace each `` `REQ-new` `` by `` `REQ-<max+k>` `` for the k-th placeholder found, zero-padded to four digits; write each changed file back byte for byte otherwise.
3. Print one line per replacement (`<file>:<line>: REQ-nnnn`) and the summary line.
4. Idempotent: a tree without placeholders changes no byte and prints `reqs: ok`.
5. Exit 2 (nothing written) when a page cannot be read, when `max+k` would exceed 9999, or when a placeholder sits in an excluded page or section (the check below would fail the result).

## `go run ./tools/ci reqs` (checks; in `fast` and `all`)

Fails (exit 1, one finding per line) on:

1. a placeholder left in any page;
2. a token `REQ-` followed by anything but four digits or `new`, inside a code span;
3. an id that appears twice across the pages;
4. an id both live and in the retired table;
5. a number from 1 to `max` that is neither live nor retired (a dropped obligation is retired, never deleted);
6. an id inside an excluded page or section: `docs/spec/13-*.md`; `docs/spec/09-*.md` from its first `## J` heading to the end; `docs/spec.md` from `## Open questions` to the end; `docs/spec/00-*.md` from `## 0.5` to the next `## ` heading; any file under `docs/adr/`;
7. a retired-table row that does not parse (four cells, an id, an ISO date).

The coverage half, present only when `docs/plan/tasks/*.md` exist (the plan branch), fails on:

8. an Implements entry of a task file whose id is not live (retired or unknown); the Implements grammar is `- Implements: REQ-nnnn (<section>), ...`, and the parser reads the ids only (the parenthetical is a reading aid in either of the two forms the spec cites itself with, `<page> <n>.<m>` or `<page> (opening)`, and is not parsed);
9. a live id that no task of fate `kept`, `new`, `deferred` or `merged` cites (`folded`, `removed` and `left-v1` files are not read); an id cited only by `deferred` tasks is reported in a second summary line and does not fail.

A task file with no front matter or no `fate:` is a finding of `tools/ci plan` (T106), not of `reqs`. `reqs` judges the commit's tree like `hygiene` and `workflows` do (the tree-judging flag of T105's registry; skill-go-code-9's rule for `plan` applies to it too), so an uncommitted task file never satisfies the coverage half in a pre-push run.

## Tests

Fixtures in `tools/ci/reqs_test.go` (in-memory trees through `os.Root` on `t.TempDir()`, as the hygiene tests do): one per failure above, titled by the failure; "allocate is idempotent"; "allocate numbers in document order from the highest live or retired id"; "the duplicate check catches a double allocation" (two trees allocated from one base, merged); "the coverage half is silent without task files"; "a deferred-only id is reported and passes"; "an untracked task file does not satisfy the coverage half". S9's 80% holds for the package.

## What is deliberately not in the tool

- No `--renumber`: ids are stable; the rebase-and-rerun rule of 12 12.3 plus failure 3 cover a double allocation.
- No `tools/new req` kind and no ledger file: the allocator edits existing pages, and `tools/new` scaffolds new files.
- No parsing of section cites in Implements lines: the parenthetical is a reading aid.
- No citation of ids from code or tests: deferred to the first implementation task that needs it (decision (a) of 2026-10-12).
