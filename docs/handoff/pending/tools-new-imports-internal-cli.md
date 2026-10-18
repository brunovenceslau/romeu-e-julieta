# tools/new imports internal/cli, which REQ-0971 does not allow

REQ-0971 (10 10.7): `tools/**` may import `internal/procgroup` directly
and any other package under `internal/` through
`internal/cmd/<binary>/all`; `tools/ci` also imports
`internal/cli/clidoc`. Measured on 2026-10-18:

- `tools/new` imports `internal/cli` directly, since T009 (#138): the
  `command` kind reads `cli.Builtins`, and the `error` kind reads the
  error table with `cli.ReadTable` and `cli.SpecRows`.
- Test packages of `tools/ci` import `internal/cli` directly too, from
  before T009.

T010 (`tools/ci imports`) would enforce the rule, so it waits for the
choice. Options: (a) the spec lets `tools/new` import `internal/cli`
the way `tools/ci` imports `clidoc`, and says whether test packages
count; (b) `internal/cli/clidoc` re-exports what `tools/new` needs and
the spec lets `tools/new` import it. Recommended: (a), one sentence in
REQ-0971.

Reopen: the operator's choice, then T010.
