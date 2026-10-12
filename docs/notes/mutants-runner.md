# The mutants runner

Reader: whoever writes tests and wants to know whether they catch a
change, and whoever builds the invariant tooling of T007. Type:
explanation.

`go run ./tools/mutants [-worktree] <mutants-file>` runs mutation tests
on demand. A YAML file lists mutants: each names a file, an exact
snippet that occurs once in it, the snippet to put in its place, and the
`go test` package and `-run` pattern meant to catch the change. The
runner reports each mutant as killed, survived or invalid. Its package
comment holds the full contract: the file format, the verdicts, the exit
codes and the go command it runs.

## What it is for

A test earns trust when a wrong line makes it fail. Proving that by hand
means editing a tracked file, running the tests and putting the file
back, and the putting back is where work gets lost: a checkout or a
stash in a shared working tree reaches further than the one file. The
runner does the whole loop in a throwaway clone. It never writes to the
working tree, the index, a ref or the stash of the repository it starts
in, so nobody has to edit tracked files to mutate them.

With `-worktree`, the clone holds the working tree as it is, uncommitted
and untracked files included, so new tests can be checked before the
commit that adds them. The snapshot goes through an index of its own and
leaves only objects behind.

## Why it is a separate command from `tools/ci mutate`

[T007](../plan.md#t007---termsafe-the-cli-writer-and-the-invariant-tooling)
plans `tools/ci mutate`, a gate of `tools/ci`: it builds each security
guard's `mutate_I<n>` stub, which disables the guard through a build tag,
and requires a tagged test to fail. The two answer different questions.

- `tools/ci mutate` is a check that runs on every pull request it
  applies to, over a fixed set of guards declared in the code. It lives
  in `tools/ci`, on the `checks` ask-first surface.
- `tools/mutants` is a tool a person or an agent runs while writing
  tests, over mutants listed in a file for the change at hand. It gates
  nothing, so it lives outside `tools/ci`, and a change to it needs no
  approval of that surface.

Keeping the names apart keeps the two from being taken for one another.

## What T007 may reuse

If the gate of T007 needs to build or run code away from the working
tree, the clone and snapshot engine of `tools/mutants` is there to reuse:
the clone with hooks off and its remote removed, the snapshot through a
private index, the go environment built from the shared lists of
`tools/ci/goenv`, and the process-group handling of
`tools/ci/procgroup`. Moving that engine into a shared package of
`tools/ci` is the natural step when the gate needs it.
