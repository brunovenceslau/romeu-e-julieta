# fieldalignment stays out

What is at stake: small; a struct laid out for readability uses a few
bytes more, and nothing else changes. Undone by enabling the analyzer.

Scope: this repository.

Decided on 2026-10-20: the `fieldalignment` analyzer of
golang.org/x/tools is not enabled and its `-fix` is not run. Measured
with v0.52.0 over the tree: 188 findings, 183 of them only the bytes the
garbage collector scans and 5 struct sizes, of which 3 stay in the same
allocator size class and 2 waste 16 bytes each (`status.Worktree`,
`check.Check`), in types with few instances. Its `-fix` drops field
comments, and here the field order is behavior: encoding/json/v2 writes
struct fields in declaration order in the `--json` outputs, and yaml.v3
writes the front matter of memory and handoff files in that order.

Reopen: a memory profile that names a struct whose layout matters.
