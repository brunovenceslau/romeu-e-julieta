# 15. Adopt spf13/cobra for the command line of romeu and julieta

Date: 2026-10-17

## Status

Accepted

## Context

The specification chose the standard library's `flag` package and no
CLI framework for both binaries: "No CLI framework (stdlib `flag`)"
(REQ-1050, 12 12.1), with the reason "two binaries with ~15 and ~25
commands do not need cobra, and fewer dependencies shrink the host
trust surface". The row "stdlib `flag`, no CLI framework" of the
product-decision table of 12 12.5 waits for its record (T123).

T008's first implementation, written on 2026-10-17, built the CLI
contract on that choice. Each review round of it found another piece a
framework already holds:

- the first version modelled one command word, while 04 4.3 has
  twelve pairs of a command and a subcommand (`julieta memory add`,
  `julieta pin check`, ...) and 04 4.1 asks for `--help` on every
  level;
- the help printed no value name and no default, and its first rule
  for the default hid a string default of `"0"`, which the standard
  library's own `flag.PrintDefaults` already decides correctly;
- a flag before a subcommand, `-h` and `help` at each level, a flag
  that clashes with `--help` or `--json`, and the column widths each
  needed code and tests of their own;
- shell completion and generated per-command reference pages were not
  in reach at all.

Decided on 2026-10-17, for that cost: both binaries move to
spf13/cobra, with GNU-style flags, shell completion and reference pages
generated from the command tree, and the tools of the repository keep
`flag`.

What we measured before deciding (2026-10-17, through
proxy.golang.org):

- The latest cobra is v1.10.2, released 2025-12-03; the latest pflag
  is v1.0.10, released 2025-09-02; cobra v1.10.2 requires pflag
  v1.0.9.
- A module that imports `github.com/spf13/cobra` alone links two
  modules into a linux or darwin binary: cobra and pflag
  (`go list -deps` with `GOOS` set to each). Its module graph also
  holds `github.com/inconshreveable/mousetrap` v1.1.0, which cobra
  imports on Windows only, and `go.yaml.in/yaml/v3`, already ours.
- Importing `github.com/spf13/cobra/doc` adds
  `github.com/cpuguy83/go-md2man/v2` v2.0.6 and
  `github.com/russross/blackfriday/v2` v2.1.0.
- Licenses, read from each module's license file: cobra and mousetrap
  Apache-2.0, pflag BSD-3-Clause, go-md2man MIT, blackfriday
  BSD-2-Clause. Each is one the GPL-3.0-only product can take.
- cobra's default help and usage are `text/template` templates
  (`cobra.go`); the help this record keeps does not use them.

### Alternatives considered

- Keep stdlib `flag` and finish the hand-written dispatcher: declined
  for the cost above, which grows with each command task.
- `github.com/urfave/cli/v3` or `github.com/alecthomas/kong`: both
  hold subcommands and help; declined because cobra is the most used
  of the three in Go, its completion and document generators come with
  it, and the skills this repository's agents follow cover it.
- cobra together with viper: declined. Host settings are a versioned
  format with its own strict decoder (03 3.4), and a second resolver of
  configuration would be a second source.

## Decision

1. **cobra for the two binaries.** `romeu` and `julieta` build their
   command line with `github.com/spf13/cobra` v1.10.2 and its flag
   package `github.com/spf13/pflag` at v1.0.9, the version cobra
   requires; a later pflag is an ask-first change of its own, never a
   side effect of another bump. Both join the dependency list of 12
   12.1 with T008, the first task that imports them (plan rule 7). The
   tools of the repository (`tools/ci`, `tools/new`, `tools/mutants`,
   `tools/release`) keep the standard library's `flag`. Reopened by: a
   tool whose command line grows a second level of subcommands.
2. **Registration by file stays.** Each command package registers one
   definition from a package-level `var _ = cli.Register(...)` (T008),
   the registration by file of ADR 0014: the binary, the command's path
   (its command and, for a subcommand, its subcommand), the five
   side-effect fields of 12 12.3 as the arguments of the one
   constructor, and a function that returns a new `*cobra.Command`.
   Registration fails for a command that sets no `Args` validator, so
   no command takes arguments it does not name. `internal/cli`
   assembles the tree of the binary from the registered definitions on
   each run, with a group command of `cobra.NoArgs` above each set of
   subcommands, so no package holds a root command, no package calls
   `AddCommand` on another's command, and nothing is written after
   `main` starts (REQ-1229). A test builds its own tree the same way.
   The commands cobra itself adds (`help`, `completion`, `__complete`
   and `__completeNoDesc`) have no definition; `internal/cli` lists
   them, and the tests that read the side-effect fields skip them.
3. **GNU-style flags.** Flags follow pflag: `--name`, `--name=value`
   and `--name value`, a one-letter short form only where a row of 04
   names one (`romeu pull -C <dir>`), flags and arguments in any order,
   and `--` ends the flags. pflag reads a single dash before letters as
   a cluster of short forms, which no other command defines, so `-json`
   is an unknown flag. `julieta hooks run <hook> [args]` passes git's
   arguments on unparsed, so it alone turns flag parsing off; it takes
   no `--help`, and its hook name passes the validators of 03 3.1
   before any argv is built.
