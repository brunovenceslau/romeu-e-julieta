# cobra writes outside the CLI writer in completion

The security audit of T008 (2026-10-18) found two paths of cobra
v1.10.2 that write outside the CLI writer, both only in the hidden
`__complete` commands:

- A completion line with a flag parse error makes cobra print
  "[Error] ..." straight to `os.Stderr` (`CompErrorln`), with the text
  the user typed, unescaped. The shell script discards that stderr.
- `BASH_COMP_DEBUG_FILE` names a file cobra appends debug lines to,
  mode 0644, with the typed command line.

Both carry only the user's own command line, and the second needs
control of the user's environment. Options: swap `os.Stderr` for a pipe
copied through the CLI writer while `__complete` runs, and unset
`BASH_COMP_DEBUG_FILE` in `Main`; or name both as raw exceptions in 04
4.1, Output.

Recommended: fix the first in `Main` and unset the variable, in a pull
request of its own before M1's gate.

Reopen: the operator decides, or the first command with a completion
function lands.
