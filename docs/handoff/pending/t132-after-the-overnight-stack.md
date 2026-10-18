# T132 after the overnight stack

T132 (step durations and the `fast` budget) changes the verdict line
`ok <name>` that five test files pin: `tools/ci/all_test.go`,
`tools/ci/fast_test.go`, and the tests of drift, license and setup. Two
of them belong to tasks open in the overnight stack (T009 and T128), and
its `owns` still names the missing `tools/ci/internal/run/progress*.go`
(pending note "t132-and-t123-owns-and-the-build-constraint-gap").

Recommended: after the stack merges, one re-plan that gives T132
`tools/ci/internal/check/printer*.go` and the five test files, then the
task.

Reopen: the merge of the overnight stack.