4. **Errors and exit codes do not change.** The commands use `RunE`,
   and `internal/cli` wraps each one: an error it returns that holds no
   `*cli.Error` becomes the error `internal` (`RJ-348`). Every error of
   the parser becomes the error `usage` (`RJ-349`, exit 2) where it
   arises: a flag error through `SetFlagErrorFunc`, an argument count
   through the `Args` validators, and an unknown command or subcommand,
   so whatever else `ExecuteContext` returns is `usage` too. The root
   sets `SilenceErrors` and `SilenceUsage`, takes its arguments from
   `SetArgs`, sets no `Version`, and leaves prefix matching, case
   folding and `TraverseChildren` off, so an abbreviation never runs a
   command. Nothing in the product calls `cobra.CheckErr`,
   `cobra.CompDebug` or a `Deprecated` field, each of which writes or
   exits outside the CLI writer. `cli.Main` runs the tree under
   `procgroup.Main` and prints the result through `cli.Report`; the
   exit codes of 04 4.1 and the signal statuses 130 and 143 stay as
   they are.
5. **All output through the CLI writer.** The tree's output and error
   streams are the escaping writers of `cli.Env` (I29), set before the
   tree runs. Its help and usage functions are `internal/cli`'s, which
   add the exit-code section generated from the error table (12 12.3)
   to every level and print each flag with its value name and its
   default, and its help command is `internal/cli`'s too
   (`SetHelpCommand`), which answers an unknown topic with the error
   `usage`. cobra's template help is not used, so the rule against
   `text/template` for structured output (10 10.6) is not in question.
6. **Shell completion.** Each binary has its own `completion` command
   for bash, zsh and fish, built on cobra's generators, and cobra's
   hidden `__complete` commands. The script and the `__complete` output
   need real tabs and line feeds, so they are a named exception to the
   escaping writer (04 4.1, Output), and they hold only the binary's
   own static text and names that pass the validators of 03 3.1: a
   completion description is static text, never text from a record or
   an agent. A completion function reads records only: it starts no
   subprocess, opens no network connection, takes no lock, writes
   nothing, returns at most 1000 names and stops after one second. The
   hidden commands take no `--json` and can do nothing a typed command
   cannot. cobra reads `COBRA_ACTIVE_HELP`,
   `COBRA_COMPLETION_DESCRIPTIONS`, their `ROMEU_` and `JULIETA_` forms
   and `BASH_COMP_DEBUG_FILE`, which names a file its debug output is
   appended to; the product uses neither active help nor that debug
   output.
7. **Reference pages from the tree.** `go generate ./...` writes one
   `docs/reference/<binary>/<command path>.md` page per command, the
   words of the path as directories, by walking the assembled tree and
   calling `GenMarkdownCustom` of `github.com/spf13/cobra/doc` for each
   command with `DisableAutoGenTag` set, so a page changes only with its
   command. The page opens with the generated-file header and the front
   matter of REQ-1117, and ends with the P marker, the appending list
   and the exit codes, which the generator writes from the definitions,
   since `cobra/doc` renders neither the side-effect fields nor the help
   functions of decision 5. A generator lives in the package that owns
   its data: this one in `internal/cli/clidoc`, a subpackage next to
   `internal/cli` that no binary imports, and the generator of the
   error table's pages in `internal/cli`. The generated check of
   `tools/ci` holds no generator of its own; it reaches both through
   `internal/cli/clidoc`, calls them and checks the drift, and `go
   generate ./...` keeps its one directive in `tools/ci/generate.go`. A
   generated Go file is named `*_gen.go` and carries no generated-code
   header, which the lint step of 10 10.2 refuses, so every lint finding
   in it stays visible.
8. **The import boundary is checked.** `github.com/spf13/cobra` is
   imported only by `internal/cli`, `internal/cli/clidoc` and the
   packages under `internal/cmd/`, and `github.com/spf13/cobra/doc` only
   by `internal/cli/clidoc`, which only `tools/ci` imports (10 10.7);
   `tools/ci imports` fails another importer, and a test fails when `go
   list -deps` of `./cmd/romeu` or `./cmd/julieta`, for each of their
   targets, holds `internal/cli/clidoc`, `cobra/doc`, go-md2man,
   blackfriday or mousetrap. The license step reads the whole module
   graph of `go.mod`, the modules no binary links included.

## Consequences

- The specification changes in one pull request with this record:
  REQ-0267 (Parsing), REQ-0270 (the raw-output exceptions), REQ-0282
  (the reference pages), REQ-0446 (the error `internal`), REQ-1049 and
  REQ-1050 (12 12.1, the dependency list and the framework clause), the
  "Why for us" sentence of 12 12.1, REQ-1109 and REQ-1124 (12 12.3, the
  definitions and the `command` kind of `tools/new`), the `completion`
  and `help` commands of 04 4.2 and 04 4.3 and the error `usage`
  (REQ-1242 to REQ-1244), the completion and import-boundary rules
  (REQ-1245 and REQ-1246), and the row "stdlib `flag`, no CLI
  framework" of 12 12.5, which leaves the table for this record.
- The plan changes in a re-plan pull request of its own: T008 owns
  `go.mod` and `go.sum` for the two modules and comes before T125 and
  T053, which own them today (plan rule 7); T008, T009, T123, T125 and
  T053 are rewritten for this record. The command tasks need no change:
  their files name what a command does, not the API it is written in.
- T008's first implementation is reworked on cobra before it merges. The
  error table, the union test, `Report` and its `--json` form, the
  generator of `errors.md` and `exit-codes.md` and `RequireID` carry
  over; the dispatcher, the help and the registry are rewritten.
- The host binaries link two more modules. Their versions are pinned in
  `go.mod`, their licenses are read by the license step's allowlist
  (T125), and govulncheck reads them in `all`.
- A user who typed `-json` or `-help` with one dash gets the error
  `usage`; no release has shipped, so no one has.
